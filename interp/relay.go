package interp

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"eyelang/internal/wire"
	"eyelang/ir"
)

// Events between programs (SPECS.md §17). A contract's events go from the
// program that serves the contract to the programs that consume them, as
// `POST /events/<topic>` with the event's value in the one JSON shape and the
// contract's @auth.
//
// Taking: eye run takes each event the program consumes (an @on(event(…))
// job, or next! on it) of a contract it doesn't serve, on the ports of the
// contracts it serves and on EYE_EVENTS_PORT. An event taken is published as
// the program's own would be, and answered 202 once it is in the journal.
//
// Sending: an event of a contract the program serves goes to each base URL
// in <MODULE>_EVENTS_URL. A delivery per subscriber is saved with the commit
// that publishes the event (Meta "outbox/<seq>"), and the runner sends it
// after the commit, again after no answer or a 5xx, in order per subscriber,
// until it is taken, refused (4xx) or 30 days old.

// relay holds what eye run sends to and takes from other programs. Tests
// have none: they send and take nothing.
type relay struct {
	// served are the contracts the program serves: only their events are
	// sent, and none of theirs is taken.
	served map[*ir.Contract]bool
	// subscribers, by the path of a contract's module, take its events in
	// this process, in place of <MODULE>_EVENTS_URL (RunOptions.Subscribers).
	subscribers map[string][]http.Handler
	// outbox holds the deliveries not yet done, in the order they were
	// saved; seq numbers them.
	outbox []*delivery
	seq    int64
	// arrived holds the events taken in this process by a run on a virtual
	// clock, which has no lock to take: the runner publishes them when it
	// next drains.
	mu      sync.Mutex
	arrived []pending
	// sent, when set, hears what each subscriber in this process answered.
	sent func(event, path string, status int)
}

// delivery is an event on its way to one subscriber, as the journal keeps it.
type delivery struct {
	seq   int64
	To    string          `json:"to"`    // the subscriber's base URL, or "#n" for the nth in this process
	Event string          `json:"event"` // eventName: notify.answered
	Body  json.RawMessage `json:"body"`  // the event's value in the one JSON shape
	At    int64           `json:"at"`    // when it was published, in Unix nanoseconds
	tries int             // how many times it was sent and not taken, since eye run started
	next  time.Time       // when to send it again
}

const outboxPrefix = "outbox/"

func outboxKey(seq int64) string { return fmt.Sprintf("%s%020d", outboxPrefix, seq) }

// keepDelivery is how long a delivery is tried: as long as a subscriber
// remembers an event's @key, so that a late repeat still starts nothing.
var keepDelivery = keepSeen

func newRelay(prog *ir.Program, subscribers map[string][]http.Handler) *relay {
	r := &relay{served: map[*ir.Contract]bool{}, subscribers: subscribers}
	for _, ct := range servedContracts(prog) {
		r.served[ct] = true
	}
	return r
}

// ---------------------------------------------------------------- taking

// takenEvents are the events a program takes from other services, by name:
// those it consumes, with an @on(event(…)) job or with next! (the consume
// effect its jobs have), of contracts it doesn't serve, and not of the
// standard library's contracts, whose events the runtime publishes itself.
// unguarded are those it would take but for their contract having no @auth:
// anyone who reaches the port could publish them.
func takenEvents(prog *ir.Program) (taken, unguarded []*ir.Event) {
	served := map[*ir.Contract]bool{}
	for _, ct := range servedContracts(prog) {
		served[ct] = true
	}
	byName := map[string][]*ir.Event{} // "Notify.answered"
	for _, m := range prog.Modules {
		for _, ct := range m.Contracts {
			for _, ev := range ct.Events {
				byName[ct.Name+"."+ev.Name] = append(byName[ct.Name+"."+ev.Name], ev)
			}
		}
	}
	found := map[*ir.Event]bool{}
	consumes := func(fn *ir.Func) {
		if fn == nil {
			return
		}
		for _, eff := range fn.Effects {
			if name, ok := strings.CutPrefix(eff, "consume "); ok {
				for _, ev := range byName[name] {
					found[ev] = true
				}
			}
		}
	}
	for _, m := range prog.Modules {
		for _, j := range m.Jobs {
			for _, t := range j.Triggers {
				if t.Event != nil {
					found[t.Event] = true
				}
			}
			consumes(j.Func)
		}
	}
	consumes(prog.Root.Main)
	for ev := range found {
		switch {
		case ev.Contract == nil || served[ev.Contract] || ev.Contract.Module.Dir == "":
		case ev.Contract.HTTP.AuthSecret == "":
			unguarded = append(unguarded, ev)
		default:
			taken = append(taken, ev)
		}
	}
	sort.Slice(taken, func(i, j int) bool { return eventName(taken[i]) < eventName(taken[j]) })
	sort.Slice(unguarded, func(i, j int) bool { return eventName(unguarded[i]) < eventName(unguarded[j]) })
	return taken, unguarded
}

// TakesEvents returns the paths of the modules whose contracts' events prog
// takes from other services: what a service in this process should send to
// it (RunOptions.Subscribers and Accept).
func TakesEvents(prog *ir.Program) map[string]bool {
	out := map[string]bool{}
	taken, _ := takenEvents(prog)
	for _, ev := range taken {
		out[ev.Contract.Module.Path] = true
	}
	return out
}

// eventsPort is the port EYE_EVENTS_PORT gives, where a program takes events
// besides the ports of the contracts it serves; ok is false when it is unset.
func eventsPort() (port int, ok bool, err error) {
	s, set := os.LookupEnv("EYE_EVENTS_PORT")
	if !set || s == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 {
		return 0, false, fmt.Errorf("EYE_EVENTS_PORT is %q; it takes a port from 0 to 65535, where 0 is any free one", s)
	}
	return n, true, nil
}

// eventNames lists events by name, for eye run's lines: "notify.answered, telegram.updates".
func eventNames(evs []*ir.Event) string {
	names := make([]string, len(evs))
	for i, ev := range evs {
		names[i] = eventName(ev)
	}
	return strings.Join(names, ", ")
}

// takenEvent is an event a program takes, with the credential its
// contract's @auth asks the publisher for.
type takenEvent struct {
	ev   *ir.Event
	auth string // the value @auth's header must have; "" without @auth
}

// eventRoutes makes the routes of the events a program takes, by path, with
// the secret each contract's @auth names, which must be set.
func (in *Interp) eventRoutes(evs []*ir.Event) (map[string]*takenEvent, error) {
	routes := map[string]*takenEvent{}
	for _, ev := range evs {
		t := &takenEvent{ev: ev}
		ct := ev.Contract
		if ct.HTTP.AuthSecret != "" {
			s, ok := in.lookupSecret(ct.HTTP.AuthSecret)
			if !ok {
				return nil, fmt.Errorf("%s needs the secret %q to check its publisher's @auth: set %s, or put a file %s in $EYE_SECRETS",
					eventName(ev), ct.HTTP.AuthSecret, envName(ct.HTTP.AuthSecret), ct.HTTP.AuthSecret)
			}
			if ct.HTTP.AuthBearer {
				s = "Bearer " + s
			}
			t.auth = s
		}
		routes[wire.EventPath(ev)] = t
	}
	return routes, nil
}

// noEvent answers a request on EYE_EVENTS_PORT that is for no event the
// program takes.
func noEvent(routes map[string]*takenEvent) http.Handler {
	paths := make([]string, 0, len(routes))
	for p := range routes {
		paths = append(paths, "POST "+p)
	}
	sort.Strings(paths)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("this program takes no event at %s %s; it takes %s", r.Method, r.URL.Path, strings.Join(paths, ", ")))
	})
}

// eventsHandler answers the requests that carry an event the program takes,
// and passes any other request to next.
func (in *Interp) eventsHandler(routes map[string]*takenEvent, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := routes[r.URL.EscapedPath()]
		if t == nil {
			next.ServeHTTP(w, r)
			return
		}
		ct := t.ev.Contract
		name := eventName(t.ev)
		if t.auth != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get(ct.HTTP.AuthHeader)), []byte(t.auth)) != 1 {
			how := "Bearer and the secret"
			if !ct.HTTP.AuthBearer {
				how = "the secret"
			}
			writeError(w, http.StatusUnauthorized, fmt.Sprintf("%s takes only events with %s in the %s header", name, how, ct.HTTP.AuthHeader))
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeError(w, http.StatusMethodNotAllowed, fmt.Sprintf("%s takes POST, not %s", r.URL.Path, r.Method))
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "the request's body couldn't be read: "+err.Error())
			return
		}
		if in.g == nil {
			// A run on a virtual clock, called in this process from inside
			// one of its own steps: the event waits for the runner.
			v, err := in.eventValue(t.ev, body)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			in.rt.relay.mu.Lock()
			in.rt.relay.arrived = append(in.rt.relay.arrived, pending{t.ev, v})
			in.rt.relay.mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			return
		}
		status, msg := in.takeEvent(t.ev, body)
		if status != http.StatusAccepted {
			writeError(w, status, msg)
			return
		}
		// The answer waits until the event is on disk, so the publisher
		// may forget it.
		if j := in.rt.journal; j != nil {
			if err := j.Sync(); err != nil {
				fmt.Fprintf(in.log, "eye run: %s couldn't be saved, answered 503: %v\n", name, err)
				writeError(w, http.StatusServiceUnavailable, "the event couldn't be saved: "+err.Error())
				return
			}
		}
		select {
		case in.rt.kick <- struct{}{}: // the runner runs the jobs it started
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

// eventValue reads an event's value from a request's body.
func (in *Interp) eventValue(ev *ir.Event, body []byte) (Value, error) {
	x, err := decodeJSON(body)
	if err != nil {
		return nil, fmt.Errorf("the body isn't JSON: %v", err)
	}
	if x == nil {
		return nil, fmt.Errorf("the body is empty; %s takes its value as JSON", eventName(ev))
	}
	v, err := in.fromWire(x, ev.Type, ev.Contract.HTTP.Snake)
	if err != nil {
		return nil, fmt.Errorf("the body isn't %s's value: %v", eventName(ev), err)
	}
	return v, nil
}

// takeEvent publishes an event taken from another program, holding the
// interpreter's lock: it is recorded, starts its jobs and wakes the runs
// waiting for it, which the runner runs. It returns 202, or the status and
// message of a refusal.
func (in *Interp) takeEvent(ev *ir.Event, body []byte) (status int, msg string) {
	in.g.mu.Lock()
	defer in.g.mu.Unlock()
	rt := in.rt
	if rt.stopped {
		return http.StatusServiceUnavailable, "the program is stopping"
	}
	v, err := in.eventValue(ev, body)
	if err != nil {
		return http.StatusBadRequest, err.Error()
	}
	if rt.real {
		if now := time.Now(); now.After(rt.now) {
			rt.now = now
		}
	}
	defer func() {
		if p := recover(); p != nil {
			switch x := p.(type) {
			case *Fault:
				fmt.Fprintf(in.log, "eye run: %s faulted, answered 500: %s\n", eventName(ev), x.Error())
				status, msg = http.StatusInternalServerError, eventName(ev)+" faulted: "+x.Msg
			case *CommitError:
				fmt.Fprintf(in.log, "eye run: %s couldn't be saved, answered 503: %v\n", eventName(ev), x)
				status, msg = http.StatusServiceUnavailable, eventName(ev)+" couldn't be saved: "+x.Error()
			default:
				panic(p)
			}
		}
	}()
	rt.deliver(ev, v)
	return http.StatusAccepted, ""
}

// takeArrived publishes the events taken in this process since the runner
// last looked, on a virtual clock.
func (rt *runtime) takeArrived() {
	r := rt.relay
	if r == nil {
		return
	}
	r.mu.Lock()
	arrived := r.arrived
	r.arrived = nil
	r.mu.Unlock()
	for _, a := range arrived {
		rt.deliver(a.event, a.value)
	}
}

// ---------------------------------------------------------------- sending

// subscribersOf are where the events of a contract module go: "#1", "#2", …
// for the handlers in this process, else the base URLs <MODULE>_EVENTS_URL
// lists, separated by commas or spaces.
func (r *relay) subscribersOf(module string) []string {
	if hs := r.subscribers[module]; len(hs) > 0 {
		out := make([]string, len(hs))
		for i := range hs {
			out[i] = "#" + strconv.Itoa(i+1)
		}
		return out
	}
	var out []string
	for _, u := range strings.FieldsFunc(os.Getenv(envName(module)+"_EVENTS_URL"), func(c rune) bool { return c == ',' || c == ' ' }) {
		out = append(out, strings.TrimSuffix(u, "/"))
	}
	return out
}

// addDeliveries adds to c a delivery of an event to each subscriber, when
// the event is of a contract the program serves, and returns them, to go in
// the outbox once c is committed.
func (rt *runtime) addDeliveries(c *Commit, e *ir.Event, v Value) []*delivery {
	r := rt.relay
	if r == nil || !r.served[e.Contract] {
		return nil
	}
	to := r.subscribersOf(e.Contract.Module.Path)
	if len(to) == 0 {
		return nil
	}
	body, err := json.Marshal(rt.in.toWire(v, e.Contract.HTTP.Snake))
	if err != nil {
		rt.in.faultf(ir.Pos{}, "%s's value can't be written as JSON: %v", eventName(e), err)
	}
	var out []*delivery
	for _, u := range to {
		r.seq++
		d := &delivery{seq: r.seq, To: u, Event: eventName(e), Body: body, At: rt.now.UnixNano()}
		data, _ := json.Marshal(d)
		c.Meta = append(c.Meta, MetaWrite{Key: outboxKey(d.seq), Data: data})
		out = append(out, d)
	}
	return out
}

// restoreOutbox reads back the deliveries not yet done.
func (rt *runtime) restoreOutbox() {
	r := rt.relay
	if r == nil {
		return
	}
	keys := rt.store.MetaKeys(outboxPrefix)
	sort.Strings(keys)
	for _, k := range keys {
		data, _ := rt.store.Meta(k)
		d := &delivery{}
		seq, err := strconv.ParseInt(strings.TrimPrefix(k, outboxPrefix), 10, 64)
		if err != nil || json.Unmarshal(data, d) != nil {
			continue
		}
		d.seq = seq
		r.outbox = append(r.outbox, d)
		r.seq = max(r.seq, seq)
	}
}

// nextDelivery is when the next delivery is due; ok is false when none waits.
func (r *relay) nextDelivery() (at time.Time, ok bool) {
	if r == nil {
		return time.Time{}, false
	}
	blocked := map[string]bool{}
	for _, d := range r.outbox {
		if blocked[d.To] {
			continue // after an earlier one to the same subscriber
		}
		blocked[d.To] = true
		if !ok || d.next.Before(at) {
			at, ok = d.next, true
		}
	}
	return at, ok
}

// sendEvents sends the deliveries that are due, each subscriber's in order:
// a delivery waits while the one before it to the same subscriber isn't
// done. On a virtual clock every delivery is due. The caller holds the
// interpreter's lock, if there is one, which is let go while a request is
// on its way.
func (in *Interp) sendEvents() {
	rt := in.rt
	r := rt.relay
	if r == nil || len(r.outbox) == 0 {
		return
	}
	rt.durable() // the subscriber sees the event
	blocked := map[string]bool{}
	for _, d := range append([]*delivery(nil), r.outbox...) {
		if blocked[d.To] {
			continue
		}
		if rt.real && rt.now.Before(d.next) {
			blocked[d.To] = true
			continue
		}
		if !in.send(d) {
			blocked[d.To] = true
		}
	}
}

// send sends one delivery and reports whether it is done: taken, refused or
// too old. One that wasn't taken is tried again later.
func (in *Interp) send(d *delivery) bool {
	rt := in.rt
	r := rt.relay
	var ev *ir.Event
	for _, m := range in.prog.Modules {
		for _, ct := range m.Contracts {
			for _, e := range ct.Events {
				if eventName(e) == d.Event {
					ev = e
				}
			}
		}
	}
	if ev == nil {
		// The program no longer declares it.
		in.dropDelivery(d, "the program no longer declares it")
		return true
	}
	ct := ev.Contract
	header := http.Header{"Content-Type": {"application/json"}}
	if ct.HTTP.AuthSecret != "" {
		s, ok := in.lookupSecret(ct.HTTP.AuthSecret)
		if !ok {
			in.retryDelivery(d, fmt.Sprintf("the secret %q isn't set: set %s, or put a file %s in $EYE_SECRETS", ct.HTTP.AuthSecret, envName(ct.HTTP.AuthSecret), ct.HTTP.AuthSecret))
			return false
		}
		if ct.HTTP.AuthBearer {
			s = "Bearer " + s
		}
		header.Set(ct.HTTP.AuthHeader, s)
	}
	client, target := eventClient, d.To+wire.EventPath(ev)
	if n, ok := strings.CutPrefix(d.To, "#"); ok {
		i, _ := strconv.Atoi(n)
		hs := r.subscribers[ct.Module.Path]
		if i < 1 || i > len(hs) {
			in.dropDelivery(d, "no subscriber "+d.To+" is in this process")
			return true
		}
		client, target = &http.Client{Transport: handlerTransport{hs[i-1]}}, "http://in-process"+wire.EventPath(ev)
	}
	status, reply, err := in.doRequestWith(client, http.MethodPost, target, header, d.Body)
	if r.sent != nil && err == nil && client != eventClient {
		r.sent(d.Event, wire.EventPath(ev), status)
	}
	switch {
	case err == nil && status >= 200 && status < 300:
		in.doneDelivery(d)
		return true
	case err == nil && status >= 400 && status < 500 && status != http.StatusRequestTimeout && status != http.StatusTooManyRequests:
		in.dropDelivery(d, fmt.Sprintf("it was refused with status %d: %s", status, strings.TrimSpace(string(reply))))
		return true
	case err != nil:
		in.retryDelivery(d, whyNoAnswer(d.To, "", err, 1))
	default:
		in.retryDelivery(d, fmt.Sprintf("it answered %d: %s", status, strings.TrimSpace(string(reply))))
	}
	return false
}

// eventClient sends events to other programs.
var eventClient = &http.Client{Timeout: 30 * time.Second}

// doneDelivery forgets a delivery the subscriber took.
func (in *Interp) doneDelivery(d *delivery) {
	r := in.rt.relay
	in.rt.store.Commit(&Commit{Meta: []MetaWrite{{Key: outboxKey(d.seq)}}})
	for i, o := range r.outbox {
		if o == d {
			r.outbox = append(r.outbox[:i:i], r.outbox[i+1:]...)
			break
		}
	}
}

// dropDelivery gives up on a delivery, saying why in the log.
func (in *Interp) dropDelivery(d *delivery, why string) {
	fmt.Fprintf(in.log, "eye run: %s wasn't delivered to %s, and is dropped: %s\n", d.Event, d.To, why)
	in.doneDelivery(d)
}

// retryDelivery notes a delivery that wasn't taken, to send it again after a
// wait that doubles from half a second to 30 seconds, and drops it once it
// is older than keepDelivery. It says so in the log when the waits reach
// their longest.
func (in *Interp) retryDelivery(d *delivery, why string) {
	rt := in.rt
	if rt.now.Sub(time.Unix(0, d.At)) > keepDelivery {
		in.dropDelivery(d, fmt.Sprintf("it wasn't taken in %v: %s", keepDelivery, why))
		return
	}
	d.tries++
	d.next = rt.now.Add(rt.retryDelay(min(d.tries, len(retryDelays)) - 1))
	if d.tries == len(retryDelays) {
		fmt.Fprintf(in.log, "eye run: %s isn't taken by %s: %s; trying again every %v\n", d.Event, d.To, why, retryDelays[len(retryDelays)-1])
	}
}
