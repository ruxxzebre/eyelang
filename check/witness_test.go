package check

import (
	"strings"
	"testing"
)

// chatModule is a contract the witness tests replay recordings of.
const chatModule = `pub type PostResult = | Posted(id: String) | Refused(error: String)
pub type Info = { name: String, members: Int, topic: Option(String), owner: Option(String) }

contract Chat {
  @key(required)
  @reconcile(posted)
  command post(text: String) -> PostResult

  query posted(key: Key) -> Option(PostResult)

  query info(channel: String) -> Result(Info, String)
}
`

const chatRecording = `{ "contract": "Chat",
  "exchanges": [
    { "op": "post", "key": "k-1", "result": { "kind": "Posted", "id": "msg-1" } },
    { "op": "info", "result": { "kind": "Ok", "value": { "name": "general", "members": 3, "topic": null } } }
  ] }`

// witnessDiagnostics checks a program whose main module is src, beside the
// chat module and its recording, and returns its diagnostics as
// "line:column: message".
func witnessDiagnostics(t *testing.T, src string, more map[string]string) []string {
	t.Helper()
	files := map[string]string{
		"main.eye":             src,
		"chat/chat.eye":        chatModule,
		"recordings/chat.json": chatRecording,
	}
	for k, v := range more {
		files[k] = v
	}
	dir := writeProgram(t, files)
	var got []string
	for _, d := range Check(dir, Options{}).Diagnostics {
		got = append(got, strings.TrimPrefix(d.String(), dir+"/main.eye:"))
	}
	return got
}

const replays = `
test "replays" {
  with fakes(chat, from: "recordings/")
}
`

// TestFieldsNoRecordingContains checks that reading a field of a service's
// answer that no recording has is an error at the read (SPECS.md §17), and
// what doesn't count as a read.
func TestFieldsNoRecordingContains(t *testing.T) {
	unwitnessed := func(of, field string) string {
		return of + "'s field " + field + " is in no recording of Chat (recordings/chat.json), so nothing shows the service sends it: record an answer that has it, or mark where it is read @unwitnessed(\"why\")"
	}
	cases := []struct {
		name string
		src  string
		more map[string]string
		want []string
	}{
		{"a variant's field no recording has", `import chat
fn post(text: String, key: Key) {
  match chat.post!(text, key:).reconcile!() {
    Posted(id) => log.info(id)
    Refused(error) => log.warn("the chat refused {key}: {error}")
  }
}` + replays, nil, []string{"5:13: " + unwitnessed("Refused", "error")}},
		{"a variant matched without reading its fields", `import chat
fn post(text: String, key: Key) -> Bool {
  match chat.post!(text, key:).reconcile!() {
    Posted(_) => true
    Refused(..) => false
  }
}
fn wild(text: String, key: Key) -> Bool {
  match chat.post!(text, key:).reconcile!() {
    Posted(..) => true
    Refused(_) => false
  }
}` + replays, nil, nil},
		{"a record's fields, read and matched", `import chat
fn describe() -> String {
  let Ok(i) = chat.info("general") else { return "" }
  let { name, owner, .. } = i
  "{name} {i.members} {i.topic ?? ""} {i.owner ?? ""} {owner ?? ""}"
}` + replays, nil, []string{
			"4:15: " + unwitnessed("Info", "owner"),
			"5:40: " + unwitnessed("Info", "owner"),
		}},
		{"@unwitnessed on the reader or on the type", `import chat
@unwitnessed("Chat refuses only when the channel is archived")
fn post(text: String, key: Key) {
  match chat.post!(text, key:).reconcile!() {
    Posted(id) => log.info(id)
    Refused(error) => log.warn(error)
  }
}
@unwitnessed("the test reads what an archived channel answers")
test "archived" {
  let Ok(i) = chat.info("old") else { return }
  print(i.owner ?? "")
}` + replays, nil, nil},
		{"recordings in the old shape", `import chat
fn post(text: String, key: Key) -> String {
  match chat.post!(text, key:).reconcile!() {
    Posted(id) => id
    Refused(error) => error
  }
}
test "replays" {
  with fakes(chat, from: "old/")
}`, map[string]string{"old/chat.json": `{ "contract": "Chat", "exchanges": [
  { "op": "post", "key": "a", "result": { "Posted": { "id": "m" } } },
  { "op": "post", "key": "b", "result": { "Refused": { "error": "archived" } } }
] }`}, nil},
		{"a contract no test replays", `import chat
fn post(text: String, key: Key) -> String {
  match chat.post!(text, key:).reconcile!() {
    Posted(id) => id
    Refused(error) => error
  }
}`, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := witnessDiagnostics(t, c.src, c.more)
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}
