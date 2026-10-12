package interp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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
// The record of a run that isn't ready is always written whole, since the
// journal reads it back from where the line has it (handles.go).
//
// Journals written before 2026-10-12 hold each run's record as base64 and
// in full; replay reads both, and the journal is written in this form from
// the snapshot that opening it writes.
//
// A line has the members of a Commit, Writes, Runs, Events, Meta and
// DropEvents, each left out when empty. A run in Runs has ID and Data, and,
// with Trail, Data is the record without its trail, and the trail is the
// last Trail.Keep steps of the trail the run's record had before this line,
// then Trail.Add.
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

// span is where a run's record is in a line: from start to end, written as
// it is. start is -1 for a record written as the steps its trail gained, or
// as base64.
type span struct{ start, end int }

// encodeLine writes c as a line of the journal, ending in a line break, and
// says where in it each run's record is. prev gives the record of c.Runs[i]
// before c, so that a run whose trail moved on is written with the steps it
// gained; with nil, or when it gives none, the run is written whole, as a
// snapshot is, and as a run whose record leaves memory must be.
func encodeLine(c *Commit, prev func(i int) ([]byte, bool)) ([]byte, []span, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // a record is written as the runtime made it
	put := func(v any) error {
		if err := enc.Encode(v); err != nil {
			return err
		}
		b.Truncate(b.Len() - 1) // the line break Encode adds
		return nil
	}
	field := func(name string) {
		if b.Len() > 1 {
			b.WriteByte(',')
		}
		b.WriteString(`"` + name + `":`)
	}
	b.WriteByte('{')
	if len(c.Writes) > 0 {
		field("Writes")
		if err := put(c.Writes); err != nil {
			return nil, nil, err
		}
	}
	var spans []span
	if len(c.Runs) > 0 {
		field("Runs")
		b.WriteByte('[')
		for i, r := range c.Runs {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"ID":` + strconv.FormatInt(r.ID, 10) + `,"Data":`)
			data, move := r.Data, (*trailMove)(nil)
			if prev != nil && bytes.Contains(r.Data, trailKey) {
				if old, ok := prev(i); ok {
					if rest, m, ok := movedTrail(old, r.Data); ok {
						data, move = rest, m
					}
				}
			}
			sp := span{-1, -1}
			if move == nil && len(data) > 0 && data[0] == '{' {
				// Compact leaves b as it was if data isn't JSON.
				if start := b.Len(); json.Compact(&b, data) == nil {
					sp = span{start, b.Len()}
				}
			}
			if sp.start < 0 {
				if err := put(jsonBytes(data)); err != nil {
					return nil, nil, err
				}
			}
			if move != nil {
				b.WriteString(`,"Trail":`)
				if err := put(move); err != nil {
					return nil, nil, err
				}
			}
			b.WriteByte('}')
			spans = append(spans, sp)
		}
		b.WriteByte(']')
	}
	if len(c.Events) > 0 {
		field("Events")
		if err := put(c.Events); err != nil {
			return nil, nil, err
		}
	}
	if len(c.Meta) > 0 {
		field("Meta")
		if err := put(c.Meta); err != nil {
			return nil, nil, err
		}
	}
	if c.DropEvents != 0 {
		field("DropEvents")
		b.WriteString(strconv.Itoa(c.DropEvents))
	}
	b.WriteString("}\n")
	return b.Bytes(), spans, nil
}

// rawLine is a line as replay reads it: each run's record as the bytes the
// line holds, so that replay can tell where it is.
type rawLine struct {
	Writes     []Write       `json:",omitempty"`
	Runs       []rawRun      `json:",omitempty"`
	Events     []EventRecord `json:",omitempty"`
	Meta       []MetaWrite   `json:",omitempty"`
	DropEvents int           `json:",omitempty"`
}

type rawRun struct {
	ID    int64
	Data  json.RawMessage
	Trail *trailMove `json:",omitempty"`
}

// decodeRawLine reads a line of the journal as the commit it holds, and
// gives, for each run written whole as a JSON object, its record as the
// line holds it; nil for the others. prev gives a run's record before the
// line, which a run written with the steps its trail gained is put together
// from.
func decodeRawLine(data []byte, prev func(id int64) ([]byte, bool)) (*Commit, [][]byte, error) {
	var line rawLine
	if err := json.Unmarshal(data, &line); err != nil {
		return nil, nil, err
	}
	c := &Commit{Writes: line.Writes, Events: line.Events, Meta: line.Meta, DropEvents: line.DropEvents}
	raws := make([][]byte, len(line.Runs))
	for i, r := range line.Runs {
		var record jsonBytes
		if err := record.UnmarshalJSON(r.Data); err != nil {
			return nil, nil, fmt.Errorf("run %d: %v", r.ID, err)
		}
		data := []byte(record)
		if r.Trail != nil {
			old, _ := prev(r.ID)
			var err error
			if data, err = withTrail(old, data, r.Trail); err != nil {
				return nil, nil, fmt.Errorf("run %d: %v", r.ID, err)
			}
		} else if len(r.Data) > 0 && r.Data[0] == '{' {
			raws[i] = r.Data
		}
		c.Runs = append(c.Runs, RunWrite{ID: r.ID, Data: data})
	}
	return c, raws, nil
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
