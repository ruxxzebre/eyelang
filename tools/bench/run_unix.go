//go:build unix

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"eyelang/cgen"
	"eyelang/internal/zig"
)

// Peak memory comes from a small C launcher, not from the rusage Go's
// os/exec reports. Go starts a child with vfork (CLONE_VM), so until the
// child calls exec it runs in the parent's address space, and Linux counts
// that address space's high-water mark in the child's ru_maxrss: every child
// of this tool would show at least this tool's own memory, about 15 MB.
// The launcher is a static C binary of a few hundred KB; it forks, execs the
// benchmark, and reports the child's rusage and wall time.
const launcherC = `#include <stdio.h>
#include <sys/resource.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

/* launcher REPORT PROGRAM ARGS...: runs PROGRAM and writes its wall time in
   ns and its peak resident memory (ru_maxrss) to REPORT. */
int main(int argc, char **argv) {
  if (argc < 3) {
    fputs("usage: launcher report program [arguments]\n", stderr);
    return 2;
  }
  struct timespec t0, t1;
  clock_gettime(CLOCK_MONOTONIC, &t0);
  pid_t pid = fork();
  if (pid < 0) {
    perror("fork");
    return 2;
  }
  if (pid == 0) {
    execv(argv[2], argv + 2);
    perror(argv[2]);
    _exit(127);
  }
  int status;
  struct rusage ru;
  if (wait4(pid, &status, 0, &ru) < 0) {
    perror("wait4");
    return 2;
  }
  clock_gettime(CLOCK_MONOTONIC, &t1);
  long long ns = (long long)(t1.tv_sec - t0.tv_sec) * 1000000000LL + (t1.tv_nsec - t0.tv_nsec);
  FILE *f = fopen(argv[1], "w");
  if (f == NULL) {
    perror(argv[1]);
    return 2;
  }
  fprintf(f, "%lld %ld\n", ns, (long)ru.ru_maxrss);
  fclose(f);
  if (WIFEXITED(status)) return WEXITSTATUS(status);
  return 128 + WTERMSIG(status);
}
`

// buildLauncher compiles the launcher with the pinned zig into work.
func buildLauncher(work string) (string, error) {
	exe, err := zig.Path(nil)
	if err != nil {
		return "", err
	}
	src := filepath.Join(work, "launcher.c")
	if err := os.WriteFile(src, []byte(launcherC), 0o644); err != nil {
		return "", err
	}
	out := filepath.Join(work, "launcher")
	args := []string{"cc", "-O2", "-o", out, src}
	if triple := cgen.Targets[cgen.HostTarget()]; triple != "" {
		args = append(args, "-target", triple)
	}
	if b, err := exec.Command(exe, args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("compiling the launcher: %v\n%s", err, b)
	}
	return out, nil
}

// runTimeout bounds one run of a binary.
const runTimeout = 5 * time.Minute

// runOnce runs a command through the launcher and gives its wall time in
// ms, its peak resident memory in KB and what it printed.
func runOnce(launcher string, argv []string, dir string) (float64, int64, string, error) {
	report, err := os.CreateTemp("", "eye-bench-report-")
	if err != nil {
		return 0, 0, "", err
	}
	report.Close()
	defer os.Remove(report.Name())
	cmd := exec.Command(launcher, append([]string{report.Name()}, argv...)...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		return 0, 0, "", err
	}
	// On a timeout, kill the benchmark as well as the launcher.
	timer := time.AfterFunc(runTimeout, func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	err = cmd.Wait()
	timer.Stop()
	if err != nil {
		return 0, 0, "", fmt.Errorf("%v\n%s", err, stderr.String())
	}
	b, err := os.ReadFile(report.Name())
	if err != nil {
		return 0, 0, "", err
	}
	var ns, rss int64
	if _, err := fmt.Sscan(string(b), &ns, &rss); err != nil {
		return 0, 0, "", fmt.Errorf("the launcher's report %q: %v", b, err)
	}
	if runtime.GOOS == "darwin" {
		rss /= 1024 // bytes there, KB on Linux
	}
	return float64(ns) / 1e6, rss, stdout.String(), nil
}
