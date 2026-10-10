package highlight

import (
	"strings"
	"testing"
)

func TestHTML(t *testing.T) {
	src := `@on(enters(idle))
job sleepy(w: Ref(World)) {
  sleep!(20 min)   # waits
  let state = w.read!()
  log.info("at {state.phase}")
}`
	got := HTML(src)
	for _, want := range []string{
		`<span class="a">@</span><span class="a">on</span>`,
		`<span class="k">job</span>`,
		`<span class="t">Ref</span>`,
		`<span class="w">sleep</span><span class="w">!</span>`,
		`<span class="n">20</span> <span class="n">min</span>`,
		`<span class="c"># waits</span>`,
		`<span class="k">let</span> state`,
		`.<span class="w">read</span><span class="w">!</span>`,
		`<span class="s">&#34;</span><span class="s">at </span><span class="s">{</span>state.phase<span class="s">}</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
}

func TestStatementsAndText(t *testing.T) {
	// Statements don't parse at the top level; they are highlighted as a body,
	// and every byte of the input comes out, in order.
	src := "let x = 1 < 2\nif x then print(\"a & b\")"
	got := HTML(src)
	if !strings.Contains(got, `<span class="k">then</span>`) {
		t.Errorf("then isn't a keyword:\n%s", got)
	}
	plain := strings.NewReplacer(`<span class="k">`, "", `<span class="s">`, "", `<span class="n">`, "", `</span>`, "").Replace(got)
	if want := "let x = 1 &lt; 2\nif x then print(&#34;a &amp; b&#34;)"; plain != want {
		t.Errorf("text changed:\n got %q\nwant %q", plain, want)
	}
}
