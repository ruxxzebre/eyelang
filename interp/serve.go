package interp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

	"strings"
	"time"
	"unicode/utf8"

	"eyelang/ir"
)

// RunOptions say how Run runs a program.
type RunOptions struct {
	Args           []string
	Stdout, Stderr io.Writer
	// Store keeps entities, runs and events; nil keeps them in memory, so
	// nothing survives the process.
	Store Store
	// Inbox is a directory of answers to approvals, which `eye approve` and
	// `eye decline` write and the runner publishes; "" reads none.
	Inbox string
	// MemoryLimit is the memory runs may hold, in bytes, which @memory and
	// reserve! are admitted against; 0 is the limit of the cgroup the
	// process runs in, or the host's memory when that is lower or there
	// is none.
	MemoryLimit int64
}

// Run runs a program as `eye run` does. main runs first, if the program has
// one, every time the program starts. Then, if the program has jobs, Run
// keeps going on the system clock: sleeping runs wake when their time comes,
// schedules fire, and runs the store kept from before a restart carry on. It
// returns when nothing is left that can happen (no schedule, and no run that
// waits), or when ctx is done. The result is the exit status: 1 if main
// returned an Err or faulted, or a job's run faulted.
func Run(ctx context.Context, prog *ir.Program, opts RunOptions) (code int) {
	in := New(prog, opts.Stdout, opts.Stderr)
	in.rt.real = true
	in.rt.now = time.Now()
	in.rt.memLimit = opts.MemoryLimit
	memory := memoryText(in.rt.memLimit)
	if in.rt.memLimit == 0 {
		var from string
		in.rt.memLimit, from = runnerMemory("/", hostMemory())
		memory = memoryText(in.rt.memLimit)
		if in.rt.memLimit > 0 {
			memory += ", " + from
		}
	}
	if opts.Store != nil {
		in.rt.useStore(opts.Store)
	}
	stderr := opts.Stderr
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Fault:
				fmt.Fprintln(stderr, x.Error())
			case *CommitError:
				fmt.Fprintf(stderr, "eye run: %v\n", x)
			default:
				panic(r)
			}
			code = 1
		}
	}()
	if err := in.rt.checkVersions(); err != nil {
		fmt.Fprintf(stderr, "eye run: %v\n", err)
		return 1
	}
	in.rt.restore()
	runsBefore := in.rt.nextRun
	if hasJobs(prog) {
		program := ""
		if len(opts.Args) > 0 {
			program = opts.Args[0]
		}
		fmt.Fprintln(stderr, in.rt.startLine(program, opts.Store, memory))
	}
	if main := prog.Root.Main; main != nil {
		var argv []argVal
		if len(main.Params) > 0 {
			list := &List{}
			for _, a := range opts.Args {
				list.Elems = append(list.Elems, a)
			}
			argv = []argVal{{v: list}}
		}
		result, _, _ := in.callFunc(main, nil, nil, argv, main.Pos)
		if e, ok := isVariant(result, "Err"); ok {
			fmt.Fprintln(stderr, in.display(e.Vals[0])) // with its Show, as a Problem has
			code = 1
		}
	} else if !hasJobs(prog) {
		fmt.Fprintln(stderr, "the program has no main function and no jobs: add `pub fn main(args: List(String))`")
		return 1
	}
	reported := 0
	report := func() {
		for _, f := range in.rt.failures[reported:] {
			fmt.Fprintln(stderr, f.Error())
			code = 1
		}
		reported = len(in.rt.failures)
	}
	report()
	if hasJobs(prog) && in.rt.nextRun == runsBefore {
		in.rt.nothingToDo(stderr, opts.Store)
	}
	if hasJobs(prog) {
		if err := in.serve(ctx, report, stderr, opts.Inbox); err != nil {
			code = 1
		}
	}
	report()
	return code
}

func hasJobs(prog *ir.Program) bool {
	for _, m := range prog.Modules {
		if len(m.Jobs) > 0 {
			return true
		}
	}
	return false
}

// serve runs the program's jobs on the system clock until nothing is left
// that can happen, or ctx is done, and publishes the answers that reach
// inbox, if it has one. It returns early, with the error, if the store can
// take no more commits.
func (in *Interp) serve(ctx context.Context, report func(), stderr io.Writer, inbox string) error {
	rt := in.rt
	var pruned time.Time
	for {
		rt.now = time.Now()
		// A commit the store refuses (a full disk) changes nothing: what it
		// was part of is tried again after retryAfter.
		err := guard(func() {
			if rt.now.Sub(pruned) >= time.Hour {
				rt.prune()
				pruned = rt.now
			}
			if inbox != "" {
				rt.readInbox(inbox, func(msg string) { fmt.Fprintf(stderr, "eye run: %s\n", msg) })
			}
			rt.catchUp()
			report()
			for rt.wakeNext(rt.now) {
				report()
				rt.now = time.Now()
			}
		})
		if err == nil && rt.commitErr != nil {
			err = rt.commitErr
		}
		var wait time.Duration
		if ce, ok := err.(*CommitError); ok && ce.Fatal {
			// The store can't go on from what it holds in memory; a restart
			// reads it again.
			fmt.Fprintf(stderr, "eye run: %v\n", err)
			return err
		}
		if err != nil {
			fmt.Fprintf(stderr, "eye run: %v; trying again in %v\n", err, retryAfter)
			wait = retryAfter
		} else {
			run := rt.nextSleeper()
			s, firing := rt.nextFiring()
			switch {
			case run == nil && s == nil:
				return nil
			case run == nil:
				wait = time.Until(firing)
			case s == nil || run.WakeAt < firing.UnixNano():
				wait = time.Until(time.Unix(0, run.WakeAt))
			default:
				wait = time.Until(firing)
			}
			wait = min(wait, time.Until(pruned.Add(time.Hour)))
			if inbox != "" {
				wait = min(wait, inboxEvery)
			}
		}
		timer := time.NewTimer(max(wait, 0))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if err != nil {
			_ = guard(rt.retryBlocked)
		}
	}
}

// restore picks up what the store kept from an earlier run of the program:
// the events and the keys already seen, the commands whose reply was lost,
// the next run's number, which entities are in each view, and the runs that
// were running when the program stopped, which start again.
func (rt *runtime) restore() {
	rt.restoreReserved()
	if data, ok := rt.store.Meta("events/dropped"); ok {
		rt.eventBase, _ = strconv.ParseInt(string(data), 10, 64)
	}
	for _, e := range rt.store.Events() {
		rt.events = append(rt.events, event{name: e.Name, data: e.Data})
	}
	for _, k := range rt.store.MetaKeys("seen/") {
		rt.seen[strings.TrimPrefix(k, "seen/")] = true
	}
	for _, k := range rt.store.MetaKeys("uncertain/") {
		data, _ := rt.store.Meta(k)
		if cmd, ok := rt.decodeLost(data); ok {
			rt.uncertain[strings.TrimPrefix(k, "uncertain/")] = cmd
		}
	}
	for _, id := range rt.store.Runs() {
		if id >= rt.nextRun {
			rt.nextRun = id + 1
		}
		data, _ := rt.store.Run(id)
		run := decodeRun(data)
		if run.Status == "ready" {
			rt.queue = append(rt.queue, id)
		}
		if job := rt.in.jobs[run.Job]; job != nil && job.Limit != nil && run.Begun > 0 {
			key := run.Job + "/" + run.RateGroup
			rt.rate[key] = append(rt.rate[key], run.Begun)
		}
	}
	for _, starts := range rt.rate {
		sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	}
	rt.fillPlaces()
	for _, m := range rt.in.prog.Modules {
		for _, v := range sortedViews(m) {
			if v.Entity == nil {
				continue
			}
			rt.members[v] = map[string]bool{}
			for _, key := range rt.store.Keys(entityName(v.Entity)) {
				row, _ := rt.store.Load(key)
				rt.members[v][key] = rt.inView(v, snapshotOf(rt.decodeRow(v.Entity, row, v.Pos), row.Version))
			}
		}
	}
	rt.drain()
}

// lostCommand is how a command whose reply was lost is saved.
type lostCommand struct {
	Op    string            // module.Contract.op
	Args  [][]byte          `json:",omitempty"`
	Named map[string][]byte `json:",omitempty"`
}

// lost keeps a command whose reply was lost, so that reconcile can ask about
// it, even after a restart.
func (rt *runtime) lost(key string, cmd sentCommand) {
	saved := lostCommand{Op: opName(cmd.op), Named: map[string][]byte{}}
	for _, a := range cmd.args {
		saved.Args = append(saved.Args, rt.in.encode(a, cmd.op.Pos))
	}
	for name, v := range cmd.named {
		saved.Named[name] = rt.in.encode(v, cmd.op.Pos)
	}
	data, err := json.Marshal(saved)
	if err != nil {
		panic(err)
	}
	rt.store.Commit(&Commit{Meta: []MetaWrite{{Key: "uncertain/" + key, Data: data}}})
	rt.uncertain[key] = cmd
}

// settled forgets a lost command once reconcile knows its outcome. Until
// then it is never pruned.
func (rt *runtime) settled(key string) {
	if _, ok := rt.uncertain[key]; !ok {
		return
	}
	rt.store.Commit(&Commit{Meta: []MetaWrite{{Key: "uncertain/" + key}}})
	delete(rt.uncertain, key)
}

func opName(op *ir.Op) string {
	return op.Contract.Module.Path + "." + op.Contract.Name + "." + op.Name
}

func (rt *runtime) decodeLost(data []byte) (sentCommand, bool) {
	var saved lostCommand
	if json.Unmarshal(data, &saved) != nil {
		return sentCommand{}, false
	}
	for _, m := range rt.in.prog.Modules {
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				if opName(op) != saved.Op {
					continue
				}
				cmd := sentCommand{op: op, named: map[string]Value{}}
				for _, a := range saved.Args {
					cmd.args = append(cmd.args, rt.in.decode(a, op.Pos))
				}
				for name, v := range saved.Named {
					cmd.named[name] = rt.in.decode(v, op.Pos)
				}
				return cmd, true
			}
		}
	}
	return sentCommand{}, false
}

// ---------------------------------------------------------------- eye runs

// RunInfo describes a job run for `eye runs`.
type RunInfo struct {
	ID     int64      `json:"id"`
	Job    string     `json:"job"`
	Status string     `json:"status"` // ready, sleeping, done, failed
	Args   []string   `json:"args"`
	WakeAt *time.Time `json:"wakeAt,omitempty"` // while sleeping
	Waits  string     `json:"waits,omitempty"`  // what a sleeping run waits for
	// Approval is the text of the approval! a sleeping run waits for.
	Approval string `json:"approval,omitempty"`
	// Holds is the memory the run has reserved with reserve!.
	Holds  string     `json:"holds,omitempty"`
	Steps  []StepInfo `json:"steps"`
	Result string     `json:"result,omitempty"`
	Fault  string     `json:"fault,omitempty"`
	// StepCount is how many steps the run took, also once only a summary
	// is kept; Size is how many bytes the store keeps for it.
	StepCount  int        `json:"stepCount"`
	Size       int        `json:"size"`
	Summarized bool       `json:"summarized,omitempty"`
	Started    *time.Time `json:"started,omitempty"`
	Ended      *time.Time `json:"ended,omitempty"`
	// Spent is what the run's model calls spent, in all its steps.
	Spent *Spent `json:"spent,omitempty"`
}

// StepInfo is one durable step of a run.
type StepInfo struct {
	Call   string     `json:"call"`
	Value  string     `json:"value,omitempty"` // what the step returned
	Until  *time.Time `json:"until,omitempty"` // for a timer or a wait: when it ends
	Status string     `json:"status"`          // done, sleeping or waiting
	Spent  *Spent     `json:"spent,omitempty"` // what the step's model calls spent
}

// Spent is what model calls spent: their tokens, those of the prompts and
// of the replies, and their price in US dollars; and which models were
// asked, with the bytes of the prompts sent and of the replies. Steps
// recorded before 2026-10-11 have only the tokens in all.
type Spent struct {
	Tokens    int64   `json:"tokens"`
	TokensIn  int64   `json:"tokensIn,omitempty"`
	TokensOut int64   `json:"tokensOut,omitempty"`
	USD       float64 `json:"usd"`
	Model     string  `json:"model,omitempty"`
	Options   string  `json:"options,omitempty"` // how the calls were sent
	Sent      int64   `json:"sent,omitempty"`
	Received  int64   `json:"received,omitempty"`
}

func spentInfo(s spend) *Spent {
	if s == (spend{}) {
		return nil
	}
	return &Spent{Tokens: s.Tokens, TokensIn: s.In, TokensOut: s.Out, USD: s.USD, Model: s.Model, Options: s.Options, Sent: s.Sent, Received: s.Received}
}

// String shows what was spent: 12000 tokens in, 900 out, $0.0525.
func (s *Spent) String() string {
	text := fmt.Sprintf("%d tokens", s.Tokens)
	if s.TokensIn+s.TokensOut == s.Tokens {
		text = fmt.Sprintf("%d tokens in, %d out", s.TokensIn, s.TokensOut)
	}
	return text + fmt.Sprintf(", $%.4f", s.USD)
}

// Detail shows a step's model calls: the model, what they spent, and the
// bytes of prompt sent and of reply received.
//
//	gpt-5.6-terra: 12000 tokens in, 900 out, $0.0525; sent 41.2 KiB, received 3.0 KiB
func (s *Spent) Detail() string {
	text := s.String()
	if s.Model != "" {
		text = s.Model + ": " + text
	}
	if s.Sent+s.Received > 0 {
		text += "; sent " + bytesText(s.Sent) + ", received " + bytesText(s.Received)
	}
	return text
}

// Runs describes every run the store holds, oldest first.
func Runs(prog *ir.Program, store Store) []RunInfo {
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(store)
	held := reservations(store)
	var out []RunInfo
	for _, id := range store.Runs() {
		data, _ := store.Run(id)
		info := in.runInfo(decodeRun(data))
		info.Size = len(data)
		if n := held[id]; n > 0 {
			info.Holds = showSize(ir.Size(n))
		}
		out = append(out, info)
	}
	return out
}

func (in *Interp) runInfo(r *jobRun) (info RunInfo) {
	// A job of the program's own module is named without a module.
	info = RunInfo{ID: r.ID, Job: strings.TrimPrefix(r.Job, "."), Status: r.Status, Args: []string{}, Steps: []StepInfo{}, Fault: r.Fault}
	value := func(data []byte) (s string) {
		defer func() {
			if recover() != nil {
				s = "(can't be read by this version of the program)"
			}
		}()
		return showValue(in.decode(data, ir.Pos{}))
	}
	for _, a := range r.Args {
		info.Args = append(info.Args, value(a))
	}
	if r.Result != nil && r.Status == "done" {
		info.Result = value(r.Result)
	}
	when := func(ns int64) *time.Time {
		t := time.Unix(0, ns)
		return &t
	}
	info.StepCount, info.Summarized = len(r.Log), r.Summarized
	if r.Summarized {
		info.StepCount = r.Steps
	}
	if r.Started != 0 {
		info.Started = when(r.Started)
	}
	if r.Ended != 0 {
		info.Ended = when(r.Ended)
	}
	var total spend
	if r.Spent != nil {
		total = *r.Spent
	}
	for _, s := range r.Log {
		st := StepInfo{Call: s.Call, Status: "done", Spent: spentInfo(s.spent())}
		total.add(s.spent())
		switch {
		case s.Timer:
			st.Status, st.Until = "sleeping", when(s.WakeAt)
		case s.Event != "":
			st.Status, st.Until = "waiting", when(s.WakeAt)
		default:
			st.Value = value(s.Value)
			if s.WakeAt != 0 {
				st.Until = when(s.WakeAt)
			}
		}
		if st.Call == "" {
			st.Call = "step"
		}
		info.Steps = append(info.Steps, st)
	}
	info.Spent = spentInfo(total)
	if r.Status == "sleeping" {
		info.WakeAt = when(r.WakeAt)
		switch {
		case r.WaitEvent == approvalEvent:
			info.Waits = "a person's answer"
			if n := len(r.Log); n > 0 {
				info.Approval = strings.TrimPrefix(r.Log[n-1].Call, "approval: ")
				info.Waits += " to " + info.Approval
			}
		case r.WaitEvent == memoryEvent:
			info.Waits = "memory"
		case r.WaitEvent != "":
			info.Waits = "the event " + r.WaitEvent
		}
	}
	return info
}

// showValue shows a value as interpolation does, quoting strings.
func showValue(v Value) string {
	var b strings.Builder
	writeShow(&b, v, true)
	return b.String()
}

// brief shortens a long value to one line.
func brief(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if r := []rune(s); len(r) > 100 {
		return string(r[:99]) + "…"
	}
	return s
}

// FormatRuns writes runs as `eye runs` shows them: one line per run, with
// its steps below it when steps is set. Values are cut to one line of 100
// characters.
func FormatRuns(w io.Writer, runs []RunInfo, steps bool, now time.Time) {
	FormatRunsWith(w, runs, FormatOptions{Steps: steps}, now)
}

// FormatOptions say what FormatRunsWith shows.
type FormatOptions struct {
	Steps bool // each run's steps
	// Replies shows, below each model call's step, its whole reply, as
	// `eye runs --show-replies` does.
	Replies bool
}

// FormatRunsWith writes runs as FormatRuns does, with the options.
func FormatRunsWith(w io.Writer, runs []RunInfo, opts FormatOptions, now time.Time) {
	if len(runs) == 0 {
		fmt.Fprintln(w, "no runs")
		return
	}
	for _, r := range runs {
		args := make([]string, len(r.Args))
		for i, a := range r.Args {
			args[i] = brief(a)
		}
		line := fmt.Sprintf("#%d %s(%s)  %s", r.ID, r.Job, strings.Join(args, ", "), r.Status)
		switch r.Status {
		case "sleeping":
			if r.Waits != "" {
				line = strings.Replace(line, "sleeping", "waiting for "+r.Waits, 1)
			}
			if r.WakeAt.Sub(now) < time.Duration(forever)/2 { // a wait with no deadline has none to show
				line += " until " + clockTime(*r.WakeAt, now)
			}
			if r.Holds != "" {
				line += ", holding " + r.Holds
			}
		case "done":
			if r.Result != "" && r.Result != "()" {
				line += ": " + brief(r.Result)
			}
		case "failed":
			line += ": " + r.Fault
		}
		count := fmt.Sprintf("%d steps", r.StepCount)
		if r.StepCount == 1 {
			count = "1 step"
		}
		line += fmt.Sprintf("  [%s, %s", count, bytesText(int64(r.Size)))
		if r.Spent != nil {
			line += ", " + r.Spent.String()
		}
		line += "]"
		if r.Summarized {
			line += " (steps dropped; summary kept)"
		}
		if r.StepCount >= longRun && !r.Summarized {
			line += "\n    warning: a run this long keeps every step it took; a job that never ends grows without bound (SPECS.md, open question 9)"
		}
		fmt.Fprintln(w, line)
		if !opts.Steps {
			continue
		}
		for i, s := range r.Steps {
			FormatStep(w, i+1, s, now)
			if opts.Replies && s.Spent != nil && s.Spent.Model != "" {
				fmt.Fprintln(w, indent(clip(s.Value, replyLimit), "        "))
			}
		}
	}
}

// indent puts prefix before every line of text.
func indent(text, prefix string) string {
	return prefix + strings.ReplaceAll(text, "\n", "\n"+prefix)
}

// FormatStep writes step n of a run as `eye runs` lists it.
func FormatStep(w io.Writer, n int, s StepInfo, now time.Time) {
	step := fmt.Sprintf("    %d. %s", n, s.Call)
	switch s.Status {
	case "sleeping":
		step += ", sleeping until " + clockTime(*s.Until, now)
	case "waiting":
		step += ", waiting until " + clockTime(*s.Until, now)
	default:
		if s.Until != nil {
			step += ", slept until " + clockTime(*s.Until, now)
		} else if s.Value != "()" {
			step += " → " + brief(s.Value)
		}
	}
	if s.Spent != nil {
		step += "  [" + s.Spent.Detail() + "]"
	}
	fmt.Fprintln(w, step)
}

// replyLimit is how much of a step's value `eye runs DIR ID STEP` shows.
const replyLimit = 16 << 10

// StepValue is the whole value step n (counting from 1) of run id returned,
// for `eye runs DIR ID STEP`, up to replyLimit bytes: for a model call, its
// reply as the program read it. A reply that didn't match the schema it was
// asked for says so first, with the problems, and then shows the reply as
// the model wrote it.
func StepValue(prog *ir.Program, store Store, id int64, n int) (string, error) {
	data, ok := store.Run(id)
	if !ok {
		return "", fmt.Errorf("there is no run %d", id)
	}
	r := decodeRun(data)
	switch {
	case r.Summarized:
		return "", fmt.Errorf("run %d keeps only a summary: its steps were dropped", id)
	case n < 1 || n > len(r.Log):
		steps := fmt.Sprintf("%d steps", len(r.Log))
		if len(r.Log) == 1 {
			steps = "1 step"
		}
		return "", fmt.Errorf("run %d has %s, so it has no step %d", id, steps, n)
	case r.Log[n-1].Timer || r.Log[n-1].Event != "":
		return "", fmt.Errorf("step %d of run %d waits, and returns no value", n, id)
	}
	in := New(prog, io.Discard, io.Discard)
	in.rt.useStore(store)
	var v Value
	func() {
		defer func() {
			if recover() != nil {
				v = nil
			}
		}()
		v = in.decode(r.Log[n-1].Value, ir.Pos{})
	}()
	if v == nil {
		return "", fmt.Errorf("step %d of run %d can't be read by this version of the program", n, id)
	}
	if e, ok := isVariant(v, "Err"); ok {
		if m, ok := isVariant(e.Vals[0], "Malformed"); ok && len(m.Vals) == 2 {
			reply, _ := m.Vals[0].(string)
			var problems []string
			if l, ok := m.Vals[1].(*List); ok {
				for _, p := range l.Elems {
					problems = append(problems, fmt.Sprint(p))
				}
			}
			return "The model's reply didn't match the schema it was asked for: " + strings.Join(problems, "; ") +
				"\nIts reply, as it wrote it:\n" + clip(reply, replyLimit), nil
		}
	}
	return clip(showValue(v), replyLimit), nil
}

// clip cuts text to at most limit bytes, on a character's boundary, and
// says how much it left out.
func clip(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + fmt.Sprintf("… (%s more)", bytesText(int64(len(text)-cut)))
}

// FormatJournal writes the summary line of `eye runs`: the journal's size on
// disk, its live data, the disk's free space, and how many runs wait.
func FormatJournal(w io.Writer, path string, st JournalStats, runs []RunInfo) {
	waiting := 0
	for _, r := range runs {
		if r.Status == "sleeping" || r.Status == "ready" {
			waiting++
		}
	}
	free := "free space unknown"
	if st.Free >= 0 {
		free = bytesText(st.Free) + " free"
	}
	fmt.Fprintf(w, "%s: %s on disk, %s live, %s; %d runs, %d not finished\n",
		path, bytesText(st.Size), bytesText(st.Live), free, len(runs), waiting)
}

// bytesText shows a number of bytes with a binary unit, as Eyelang shows a Size.
func bytesText(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	f, u := float64(n), 0
	for f >= 1024 && u < len(units)-1 {
		f /= 1024
		u++
	}
	if u == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return strconv.FormatFloat(f, 'f', 1, 64) + " " + units[u]
}

// clockTime shows a time on the local clock, with how far off it is.
func clockTime(t, now time.Time) string {
	s := t.Local().Format("2006-01-02 15:04:05")
	d := t.Sub(now).Round(time.Second)
	switch {
	case d > 0:
		return s + " (in " + d.String() + ")"
	case d < 0:
		return s + " (" + (-d).String() + " ago)"
	}
	return s
}
