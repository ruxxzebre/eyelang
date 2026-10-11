package interp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"eyelang/check"
)

// testPostgres gives a test a schema of its own in the database
// $EYE_TEST_POSTGRES names, dropped when the test ends, and returns the
// address of the store in it. Without the variable the test is skipped.
func testPostgres(t *testing.T) string {
	t.Helper()
	base := os.Getenv("EYE_TEST_POSTGRES")
	if base == "" {
		t.Skip("set EYE_TEST_POSTGRES to a Postgres address to test the Postgres store")
	}
	return newSchema(t, base)
}

// newSchema makes a schema in the database at base, dropped when the test
// ends, and returns the address of a store in it.
func newSchema(t *testing.T, base string) string {
	t.Helper()
	var b [6]byte
	_, _ = rand.Read(b[:])
	schema := "eyetest_" + hex.EncodeToString(b[:])
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := pgx.Connect(ctx, base)
		if err != nil {
			return
		}
		defer conn.Close(ctx)
		_, _ = conn.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	})
	return withSearchPath(base, schema)
}

// withSearchPath returns the address url with schema as its search_path.
func withSearchPath(url, schema string) string {
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	return url + sep + "search_path=" + schema
}

func openPostgres(t *testing.T, url string) *Postgres {
	t.Helper()
	p, err := OpenPostgres(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

func TestPostgresSurvivesReopening(t *testing.T) {
	url := testPostgres(t)
	p := openPostgres(t, url)
	p.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("a"), Version: 1, Schema: 1}}})
	p.Commit(&Commit{
		Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("b"), Version: 2, Schema: 1}},
		Runs:   []RunWrite{{ID: 1, Data: []byte(`{"ID":1}`)}},
		Events: []EventRecord{{Name: "m.ev", Data: []byte("x")}},
		Meta:   []MetaWrite{{Key: "fired/m.job", Data: []byte("7")}},
	})

	if _, err := OpenPostgres(url); err == nil || !strings.Contains(err.Error(), "in use by another eye run") {
		t.Errorf("a second runner opened the store: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	again := openPostgres(t, url)
	if r, _ := again.Load("m.E/1"); string(r.State) != "b" || r.Version != 2 || r.Schema != 1 {
		t.Errorf("entity = %q v%d schema %d, want \"b\" v2 schema 1", r.State, r.Version, r.Schema)
	}
	if ids := again.Runs(); len(ids) != 1 || ids[0] != 1 {
		t.Errorf("runs = %v", ids)
	}
	if evs := again.Events(); len(evs) != 1 || evs[0].Name != "m.ev" {
		t.Errorf("events = %v", evs)
	}
	if d, ok := again.Meta("fired/m.job"); !ok || string(d) != "7" {
		t.Errorf("meta = %q, %v", d, ok)
	}

	read, err := ReadPostgres(url)
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := read.Load("m.E/1"); string(r.State) != "b" {
		t.Errorf("ReadPostgres sees %q, want \"b\"", r.State)
	}
}

// Every store keeps what the same commits wrote, in the same order.
func TestStoresAgree(t *testing.T) {
	commits := []*Commit{
		{Writes: []Write{{Key: "m.B/1", Entity: "m.B", State: []byte("b1"), Version: 1, Schema: 1}}},
		{
			Writes: []Write{
				{Key: "m.A/2", Entity: "m.A", State: []byte("a2"), Version: 1, Schema: 2},
				{Key: "m.A/1", Entity: "m.A", State: []byte("a1"), Version: 1, Schema: 2},
			},
			Runs:   []RunWrite{{ID: 1, Data: []byte("r1")}, {ID: 2, Data: []byte("r2")}},
			Events: []EventRecord{{Name: "e", Data: []byte("1")}, {Name: "e", Data: []byte("2")}, {Name: "f", Data: []byte("3")}},
			Meta:   []MetaWrite{{Key: "seen/x", Data: []byte("1")}, {Key: "fired/j", Data: []byte("9")}},
		},
		{
			Writes:     []Write{{Key: "m.A/2", Entity: "m.A", State: []byte("a2'"), Version: 2, Schema: 2}},
			Runs:       []RunWrite{{ID: 1, Data: []byte("r1'")}},
			Events:     []EventRecord{{Name: "g", Data: []byte("4")}},
			Meta:       []MetaWrite{{Key: "seen/x"}},
			DropEvents: 2,
		},
	}
	stores := map[string]Store{"memory": newMemStore()}
	j, err := OpenJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	stores["journal"] = j
	if os.Getenv("EYE_TEST_POSTGRES") != "" {
		stores["postgres"] = openPostgres(t, testPostgres(t))
	}
	for _, c := range commits {
		for _, s := range stores {
			s.Commit(c)
		}
	}
	want := dumpStore(stores["memory"])
	for name, s := range stores {
		if got := dumpStore(s); got != want {
			t.Errorf("%s holds\n%s\nwant\n%s", name, got, want)
		}
	}
	// What was written comes back the same after reopening.
	if p, ok := stores["postgres"].(*Postgres); ok {
		url := p.url
		p.Close()
		again := openPostgres(t, url)
		if got := dumpStore(again); got != want {
			t.Errorf("reopened postgres holds\n%s\nwant\n%s", got, want)
		}
	}
}

// dumpStore writes out what a store holds, entity types in order.
func dumpStore(s Store) string {
	var b strings.Builder
	for _, e := range []string{"m.A", "m.B"} {
		for _, k := range s.Keys(e) {
			r, _ := s.Load(k)
			b.WriteString(k + "=" + string(r.State) + " v" + itoa(r.Version) + " s" + itoa(int64(r.Schema)) + "\n")
		}
	}
	for _, id := range s.Runs() {
		d, _ := s.Run(id)
		b.WriteString("run " + itoa(id) + "=" + string(d) + "\n")
	}
	ev, _ := json.Marshal(s.Events())
	b.WriteString("events " + string(ev) + "\n")
	for _, k := range s.MetaKeys("") {
		d, _ := s.Meta(k)
		b.WriteString("meta " + k + "=" + string(d) + "\n")
	}
	return b.String()
}

// A commit while the database can't be reached changes nothing; once it can
// be, the store connects again and carries on.
func TestPostgresCommitWhileTheDatabaseIsDown(t *testing.T) {
	url := testPostgres(t)
	p := openPostgres(t, url)
	p.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("a"), Version: 1}}})

	p.drop()
	p.url = "postgres://eye:eye@127.0.0.1:1/eye" // nothing listens there
	err := guard(func() {
		p.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("b"), Version: 2}}})
	})
	ce, ok := err.(*CommitError)
	if !ok || ce.Fatal {
		t.Fatalf("commit with the database down: %v, want a CommitError to retry", err)
	}
	if r, _ := p.Load("m.E/1"); string(r.State) != "a" {
		t.Errorf("after the failed commit the entity is %q, want \"a\"", r.State)
	}

	p.url = url
	p.Commit(&Commit{Writes: []Write{{Key: "m.E/1", Entity: "m.E", State: []byte("c"), Version: 2}}})
	p.Close()
	again := openPostgres(t, url)
	if r, _ := again.Load("m.E/1"); string(r.State) != "c" || r.Version != 2 {
		t.Errorf("after reconnecting the entity is %q v%d, want \"c\" v2", r.State, r.Version)
	}
}

// When the store can't know that memory matches the database, it stops
// taking commits, so a restart reads the database again.
func TestPostgresStopsWhenMemoryMayNotMatch(t *testing.T) {
	t.Run("another runner wrote", func(t *testing.T) {
		url := testPostgres(t)
		p := openPostgres(t, url)
		p.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("1")}}})
		p.drop() // the lock goes with the connection, as when the database restarts
		other := openPostgres(t, url)
		other.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("2")}}})
		other.Close()

		err := guard(func() { p.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("3")}}}) })
		if ce, ok := err.(*CommitError); !ok || !ce.Fatal || !strings.Contains(ce.Error(), "another runner") {
			t.Fatalf("commit after another runner wrote: %v, want a fatal CommitError", err)
		}
		if d, _ := p.Meta("k"); string(d) != "1" {
			t.Errorf("memory holds %q, want what this runner wrote, \"1\"", d)
		}
	})
	t.Run("a lost reply arrived", func(t *testing.T) {
		url := testPostgres(t)
		p := openPostgres(t, url)
		p.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("1")}}})
		// The next commit reached the database, but its reply was lost.
		ctx := context.Background()
		tx, err := p.conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := pgApply(ctx, tx, &Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("2")}}}, p.commits); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		p.unsure = true
		p.drop()

		err = guard(func() { p.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("3")}}}) })
		if ce, ok := err.(*CommitError); !ok || !ce.Fatal || !strings.Contains(ce.Error(), "reply was lost") {
			t.Fatalf("commit after a lost reply that arrived: %v, want a fatal CommitError", err)
		}
	})
	t.Run("a lost reply didn't arrive", func(t *testing.T) {
		url := testPostgres(t)
		p := openPostgres(t, url)
		p.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("1")}}})
		p.unsure = true
		p.drop()
		p.Commit(&Commit{Meta: []MetaWrite{{Key: "k", Data: []byte("3")}}})
		if p.unsure {
			t.Error("the store is still unsure after learning the commit never arrived")
		}
	})
}

// The program tests of testdata/programs/jobs pass on every kind of store,
// unchanged.
func TestJobsOnEveryStore(t *testing.T) {
	prog, diags := check.Program("../testdata/programs/jobs")
	for _, d := range diags {
		t.Fatal(d)
	}
	stores := map[string]func(t *testing.T) Store{
		"journal": func(t *testing.T) Store {
			j, err := OpenJournal(filepath.Join(t.TempDir(), "journal"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { j.Close() })
			return j
		},
		"postgres": func(t *testing.T) Store { return openPostgres(t, testPostgres(t)) },
	}
	for name, open := range stores {
		t.Run(name, func(t *testing.T) {
			defer func() { newTestStore = nil }()
			var made []Store
			newTestStore = func() Store {
				s := open(t)
				made = append(made, s)
				return s
			}
			results := RunTests(prog, "")
			if len(results) == 0 {
				t.Fatal("the program has no tests")
			}
			for _, r := range results {
				if r.Failure != "" {
					t.Errorf("%s: %s\n%s", r.Name, r.Failure, r.Log)
				}
			}
			if len(made) != len(results) {
				t.Errorf("%d stores for %d tests", len(made), len(results))
			}
			var runs int
			for _, s := range made {
				runs += len(s.Runs())
			}
			if runs == 0 {
				t.Error("the tests saved no runs in the store")
			}
		})
	}
}
