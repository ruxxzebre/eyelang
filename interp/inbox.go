package interp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The inbox is a directory a running `eye run` reads answers from: each file
// is one answer to an approval, written by `eye approve` or `eye decline`.
// The runner publishes it as approvals.answered, which wakes the runs waiting
// for it, and then removes the file. A file is written under a temporary
// name and renamed, so the runner never reads half of one.

// inboxEvery is how often the runner looks in its inbox.
const inboxEvery = time.Second

// InboxAnswer is one answer in the inbox.
type InboxAnswer struct {
	What     string `json:"what"`
	Approved bool   `json:"approved"`
	By       string `json:"by"`
}

// SendAnswer puts an answer in the inbox at dir, for the runner to publish.
func SendAnswer(dir string, a InboxAnswer) error {
	if strings.TrimSpace(a.What) == "" {
		return fmt.Errorf("an answer needs text from what was asked")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%d.json", time.Now().UnixNano(), os.Getpid())
	tmp := filepath.Join(dir, "."+name)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

// readInbox publishes the answers waiting in the inbox, oldest first, and
// removes each once it is published. A file that can't be read as an answer
// is renamed to .bad and reported, so it isn't read again.
func (rt *runtime) readInbox(dir string, report func(string)) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var a InboxAnswer
		if err := json.Unmarshal(data, &a); err != nil || strings.TrimSpace(a.What) == "" {
			report(fmt.Sprintf("the inbox file %s isn't an answer; it is kept as %s.bad", path, name))
			_ = os.Rename(path, path+".bad")
			continue
		}
		by := a.By
		if by == "" {
			by = "eye"
		}
		rt.answer(&Record{Names: []string{"what", "approved", "by"}, Vals: []Value{a.What, a.Approved, by}})
		_ = os.Remove(path) // published: a commit that failed would have panicked before this
	}
}
