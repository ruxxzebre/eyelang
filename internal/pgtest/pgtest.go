// Package pgtest starts Postgres databases for tests that need one of their
// own: to stop, pause or restart without disturbing other tests.
package pgtest

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Docker runs a docker command and returns what it printed.
func Docker(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// WaitFor polls cond until it holds, or fails the test after limit.
func WaitFor(t *testing.T, limit time.Duration, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting until %s", what)
}

// Own starts a Postgres container for this test alone and returns the
// database's address and the container, which is removed when the test
// ends. It needs EYE_TEST_DOCKER=1, and skips the test without it.
func Own(t *testing.T) (url, container string) {
	t.Helper()
	if os.Getenv("EYE_TEST_DOCKER") != "1" {
		t.Skip("set EYE_TEST_DOCKER=1 to let tests start a Postgres container of their own")
	}
	// A fixed port, so that it stays the same when the container restarts.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	container = Docker(t, "run", "-d", "-e", "POSTGRES_USER=eye", "-e", "POSTGRES_PASSWORD=eye",
		"-e", "POSTGRES_DB=eye", "-p", fmt.Sprintf("127.0.0.1:%d:5432", port), "postgres:18-alpine")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", container).Run() })
	url = fmt.Sprintf("postgres://eye:eye@127.0.0.1:%d/eye?sslmode=disable", port)
	Ready(t, url)
	return url, container
}

// Ready waits until the database at url takes connections. A Postgres
// container starts a server without TCP to set itself up, then the real
// one: it is ready when a connection over TCP works.
func Ready(t *testing.T, url string) {
	t.Helper()
	WaitFor(t, 60*time.Second, "the database is ready", func() bool {
		conn, err := pgx.Connect(context.Background(), url)
		if err != nil {
			return false
		}
		conn.Close(context.Background())
		return true
	})
}
