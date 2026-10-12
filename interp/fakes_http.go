package interp

import (
	"bytes"
	"encoding/json"
	"fmt"

	"eyelang/ir"
)

// HTTP fakes answer http.get from recordings, for services without a
// contract, such as web pages: `with fakes(http, from: "recordings/")` reads
// recordings/http.json. Each exchange is one URL, its "key", and the Result
// http.get returned, in the JSON shape every value travels in:
//
//	{
//	  "contract": "http",
//	  "exchanges": [
//	    { "op": "get", "key": "https://www.dublincity.ie/events",
//	      "result": { "kind": "Ok", "value": { "status": 200, "headers": {}, "body": "<html>…" } } },
//	    { "op": "get", "key": "https://example.com/down",
//	      "result": { "kind": "Err", "value": { "kind": "NoAnswer", "reason": "connection refused" } } }
//	  ]
//	}
//
// An exchange without a key answers every URL that no other names. A URL
// that none answers fails the test, as a contract fake with no exchange does.
type httpFakes struct {
	path    string
	answers map[string]Value // by URL; "" answers the URLs no exchange names
}

// loadHTTPFakes reads the recorded answers to http.get at path, each checked
// against http.get's result type.
func (in *Interp) loadHTTPFakes(path string, at ir.Pos) *httpFakes {
	data, err := in.readFile(path)
	if err != nil {
		in.faultf(at, "no recordings for http: %v", err)
	}
	var rec recording
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&rec); err != nil {
		in.faultf(at, "%s: %v", path, err)
	}
	if rec.Contract != "http" {
		in.faultf(at, `%s records %q; recordings for http say "contract": "http"`, path, rec.Contract)
	}
	get := in.stdModule("http", at).Funcs["get"]
	f := &httpFakes{path: path, answers: map[string]Value{}}
	for i, e := range rec.Exchanges {
		if e.Op != "get" {
			in.faultf(at, "%s: exchange %d: http has no operation %q to fake, only get", path, i+1, e.Op)
		}
		if _, dup := f.answers[e.Key]; dup {
			in.faultf(at, "%s: exchange %d: %s is answered twice", path, i+1, describeURL(e.Key))
		}
		v, err := in.recorded(e.Result, get.Return)
		if err != nil {
			in.faultf(at, "%s: exchange %d (%s): %v", path, i+1, describeURL(e.Key), err)
		}
		f.answers[e.Key] = v
	}
	return f
}

// fakeHTTPGet answers http.get from the test's recordings, if it has any.
func (in *Interp) fakeHTTPGet(params map[string]Value, at ir.Pos) (Value, bool) {
	f := in.rt.httpFakes
	if f == nil {
		return nil, false
	}
	target, _ := params["url"].(string)
	if v, ok := f.answers[target]; ok {
		return v, true
	}
	if v, ok := f.answers[""]; ok {
		return v, true
	}
	in.faultf(at, "%s has no exchange for http.get of %s: add one with \"key\": %q", f.path, target, target)
	return nil, false
}

func describeURL(key string) string {
	if key == "" {
		return "every other URL"
	}
	return fmt.Sprintf("%q", key)
}
