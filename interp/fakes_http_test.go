package interp

import (
	"strings"
	"testing"
)

// fetchProgram reads URLs with http.get and shows what came back.
const fetchProgram = `import http

fn fetch(url: String) -> String uses {net} =
  match http.get(url, via: Some("http://proxy.example:8888")) {
    Ok(r) => "{r.status}: {r.body}"
    Err(e) => "{e}"
  }

test "pages" {
  with fakes(http, from: "rec/")
  expect(fetch("https://a.example/") == "200: hello")
  expect(fetch("https://b.example/") == "NoAnswer(reason: \"refused\")")
  expect(fetch("https://elsewhere.example/") == "404: none")
}
`

// runFetchTest runs fetchProgram's test with rec/http.json as given, and
// returns its failure, "" if it passed.
func runFetchTest(t *testing.T, recording string) string {
	t.Helper()
	prog := loadFiles(t, map[string]string{"main.eye": fetchProgram, "rec/http.json": recording})
	results := RunTests(prog, "")
	if len(results) != 1 {
		t.Fatalf("%d test results, want 1", len(results))
	}
	return results[0].Failure
}

func TestHTTPFakesAnswerByURL(t *testing.T) {
	failure := runFetchTest(t, `{
	  "contract": "http",
	  "exchanges": [
	    { "op": "get", "key": "https://a.example/",
	      "result": { "kind": "Ok", "value": { "status": 200, "headers": {}, "body": "hello" } } },
	    { "op": "get", "key": "https://b.example/",
	      "result": { "kind": "Err", "value": { "kind": "NoAnswer", "reason": "refused" } } },
	    { "op": "get",
	      "result": { "kind": "Ok", "value": { "status": 404, "headers": { "content-type": "text/plain" }, "body": "none" } } }
	  ]
	}`)
	if failure != "" {
		t.Error(failure)
	}
}

func TestHTTPFakesRefuseWhatTheyCantAnswer(t *testing.T) {
	ok := `{ "op": "get", "key": "https://a.example/", "result": { "kind": "Ok", "value": { "status": 200, "headers": {}, "body": "hello" } } }`
	cases := []struct{ name, recording, want string }{
		{"a URL nobody recorded", `{"contract": "http", "exchanges": [` + ok + `]}`,
			`has no exchange for http.get of https://b.example/: add one with "key": "https://b.example/"`},
		{"another contract's recording", `{"contract": "Notify", "exchanges": []}`,
			`records "Notify"; recordings for http say "contract": "http"`},
		{"an operation http doesn't have", `{"contract": "http", "exchanges": [{ "op": "post", "result": null }]}`,
			`exchange 1: http has no operation "post" to fake, only get`},
		{"a URL answered twice", `{"contract": "http", "exchanges": [` + ok + `,` + ok + `]}`,
			`exchange 2: "https://a.example/" is answered twice`},
		{"a result that isn't http.get's", `{"contract": "http", "exchanges": [{ "op": "get", "key": "https://a.example/", "result": { "kind": "Ok", "value": { "status": "fine" } } }]}`,
			`exchange 1 ("https://a.example/"):`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if failure := runFetchTest(t, c.recording); !strings.Contains(failure, c.want) {
				t.Errorf("the test failed with %q, want it to say %q", failure, c.want)
			}
		})
	}
}
