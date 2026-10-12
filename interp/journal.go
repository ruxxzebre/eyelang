package interp

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Journal is the store `eye run` uses: the in-memory store, with every
// commit appended to a file and synced to disk before anything outside the
// runner can see it.
//
//   - The record of a run that isn't ready, because it waits or has ended,
//     stays in the file and leaves memory: the journal keeps where the file
//     has it, and reads it back when the runtime asks for it (handles.go).
//   - Opening the journal replays it, keeping in memory only what a running
//     program needs there: entities, events, meta, the records of the runs
//     that are ready, and each run's summary, which the runtime's index
//     takes. A last line cut short by a crash is dropped, since the commit
//     it held was never acknowledged, and so is a line with the zeros a
//     crash leaves where a write never reached the disk, with every line
//     after it.
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
//   - The journal is rewritten as a snapshot holding the whole state when it
//     is opened, and whenever it grows past four times the live data (and
//     past 1 MiB), so it grows with the state, not with the history. A
//     snapshot is several lines of at most about 1 MiB each, so that neither
//     writing nor reading it holds the whole state in memory.
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

	// out holds where the file has each run's record that isn't in memory.
	out paged[outRecord]
	// rf is the journal, open for reading those records.
	rf *os.File
	// last is the record read last, which the commit that wakes the run
	// writes the steps its trail gained against.
	last struct {
		id   int64
		data []byte
	}
	// loads counts the records read from the file since it opened.
	loads int64
	// sums holds the runs' summaries replay read, until the index takes them.
	sums map[int64]runSummary

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

// outRecord is where the journal has a run's record: n bytes at at. n is 0
// for a run the journal doesn't hold, and heldInMemory for one whose record is
// in memory.
type outRecord struct{ at, n int64 }

const heldInMemory = -1

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
	// Read says it was a run's record the store couldn't read back, not a
	// commit; it too is tried again later.
	Read bool
}

func (e *CommitError) Error() string {
	verb := "write"
	if e.Read {
		verb = "read"
	}
	if strings.HasPrefix(e.Path, "postgres ") {
		// A reconnect's error already names the store.
		return fmt.Sprintf("can't %s the store in %s: %s", verb, e.Path, strings.TrimPrefix(e.Err.Error(), e.Path+": "))
	}
	return fmt.Sprintf("can't %s the journal %s: %v", verb, e.Path, e.Err)
}

func (e *CommitError) Unwrap() error { return e.Err }

const (
	reserveSize    = 4 << 20 // bytes kept free for compaction
	compactAtLeast = 1 << 20 // the journal is compacted only past this size
	compactRatio   = 4       // and past this many times its live data
	snapshotLine   = 1 << 20 // bytes a line of a snapshot holds, about
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
	r, err := replayJournal(path, false)
	if err != nil {
		lock.Close()
		return nil, err
	}
	j := &Journal{memory: r.mem, path: path, lock: lock, out: r.out, rf: r.file, sums: r.sums}
	if err := j.compact(); err != nil {
		if j.rf != nil {
			j.rf.Close()
		}
		lock.Close()
		return nil, err
	}
	j.reserve()
	return j, nil
}

// ReadJournal reads the journal at path without changing it, for tools such
// as `eye runs` that look while the program runs. It holds every record in
// memory, as such a tool reads them all.
func ReadJournal(path string) (Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return replay(path)
}

// syncLater makes each commit from now on wait for Sync, which the runtime
// calls before the runner does anything outside.
func (j *Journal) syncLater() { j.later = true }

// takeSummaries gives the runs' summaries replay read, once.
func (j *Journal) takeSummaries() map[int64]runSummary {
	sums := j.sums
	j.sums = nil
	return sums
}

// Run returns a run's record: from memory if the run is ready, else from
// the file. A record the file can't give panics with a *CommitError.
func (j *Journal) Run(id int64) ([]byte, bool) {
	if o := j.out.at(id); o != nil && o.n > 0 {
		return j.load(id, *o), true
	}
	return j.memory.Run(id)
}

func (j *Journal) load(id int64, o outRecord) []byte {
	if j.last.data != nil && j.last.id == id {
		return j.last.data
	}
	data, err := readRecord(j.rf, o)
	if err != nil {
		panic(&CommitError{Path: j.path, Err: err, Read: true})
	}
	j.loads++
	j.last.id, j.last.data = id, data
	return data
}

func readRecord(f *os.File, o outRecord) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("a record at byte %d, and no file to read it from", o.at)
	}
	data := make([]byte, o.n)
	if _, err := f.ReadAt(data, o.at); err != nil {
		return nil, fmt.Errorf("reading a run's record at byte %d: %w", o.at, err)
	}
	if data[0] != '{' || data[len(data)-1] != '}' {
		return nil, fmt.Errorf("byte %d doesn't start a run's record", o.at)
	}
	return data, nil
}

// Runs lists the runs the journal holds, in memory or not, by number.
func (j *Journal) Runs() []int64 {
	ids := j.memory.Runs()
	j.out.each(func(id int64, o *outRecord) {
		if o.n > 0 {
			ids = append(ids, id)
		}
	})
	slices.Sort(ids)
	return ids
}

// snapshot is everything the journal holds, as one commit: with every run's
// record, read from the file if need be.
func (j *Journal) snapshot() *Commit {
	c := j.memory.snapshot()
	j.out.each(func(id int64, o *outRecord) {
		if o.n > 0 {
			c.Runs = append(c.Runs, RunWrite{ID: id, Data: j.load(id, *o)})
		}
	})
	slices.SortFunc(c.Runs, func(a, b RunWrite) int { return cmp.Compare(a.ID, b.ID) })
	return c
}

// Commit appends the commit to the journal, and only then applies it. It
// syncs the journal first, unless commits wait for Sync.
func (j *Journal) Commit(c *Commit) {
	if err := j.fault(); err != nil {
		panic(err)
	}
	keep := make([]bool, len(c.Runs)) // whether each run's record stays in memory
	for i, r := range c.Runs {
		sum, ok := summaryOfWrite(r)
		keep[i] = !ok || sum.inMemory()
	}
	line, spans, err := encodeLine(c, func(i int) ([]byte, bool) {
		if !keep[i] {
			return nil, false // written whole, to be read back from where it is
		}
		id := c.Runs[i].ID
		if o := j.out.at(id); o != nil && o.n > 0 {
			// Waking: the record was read just before, unless it wasn't.
			if j.last.data != nil && j.last.id == id {
				return j.last.data, true
			}
			return nil, false
		}
		return j.memory.Run(id)
	})
	if err != nil {
		panic(err)
	}
	at, err := j.append(line)
	if err != nil {
		// Space may be short: give back the reserve and compact, which
		// frees the space the history took. The commit itself still fails;
		// the runtime tries it again later.
		if j.release() {
			_ = j.compact()
		}
		panic(&CommitError{Path: j.path, Err: err})
	}
	j.apply(c, at, spans, keep)
	if j.size > compactAtLeast && j.size > compactRatio*j.live {
		_ = j.compact() // if it fails, the journal just stays longer
	}
}

// apply applies a commit written at byte at: each run's record goes to
// memory, or stays where the line has it.
func (j *Journal) apply(c *Commit, at int64, spans []span, keep []bool) {
	mem := *c
	mem.Runs = nil
	var gone []int64
	for i, r := range c.Runs {
		if j.last.id == r.ID {
			j.last.data = nil
		}
		o := j.out.put(r.ID)
		if !keep[i] && spans[i].start >= 0 {
			if o.n == heldInMemory {
				gone = append(gone, r.ID)
			}
			*o = outRecord{at: at + int64(spans[i].start), n: int64(spans[i].end - spans[i].start)}
			continue
		}
		*o = outRecord{n: heldInMemory}
		mem.Runs = append(mem.Runs, r)
	}
	if len(mem.Writes)+len(mem.Runs)+len(mem.Events)+len(mem.Meta) > 0 || mem.DropEvents > 0 {
		j.memory.Commit(&mem)
	}
	if len(gone) > 0 {
		j.forget(gone)
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
// the journal as it was. It returns the byte the line starts at.
func (j *Journal) append(line []byte) (int64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	at := j.size
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
		return 0, err
	}
	j.size += int64(len(line))
	j.written++
	if !j.later {
		j.synced = j.written
		j.syncs++
	}
	return at, nil
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

// compact rewrites the journal as a snapshot of the whole state: to a new
// file, synced and then renamed over the old one, so a crash leaves one or
// the other. Every commit is then synced. The records of the runs that
// aren't ready are copied from the old file to the new one a line at a
// time, and stay out of memory.
func (j *Journal) compact() error {
	j.syncing.Lock()
	defer j.syncing.Unlock()
	tmp := j.path + ".new"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(f, 1<<16)
	snap, err := writeSnapshot(w, j.memory.snapshot(), &j.out, func(o outRecord) ([]byte, error) { return readRecord(j.rf, o) })
	if err == nil {
		err = w.Flush()
	}
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
	rf, err := os.Open(j.path)
	if err != nil {
		nf.Close()
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.f != nil {
		j.f.Close()
	}
	if j.rf != nil {
		j.rf.Close()
	}
	j.f, j.rf, j.size, j.live, j.broken = nf, rf, snap.size, snap.size, nil
	j.out, j.last.data = snap.out, nil
	if len(snap.gone) > 0 {
		j.forget(snap.gone)
	}
	j.synced = j.written
	j.syncs++
	return nil
}

// snapshotWritten is what writing a snapshot gives: its size, where it has
// each run's record, and the runs whose records were in memory and now stay
// in the file.
type snapshotWritten struct {
	size int64
	out  paged[outRecord]
	gone []int64
}

// writeSnapshot writes the state as lines of a snapshot: what memory holds,
// mem, and the records out says the old file has, which read reads. A line
// holds about snapshotLine bytes of entities, events, meta and runs, so a
// small state is one line.
func writeSnapshot(w io.Writer, mem *Commit, out *paged[outRecord], read func(o outRecord) ([]byte, error)) (*snapshotWritten, error) {
	s := &snapshotWritten{}
	var c Commit
	var stays []bool // whether each of c's runs' records stays in memory
	filled := 0
	flush := func() error {
		data, spans, err := encodeLine(&c, nil)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		for i, r := range c.Runs {
			o := s.out.put(r.ID)
			if !stays[i] && spans[i].start >= 0 {
				*o = outRecord{at: s.size + int64(spans[i].start), n: int64(spans[i].end - spans[i].start)}
			} else {
				*o = outRecord{n: heldInMemory}
			}
		}
		s.size += int64(len(data))
		c, stays, filled = Commit{}, nil, 0
		return nil
	}
	add := func(n int) error {
		if filled += n; filled > snapshotLine {
			return flush()
		}
		return nil
	}
	for _, x := range mem.Writes {
		c.Writes = append(c.Writes, x)
		if err := add(len(x.Key) + len(x.State)*4/3 + 64); err != nil {
			return nil, err
		}
	}
	for _, x := range mem.Events {
		c.Events = append(c.Events, x)
		if err := add(len(x.Name) + len(x.Data)*4/3 + 32); err != nil {
			return nil, err
		}
	}
	for _, x := range mem.Meta {
		c.Meta = append(c.Meta, x)
		if err := add(len(x.Key) + len(x.Data)*4/3 + 32); err != nil {
			return nil, err
		}
	}
	// Runs, in the order they started: those in memory and those in the
	// file together. A record in memory stays there if its run is ready.
	ids := make([]int64, 0, len(mem.Runs))
	held := map[int64][]byte{}
	for _, r := range mem.Runs {
		held[r.ID] = r.Data
		ids = append(ids, r.ID)
	}
	out.each(func(id int64, o *outRecord) {
		if o.n > 0 {
			ids = append(ids, id)
		}
	})
	slices.Sort(ids)
	for _, id := range ids {
		data, inMem := held[id]
		stay := false
		if inMem {
			sum, ok := summaryOf(data)
			stay = !ok || sum.inMemory()
			if !stay {
				s.gone = append(s.gone, id)
			}
		} else {
			var err error
			if data, err = read(*out.at(id)); err != nil {
				return nil, err
			}
		}
		c.Runs = append(c.Runs, RunWrite{ID: id, Data: data})
		stays = append(stays, stay)
		if err := add(len(data) + 32); err != nil {
			return nil, err
		}
	}
	if filled > 0 || s.size == 0 {
		// An empty state is one empty commit, as it always was.
		if err := flush(); err != nil {
			return nil, err
		}
	}
	return s, nil
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
	if j.rf != nil {
		j.rf.Close()
	}
	if lerr := j.lock.Close(); err == nil {
		err = lerr
	}
	return err
}

// replayed is what replaying a journal gives: the state to keep in memory,
// where the file has each record that isn't in it, each run's summary, and
// the file, open for reading those records; nil if there is no journal.
type replayed struct {
	mem  *memStore
	out  paged[outRecord]
	sums map[int64]runSummary
	file *os.File
}

// replay reads a journal into an in-memory store, every record included. A
// missing journal is empty.
func replay(path string) (*memStore, error) {
	r, err := replayJournal(path, true)
	if err != nil {
		return nil, err
	}
	if r.file != nil {
		r.file.Close()
	}
	return r.mem, nil
}

// replayJournal reads a journal a line at a time. Unless all is set, the
// record of a run that isn't ready stays in the file: memory holds where it
// is, and the run's summary.
func replayJournal(path string, all bool) (*replayed, error) {
	r := &replayed{mem: newMemStore(), sums: map[int64]runSummary{}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	r.file = f
	fail := func(err error) (*replayed, error) {
		f.Close()
		return nil, err
	}
	names := map[string]string{} // job names and the like, kept once
	intern := func(s string) string {
		if n, ok := names[s]; ok {
			return n
		}
		names[s] = s
		return s
	}
	prev := func(id int64) ([]byte, bool) {
		if o := r.out.at(id); o != nil && o.n > 0 {
			data, err := readRecord(f, *o)
			return data, err == nil
		}
		return r.mem.Run(id)
	}
	in := bufio.NewReaderSize(f, 1<<16)
	var pos int64
	for n := 1; ; n++ {
		line, err := in.ReadBytes('\n')
		if err == io.EOF {
			// A last line without its line break was cut short: its commit
			// never counted.
			return r, nil
		}
		if err != nil {
			return fail(err)
		}
		start := pos
		pos += int64(len(line))
		c, raws, jerr := decodeRawLine(line, prev)
		if jerr != nil {
			if _, more := in.Peek(1); more == io.EOF {
				return r, nil // the last line is torn
			}
			if bytes.IndexByte(line, 0) >= 0 {
				// A crash left zeros where a write never reached the disk:
				// this commit, and the ones written after it, were never
				// synced, so nothing outside saw them.
				return r, nil
			}
			return fail(fmt.Errorf("%s:%d: %v", path, n, jerr))
		}
		runs := c.Runs
		c.Runs = nil
		var gone []int64
		from := 0 // records are found in the line in order
		for i, rw := range runs {
			o := r.out.put(rw.ID)
			sum, ok := summaryOf(rw.Data)
			if ok {
				sum.Job, sum.Status, sum.Resume = intern(sum.Job), intern(sum.Status), intern(sum.Resume)
				if !strings.HasPrefix(sum.WaitEvent, "run") {
					sum.WaitEvent = intern(sum.WaitEvent)
				}
				r.sums[rw.ID] = sum
			}
			if !all && ok && !sum.inMemory() && raws[i] != nil {
				if k := bytes.Index(line[from:], raws[i]); k >= 0 && bytes.HasSuffix(line[:from+k], []byte(`"Data":`)) {
					if o.n == heldInMemory {
						gone = append(gone, rw.ID)
					}
					*o = outRecord{at: start + int64(from+k), n: int64(len(raws[i]))}
					from += k + len(raws[i])
					continue
				}
			}
			*o = outRecord{n: heldInMemory}
			c.Runs = append(c.Runs, rw)
		}
		r.mem.Commit(c)
		r.mem.forget(gone)
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
	r, err := replayJournal(path, false)
	if err != nil {
		return JournalStats{}, err
	}
	if r.file != nil {
		defer r.file.Close()
	}
	snap, err := writeSnapshot(io.Discard, r.mem.snapshot(), &r.out, func(o outRecord) ([]byte, error) { return readRecord(r.file, o) })
	if err != nil {
		return JournalStats{}, err
	}
	return JournalStats{Size: info.Size(), Live: snap.size, Free: freeSpace(filepath.Dir(path))}, nil
}
