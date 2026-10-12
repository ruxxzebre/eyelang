package interp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"eyelang/ir"
)

// The rules of SPECS.md §14 that a test in Eyelang can't watch: a fault
// fails the test it happens in, so these tests look at the store after one.

const accounts = `
entity Account(id: String) {
  state cents: Int = 0
  state moves: Int = 0

  pub fn topUp(n: Int) { cents += n }

  pub fn take(n: Int) {
    if cents < n { unreachable("{id} has {cents} cents, less than {n}") }
    cents -= n
  }

  ## Moves n cents to another account: it tops that one up and starts an
  ## audit before it takes from this one, which faults when there isn't
  ## enough.
  pub fn move(to: String, n: Int) {
    moves += 1
    Account(to).topUp(n)
    audit.start(id, n)
    take(n)
  }
}

entity Audit(id: String) {
  state lines: List(String) = []

  pub fn add(s: String) { lines += [s] }
}

job audit(from: String, n: Int) { Audit("log").add!("{from} moved {n}") }

job mover(from: String, to: String, n: Int) { Account(from).move!(to, n) }
`

// account returns an account's state and version, and whether it is saved.
func account(in *Interp, id string) (cents, moves any, version int64, saved bool) {
	ref := &EntityRef{Entity: in.prog.Root.Entities["Account"], ID: id}
	state, version, saved := in.rt.load(ref, ir.Pos{})
	cents, _ = state.get("cents")
	moves, _ = state.get("moves")
	return cents, moves, version, saved
}

// TestAFaultRollsBackEveryEntity faults part way through a call that
// changes two entities and starts a job, from a job's run and from outside
// one: neither entity changes, and the job never runs. An Err would have
// committed; only a fault rolls back.
func TestAFaultRollsBackEveryEntity(t *testing.T) {
	prog := load(t, accounts)
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	acct := prog.Root.Entities["Account"]
	in.rt.callEntity(&EntityRef{Entity: acct, ID: "a"}, acct.Methods["topUp"], []argVal{{v: int64(100)}}, ir.Pos{})

	check := func(when string) {
		t.Helper()
		if cents, moves, version, _ := account(in, "a"); cents != int64(100) || moves != int64(0) || version != 1 {
			t.Errorf("%s: a has %v cents, %v moves at version %d, want 100, 0 and 1", when, cents, moves, version)
		}
		if cents, _, _, saved := account(in, "b"); saved {
			t.Errorf("%s: b was saved, with %v cents", when, cents)
		}
		for _, id := range in.rt.store.Runs() {
			data, _ := in.rt.store.Run(id)
			if r := decodeRun(data); r.Job != ".mover" {
				t.Errorf("%s: run %d of %s exists, started by a call that rolled back", when, id, r.Job)
			}
		}
		state, _, _ := in.rt.load(&EntityRef{Entity: prog.Root.Entities["Audit"], ID: "log"}, ir.Pos{})
		if lines, _ := state.get("lines"); show(lines) != "[]" {
			t.Errorf("%s: the audit ran: %s", when, show(lines))
		}
	}

	// From a job: the run fails, and the step's transaction never commits.
	in.rt.startNow(prog.Root.Jobs["mover"], []Value{"a", "b", int64(500)}, nil, ir.Pos{})
	data, _ := in.rt.store.Run(1)
	if r := decodeRun(data); r.Status != "failed" || !strings.Contains(r.Fault, "a has 100 cents, less than 500") {
		t.Fatalf("the mover is %s with %q, want it failed for want of cents", r.Status, r.Fault)
	}
	in.rt.advance(ir.Duration(time.Hour))
	check("after the job's call faulted")

	// From outside a job, as main or a test calls it.
	func() {
		defer func() {
			if _, ok := recover().(*Fault); !ok {
				t.Error("the call didn't fault")
			}
		}()
		in.rt.callEntity(&EntityRef{Entity: acct, ID: "a"}, acct.Methods["move"], []argVal{{v: "b"}, {v: int64(500)}}, ir.Pos{})
	}()
	if in.rt.tx != nil {
		t.Error("the transaction is still open after the fault")
	}
	in.rt.advance(ir.Duration(time.Hour))
	check("after the direct call faulted")

	// The same call with enough cents commits both accounts and the audit.
	in.rt.callEntity(&EntityRef{Entity: acct, ID: "a"}, acct.Methods["move"], []argVal{{v: "b"}, {v: int64(30)}}, ir.Pos{})
	if cents, moves, version, _ := account(in, "a"); cents != int64(70) || moves != int64(1) || version != 2 {
		t.Errorf("a has %v cents, %v moves at version %d, want 70, 1 and 2", cents, moves, version)
	}
	if cents, _, version, _ := account(in, "b"); cents != int64(30) || version != 1 {
		t.Errorf("b has %v cents at version %d, want 30 and 1", cents, version)
	}
	state, _, _ := in.rt.load(&EntityRef{Entity: prog.Root.Entities["Audit"], ID: "log"}, ir.Pos{})
	if lines, _ := state.get("lines"); show(lines) != `["a moved 30"]` {
		t.Errorf("the audit has %s, want the one move that committed", show(lines))
	}
}

// faultOf runs f and returns the message of the fault it stops with, or "".
func faultOf(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			fault, ok := r.(*Fault)
			if !ok {
				panic(r)
			}
			msg = fault.Msg
		}
	}()
	f()
	return ""
}

// TestAnEndedEntityStaysEnded ends an entity in a journal's store. Calling
// it, reading it, or calling it from a job that kept its reference faults,
// saying when it ended and whose approval let it; so does a second call in
// the transaction that ends it, which rolls the end back. Reopened, the
// journal keeps only that it ended, and the identity is still ended.
func TestAnEndedEntityStaysEnded(t *testing.T) {
	prog := load(t, `
entity Player(id: String) {
  state name: String = ""

  pub fn rename(to: String) { name = to }

  pub fn retire(ok: Approval) { end(ok) }
}

entity Club(id: String) {
  ## Ends the player, then calls it again in the same transaction.
  pub fn retireThenRename(p: Ref(Player), ok: Approval) {
    p.retire(ok)
    p.rename("again")
  }
}

job renameLater(p: Ref(Player)) {
  sleep!(1 h)
  p.rename!("later")
}
`)
	path := filepath.Join(t.TempDir(), ".eye", "journal")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	in := New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.useStore(j)
	player, club := prog.Root.Entities["Player"], prog.Root.Entities["Club"]
	ann, bob := &EntityRef{Entity: player, ID: "ann"}, &EntityRef{Entity: player, ID: "bob"}
	in.rt.callEntity(ann, player.Methods["rename"], []argVal{{v: "Ann Smith"}}, ir.Pos{})
	in.rt.callEntity(bob, player.Methods["rename"], []argVal{{v: "Bob Jones"}}, ir.Pos{})
	ok := &ApprovalVal{What: "Retire ann?", By: "operator"}

	// Ending and calling again in one transaction faults, and nothing ends.
	msg := faultOf(func() {
		in.rt.callEntity(&EntityRef{Entity: club, ID: "c"}, club.Methods["retireThenRename"], []argVal{{v: ann}, {v: ok}}, ir.Pos{})
	})
	if !strings.Contains(msg, `Player("ann") ended earlier in this transaction`) {
		t.Errorf("a call after the end in the same transaction gave %q", msg)
	}
	if state, _, _ := in.rt.load(ann, ir.Pos{}); show(state) == "" {
		t.Error("ann can't be loaded after the end rolled back")
	}

	in.rt.startNow(prog.Root.Jobs["renameLater"], []Value{ann}, nil, ir.Pos{})
	in.rt.callEntity(ann, player.Methods["retire"], []argVal{{v: ok}}, ir.Pos{})
	want := `Player("ann") ended at 2026-01-01 00:00 UTC, approved by operator ("Retire ann?"), so it can't be read or changed`
	if msg := faultOf(func() { in.rt.callEntity(ann, player.Methods["rename"], []argVal{{v: "Ann"}}, ir.Pos{}) }); msg != want {
		t.Errorf("a call on the ended player gave %q, want %q", msg, want)
	}
	if msg := faultOf(func() { in.rt.snapshot(ann, ir.Pos{}) }); msg != want {
		t.Errorf("reading the ended player gave %q, want %q", msg, want)
	}
	in.rt.advance(ir.Duration(2 * time.Hour))
	data, _ := in.rt.store.Run(1)
	if r := decodeRun(data); r.Status != "failed" || !strings.Contains(r.Fault, want) {
		t.Errorf("the job that kept the reference is %s with %q, want it failed", r.Status, r.Fault)
	}
	if all := in.rt.all(player, ir.Pos{}); len(all.Elems) != 1 || !strings.Contains(show(all), "Bob") {
		t.Errorf("Player.all is %s, want only bob", show(all))
	}
	j.Close()

	again, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	row, saved := again.Load(entityKey(ann))
	if !saved || !row.Ended || strings.Contains(string(row.State), "Ann Smith") {
		t.Errorf("the journal keeps %q for ann (ended %v), want only that it ended", row.State, row.Ended)
	}
	in = New(prog, &strings.Builder{}, &strings.Builder{})
	in.rt.useStore(again)
	in.rt.restore()
	if msg := faultOf(func() { in.rt.callEntity(ann, player.Methods["rename"], []argVal{{v: "Ann"}}, ir.Pos{}) }); msg != want {
		t.Errorf("after the restart, a call on the ended player gave %q, want %q", msg, want)
	}
}

// TestOneWriterAtATimeOnTheSystemClock calls one entity's method from 200
// tasks on the system clock, where tasks are goroutines whose waits
// overlap: every call sees the count the one before it left, so none is
// lost and no two saw the same.
func TestOneWriterAtATimeOnTheSystemClock(t *testing.T) {
	prog := load(t, `
entity Counter(id: String) {
  state n: Int = 0

  pub fn bump() -> Int {
    n += 1
    n
  }
}

fn bumpAfter(ms: Int) -> Int {
  sleep(1 ms * ms)
  Counter("c").bump()
}

pub fn main() uses {entity Counter, stdio} {
  let seen = (1..=200) |> parallelMap(fn i: bumpAfter(i % 7))
  print(Counter("c").n, seen |> toSet |> size, Counter("c").read().version)
}
`)
	var out, errs strings.Builder
	if code := Run(context.Background(), prog, RunOptions{Stdout: &out, Stderr: &errs}); code != 0 {
		t.Fatalf("eye run exited with %d: %s", code, errs.String())
	}
	if got := strings.TrimSpace(out.String()); got != "200 200 200" {
		t.Errorf("count, distinct counts seen and version = %s, want 200 200 200", got)
	}
}
