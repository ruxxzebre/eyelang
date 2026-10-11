package check

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// Entity versions and migrations (SPECS.md §14, Evolution).
//
// @version(N) numbers the shape of an entity's stored state; every row
// records the version it was saved with. Adding a field with a default
// needs no new version. Any other change raises @version by one and adds
// `migrate from` the old version, a pure function from a row as it was
// stored at that version to the state at the next one. So an entity at
// version N has migrations from some version k to N-1, with no gaps; a row
// saved before k has no way to the current version.

// entityVersions reads an entity's @version and lowers its migrations.
func (c *checker) entityVersions(m *module, f *ir.File, d *syntax.Node, e *ir.Entity) {
	e.Version = 1
	if args := attributeArgs(f, d, "version"); len(args) == 1 && args[0].Kind == syntax.Int {
		e.Version, _ = strconv.Atoi(strings.ReplaceAll(text(f, args[0]), "_", ""))
	}
	from := map[int]bool{}
	for _, member := range d.ChildByField(syntax.FieldBody).NamedChildren() {
		if member.Kind != syntax.Migration {
			continue
		}
		at := pos(f, member)
		k, _ := strconv.Atoi(text(f, member.ChildByField(syntax.FieldVersion)))
		switch {
		case k < 1:
			c.errorf(at, "versions start at 1; there is no version %d to migrate from", k)
			continue
		case k >= e.Version:
			c.errorf(at, "migrate from %d needs %s at a later version: it is at @version(%d), so a migration goes to %d at most",
				k, e.Name, e.Version, e.Version)
			continue
		case from[k]:
			c.errorf(at, "%s has two migrations from version %d", e.Name, k)
			continue
		}
		from[k] = true
		param := member.ChildByField(syntax.FieldParameter)
		l := c.newFunc(m, f, fmt.Sprintf("%s migrate from %d", e.Name, k), at, ctxGlobal)
		l.fn.Params = []*ir.Param{{Name: text(f, param), Slot: l.bind(param, text(f, param), false, false)}}
		l.fn.Body = l.block(member.ChildByField(syntax.FieldBody))
		e.Migrations = append(e.Migrations, &ir.Migration{From: k, Func: l.done(), Pos: at})
	}
	sort.Slice(e.Migrations, func(i, j int) bool { return e.Migrations[i].From < e.Migrations[j].From })
	if e.Version == 1 {
		return
	}
	if !from[e.Version-1] {
		c.errorf(pos(f, d), "%s is at @version(%d), so it needs migrate from %d: adding a field with a default needs no new version, and any other change needs a migration from the old one",
			e.Name, e.Version, e.Version-1)
		return
	}
	for k := e.Migrations[0].From; k < e.Version; k++ {
		if !from[k] {
			c.errorf(pos(f, d), "%s migrates from %d but not from %d, so rows saved at version %d have no way to version %d: add migrate from %d",
				e.Name, k-1, k, k, e.Version, k)
			return
		}
	}
}

// rowType is the type of a row as it was stored at an older version of an
// entity. That shape isn't declared anywhere, so its fields are read by
// name: each field's type is what the migration that made the row gave it,
// or, for the oldest version, what the code that reads it needs.
func (c *checker) rowType(e *ir.Entity, version int) *tcon {
	if c.ts.rows[e] == nil {
		c.ts.rows[e] = map[int]*tcon{}
	}
	if t := c.ts.rows[e][version]; t != nil {
		return t
	}
	d := &ir.Type{Name: fmt.Sprintf("%s at version %d", e.Name, version), Module: e.Module, Methods: map[string]*ir.Func{}, Pos: e.Pos}
	c.ts.rowFields[d] = map[string]typ{}
	t := &tcon{name: d.Name, decl: d}
	c.ts.rows[e][version] = t
	return t
}

// rowField is the type of a field of an old row: one type for each name,
// which its uses decide.
func (c *checker) rowField(d *ir.Type, name string) (typ, bool) {
	fields, ok := c.ts.rowFields[d]
	if !ok {
		return nil, false
	}
	if t, ok := fields[name]; ok {
		return t, true
	}
	t := c.fresh()
	fields[name] = t
	return t, true
}

// migrationResult is the type a migration gives: the entity's state, for the
// migration to the current version, which the checker holds to the state's
// fields; and, for older ones, the row of the next version, which the next
// migration reads by name.
func (c *checker) migrationResult(e *ir.Entity, mg *ir.Migration) typ {
	if mg.From != e.Version-1 {
		return c.rowType(e, mg.From+1)
	}
	// Every state field may be given; those without a default must be.
	rec := &ir.TypeExpr{Name: "record"}
	t := &ir.Type{Name: e.Name + " state", Module: e.Module, Methods: map[string]*ir.Func{}, Pos: e.Pos}
	for _, s := range e.States {
		if s.Default == nil {
			t.Fields = append(t.Fields, s.Name)
		}
		rec.Fields = append(rec.Fields, s.Name)
		rec.Args = append(rec.Args, s.Type)
	}
	t.Alias = rec
	return &tcon{name: t.Name, decl: t}
}

// checkMigrationEffects reports a migration with effects: a migration is a
// pure function of the stored row.
func (c *checker) checkMigrationEffects(all map[*ir.Func]map[string]bool) {
	for _, m := range c.prog.Modules {
		for _, e := range m.Entities {
			for _, mg := range e.Migrations {
				var effects []string
				for eff := range all[mg.Func] {
					effects = append(effects, eff)
				}
				if len(effects) > 0 {
					sort.Strings(effects)
					c.typeErrorf(mg.Pos, "a migration is a pure function of the stored row, and migrate from %d uses %s",
						mg.From, strings.Join(effects, ", "))
				}
			}
		}
	}
}
