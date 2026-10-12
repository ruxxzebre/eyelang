package cgen

import (
	"fmt"

	"eyelang/lir"
)

var cmpOps = map[string]bool{"==": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true}

// op writes an operator on built-in types, as the interpreter's binary and
// unary do: Int never overflows, the fixed-width integers wrap on their 64
// bits, and times, durations and sizes have their own arithmetic.
func (e *fn) op(x *lir.Op) {
	a := x.Args[0]
	ka := kind(a)
	set := func(format string, args ...any) { e.assign(x.Dst, fmt.Sprintf(format, args...)) }
	if len(x.Args) == 1 {
		switch {
		case x.Op == "!":
			set("!%s", e.v(a))
		case x.Op == "neg" && ka == "Int":
			set("ey_int_neg(%s)", e.v(a))
		case x.Op == "neg" && ka == "Float":
			set("ey_f2v(-ey_v2f(%s))", e.v(a))
		case x.Op == "neg" && (widths[ka].bits > 0 || ka == "Duration" || ka == "Size"):
			set("(ey_v)0 - %s", e.v(a))
		default:
			e.unsupported("the operator " + x.Op + " on " + a.Ty.String())
		}
		return
	}
	b := x.Args[1]
	kb := kind(b)
	va, vb := e.v(a), e.v(b)
	if cmpOps[x.Op] {
		e.compare(x, ka, va, vb)
		return
	}
	switch {
	case ka == "Int" && kb == "Int":
		switch x.Op {
		case "+":
			set("ey_int_add(%s, %s)", va, vb)
		case "-":
			set("ey_int_sub(%s, %s)", va, vb)
		case "*":
			set("ey_int_mul(%s, %s)", va, vb)
		case "/", "%":
			name := map[string]string{"/": "div", "%": "mod"}[x.Op]
			// By a constant other than 0 and -1, which can't fault or
			// overflow, the C compiler divides by multiplying.
			if k, ok := e.smallConst(b); ok && k != 0 && k != -1 {
				set("ey_int_%s_k(%s, %s, INT64_C(%d))", name, e.at, va, k)
			} else {
				set("ey_int_%s(%s, %s, %s)", name, e.at, va, vb)
			}
		default:
			e.unsupported("the operator " + x.Op + " on Int")
		}
	case ka == "Float" && kb == "Float" && (x.Op == "+" || x.Op == "-" || x.Op == "*" || x.Op == "/"):
		set("ey_f2v(ey_v2f(%s) %s ey_v2f(%s))", va, x.Op, vb)
	case ka == "String" && kb == "String" && x.Op == "+":
		set("ey_concat(2, (ey_v[]){%s, %s})", va, vb)
	case ka == "List" && kb == "List" && x.Op == "+":
		set("eyb_list_concat(%s, %s)", va, vb)
	case ka == "Set" && kb == "Set" && x.Op == "+":
		set("eyb_set_union(%s, %s)", va, vb)
	case (ka == "Duration" || ka == "Size") && ka == kb && (x.Op == "+" || x.Op == "-"):
		set("%s %s %s", va, x.Op, vb)
	case (ka == "Duration" || ka == "Size") && kb == "Int" && x.Op == "*":
		set("(ey_v)((int64_t)%s * ey_small_or_fault(%s, %s))", va, e.at, vb)
	case ka == "Int" && (kb == "Duration" || kb == "Size") && x.Op == "*":
		set("(ey_v)(ey_small_or_fault(%s, %s) * (int64_t)%s)", e.at, va, vb)
	case (ka == "Duration" || ka == "Size") && kb == "Int" && x.Op == "/":
		e.printf("if (%s == EY_SMALL(0)) ey_fault(%s, \"division by zero\");", vb, e.at)
		set("(ey_v)((int64_t)%s / ey_small_or_fault(%s, %s))", va, e.at, vb)
	case ka == "Instant" && kb == "Duration" && (x.Op == "+" || x.Op == "-"):
		sign := ""
		if x.Op == "-" {
			sign = "-"
		}
		set("eyb_instant_add(%s, %s(int64_t)%s)", va, sign, vb)
	case ka == "Instant" && kb == "Instant" && x.Op == "-":
		set("(ey_v)eyb_instant_sub(%s, %s)", va, vb)
	case ka == "Date" && kb == "Duration" && (x.Op == "+" || x.Op == "-"):
		set("(ey_v)((int64_t)%s %s (int64_t)%s / INT64_C(86400000000000))", va, x.Op, vb)
	case widths[ka].bits > 0 && ka == kb && (x.Op == "+" || x.Op == "-" || x.Op == "*"):
		set("%s %s %s", va, x.Op, vb)
	default:
		e.unsupported(fmt.Sprintf("%s %s %s", a.Ty, x.Op, b.Ty))
	}
}

// compare writes a comparison of two values of a built-in type.
func (e *fn) compare(x *lir.Op, k, va, vb string) {
	op := x.Op
	set := func(format string, args ...any) { e.assign(x.Dst, fmt.Sprintf(format, args...)) }
	switch k {
	case "Int":
		switch op {
		case "==":
			set("ey_int_eq(%s, %s)", va, vb)
		case "!=":
			set("!ey_int_eq(%s, %s)", va, vb)
		// The inline ey_int_lt compares small Ints without a call.
		case "<":
			set("ey_int_lt(%s, %s)", va, vb)
		case ">":
			set("ey_int_lt(%s, %s)", vb, va)
		case "<=":
			set("!ey_int_lt(%s, %s)", vb, va)
		default: // >=
			set("!ey_int_lt(%s, %s)", va, vb)
		}
	case "Float":
		switch op {
		case "==":
			set("ey_eq(&ey_t_float, %s, %s)", va, vb)
		case "!=":
			set("!ey_eq(&ey_t_float, %s, %s)", va, vb)
		default:
			set("ey_v2f(%s) %s ey_v2f(%s)", va, op, vb)
		}
	case "String", "Zone", "Key":
		switch op {
		case "==":
			set("ey_str_eq(%s, %s)", va, vb)
		case "!=":
			set("!ey_str_eq(%s, %s)", va, vb)
		default:
			set("ey_str_cmp(%s, %s) %s 0", va, vb, op)
		}
	case "Instant":
		set("ey_cmp(&ey_t_instant, %s, %s) %s 0", va, vb, op)
	case "Int32", "Int64", "Char", "Duration", "Size", "Date", "Time":
		set("(int64_t)%s %s (int64_t)%s", va, op, vb)
	case "Bool", "Byte", "UInt32", "UInt64", "()":
		set("%s %s %s", va, op, vb)
	default:
		e.unsupported("comparing " + x.Args[0].Ty.String())
	}
}
