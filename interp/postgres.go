package interp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres is the store for a program whose state must outlive its machine:
// the in-memory store, with every commit written to Postgres in one SQL
// transaction before it counts, as the journal writes it to a file.
//
//   - Opening the store reads everything into memory. The tables are made on
//     first use, in the first schema of the connection's search_path, so one
//     database can hold several programs.
//   - One runner at a time: the store holds an advisory lock on the
//     connection it writes with, and a second eye run on the same tables
//     refuses to start.
//   - A commit that can't be written, because the database is down or for any
//     other reason, changes nothing in memory and panics with a *CommitError,
//     which the runtime turns into a retry later. The connection is made
//     again on the next commit, so the runner carries on once the database is
//     back.
//   - Every commit also counts itself in eye_store.commits. If a commit's
//     reply was lost, the next connection reads the count to learn whether it
//     arrived; and a count that moved without this runner means another one
//     wrote. Either way memory no longer matches the database, so the store
//     stops with a fatal *CommitError, and a restart reads the database again.
type Postgres struct {
	memory
	url  string
	name string // where, for messages: host, database and schema, never the password

	mu      sync.Mutex
	conn    *pgx.Conn // nil after a failure, until the next commit connects again
	commits int64     // eye_store.commits as this runner last wrote it
	unsure  bool      // the last commit's reply was lost: it may have arrived
	broken  error     // set when memory no longer matches the database
}

// pgTimeout bounds every exchange with the database, so that a database that
// stops answering fails a commit instead of hanging the runner.
var pgTimeout = 15 * time.Second

// pgSchema is the version of the tables this runtime writes. A later runtime
// that changes them adds a step to pgMigrations.
const pgSchema = 2

// pgMigrations are the SQL that brings the tables from each version to the
// next; pgMigrations[0] makes them.
var pgMigrations = []string{
	`CREATE TABLE IF NOT EXISTS eye_rows (
		key text PRIMARY KEY,
		entity text NOT NULL,
		state bytea NOT NULL,
		version bigint NOT NULL,
		schema integer NOT NULL,
		seq bigserial NOT NULL
	);
	CREATE TABLE IF NOT EXISTS eye_runs (id bigint PRIMARY KEY, data bytea NOT NULL);
	CREATE TABLE IF NOT EXISTS eye_events (seq bigserial PRIMARY KEY, name text NOT NULL, data bytea NOT NULL);
	CREATE TABLE IF NOT EXISTS eye_meta (key text PRIMARY KEY, data bytea NOT NULL);`,
	// Version 2: what is kept of an entity that end(ok) ended.
	`ALTER TABLE eye_rows ADD COLUMN ended boolean NOT NULL DEFAULT false;`,
}

// IsPostgres reports whether a --store value names a Postgres database.
func IsPostgres(url string) bool {
	return strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://")
}

// OpenPostgres opens the store in the database at url for a running program,
// making its tables if need be. Only one process may hold a store open for
// writing.
func OpenPostgres(url string) (*Postgres, error) {
	return openPostgresCtx(context.Background(), url)
}

func openPostgresCtx(parent context.Context, url string) (*Postgres, error) {
	p := &Postgres{url: url}
	ctx, cancel := context.WithTimeout(parent, pgTimeout)
	defer cancel()
	conn, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.lock(ctx, conn); err != nil {
		conn.Close(context.Background())
		return nil, err
	}
	if err := pgMigrate(ctx, conn, p.name); err != nil {
		conn.Close(context.Background())
		return nil, err
	}
	mem, commits, err := pgRead(ctx, conn)
	if err != nil {
		conn.Close(context.Background())
		return nil, fmt.Errorf("%s: %w", p.name, err)
	}
	p.memory, p.conn, p.commits = mem, conn, commits
	return p, nil
}

// WaitForPostgres opens the store as OpenPostgres does, for a runner that
// starts before its database can be reached: while the database refuses
// connections, doesn't answer, has a name that doesn't resolve yet, or is
// still starting up, it says so on log and tries again every retryAfter, as
// a runner does with a commit, until ctx is done. It waits the same way
// while another eye run holds the store, as the runner of a pod being
// replaced does until it stops, or the database notices it has gone; two
// runners started by mistake show in the log as one that keeps waiting. A
// failure that waiting won't fix returns at once: an address that can't be
// read, a login or TLS the database refuses, a database or schema that
// doesn't exist, a user that may not make the tables, or tables a newer eye
// wrote.
func WaitForPostgres(ctx context.Context, url string, log io.Writer) (*Postgres, error) {
	for {
		p, err := pgOpen(ctx, url)
		var held *storeHeld
		switch {
		case err == nil || ctx.Err() != nil:
			return p, err
		case errors.As(err, &held):
			fmt.Fprintf(log, "eye run: another runner holds this store (%s); waiting, trying again in %v\n", held.name, retryAfter)
		case pgTransient(err):
			fmt.Fprintf(log, "eye run: %v; trying again in %v\n", err, retryAfter)
		default:
			return nil, err
		}
		timer := time.NewTimer(retryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, err
		case <-timer.C:
		}
	}
}

// pgTransient reports whether a failure to open the store may go away by
// itself: the network, a database that is starting, restarting or full of
// connections. Anything the database answered otherwise is a mistake to
// fix, not to wait out.
func pgTransient(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "57P01", "57P02", "57P03", // admin_shutdown, crash_shutdown, cannot_connect_now
			"53300": // too_many_connections
			return true
		}
		return strings.HasPrefix(pgErr.Code, "08") // connection_exception
	}
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET)
}

// ReadPostgres reads the store in the database at url without changing it,
// for tools such as `eye runs` that look while the program runs.
func ReadPostgres(url string) (Store, error) {
	p := &Postgres{url: url}
	ctx, cancel := context.WithTimeout(context.Background(), pgTimeout)
	defer cancel()
	conn, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('eye_store') IS NOT NULL`).Scan(&exists); err != nil {
		return nil, fmt.Errorf("%s: %v", p.name, err)
	}
	if !exists {
		return nil, fmt.Errorf("%s has no eye store yet; eye run --store makes one", p.name)
	}
	mem, _, err := pgRead(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", p.name, err)
	}
	return mem, nil
}

// connect opens a connection, and names the store for messages from what it
// connected to.
func (p *Postgres) connect(ctx context.Context) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig(p.url)
	if err != nil {
		return nil, fmt.Errorf("the store's address: %v", err)
	}
	if p.name == "" {
		p.name = pgName(cfg)
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p.name, pgReason(err))
	}
	var schema *string
	if err := conn.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		conn.Close(context.Background())
		return nil, fmt.Errorf("%s: %w", p.name, pgReason(err))
	}
	if schema == nil {
		conn.Close(context.Background())
		return nil, fmt.Errorf("%s: no schema of the search_path exists; create it, or name one that does", p.name)
	}
	if !strings.Contains(p.name, " schema ") {
		p.name += " schema " + *schema
	}
	return conn, nil
}

// pgName names a database for messages and the log: its host, port and name,
// and never its user or password.
func pgName(cfg *pgx.ConnConfig) string {
	return fmt.Sprintf("postgres %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
}

// lock takes the advisory lock that says this runner writes the store: one
// per database and schema.
func (p *Postgres) lock(ctx context.Context, conn *pgx.Conn) error {
	var got bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtext('eyelang store ' || current_schema()))`).Scan(&got); err != nil {
		return fmt.Errorf("%s: %w", p.name, pgReason(err))
	}
	if !got {
		return &storeHeld{p.name}
	}
	return nil
}

// storeHeld is the failure to take a store's lock: another eye run holds it.
type storeHeld struct{ name string }

func (e *storeHeld) Error() string { return e.name + " is in use by another eye run" }

// pgOpen opens a store for WaitForPostgres; tests replace it.
var pgOpen = openPostgresCtx

// pgMigrate makes the tables, or brings them up to this runtime's version.
// The caller holds the lock, so no other runner does this at the same time.
func pgMigrate(ctx context.Context, conn *pgx.Conn, name string) error {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS eye_store (
		id integer PRIMARY KEY CHECK (id = 1),
		schema integer NOT NULL,
		commits bigint NOT NULL
	)`); err != nil {
		return fmt.Errorf("%s: %w", name, pgReason(err))
	}
	if _, err := conn.Exec(ctx, `INSERT INTO eye_store VALUES (1, 0, 0) ON CONFLICT DO NOTHING`); err != nil {
		return fmt.Errorf("%s: %w", name, pgReason(err))
	}
	var version int
	if err := conn.QueryRow(ctx, `SELECT schema FROM eye_store`).Scan(&version); err != nil {
		return fmt.Errorf("%s: %w", name, pgReason(err))
	}
	if version > pgSchema {
		return fmt.Errorf("%s was written by a newer eye (its tables are at version %d, this eye knows %d)", name, version, pgSchema)
	}
	for ; version < pgSchema; version++ {
		tx, err := conn.Begin(ctx)
		if err == nil {
			_, err = tx.Exec(ctx, pgMigrations[version])
		}
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE eye_store SET schema = $1`, version+1)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			if tx != nil {
				_ = tx.Rollback(context.Background())
			}
			return fmt.Errorf("%s: making the tables at version %d: %v", name, version+1, pgReason(err))
		}
	}
	return nil
}

// pgRead reads the whole store into memory, in one snapshot of the database,
// with the count of commits it has taken.
func pgRead(ctx context.Context, conn *pgx.Conn) (*memStore, int64, error) {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, pgReason(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }() // read only: nothing to keep
	var commits int64
	if err := tx.QueryRow(ctx, `SELECT commits FROM eye_store`).Scan(&commits); err != nil {
		return nil, 0, pgReason(err)
	}
	c := &Commit{}
	rows, _ := tx.Query(ctx, `SELECT key, entity, state, version, schema, ended FROM eye_rows ORDER BY seq`)
	var w Write
	if _, err := pgx.ForEachRow(rows, []any{&w.Key, &w.Entity, &w.State, &w.Version, &w.Schema, &w.Ended}, func() error {
		c.Writes = append(c.Writes, w)
		return nil
	}); err != nil {
		return nil, 0, pgReason(err)
	}
	rows, _ = tx.Query(ctx, `SELECT id, data FROM eye_runs ORDER BY id`)
	var r RunWrite
	if _, err := pgx.ForEachRow(rows, []any{&r.ID, &r.Data}, func() error {
		c.Runs = append(c.Runs, r)
		return nil
	}); err != nil {
		return nil, 0, pgReason(err)
	}
	rows, _ = tx.Query(ctx, `SELECT name, data FROM eye_events ORDER BY seq`)
	var e EventRecord
	if _, err := pgx.ForEachRow(rows, []any{&e.Name, &e.Data}, func() error {
		c.Events = append(c.Events, e)
		return nil
	}); err != nil {
		return nil, 0, pgReason(err)
	}
	rows, _ = tx.Query(ctx, `SELECT key, data FROM eye_meta ORDER BY key`)
	var m MetaWrite
	if _, err := pgx.ForEachRow(rows, []any{&m.Key, &m.Data}, func() error {
		c.Meta = append(c.Meta, m)
		return nil
	}); err != nil {
		return nil, 0, pgReason(err)
	}
	mem := newMemStore()
	mem.Commit(c)
	return mem, commits, nil
}

// Commit writes the commit to the database in one transaction, and only then
// applies it.
func (p *Postgres) Commit(c *Commit) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.broken == nil {
		if err := p.write(c); err != nil {
			panic(&CommitError{Path: p.name, Err: err, Fatal: p.broken != nil})
		}
		p.memory.Commit(c)
		return
	}
	panic(&CommitError{Path: p.name, Err: p.broken, Fatal: true})
}

// write sends one commit, connecting first if the last commit failed.
func (p *Postgres) write(c *Commit) error {
	ctx, cancel := context.WithTimeout(context.Background(), pgTimeout)
	defer cancel()
	if p.conn == nil {
		if err := p.reconnect(ctx); err != nil {
			return err
		}
	}
	tx, err := p.conn.Begin(ctx)
	if err != nil {
		p.drop()
		return pgReason(err)
	}
	if err := pgApply(ctx, tx, c, p.commits); err != nil {
		_ = tx.Rollback(context.Background())
		if errors.Is(err, errOtherWriter) {
			p.broken = fmt.Errorf("another runner has written to the store; restart eye run to read it again")
			return p.broken
		}
		if p.conn.IsClosed() {
			p.drop()
		}
		return pgReason(err)
	}
	if err := tx.Commit(ctx); err != nil {
		// The reply was lost: the commit may or may not have arrived. The
		// next connection finds out.
		p.unsure = true
		p.drop()
		return pgReason(err)
	}
	p.commits++
	return nil
}

var errOtherWriter = errors.New("eye_store.commits moved")

// pgApply sends a commit's writes in tx, counting the commit; base is the
// count this runner expects the store to be at.
func pgApply(ctx context.Context, tx pgx.Tx, c *Commit, base int64) error {
	tag, err := tx.Exec(ctx, `UPDATE eye_store SET commits = commits + 1 WHERE commits = $1`, base)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errOtherWriter
	}
	b := &pgx.Batch{}
	for _, w := range c.Writes {
		b.Queue(`INSERT INTO eye_rows (key, entity, state, version, schema, ended) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (key) DO UPDATE SET state = excluded.state, version = excluded.version, schema = excluded.schema, ended = excluded.ended`,
			w.Key, w.Entity, w.State, w.Version, w.Schema, w.Ended)
	}
	for _, r := range c.Runs {
		b.Queue(`INSERT INTO eye_runs (id, data) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET data = excluded.data`, r.ID, r.Data)
	}
	if c.DropEvents > 0 {
		b.Queue(`DELETE FROM eye_events WHERE seq IN (SELECT seq FROM eye_events ORDER BY seq LIMIT $1)`, c.DropEvents)
	}
	for _, e := range c.Events {
		b.Queue(`INSERT INTO eye_events (name, data) VALUES ($1, $2)`, e.Name, e.Data)
	}
	for _, m := range c.Meta {
		if m.Data == nil {
			b.Queue(`DELETE FROM eye_meta WHERE key = $1`, m.Key)
		} else {
			b.Queue(`INSERT INTO eye_meta (key, data) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET data = excluded.data`, m.Key, m.Data)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	return tx.SendBatch(ctx, b).Close()
}

// reconnect connects again after a failure: it takes the lock again, and
// checks that the database holds what memory does.
func (p *Postgres) reconnect(ctx context.Context) error {
	conn, err := p.connect(ctx)
	if err != nil {
		return err
	}
	if err := p.lock(ctx, conn); err != nil {
		conn.Close(context.Background())
		return err
	}
	var commits int64
	if err := conn.QueryRow(ctx, `SELECT commits FROM eye_store`).Scan(&commits); err != nil {
		conn.Close(context.Background())
		return pgReason(err)
	}
	switch {
	case commits == p.commits:
		p.unsure = false
	case commits == p.commits+1 && p.unsure:
		p.broken = fmt.Errorf("a commit whose reply was lost reached the database; restart eye run to carry on from it")
	default:
		p.broken = fmt.Errorf("another runner has written to the store; restart eye run to read it again")
	}
	if p.broken != nil {
		conn.Close(context.Background())
		return p.broken
	}
	p.conn = conn
	return nil
}

// drop closes the connection after a failure; the next commit connects again.
func (p *Postgres) drop() {
	if p.conn != nil {
		p.conn.Close(context.Background())
		p.conn = nil
	}
}

// Close closes the store and releases it for another process.
func (p *Postgres) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), pgTimeout)
	defer cancel()
	err := p.conn.Close(ctx)
	p.conn = nil
	return err
}

// pgReason shortens what pgx reports to what a person needs.
func pgReason(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return noAnswer{err}
	}
	return err
}

// noAnswer is a database that didn't answer in time; it still unwraps to
// what pgx reported, so pgTransient can tell.
type noAnswer struct{ err error }

func (e noAnswer) Error() string {
	return fmt.Sprintf("the database didn't answer within %v", pgTimeout)
}
func (e noAnswer) Unwrap() error { return e.err }
