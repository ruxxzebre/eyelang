package interp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"eyelang/ir"
)

// Live tests (SPECS.md §20): `eye test --live ai,notify` sends the named
// modules' calls to the real services, where a `with fakes(…)` would have
// answered them from recordings. A test that needs a live service it can't
// reach is skipped. With --record, what the live services answered is
// written as recordings, in the files the fakes would have read, from the
// tests that passed.

// TestOptions say how RunTestsWith runs a program's tests.
type TestOptions struct {
	// Filter keeps the tests whose names contain it.
	Filter string
	// Live are the modules whose calls go to the real services: ai, http, or
	// modules that declare one contract. LiveModules finds them by name.
	Live []*ir.Module
	// Record writes what the live services answered as recordings.
	Record bool
	// Hosts are the host functions a Go program that embeds Eyelang
	// registered, which extern fns call.
	Hosts *Hosts
	// Crossings counts, for each test, what would cross between compiled
	// code and the runner (TestResult.Crossings).
	Crossings bool
}

// Recorded is a recording file --record wrote, and how many exchanges the
// tests recorded into it.
type Recorded struct {
	Path      string
	Exchanges int
}

// LiveModules finds the modules --live names: each by its import path, or
// by its name when one module has it.
func LiveModules(prog *ir.Program, names []string) ([]*ir.Module, error) {
	var mods []*ir.Module
	for _, name := range names {
		var found []*ir.Module
		for _, m := range prog.Modules {
			if m != prog.Root && m.Path == name {
				found = append(found, m)
			}
		}
		if len(found) == 0 {
			for _, m := range prog.Modules {
				if m != prog.Root && m.Name == name {
					found = append(found, m)
				}
			}
		}
		switch {
		case len(found) == 0:
			return nil, fmt.Errorf("--live %s: the program has no module %s", name, name)
		case len(found) > 1:
			var paths []string
			for _, m := range found {
				paths = append(paths, m.Path)
			}
			return nil, fmt.Errorf("--live %s: several modules are named %s; name one by its path: %s", name, name, strings.Join(paths, ", "))
		}
		m := found[0]
		std := m.Dir == ""
		switch {
		case std && (m.Path == "ai" || m.Path == "http"):
		case std:
			return nil, fmt.Errorf("--live %s: the standard module %s calls no other service; --live takes ai, http and contract modules", name, m.Path)
		case len(m.Contracts) != 1:
			return nil, fmt.Errorf("--live %s: %s declares %d contracts, and --live takes modules that declare one, as fakes(…) does", name, m.Path, len(m.Contracts))
		}
		mods = append(mods, m)
	}
	return mods, nil
}

// liveTest is what a test run with --live keeps: which modules are live,
// why the test is skipped, if it is, and what --record will write.
type liveTest struct {
	modules map[*ir.Module]bool
	ai      bool // the standard ai module is live
	record  bool
	skip    string
	// secrets are the values of the secrets the test read, by name, which
	// a recording never holds.
	secrets map[string]string
	// files are the recordings the test made, in the order first written
	// to; aiFile and httpFile are where model replies and http.get's
	// answers go, when a `with fakes(ai, …)` or `with fakes(http, …)` left
	// them live.
	files            []*taking
	aiFile, httpFile *taking
}

func newLiveTest(opts TestOptions) *liveTest {
	if len(opts.Live) == 0 {
		return nil
	}
	lt := &liveTest{modules: map[*ir.Module]bool{}, record: opts.Record, secrets: map[string]string{}}
	for _, m := range opts.Live {
		lt.modules[m] = true
		lt.ai = lt.ai || (m.Path == "ai" && m.Dir == "")
	}
	return lt
}

// isLive reports whether m's calls go to the real service in this test.
func (rt *runtime) isLive(m *ir.Module) bool {
	return rt.live != nil && rt.live.modules[m]
}

// liveAI reports whether model calls go to the real providers in this test.
func (rt *runtime) liveAI() bool { return rt.live != nil && rt.live.ai }

// skipTest stops a live test that can't reach a service it needs. The test
// is reported as skipped, with why, wherever the fault is caught.
func (rt *runtime) skipTest(at ir.Pos, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if rt.live.skip == "" {
		rt.live.skip = msg
	}
	panic(&Fault{Pos: at, Msg: msg})
}

// lookupSecret reads a secret as the package's lookupSecret does, and in a
// live test keeps its value, so that a recording never holds it.
func (in *Interp) lookupSecret(name string) (string, bool) {
	v, ok := lookupSecret(name)
	if ok && v != "" && in.rt.live != nil {
		in.rt.live.secrets[name] = v
	}
	return v, ok
}

// applyLive applies a `with fakes(…)` to module m, which is live: its calls
// go to the real service, and with --record what it answers is kept for
// path. The test is skipped if the service's address or secret isn't set.
func (h *fakesHandler) applyLive(in *Interp, m *ir.Module, path string, at ir.Pos) {
	rt := in.rt
	lt := rt.live
	switch {
	case m.Path == "ai" && m.Dir == "":
		_, gateway := serviceURL("ai")
		_, anthropic := in.lookupSecret("anthropic-api-key")
		_, openai := in.lookupSecret("openai-api-key")
		if !gateway && !anthropic && !openai {
			rt.skipTest(at, "--live ai: no model provider is set: set AI_URL, ANTHROPIC_API_KEY or OPENAI_API_KEY")
		}
		rt.aiReplies = nil
		lt.aiFile = nil
		if lt.record {
			lt.aiFile = lt.taking(path, "ai", recordReplies)
		}
		return
	case m.Path == "http" && m.Dir == "":
		rt.httpFakes = nil
		lt.httpFile = nil
		if lt.record {
			lt.httpFile = lt.taking(path, "http", recordByKey)
		}
		return
	}
	var ct *ir.Contract
	for _, c := range m.Contracts {
		ct = c
	}
	var missing []string
	if _, ok := serviceURL(m.Path); !ok {
		missing = append(missing, envName(m.Path)+"_URL, the address of its service")
	}
	if s := ct.HTTP.AuthSecret; s != "" {
		if _, ok := in.lookupSecret(s); !ok {
			missing = append(missing, fmt.Sprintf("%s, its secret %q", envName(s), s))
		}
	}
	if len(missing) > 0 {
		rt.skipTest(at, "--live %s: set %s", m.Path, strings.Join(missing, ", and "))
	}
	fs := &fakeSet{path: path, live: true}
	if lt.record {
		fs.file = lt.taking(path, ct.Name, recordByKey)
	}
	rt.fakes[ct] = fs
	if h.sets == nil {
		h.sets = map[*ir.Contract]*fakeSet{}
	}
	h.sets[ct] = fs
}

// callLive calls an operation of a live module's service, through a
// `with fakes(…)` that left the module live: over HTTP, as eye run does.
// The fakes still list the call, and --record keeps the answer.
func (rt *runtime) callLive(fs *fakeSet, op *ir.Op, args []Value, named map[string]Value, at ir.Pos) Value {
	mod := op.Contract.Module
	if op.HTTP == nil {
		rt.skipTest(at, "--live %s: %s.%s has no @http, so it can't be called live", mod.Path, mod.Name, op.Name)
	}
	v, ok := rt.callHTTP(op, args, named, at)
	if !ok {
		rt.skipTest(at, "--live %s: set %s_URL, the address of its service", mod.Path, envName(mod.Path))
	}
	fs.answered = append(fs.answered, fakeCall{op, rt.callArgs(op, args, named)})
	if fs.file != nil {
		key, _ := named["key"].(string)
		fs.file.answer(op.Name, key, rt.in.toWire(v, false))
	}
	return v
}

// recordReply keeps a live model's reply for --record: its text, what it
// spent, and the tools it called. A provider's failure or refusal isn't
// kept, since a recording holds replies.
func (rt *runtime) recordReply(r aiReply, failure string) {
	if rt.live == nil || rt.live.aiFile == nil || failure != "" || r.refusal != "" {
		return
	}
	e := takenExchange{}
	switch text := strings.TrimSpace(r.text); {
	case text == "":
	case json.Valid([]byte(text)):
		e.Result = json.RawMessage(text)
	default:
		e.Result, _ = json.Marshal(r.text)
	}
	if r.spent.In > 0 || r.spent.Out > 0 {
		e.Usage = &recordedUsage{Input: r.spent.In, Output: r.spent.Out}
		if r.spent.USD > 0 {
			usd := r.spent.USD
			e.Usage.USD = &usd
		}
	}
	for _, c := range r.calls {
		e.Calls = append(e.Calls, recordedCall{Tool: c.Name, Input: c.Input})
	}
	rt.live.aiFile.Exchanges = append(rt.live.aiFile.Exchanges, e)
}

// recordGet keeps a live http.get's answer for --record, by URL.
func (rt *runtime) recordGet(target string, v Value) {
	if rt.live == nil || rt.live.httpFile == nil {
		return
	}
	rt.live.httpFile.answer("get", target, rt.in.toWire(v, false))
}

// How a recording file is written: by operation and key, merged with what
// the file already holds, or as one test's replies in order, replacing it.
const (
	recordByKey = iota
	recordReplies
)

// taking is a recording being made: the exchanges for one file.
type taking struct {
	path      string
	contract  string
	how       int
	Exchanges []takenExchange
}

// takenExchange is an exchange as a recording file holds it.
type takenExchange struct {
	Op     string          `json:"op,omitempty"`
	Key    string          `json:"key,omitempty"`
	Args   json.RawMessage `json:"args,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Usage  *recordedUsage  `json:"usage,omitempty"`
	Calls  []recordedCall  `json:"calls,omitempty"`
}

// taking is the recording this test makes at path.
func (lt *liveTest) taking(path, contract string, how int) *taking {
	for _, t := range lt.files {
		if t.path == path && t.how == how {
			return t
		}
	}
	t := &taking{path: path, contract: contract, how: how}
	lt.files = append(lt.files, t)
	return t
}

// answer keeps what a service answered to op with key, in place of an
// earlier answer to the same: the last answer is the one replayed.
func (t *taking) answer(op, key string, result any) {
	raw, err := marshalRecorded(result)
	if err != nil {
		return
	}
	t.put(takenExchange{Op: op, Key: key, Result: raw})
}

func (t *taking) put(e takenExchange) {
	for i, old := range t.Exchanges {
		if old.Op == e.Op && old.Key == e.Key {
			t.Exchanges[i] = e
			return
		}
	}
	t.Exchanges = append(t.Exchanges, e)
}

// recorder gathers the recordings of the tests that passed, and writes
// them when the tests are done.
type recorder struct {
	files   []*taking
	secrets map[string]string
}

// add keeps the recordings of a test that passed. A file of replies is
// written by the first test that records into it.
func (r *recorder) add(lt *liveTest) {
	for name, v := range lt.secrets {
		r.secrets[name] = v
	}
outer:
	for _, t := range lt.files {
		if len(t.Exchanges) == 0 {
			continue
		}
		for _, have := range r.files {
			if have.path != t.path {
				continue
			}
			if t.how == recordByKey && have.how == recordByKey {
				for _, e := range t.Exchanges {
					have.put(e)
				}
			}
			continue outer
		}
		r.files = append(r.files, t)
	}
}

// write writes each recording: a file by key is merged with what it
// already holds, and a file of replies replaced. Every secret's value is
// replaced by its name first.
func (r *recorder) write() ([]Recorded, error) {
	var written []Recorded
	for _, t := range r.files {
		out := recordingFile{Contract: t.contract}
		if t.how == recordByKey {
			data, err := os.ReadFile(t.path)
			switch {
			case errors.Is(err, fs.ErrNotExist):
			case err != nil:
				return written, err
			default:
				if err := json.Unmarshal(data, &out); err != nil {
					return written, fmt.Errorf("%s: can't add to it: %v", t.path, err)
				}
				if out.Contract != t.contract {
					return written, fmt.Errorf("%s records contract %q, not %s, so it wasn't changed", t.path, out.Contract, t.contract)
				}
			}
			kept := &taking{Exchanges: out.Exchanges}
			for _, e := range t.Exchanges {
				kept.put(e)
			}
			out.Exchanges = kept.Exchanges
		} else {
			out.Exchanges = t.Exchanges
		}
		for i := range out.Exchanges {
			out.Exchanges[i] = r.scrub(out.Exchanges[i])
		}
		data, err := marshalRecorded(out)
		if err != nil {
			return written, err
		}
		if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(t.path, append(data, '\n'), 0o644); err != nil {
			return written, err
		}
		written = append(written, Recorded{Path: t.path, Exchanges: len(t.Exchanges)})
	}
	return written, nil
}

// recordingFile is a recording file as --record writes it.
type recordingFile struct {
	Contract  string          `json:"contract"`
	Exchanges []takenExchange `json:"exchanges"`
}

// scrub replaces the value of every secret the tests read with
// `<secret name>`, wherever it appears in an exchange.
func (r *recorder) scrub(e takenExchange) takenExchange {
	if len(r.secrets) == 0 {
		return e
	}
	names := make([]string, 0, len(r.secrets))
	for name := range r.secrets {
		names = append(names, name)
	}
	// Longer secrets first, so that one containing another is replaced whole.
	sort.Slice(names, func(i, j int) bool {
		a, b := r.secrets[names[i]], r.secrets[names[j]]
		return len(a) > len(b) || len(a) == len(b) && names[i] < names[j]
	})
	text := func(s string) string {
		for _, name := range names {
			s = strings.ReplaceAll(s, r.secrets[name], "<secret "+name+">")
		}
		return s
	}
	raw := func(m json.RawMessage) json.RawMessage {
		if len(m) == 0 {
			return m
		}
		x, err := decodeJSON(m)
		if err != nil {
			return m
		}
		y := scrubJSON(x, text)
		if reflect.DeepEqual(x, y) {
			return m // as it came, in its order
		}
		out, err := marshalRecorded(y)
		if err != nil {
			return m
		}
		return out
	}
	e.Op, e.Key = text(e.Op), text(e.Key)
	e.Result = raw(e.Result)
	calls := make([]recordedCall, len(e.Calls))
	for i, c := range e.Calls {
		calls[i] = recordedCall{Tool: text(c.Tool), Input: raw(c.Input)}
	}
	if e.Calls != nil {
		e.Calls = calls
	}
	return e
}

// scrubJSON applies text to every string in a JSON value, keys included.
func scrubJSON(x any, text func(string) string) any {
	switch v := x.(type) {
	case string:
		return text(v)
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = scrubJSON(e, text)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[text(k)] = scrubJSON(e, text)
		}
		return out
	}
	return x
}

// marshalRecorded writes a value as a recording holds it: indented, and
// with <, > and & as they are.
func marshalRecorded(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

