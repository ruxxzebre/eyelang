package specdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsExamples parses every `eye` code block written into the docs site's
// pages, as a file or as the statements of a function body. Code that pages
// include from programs is tested by running those programs (interp's
// TestDocsPrograms); the generated pages come from sources tested elsewhere.
func TestDocsExamples(t *testing.T) {
	generated := []string{"reference/language/", "reference/api/", "design/"}
	err := filepath.WalkDir("../../docs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("../../docs", path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == "node_modules" || rel != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			for _, g := range generated {
				if rel+"/" == g {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		md, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, b := range blocks(string(md)) {
			if !parses(b.src) && !parses("fn example() {\n"+indentLines(b.src)+"}\n") {
				t.Errorf("docs/%s:%d: this eye block doesn't parse, as a file or as statements; mark text that isn't Eyelang as ```text", rel, b.line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type block struct {
	line int
	src  string
}

// blocks returns the eye code blocks of a Markdown page. VitePress lets the
// info string carry more after the language, such as {2,3} to mark lines.
func blocks(md string) []block {
	var out []block
	lines := strings.Split(md, "\n")
	for i := 0; i < len(lines); i++ {
		info := strings.TrimSpace(lines[i])
		if info != "```eye" && !strings.HasPrefix(info, "```eye ") && !strings.HasPrefix(info, "```eye{") {
			continue
		}
		start := i + 2 // the first line of code, counting from 1
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " "))]
		var body []string
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "```"; i++ {
			body = append(body, strings.TrimPrefix(lines[i], indent))
		}
		out = append(out, block{line: start, src: strings.Join(body, "\n") + "\n"})
	}
	return out
}

func TestBlocks(t *testing.T) {
	md := "Text\n\n```eye\nlet x = 1\n```\n\n- item\n\n  ```eye {1}\n  fn f() = 2\n  ```\n\n```text\nnot eye\n```\n"
	got := blocks(md)
	if len(got) != 2 || got[0].src != "let x = 1\n" || got[0].line != 4 || got[1].src != "fn f() = 2\n" {
		t.Errorf("blocks = %+v", got)
	}
}
