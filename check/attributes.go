package check

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"eyelang/ir"
	"eyelang/syntax"
)

// Attributes are declared here: where each one may appear and what it takes.
// Every attribute in a program is checked against its declaration, so a
// misspelt or misplaced attribute, or a wrong argument, is an error rather
// than something the program silently does without (SPECS.md §2, "Typed
// attributes").

// argKind is what an attribute's argument must be.
type argKind int

const (
	aInt      argKind = iota // a whole number above zero
	aNumber                  // a number
	aDuration                // 20 min
	aSize                    // 64 MiB
	aString                  // "text"
	aName                    // a lower-case name
	aZone                    // "Europe/Dublin", or a name for a Zone
	aDeadline                // a Duration, or a time of day with in:
	aTrigger                 // checked when triggers are read
	aStatuses                // { 200: Accepted, 202: Pending }
	aWord                    // one of a fixed set of names
)

type attrArg struct {
	kind  argKind
	words []string // for aWord
}

// attrSpec declares an attribute for some kinds of declaration.
type attrSpec struct {
	places   []string // job, fn, extern, entity, contract, command, query, event, type, test
	pos      []attrArg
	minPos   int
	morePos  *attrArg // any number of further positional arguments
	named    map[string]attrArg
	required []string // named arguments that must be given
	oneOf    []string // at least one of these named arguments must be given
	example  string
}

var attributes = map[string][]attrSpec{
	"on": {{places: []string{"job"}, pos: []attrArg{{kind: aTrigger}}, minPos: 1,
		example: "@on(daily(07:40, in: dublin)), @on(enters(view)), @on(event(notify.answered))"}},
	"priority": {{places: []string{"job"}, minPos: 1,
		pos:     []attrArg{{kind: aWord, words: []string{"interactive", "normal", "background", "batch"}}},
		example: "@priority(background)"}},
	"memory":   {{places: []string{"job"}, pos: []attrArg{{kind: aSize}}, minPos: 1, example: "@memory(64 MiB)"}},
	"deadline": {{places: []string{"job"}, pos: []attrArg{{kind: aDeadline}}, minPos: 1, named: map[string]attrArg{"in": {kind: aZone}}, example: `@deadline(8 h) or @deadline(13:40, in: dublin)`}},
	"concurrency": {{places: []string{"job"}, pos: []attrArg{{kind: aInt}}, minPos: 1,
		named: map[string]attrArg{"by": {kind: aName}}, example: "@concurrency(1), or @concurrency(1, by: chat)"}},
	"limit": {{places: []string{"job"}, pos: []attrArg{{kind: aInt}}, minPos: 1,
		named:    map[string]attrArg{"per": {kind: aDuration}, "by": {kind: aName}},
		required: []string{"per"}, example: "@limit(1, per: 1 s, by: chat)"}},
	"budget": {{places: []string{"job", "fn"},
		named: map[string]attrArg{"usd": {kind: aNumber}, "tokens": {kind: aInt}},
		oneOf: []string{"usd", "tokens"}, example: "@budget(usd: 0.50, tokens: 2_000_000)"}},
	"version": {{places: []string{"entity"}, pos: []attrArg{{kind: aInt}}, minPos: 1, example: "@version(3)"}},
	"port":    {{places: []string{"contract"}, pos: []attrArg{{kind: aInt}}, minPos: 1, example: "@port(3001)"}},
	"allow":   {{places: []string{"contract"}, minPos: 1, morePos: &attrArg{kind: aName}, example: "@allow(digest, inbox)"}},
	"http": {
		{places: []string{"contract"}, named: map[string]attrArg{"names": {kind: aWord, words: []string{"snake"}}},
			oneOf: []string{"names"}, example: "@http(names: snake)"},
		{places: []string{"command", "query"}, pos: []attrArg{{kind: aString}}, minPos: 1,
			named: map[string]attrArg{"status": {kind: aStatuses}}, example: `@http("POST /v1/send", status: { 202: Pending })`},
	},
	"auth": {{places: []string{"contract"},
		named: map[string]attrArg{"bearer": {kind: aString}, "header": {kind: aString}, "secret": {kind: aString}},
		oneOf: []string{"bearer", "header"}, example: `@auth(bearer: "notify-token")`}},
	"key": {
		{places: []string{"command"}, pos: []attrArg{{kind: aWord, words: []string{"required"}}}, minPos: 1,
			named: map[string]attrArg{"header": {kind: aString}, "field": {kind: aString}}, example: `@key(required, header: "Idempotency-Key")`},
		{places: []string{"event"}, pos: []attrArg{{kind: aName}}, minPos: 1, example: "@key(id), naming the field that tells events apart"},
	},
	"reconcile":   {{places: []string{"command"}, pos: []attrArg{{kind: aName}}, minPos: 1, example: "@reconcile(receipt), naming a query"}},
	"fresh":       {{places: []string{"command"}, example: "@fresh"}},
	"destructive": {{places: []string{"command", "fn"}, example: "@destructive"}},
	"unwitnessed": {{places: []string{"type", "fn", "job", "test"}, pos: []attrArg{{kind: aString}}, minPos: 1, example: `@unwitnessed("why")`}},
	"library":     {{places: []string{"extern"}, pos: []attrArg{{kind: aString}}, minPos: 1, example: `@library("z")`}},
	"host":        {{places: []string{"extern"}, pos: []attrArg{{kind: aString}}, minPos: 1, example: `@host("lab.desk")`}},
}

// placeOf names the kind of declaration a node is, as attributes see it.
func placeOf(d *syntax.Node) string {
	switch d.Kind {
	case syntax.JobDeclaration:
		return "job"
	case syntax.FunctionDeclaration, syntax.FunctionSignature:
		return "fn"
	case syntax.ExternDeclaration:
		return "extern"
	case syntax.EntityDeclaration:
		return "entity"
	case syntax.ContractDeclaration:
		return "contract"
	case syntax.CommandDeclaration:
		return "command"
	case syntax.QueryDeclaration:
		return "query"
	case syntax.EventDeclaration:
		return "event"
	case syntax.TypeDeclaration:
		return "type"
	case syntax.TestDeclaration:
		return "test"
	case syntax.ViewDeclaration:
		return "view"
	case syntax.StateDeclaration:
		return "state"
	case syntax.Migration:
		return "migration"
	case syntax.TraitDeclaration:
		return "trait"
	case syntax.ImplDeclaration:
		return "impl"
	}
	return "declaration"
}

// an names a place with its article, for messages.
func an(place string) string {
	switch place {
	case "entity", "extern", "event", "impl":
		return "an " + place
	case "fn":
		return "a function"
	}
	return "a " + place
}

// checkAttributes checks a declaration's attributes, and those of the
// members inside it.
func (c *checker) checkAttributes(f *ir.File, d *syntax.Node) {
	c.checkOwnAttributes(f, d)
	var body *syntax.Node
	switch d.Kind {
	case syntax.EntityDeclaration, syntax.ContractDeclaration, syntax.ImplDeclaration, syntax.TraitDeclaration:
		body = d.ChildByField(syntax.FieldBody)
	}
	if body == nil {
		return
	}
	for _, member := range body.NamedChildren() {
		c.checkOwnAttributes(f, member)
	}
}

func (c *checker) checkOwnAttributes(f *ir.File, d *syntax.Node) {
	place := placeOf(d)
	seen := map[string]bool{}
	for _, attr := range d.Children {
		if attr.Kind != syntax.Attribute {
			continue
		}
		nameNode := attr.ChildByField(syntax.FieldName)
		name := text(f, nameNode)
		at := pos(f, attr)
		specs, known := attributes[name]
		if known {
			spec := GoText{File: "check/attributes.go", Text: `"` + name + `": {`}
			c.use(pos(f, nameNode), spec)
			// Its argument names, and the words it takes, are declared there too.
			if args := attr.ChildByField(syntax.FieldArguments); args != nil {
				for _, a := range args.NamedChildren() {
					if a.Kind == syntax.NamedArgument {
						c.use(pos(f, a.ChildByField(syntax.FieldName)), spec)
					}
				}
			}
		}
		if !known {
			if near := nearestAttribute(name); near != "" {
				c.errorf(at, "@%s isn't an attribute; did you mean @%s?", name, near)
			} else {
				c.errorf(at, "@%s isn't an attribute (SPECS.md §15 lists a job's, §17 a contract's)", name)
			}
			continue
		}
		var spec *attrSpec
		var places []string
		for i := range specs {
			places = append(places, specs[i].places...)
			for _, p := range specs[i].places {
				if p == place {
					spec = &specs[i]
				}
			}
		}
		if spec == nil {
			var where []string
			for _, p := range places {
				where = append(where, an(p))
			}
			c.errorf(at, "@%s goes on %s, not on %s", name, strings.Join(where, " or "), an(place))
			continue
		}
		if seen[name] {
			c.errorf(at, "@%s is given twice", name)
			continue
		}
		seen[name] = true
		c.checkAttributeArgs(f, attr, name, spec)
	}
}

func (c *checker) checkAttributeArgs(f *ir.File, attr *syntax.Node, name string, spec *attrSpec) {
	var pos []*syntax.Node
	named := map[string]*syntax.Node{}
	if args := attr.ChildByField(syntax.FieldArguments); args != nil {
		for _, a := range args.NamedChildren() {
			if a.Kind == syntax.NamedArgument {
				label := text(f, a.ChildByField(syntax.FieldName))
				v := a.ChildByField(syntax.FieldValue)
				if v == nil {
					v = a.ChildByField(syntax.FieldName) // `by:` alone names the variable by
				}
				if _, ok := spec.named[label]; !ok {
					c.errorf(posOf(f, a), "@%s has no argument %s:; it takes %s", name, label, takes(spec))
					continue
				}
				named[label] = v
				continue
			}
			pos = append(pos, a)
		}
	}
	at := posOf(f, attr)
	switch {
	case len(pos) < spec.minPos:
		c.errorf(at, "@%s needs %s: %s", name, plural(spec.minPos, "argument"), spec.example)
		return
	case len(pos) > len(spec.pos) && spec.morePos == nil:
		if len(spec.pos) == 0 {
			c.errorf(posOf(f, pos[len(spec.pos)]), "@%s takes no arguments", name)
		} else {
			c.errorf(posOf(f, pos[len(spec.pos)]), "@%s takes %s: %s", name, plural(len(spec.pos), "argument"), spec.example)
		}
		return
	}
	for i, a := range pos {
		want := spec.morePos
		if i < len(spec.pos) {
			want = &spec.pos[i]
		}
		c.checkAttributeArg(f, a, name, *want, spec)
	}
	for label, v := range named {
		c.checkAttributeArg(f, v, name+" "+label+":", spec.named[label], spec)
	}
	for _, r := range spec.required {
		if named[r] == nil {
			c.errorf(at, "@%s needs %s: %s", name, r+":", spec.example)
		}
	}
	if len(spec.oneOf) > 0 {
		found := false
		for _, o := range spec.oneOf {
			found = found || named[o] != nil
		}
		if !found {
			c.errorf(at, "@%s needs %s: %s", name, orList(spec.oneOf), spec.example)
		}
	}
}

func (c *checker) checkAttributeArg(f *ir.File, a *syntax.Node, what string, want attrArg, spec *attrSpec) {
	at := posOf(f, a)
	bad := func(expected string) {
		c.errorf(at, "@%s takes %s here, not %s: %s", what, expected, text(f, a), spec.example)
	}
	switch want.kind {
	case aInt:
		n, err := strconv.ParseInt(strings.ReplaceAll(text(f, a), "_", ""), 10, 64)
		if a.Kind != syntax.Int || err != nil || n < 1 {
			bad("a whole number above zero")
		}
	case aNumber:
		if a.Kind != syntax.Int && a.Kind != syntax.Float {
			bad("a number")
		}
	case aDuration:
		if a.Kind != syntax.Duration {
			bad("a duration, such as 1 s")
		}
	case aSize:
		if a.Kind != syntax.Size {
			bad("a size, such as 64 MiB")
		}
	case aString:
		if a.Kind != syntax.String {
			bad("a string")
		}
	case aName:
		if a.Kind != syntax.Ident {
			bad("a name")
		}
	case aZone:
		if a.Kind != syntax.String && a.Kind != syntax.Ident {
			bad(`a zone, such as "Europe/Dublin" or dublin`)
		}
	case aDeadline:
		if a.Kind != syntax.Duration && a.Kind != syntax.Time {
			bad("a duration, or a time of day with in:")
		}
	case aWord:
		found := false
		for _, w := range want.words {
			found = found || a.Kind == syntax.Ident && text(f, a) == w
		}
		if !found {
			bad(orList(want.words))
		}
	case aStatuses:
		if a.Kind != syntax.Map {
			bad("statuses and their variants, such as { 202: Pending }")
		}
	case aTrigger:
		// Read with the job's triggers.
	}
}

// takes lists an attribute's named arguments, for messages.
func takes(spec *attrSpec) string {
	var names []string
	for n := range spec.named {
		names = append(names, n+":")
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "no named arguments"
	}
	return orList(names)
}

func orList(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func posOf(f *ir.File, n *syntax.Node) ir.Pos { return pos(f, n) }

// nearestAttribute suggests the declared attribute closest to a misspelt one.
func nearestAttribute(name string) string {
	best, bestDist := "", 3
	for known := range attributes {
		if d := editDistance(name, known); d < bestDist || d == bestDist && known < best {
			best, bestDist = known, d
		}
	}
	if bestDist > 2 {
		return ""
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

// ---------------------------------------------------------------- what the runtime enforces

// jobLimits reads the attributes the runtime enforces on a job's runs:
// @concurrency, @limit and @deadline. Their arguments were checked already.
func (c *checker) jobLimits(m *module, f *ir.File, d *syntax.Node, job *ir.Job) {
	param := func(n *syntax.Node, attr string) int {
		if n == nil {
			return -1
		}
		name := text(f, n)
		c.use(pos(f, n), paramOf(job.Func, name))
		for i, p := range job.Func.Params {
			if p.Name == name {
				return i
			}
		}
		c.errorf(pos(f, n), "@%s's by: names a parameter of %s, and it has no parameter %s", attr, job.Name, name)
		return -1
	}
	if args, named, ok := attribute(f, d, "concurrency"); ok && len(args) == 1 {
		n, _ := strconv.Atoi(strings.ReplaceAll(text(f, args[0]), "_", ""))
		job.Concurrency = &ir.Concurrency{N: n, By: param(named["by"], "concurrency")}
	}
	if args, named, ok := attribute(f, d, "limit"); ok && len(args) == 1 && named["per"] != nil && named["per"].Kind == syntax.Duration {
		n, _ := strconv.Atoi(strings.ReplaceAll(text(f, args[0]), "_", ""))
		per := named["per"]
		job.Limit = &ir.RateLimit{
			N:   n,
			Per: durationOf(text(f, per.ChildByField(syntax.FieldValue)), text(f, per.ChildByField(syntax.FieldUnit))),
			By:  param(named["by"], "limit"),
		}
	}
	if args, named, ok := attribute(f, d, "deadline"); ok && len(args) == 1 {
		l := c.newFunc(m, f, job.Name+" deadline", pos(f, args[0]), ctxGlobal)
		switch a := args[0]; a.Kind {
		case syntax.Duration:
			l.fn.Body = l.expr(a)
		case syntax.Time:
			zone := named["in"]
			if zone == nil {
				c.errorf(pos(f, a), "@deadline at a time of day needs its zone: @deadline(%s, in: dublin)", text(f, a))
				return
			}
			v := ir.Instant{Time: parseTime(text(f, a))}
			if zone.Kind == syntax.Ident {
				l.fn.Body = &ir.InZone{At: l.at(a), Instant: v, Zone: l.name(zone)}
			} else {
				v.Zone = strings.Trim(text(f, zone), `"`)
				l.fn.Body = &ir.Lit{At: l.at(a), Value: v}
			}
		}
		if l.fn.Body != nil {
			job.Deadline = l.done()
		}
	}
}
