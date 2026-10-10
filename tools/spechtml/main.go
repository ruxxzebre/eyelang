// Command spechtml renders SPECS.md as one HTML page: the reading copy of
// the spec. Eyelang examples are highlighted by Eyelang's own parser. Run it
// from the repository root with `task spec`, which writes bin/spec.html.
package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"eyelang/internal/highlight"
)

//go:embed page.html
var pageTemplate string

func main() {
	in := flag.String("spec", "SPECS.md", "the spec to render")
	out := flag.String("o", "bin/spec.html", "where to write the page")
	repo := flag.String("repo", "https://github.com/ruxxzebre/eyelang/blob/main/", "the URL that links to repository files start with")
	standalone := flag.Bool("standalone", true, "write a whole document; without it, only what goes inside <body>, for publishing")
	flag.Parse()

	src, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	page, err := Render(src, *repo, *standalone)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, page, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s\n", *out)
}

// Render returns the HTML page for a spec.
func Render(spec []byte, repo string, standalone bool) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(
			ghtml.WithUnsafe(),
			renderer.WithNodeRenderers(util.Prioritized(codeRenderer{}, 100)),
		),
	)
	var body bytes.Buffer
	if err := md.Convert(spec, &body); err != nil {
		return nil, err
	}
	b := body.String()

	// The page header has the title, and the first paragraph about the
	// language becomes its lede.
	b = regexp.MustCompile(`(?s)<h1[^>]*>.*?</h1>\s*`).ReplaceAllString(b, "")
	lede := ""
	if m := regexp.MustCompile(`(?s)<p>(Eyelang 2 is .*?)</p>\s*`).FindStringSubmatchIndex(b); m != nil {
		lede = b[m[2]:m[3]]
		b = b[:m[0]] + b[m[1]:]
	}
	// The contents list becomes a row of links.
	b = regexp.MustCompile(`(?s)<h2[^>]*>Contents</h2>\s*<ol>(.*?)</ol>`).ReplaceAllStringFunc(b, func(s string) string {
		links := regexp.MustCompile(`<a href="(#[^"]+)">(.*?)</a>`).FindAllStringSubmatch(s, -1)
		var nav strings.Builder
		nav.WriteString(`<nav class="toc" aria-label="Contents">`)
		for _, l := range links {
			fmt.Fprintf(&nav, "\n<a href=%q>%s</a>", l[1], l[2])
		}
		nav.WriteString("\n</nav>")
		return nav.String()
	})
	b = strings.ReplaceAll(b, "<table>", `<div class="table-wrap"><table>`)
	b = strings.ReplaceAll(b, "</table>", "</table></div>")
	// Links to files of the repository point at the repository.
	b = regexp.MustCompile(`href="([A-Za-z][\w./-]*\.(?:md|js|json|eye))(#[^"]*)?"`).ReplaceAllString(b, `href="`+repo+`$1$2"`)

	t, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = t.Execute(&out, map[string]any{"Lede": lede, "Body": b, "Repo": repo, "Standalone": standalone})
	return out.Bytes(), err
}

// codeRenderer renders fenced code; Eyelang code is highlighted.
type codeRenderer struct{}

func (codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, func(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		block := n.(*ast.FencedCodeBlock)
		var code strings.Builder
		lines := block.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			code.Write(seg.Value(source))
		}
		lang := string(block.Language(source))
		text := strings.TrimRight(code.String(), "\n")
		if lang == "eye" {
			fmt.Fprintf(w, "<pre class=\"eye\"><code>%s</code></pre>\n", highlight.HTML(text))
		} else {
			fmt.Fprintf(w, "<pre class=%q><code>%s</code></pre>\n", lang, html.EscapeString(text))
		}
		return ast.WalkSkipChildren, nil
	})
}
