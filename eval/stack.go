package eval

import "eyelang/object"

type Stack struct {
	ir []*object.Function // internal representation
}

func (s *Stack) Push(fn *object.Function) {
	s.ir = append(s.ir, fn)
}

func (s *Stack) Pop() *object.Function {
	last := len(s.ir) - 1
	fn := s.ir[last]
	s.ir = s.ir[:last]
	return fn
}

func (s *Stack) Size() int {
	return len(s.ir)
}

func NewStack() *Stack {
	return &Stack{
		ir: []*object.Function{},
	}
}
