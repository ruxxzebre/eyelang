package interp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Journal is the store `eye run` uses: the in-memory store, with every
// commit appended to a file and synced to disk before it counts.
//
//   - Opening the journal replays it. A last line cut short by a crash is
//     dropped, since the commit it held was never acknowledged.
//   - A commit that can't be written, because the disk is full or for any
//     other reason, changes nothing: the journal is cut back to where it was,
//     memory isn't touched, and Commit panics with a *CommitError, which the
//     runtime turns into a retry later.
//   - The journal is rewritten as one commit holding the whole state when it
//     is opened, and whenever it grows past four times the live data (and
//     past 1 MiB), so it grows with the state, not with the history.
//   - Beside it, a 4 MiB reserve file holds space that is released when a
//     write fails, so that compaction, which frees space, can still finish.
type Journal struct {
	memory
	path   string
	f      journalFile
	lock   *os.File
	size   int64 // bytes in the journal
	live   int64 // bytes of its last snapshot: the live data then
	broken error // set when a failed write couldn't be undone
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

// Commit appends the commit to the journal, syncs it, and only then applies
// it.
func (j *Journal) Commit(c *Commit) {
	if j.broken != nil {
		panic(&CommitError{Path: j.path, Err: j.broken})
	}
	line, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	line = append(line, '\n')
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

// append writes line and syncs it, or leaves the journal as it was.
func (j *Journal) append(line []byte) error {
	_, err := j.f.Write(line)
	if err == nil {
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
	return nil
}

// compact rewrites the journal as one commit holding the whole state: to a
// new file, synced and then renamed over the old one, so a crash leaves one
// or the other.
func (j *Journal) compact() error {
	line, err := json.Marshal(j.snapshot())
	if err != nil {
		return err
	}
	line = append(line, '\n')
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
	if j.f != nil {
		j.f.Close()
	}
	j.f, j.size, j.live, j.broken = nf, int64(len(line)), int64(len(line)), nil
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

// Close closes the journal and releases it for another process.
func (j *Journal) Close() error {
	err := j.f.Close()
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
		var c Commit
		if jerr := json.Unmarshal(line, &c); jerr != nil {
			if _, more := r.Peek(1); more == io.EOF {
				return mem, nil // the last line is torn
			}
			return nil, fmt.Errorf("%s:%d: %v", path, n, jerr)
		}
		mem.Commit(&c)
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
	line, err := json.Marshal(mem.snapshot())
	if err != nil {
		return JournalStats{}, err
	}
	return JournalStats{Size: info.Size(), Live: int64(len(line)) + 1, Free: freeSpace(filepath.Dir(path))}, nil
}
