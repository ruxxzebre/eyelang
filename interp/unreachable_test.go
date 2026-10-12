package interp

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// freeAddr is an address nothing listens on, so a connection to it is
// refused until serveAt starts a server there.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// serveAt starts h on addr.
func serveAt(t *testing.T, addr string, h http.Handler) {
	t.Helper()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(h)
	_ = srv.Listener.Close()
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)
}

// upAfter makes the runtime start h on addr before its second retry: the
// first two tries are refused, the third gets through.
func upAfter(t *testing.T, in *Interp, addr string, h http.Handler) {
	in.rt.fastRetries = true
	in.rt.beforeRetry = func(try int) {
		if try == 1 {
			serveAt(t, addr, h)
		}
	}
}

const getPage = `
import http

job get(url: String) -> String {
  match http.get!(url) {
    Ok(r) => "{r.status} {r.body}"
    Err(e) => "{e}"
  }
}
`

func TestHTTPGetTriesAgainWhenRefused(t *testing.T) {
	addr := freeAddr(t)
	in := New(load(t, getPage), io.Discard, io.Discard)
	tries := 0
	in.rt.fastRetries = true
	in.rt.beforeRetry = func(try int) {
		tries++
		if try == 1 {
			serveAt(t, addr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "hello")
			}))
		}
	}
	if got := runJob(t, in, "get", "http://"+addr+"/"); got != `Completed("200 hello")` {
		t.Errorf("get = %s", got)
	}
	if tries != 2 {
		t.Errorf("waited %d times before it got through, want 2", tries)
	}
}

func TestHTTPGetGivesUpSayingWhy(t *testing.T) {
	addr := freeAddr(t)
	in := New(load(t, getPage), io.Discard, io.Discard)
	in.rt.fastRetries = true
	want := `Completed("NoAnswer(reason: \"127.0.0.1 refused the connection, 8 times over a minute\")")`
	if got := runJob(t, in, "get", "http://"+addr+"/"); got != want {
		t.Errorf("get = %s\nwant %s", got, want)
	}
}

const askGateway = `
import ai

type Answer = { n: Int }

job ask() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "lab-model", prompt: "three", effort: Some(High))
}
`

// busyGateway answers chat completions with {"n": 3}, after answering the
// first busy times with 503, and keeps the requests.
type busyGateway struct {
	mu       sync.Mutex
	busy     int
	requests []map[string]any
}

func (g *busyGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, body)
	if len(g.requests) <= g.busy {
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	out, _ := json.Marshal(chatReply(`{"n": 3}`, "stop", 10, 2))
	_, _ = w.Write(out)
}

func TestModelCallTriesAgainAndSendsEffort(t *testing.T) {
	addr := freeAddr(t)
	t.Setenv("AI_URL", "http://"+addr+"/v1")
	t.Setenv("AI_PRICES", "")
	in := New(load(t, askGateway), io.Discard, io.Discard)
	g := &busyGateway{busy: 1}
	upAfter(t, in, addr, g)
	if got := runJob(t, in, "ask"); got != "Completed(Ok({ n: 3 }))" {
		t.Fatalf("ask = %s", got)
	}
	// Refused twice, then a 503, then the answer.
	if len(g.requests) != 2 {
		t.Fatalf("the gateway got %d requests, want 2: the busy one and the one answered", len(g.requests))
	}
	if e := g.requests[1]["reasoning_effort"]; e != "high" {
		t.Errorf("reasoning_effort = %v, want high", e)
	}
}

func TestModelCallGivesUpSayingWhy(t *testing.T) {
	t.Setenv("AI_PRICES", "")
	t.Run("refused", func(t *testing.T) {
		addr := freeAddr(t)
		t.Setenv("AI_URL", "http://"+addr+"/v1")
		in := New(load(t, askGateway), io.Discard, io.Discard)
		in.rt.fastRetries = true
		want := `Completed(Err(ProviderError(status: 0, message: "127.0.0.1 refused the connection, 8 times over a minute")))`
		if got := runJob(t, in, "ask"); got != want {
			t.Errorf("ask = %s\nwant %s", got, want)
		}
	})
	t.Run("busy", func(t *testing.T) {
		g := &busyGateway{busy: 100}
		srv := httptest.NewServer(g)
		defer srv.Close()
		t.Setenv("AI_URL", srv.URL+"/v1")
		in := New(load(t, askGateway), io.Discard, io.Discard)
		in.rt.fastRetries = true
		want := `Completed(Err(ProviderError(status: 503, message: "busy, 8 times over a minute")))`
		if got := runJob(t, in, "ask"); got != want {
			t.Errorf("ask = %s\nwant %s", got, want)
		}
		if len(g.requests) != 8 {
			t.Errorf("%d requests, want 8", len(g.requests))
		}
	})
}

func TestAnthropicGetsEffort(t *testing.T) {
	noGateway(t)
	api := &standIn{replies: []map[string]any{anthropicReply(`{"n": 3}`, "end_turn", 30, 5)}}
	t.Setenv("ANTHROPIC_URL", api.serve(t).URL)
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	in := New(load(t, `
import ai

type Answer = { n: Int }

job ask() -> Result(Answer, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", prompt: "three", effort: Some(ai.Medium))
}
`), io.Discard, io.Discard)
	if got := runJob(t, in, "ask"); got != "Completed(Ok({ n: 3 }))" {
		t.Fatalf("ask = %s", got)
	}
	output := api.requests[0]["output_config"].(map[string]any)
	if output["effort"] != "medium" || output["format"] == nil {
		t.Errorf("output_config = %v, want effort medium beside the format", output)
	}
}
