package interp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"eyelang/ir"
)

// Services in this process. The online playground runs two programs side by
// side, one serving a contract and one calling it, with no ports and no
// network: StartService makes the serving program's handlers, and the
// caller's RunOptions.Services sends its calls to them through
// handlerTransport. Each call is the HTTP request and reply `eye run` would
// send and serve, so what the page shows is what two `eye run`s would do.

// clockHeader carries a caller's clock to a service in this process, so
// that the service's virtual clock moves with the caller's.
const clockHeader = "Eye-Clock"

// handlerTransport answers a request with a handler, in this process.
type handlerTransport struct{ h http.Handler }

func (t handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	req := r.Clone(r.Context())
	if req.Body == nil {
		req.Body = http.NoBody
	}
	rec := &replyRecorder{header: http.Header{}}
	t.h.ServeHTTP(rec, req)
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", rec.status, http.StatusText(rec.status)),
		StatusCode:    rec.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        rec.header,
		Body:          io.NopCloser(bytes.NewReader(rec.body.Bytes())),
		ContentLength: int64(rec.body.Len()),
		Request:       r,
	}, nil
}

// replyRecorder is the ResponseWriter a handler in this process writes to.
type replyRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *replyRecorder) Header() http.Header { return w.header }

func (w *replyRecorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *replyRecorder) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.body.Write(b)
}

// skipAhead is tryAgain for a service in this process: waiting on the
// system clock can't help it, so the clock moves as the wait would have.
func (rt *runtime) skipAhead(try int) bool {
	if try >= len(retryDelays) {
		return false
	}
	rt.now = rt.now.Add(rt.retryDelay(try))
	return true
}

// Service is a program that serves its contracts in this process, on a
// virtual clock: no port is taken, and RunOptions.Services gives another
// program its Handlers. A call moves the service's clock to the caller's.
type Service struct {
	in       *Interp
	opts     RunOptions
	handlers map[string]http.Handler
	reported int
}

// StartService runs prog's main, if it has one, on a virtual clock, and
// makes a handler for each contract prog serves, by the path of the module
// that declares the contract. opts.Virtual sets the clock and the limits
// Finish runs to; nil is the defaults. Each call the service answers is
// written to opts.Stderr as a line naming the request and its status.
func StartService(prog *ir.Program, opts RunOptions) (*Service, error) {
	if opts.Virtual == nil {
		opts.Virtual = &VirtualClock{}
	}
	in := New(prog, opts.Stdout, opts.Stderr)
	in.files, in.hosts = opts.Files, opts.Hosts
	in.rt.services, in.rt.secrets = opts.Services, opts.Secrets
	in.rt.real = false
	in.rt.now = time.Now()
	if !opts.Virtual.Start.IsZero() {
		in.rt.now = opts.Virtual.Start
	}
	in.rt.memLimit = opts.MemoryLimit
	s := &Service{in: in, opts: opts, handlers: map[string]http.Handler{}}
	if missing := in.linkExterns(); len(missing) > 0 {
		return nil, errors.New(strings.Join(missing, "\n"))
	}
	ls, err := in.listeners(servedContracts(prog))
	if err != nil {
		return nil, err
	}
	err = s.run(func() {
		in.rt.restore()
		if main := prog.Root.Main; main != nil {
			var argv []argVal
			if len(main.Params) > 0 {
				argv = []argVal{{v: &List{}}}
			}
			result, _ := in.callFunc(main, nil, nil, argv, main.Pos)
			if e, ok := isVariant(result, "Err"); ok {
				fmt.Fprintln(opts.Stderr, in.display(e.Vals[0]))
			}
		}
		in.rt.drain()
	})
	if err != nil {
		return nil, err
	}
	in.g = &gil{} // requests take it, as they do from a port
	for _, l := range ls {
		s.handlers[l.ct.Module.Path] = s.wrap(l, in.handler(l))
	}
	return s, nil
}

// Handlers are the service's contracts' handlers, by the path of the module
// that declares each contract: what another program's
// RunOptions.Services takes.
func (s *Service) Handlers() map[string]http.Handler { return s.handlers }

// Finish runs what the calls started and what waits, on the service's
// clock, to the limits of its VirtualClock, as Run does once main is done.
func (s *Service) Finish() {
	s.in.g.mu.Lock()
	defer s.in.g.mu.Unlock()
	if err := s.run(func() { s.in.rt.runVirtual(*s.opts.Virtual, s.report, s.opts.Stderr) }); err != nil {
		fmt.Fprintln(s.opts.Stderr, err)
	}
}

// wrap answers a call with h, after moving the service's clock to the
// caller's, then says what was asked and answered, and runs the job runs the
// call started.
func (s *Service) wrap(l *listener, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		in := s.in
		if t, err := time.Parse(time.RFC3339Nano, r.Header.Get(clockHeader)); err == nil {
			s.locked(func() {
				if d := t.Sub(in.rt.now); d > 0 {
					in.rt.advance(ir.Duration(d))
				}
			})
		}
		rec := &replyRecorder{header: http.Header{}}
		h.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		fmt.Fprintf(s.opts.Stderr, "%s %s %s → %d %s\n", contractName(l.ct), r.Method, r.URL.RequestURI(), rec.status, http.StatusText(rec.status))
		for k, v := range rec.header {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.status)
		_, _ = w.Write(rec.body.Bytes())
		s.locked(in.rt.drain)
	})
}

// locked runs f holding the service's lock, reporting a fault.
func (s *Service) locked(f func()) {
	s.in.g.mu.Lock()
	defer s.in.g.mu.Unlock()
	if err := s.run(f); err != nil {
		fmt.Fprintln(s.opts.Stderr, err)
	}
}

// run runs f, turning a fault into an error, and reports the job runs that
// failed meanwhile.
func (s *Service) run(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Fault:
				err = x
			case *CommitError:
				err = x
			default:
				panic(r)
			}
		}
		s.report()
	}()
	f()
	return nil
}

func (s *Service) report() {
	for _, f := range s.in.rt.failures[s.reported:] {
		fmt.Fprintln(s.opts.Stderr, f.Error())
	}
	s.reported = len(s.in.rt.failures)
}
