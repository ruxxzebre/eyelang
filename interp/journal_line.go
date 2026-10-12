package interp

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// How a commit is written as a line of the journal. A line is the Commit as
// JSON, with two differences that keep it small:
//
//   - A run's record, which is JSON already, is written as JSON, not as
//     base64 of it, which took a third more.
//   - A run whose trail (the steps eye runs shows) only moved on, as it does
//     at each checkpoint, is written without its trail, and with the steps
//     the trail gained and how many of its earlier steps it keeps. The whole
//     trail took about four fifths of a looping job's commit.
//
// Journals written before 2026-10-12 hold each run's record as base64 and
// in full; replay reads both, and the journal is written in this form from
// the snapshot that opening it writes.
type journalLine struct {
	Writes     []Write       `json:",omitempty"`
	Runs       []journalRun  `json:",omitempty"`
	Events     []EventRecord `json:",omitempty"`
	Meta       []MetaWrite   `json:",omitempty"`
	DropEvents int           `json:",omitempty"`
}

// journalRun is a run's record in a line. With Trail, Data is the record
// without its trail, and the trail is the last Trail.Keep steps of the trail
// the run's record had before this line, then Trail.Add.
type journalRun struct {
	ID    int64
	Data  jsonBytes
	Trail *trailMove `json:",omitempty"`
}

type trailMove struct {
	Keep int               `json:",omitempty"`
	Add  []json.RawMessage `json:",omitempty"`
}

// jsonBytes is bytes that are written as they are when they hold a JSON
// object or array, and as base64 otherwise, as []byte is; either is read.
type jsonBytes []byte

func (b jsonBytes) MarshalJSON() ([]byte, error) {
	if len(b) > 0 && (b[0] == '{' || b[0] == '[') && json.Valid(b) {
		return b, nil
	}
	return json.Marshal([]byte(b))
}

func (b *jsonBytes) UnmarshalJSON(data []byte) error {
	switch {
	case len(data) > 0 && data[0] == '"':
		var raw []byte
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		*b = raw
	case string(data) == "null":
		*b = nil
	default:
		*b = append(jsonBytes(nil), data...)
	}
	return nil
}

// trailKey is how the trail's member starts in a run's record: no other
// member of the record, at any depth, has that name.
var trailKey = []byte(`"Trail":`)

// encodeLine writes c as a line of the journal, ending in a line break.
// prev gives a run's record before c, so that a run whose trail moved on is
// written with the steps it gained; with nil, each run is written whole, as
// a snapshot is.
func encodeLine(c *Commit, prev func(id int64) ([]byte, bool)) ([]byte, error) {
	line := journalLine{Writes: c.Writes, Events: c.Events, Meta: c.Meta, DropEvents: c.DropEvents}
	for _, r := range c.Runs {
		jr := journalRun{ID: r.ID, Data: r.Data}
		if prev != nil && bytes.Contains(r.Data, trailKey) {
			if old, ok := prev(r.ID); ok {
				if rest, move, ok := movedTrail(old, r.Data); ok {
					jr.Data, jr.Trail = rest, move
				}
			}
		}
		line.Runs = append(line.Runs, jr)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // a record is written as the runtime made it
	if err := enc.Encode(&line); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// decodeLine reads a line of the journal as the commit it holds. prev gives
// a run's record before the line, which a run written with the steps its
// trail gained is put together from.
func decodeLine(data []byte, prev func(id int64) ([]byte, bool)) (*Commit, error) {
	var line journalLine
	if err := json.Unmarshal(data, &line); err != nil {
		return nil, err
	}
	c := &Commit{Writes: line.Writes, Events: line.Events, Meta: line.Meta, DropEvents: line.DropEvents}
	for _, r := range line.Runs {
		data := []byte(r.Data)
		if r.Trail != nil {
			old, _ := prev(r.ID)
			var err error
			if data, err = withTrail(old, data, r.Trail); err != nil {
				return nil, fmt.Errorf("run %d: %v", r.ID, err)
			}
		}
		c.Runs = append(c.Runs, RunWrite{ID: r.ID, Data: data})
	}
	return c, nil
}

// movedTrail splits a run's new record into the record without its trail
// and how its trail moved on from the old record's. ok is false when the
// new trail shares no steps with the old one, or either record can't be
// read: the record is then written whole.
func movedTrail(old, record []byte) (rest []byte, move *trailMove, ok bool) {
	rest, trail, ok := cutMember(record, "Trail")
	if !ok {
		return nil, nil, false
	}
	var steps, before []json.RawMessage
	if json.Unmarshal(trail, &steps) != nil {
		return nil, nil, false
	}
	if _, was, had := cutMember(old, "Trail"); !had || json.Unmarshal(was, &before) != nil {
		return nil, nil, false
	}
	// keep is the most steps that end the old trail and begin the new one.
	for keep := min(len(before), len(steps)); keep > 0; keep-- {
		if sameSteps(before[len(before)-keep:], steps[:keep]) {
			return rest, &trailMove{Keep: keep, Add: steps[keep:]}, true
		}
	}
	return nil, nil, false
}

func sameSteps(a, b []json.RawMessage) bool {
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// withTrail puts a run's record back together: rest, the record without
// its trail, with the trail that move makes of the old record's. The trail
// goes last, where jobRun has it.
func withTrail(old, rest []byte, move *trailMove) ([]byte, error) {
	if old == nil {
		return nil, fmt.Errorf("its trail follows a record the journal doesn't hold")
	}
	var before []json.RawMessage
	if _, was, had := cutMember(old, "Trail"); had {
		if err := json.Unmarshal(was, &before); err != nil {
			return nil, fmt.Errorf("its earlier trail can't be read: %v", err)
		}
	}
	if move.Keep > len(before) {
		return nil, fmt.Errorf("its trail keeps %d steps of %d", move.Keep, len(before))
	}
	rest = bytes.TrimSpace(rest)
	if len(rest) < 2 || rest[0] != '{' || rest[len(rest)-1] != '}' {
		return nil, fmt.Errorf("its record isn't a JSON object")
	}
	var b bytes.Buffer
	b.Grow(len(rest) + 64*(move.Keep+len(move.Add)))
	b.Write(rest[:len(rest)-1])
	if len(bytes.TrimSpace(rest[1:len(rest)-1])) > 0 {
		b.WriteByte(',')
	}
	b.Write(trailKey)
	b.WriteByte('[')
	steps := append(before[len(before)-move.Keep:len(before):len(before)], move.Add...)
	for i, s := range steps {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(s)
	}
	b.WriteString("]}")
	return b.Bytes(), nil
}

// cutMember splits a JSON object into the object without its member named
// key, and that member's value. ok is false when obj isn't an object or has
// no such member.
func cutMember(obj []byte, key string) (rest, val []byte, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, nil, false
	}
	open := int(dec.InputOffset()) // just past the {
	end := open                    // just past the member before
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, nil, false
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, false
		}
		start, next := end, int(dec.InputOffset())
		end = next
		if name, _ := t.(string); name != key {
			continue
		}
		if start == open {
			// The first member: what follows it loses its comma instead.
			if i := bytes.IndexByte(obj[next:], ','); i >= 0 && dec.More() {
				next += i + 1
			}
		}
		rest = make([]byte, 0, len(obj)-(next-start))
		rest = append(append(rest, obj[:start]...), obj[next:]...)
		return rest, raw, true
	}
	return nil, nil, false
}
