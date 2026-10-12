package interp

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const shopContract = `
pub type ItemId = | ItemId(String)

pub type Item = { id: ItemId, name: String, tags: List(String) }

pub type Placing =
  | Placed(order: String, count: Int, repeated: Bool)
  | OutOfStock(item: ItemId)
  | Refused(reason: String)

@port(4100)
@http(names: snake)
@auth(header: "X-Shop-Token", secret: "shop-token")
contract Shop {
  @http("POST /v1/orders", status: { 201: Placed, 409: OutOfStock, 422: Refused, 400: Refused })
  @key(required, header: "Order-Key")
  command place(item: ItemId, count: Int = 1, giftNote: Option(String) = None) -> Placing

  @http("GET /v1/items/:id")
  query item(id: ItemId, tags: List(String) = [], limit: Option(Int) = None) -> Option(Item)

  query internal() -> Int
}
`

// shopServer is a program that serves Shop, with no jobs and no main: it
// runs only to answer requests.
const shopServer = `
import shop.{ItemId, Placing}

entity Order(key: Key) {
  state item: Option(ItemId) = None
  state count: Int = 0

  pub fn place(i: ItemId, n: Int) -> Placing {
    let repeated = item != None
    if !repeated {
      item = Some(i)
      count = n
    }
    Placed(order: "{key}", count:, repeated:)
  }
}

impl shop.Shop {
  fn place(item, count, giftNote, key) {
    if item == ItemId("gone") { return OutOfStock(item:) }
    if count < 1 { return Refused(reason: "count must be at least 1") }
    let placed = Order(key).place(item, count)
    if giftNote == Some("fault") { unreachable("a gift note that faults") }
    placed
  }

  fn item(id, tags, limit) = Some({ id, name: if limit == Some(2) then "two things" else "a thing", tags })

  fn internal() = 1
}
`

// serveShop runs the Shop server under Run on a free port, and returns its
// base URL and what it wrote.
func serveShop(t *testing.T) (string, *lockedBuffer) {
	t.Helper()
	t.Setenv("SHOP_PORT", "0")
	t.Setenv("SHOP_TOKEN", "s3cret")
	prog := loadFiles(t, map[string]string{"shop/shop.eye": shopContract, "main.eye": shopServer})
	ctx, cancel := context.WithCancel(context.Background())
	out := &lockedBuffer{}
	done := make(chan int)
	go func() { done <- Run(ctx, prog, RunOptions{Stdout: out, Stderr: out}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case code := <-done:
			if code != 0 {
				t.Errorf("Run returned %d:\n%s", code, out)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("Run didn't stop:\n%s", out)
		}
	})
	serving := regexp.MustCompile(`eye run: serving shop\.Shop on :(\d+)`)
	var port string
	waitFor(t, 10*time.Second, "the shop to listen", func() bool {
		if m := serving.FindStringSubmatch(out.String()); m != nil {
			port = m[1]
		}
		return port != ""
	})
	return "http://127.0.0.1:" + port, out
}

type shopReply struct {
	status int
	body   string
	header http.Header
}

func shopRequest(t *testing.T, method, url string, header map[string]string, body string) shopReply {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return shopReply{resp.StatusCode, strings.TrimSpace(string(data)), resp.Header}
}

// eye run serves a contract on its port: requests are routed by @http,
// decoded and encoded in the one JSON shape, checked against @auth, and run
// as one transaction each.
func TestServeContractOverHTTP(t *testing.T) {
	base, out := serveShop(t)
	auth := map[string]string{"X-Shop-Token": "s3cret"}
	with := func(extra map[string]string) map[string]string {
		h := map[string]string{"X-Shop-Token": "s3cret"}
		for k, v := range extra {
			h[k] = v
		}
		return h
	}
	cases := []struct {
		name, method, path string
		header             map[string]string
		body               string
		status             int
		reply              string // the reply's JSON, or a part of an error's text
	}{
		{"no credential", "POST", "/v1/orders", map[string]string{"Order-Key": "o1"}, `{"item": "pen"}`, 401, "X-Shop-Token"},
		{"a wrong credential", "GET", "/v1/items/a", map[string]string{"X-Shop-Token": "guess"}, "", 401, "X-Shop-Token"},
		{"a command, its status from status:", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o1"}), `{"item": "pen", "count": 2}`,
			201, `{"count":2,"kind":"Placed","order":"o1","repeated":false}`},
		{"the same key again finds its order", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o1"}), `{"item": "pen", "count": 2}`,
			201, `{"count":2,"kind":"Placed","order":"o1","repeated":true}`},
		{"a default filled in", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o2"}), `{"item": "ink"}`,
			201, `{"count":1,"kind":"Placed","order":"o2","repeated":false}`},
		{"another variant, another status", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o3"}), `{"item": "gone"}`,
			409, `{"item":"gone","kind":"OutOfStock"}`},
		{"the lowest of two statuses", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o4"}), `{"item": "pen", "count": 0}`,
			400, `{"kind":"Refused","reason":"count must be at least 1"}`},
		{"no key", "POST", "/v1/orders", auth, `{"item": "pen"}`, 400, "place takes a key, in the Order-Key header"},
		{"a wrong argument", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o5"}), `{"item": 3}`, 400, "place's argument item: expected a string"},
		{"a missing argument", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o5"}), `{}`, 400, "place's argument item: it is missing"},
		{"not JSON", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o5"}), `item=pen`, 400, "the body isn't JSON"},
		{"a fault answers 500", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o6"}), `{"item": "pen", "gift_note": "fault"}`,
			500, "a gift note that faults"},
		{"and changed nothing", "POST", "/v1/orders", with(map[string]string{"Order-Key": "o6"}), `{"item": "pen"}`,
			201, `{"count":1,"kind":"Placed","order":"o6","repeated":false}`},
		{"a query: path and repeated parameters", "GET", "/v1/items/a%2Fb?tags=x&tags=y", auth, "",
			200, `{"id":"a/b","name":"a thing","tags":["x","y"]}`},
		{"a query's default", "GET", "/v1/items/c", auth, "", 200, `{"id":"c","name":"a thing","tags":[]}`},
		{"an Option in the query", "GET", "/v1/items/c?limit=2", auth, "", 200, `{"id":"c","name":"two things","tags":[]}`},
		{"a path with another method", "DELETE", "/v1/items/c", auth, "", 405, "takes GET, not DELETE"},
		{"a path nothing answers", "GET", "/v1/internal", auth, "", 404, "shop.Shop has no operation at GET /v1/internal"},
	}
	for _, c := range cases {
		got := shopRequest(t, c.method, base+c.path, c.header, c.body)
		ok := got.status == c.status
		if strings.HasPrefix(c.reply, "{") {
			var a, b any
			_ = json.Unmarshal([]byte(got.body), &a)
			_ = json.Unmarshal([]byte(c.reply), &b)
			ab, _ := json.Marshal(a)
			bb, _ := json.Marshal(b)
			ok = ok && string(ab) == string(bb)
		} else {
			ok = ok && strings.Contains(got.body, c.reply)
		}
		if !ok {
			t.Errorf("%s: %s %s answered %d %s; want %d %s", c.name, c.method, c.path, got.status, got.body, c.status, c.reply)
		}
		if c.status == 405 && got.header.Get("Allow") != "GET" {
			t.Errorf("%s: Allow is %q, want GET", c.name, got.header.Get("Allow"))
		}
	}
	if !strings.Contains(out.String(), "eye run: shop.Shop.place faulted, answered 500") {
		t.Errorf("the fault wasn't reported; eye run said:\n%s", out)
	}
}

// A program that calls the contract over HTTP and the program that serves
// it read the same attributes, so they agree on every request and reply.
func TestCallServedContract(t *testing.T) {
	base, _ := serveShop(t)
	t.Setenv("SHOP_URL", base)
	client := New(loadFiles(t, map[string]string{"shop/shop.eye": shopContract, "main.eye": `
import shop.{ItemId}

job buy(key: Key) -> String {
  match shop.place!(ItemId("book"), count: 3, key:) {
    Placed(order, count, repeated) => "placed {order} {count} {repeated}"
    OutOfStock(_) => "out of stock"
    Refused(reason) => "refused: {reason}"
    Uncertain(_) => "uncertain"
  }
}

job soldOut() -> String {
  match shop.place!(ItemId("gone"), key: "c2") {
    OutOfStock(item) => "out of stock: {item == ItemId("gone")}"
    _ => "something else"
  }
}

job look() -> String {
  match shop.item!(ItemId("x y"), tags: ["a", "b"]) {
    Some(item) => "{item.id == ItemId("x y")} {item.tags}"
    None => "none"
  }
}
`}), io.Discard, io.Discard)
	for job, want := range map[string]string{
		"soldOut": `Completed("out of stock: true")`,
		"look":    `Completed("true [\"a\", \"b\"]")`,
	} {
		if got := runJob(t, client, job); got != want {
			t.Errorf("%s = %s, want %s", job, got, want)
		}
	}
	if got := runJob(t, client, "buy", "c1"); got != `Completed("placed c1 3 false")` {
		t.Errorf("buy = %s", got)
	}
	if got := runJob(t, client, "buy", "c1"); got != `Completed("placed c1 3 true")` {
		t.Errorf("buy again = %s", got)
	}
	// With the wrong credential, the service refuses the request, and the
	// run ends with a fault that shows the reply.
	t.Setenv("SHOP_TOKEN", "guess")
	if got := runJob(t, client, "buy", "c3"); !strings.Contains(got, "refused with status 401") {
		t.Errorf("buy with a wrong token = %s", got)
	}
}

// eye run doesn't start serving without the secret @auth checks, or on a
// port it can't have.
func TestServeNeedsItsSecretAndPort(t *testing.T) {
	prog := loadFiles(t, map[string]string{"shop/shop.eye": shopContract, "main.eye": shopServer})
	run := func() (int, string) {
		var out strings.Builder
		code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &out})
		return code, out.String()
	}
	t.Setenv("SHOP_PORT", "0")
	t.Setenv("SHOP_TOKEN", "") // restored when the test ends
	_ = os.Unsetenv("SHOP_TOKEN")
	t.Setenv("EYE_SECRETS", t.TempDir())
	if code, out := run(); code != 1 || !strings.Contains(out, `eye run: shop.Shop needs the secret "shop-token" to check its callers' @auth: set SHOP_TOKEN`) {
		t.Errorf("without its secret, Run returned %d:\n%s", code, out)
	}
	t.Setenv("SHOP_TOKEN", "s3cret")
	t.Setenv("SHOP_PORT", "eighty")
	if code, out := run(); code != 1 || !strings.Contains(out, `SHOP_PORT is "eighty"; it takes a port from 0 to 65535`) {
		t.Errorf("with a wrong SHOP_PORT, Run returned %d:\n%s", code, out)
	}
	taken, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	port := strconv.Itoa(taken.Addr().(*net.TCPAddr).Port)
	t.Setenv("SHOP_PORT", port)
	if code, out := run(); code != 1 || !strings.Contains(out, "eye run: can't serve shop.Shop on port "+port) {
		t.Errorf("on a port in use, Run returned %d:\n%s", code, out)
	}
}

// A request is answered while a run waits for another service, and the job
// it starts runs on the runner once the runner is free.
func TestServeWhileARunWaits(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	let := func() { once.Do(func() { close(release) }) }
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = io.WriteString(w, "late")
	}))
	defer slow.Close()
	defer let()
	t.Setenv("SLOW_URL", slow.URL)
	t.Setenv("BELL_PORT", "0")
	prog := loadFiles(t, map[string]string{
		"bell/bell.eye": `
@port(4200)
contract Bell {
  @http("POST /ring")
  @key(required)
  command ring(times: Int) -> Int
}
`,
		"main.eye": `
import bell
import http

entity Rung(key: Key) {
  state times: Int = 0

  pub fn set(n: Int) { times = n }
}

job wait() {
  let url = env.get!("SLOW_URL") ?? ""
  let _ = http.get!(url)
}

job echo(key: Key) {
  print("rang {key}")
}

impl bell.Bell {
  fn ring(times, key) {
    Rung(key).set(times)
    echo.start(key, key:)
    times * 2
  }
}

pub fn main() {
  wait.start(key: "wait")
}
`,
	})
	ctx, cancel := context.WithCancel(context.Background())
	out := &lockedBuffer{}
	done := make(chan int)
	go func() { done <- Run(ctx, prog, RunOptions{Stdout: out, Stderr: out}) }()
	defer func() {
		cancel()
		<-done
	}()
	serving := regexp.MustCompile(`eye run: serving bell\.Bell on :(\d+)`)
	var port string
	waitFor(t, 10*time.Second, "the bell to listen", func() bool {
		if m := serving.FindStringSubmatch(out.String()); m != nil {
			port = m[1]
		}
		return port != ""
	})
	got := shopRequest(t, "POST", "http://127.0.0.1:"+port+"/ring", map[string]string{"Idempotency-Key": "k1"}, `{"times": 2}`)
	if got.status != 200 || got.body != "4" {
		t.Errorf("ring answered %d %s, want 200 4", got.status, got.body)
	}
	if strings.Contains(out.String(), "rang k1") {
		t.Errorf("echo ran while the runner was in the middle of wait")
	}
	let()
	waitFor(t, 10*time.Second, "the job the request started to run", func() bool {
		return strings.Contains(out.String(), "rang k1")
	})
}
