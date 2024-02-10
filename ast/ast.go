package ast

import (
	"bytes"
	"encoding/json"
	"eyelang/token"
	"reflect"
	"strings"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement `json:"statements"`
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer

	for _, s := range p.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

type Identifier struct {
	Token token.Token `json:"token" yaml:"token"`
	Value string      `json:"value" yaml:"value"`
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string {
	return i.Value
}

type ReturnStatement struct {
	Token       token.Token `json:"token" yaml:"token"`
	ReturnValue Expression  `json:"returnValue" yaml:"returnValue"`
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer

	out.WriteString(rs.TokenLiteral() + " ")

	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}

	out.WriteString(";")

	return out.String()
}

type LetStatement struct {
	Token token.Token `json:"token" yaml:"token"`
	Name  *Identifier `json:"name" yaml:"name"`
	Value Expression  `json:"value" yaml:"value"`
}

func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer

	out.WriteString(ls.TokenLiteral() + " ")
	out.WriteString(ls.Name.String() + " ")
	out.WriteString("= ")

	if ls.Value != nil {
		out.WriteString(ls.Value.String())
	}

	out.WriteString(";")
	return out.String()

}

type ExpressionStatement struct {
	Token      token.Token `json:"token" yaml:"token"`
	Expression Expression  `json:"expression" yaml:"expression"`
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

type IntegerLiteral struct {
	Token token.Token `json:"token" yaml:"token"`
	Value int64       `json:"value" yaml:"value"`
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token token.Token `json:"token" yaml:"token"`
	Value float64     `json:"value" yaml:"value"`
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type StringLiteral struct {
	Token token.Token `json:"token" yaml:"token"`
	Value string      `json:"value" yaml:"value"`
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }

type PrefixExpression struct {
	Token    token.Token `json:"token" yaml:"token"`       // the prefix token, e.g. !
	Operator string      `json:"operator" yaml:"operator"` // the operator, e.g. !
	Right    Expression  `json:"right" yaml:"right"`       // the right expression, e.g. 5
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")

	return out.String()
}

type InfixExpression struct {
	Token    token.Token `json:"token" yaml:"token"`
	Operator string      `json:"operator" yaml:"operator"`
	Left     Expression  `json:"left" yaml:"left"`
	Right    Expression  `json:"right" yaml:"right`
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")

	return out.String()
}

type Boolean struct {
	Token token.Token
	Value bool
}

func (b *Boolean) expressionNode()      {}
func (b *Boolean) TokenLiteral() string { return b.Token.Literal }
func (b *Boolean) String() string       { return b.Token.Literal }

type BlockStatement struct {
	Token      token.Token `json:"token" yaml:"token"`
	Statements []Statement `json:"statements" yaml:"statements"`
}

func (bs *BlockStatement) expressionNode()      {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer

	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

type IfExpression struct {
	Token       token.Token     `json:"token" yaml:"token"`
	Condition   Expression      `json:"condition" yaml:"condition"`
	Consequence *BlockStatement `json:"consequence" yaml:"consequence"`
	Alternative *BlockStatement `json:"alternative" yaml:"alternative"`
}

func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	var out bytes.Buffer

	out.WriteString("if")
	out.WriteString(ie.Condition.String())
	out.WriteString(" ")
	out.WriteString(ie.Consequence.String())

	if ie.Alternative != nil {
		out.WriteString("else ")
		out.WriteString(ie.Alternative.String())
	}

	return out.String()
}

type FunctionLiteral struct {
	Token      token.Token     `json:"token" yaml:"token"`
	Parameters []*Identifier   `json:"parameters" yaml:"parameters"`
	Body       *BlockStatement `json:"body" yaml:"body"`
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer

	params := []string{}
	for _, param := range fl.Parameters {
		params = append(params, param.String())
	}
	out.WriteString(fl.TokenLiteral())
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	out.WriteString("{ " + fl.Body.String() + " }")

	return out.String()
}

type CallExpression struct {
	Token     token.Token  `json:"token" yaml:"token"`
	Function  Expression   `json:"function" yaml:"function"`
	Arguments []Expression `json:"arguments" yaml:"arguments"`
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer

	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}

	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")

	return out.String()
}

type ArrayLiteral struct {
	Token    token.Token  `json:"token" yaml:"token"`
	Elements []Expression `json:"elements" yaml:"elements"`
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer

	out.WriteString("[")
	elements := []string{}
	for _, expr := range al.Elements {
		elements = append(elements, expr.String())
		// if idx == len(al.Elements)-1 {
		// 	out.WriteString(expr.String())
		// } else {
		// 	out.WriteString(expr.String() + ", ")
		// }
	}

	out.WriteString(strings.Join(elements, ", "))

	out.WriteString("]")
	return out.String()
}

type AccessExpression struct {
	Token token.Token `json:"token" yaml:"token"` // the '.' token
	Left  Expression  `json:"left" yaml:"left"`
	Right Expression  `json:"right" yaml:"right"`
}

func (ae *AccessExpression) expressionNode()      {}
func (ae *AccessExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AccessExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ae.Left.String())
	out.WriteString(".")
	out.WriteString(ae.Right.String())
	out.WriteString(")")

	return out.String()
}

type IndexExpression struct {
	Token token.Token `json:"token" yaml:"token"` // the '[' token
	Left  Expression  `json:"left" yaml:"left"`
	Index Expression  `json:"index" yaml:"index"`
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString("[")
	out.WriteString(ie.Index.String())
	out.WriteString("])")

	return out.String()
}

type HashLiteral struct {
	Token token.Token               `json:"token" yaml:"token"`
	Pairs map[Expression]Expression `json:"pairs" yaml:"pairs"`
}

func (hl *HashLiteral) expressionNode()      {}
func (hl *HashLiteral) TokenLiteral() string { return hl.Token.Literal }
func (hl *HashLiteral) String() string {
	var out bytes.Buffer

	out.WriteString("{")

	kvpairs := []string{}
	for key, value := range hl.Pairs {
		kvpairs = append(kvpairs, key.String()+":"+value.String())
	}

	out.WriteString(strings.Join(kvpairs, ", "))

	out.WriteString("}")

	return out.String()
}

func ToJSON(node interface{}) string {
	bytes, err := json.Marshal(node)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

// func ToYAML(node interface{}) string {
// 	bytes, err := yaml.Marshal(node)
// 	if err != nil {
// 		return err.Error()
// 	}
// 	return string(bytes)
// }

func DebugAst(node interface{}) string {
	panic("not implemented")

	var out bytes.Buffer

	reflect.ValueOf(node)
	switch node := node.(type) {
	case Program:
		for _, stmt := range node.Statements {
			out.WriteString(DebugAst(stmt))
		}
	}

	expr, ok := node.(Expression)
	if ok {
		out.WriteString(expr.String())
	}

	return out.String()
}
