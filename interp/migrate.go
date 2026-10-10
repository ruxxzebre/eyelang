package interp

import (
	"fmt"
	"sort"
	"strings"

	"eyelang/ir"
)

// Entity versions (SPECS.md §14, Evolution). A row records the @version its
// state was saved with. Loading a row saved at an older version runs the
// entity's migrations in turn, from that version to the current one; then
// every state field the row lacks gets its default, and fields the entity no
// longer declares are dropped. The row is saved at the current version the
// next time a transaction changes the entity.

// schemaOf is the version a row was saved at: 0, from journals written
// before entities had versions, is 1.
func schemaOf(row Row) int { return max(row.Schema, 1) }

func entityVersion(e *ir.Entity) int { return max(e.Version, 1) }

// decodeRow reads an entity's saved state, as the current version declares
// it.
func (rt *runtime) decodeRow(e *ir.Entity, row Row, at ir.Pos) *Record {
	state := rt.in.decode(row.State, at).(*Record)
	from, current := schemaOf(row), entityVersion(e)
	if from > current {
		rt.in.faultf(at, "a %s was saved at version %d, and this program's %s is at @version(%d): it is older than the data", e.Name, from, e.Name, current)
	}
	for v := from; v < current; v++ {
		mg := migrationFrom(e, v)
		if mg == nil {
			rt.in.faultf(at, "a %s was saved at version %d, and %s has no migrate from %d", e.Name, v, e.Name, v)
		}
		next := rt.in.evalMigration(mg, state)
		rec, ok := next.(*Record)
		if !ok {
			rt.in.faultf(mg.Pos, "migrate from %d gives %s, not the state as a record", v, kindName(next))
		}
		if id, ok := state.get(e.ID.Name); ok {
			if _, has := rec.get(e.ID.Name); !has {
				rec = &Record{Names: append([]string{e.ID.Name}, rec.Names...), Vals: append([]Value{id}, rec.Vals...)}
			}
		}
		state = rec
	}
	return rt.fitState(e, state, at)
}

func migrationFrom(e *ir.Entity, v int) *ir.Migration {
	for _, mg := range e.Migrations {
		if mg.From == v {
			return mg
		}
	}
	return nil
}

// evalMigration runs a migration on a row's state, apart from whatever the
// interpreter is in the middle of.
func (in *Interp) evalMigration(mg *ir.Migration, old *Record) Value {
	prevRun, prevScope := in.run, in.scope
	in.run, in.scope = nil, nil
	defer func() { in.run, in.scope = prevRun, prevScope }()
	v, _, _ := in.callFunc(mg.Func, nil, nil, []argVal{{v: old}}, mg.Pos)
	return v
}

// fitState gives a state the fields the entity declares, in their order: the
// identity, then each state field, with its default where the row lacks it.
func (rt *runtime) fitState(e *ir.Entity, state *Record, at ir.Pos) *Record {
	if len(state.Names) == len(e.States)+1 && state.Names[0] == e.ID.Name {
		same := true
		for i, s := range e.States {
			same = same && state.Names[i+1] == s.Name
		}
		if same {
			return state
		}
	}
	id, _ := state.get(e.ID.Name)
	out := &Record{Names: []string{e.ID.Name}, Vals: []Value{id}}
	for _, s := range e.States {
		v, ok := state.get(s.Name)
		if !ok {
			if s.Default == nil {
				rt.in.faultf(at, "%s.%s has no default, and a %s saved before it was added doesn't have it: give it a default, or a migration", e.Name, s.Name, e.Name)
			}
			v = rt.in.evalGlobalFunc(s.Default)
		}
		out.Names = append(out.Names, s.Name)
		out.Vals = append(out.Vals, coerce(v, s.Type))
	}
	return out
}

// checkVersions finds stored rows that can't reach their entity's current
// version: saved at a newer version than the program's, or at one older than
// its oldest migration. `eye run` refuses to start on them, rather than
// fault on each when it is loaded.
func (rt *runtime) checkVersions() error {
	var problems []string
	for _, m := range rt.in.prog.Modules {
		var names []string
		for name := range m.Entities {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			e := m.Entities[name]
			current := entityVersion(e)
			oldest := current
			if len(e.Migrations) > 0 {
				oldest = e.Migrations[0].From
			}
			bad := map[int]int{}
			for _, key := range rt.store.Keys(entityName(e)) {
				row, _ := rt.store.Load(key)
				if v := schemaOf(row); v > current || v < oldest {
					bad[v]++
				}
			}
			var versions []int
			for v := range bad {
				versions = append(versions, v)
			}
			sort.Ints(versions)
			for _, v := range versions {
				if v > current {
					problems = append(problems, fmt.Sprintf("%d %s saved at version %d, newer than the program's @version(%d)", bad[v], e.Name, v, current))
				} else {
					problems = append(problems, fmt.Sprintf("%d %s saved at version %d, and %s has no migrate from %d", bad[v], e.Name, v, e.Name, v))
				}
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("stored entities can't reach this program's versions: %s", strings.Join(problems, "; "))
}
