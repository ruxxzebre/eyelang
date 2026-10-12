/* The fs and process primitives (check/std/fs.eye, process.eye), as the
 * interpreter's host functions have them (interp/system.go). In a test,
 * relative paths and programs work in an empty directory of the test's
 * own, removed when the test ends. */
#define _POSIX_C_SOURCE 200809L
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/stat.h>
#include <dirent.h>

#ifdef _WIN32
#include <direct.h>
#include <io.h>
#define mkdir_one(p) _mkdir(p)
#else
#include <fcntl.h>
#include <poll.h>
#include <signal.h>
#include <sys/wait.h>
#include <unistd.h>
#define mkdir_one(p) mkdir(p, 0755)
#endif

#include "eye.h"
#include "builtins.h"

/* ---------------------------------------------------------------- the test's directory */

static char work[4096]; /* the running test's directory, once it has one */
static bool sandbox;

void ey_sandbox(bool on) { sandbox = on; }

static bool absolute(const char *p) {
#ifdef _WIN32
	return p[0] == '/' || p[0] == '\\' || (p[0] && p[1] == ':');
#else
	return p[0] == '/';
#endif
}

static const char *work_dir(void) {
	if (!sandbox) return NULL;
	if (!work[0]) {
		const char *tmp = getenv("TMPDIR");
#ifdef _WIN32
		if (!tmp) tmp = getenv("TEMP");
#endif
		if (!tmp) tmp = "/tmp";
		for (int i = 0; i < 1000; i++) {
			snprintf(work, sizeof work, "%s/eye-test-%d-%d", tmp, (int)(rand() % 1000000), i);
			if (mkdir_one(work) == 0) return work;
		}
		ey_fault("<test>", "making the test's directory failed");
	}
	return work;
}

/* path is where p, as the program wrote it, is: in a test, inside its directory */
static char *path(ey_v p) {
	ey_str *s = EY_STR(p);
	const char *dir = work_dir();
	size_t n = (size_t)s->len + (dir ? strlen(dir) + 2 : 1);
	char *out = malloc(n);
	if (dir && !absolute(s->b))
		snprintf(out, n, "%s/%s", dir, s->b);
	else
		snprintf(out, n, "%s", s->b);
	return out;
}

static void remove_all(const char *p) {
	struct stat st;
	if (stat(p, &st) != 0) return;
	if (S_ISDIR(st.st_mode)) {
		DIR *d = opendir(p);
		if (d) {
			struct dirent *e;
			while ((e = readdir(d))) {
				if (!strcmp(e->d_name, ".") || !strcmp(e->d_name, "..")) continue;
				size_t n = strlen(p) + strlen(e->d_name) + 2;
				char *child = malloc(n);
				snprintf(child, n, "%s/%s", p, e->d_name);
				remove_all(child);
				free(child);
			}
			closedir(d);
		}
		rmdir(p);
	} else {
		remove(p);
	}
}

void ey_sandbox_end(void) {
	if (work[0]) remove_all(work);
	work[0] = 0;
}

/* ---------------------------------------------------------------- fs */

/* the FsError type, from a Result(_, FsError) */
static const ey_type *fs_error(const ey_type *res) { return res->variants[1].fields[0]; }

static ey_v reason(int err) {
	ey_buf b = {0};
	ey_buf_str(&b, strerror(err));
	if (b.len > 0 && b.b[0] >= 'A' && b.b[0] <= 'Z' && (b.len < 2 || b.b[1] < 'A' || b.b[1] > 'Z')) b.b[0] = (char)(b.b[0] + 32);
	return ey_buf_take(&b);
}

static void fs_err(void *out, const ey_type *res, ey_v p, int err, const char *why) {
	const ey_type *t = fs_error(res);
	uint32_t tag = err == ENOENT ? 0 : (err == EACCES || err == EPERM) ? 1 : err == EEXIST ? 2 : 3;
	ey_dup(p);
	ey_v text = tag == 3 ? (why ? ey_str_c(why) : reason(err)) : 0;
	ey_tmp e;
	ey_variant_make(t, e.b, tag, (const void *const[]){&p, &text});
	ey_err(res, out, e.b);
}

static void ok_unit(void *out, const ey_type *res) {
	uint8_t unit = 0;
	ey_ok(res, out, &unit);
}

/* read_file reads a whole file: as a String, which must be UTF-8, when
 * text is set, and as Bytes otherwise. */
static void read_file(ey_v p, void *out, const ey_type *res, bool text) {
	char *full = path(p);
	FILE *f = fopen(full, "rb");
	free(full);
	if (!f) {
		fs_err(out, res, p, errno, NULL);
		return;
	}
	ey_buf b = {0};
	char chunk[65536];
	size_t n;
	while ((n = fread(chunk, 1, sizeof chunk, f)) > 0) ey_buf_add(&b, chunk, (int64_t)n);
	int failed = ferror(f) ? errno : 0;
	fclose(f);
	if (failed) {
		free(b.b);
		fs_err(out, res, p, failed, NULL);
		return;
	}
	if (!text) {
		ey_v bytes = ey_str_new(b.b ? b.b : "", b.len, &ey_t_bytes);
		free(b.b);
		ey_ok_v(res, out, bytes);
		return;
	}
	if (!ey_utf8_valid(b.b ? b.b : "", b.len)) {
		free(b.b);
		fs_err(out, res, p, EILSEQ, "not UTF-8 text");
		return;
	}
	ey_ok_v(res, out, ey_buf_take(&b));
}

void eyb_fs_read_text(ey_v p, void *out, const ey_type *res) { read_file(p, out, res, true); }

void eyb_fs_read_bytes(ey_v p, void *out, const ey_type *res) { read_file(p, out, res, false); }

void eyb_fs_write_text(ey_v p, ey_v text, bool append, void *out, const ey_type *res) {
	char *full = path(p);
	FILE *f = fopen(full, append ? "ab" : "wb");
	free(full);
	if (!f) {
		fs_err(out, res, p, errno, NULL);
		return;
	}
	ey_str *s = EY_STR(text);
	size_t w = fwrite(s->b, 1, (size_t)s->len, f);
	int err = w != (size_t)s->len ? errno : 0;
	if (fclose(f) != 0 && !err) err = errno;
	if (err) {
		fs_err(out, res, p, err, NULL);
		return;
	}
	ok_unit(out, res);
}

bool eyb_fs_exists(ey_v p) {
	char *full = path(p);
	struct stat st;
	bool ok = stat(full, &st) == 0;
	free(full);
	return ok;
}

static int cmp_names(const void *a, const void *b) { return ey_str_cmp(*(const ey_v *)a, *(const ey_v *)b); }

void eyb_fs_list(ey_v p, void *out, const ey_type *res) {
	char *full = path(p);
	DIR *d = opendir(full);
	free(full);
	if (!d) {
		fs_err(out, res, p, errno, NULL);
		return;
	}
	ey_v *names = NULL;
	int64_t n = 0, cap = 0;
	struct dirent *e;
	while ((e = readdir(d))) {
		if (!strcmp(e->d_name, ".") || !strcmp(e->d_name, "..")) continue;
		if (n == cap) {
			cap = cap ? cap * 2 : 16;
			names = realloc(names, sizeof(ey_v) * (size_t)cap);
			if (!names) ey_out_of_memory();
		}
		names[n++] = ey_str_c(e->d_name);
	}
	closedir(d);
	if (n > 1) qsort(names, (size_t)n, sizeof(ey_v), cmp_names);
	ey_ok_v(res, out, ey_list_take(res->variants[0].fields[0], n, names));
}

void eyb_fs_make_dir(ey_v p, void *out, const ey_type *res) {
	char *full = path(p);
	size_t len = strlen(full);
	for (size_t i = 1; i <= len; i++) {
		if (i == len || full[i] == '/' || full[i] == '\\') {
			char c = full[i];
			full[i] = 0;
			if (mkdir_one(full) != 0 && errno != EEXIST) {
				int err = errno;
				free(full);
				fs_err(out, res, p, err, NULL);
				return;
			}
			full[i] = c;
		}
	}
	struct stat st;
	bool dir = stat(full, &st) == 0 && S_ISDIR(st.st_mode);
	free(full);
	if (!dir) {
		fs_err(out, res, p, ENOTDIR, NULL);
		return;
	}
	ok_unit(out, res);
}

void eyb_fs_remove(ey_v p, void *out, const ey_type *res) {
	char *full = path(p);
	struct stat st;
	int r;
	if (stat(full, &st) == 0 && S_ISDIR(st.st_mode))
		r = rmdir(full);
	else
		r = remove(full);
	int err = errno;
	free(full);
	if (r != 0) {
		fs_err(out, res, p, err, NULL);
		return;
	}
	ok_unit(out, res);
}

/* ---------------------------------------------------------------- process */

static void not_started(void *out, const ey_type *res, ey_v program, ey_v why) {
	ey_dup(program);
	ey_tmp e;
	ey_variant_make(res->variants[1].fields[0], e.b, 0, (const void *const[]){&program, &why});
	ey_err(res, out, e.b);
}

#ifndef _WIN32
/* valid_text is s with each run of bytes that aren't UTF-8 made U+FFFD */
static ey_v valid_text(ey_buf *b) {
	ey_buf out = {0};
	bool bad = false;
	for (int64_t i = 0; i < b->len;) {
		uint32_t r;
		int64_t k = ey_utf8_decode(b->b + i, b->len - i, &r);
		if (r == 0xfffd && k == 1 && (unsigned char)b->b[i] >= 0x80) {
			if (!bad) ey_buf_rune(&out, 0xfffd);
			bad = true;
		} else {
			ey_buf_add(&out, b->b + i, k);
			bad = false;
		}
		i += k;
	}
	free(b->b);
	return ey_buf_take(&out);
}
#endif

/* dir is an Option(String) of type dirt */
void eyb_process_run(ey_v program, ey_v args, ey_v input, const void *dir, const ey_type *dirt, void *out, const ey_type *res) {
#ifdef _WIN32
	(void)args, (void)input, (void)dir, (void)dirt;
	not_started(out, res, program, ey_str_c("the C back end doesn't run programs on Windows yet"));
#else
	ey_list *al = EY_LIST(args);
	char **argv = calloc((size_t)al->len + 2, sizeof(char *));
	argv[0] = EY_STR(program)->b;
	for (int64_t i = 0; i < al->len; i++) argv[i + 1] = EY_STR(((ey_v *)al->items)[i])->b;
	char *cwd = NULL;
	if (ey_variant_of(dirt, dir) == 0)
		cwd = path(*(ey_v *)ey_variant_field(dirt, dir, 0, 0));
	else if (work_dir())
		cwd = strdup(work_dir());
	int in[2], outp[2], errp[2], status[2];
	if (pipe(in) || pipe(outp) || pipe(errp) || pipe(status)) {
		free(argv);
		free(cwd);
		not_started(out, res, program, reason(errno));
		return;
	}
	fcntl(status[1], F_SETFD, FD_CLOEXEC);
	pid_t pid = fork();
	if (pid == 0) {
		dup2(in[0], 0);
		dup2(outp[1], 1);
		dup2(errp[1], 2);
		close(in[1]);
		close(outp[0]);
		close(errp[0]);
		close(status[0]);
		int err = 0;
		if (cwd && chdir(cwd) != 0) err = errno;
		if (!err) {
			execvp(argv[0], argv);
			err = errno;
		}
		ssize_t w = write(status[1], &err, sizeof err);
		(void)w;
		_exit(127);
	}
	free(argv);
	free(cwd);
	close(in[0]);
	close(outp[1]);
	close(errp[1]);
	close(status[1]);
	if (pid < 0) {
		not_started(out, res, program, reason(errno));
		return;
	}
	int err = 0;
	if (read(status[0], &err, sizeof err) == (ssize_t)sizeof err) {
		close(status[0]);
		close(in[1]);
		close(outp[0]);
		close(errp[0]);
		waitpid(pid, NULL, 0);
		if (err == ENOENT) {
			ey_buf b = {0};
			ey_buf_printf(&b, "exec: \"%s\": executable file not found in $PATH", EY_STR(program)->b);
			not_started(out, res, program, ey_buf_take(&b));
			return;
		}
		not_started(out, res, program, reason(err));
		return;
	}
	close(status[0]);
	/* write the input and read both outputs as they come */
	void (*old)(int) = signal(SIGPIPE, SIG_IGN);
	ey_str *text = EY_STR(input);
	int64_t sent = 0;
	if (text->len == 0) close(in[1]);
	ey_buf ob = {0}, eb = {0};
	bool open_in = text->len > 0, open_out = true, open_err = true;
	while (open_in || open_out || open_err) {
		struct pollfd fds[3];
		int nf = 0;
		if (open_in) fds[nf++] = (struct pollfd){in[1], POLLOUT, 0};
		if (open_out) fds[nf++] = (struct pollfd){outp[0], POLLIN, 0};
		if (open_err) fds[nf++] = (struct pollfd){errp[0], POLLIN, 0};
		if (poll(fds, (nfds_t)nf, -1) < 0) {
			if (errno == EINTR) continue;
			break;
		}
		for (int i = 0; i < nf; i++) {
			if (!fds[i].revents) continue;
			char chunk[65536];
			if (fds[i].fd == in[1]) {
				ssize_t w = write(in[1], text->b + sent, (size_t)(text->len - sent));
				if (w > 0) sent += w;
				if (w <= 0 || sent == text->len) {
					close(in[1]);
					open_in = false;
				}
				continue;
			}
			ssize_t r = read(fds[i].fd, chunk, sizeof chunk);
			if (r <= 0) {
				close(fds[i].fd);
				if (fds[i].fd == outp[0]) open_out = false;
				else open_err = false;
				continue;
			}
			ey_buf_add(fds[i].fd == outp[0] ? &ob : &eb, chunk, r);
		}
	}
	signal(SIGPIPE, old);
	int st = 0;
	waitpid(pid, &st, 0);
	int code = WIFEXITED(st) ? WEXITSTATUS(st) : -1;
	const ey_type *output = res->variants[0].fields[0];
	/* Output's fields in their declared order: status, stdout, stderr */
	ey_v fields[3];
	for (uint32_t i = 0; i < 3; i++) {
		const char *name = output->names[i];
		if (!strcmp(name, "status"))
			fields[i] = EY_SMALL(code);
		else if (!strcmp(name, "stdout"))
			fields[i] = valid_text(&ob);
		else
			fields[i] = valid_text(&eb);
	}
	ey_tmp o;
	ey_tuple_make(output, o.b, (const void *const[]){&fields[0], &fields[1], &fields[2]});
	ey_ok(res, out, o.b);
#endif
}
