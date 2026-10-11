package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// testPostgres gives a test a schema of its own in the database
// $EYE_TEST_POSTGRES names, dropped when the test ends, and returns the
// address of a store in it. Without the variable the test is skipped.
func testPostgres(t *testing.T) string {
	t.Helper()
	base := os.Getenv("EYE_TEST_POSTGRES")
	if base == "" {
		t.Skip("set EYE_TEST_POSTGRES to a Postgres address to test the Postgres store")
	}
	schema := fmt.Sprintf("eyetest_cmd_%d", time.Now().UnixNano())
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
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "search_path=" + schema
}

// eye run and eye runs refuse a store that isn't a Postgres address, and two
// stores at once.
func TestRunStoreFlags(t *testing.T) {
	dir := program(t, map[string]string{"main.eye": "pub fn main() {}\n"})
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"run", "--store", "mysql://x", dir}, "--store takes a Postgres address"},
		{[]string{"run", "--store", "postgres://x", "--memory", dir}, "give one of --store, --journal and --memory"},
		{[]string{"run", "--journal", "j", "--memory", dir}, "give one of --store, --journal and --memory"},
		{[]string{"runs", "--store", "postgres://x", "--journal", "j", dir}, "give --store or --journal, not both"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(c.args, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("eye %s: exit %d, %q; want the usage error %q", strings.Join(c.args, " "), code, stderr.String(), c.want)
		}
	}
	// $EYE_STORE gives the store when no flag does; --memory overrides it.
	t.Setenv("EYE_STORE", "mysql://x")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", dir}, &stdout, &stderr); code != exitUsage {
		t.Errorf("eye run with EYE_STORE=mysql://x: exit %d, want the usage error", code)
	}
	stderr.Reset()
	if code := run([]string{"run", "--memory", dir}, &stdout, &stderr); code != 0 {
		t.Errorf("eye run --memory with EYE_STORE set: exit %d: %s", code, stderr.String())
	}
}
