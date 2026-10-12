// Package wire holds the rules for how a contract's values and operations go
// over HTTP (SPECS.md §17, "Values as JSON" and "Over HTTP"). The interpreter
// follows them when it calls a contract and when eye run serves one, and
// eye gen go|ts (internal/codegen) writes them into the clients and server
// stubs it generates, so the two sides read the same rules and can't drift
// apart.
package wire

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"eyelang/ir"
)

// DefaultKeyHeader is the header that carries a command's key when its @key
// names none.
const DefaultKeyHeader = "Idempotency-Key"

// Name is a field's or a variant's name on the wire. With snake set (from
// @http(names: snake)) it is snake_case: dedupeKey is dedupe_key, and
// variant Normal is "normal". A variant's "kind" tag keeps its name as it is.
func Name(name string, snake bool) string {
	if !snake {
		return name
	}
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// FieldName is the JSON name of a variant's field i, before Name: its own,
// or "value" for a positional field ("value1", "value2", … when the variant
// has several).
func FieldName(v *ir.Variant, i int) string {
	if i < len(v.Fields) && v.Fields[i].Name != "" {
		return v.Fields[i].Name
	}
	if len(v.Fields) == 1 {
		return "value"
	}
	return fmt.Sprintf("value%d", i+1)
}

// IsEnum reports whether a type's variants all have no fields, so that its
// values are strings on the wire: `Normal` is "Normal".
func IsEnum(t *ir.Type) bool {
	if len(t.Variants) == 0 {
		return false
	}
	for _, v := range t.Variants {
		if len(v.Fields) > 0 {
			return false
		}
	}
	return true
}

// InQuery reports whether a request of this method sends its arguments as
// query parameters (GET and DELETE) rather than as a JSON object.
func InQuery(method string) bool {
	return method == "GET" || method == "DELETE"
}

// KeyHeader is the header that carries a command's key.
func KeyHeader(op *ir.Op) string {
	if op.KeyHeader != "" {
		return op.KeyHeader
	}
	return DefaultKeyHeader
}

// PathParams are the names of the arguments an operation's path takes, in
// order: "/v1/categories/:category/entries" takes category.
func PathParams(path string) []string {
	var out []string
	for _, seg := range strings.Split(path, "/") {
		if name, ok := strings.CutPrefix(seg, ":"); ok {
			out = append(out, name)
		}
	}
	return out
}

// ReplyStatus is the status a served operation answers a result of this
// variant with: the lowest that `status:` gives the variant, else 200.
func ReplyStatus(op *ir.Op, variant string) int {
	best := 0
	if op.HTTP != nil {
		for code, name := range op.HTTP.Status {
			if name == variant && (best == 0 || code < best) {
				best = code
			}
		}
	}
	if best == 0 {
		return 200
	}
	return best
}

// Statuses are the codes `status:` maps to a variant, lowest first.
func Statuses(op *ir.Op) []int {
	if op.HTTP == nil {
		return nil
	}
	codes := make([]int, 0, len(op.HTTP.Status))
	for code := range op.HTTP.Status {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	return codes
}

// Status codes a @fresh command's service answers with when `status:`
// doesn't map them: the state changed after basedOn, or what the command was
// based on is gone.
const (
	StaleStatus   = 409
	ExpiredStatus = 410
)

// HTTPErrorType returns http's HttpError when t is Result(T, http.HttpError):
// a query that handles its own failures, which come back as Err instead of
// being retried.
func HTTPErrorType(t *ir.TypeExpr) *ir.Type {
	if t == nil || t.Name != "Result" || len(t.Args) != 2 {
		return nil
	}
	e := t.Args[1].Decl
	if e == nil || e.Name != "HttpError" || e.Module == nil || e.Module.Path != "http" {
		return nil
	}
	return e
}

// HTTPOps are a contract's operations that have @http, in source order: the
// ones a client can call and a server answers. The others are reached only
// from inside the program that serves the contract.
func HTTPOps(ct *ir.Contract) (routed, internal []*ir.Op) {
	for _, op := range SortedOps(ct) {
		if op.HTTP != nil {
			routed = append(routed, op)
		} else {
			internal = append(internal, op)
		}
	}
	return routed, internal
}

// SortedOps are a contract's operations in source order.
func SortedOps(ct *ir.Contract) []*ir.Op {
	ops := make([]*ir.Op, 0, len(ct.Ops))
	for _, op := range ct.Ops {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Pos.Offset != ops[j].Pos.Offset {
			return ops[i].Pos.Offset < ops[j].Pos.Offset
		}
		return ops[i].Name < ops[j].Name
	})
	return ops
}
