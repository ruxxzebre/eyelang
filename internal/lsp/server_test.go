package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// session runs the server over a scripted conversation and returns every
// message it sent, in order.
type session struct {
	t      *testing.T
	in     bytes.Buffer
	nextID int
}

func newSession(t *testing.T) *session {
	s := &session{t: t}
	s.request("initialize", map[string]any{"capabilities": map[string]any{}})
	s.notify("initialized", map[string]any{})
	return s
}

func (s *session) send(m map[string]any) {
	m["jsonrpc"] = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		s.t.Fatal(err)
	}
	fmt.Fprintf(&s.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

// request sends a request and returns its ID.
func (s *session) request(method string, params any) int {
	s.nextID++
	s.send(map[string]any{"id": s.nextID, "method": method, "params": params})
	return s.nextID
}

func (s *session) notify(method string, params any) {
	s.send(map[string]any{"method": method, "params": params})
}

func (s *session) open(uri, text string) {
	s.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "eyelang", "version": 1, "text": text},
	})
}

func doc(uri string) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": uri}}
}

// run ends the conversation and returns what the server sent.
func (s *session) run() []message {
	s.request("shutdown", nil)
	s.notify("exit", nil)
	var out bytes.Buffer
	if err := Serve(&s.in, &out); err != nil {
		s.t.Fatalf("Serve: %v", err)
	}
	var msgs []message
	c := newConn(bufio.NewReader(&out), nil)
	for {
		m, err := c.read()
		if err != nil {
			break
		}
		msgs = append(msgs, *m)
	}
	return msgs
}

func response(t *testing.T, msgs []message, id int, v any) {
	t.Helper()
	for _, m := range msgs {
		if string(m.ID) != fmt.Sprint(id) || m.Method != "" {
			continue
		}
		if m.Error != nil {
			t.Fatalf("request %d failed: %v", id, m.Error)
		}
		if err := json.Unmarshal(m.Result, v); err != nil {
			t.Fatalf("decoding the result of request %d: %v\n%s", id, err, m.Result)
		}
		return
	}
	t.Fatalf("no response to request %d", id)
}

func diagnosticsFor(t *testing.T, msgs []message, uri string) [][]diagnostic {
	t.Helper()
	var all [][]diagnostic
	for _, m := range msgs {
		if m.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var p publishDiagnostics
		if err := json.Unmarshal(m.Params, &p); err != nil {
			t.Fatal(err)
		}
		if p.URI == uri {
			all = append(all, p.Diagnostics)
		}
	}
	return all
}

func TestInitialize(t *testing.T) {
	s := &session{t: t}
	id := s.request("initialize", map[string]any{})
	msgs := s.run()
	var result struct {
		Capabilities struct {
			PositionEncoding       string `json:"positionEncoding"`
			SemanticTokensProvider struct {
				Legend struct {
					TokenTypes []string `json:"tokenTypes"`
				} `json:"legend"`
			} `json:"semanticTokensProvider"`
		} `json:"capabilities"`
	}
	response(t, msgs, id, &result)
	if result.Capabilities.PositionEncoding != "utf-16" {
		t.Errorf("positionEncoding = %q", result.Capabilities.PositionEncoding)
	}
	if got := result.Capabilities.SemanticTokensProvider.Legend.TokenTypes; len(got) != len(tokenTypes) || got[tokWaitingCall] != "waitingCall" {
		t.Errorf("token types = %v", got)
	}
}

func TestDiagnostics(t *testing.T) {
	s := newSession(t)
	// "é" is one UTF-16 unit and two bytes, "🙂" two units and four bytes, so
	// the stray parenthesis below is at column 16 in UTF-16 units and byte 19.
	s.open("file:///a.eye", "let s = \"é🙂\"\nlet x = (1 + \n")
	s.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": "file:///a.eye", "version": 2},
		"contentChanges": []any{map[string]any{"text": "let s = \"é🙂\" + )\n"}},
	})
	s.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": "file:///a.eye", "version": 3},
		"contentChanges": []any{map[string]any{"text": "let s = \"é🙂\"\n"}},
	})
	s.notify("textDocument/didClose", doc("file:///a.eye"))
	msgs := s.run()

	got := diagnosticsFor(t, msgs, "file:///a.eye")
	if len(got) != 4 {
		t.Fatalf("published %d times, want 4 (open, two changes, close)", len(got))
	}
	if len(got[0]) == 0 {
		t.Fatalf("no diagnostics for an unclosed parenthesis")
	}
	if len(got[1]) == 0 {
		t.Fatalf("no diagnostics for a stray parenthesis")
	}
	if r := got[1][0].Range; r.Start != (Position{0, 16}) || r.End != (Position{0, 17}) {
		t.Errorf("stray parenthesis at %+v, want 0:16–0:17 in UTF-16 units: %s", r, got[1][0].Message)
	}
	if len(got[2]) != 0 || len(got[3]) != 0 {
		t.Errorf("diagnostics remain after the fix or close: %v %v", got[2], got[3])
	}
}

func TestFormatting(t *testing.T) {
	s := newSession(t)
	s.open("file:///messy.eye", "let  a=1\n")
	s.open("file:///tidy.eye", "let a = 1\n")
	s.open("file:///broken.eye", "let a = (\n")
	messy := s.request("textDocument/formatting", doc("file:///messy.eye"))
	tidy := s.request("textDocument/formatting", doc("file:///tidy.eye"))
	broken := s.request("textDocument/formatting", doc("file:///broken.eye"))
	closed := s.request("textDocument/formatting", doc("file:///nowhere.eye"))
	msgs := s.run()

	var edits []textEdit
	response(t, msgs, messy, &edits)
	if len(edits) != 1 || edits[0].NewText != "let a = 1\n" || edits[0].Range.End != (Position{1, 0}) {
		t.Errorf("edits for a messy file = %+v", edits)
	}
	response(t, msgs, tidy, &edits)
	if len(edits) != 0 {
		t.Errorf("edits for a tidy file = %+v", edits)
	}
	for _, id := range []int{broken, closed} {
		for _, m := range msgs {
			if string(m.ID) == fmt.Sprint(id) && m.Error == nil {
				t.Errorf("request %d succeeded, want an error", id)
			}
		}
	}
}

func TestDocumentSymbols(t *testing.T) {
	src, err := os.ReadFile("../../testdata/syntax/valid/spec-03-a-tour-1.eye")
	if err != nil {
		t.Fatal(err)
	}
	s := newSession(t)
	s.open("file:///tour.eye", string(src))
	id := s.request("textDocument/documentSymbol", doc("file:///tour.eye"))
	msgs := s.run()

	var syms []documentSymbol
	response(t, msgs, id, &syms)
	var outline []string
	var walk func(prefix string, syms []documentSymbol)
	walk = func(prefix string, syms []documentSymbol) {
		for _, s := range syms {
			outline = append(outline, prefix+s.Detail+" "+s.Name)
			walk(prefix+"  ", s.Children)
		}
	}
	walk("", syms)
	got := strings.Join(outline, "\n")
	for _, want := range []string{"entity World", "  fn join", "  state phase", "view idleWorlds", "job sleepWhenIdle", " Asleep"} {
		if !strings.Contains(got, want) {
			t.Errorf("outline lacks %q:\n%s", want, got)
		}
	}
}

func TestFoldingRanges(t *testing.T) {
	s := newSession(t)
	s.open("file:///f.eye", "# one\n# two\nfn f() {\n  1\n}\n")
	id := s.request("textDocument/foldingRange", doc("file:///f.eye"))
	msgs := s.run()

	var ranges []foldingRange
	response(t, msgs, id, &ranges)
	want := map[foldingRange]bool{{StartLine: 2, EndLine: 3}: true, {StartLine: 0, EndLine: 1, Kind: "comment"}: true}
	for _, r := range ranges {
		delete(want, r)
	}
	if len(want) > 0 {
		t.Errorf("folding ranges %+v lack %+v", ranges, want)
	}
}

func TestSemanticTokens(t *testing.T) {
	src := "type Phase = Asleep | Running\n" +
		"@on(enters(idle))\n" +
		"job sleepy(w: Ref(World)) -> Int {\n" +
		"  sleep!(20 min)\n" +
		"  let n = w.read!().count\n" +
		"  w.markAsleep()\n" +
		"  match n { Asleep => 1, _ => count(n) }\n" +
		"}\n"
	s := newSession(t)
	s.open("file:///t.eye", src)
	id := s.request("textDocument/semanticTokens/full", doc("file:///t.eye"))
	msgs := s.run()

	var toks semanticTokens
	response(t, msgs, id, &toks)
	if len(toks.Data)%5 != 0 {
		t.Fatalf("data has %d numbers, not a multiple of 5", len(toks.Data))
	}
	lines := strings.Split(src, "\n")
	got := map[string]string{}
	line, char := 0, 0
	for i := 0; i < len(toks.Data); i += 5 {
		dl, dc, length, typ := int(toks.Data[i]), int(toks.Data[i+1]), int(toks.Data[i+2]), int(toks.Data[i+3])
		if dl > 0 {
			line, char = line+dl, dc
		} else {
			char += dc
		}
		text := lines[line][char : char+length]
		got[fmt.Sprintf("%d:%s", line+1, text)] = tokenTypes[typ]
	}
	for tok, want := range map[string]string{
		"1:type":       "keyword",
		"1:Phase":      "type",
		"1:Asleep":     "enumMember",
		"2:@":          "decorator",
		"2:on":         "decorator",
		"2:enters":     "function",
		"3:job":        "keyword",
		"3:sleepy":     "function",
		"3:w":          "parameter",
		"3:Ref":        "type",
		"4:sleep":      "waitingCall",
		"4:!":          "waitingCall",
		"5:read":       "waitingCall",
		"5:count":      "property",
		"6:markAsleep": "method",
		"7:Asleep":     "enumMember",
		"7:count":      "function",
	} {
		if got[tok] != want {
			t.Errorf("%s is %q, want %q", tok, got[tok], want)
		}
	}
}

func TestUnknownRequest(t *testing.T) {
	s := newSession(t)
	id := s.request("textDocument/signatureHelp", map[string]any{})
	s.notify("textDocument/someNotification", map[string]any{})
	msgs := s.run()
	for _, m := range msgs {
		if string(m.ID) == fmt.Sprint(id) {
			if m.Error == nil || m.Error.Code != codeMethodNotFound {
				t.Errorf("signatureHelp answered %+v, want MethodNotFound", m)
			}
			return
		}
	}
	t.Error("no answer to signatureHelp")
}

func TestPositions(t *testing.T) {
	d := newDocument("file:///p.eye", 1, []byte("a\nxé🙂y\n"))
	for _, c := range []struct {
		offset int
		pos    Position
	}{
		{0, Position{0, 0}}, {2, Position{1, 0}}, {3, Position{1, 1}}, {5, Position{1, 2}},
		{9, Position{1, 4}}, {10, Position{1, 5}}, {11, Position{2, 0}},
	} {
		if got := d.position(c.offset); got != c.pos {
			t.Errorf("position(%d) = %+v, want %+v", c.offset, got, c.pos)
		}
		if got := d.offset(c.pos); got != c.offset {
			t.Errorf("offset(%+v) = %d, want %d", c.pos, got, c.offset)
		}
	}
	if got := d.offset(Position{1, 99}); got != 10 {
		t.Errorf("a column past the end of a line clamps to the newline: got %d", got)
	}
}
