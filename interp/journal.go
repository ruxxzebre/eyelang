package interp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Journal is the store `eye run` uses: the in-memory store, with every
// commit appended to a file and synced to disk before anything outside the
// runner can see it.
//
//   - Opening the journal replays it. A last line cut short by a crash is
//     dropped, since the commit it held was never acknowledged, and so is a
//     line with the zeros a crash leaves where a write never reached the
//     disk, with every line after it.
//   - A commit that can't be written, because the disk is full or for any
//     other reason, changes nothing: the journal is cut back to where it was,
//     memory isn't touched, and Commit panics with a *CommitError, which the
//     runtime turns into a retry later.
//   - Commits are synced in groups. Each commit syncs itself, unless the
//     runtime has asked commits to wait for Sync (syncLater), which it calls
//     before the runner does anything outside: calls a service, runs a
//     program, writes a file, prints, answers a request, or waits. One fsync
//     then covers every commit written since the last, and callers that sync
//     at the same time, such as requests answered together, share one. A
//     sync that fails leaves memory ahead of the disk, so the journal takes
//     no more commits until the program starts again.
//   - The journal is rewritten as one commit holding the whole state when it
//     is opened, and whenever it grows past four times the live data (and
//     past 1 MiB), so it grows with the state, not with the history.
//   - Beside it, a 4 MiB reserve file holds space that is released when a
//     write fails, so that compaction, which frees space, can still finish.
//
// journal_line.go says how a commit is written.
type Journal struct {
	memory
	path  string
	lock  *os.File
	live  int64 // bytes of its last snapshot: the live data then
	later bool  // commits wait for Sync instead of syncing themselves

	// mu guards what follows, since Sync may run without the interpreter's
	// lock; syncing lets one fsync run at a time.
	mu      sync.Mutex
	syncing sync.Mutex
	f       journalFile
	size    int64 // bytes in the journal
	written int64 // commits written since the journal was opened
	synced  int64 // how many of them are synced
	syncs   int64 // fsyncs of the journal, a compaction's included
	broken  error // set when a failed write couldn't be undone, or a sync failed
}

// journalFile is what the journal writes to: an *os.File, or in tests one
// that fails.
type journalFile interface {
	io.Writer
	Sync() error
	Truncate(size int64) error
	Close() error
}

// CommitError is a commit the store couldn't save. Nothing in it took
// effect.
type CommitError struct {
	Path string // the journal's path, or the database the store is in
	Err  error
	// Fatal says the store can take no more commits until the program
	// starts again: what it holds in memory may no longer match what it
	// saved.
	Fatal bool
}

func (e *CommitError) Error() string {
	if strings.HasPrefix(e.Path, "postgres ") {
		// A reconnect's error already names the store.
		return fmt.Sprintf("can't write the store in %s: %s", e.Path, strings.TrimPrefix(e.Err.Error(), e.Path+": "))
	}
	return fmt.Sprintf("can't write the journal %s: %v", e.Path, e.Err)
}

func (e *CommitError) Unwrap() error { return e.Err }

const (
	reserveSize    = 4 << 20 // bytes kept free for compaction
	compactAtLeast = 1 << 20 // the journal is compacted only past this size
	compactRatio   = 4       // and past this many times its live data
)

// OpenJournal opens the journal at path for a running program, creating it
// if need be. Only one process may hold a journal open for writing.
func OpenJournal(path string) (*Journal, error) {
	return openJournal(path, false)
}

// OpenFreshJournal opens an empty journal at path, as `eye run --fresh`
// does: once it holds the journal's lock, it moves the journal there, if
// any, to path.old, replacing an older one.
func OpenFreshJournal(path string) (*Journal, error) {
	return openJournal(path, true)
}

func openJournal(path string, fresh bool) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(lock); err != nil {
		lock.Close()
		return nil, fmt.Errorf("%s is in use by another eye run", path)
	}
	if fresh {
		if err := os.Rename(path, path+".old"); err != nil && !os.IsNotExist(err) {
			lock.Close()
			return nil, err
		}
	}
	mem, err := replay(path)
	if err != nil {
		lock.Close()
		return nil, err
	}
	j := &Journal{memory: mem, path: path, lock: lock}
	if err := j.compact(); err != nil {
		lock.Close()
		return nil, err
	}
	j.reserve()
	return j, nil
}

// ReadJournal reads the journal at path without changing it, for tools such
// as `eye runs` that look while the program runs.
func ReadJournal(path string) (Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return replay(path)
}

// syncLater makes each commit from now on wait for Sync, which the runtime
// calls before the runner does anything outside.
func (j *Journal) syncLater() { j.later = true }

// Commit appends the commit to the journal, and only then applies it. It
// syncs the journal first, unless commits wait for Sync.
func (j *Journal) Commit(c *Commit) {
	if err := j.fault(); err != nil {
		panic(err)
	}
	line, err := encodeLine(c, j.Run) // the records before c, from memory
	if err != nil {
		panic(err)
	}
	if err := j.append(line); err != nil {
		// Space may be short: give back the reserve and compact, which
		// frees the space the history took. The commit itself still fails;
		// the runtime tries it again later.
		if j.release() {
			_ = j.compact()
		}
		panic(&CommitError{Path: j.path, Err: err})
	}
	j.memory.Commit(c)
	if j.size > compactAtLeast && j.size > compactRatio*j.live {
		_ = j.compact() // if it fails, the journal just stays longer
	}
}

// fault is the error every commit fails with once the journal is broken.
func (j *Journal) fault() *CommitError {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.broken == nil {
		return nil
	}
	return &CommitError{Path: j.path, Err: j.broken, Fatal: true}
}

// append writes line, and syncs it unless commits wait for Sync, or leaves
// the journal as it was.
func (j *Journal) append(line []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, err := j.f.Write(line)
	if err == nil && !j.later {
		err = j.f.Sync()
	}
	if err != nil {
		if terr := j.f.Truncate(j.size); terr != nil {
			// The journal may now end in a part of this commit. It stays
			// the last line, which replay drops, as long as nothing more
			// is written.
			j.broken = fmt.Errorf("%v, and cutting the journal back failed: %v", err, terr)
		}
		return err
	}
	j.size += int64(len(line))
	j.written++
	if !j.later {
		j.synced = j.written
		j.syncs++
	}
	return nil
}

// Sync makes every commit written so far durable, with one fsync for all
// of them. It may be called without the interpreter's lock: callers that
// arrive while an fsync runs wait for it, and the next fsync covers them
// all. If the sync fails, the journal is broken: it returns a fatal
// *CommitError, and so does every commit after it.
func (j *Journal) Sync() error {
	j.mu.Lock()
	target := j.written
	j.mu.Unlock()
	j.syncing.Lock()
	defer j.syncing.Unlock()
	j.mu.Lock()
	if j.broken != nil || j.synced >= target {
		j.mu.Unlock()
		if err := j.fault(); err != nil {
			return err
		}
		return nil
	}
	f, upto := j.f, j.written
	j.mu.Unlock()
	err := f.Sync()
	j.mu.Lock()
	if err != nil {
		j.broken = fmt.Errorf("syncing it failed, so commits made since the last sync may be lost: %w", err)
	} else {
		j.synced = max(j.synced, upto)
		j.syncs++
	}
	j.mu.Unlock()
	if err := j.fault(); err != nil {
		return err
	}
	return nil
}

// compact rewrites the journal as one commit holding the whole state: to a
// new file, synced and then renamed over the old one, so a crash leaves one
// or the other. Every commit is then synced.
func (j *Journal) compact() error {
	j.syncing.Lock()
	defer j.syncing.Unlock()
	line, err := encodeLine(j.snapshot(), nil)
	if err != nil {
		return err
	}
	tmp := j.path + ".new"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = f.Write(line)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, j.path)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if dir, err := os.Open(filepath.Dir(j.path)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	nf, err := os.OpenFile(j.path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.f != nil {
		j.f.Close()
	}
	j.f, j.size, j.live, j.broken = nf, int64(len(line)), int64(len(line)), nil
	j.synced = j.written
	j.syncs++
	return nil
}

// reserve makes sure the reserve file is there, if there is room for it.
func (j *Journal) reserve() {
	path := j.path + ".reserve"
	if info, err := os.Stat(path); err == nil && info.Size() == reserveSize {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	_, err = f.Write(make([]byte, reserveSize))
	if err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		os.Remove(path)
	}
}

// release deletes the reserve file, and reports whether there was one.
func (j *Journal) release() bool {
	return os.Remove(j.path+".reserve") == nil
}

// Close syncs what the journal holds unsynced, closes it, and releases it
// for another process.
func (j *Journal) Close() error {
	err := j.Sync()
	j.syncing.Lock()
	defer j.syncing.Unlock()
	j.mu.Lock()
	defer j.mu.Unlock()
	if cerr := j.f.Close(); err == nil {
		err = cerr
	}
	if lerr := j.lock.Close(); err == nil {
		err = lerr
	}
	return err
}

// replay reads a journal into an in-memory store. A missing journal is
// empty.
func replay(path string) (*memStore, error) {
	mem := newMemStore()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return mem, nil
	}
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(bytes.NewReader(data))
	for n := 1; ; n++ {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			// A last line without its line break was cut short: its commit
			// never counted.
			return mem, nil
		}
		c, jerr := decodeLine(line, mem.Run)
		if jerr != nil {
			if _, more := r.Peek(1); more == io.EOF {
				return mem, nil // the last line is torn
			}
			if bytes.IndexByte(line, 0) >= 0 {
				// A crash left zeros where a write never reached the disk:
				// this commit, and the ones written after it, were never
				// synced, so nothing outside saw them.
				return mem, nil
			}
			return nil, fmt.Errorf("%s:%d: %v", path, n, jerr)
		}
		mem.Commit(c)
	}
}

// JournalStats is what `eye runs` says about a journal.
type JournalStats struct {
	Size int64 // bytes the journal takes on disk
	Live int64 // bytes its state would take as one snapshot
	Free int64 // bytes free on its disk; -1 if unknown
}

// StatJournal measures the journal at path.
func StatJournal(path string) (JournalStats, error) {
	info, err := os.Stat(path)
	if err != nil {
		return JournalStats{}, err
	}
	mem, err := replay(path)
	if err != nil {
		return JournalStats{}, err
	}
	line, err := encodeLine(mem.snapshot(), nil)
	if err != nil {
		return JournalStats{}, err
	}
	return JournalStats{Size: info.Size(), Live: int64(len(line)), Free: freeSpace(filepath.Dir(path))}, nil
}
