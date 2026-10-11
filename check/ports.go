package check

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// Ports and allowed callers (SPECS.md §17). @port(n) on a contract is where
// its service listens, so a contract this program serves must declare one.
// @allow(m, …) lists the modules that may call its operations; a use of an
// operation from any other module is an error at the operation's name. The
// contract's own module, the modules that serve it, and tests may always
// call it. @allow(nobody) lets no module call it, and @allow needs @port.

// contractPolicy reads a contract's @port and @allow. Their arguments'
// kinds are checked with the other attributes; the names in @allow are
// checked once every module is loaded.
func (c *checker) contractPolicy(m *module, f *ir.File, d *syntax.Node, ct *ir.Contract) {
	if args, _, ok := attribute(f, d, "port"); ok && len(args) == 1 && args[0].Kind == syntax.Int {
		n, err := strconv.Atoi(strings.ReplaceAll(text(f, args[0]), "_", ""))
		switch {
		case err != nil || n > 65535:
			c.errorf(pos(f, args[0]), "@port takes a port from 1 to 65535, not %s", text(f, args[0]))
		case n > 0:
			ct.Port = n
		}
	}
	attr := attributeNode(f, d, "allow")
	if attr == nil {
		return
	}
	ct.Guarded = true
	args := attributeArgs(f, d, "allow")
	var names []*syntax.Node // the arguments that are names; others are reported with the attributes
	for _, a := range args {
		if a.Kind == syntax.Ident {
			names = append(names, a)
		}
	}
	for _, n := range names {
		if name := text(f, n); name != "nobody" {
			ct.Allowed = append(ct.Allowed, name)
		} else if len(args) > 1 {
			c.errorf(pos(f, n), "@allow(nobody) lets no module call %s, so it takes no other names", ct.Name)
		}
	}
	if !hasAttribute(f, d, "port") {
		c.errorf(pos(f, attr), "@allow needs @port on %s: the modules it lists are let through to that port, and a contract without one is a service outside the network, which any module may call", ct.Name)
	}
	m.decls = append(m.decls, func() {
		for _, n := range names {
			if name := text(f, n); name != "nobody" && !c.moduleExists(name) {
				c.errorf(pos(f, n), "@allow names the modules that may call %s, and the program has no module %s", ct.Name, name)
			}
		}
	})
}

// moduleExists reports whether the program has a module of this name, or
// could import one: the lab's digest deployed alone still names the lab's
// other modules in Notify's @allow.
func (c *checker) moduleExists(name string) bool {
	for _, m := range c.modules {
		if m.mod.Name == name && m.mod.Dir != "" {
			return true
		}
	}
	for _, root := range c.importRoots {
		if files, _ := filepath.Glob(filepath.Join(root, name, "*.eye")); len(files) > 0 {
			return true
		}
	}
	return false
}

// servedPort reports an impl of a contract without @port: a program that
// serves a contract serves it on the contract's port.
func (c *checker) servedPort(m *module, f *ir.File, d *syntax.Node, ct *ir.Contract) {
	at := d.ChildByField(syntax.FieldType)
	m.decls = append(m.decls, func() {
		if ct.Port == 0 {
			c.errorf(pos(f, at), "this program serves %s, so %s needs @port(…), the port it serves it on, such as @port(3001)", ct.Name, ct.Name)
		}
	})
}

// mayCall reports a use of a contract's operation from a module the
// contract's @allow doesn't list.
func (l *lowerer) mayCall(at *syntax.Node, decl any) {
	op, ok := decl.(*ir.Op)
	if !ok || !op.Contract.Guarded || l.method().kind == ctxTest {
		return
	}
	ct, from := op.Contract, l.m.mod
	if from == ct.Module || from.Path != "" && slices.Contains(ct.Allowed, from.Name) {
		return
	}
	for _, o := range ct.Ops {
		if o.Impl != nil && o.Impl.Module == from {
			return
		}
	}
	who, hint := "module "+from.Name, "add "+from.Name+" there to call it from here"
	if from.Path == "" {
		who, hint = "the program's own module", "call it from a module the list can name"
	}
	if len(ct.Allowed) == 0 {
		l.errorf(at, "%s may not call %s, an operation of %s: @allow(nobody) lets no module of the program call it", who, op.Name, ct.Name)
		return
	}
	l.errorf(at, "%s may not call %s, an operation of %s: its @allow lists only %s; %s", who, op.Name, ct.Name, andList(ct.Allowed), hint)
}

// attributeNode returns a declaration's attribute @name, or nil.
func attributeNode(f *ir.File, d *syntax.Node, name string) *syntax.Node {
	for _, attr := range d.Children {
		if attr.Kind == syntax.Attribute && text(f, attr.ChildByField(syntax.FieldName)) == name {
			return attr
		}
	}
	return nil
}

func andList(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return fmt.Sprintf("%s and %s", strings.Join(words[:len(words)-1], ", "), words[len(words)-1])
}
