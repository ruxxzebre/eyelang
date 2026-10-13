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

// The inbox is a directory a running `eye run` reads requests from. Each
// file is one answer to an approval, written by `eye approve` or `eye
// decline`, which the runner publishes as approvals.answered, waking the run
// waiting for it; or a retry, written by `eye runs --retry` while the runner
// holds the store, which the runner carries out (retry.go). The runner then
// removes the file. A file is written under a temporary name and renamed, so
// the runner never reads half of one.

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

// InboxRetry asks the runner to start failed runs again: those with the ids
// in Retry, or with All, every failed run a retry can cure.
type InboxRetry struct {
	Retry []int64 `json:"retry,omitempty"`
	All   bool    `json:"retryFailed,omitempty"`
}

// inboxFile is what a file in the inbox holds: an answer or a retry.
type inboxFile struct {
	InboxAnswer
	InboxRetry
}

func (f *inboxFile) isRetry() bool { return len(f.Retry) > 0 || f.All }

// SendAnswer puts an answer in the inbox at dir, for the runner to publish.
func SendAnswer(dir string, a InboxAnswer) error {
	if a.ID == "" && strings.TrimSpace(a.What) == "" {
		return fmt.Errorf("an answer needs the id of the approval it answers, or text from what was asked")
	}
	return sendInbox(dir, a)
}

// SendRetry puts a retry in the inbox at dir, for the runner to carry out.
func SendRetry(dir string, r InboxRetry) error {
	if len(r.Retry) == 0 && !r.All {
		return fmt.Errorf("a retry needs the ids of the runs to start again, or all of them")
	}
	return sendInbox(dir, r)
}

// sendInbox writes a request to the inbox at dir.
func sendInbox(dir string, request any) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(request)
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

// readInbox publishes the answers waiting in the inbox and carries out the
// retries, oldest first, and removes each once it is done. A file that can't
// be read as either is renamed to .bad and reported, so it isn't read again.
// An answer that names no waiting approval, or whose text several contain,
// and a retry of a run that a retry can't cure, are reported and removed.
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
		var f inboxFile
		err = json.Unmarshal(data, &f)
		a := f.InboxAnswer
		switch {
		case err == nil && f.isRetry():
			rt.inboxRetry(f.InboxRetry, path, report)
			continue
		case err != nil || (a.ID == "" && strings.TrimSpace(a.What) == ""):
			report(fmt.Sprintf("the inbox file %s isn't an answer or a retry; it is kept as %s.bad", path, name))
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
