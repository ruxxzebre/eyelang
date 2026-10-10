package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"eyelang/syntax"
)

// Serve runs the language server until the client sends `exit` or closes the
// connection.
func Serve(in io.Reader, out io.Writer) error {
	s := &server{conn: newConn(in, out), docs: map[string]*document{}}
	return s.run()
}

type server struct {
	conn     *conn
	docs     map[string]*document
	shutdown bool
}

func (s *server) run() error {
	for {
		m, err := s.conn.read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if m.Method == "exit" {
			return nil
		}
		result, rerr := s.handle(m)
		if m.ID == nil {
			continue // a notification gets no response
		}
		resp := &message{ID: m.ID}
		if rerr != nil {
			resp.Error = rerr
		} else {
			raw, err := json.Marshal(result)
			if err != nil {
				return err
			}
			resp.Result = raw
		}
		if err := s.conn.write(resp); err != nil {
			return err
		}
	}
}

func (s *server) handle(m *message) (any, *responseError) {
	decode := func(v any) *responseError {
		if err := json.Unmarshal(m.Params, v); err != nil {
			return &responseError{Code: codeInvalidParams, Message: err.Error()}
		}
		return nil
	}
	switch m.Method {
	case "initialize":
		return s.initialize(), nil
	case "initialized", "$/cancelRequest", "$/setTrace", "workspace/didChangeConfiguration":
		return nil, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil

	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
				Text    string `json:"text"`
			} `json:"textDocument"`
		}
		if err := decode(&p); err != nil {
			return nil, err
		}
		s.open(p.TextDocument.URI, p.TextDocument.Version, p.TextDocument.Text)
		return nil, nil
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := decode(&p); err != nil {
			return nil, err
		}
		if n := len(p.ContentChanges); n > 0 {
			// The server asks for full syncs, so the last change is the whole text.
			s.open(p.TextDocument.URI, p.TextDocument.Version, p.ContentChanges[n-1].Text)
		}
		return nil, nil
	case "textDocument/didClose":
		var p textDocumentParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		delete(s.docs, p.TextDocument.URI)
		_ = s.conn.notify("textDocument/publishDiagnostics", publishDiagnostics{URI: p.TextDocument.URI, Diagnostics: []diagnostic{}})
		return nil, nil
	case "textDocument/didSave":
		return nil, nil

	case "textDocument/formatting":
		var p textDocumentParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		return s.withDoc(p.TextDocument.URI, s.format)
	case "textDocument/documentSymbol":
		var p textDocumentParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		return s.withDoc(p.TextDocument.URI, func(d *document) (any, *responseError) { return symbols(d), nil })
	case "textDocument/foldingRange":
		var p textDocumentParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		return s.withDoc(p.TextDocument.URI, func(d *document) (any, *responseError) { return foldingRanges(d), nil })
	case "textDocument/semanticTokens/full":
		var p textDocumentParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		return s.withDoc(p.TextDocument.URI, func(d *document) (any, *responseError) {
			return semanticTokens{Data: semanticData(d)}, nil
		})
	}
	if strings.HasPrefix(m.Method, "$/") || m.ID == nil {
		return nil, nil // notifications the server doesn't need
	}
	return nil, &responseError{Code: codeMethodNotFound, Message: "eye lsp doesn't support " + m.Method + " yet"}
}

type textDocumentParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
}

func (s *server) withDoc(uri string, f func(*document) (any, *responseError)) (any, *responseError) {
	d, ok := s.docs[uri]
	if !ok {
		return nil, &responseError{Code: codeRequestFailed, Message: "the document isn't open: " + uri}
	}
	return f(d)
}

func (s *server) initialize() any {
	return map[string]any{
		"capabilities": map[string]any{
			"positionEncoding": "utf-16",
			"textDocumentSync": map[string]any{
				"openClose": true,
				"change":    1, // full text on every change
				"save":      false,
			},
			"documentFormattingProvider": true,
			"documentSymbolProvider":     true,
			"foldingRangeProvider":       true,
			"semanticTokensProvider": map[string]any{
				"legend": map[string]any{"tokenTypes": tokenTypes, "tokenModifiers": tokenModifiers},
				"full":   true,
			},
		},
		"serverInfo": map[string]any{"name": "eye lsp"},
	}
}

// ---------------------------------------------------------------- diagnostics

type diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type publishDiagnostics struct {
	URI         string       `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

// open parses a document and publishes its diagnostics.
func (s *server) open(uri string, version int, text string) {
	d := newDocument(uri, version, []byte(text))
	s.docs[uri] = d
	diags := []diagnostic{}
	for _, e := range d.file.Diagnostics {
		r := d.rangeOf(e.Span)
		if r.Start == r.End {
			// Show something for a missing token: the character after it, if
			// there is one on the line.
			end := d.position(e.Span.End + 1)
			if end.Line == r.Start.Line {
				r.End = end
			}
		}
		diags = append(diags, diagnostic{Range: r, Severity: 1, Source: "eye", Message: e.Message})
	}
	_ = s.conn.notify("textDocument/publishDiagnostics", publishDiagnostics{URI: uri, Version: version, Diagnostics: diags})
}

// ---------------------------------------------------------------- formatting

type textEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

func (s *server) format(d *document) (any, *responseError) {
	if len(d.file.Diagnostics) > 0 {
		return nil, &responseError{Code: codeRequestFailed, Message: "fix the syntax errors before formatting"}
	}
	out, err := syntax.Format(d.text)
	if err != nil {
		return nil, &responseError{Code: codeRequestFailed, Message: err.Error()}
	}
	if string(out) == string(d.text) {
		return []textEdit{}, nil
	}
	return []textEdit{{Range: Range{Start: Position{}, End: d.position(len(d.text))}, NewText: string(out)}}, nil
}

// ---------------------------------------------------------------- outline

type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

// LSP symbol kinds.
const (
	symbolModule    = 2
	symbolClass     = 5
	symbolMethod    = 6
	symbolField     = 8
	symbolInterface = 11
	symbolFunction  = 12
	symbolVariable  = 13
	symbolEnum      = 10
	symbolStruct    = 23
	symbolEvent     = 24
	symbolEnumMem   = 22
)

func symbols(d *document) []documentSymbol {
	out := []documentSymbol{}
	for _, n := range d.file.Root.Children {
		if sym, ok := symbolFor(d, n, false); ok {
			out = append(out, sym)
		}
	}
	return out
}

func symbolFor(d *document, n *syntax.Node, member bool) (documentSymbol, bool) {
	kind, detail := 0, ""
	switch n.Kind {
	case syntax.FunctionDeclaration, syntax.FunctionSignature:
		kind, detail = symbolFunction, "fn"
		if member {
			kind = symbolMethod
		}
	case syntax.ExternDeclaration:
		kind, detail = symbolFunction, "extern fn"
	case syntax.JobDeclaration:
		kind, detail = symbolFunction, "job"
	case syntax.EntityDeclaration:
		kind, detail = symbolClass, "entity"
	case syntax.TypeDeclaration:
		kind, detail = symbolStruct, "type"
		if v := n.ChildByField(syntax.FieldValue); v != nil && v.Kind == syntax.VariantList {
			kind = symbolEnum
		}
	case syntax.TraitDeclaration:
		kind, detail = symbolInterface, "trait"
	case syntax.ImplDeclaration:
		kind, detail = symbolClass, "impl"
	case syntax.ContractDeclaration:
		kind, detail = symbolModule, "contract"
	case syntax.CommandDeclaration:
		kind, detail = symbolMethod, "command"
	case syntax.QueryDeclaration:
		kind, detail = symbolMethod, "query"
	case syntax.EventDeclaration:
		kind, detail = symbolEvent, "event"
	case syntax.StateDeclaration:
		kind, detail = symbolField, "state"
	case syntax.ViewDeclaration:
		kind, detail = symbolVariable, "view"
	case syntax.TestDeclaration:
		kind, detail = symbolFunction, "test"
	case syntax.LetStatement:
		kind, detail = symbolVariable, "let"
	case syntax.Migration:
		kind, detail = symbolMethod, "migrate"
	case syntax.Variant:
		kind = symbolEnumMem
	default:
		return documentSymbol{}, false
	}

	nameNode := n.ChildByField(syntax.FieldName)
	if nameNode == nil {
		nameNode = n.ChildByField(syntax.FieldPattern)
	}
	if n.Kind == syntax.ImplDeclaration {
		nameNode = n.ChildByField(syntax.FieldType)
	}
	if n.Kind == syntax.Migration {
		nameNode = n.ChildByField(syntax.FieldVersion)
	}
	if nameNode == nil {
		return documentSymbol{}, false
	}
	name := nameNode.Text(d.text)
	switch n.Kind {
	case syntax.ImplDeclaration:
		if tr := n.ChildByField(syntax.FieldTrait); tr != nil {
			name = tr.Text(d.text) + " for " + name
		}
	case syntax.Migration:
		name = "from " + name
	case syntax.TestDeclaration:
		name = strings.Trim(name, `"`)
	}
	sym := documentSymbol{
		Name: name, Detail: detail, Kind: kind,
		Range: d.rangeOf(n.Span), SelectionRange: d.rangeOf(nameNode.Span),
	}
	// Members of bodies, and the variants of a type.
	if body := n.ChildByField(syntax.FieldBody); body != nil {
		switch body.Kind {
		case syntax.EntityBody, syntax.TraitBody, syntax.ImplBody, syntax.ContractBody:
			for _, c := range body.Children {
				if child, ok := symbolFor(d, c, true); ok {
					sym.Children = append(sym.Children, child)
				}
			}
		}
	}
	if v := n.ChildByField(syntax.FieldValue); n.Kind == syntax.TypeDeclaration && v != nil && v.Kind == syntax.VariantList {
		for _, c := range v.Children {
			if child, ok := symbolFor(d, c, true); ok {
				sym.Children = append(sym.Children, child)
			}
		}
	}
	return sym, true
}

// ---------------------------------------------------------------- folding

type foldingRange struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Kind      string `json:"kind,omitempty"`
}

var foldable = map[syntax.Kind]bool{
	syntax.Block: true, syntax.EntityBody: true, syntax.TraitBody: true, syntax.ImplBody: true,
	syntax.ContractBody: true, syntax.MatchBody: true, syntax.Arguments: true, syntax.Parameters: true,
	syntax.List: true, syntax.Record: true, syntax.RecordUpdate: true, syntax.Map: true,
	syntax.RecordType: true, syntax.VariantList: true,
}

func foldingRanges(d *document) []foldingRange {
	out := []foldingRange{}
	d.file.Root.Walk(func(n *syntax.Node) bool {
		if foldable[n.Kind] {
			start, end := d.position(n.Span.Start).Line, d.position(n.Span.End).Line
			if end > start {
				// Keep the closing bracket's line visible.
				out = append(out, foldingRange{StartLine: start, EndLine: end - 1})
			}
		}
		return true
	})
	// Runs of comment lines.
	start, prev := -1, -2
	flush := func() {
		if start >= 0 && prev > start {
			out = append(out, foldingRange{StartLine: start, EndLine: prev, Kind: "comment"})
		}
	}
	d.file.Root.Walk(func(n *syntax.Node) bool {
		for _, t := range n.Leading {
			if t.Kind != syntax.Comment {
				continue
			}
			line := d.position(t.Span.Start).Line
			if line != prev+1 {
				flush()
				start = line
			}
			prev = line
		}
		return true
	})
	flush()
	return out
}

func (e *responseError) Error() string { return fmt.Sprintf("%d: %s", e.Code, e.Message) }
