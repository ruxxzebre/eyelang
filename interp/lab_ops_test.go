package interp

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/ir"

	"github.com/jackc/pgx/v5"
)

// The lab's operations: what deploying its programs depends on.

// digestDefaultModel is the model the digest asks for when DIGEST_MODEL
// isn't set, read from its source.
func digestDefaultModel(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("../testdata/lab/digest/digest.eye")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`env\.get!\("DIGEST_MODEL"\) \?\? "([^"]+)"`).FindSubmatch(src)
	if m == nil {
		t.Fatal(`digest.eye no longer reads its model as env.get!("DIGEST_MODEL") ?? "…"`)
	}
	return string(m[1])
}

// digestServices are stand-ins for the services the digest calls.
type digestServices struct {
	mu    sync.Mutex
	asks  int              // how many times the model was asked
	sends []map[string]any // the bodies Notify was sent
	keys  []string         // and their idempotency keys
}

func (s *digestServices) asked() int { s.mu.Lock(); defer s.mu.Unlock(); return s.asks }

// digestStandIns starts stand-ins for the services the digest calls, points
// the environment at them, and leaves the model and its price to the test.
func digestStandIns(t *testing.T) *digestServices {
	t.Helper()
	lab := &digestServices{}
	serve := func(h http.HandlerFunc) string {
		s := httptest.NewServer(h)
		t.Cleanup(s.Close)
		return s.URL
	}
	miniflux := serve(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/me":
			_, _ = io.WriteString(w, `{"id": 1, "username": "digest"}`)
		case strings.HasSuffix(r.URL.Path, "/entries"):
			_, _ = io.WriteString(w, `{"total": 0, "entries": []}`)
		default:
			http.NotFound(w, r)
		}
	})
	proxy := serve(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no route", http.StatusBadGateway)
	})
	flare := serve(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"status": "ok", "message": "", "solution": {"url": "https://www.dublincity.ie/events",
			"status": 200, "response": "<html>Culture Night</html>"}}`)
	})
	gateway := serve(func(w http.ResponseWriter, r *http.Request) {
		lab.mu.Lock()
		lab.asks++
		lab.mu.Unlock()
		briefing := `{"highlights": [], "sections": [
			{"heading": "Temporary protection", "stories": []},
			{"heading": "Dublin", "stories": []},
			{"heading": "Coming up in Dublin", "stories": []}]}`
		reply, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": briefing}}},
			"usage": map[string]any{"prompt_tokens": 30000, "completion_tokens": 3000}})
		_, _ = w.Write(reply)
	})
	notify := serve(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		lab.mu.Lock()
		lab.sends = append(lab.sends, body)
		lab.keys = append(lab.keys, r.Header.Get("X-Actions-Idempotency-Key"))
		lab.mu.Unlock()
		_, _ = io.WriteString(w, `{"id": "msg-1", "deliveries": 1, "deduped": false}`)
	})
	for k, v := range map[string]string{
		"MINIFLUX_URL": miniflux, "MINIFLUX_KEY": "mf-key", "VPN_PROXY_URL": proxy,
		"FLARESOLVERR_URL": flare, "AI_URL": gateway + "/v1", "AI_KEY": "ai-secret",
		"NOTIFY_URL": notify, "NOTIFY_TOKEN": "notify-secret",
	} {
		t.Setenv(k, v)
	}
	for _, k := range []string{"DIGEST_MODEL", "DIGEST_EVENT", "DIGEST_TO"} {
		t.Setenv(k, "") // restored after the test
		_ = os.Unsetenv(k)
	}
	return lab
}

// runDigestWrite runs the digest's write job for 12 October, as the morning
// does, and returns its run once it has ended.
func runDigestWrite(t *testing.T) (*Interp, *jobRun) {
	t.Helper()
	return runDigestJob(t, "../testdata/lab", "write")
}

// runDigestJob runs one of the digest's jobs for 12 October at 07:40 in the
// program in dir, and returns its run once it has ended.
func runDigestJob(t *testing.T, dir, job string) (*Interp, *jobRun) {
	t.Helper()
	prog, diags := check.Program(dir)
	for _, d := range diags {
		t.Fatal(d)
	}
	in := New(prog, io.Discard, io.Discard)
	dublin, _ := time.LoadLocation("Europe/Dublin")
	in.rt.now = time.Date(2026, 10, 12, 7, 40, 0, 0, dublin)
	digest := moduleByPath(t, prog, "digest")
	run := in.rt.startNow(digest.Jobs[job], []Value{ir.Date{Year: 2026, Month: 10, Day: 12}}, nil, ir.Pos{})
	data, _ := in.rt.store.Run(run.(*RunVal).ID)
	return in, decodeRun(data)
}

// The digest's @budget is in dollars, and its default model isn't one whose
// price the runtime knows. Deployed without a price for it, the write run
// fails before the model is asked, and says to give the price in AI_PRICES;
// with the price that the lab's README tells the operator to give, it is
// written within its budget.
func TestDigestBudgetOnItsDefaultModel(t *testing.T) {
	model := digestDefaultModel(t)
	if _, known := prices[model]; known {
		t.Logf("%s now has a built-in price; AI_PRICES is optional for it", model)
	}

	readme, err := os.ReadFile("../testdata/lab/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "AI_PRICES") || !strings.Contains(string(readme), model) {
		t.Errorf("the lab's README doesn't say to give %s's price in AI_PRICES, which the digest's @budget(usd: …) needs", model)
	}

	t.Run("without a price", func(t *testing.T) {
		if _, known := prices[model]; known {
			t.Skipf("%s has a built-in price", model)
		}
		lab := digestStandIns(t)
		t.Setenv("AI_PRICES", "")
		_, run := runDigestWrite(t)
		if run.Status != "failed" || !strings.Contains(run.Fault, "AI_PRICES") || !strings.Contains(run.Fault, model) {
			t.Fatalf("write is %s (%s), want failed with a fault that names %s and AI_PRICES", run.Status, run.Fault, model)
		}
		if lab.asked() != 0 {
			t.Errorf("the model was asked %d times without a price to keep the budget by", lab.asked())
		}
	})

	t.Run("with its price in AI_PRICES", func(t *testing.T) {
		lab := digestStandIns(t)
		// Dear on purpose: 30,000 tokens in and 3,000 out cost $0.165, a
		// third of the budget, so the reply's allowance is cut by the budget
		// but the call is still sent.
		t.Setenv("AI_PRICES", "other-model=1/1, "+model+"=5/5")
		in, run := runDigestWrite(t)
		if run.Status != "done" {
			t.Fatalf("write is %s (%s %s)", run.Status, run.FaultKind, run.Fault)
		}
		if got := show(in.decode(run.Result, ir.Pos{})); !strings.HasPrefix(got, "Ok(") {
			t.Errorf("write returned %s", got)
		}
		if lab.asked() != 1 {
			t.Errorf("the model was asked %d times, want 1", lab.asked())
		}
		for _, s := range run.Log {
			if s.Call == "ai.generate" && (s.Tokens != 33000 || math.Abs(s.USD-0.165) > 1e-9) {
				t.Errorf("the model call's step spent %d tokens and $%g, want 33000 and $0.165", s.Tokens, s.USD)
			}
		}
		// eye runs shows it, on the step and in the run's total, also
		// once the run keeps only a summary.
		var out strings.Builder
		FormatRuns(&out, []RunInfo{in.runInfo(run)}, true, in.rt.now)
		const spent = "30000 tokens in, 3000 out, $0.1650]"
		if strings.Count(out.String(), spent) != 2 || !strings.Contains(out.String(), "  ["+spent) || !strings.Contains(out.String(), " B, "+spent) {
			t.Errorf("eye runs shows:\n%s\nwant %s on the run and on its ai.generate step", out.String(), spent)
		}
		run.summarize()
		out.Reset()
		FormatRuns(&out, []RunInfo{in.runInfo(run)}, false, in.rt.now)
		if !strings.Contains(out.String(), ", 30000 tokens in, 3000 out, $0.1650]") {
			t.Errorf("a summarized run shows:\n%s", out.String())
		}
	})
}

// Every module that calls a contract's operations is in its @allow, and
// every module in it calls them: phase 6 refuses a call from a module the
// contract doesn't allow, and a name left in the list keeps a door open.
// The module that serves the contract calls its own implementation, and
// tests call fakes, so neither counts.
func TestLabAllowMatchesCallers(t *testing.T) {
	prog, diags := check.Program("../testdata/lab")
	for _, d := range diags {
		t.Fatal(d)
	}
	callers := opCallers(prog)
	allowRe := regexp.MustCompile(`(?m)^@allow\(([^)]*)\)`)
	for _, m := range prog.Modules {
		for _, ct := range m.Contracts {
			var allowed []string
			for _, f := range m.Files {
				if a := allowRe.FindSubmatch(f.Src); a != nil {
					for _, name := range strings.Split(string(a[1]), ",") {
						allowed = append(allowed, strings.TrimSpace(name))
					}
				}
			}
			served := map[string]bool{}
			for _, op := range ct.Ops {
				if op.Impl != nil {
					served[op.Impl.Module.Name] = true
				}
			}
			var calling []string
			for name := range callers[ct] {
				if !served[name] {
					calling = append(calling, name)
				}
			}
			sort.Strings(calling)
			if allowed == nil {
				t.Logf("%s has no @allow; called by %v", ct.Name, calling)
				continue
			}
			sort.Strings(allowed)
			if strings.Join(allowed, ",") != strings.Join(calling, ",") {
				t.Errorf("%s: @allow(%s), but its callers are %s", ct.Name, strings.Join(allowed, ", "), strings.Join(calling, ", "))
			}
		}
	}
}

// opCallers finds, for each contract, the modules whose code (tests left
// out) names one of its operations.
func opCallers(prog *ir.Program) map[*ir.Contract]map[string]bool {
	byFile := map[*ir.File]*ir.Module{}
	for _, m := range prog.Modules {
		for _, f := range m.Files {
			byFile[f] = m
		}
	}
	found := map[*ir.Contract]map[string]bool{}
	seen := map[uintptr]bool{}
	skip := map[reflect.Type]bool{
		reflect.TypeOf(&ir.Module{}): true, reflect.TypeOf(&ir.File{}): true,
		reflect.TypeOf(&ir.Test{}): true, reflect.TypeOf(&ir.Contract{}): true,
		reflect.TypeOf(&ir.Op{}): true, reflect.TypeOf(&ir.Type{}): true,
		reflect.TypeOf(&ir.TypeExpr{}): true,
	}
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || skip[v.Type()] || seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
			if ref, ok := v.Interface().(*ir.Ref); ok {
				if op, ok := ref.Decl.(*ir.Op); ok && ref.Pos.File != nil {
					if found[op.Contract] == nil {
						found[op.Contract] = map[string]bool{}
					}
					found[op.Contract][byFile[ref.Pos.File].Name] = true
				}
				return // what it names is walked from its own module
			}
			walk(v.Elem())
		case reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			for i := range v.NumField() {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := range v.Len() {
				walk(v.Index(i))
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				walk(v.MapIndex(k))
			}
		}
	}
	for _, m := range prog.Modules {
		if m.Dir == "" {
			continue // the prelude and the standard modules
		}
		walk(reflect.ValueOf(m.Funcs))
		walk(reflect.ValueOf(m.Jobs))
		walk(reflect.ValueOf(m.Entities))
		walk(reflect.ValueOf(m.Globals))
		walk(reflect.ValueOf(m.Impls))
		walk(reflect.ValueOf(m.Views))
		walk(reflect.ValueOf(m.Main))
		for _, ct := range m.Contracts {
			for _, op := range ct.Ops {
				walk(reflect.ValueOf(op.Impl))
			}
		}
	}
	return found
}

// The program deployed is the digest alone: testdata/lab/deploy/digest
// checks, its tests pass, and of the lab's jobs it has only the digest's,
// so the notifier, the reminders and the NetBird sync don't run beside it.
func TestDigestDeployProgram(t *testing.T) {
	dir := "../testdata/lab/deploy/digest"
	prog, diags := check.Program(dir)
	for _, d := range diags {
		t.Fatal(d)
	}
	var jobs []string
	for _, m := range prog.Modules {
		for name := range m.Jobs {
			jobs = append(jobs, m.Path+"."+name)
		}
	}
	sort.Strings(jobs)
	if got := strings.Join(jobs, " "); got != "digest.command digest.morning digest.write" {
		t.Errorf("the deployed program has the jobs %s, want the digest's alone", got)
	}
	results := RunTests(prog, "")
	if len(results) == 0 {
		t.Error("the deployed program runs none of the digest's tests")
	}
	for _, r := range results {
		if r.Failure != "" {
			t.Errorf("%s: %s", r.Name, r.Failure)
		}
	}
}

// DIGEST_EVENT and DIGEST_TO send the digest on an event of its own, to
// the Authentik uids listed, with keys of its own: a shadow run beside the
// TypeScript digest reaches only the operator, and Notify doesn't drop
// either digest's send as a repeat of the other's, since it dedupes by
// caller and key whatever the event and the recipients. Unset or empty,
// they change nothing. The failure notice goes the same way.
func TestDigestShadowChannel(t *testing.T) {
	cases := []struct {
		name, event, to string
		failed          bool   // the write fails, so the morning sends its failure notice
		wantEvent, key  string // the key without the -failed of a notice
		wantTo          string // `to` as JSON, "" for none
	}{
		{"unset", "", "", false, "digest.ireland", "ireland-2026-10-12", ""},
		{"empty", " ", " , ", false, "digest.ireland", "ireland-2026-10-12", ""},
		{"default event", "digest.ireland", "", false, "digest.ireland", "ireland-2026-10-12", ""},
		{"own event", "digest.ireland.eye", "", false, "digest.ireland.eye", "digest.ireland.eye:ireland-2026-10-12", ""},
		{"operator only", "digest.ireland.eye", " 7bd86dfac2ab ,, b-2 ", false, "digest.ireland.eye", "digest.ireland.eye:ireland-2026-10-12", `["7bd86dfac2ab","b-2"]`},
		{"recipients alone", "", "7bd86dfac2ab", false, "digest.ireland", "digest.ireland:ireland-2026-10-12", `["7bd86dfac2ab"]`},
		{"failure notice", "digest.ireland.eye", "7bd86dfac2ab", true, "digest.ireland.eye", "digest.ireland.eye:ireland-2026-10-12-failed", `["7bd86dfac2ab"]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lab := digestStandIns(t)
			if !c.failed {
				t.Setenv("AI_PRICES", digestDefaultModel(t)+"=1/1")
			}
			if c.event != "" {
				t.Setenv("DIGEST_EVENT", c.event)
			}
			if c.to != "" {
				t.Setenv("DIGEST_TO", c.to)
			}
			in, run := runDigestJob(t, "../testdata/lab/deploy/digest", "morning")
			if run.Status != "done" {
				t.Fatalf("morning is %s (%s)", run.Status, run.Fault)
			}
			want := "Ok(Accepted("
			if c.failed {
				want = "Err(Faulted("
			}
			if got := show(in.decode(run.Result, ir.Pos{})); !strings.HasPrefix(got, want) {
				t.Errorf("morning returned %s", got)
			}
			if len(lab.sends) != 1 {
				t.Fatalf("Notify got %d sends, want 1", len(lab.sends))
			}
			b := lab.sends[0]
			if b["event"] != c.wantEvent || b["dedupe_key"] != c.key || lab.keys[0] != c.key {
				t.Errorf("Notify got event %v with key %v (header %s), want %s with %s", b["event"], b["dedupe_key"], lab.keys[0], c.wantEvent, c.key)
			}
			to, sent := b["to"]
			got, _ := json.Marshal(to)
			if c.wantTo == "" && sent || c.wantTo != "" && string(got) != c.wantTo {
				t.Errorf("Notify got to %s (sent: %v), want %q", got, sent, c.wantTo)
			}
		})
	}
}

// Notify sends to at most 20 people, so DIGEST_TO with more fails the
// morning's run before anything is written or sent, with a fault that
// names it.
func TestDigestToTakesAtMostTwenty(t *testing.T) {
	lab := digestStandIns(t)
	t.Setenv("AI_PRICES", digestDefaultModel(t)+"=1/1")
	uids := make([]string, 21)
	for i := range uids {
		uids[i] = "u" + strconv.Itoa(i)
	}
	t.Setenv("DIGEST_TO", strings.Join(uids, ","))
	_, run := runDigestJob(t, "../testdata/lab/deploy/digest", "morning")
	if run.Status != "failed" || !strings.Contains(run.Fault, "DIGEST_TO lists 21 people") {
		t.Errorf("morning is %s (%s), want failed with a fault that names DIGEST_TO", run.Status, run.Fault)
	}
	if lab.asked() != 0 || len(lab.sends) != 0 {
		t.Errorf("the model was asked %d times and Notify sent %d messages", lab.asked(), len(lab.sends))
	}
}

// eye run on a database writes nothing to the program's directory or to
// $HOME: a pod can run it on a read-only root filesystem. (It reads
// DIR/.eye/inbox, for answers to approvals, if that exists.)
func TestRunOnAStoreWritesNoFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TMPDIR", t.TempDir())
	dir := t.TempDir() + "/digest"
	if out, err := exec.Command("cp", "-RL", "../testdata/lab/deploy/digest", dir).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	before := listFiles(t, dir) + listFiles(t, home) + listFiles(t, os.Getenv("TMPDIR"))
	prog, diags := check.Program(dir)
	for _, d := range diags {
		t.Fatal(d)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var out strings.Builder
	Run(ctx, prog, RunOptions{Stdout: &out, Stderr: &out, Inbox: dir + "/.eye/inbox"})
	if after := listFiles(t, dir) + listFiles(t, home) + listFiles(t, os.Getenv("TMPDIR")); after != before {
		t.Errorf("eye run wrote files:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func listFiles(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		b.WriteString(path + "\n")
		return nil
	})
	return b.String()
}

// The store takes CloudNativePG's address as the app's secret gives it, with
// the schema added as search_path, and the usual libpq parameters: the lab's
// README says so.
func TestStoreURLForCloudNativePG(t *testing.T) {
	url := "postgresql://app:secret@postgres-rw.digest.svc.cluster.local:5432/app?search_path=lab&sslmode=require&connect_timeout=5"
	if !IsPostgres(url) {
		t.Fatalf("%s isn't taken as a Postgres address", url)
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RuntimeParams["search_path"] != "lab" || cfg.TLSConfig == nil || cfg.ConnectTimeout != 5*time.Second ||
		cfg.Database != "app" || cfg.User != "app" || cfg.Port != 5432 {
		t.Errorf("parsed as %+v", cfg)
	}
}
