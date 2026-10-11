package interp

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"eyelang/check"
	"eyelang/ir"
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

// digestStandIns starts stand-ins for the services the digest's write job
// calls, points the environment at them, and leaves the model and its price
// to the test. It returns how many times the model was asked.
func digestStandIns(t *testing.T) func() int {
	t.Helper()
	var mu sync.Mutex
	asked := 0
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
		mu.Lock()
		asked++
		mu.Unlock()
		briefing := `{"highlights": [], "sections": [
			{"heading": "Temporary protection", "stories": []},
			{"heading": "Dublin", "stories": []},
			{"heading": "Coming up in Dublin", "stories": []}]}`
		reply, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": briefing}}},
			"usage": map[string]any{"prompt_tokens": 30000, "completion_tokens": 3000}})
		_, _ = w.Write(reply)
	})
	for k, v := range map[string]string{
		"MINIFLUX_URL": miniflux, "MINIFLUX_KEY": "mf-key", "VPN_PROXY_URL": proxy,
		"FLARESOLVERR_URL": flare, "AI_URL": gateway + "/v1", "AI_KEY": "ai-secret",
		"NOTIFY_URL": "http://127.0.0.1:1", // the write job sends nothing
	} {
		t.Setenv(k, v)
	}
	t.Setenv("DIGEST_MODEL", "")
	_ = os.Unsetenv("DIGEST_MODEL")
	return func() int { mu.Lock(); defer mu.Unlock(); return asked }
}

// runDigestWrite runs the digest's write job for 12 October, as the morning
// does, and returns its run once it has ended.
func runDigestWrite(t *testing.T) (*Interp, *jobRun) {
	t.Helper()
	prog, diags := check.Program("../testdata/lab")
	for _, d := range diags {
		t.Fatal(d)
	}
	in := New(prog, io.Discard, io.Discard)
	dublin, _ := time.LoadLocation("Europe/Dublin")
	in.rt.now = time.Date(2026, 10, 12, 7, 40, 0, 0, dublin)
	digest := moduleByPath(t, prog, "digest")
	run := in.rt.startNow(digest.Jobs["write"], []Value{ir.Date{Year: 2026, Month: 10, Day: 12}}, nil, ir.Pos{})
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
		asked := digestStandIns(t)
		t.Setenv("AI_PRICES", "")
		_, run := runDigestWrite(t)
		if run.Status != "failed" || !strings.Contains(run.Fault, "AI_PRICES") || !strings.Contains(run.Fault, model) {
			t.Fatalf("write is %s (%s), want failed with a fault that names %s and AI_PRICES", run.Status, run.Fault, model)
		}
		if asked() != 0 {
			t.Errorf("the model was asked %d times without a price to keep the budget by", asked())
		}
	})

	t.Run("with its price in AI_PRICES", func(t *testing.T) {
		asked := digestStandIns(t)
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
		if asked() != 1 {
			t.Errorf("the model was asked %d times, want 1", asked())
		}
		for _, s := range run.Log {
			if s.Call == "ai.generate" && (s.Tokens != 33000 || math.Abs(s.USD-0.165) > 1e-9) {
				t.Errorf("the model call's step spent %d tokens and $%g, want 33000 and $0.165", s.Tokens, s.USD)
			}
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
