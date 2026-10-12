// The Go side of the round trip test of eye gen go
// (cmd/eye/gen_roundtrip_test.go). The test builds it in a Go module of its
// own, beside the packages eye gen go wrote, and runs it against eye run
// serving the lab's Notify, a Miniflux written in Eyelang and the Echo
// contract. It sends every type of the lab's contracts through the generated
// clients and checks that each comes back as it went. It runs the Echo
// checks a second time through the generated server stub, which passes each
// call on to eye run with the client: so the stub reads and writes every
// type too.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"time"

	"roundtrip/echo"
	"roundtrip/miniflux"
	"roundtrip/notify"
)

var checks, failures int

func fail(format string, args ...any) {
	failures++
	fmt.Printf("FAIL: "+format+"\n", args...)
}

// same checks that a call gave want.
func same[T any](what string, got T, err error, want T) {
	checks++
	if err != nil {
		fail("%s: %v", what, err)
		return
	}
	if !reflect.DeepEqual(got, want) {
		g, _ := json.Marshal(got)
		w, _ := json.Marshal(want)
		fail("%s: got %s %#v, want %s %#v", what, g, got, w, want)
	}
}

// failsWith checks that a call failed with an error that is, or is like, want.
func failsWith[T any](what string, _ T, err error, want error) {
	checks++
	switch w := want.(type) {
	case *echo.Uncertain:
		var u *echo.Uncertain
		if !errors.As(err, &u) || *u != *w {
			fail("%s: got %v, want %v", what, err, want)
		}
	case *echo.HTTPError:
		var h *echo.HTTPError
		if !errors.As(err, &h) || *h != *w {
			fail("%s: got %#v, want %#v", what, err, want)
		}
	default:
		if !errors.Is(err, want) {
			fail("%s: got %v, want %v", what, err, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

var at = time.Date(2026, 10, 12, 7, 40, 2, 0, time.UTC)

func main() {
	ctx := context.Background()
	direct := &echo.EchoClient{BaseURL: os.Getenv("ECHO_URL"), Secret: os.Getenv("ECHO_TOKEN")}
	echoChecks(ctx, "eye run", direct)

	stub := httptest.NewServer(echo.NewEchoHandler(direct, "stub-secret"))
	defer stub.Close()
	echoChecks(ctx, "the Go stub", &echo.EchoClient{BaseURL: stub.URL, Secret: "stub-secret"})
	r, err := (&echo.EchoClient{BaseURL: stub.URL, Secret: "guess"}).Priority(ctx, echo.PriorityArgs{Value: echo.PriorityHigh})
	failsWith("the Go stub with a wrong secret", r, err, &echo.HTTPError{Op: "echo.priority", Status: 401, Body: "{\"error\":\"echo.Echo takes only requests with Bearer and the secret in the Authorization header\"}\n"})

	// A command that never reached its service can be sent again: it isn't
	// Uncertain.
	_, err = (&echo.EchoClient{BaseURL: "http://127.0.0.1:1", Secret: "x"}).Grade(ctx, echo.GradeArgs{Score: 1})
	var u *echo.Uncertain
	checks++
	if err == nil || errors.As(err, &u) {
		fail("a command to a closed port: got %v, want the error that stopped it", err)
	}

	for _, r := range rawRequests {
		s1, b1 := r.send(os.Getenv("ECHO_URL"), os.Getenv("ECHO_TOKEN"))
		s2, b2 := r.send(stub.URL, "stub-secret")
		checks++
		if s1 != s2 || !alike(s1, b1, b2) {
			fail("%s %s answered %d %s from eye run, but %d %s from the Go stub", r.method, r.path, s1, b1, s2, b2)
		}
	}

	minifluxChecks(ctx)
	notifyChecks(ctx)
	if failures > 0 {
		os.Exit(1)
	}
	fmt.Printf("ok: %d checks\n", checks)
}

// rawRequest is a request written by hand, which eye run and the stub must
// answer alike: defaults filled in, refusals, routing.
type rawRequest struct {
	method, path, body, key string
}

var rawRequests = []rawRequest{
	{method: "POST", path: "/defaults", body: `{"given": "x"}`},
	{method: "POST", path: "/defaults", body: `{"given": "x", "note": null, "count": 9}`},
	{method: "POST", path: "/defaults", body: `{}`},
	{method: "POST", path: "/defaults", body: `[1]`},
	{method: "POST", path: "/defaults", body: `{"given": null}`},
	{method: "GET", path: "/defaults"},
	{method: "GET", path: "/nowhere"},
	{method: "POST", path: "/grade", body: `{"score": 80}`},
	{method: "POST", path: "/grade", body: `{"score": 20}`},
	{method: "POST", path: "/shelves/s1/clear", body: `{"basedOn": 1, "approval": {"what": "clear", "by": "operator"}}`},
	{method: "POST", path: "/shelves/s1/clear", body: `{"basedOn": 1, "approval": {"what": "clear", "by": "operator"}}`, key: "k-raw"},
	{method: "GET", path: "/ids/5?user=operator&priority=Low&tags=a&tags=b&at=2026-10-12T07:40:02Z"},
	{method: "GET", path: "/ids/5?user=operator&priority=Low&tags=a&limit=3&at=2026-10-12T07:40:02Z"},
	{method: "GET", path: "/ids/x?user=operator&priority=Low&tags=a&at=2026-10-12T07:40:02Z"},
	{method: "GET", path: "/page?status=503"},
	{method: "POST", path: "/priority", body: `{"value": "Urgent"}`},
	{method: "POST", path: "/update-body", body: `{"value": {"kind": "Text", "value": "hi"}}`},
	{method: "POST", path: "/update-body", body: `{"value": {"kind": "Shout"}}`},
}

// alike reports whether two replies of one status say the same: a result as
// the same JSON, whatever the order of its fields; a refusal of a request
// (400) as an error, which the stub words in its own way when a value has
// the wrong shape; any other refusal in the same words.
func alike(status int, a, b string) bool {
	var x, y any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return a == b
	}
	if status == 400 {
		ex, _ := x.(map[string]any)
		ey, _ := y.(map[string]any)
		_, okx := ex["error"].(string)
		_, oky := ey["error"].(string)
		return okx && oky
	}
	return reflect.DeepEqual(x, y)
}

func (r rawRequest) send(base, secret string) (int, string) {
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	req, err := http.NewRequest(r.method, base+r.path, body)
	if err != nil {
		return 0, err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	if r.key != "" {
		req.Header.Set("Idempotency-Key", r.key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(data)
}

// echoChecks sends a value of every type through Echo, which sends it back.
func echoChecks(ctx context.Context, where string, c *echo.EchoClient) {
	w := func(what string) string { return where + ": " + what }
	{
		v, err := c.Priority(ctx, echo.PriorityArgs{Value: echo.PriorityHigh})
		same(w("Priority"), v, err, echo.PriorityHigh)
	}
	{
		v, err := c.Format(ctx, echo.FormatArgs{Value: echo.FormatMarkdown})
		same(w("Format"), v, err, echo.FormatMarkdown)
	}
	{
		v, err := c.MessageID(ctx, echo.MessageIDArgs{Value: "m-1"})
		same(w("MessageId"), v, err, "m-1")
	}
	for _, d := range []echo.Delivery{
		echo.Accepted{ID: "m-1", Deliveries: 2, Deduped: true},
		echo.DeliveryPending{ActionID: "a-1"},
		echo.Refused{Error: "nobody receives digest.ireland"},
	} {
		v, err := c.Delivery(ctx, echo.DeliveryArgs{Value: d})
		same(w("Delivery"), v, err, d)
	}
	{
		v, err := c.Refusal(ctx, echo.RefusalArgs{Value: echo.RefusalBadChoices})
		same(w("Refusal"), v, err, echo.RefusalBadChoices)
	}
	{
		want := echo.Choice{ID: "yes", Label: "Yes, take it"}
		v, err := c.Choice(ctx, echo.ChoiceArgs{Value: want})
		same(w("Choice"), v, err, want)
	}
	{
		v, err := c.QuestionID(ctx, echo.QuestionIDArgs{Value: "q-1"})
		same(w("QuestionId"), v, err, "q-1")
	}
	for _, a := range []echo.Asked{echo.Asking{Value: "q-1"}, echo.NotAsked{Value: echo.RefusalNotAllowed}} {
		v, err := c.Asked(ctx, echo.AskedArgs{Value: a})
		same(w("Asked"), v, err, a)
	}
	{
		want := echo.Answered{Question: "q-1", Choice: "yes", By: "operator", At: at}
		v, err := c.Answered(ctx, echo.AnsweredArgs{Value: want})
		same(w("Answered"), v, err, want)
	}
	{
		v, err := c.UserID(ctx, echo.UserIDArgs{Value: "operator"})
		same(w("UserId"), v, err, "operator")
	}
	{
		v, err := c.CategoryID(ctx, echo.CategoryIDArgs{Value: 7})
		same(w("CategoryId"), v, err, 7)
	}
	feed := echo.Feed{Title: "News"}
	{
		v, err := c.Feed(ctx, echo.FeedArgs{Value: feed})
		same(w("Feed"), v, err, feed)
	}
	entry := echo.Entry{Title: "Ireland", URL: "https://news.example/1", PublishedAt: at, Content: "<p>Hello & \"bye\"</p>", Feed: feed}
	{
		v, err := c.Entry(ctx, echo.EntryArgs{Value: entry})
		same(w("Entry"), v, err, entry)
	}
	{
		want := echo.Entries{Total: 1, Entries: []echo.Entry{entry}}
		v, err := c.Entries(ctx, echo.EntriesArgs{Value: want})
		same(w("Entries"), v, err, want)
		// A nil slice goes as an empty list, which Eyelang reads.
		v, err = c.Entries(ctx, echo.EntriesArgs{Value: echo.Entries{}})
		same(w("Entries with a nil slice"), v, err, echo.Entries{Entries: []echo.Entry{}})
	}
	{
		want := echo.User{ID: 1, Username: "lab"}
		v, err := c.User(ctx, echo.UserArgs{Value: want})
		same(w("User"), v, err, want)
	}
	{
		v, err := c.Refreshed(ctx, echo.RefreshedArgs{Value: echo.RefreshedRefreshed})
		same(w("Refreshed"), v, err, echo.RefreshedRefreshed)
	}
	category := echo.Category{ID: 7, Title: "News"}
	{
		v, err := c.Category(ctx, echo.CategoryArgs{Value: category})
		same(w("Category"), v, err, category)
	}
	{
		want := echo.Subscription{FeedURL: "https://news.example/feed", Title: "News", Category: category, ParsingErrorCount: 2, ParsingErrorMessage: "timeout"}
		v, err := c.Subscription(ctx, echo.SubscriptionArgs{Value: want})
		same(w("Subscription"), v, err, want)
	}
	for _, s := range []echo.Subscribed{echo.Added{FeedID: 42}, echo.NotAdded{ErrorMessage: "This feed already exists."}} {
		v, err := c.Subscribed(ctx, echo.SubscribedArgs{Value: s})
		same(w("Subscribed"), v, err, s)
	}
	{
		v, err := c.ChatID(ctx, echo.ChatIDArgs{Value: -100123})
		same(w("ChatId"), v, err, -100123)
	}
	{
		v, err := c.UpdateID(ctx, echo.UpdateIDArgs{Value: 6})
		same(w("UpdateId"), v, err, 6)
	}
	{
		v, err := c.MessageRef(ctx, echo.MessageRefArgs{Value: 7})
		same(w("MessageRef"), v, err, 7)
	}
	{
		want := echo.Button{ID: "no", Label: "Not taking it"}
		v, err := c.Button(ctx, echo.ButtonArgs{Value: want})
		same(w("Button"), v, err, want)
	}
	bodies := []echo.UpdateBody{
		echo.Command{Name: "start", Args: "now"},
		echo.Pressed{Message: 7, Button: "yes"},
		echo.Text{Value: "hello"},
	}
	for _, b := range bodies {
		v, err := c.UpdateBody(ctx, echo.UpdateBodyArgs{Value: b})
		same(w("UpdateBody"), v, err, b)
	}
	for _, u := range []echo.Update{
		{ID: 1, Chat: 5, Private: true, From: ptr(echo.ChatID(5)), SentAt: at, Body: bodies[1]},
		{ID: 2, Chat: -100123, Private: false, From: nil, SentAt: at, Body: bodies[2]},
	} {
		v, err := c.Update(ctx, echo.UpdateArgs{Value: u})
		same(w("Update"), v, err, u)
	}
	for _, s := range []echo.Sending{echo.Posted{Value: 7}, echo.TooFast{RetryAfter: echo.Duration(1500 * time.Millisecond)}, echo.Blocked{}} {
		v, err := c.Sending(ctx, echo.SendingArgs{Value: s})
		same(w("Sending"), v, err, s)
	}
	{
		want := echo.Person{ID: "operator", Email: "op@lab.example", Active: true, Groups: []string{"admins", "lab"}}
		v, err := c.Person(ctx, echo.PersonArgs{Value: want})
		same(w("Person"), v, err, want)
	}
	{
		want := echo.Proxy{URL: "socks5://vpn:1080"}
		v, err := c.Proxy(ctx, echo.ProxyArgs{Value: want})
		same(w("Proxy"), v, err, want)
	}
	solution := echo.Solution{URL: "https://news.example", Status: 200, Response: "<html></html>"}
	{
		v, err := c.Solution(ctx, echo.SolutionArgs{Value: solution})
		same(w("Solution"), v, err, solution)
	}
	for _, s := range []echo.Solved{{Status: "ok", Message: "", Solution: &solution}, {Status: "error", Message: "timed out"}} {
		v, err := c.Solved(ctx, echo.SolvedArgs{Value: s})
		same(w("Solved"), v, err, s)
	}
	{
		v, err := c.TrainCode(ctx, echo.TrainCodeArgs{Value: "E101"})
		same(w("TrainCode"), v, err, "E101")
	}
	{
		v, err := c.StopID(ctx, echo.StopIDArgs{Value: "PERSE"})
		same(w("StopId"), v, err, "PERSE")
	}
	{
		want := echo.Movement{
			Train:      "E101",
			Date:       "2026-10-12",
			Departures: map[echo.StopID]time.Time{"PERSE": at, "TARA": at.Add(5 * time.Minute)},
			Arrivals:   map[echo.StopID]time.Time{},
			Delay:      echo.Duration(2 * time.Minute),
			Cancelled:  true,
		}
		v, err := c.Movement(ctx, echo.MovementArgs{Value: want})
		same(w("Movement"), v, err, want)
	}
	{
		v, err := c.PeerUserID(ctx, echo.PeerUserIDArgs{Value: "u-1"})
		same(w("PeerUserId"), v, err, "u-1")
	}
	{
		want := echo.PeerUser{ID: "u-1", Email: "guest@lab.example", Groups: []string{"guests"}, Service: false, Blocked: true}
		v, err := c.PeerUser(ctx, echo.PeerUserArgs{Value: want})
		same(w("PeerUser"), v, err, want)
	}
	{
		want := echo.NetbirdPending{ID: "u-2", Email: "new@lab.example", JoinedAt: at}
		v, err := c.Pending(ctx, echo.PendingArgs{Value: want})
		same(w("Pending"), v, err, want)
	}
	{
		v, err := c.Change(ctx, echo.ChangeArgs{Value: echo.ChangeNoSuchUser})
		same(w("Change"), v, err, echo.ChangeNoSuchUser)
	}
	for _, version := range []string{`42`, `"v7"`} {
		v, err := c.Version(ctx, echo.VersionArgs{Value: echo.Version(version)})
		same(w("Version"), string(v), err, version)
	}
	{
		want := echo.Approval{What: "burn shelf 3", By: "operator"}
		v, err := c.Approval(ctx, echo.ApprovalArgs{Value: want})
		same(w("Approval"), v, err, want)
	}

	// Arguments in the path and the query.
	{
		args := echo.IdsArgs{Chat: -100123, User: "operator", Priority: echo.PriorityLow, Tags: []string{"a b", "c/d", "e&f"}, Limit: ptr(int64(10)), At: at}
		v, err := c.Ids(ctx, args)
		same(w("Ids"), v, err, echo.Ids{Chat: args.Chat, User: args.User, Priority: args.Priority, Tags: args.Tags, Limit: args.Limit, At: at})
		args.Limit, args.Tags = nil, []string{"only"}
		v, err = c.Ids(ctx, args)
		same(w("Ids without a limit"), v, err, echo.Ids{Chat: args.Chat, User: args.User, Priority: args.Priority, Tags: args.Tags, At: at})
	}
	// Arguments left out take their defaults; the client fills them in.
	{
		v, err := c.Defaults(ctx, echo.DefaultsArgs{Given: "x"})
		same(w("Defaults left out"), v, err, echo.Defaults{Given: "x", Priority: echo.PriorityHigh, Tags: []string{"a", "b"}, Count: 3})
		args := echo.DefaultsArgs{Given: "y", Priority: ptr(echo.PriorityLow), Tags: &[]string{}, Note: ptr("a note"), Count: ptr(int64(0))}
		v, err = c.Defaults(ctx, args)
		same(w("Defaults given"), v, err, echo.Defaults{Given: "y", Priority: echo.PriorityLow, Tags: []string{}, Note: ptr("a note"), Count: 0})
	}
	// Result(T, http.HttpError): an error the caller handles.
	{
		v, err := c.Page(ctx, echo.PageArgs{Status: 200})
		same(w("Page"), v, err, "the page")
		v, err = c.Page(ctx, echo.PageArgs{Status: 503})
		failsWith(w("Page that is paused"), v, err, &echo.HTTPError{Op: "echo.page", Status: 503, Body: "paused"})
	}
	// Statuses that `status:` maps to variants.
	{
		v, err := c.Grade(ctx, echo.GradeArgs{Score: 80})
		same(w("Grade 80"), v, err, echo.Grade(echo.Good{Score: 80}))
		v, err = c.Grade(ctx, echo.GradeArgs{Score: 20})
		same(w("Grade 20"), v, err, echo.Grade(echo.Poor{Score: 20, Reason: "below 50"}))
	}
	// A fresh and destructive command, and its outcomes.
	{
		approval := echo.Approval{What: "clear shelf s3", By: "operator"}
		v, err := c.Clear(ctx, echo.ClearArgs{ID: "s3", BasedOn: echo.Version(`5`), Approval: approval}, "k-1")
		same(w("Clear"), v, err, echo.Cleared{ID: "s3", By: "operator", What: "clear shelf s3", Key: "k-1"})
		v, err = c.Clear(ctx, echo.ClearArgs{ID: "s3", BasedOn: echo.Version(`1`), Approval: approval}, "k-2")
		failsWith(w("Clear that is stale"), v, err, echo.ErrStale)
		v, err = c.Clear(ctx, echo.ClearArgs{ID: "s3", BasedOn: echo.Version(`2`), Approval: approval}, "k-3")
		failsWith(w("Clear that expired"), v, err, echo.ErrExpired)
	}
	// A command whose impl faults: its reply is lost, so it is Uncertain.
	{
		v, err := c.Fail(ctx, echo.FailArgs{Why: "on purpose"}, "k-9")
		failsWith(w("Fail"), v, err, &echo.Uncertain{Op: "echo.fail", Key: "k-9"})
	}
}

// minifluxChecks calls the Miniflux written in Eyelang, whose answers show
// what each request carried.
func minifluxChecks(ctx context.Context) {
	c := &miniflux.MinifluxClient{BaseURL: os.Getenv("MINIFLUX_URL"), Secret: os.Getenv("MINIFLUX_KEY")}
	{
		v, err := c.Me(ctx)
		same("miniflux.me", v, err, miniflux.User{ID: 1, Username: "lab"})
	}
	entry := func(title, url string) miniflux.Entries {
		return miniflux.Entries{Total: 5, Entries: []miniflux.Entry{{Title: title, URL: url, PublishedAt: time.Date(2026, 10, 12, 7, 40, 0, 0, time.UTC), Content: "<p>Hello</p>", Feed: miniflux.Feed{Title: "News"}}}}
	}
	{
		v, err := c.Entries(ctx, miniflux.EntriesArgs{Category: 3, PublishedAfter: 1760000000, Limit: 5})
		same("miniflux.entries with defaults", v, err, entry("3 1760000000 unread,read published_at desc", "https://news.example/5"))
		v, err = c.Entries(ctx, miniflux.EntriesArgs{Category: 3, PublishedAfter: 1760000000, Limit: 5, Status: &[]string{"unread"}, Direction: ptr("asc")})
		same("miniflux.entries", v, err, entry("3 1760000000 unread published_at asc", "https://news.example/5"))
	}
	{
		v, err := c.Categories(ctx)
		same("miniflux.categories", v, err, []miniflux.Category{{ID: 1, Title: "News"}})
	}
	{
		v, err := c.CreateCategory(ctx, miniflux.CreateCategoryArgs{Title: "Lab"})
		same("miniflux.createCategory", v, err, miniflux.Category{ID: 7, Title: "Lab"})
	}
	{
		v, err := c.Feeds(ctx)
		same("miniflux.feeds", v, err, []miniflux.Subscription{{FeedURL: "https://news.example/feed", Title: "News", Category: miniflux.Category{ID: 1, Title: "News"}}})
	}
	{
		v, err := c.Subscribe(ctx, miniflux.SubscribeArgs{FeedURL: "https://news.example/feed", CategoryID: 1})
		same("miniflux.subscribe, 400", v, err, miniflux.Subscribed(miniflux.NotAdded{ErrorMessage: "This feed already exists."}))
		v, err = c.Subscribe(ctx, miniflux.SubscribeArgs{FeedURL: "https://other.example/feed", CategoryID: 1})
		same("miniflux.subscribe, 201", v, err, miniflux.Subscribed(miniflux.Added{FeedID: 42}))
	}
	{
		v, err := c.Refresh(ctx)
		same("miniflux.refresh, 204", v, err, miniflux.RefreshedRefreshed)
	}
}

// notifyChecks sends through the lab's notifier.
func notifyChecks(ctx context.Context) {
	c := &notify.NotifyClient{BaseURL: os.Getenv("NOTIFY_URL"), Secret: os.Getenv("NOTIFY_TOKEN")}
	args := notify.SendArgs{Event: "digest.ireland", Title: "Hello", Body: "From Go", To: &[]notify.UserID{"operator"}}
	v, err := c.Send(ctx, args, "roundtrip-go")
	same("notify.send", v, err, notify.Delivery(notify.Accepted{ID: "roundtrip-go", Deliveries: 1, Deduped: false}))
	v, err = c.Send(ctx, args, "roundtrip-go")
	same("notify.send again", v, err, notify.Delivery(notify.Accepted{ID: "roundtrip-go", Deliveries: 1, Deduped: true}))
	args.To = &[]notify.UserID{}
	v, err = c.Send(ctx, args, "roundtrip-go-nobody")
	same("notify.send to nobody, 400", v, err, notify.Delivery(notify.Refused{Error: "nobody receives digest.ireland"}))
}
