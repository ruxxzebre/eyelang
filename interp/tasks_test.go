package interp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// On the system clock, tasks that wait for the network overlap: two requests
// to servers that each take 300 ms take about 300 ms together, not 600.
func TestTasksOverlapTheirWaits(t *testing.T) {
	slow := func(body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-r.Context().Done():
			}
			_, _ = w.Write([]byte(body))
		}))
	}
	a, b := slow("a"), slow("b")
	defer a.Close()
	defer b.Close()
	t.Setenv("A_URL", a.URL)
	t.Setenv("B_URL", b.URL)
	prog := loadFiles(t, map[string]string{"main.eye": `
import http

fn body(name: String) -> String uses {net, env} {
  let url = env.get(name) ?? ""
  match http.get(url) {
    Ok(r) => r.body
    Err(_) => "failed"
  }
}

pub fn main() uses {net, env, stdio, clock} {
  let (x, y) = both(fn { body("A_URL") }, fn { body("B_URL") })
  let all = ["A_URL", "B_URL", "A_URL", "B_URL"] |> parallelMap(fn n: body(n), limit: 2)
  let first = race([fn { body("A_URL") }, fn {
    sleep(5 s)
    "late"
  }])
  let none = timeout(50 ms, fn { body("B_URL") })
  print(x, y, all, first, none)
}
`})
	var out bytes.Buffer
	start := time.Now()
	if code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: os.Stderr}); code != 0 {
		t.Fatalf("exit %d: %s", code, out.String())
	}
	// both: 300 ms; parallelMap, two at a time: 600 ms; race: 300 ms;
	// timeout: 50 ms. One after another it would be 3 s or more.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v; the tasks' waits didn't overlap", elapsed)
	}
	if got := strings.TrimSpace(out.String()); got != `a b ["a", "b", "a", "b"] a None` {
		t.Errorf("printed %q", got)
	}
}

// A task that fails takes its siblings down with it, and the call reports
// the failure.
func TestATaskFaultEndsTheCall(t *testing.T) {
	prog := loadFiles(t, map[string]string{"main.eye": `
pub fn main() uses {stdio, clock} {
  let pair = both(fn {
    sleep(10 ms)
    unreachable("broken")
  }, fn {
    sleep(5 s)
    1
  })
  print(pair)
}
`})
	var out, errs bytes.Buffer
	start := time.Now()
	if code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &errs}); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errs.String(), "broken") {
		t.Errorf("stderr %q", errs.String())
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v; the other task wasn't cancelled", elapsed)
	}
}
