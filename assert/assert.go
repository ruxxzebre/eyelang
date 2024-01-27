package assert

import (
	// "go/types"
	"fmt"
	"reflect"
	"testing"
)

type Assert struct {
	t       *testing.T
	value   any
	reverse bool
}

type PendingAssertions interface {
	Expect(any) FilledAssertions
}

type FilledAssertions interface {
	Equal(any) FilledAssertions
	Not() FilledAssertions
	Type(any) FilledAssertions
}

func New(t *testing.T) PendingAssertions {
	return Assert{t: t, value: nil}
}

func (a Assert) Expect(val any) FilledAssertions {
	c := Assert{t: a.t, value: val}
	return c
}

func (a Assert) Equal(val any) FilledAssertions {
	if a.value == nil {
		a.t.Fatalf("expected %v to equal %v", a.value, val)
	}
	return Assert{
		t:     a.t,
		value: a.value,
	}
}

func (a Assert) Type(t any) FilledAssertions {
	valType := reflect.TypeOf(a.value).Name()
	targetType := reflect.TypeOf(t).Name()
	fmt.Println(valType, targetType, reflect.TypeOf(a.value))
	if valType != targetType {
		a.t.Fatalf("expected '%s' type. got=%s", targetType, valType)
	}
	return a
}

func (a Assert) Not() FilledAssertions {
	return Assert{
		t:       a.t,
		value:   a.value,
		reverse: !a.reverse,
	}
}
