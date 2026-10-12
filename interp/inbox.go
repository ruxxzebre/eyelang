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
// The runner publishes it as approvals.answered, which wakes the run waiting
// for it, and then removes the file. A file is written under a temporary
// name and renamed, so the runner never reads half of one.

// inboxEvery is how often the runner looks in its inbox.
const inboxEvery = time.Second

// InboxAnswer is one answer in the inbox. It names its request by ID, or by
// What, text that only that request contains, when `eye approve` couldn't
// read the journal to find the id. Without By, the person asked answers.
type InboxAnswer struct {
	ID       string `json:"id,omitempty"`
	What     string `json:"what,omitempty"`
	Approved bool   `json:"approved"`
	By       string `json:"by,omitempty"`
}

// SendAnswer puts an answer in the inbox at dir, for the runner to publish.
func SendAnswer(dir string, a InboxAnswer) error {
	if a.ID == "" && strings.TrimSpace(a.What) == "" {
		return fmt.Errorf("an answer needs the id of the approval it answers, or text from what was asked")
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
// is renamed to .bad and reported, so it isn't read again. An answer that
// names no waiting approval, or whose text several contain, is reported and
// removed.
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
		if err := json.Unmarshal(data, &a); err != nil || (a.ID == "" && strings.TrimSpace(a.What) == "") {
			report(fmt.Sprintf("the inbox file %s isn't an answer; it is kept as %s.bad", path, name))
			_ = os.Rename(path, path+".bad")
			continue
		}
		if req, problem := rt.inboxRequest(a); problem != "" {
			report(problem + "; the answer in " + path + " is dropped")
		} else {
			by := a.By
			if by == "" {
				by = req.from
			}
			rt.answer(req.id, a.Approved, by)
		}
		rt.durable()        // the answer is gone once its file is
		_ = os.Remove(path) // published: a commit that failed would have panicked before this
	}
}

// inboxRequest finds the waiting approval an inbox answer names, or says why
// there is none.
func (rt *runtime) inboxRequest(a InboxAnswer) (asked, string) {
	if a.ID != "" {
		for _, w := range rt.waitingApprovals() {
			if w.id == a.ID {
				return w, ""
			}
		}
		return asked{}, fmt.Sprintf("no run waits for the approval %s", a.ID)
	}
	waiting := rt.approvalsWith(a.What)
	switch len(waiting) {
	case 0:
		return asked{}, fmt.Sprintf("no run waits for an approval whose text contains %q", a.What)
	case 1:
		return waiting[0], ""
	}
	ids := make([]string, len(waiting))
	for i, w := range waiting {
		ids[i] = w.id
	}
	return asked{}, fmt.Sprintf("%d approvals contain %q (%s), and an answer goes to one", len(waiting), a.What, strings.Join(ids, ", "))
}
