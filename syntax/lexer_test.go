package syntax

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// describe renders the significant tokens of src as "kind`text`" separated by
// spaces, with "⏎" before tokens that follow a line break, "·" before tokens
// that follow other trivia, and "missing" before tokens marked Missing.
func describe(src string) string {
	tokens, _ := Lex([]byte(src))
	var parts []string
	for _, tok := range tokens {
		if tok.Kind.IsTrivia() || tok.Kind == EOF {
			continue
		}
		var b strings.Builder
		switch {
		case tok.Flags&NewlineBefore != 0:
			b.WriteString("⏎")
		case tok.Flags&SpaceBefore != 0:
			b.WriteString("·")
		}
		text := src[tok.Span.Start:tok.Span.End]
		switch name := tok.Kind.String(); {
		case tok.Flags&Missing != 0:
			b.WriteString("missing " + name)
		case name != text:
			b.WriteString(name + "`" + text + "`")
		default:
			b.WriteString("`" + text + "`")
		}
		parts = append(parts, b.String())
	}
	return strings.Join(parts, " ")
}

func TestLexTokens(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"names", "world World _ _x self",
			"identifier`world` ·type_identifier`World` ·`_` ·identifier`_x` ·identifier`self`"},
		{"keywords", "let var fn pub return if else match for in while break continue",
			"`let` ·`var` ·`fn` ·`pub` ·`return` ·`if` ·`else` ·`match` ·`for` ·`in` ·`while` ·`break` ·`continue`"},
		{"more keywords", "type trait impl entity state job view contract import test with start true false",
			"`type` ·`trait` ·`impl` ·`entity` ·`state` ·`job` ·`view` ·`contract` ·`import` ·`test` ·`with` ·`start` ·`true` ·`false`"},
		{"contextual keywords are names", "command query event on migrate from after at uses min MiB",
			"identifier`command` ·identifier`query` ·identifier`event` ·identifier`on` ·identifier`migrate` ·identifier`from` ·identifier`after` ·identifier`at` ·identifier`uses` ·identifier`min` ·type_identifier`MiB`"},
		{"keyword prefixes are names", "lets fnord instance",
			"identifier`lets` ·identifier`fnord` ·identifier`instance`"},
		{"integers", "0 42 1_000_000 007",
			"integer`0` ·integer`42` ·integer`1_000_000` ·integer`007`"},
		{"underscores need digits on both sides", "1__0 1_",
			"integer`1` identifier`__0` ·integer`1` `_`"},
		{"floats", "1.5 0.25 6.02e23 2.5E-3 1_000.000_1 1e9",
			"float`1.5` ·float`0.25` ·float`6.02e23` ·float`2.5E-3` ·float`1_000.000_1` ·integer`1` identifier`e9`"},
		{"a dot without digits is a field", "1.abs 1.5.x 1.5e",
			"integer`1` `.` identifier`abs` ·float`1.5` `.` identifier`x` ·float`1.5` identifier`e`"},
		{"units are names after numbers", "500 ms 20min 64 MiB",
			"integer`500` ·identifier`ms` ·integer`20` identifier`min` ·integer`64` ·type_identifier`MiB`"},
		{"dates", "2026-10-10 2026 - 10 - 10 2026-10 20261-10-10 2026-10-100",
			"date`2026-10-10` ·integer`2026` ·`-` ·integer`10` ·`-` ·integer`10` ·integer`2026` `-` integer`10` ·integer`20261` `-` integer`10` `-` integer`10` ·date`2026-10-10` integer`0`"},
		{"times", "07:40 23:59:30 7:40 123:45 07:40:3",
			"time`07:40` ·time`23:59:30` ·integer`7` `:` integer`40` ·integer`123` `:` integer`45` ·time`07:40` `:` integer`3`"},
		{"instant", `2026-10-10 07:40 in "Europe/Dublin"`,
			"date`2026-10-10` ·time`07:40` ·`in` ·`\"` string_content`Europe/Dublin` `\"`"},
		{"operators", "-> => = == != < <= > >= + - * / % ! ? ?? | |> || &&",
			"`->` ·`=>` ·`=` ·`==` ·`!=` ·`<` ·`<=` ·`>` ·`>=` ·`+` ·`-` ·`*` ·`/` ·`%` ·`!` ·`?` ·`??` ·`|` ·`|>` ·`||` ·`&&`"},
		{"delimiters", "( ) [ ] { } , : ; . .. .{ @",
			"`(` ·`)` ·`[` ·`]` ·`{` ·`}` ·`,` ·`:` ·`;` ·`.` ·`..` ·`.{` ·`@`"},
		{"longest match", "a!=b a!(b) a??b ...x",
			"identifier`a` `!=` identifier`b` ·identifier`a` `!` `(` identifier`b` `)` ·identifier`a` `??` identifier`b` ·`..` `.` identifier`x`"},
		{"waiting call and not", "f!(x) !f(x) f !(x) w.read!()",
			"identifier`f` `!` `(` identifier`x` `)` ·`!` identifier`f` `(` identifier`x` `)` ·identifier`f` ·`!` `(` identifier`x` `)` ·identifier`w` `.` identifier`read` `!` `(` `)`"},
		{"attributes", "@on(enters(v))",
			"`@` identifier`on` `(` identifier`enters` `(` identifier`v` `)` `)`"},
		{"comments are trivia", "a # comment\n# another\nb",
			"identifier`a` ⏎identifier`b`"},
		{"newline flags", "a\n  .b\n\n(c)",
			"identifier`a` ⏎`.` identifier`b` ⏎`(` identifier`c` `)`"},
		{"empty string", `""`, "`\"` `\"`"},
		{"string text keeps spaces", `"  two  words  "`, "`\"` string_content`  two  words  ` `\"`"},
		{"hash inside a string", `"#1"`, "`\"` string_content`#1` `\"`"},
		{"escapes", `"\n\r\t\0\\\"\{\}\u{1F600}\u{0}"`,
			"`\"` escape_sequence`\\n` escape_sequence`\\r` escape_sequence`\\t` escape_sequence`\\0` escape_sequence`\\\\` escape_sequence`\\\"` escape_sequence`\\{` escape_sequence`\\}` escape_sequence`\\u{1F600}` escape_sequence`\\u{0}` `\"`"},
		{"invalid escapes", `"\q\u{}\u{1234567}"`,
			"`\"` illegal`\\q` illegal`\\u` interpolation_open`{` interpolation_close`}` illegal`\\u` interpolation_open`{` integer`1234567` interpolation_close`}` `\"`"},
		{"interpolation", `"a {b} c"`,
			"`\"` string_content`a ` interpolation_open`{` identifier`b` interpolation_close`}` string_content` c` `\"`"},
		{"nested interpolation", `"{f("x{y}")}"`,
			"`\"` interpolation_open`{` identifier`f` `(` `\"` string_content`x` interpolation_open`{` identifier`y` interpolation_close`}` `\"` `)` interpolation_close`}` `\"`"},
		{"braces inside an interpolation", `"{ {a: 1}.a }"`,
			"`\"` interpolation_open`{` ·`{` identifier`a` `:` ·integer`1` `}` `.` identifier`a` ·interpolation_close`}` `\"`"},
		{"a closing brace in text is text", `"a } b"`, "`\"` string_content`a } b` `\"`"},
		{"unterminated string", "\"abc\nx",
			"`\"` string_content`abc` missing \" ⏎identifier`x`"},
		{"unterminated string at end of input", `"abc`, "`\"` string_content`abc` missing \""},
		{"unterminated interpolation", `"a {b`,
			"`\"` string_content`a ` interpolation_open`{` identifier`b` missing interpolation_close missing \""},
		{"interpolation across lines", "\"{a\n+ b}\"",
			"`\"` interpolation_open`{` identifier`a` ⏎`+` ·identifier`b` interpolation_close`}` `\"`"},
		{"illegal characters", "a $ é & b",
			"identifier`a` ·illegal`$` ·illegal`é` ·illegal`&` ·identifier`b`"},
		{"invalid UTF-8", "a\xffb", "identifier`a` illegal`\xff` identifier`b`"},
		{"crlf", "a\r\nb", "identifier`a` ⏎identifier`b`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := describe(tt.src); got != tt.want {
				t.Errorf("Lex(%q)\n got: %s\nwant: %s", tt.src, got, tt.want)
			}
		})
	}
}

func TestLexDiagnostics(t *testing.T) {
	tests := []struct {
		src  string
		want []string // "start-end message"
	}{
		{"a + b", nil},
		{"a $ b", []string{"2-3 unexpected character '$'"}},
		{"\xff", []string{"0-1 invalid UTF-8 byte 0xff"}},
		{`"\q"`, []string{`1-3 invalid escape sequence "\\q"`}},
		{"x = \"abc\ny", []string{`4-8 unterminated string: missing "`}},
		{`"a {b`, []string{"3-4 unterminated interpolation: missing }"}},
		{`"{"{`, []string{"3-4 unterminated interpolation: missing }", "1-2 unterminated interpolation: missing }"}},
	}
	for _, tt := range tests {
		_, diags := Lex([]byte(tt.src))
		var got []string
		for _, d := range diags {
			got = append(got, fmt.Sprintf("%d-%d %s", d.Span.Start, d.Span.End, d.Message))
		}
		if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
			t.Errorf("Lex(%q) diagnostics\n got: %q\nwant: %q", tt.src, got, tt.want)
		}
	}
}


// checkRoundTrip checks the lexer's invariants: the tokens cover the source
// exactly, in order, ending with a zero-width EOF; only Missing tokens are
// empty; and the flags agree with the trivia before each token.
func checkRoundTrip(t *testing.T, name string, src []byte) {
	t.Helper()
	tokens, _ := Lex(src)
	if len(tokens) == 0 || tokens[len(tokens)-1].Kind != EOF {
		t.Fatalf("%s: tokens do not end with EOF", name)
	}
	var rebuilt []byte
	pos := 0
	var space, newline bool
	for i, tok := range tokens {
		if tok.Span.Start != pos {
			t.Fatalf("%s: token %d (%s) starts at %d, want %d", name, i, tok.Kind, tok.Span.Start, pos)
		}
		if tok.Span.Len() == 0 && tok.Kind != EOF && tok.Flags&Missing == 0 {
			t.Fatalf("%s: token %d (%s) at %d is empty", name, i, tok.Kind, pos)
		}
		rebuilt = append(rebuilt, src[tok.Span.Start:tok.Span.End]...)
		pos = tok.Span.End
		if tok.Kind.IsTrivia() {
			space = true
			newline = newline || strings.Contains(string(src[tok.Span.Start:tok.Span.End]), "\n")
			continue
		}
		if got := tok.Flags&SpaceBefore != 0; got != space {
			t.Fatalf("%s: token %d (%s) at %d: SpaceBefore = %v, want %v", name, i, tok.Kind, pos, got, space)
		}
		if got := tok.Flags&NewlineBefore != 0; got != newline {
			t.Fatalf("%s: token %d (%s) at %d: NewlineBefore = %v, want %v", name, i, tok.Kind, pos, got, newline)
		}
		space, newline = false, false
	}
	if string(rebuilt) != string(src) {
		t.Fatalf("%s: tokens do not reproduce the source", name)
	}
}

func TestLexRoundTrip(t *testing.T) {
	inputs := exampleInputs(t)
	if len(inputs) < 100 {
		t.Fatalf("found %d example inputs, want the whole corpus and SPECS.md", len(inputs))
	}
	inputs["edge cases"] = "\"unterminated\n\"{a\n\"\\\n$ é \xff \"{\"{"
	for name, src := range inputs {
		checkRoundTrip(t, name, []byte(src))
	}
}

func FuzzLex(f *testing.F) {
	for _, src := range exampleInputs(f) {
		f.Add(src)
	}
	f.Add("\"{\"{\"\\u{")
	f.Fuzz(func(t *testing.T, src string) {
		checkRoundTrip(t, "fuzz input", []byte(src))
	})
}

// exampleInputs returns every input from the tree-sitter corpus and every eye
// example in SPECS.md, keyed by a descriptive name.
func exampleInputs(tb testing.TB) map[string]string {
	tb.Helper()
	inputs := map[string]string{}

	files, err := filepath.Glob("../tree-sitter-eyelang/test/corpus/*.txt")
	if err != nil || len(files) == 0 {
		tb.Fatalf("no corpus files found: %v", err)
	}
	header := regexp.MustCompile(`(?m)^={3,}\n(.*)\n(?::\w+\n)*={3,}\n`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			tb.Fatal(err)
		}
		text := string(data)
		matches := header.FindAllStringSubmatchIndex(text, -1)
		for i, m := range matches {
			end := len(text)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			body := text[m[1]:end]
			if sep := regexp.MustCompile(`(?m)^-{3,}$`).FindStringIndex(body); sep != nil {
				body = body[:sep[0]]
			}
			inputs[filepath.Base(file)+": "+text[m[2]:m[3]]] = strings.Trim(body, "\n") + "\n"
		}
	}

	spec, err := os.ReadFile("../SPECS.md")
	if err != nil {
		tb.Fatal(err)
	}
	block := regexp.MustCompile("(?ms)^[ \t]*```eye\n(.*?)^[ \t]*```")
	for i, m := range block.FindAllStringSubmatch(string(spec), -1) {
		inputs[fmt.Sprintf("SPECS.md example %d", i+1)] = m[1]
	}
	return inputs
}
