package interp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

// fakePostgres is a server that answers every login with the next of
// codes, an SQLSTATE, repeating the last; it counts the logins.
type fakePostgres struct {
	ln     net.Listener
	mu     sync.Mutex
	codes  []string
	logins int
}

func newFakePostgres(t *testing.T, codes ...string) *fakePostgres {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakePostgres{ln: ln, codes: codes}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	return f
}

func (f *fakePostgres) serve(conn net.Conn) {
	defer conn.Close()
	b := pgproto3.NewBackend(conn, conn)
	for {
		msg, err := b.ReceiveStartupMessage()
		if err != nil {
			return
		}
		if _, ok := msg.(*pgproto3.StartupMessage); ok {
			break
		}
		if _, err := conn.Write([]byte("N")); err != nil { // no TLS, no GSS
			return
		}
	}
	f.mu.Lock()
	code := f.codes[min(f.logins, len(f.codes)-1)]
	f.logins++
	f.mu.Unlock()
	b.Send(&pgproto3.ErrorResponse{Severity: "FATAL", Code: code, Message: "fake " + code})
	_ = b.Flush()
}

func (f *fakePostgres) url() string {
	return fmt.Sprintf("postgres://app:secret@%s/app?sslmode=disable", f.ln.Addr())
}

func (f *fakePostgres) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logins
}

func quickRetries(t *testing.T) {
	prev := retryAfter
	retryAfter = 10 * time.Millisecond
	t.Cleanup(func() { retryAfter = prev })
}

// A database that refuses connections at startup, as one behind a network
// policy that is still landing does, is waited for: the runner says it is
// trying again, and keeps trying until it is stopped.
func TestWaitForPostgresWaitsForTheNetwork(t *testing.T) {
	quickRetries(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens there now: connections are refused
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var log lockedBuffer
	p, err := WaitForPostgres(ctx, "postgres://app:secret@"+addr+"/app?sslmode=disable", &log)
	if p != nil || err == nil {
		t.Fatalf("opened %v, %v", p, err)
	}
	if n := strings.Count(log.String(), "; trying again in 10ms\n"); n < 2 {
		t.Errorf("tried again %d times, want several:\n%s", n, log.String())
	}
	if strings.Contains(log.String(), "secret") {
		t.Errorf("the log shows the password:\n%s", log.String())
	}
}

// A database that is still starting is waited for too; once it answers
// that the login is wrong, the runner stops at once.
func TestWaitForPostgresStopsOnAWrongPassword(t *testing.T) {
	quickRetries(t)
	db := newFakePostgres(t, "57P03", "57P03", "28P01")
	var log lockedBuffer
	p, err := WaitForPostgres(context.Background(), db.url(), &log)
	if p != nil || err == nil || !strings.Contains(err.Error(), "fake 28P01") {
		t.Fatalf("opened %v, %v; want the login refused", p, err)
	}
	if n := strings.Count(log.String(), "trying again"); n != 2 || db.count() != 3 {
		t.Errorf("logged in %d times and tried again %d times, want 3 and 2:\n%s", db.count(), n, log.String())
	}
}

// Failures that waiting won't fix return at once.
func TestWaitForPostgresFailsFast(t *testing.T) {
	quickRetries(t)
	for _, code := range []string{"28P01", "28000", "3D000", "42501"} {
		db := newFakePostgres(t, code)
		var log lockedBuffer
		if _, err := WaitForPostgres(context.Background(), db.url(), &log); err == nil || log.String() != "" || db.count() != 1 {
			t.Errorf("%s: %v after %d logins; log:\n%s", code, err, db.count(), log.String())
		}
	}
	var log lockedBuffer
	if _, err := WaitForPostgres(context.Background(), "postgres://app@host:port/app", &log); err == nil || log.String() != "" {
		t.Errorf("a bad address: %v; log:\n%s", err, log.String())
	}
}

func TestTransientPostgresErrors(t *testing.T) {
	for err, want := range map[error]bool{
		fmt.Errorf("db: %w", noAnswer{context.DeadlineExceeded}):                      true,
		fmt.Errorf("db: %w", &net.OpError{Op: "dial", Err: errors.New("refused")}):    true,
		fmt.Errorf("db: %w", &net.DNSError{Err: "no such host", Name: "postgres-rw"}): true,
		fmt.Errorf("db: %w", &pgconn.PgError{Code: "53300"}):                          true,
		fmt.Errorf("db: %w", &pgconn.PgError{Code: "28P01"}):                          false,
		fmt.Errorf("db: no schema of the search_path exists; create it"):              false,
		fmt.Errorf("postgres h:5432/app schema lab is in use by another eye run"):     false,
	} {
		if got := pgTransient(err); got != want {
			t.Errorf("pgTransient(%v) = %v, want %v", err, got, want)
		}
	}
}
