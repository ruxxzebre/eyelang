// Command eye-wasm is the online playground's engine: the checker,
// interpreter, formatter and language server, built for WebAssembly
// (GOOS=js GOARCH=wasm) and called from the docs site's playground page
// through one function, eye.call(op, args, onOutput), on the page's
// globalThis. `task playground` builds it into docs/public/wasm.
//
// Built for anything else, it says how to build it.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"eyelang/internal/lsp"
	"eyelang/internal/playground"
)

// args are a call's arguments, as JSON; each op reads those it needs.
type args struct {
	Files  map[string]string `json:"files"`
	Path   string            `json:"path"`
	Text   string            `json:"text"`
	Filter string            `json:"filter"`
	Args   []string          `json:"args"`
	Start  string            `json:"start"` // RFC 3339; "" is now
	Zone   string            `json:"zone"`  // the visitor's time zone, such as "Europe/Dublin"
	At     lsp.Position      `json:"at"`
}

// call runs one op on the playground and returns its result as JSON. A run
// writes what the program prints to out as it prints it.
func call(p *playground.Playground, op, raw string, out func(stream, text string)) string {
	var a args
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			return errorJSON(fmt.Errorf("%s: %v", op, err))
		}
	}
	var result any
	switch op {
	case "setFiles":
		p.SetFiles(a.Files)
		result = true
	case "set":
		p.Set(a.Path, a.Text)
		result = true
	case "remove":
		p.Remove(a.Path)
		result = true
	case "check":
		result = p.Check()
	case "test":
		result = p.Test(a.Filter)
	case "run":
		opts := playground.RunOptions{Args: a.Args, Start: time.Now()}
		if a.Zone != "" {
			// A browser has no local zone for Go; the page names it.
			if loc, err := time.LoadLocation(a.Zone); err == nil {
				opts.Start = opts.Start.In(loc)
			}
		}
		if a.Start != "" {
			t, err := time.Parse(time.RFC3339, a.Start)
			if err != nil {
				return errorJSON(fmt.Errorf("run: start: %v", err))
			}
			opts.Start = t
		}
		result = p.Run(opts, writer{"stdout", out}, writer{"stderr", out}, writer{"service", out})
	case "format":
		text, err := p.Format(a.Path)
		if err != nil {
			return errorJSON(err)
		}
		result = map[string]string{"text": text}
	case "hover":
		result = p.Hover(a.Path, a.At)
	case "complete":
		result = p.Complete(a.Path, a.At)
	case "definition":
		result = p.Definition(a.Path, a.At)
	case "highlight":
		result = playground.Highlight(a.Text)
	default:
		return errorJSON(fmt.Errorf("no op %q", op))
	}
	data, err := json.Marshal(map[string]any{"result": result})
	if err != nil {
		return errorJSON(err)
	}
	return string(data)
}

func errorJSON(err error) string {
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(data)
}

// writer passes what a run writes to the page, as it is written.
type writer struct {
	stream string
	out    func(stream, text string)
}

func (w writer) Write(b []byte) (int, error) {
	if w.out != nil {
		w.out(w.stream, string(b))
	}
	return len(b), nil
}

var _ io.Writer = writer{}
