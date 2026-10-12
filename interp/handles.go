package interp

import (
	"bytes"
	"container/heap"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// A waiting run's record leaves memory (SPECS.md §15). The runtime keeps a
// handle of each run in memory, a few dozen bytes: its job, its status, when
// it wakes and what event wakes it, what pruning and the startup checks
// count. The record itself, with the run's variables, steps, arguments and
// trail, is in memory only while the run is ready: the journal and the
// Postgres store keep the record of a run that waits or has ended on disk,
// and read it back when the run wakes or something asks for it.
//
// A store works a run's summary out of its record whenever the record is
// written (summaryOf). The index below keeps the summary as a handle, and
// the stores that keep records out of memory keep the summaries they read
// when they opened until the index takes them, so that a restart reads no
// record but those of the runs that are ready.

// runSummary is what a store knows of a run without reading its record.
// The Postgres store keeps it beside the record, as JSON.
type runSummary struct {
	Job        string
	Status     string
	WakeAt     int64  `json:",omitempty"`
	WaitEvent  string `json:",omitempty"`
	Key        string `json:",omitempty"`
	Group      string `json:",omitempty"`
	RateGroup  string `json:",omitempty"`
	Begun      int64  `json:",omitempty"`
	Ended      int64  `json:",omitempty"`
	Summarized bool   `json:",omitempty"`
	// Sees is the number of the event after which the run may still be
	// waiting for events (jobRun.sees), and Resume where its checkpoint
	// resumes (resumePoint); the record holds neither as such.
	Sees   int64  `json:",omitempty"`
	Resume string `json:",omitempty"`
}

// inMemory says whether a store keeps the record of a run with this summary
// in memory: only while the run is ready to run.
func (s *runSummary) inMemory() bool { return s.Status == "ready" }

// summaryOf works out a run's summary from its record.
func summaryOf(data []byte) (runSummary, bool) {
	var r struct {
		runSummary
		Since int64
		Log   []struct {
			Event      string
			Since, Seq int64
		}
		// Checkpoint is read for resumePoint only: the variables' values
		// are counted, not decoded.
		Checkpoint *struct {
			Path []struct {
				Kind byte
				I    int64
			}
			Slots elements
			Code  string
		}
	}
	if json.Unmarshal(data, &r) != nil {
		return runSummary{}, false
	}
	s := r.runSummary
	s.Sees = r.Since
	for _, step := range r.Log {
		if step.Event != "" {
			s.Sees = step.Since
			break
		}
		s.Sees = step.Seq
	}
	if ck := r.Checkpoint; ck != nil {
		point := &checkpoint{Slots: make([][]byte, ck.Slots), Code: ck.Code}
		for _, p := range ck.Path {
			point.Path = append(point.Path, pathElem{Kind: p.Kind, I: p.I})
		}
		s.Resume = resumePoint(point)
	}
	return s, true
}

// elements is the number of elements of a JSON array, counted without
// decoding them.
type elements int

func (n *elements) UnmarshalJSON(data []byte) error {
	*n = 0
	data = bytes.TrimSpace(data)
	if len(data) < 2 || data[0] != '[' || len(bytes.TrimSpace(data[1:len(data)-1])) == 0 {
		return nil // null, or no elements
	}
	depth, inString := 0, false
	for i := 0; i < len(data); i++ {
		switch c := data[i]; {
		case inString && c == '\\':
			i++ // what follows is escaped
		case inString:
			inString = c != '"'
		case c == '"':
			inString = true
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		case c == ',' && depth == 1:
			*n++
		}
	}
	*n++
	return nil
}

// summaryOfRun is summaryOf for a run already decoded.
func summaryOfRun(run *jobRun) runSummary {
	s := runSummary{Job: run.Job, Status: run.Status, WakeAt: run.WakeAt, WaitEvent: run.WaitEvent, Key: run.Key,
		Group: run.Group, RateGroup: run.RateGroup, Begun: run.Begun, Ended: run.Ended, Summarized: run.Summarized, Sees: run.sees()}
	if run.Checkpoint != nil {
		s.Resume = resumePoint(run.Checkpoint)
	}
	return s
}

// resumePoint is what resumable reads of a checkpoint, as text: how many
// variables the job keeps, the path to the statement (the index of each
// block statement, branch and arm on the way, but not which turn of a loop),
// and the statement's hash. Runs paused at one statement share it.
func resumePoint(ck *checkpoint) string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(ck.Slots)))
	b.WriteByte(' ')
	for _, p := range ck.Path {
		b.WriteByte(p.Kind)
		if p.Kind == 'b' || p.Kind == 'i' || p.Kind == 'm' {
			b.WriteString(strconv.FormatInt(p.I, 10))
		}
		b.WriteByte('.')
	}
	b.WriteByte(' ')
	b.WriteString(ck.Code)
	return b.String()
}

// resumePointCheckpoint is the checkpoint resumable needs to check a resume
// point; false if the text isn't one.
func resumePointCheckpoint(point string) (*checkpoint, bool) {
	slots, rest, ok1 := strings.Cut(point, " ")
	path, code, ok2 := strings.Cut(rest, " ")
	n, err := strconv.Atoi(slots)
	if !ok1 || !ok2 || err != nil || n < 0 {
		return nil, false
	}
	ck := &checkpoint{Slots: make([][]byte, n), Code: code}
	for _, p := range strings.Split(strings.TrimSuffix(path, "."), ".") {
		if p == "" {
			continue
		}
		e := pathElem{Kind: p[0]}
		if len(p) > 1 {
			if e.I, err = strconv.ParseInt(p[1:], 10, 64); err != nil {
				return nil, false
			}
		}
		ck.Path = append(ck.Path, e)
	}
	return ck, true
}

// ---------------------------------------------------------------- handles

// runStatus is a run's status in its handle.
type runStatus uint8

const (
	noRun runStatus = iota
	readyRun
	sleepingRun
	queuedRun
	doneRun
	failedRun
	otherRun // a status this runtime doesn't know
)

func statusOf(s string) runStatus {
	switch s {
	case "ready":
		return readyRun
	case "sleeping":
		return sleepingRun
	case "queued":
		return queuedRun
	case "done":
		return doneRun
	case "failed":
		return failedRun
	}
	return otherRun
}

func (s runStatus) finished() bool { return s == doneRun || s == failedRun }

// runHandle is what the runtime keeps in memory of a run.
type runHandle struct {
	at     int64  // while it sleeps, when it wakes; once it has ended, when it did
	sees   int64  // runSummary.Sees
	event  string // while it sleeps, the event it waits for (waitsFor)
	job    uint32 // the job's name, in names
	resume uint32 // where its checkpoint resumes, in names; 0 for none
	heap   int32  // its place in timers, plus 1; 0 when it isn't there
	status runStatus
	// summarized says it ended long ago and keeps only a summary.
	summarized bool
}

// pageSize is how many runs one page of a paged table holds.
const pageSize = 1024

// paged is a table by run number, in pages, since run numbers are dense:
// a run costs its entry and nothing for a map.
type paged[T any] struct {
	pages []*[pageSize]T
}

// at returns id's entry, or nil if its page doesn't exist.
func (t *paged[T]) at(id int64) *T {
	p := id / pageSize
	if id < 0 || p >= int64(len(t.pages)) || t.pages[p] == nil {
		return nil
	}
	return &t.pages[p][id%pageSize]
}

// put returns id's entry, making its page if need be.
func (t *paged[T]) put(id int64) *T {
	if id < 0 {
		panic(fmt.Sprintf("run number %d", id))
	}
	p := id / pageSize
	for int64(len(t.pages)) <= p {
		t.pages = append(t.pages, nil)
	}
	if t.pages[p] == nil {
		t.pages[p] = new([pageSize]T)
	}
	return &t.pages[p][id%pageSize]
}

// each calls fn on every entry of every page, in order.
func (t *paged[T]) each(fn func(id int64, e *T)) {
	for p, page := range t.pages {
		if page == nil {
			continue
		}
		for i := range page {
			fn(int64(p)*pageSize+int64(i), &page[i])
		}
	}
}

// ---------------------------------------------------------------- the index

// indexed is a store with the handles of its runs, which find the run to
// wake next, or the runs an event wakes, without reading a record: the cost
// is the same with ten waiting runs as with ten thousand.
type indexed struct {
	Store
	handles paged[runHandle]
	// names holds the job names and resume points handles refer to, each
	// once; names[0] is "".
	names  []string
	nameID map[string]uint32
	// timers holds the sleeping runs' ids as a min-heap on when they wake,
	// the earlier started first among runs that wake at the same time.
	timers timerHeap
	// byEvent holds the sleeping runs by the event that wakes them (waitKeys).
	byEvent map[string]map[int64]struct{}
	// open holds the unfinished runs of jobs with @concurrency: those that
	// have begun (ready or sleeping), and those queued for a free place.
	open map[int64]openRun
	// keys finds a run by the key it was started with: job/key → run.
	keys map[string]int64
	// last is the highest run number the store holds.
	last int64
	// concurrent and limited say whether a job has @concurrency or @limit;
	// nil counts every job.
	concurrent, limited func(job string) bool
	// begun holds, from when the store was opened, when the runs of each
	// job and @limit group with @limit began, for the runtime's restore.
	begun map[string][]int64
}

type openRun struct {
	job    uint32
	group  string // the value of the job's @concurrency by: parameter
	queued bool
	begun  bool // past @concurrency and @limit
}

// summarizing is a store that keeps run records out of memory. It gives
// the summaries of its runs, read when it opened, once: to the index of the
// runtime that runs it. A second index reads the records instead.
type summarizing interface {
	takeSummaries() map[int64]runSummary
}

// indexStore indexes the runs of s; concurrent and limited say which jobs
// have @concurrency and @limit, or nil for every job.
func indexStore(s Store, concurrent, limited func(job string) bool) *indexed {
	ix := &indexed{Store: s, names: []string{""}, nameID: map[string]uint32{"": 0},
		byEvent: map[string]map[int64]struct{}{}, open: map[int64]openRun{}, keys: map[string]int64{},
		concurrent: concurrent, limited: limited, begun: map[string][]int64{}}
	ix.timers.ix = ix
	add := func(id int64, sum *runSummary) {
		ix.note(id, sum)
		if sum.Begun > 0 && (limited == nil || limited(sum.Job)) {
			key := sum.Job + "/" + sum.RateGroup
			ix.begun[key] = append(ix.begun[key], sum.Begun)
		}
	}
	if ss, ok := s.(summarizing); ok {
		if sums := ss.takeSummaries(); sums != nil {
			ids := make([]int64, 0, len(sums))
			for id := range sums {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			for _, id := range ids {
				sum := sums[id]
				add(id, &sum)
			}
			return ix
		}
	}
	for _, id := range s.Runs() {
		data, _ := s.Run(id)
		if sum, ok := summaryOf(data); ok {
			add(id, &sum)
		}
	}
	return ix
}

// Commit works out the summary of each run the commit writes, which the
// store underneath may use, commits, and then updates the handles.
func (ix *indexed) Commit(c *Commit) {
	for i := range c.Runs {
		r := &c.Runs[i]
		if r.summary == nil {
			if sum, ok := summaryOf(r.Data); ok {
				r.summary = &sum
			}
		}
	}
	ix.Store.Commit(c)
	for _, r := range c.Runs {
		if r.summary != nil {
			ix.note(r.ID, r.summary)
		}
	}
}

// name returns the number of a job name or resume point in names.
func (ix *indexed) name(s string) uint32 {
	if n, ok := ix.nameID[s]; ok {
		return n
	}
	n := uint32(len(ix.names))
	ix.names = append(ix.names, s)
	ix.nameID[s] = n
	return n
}

// handle returns the handle of run id, or nil if the store doesn't hold it.
func (ix *indexed) handle(id int64) *runHandle {
	h := ix.handles.at(id)
	if h == nil || h.status == noRun {
		return nil
	}
	return h
}

// jobOf is the name of the job of the run with handle h.
func (ix *indexed) jobOf(h *runHandle) string { return ix.names[h.job] }

// each calls fn on the handle of every run, in the order they started.
func (ix *indexed) each(fn func(id int64, h *runHandle)) {
	ix.handles.each(func(id int64, h *runHandle) {
		if h.status != noRun {
			fn(id, h)
		}
	})
}

// note updates run id's handle from its summary.
func (ix *indexed) note(id int64, s *runSummary) {
	h := ix.handles.put(id)
	status := statusOf(s.Status)
	h.status, h.job, h.sees, h.summarized = status, ix.name(s.Job), s.Sees, s.Summarized
	h.resume = 0
	if s.Resume != "" {
		h.resume = ix.name(s.Resume)
	}
	if id > ix.last {
		ix.last = id
	}
	if status == sleepingRun {
		ix.wait(id, h, s.WakeAt, s.WaitEvent)
	} else {
		ix.unwait(id, h)
		h.at = 0
		if status.finished() {
			h.at = s.Ended
		}
	}
	if !status.finished() && (status == queuedRun || ix.concurrent == nil || ix.concurrent(s.Job)) {
		ix.open[id] = openRun{job: h.job, group: s.Group, queued: status == queuedRun, begun: s.Begun > 0}
	} else {
		delete(ix.open, id)
	}
	if s.Key != "" {
		ix.keys[s.Job+"/"+s.Key] = id
	}
}

// running counts a job's unfinished runs in a group that have begun.
func (ix *indexed) running(job, group string) int {
	n := 0
	for _, r := range ix.open {
		if ix.names[r.job] == job && r.group == group && r.begun {
			n++
		}
	}
	return n
}

// firstQueued returns the earliest run of a job's group that waits for a
// place, if any.
func (ix *indexed) firstQueued(job, group string) (int64, bool) {
	first := int64(-1)
	for id, r := range ix.open {
		if ix.names[r.job] == job && r.group == group && r.queued && (first < 0 || id < first) {
			first = id
		}
	}
	return first, first >= 0
}

// wait records that a run sleeps until wakeAt or event, or changes what it
// waits for.
func (ix *indexed) wait(id int64, h *runHandle, wakeAt int64, event string) {
	if !strings.HasPrefix(event, "run") {
		// An event's name is the program's, and many runs share it; a run's
		// isn't.
		event = ix.names[ix.name(event)]
	}
	if h.heap == 0 {
		h.at, h.event = wakeAt, event
		heap.Push(&ix.timers, id)
		ix.addEvent(id, event)
		return
	}
	if h.at != wakeAt {
		h.at = wakeAt
		heap.Fix(&ix.timers, int(h.heap-1))
	}
	if h.event != event {
		ix.dropEvent(id, h.event)
		h.event = event
		ix.addEvent(id, event)
	}
}

// unwait forgets that a run sleeps.
func (ix *indexed) unwait(id int64, h *runHandle) {
	if h.heap == 0 {
		return
	}
	heap.Remove(&ix.timers, int(h.heap-1))
	ix.dropEvent(id, h.event)
	h.event = ""
}

func (ix *indexed) addEvent(id int64, wait string) {
	for _, k := range waitKeys(wait) {
		set := ix.byEvent[k]
		if set == nil {
			set = map[int64]struct{}{}
			ix.byEvent[k] = set
		}
		set[id] = struct{}{}
	}
}

func (ix *indexed) dropEvent(id int64, wait string) {
	for _, k := range waitKeys(wait) {
		delete(ix.byEvent[k], id)
		if len(ix.byEvent[k]) == 0 {
			delete(ix.byEvent, k)
		}
	}
}

// waitKeys are the names of the events that wake a run waiting for wait
// (waitsFor): the name itself, or, for runs/<id>,<id>…, each run/<id>. A
// run that waits only for its time has none.
func waitKeys(wait string) []string {
	if wait == "" {
		return nil
	}
	keys := []string{wait}
	if ids, ok := strings.CutPrefix(wait, "runs/"); ok {
		for _, s := range strings.Split(ids, ",") {
			keys = append(keys, "run/"+s)
		}
	}
	return keys
}

// firstToWake returns the sleeping run that wakes first, and when; of those
// that wake at the same time, the one started first.
func (ix *indexed) firstToWake() (id, wakeAt int64, ok bool) {
	if len(ix.timers.ids) == 0 {
		return 0, 0, false
	}
	id = ix.timers.ids[0]
	return id, ix.handles.at(id).at, true
}

// waiting counts the sleeping runs.
func (ix *indexed) waiting() int { return len(ix.timers.ids) }

// waitingFor lists the runs that an event named name wakes, in the order
// they were started.
func (ix *indexed) waitingFor(name string) []int64 {
	set := ix.byEvent[name]
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// timerHeap is the sleeping runs' ids, ordered by when they wake and then by
// id, which is the order they were started. Each run's handle has its place,
// so that a run whose wait changes or ends moves in O(log n).
type timerHeap struct {
	ix  *indexed
	ids []int64
}

func (h *timerHeap) Len() int { return len(h.ids) }

func (h *timerHeap) Less(i, j int) bool {
	a, b := h.ix.handles.at(h.ids[i]).at, h.ix.handles.at(h.ids[j]).at
	if a != b {
		return a < b
	}
	return h.ids[i] < h.ids[j]
}

func (h *timerHeap) Swap(i, j int) {
	h.ids[i], h.ids[j] = h.ids[j], h.ids[i]
	h.ix.handles.at(h.ids[i]).heap = int32(i + 1)
	h.ix.handles.at(h.ids[j]).heap = int32(j + 1)
}

func (h *timerHeap) Push(x any) {
	id := x.(int64)
	h.ix.handles.at(id).heap = int32(len(h.ids) + 1)
	h.ids = append(h.ids, id)
}

func (h *timerHeap) Pop() any {
	n := len(h.ids) - 1
	id := h.ids[n]
	h.ids = h.ids[:n]
	h.ix.handles.at(id).heap = 0
	return id
}
