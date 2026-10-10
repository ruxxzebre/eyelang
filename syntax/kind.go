package syntax

// Token kinds that the parser assigns in place of what the lexer produced.
const (
	// Keyword is a contextual keyword such as `migrate`, `uses` or `after`.
	// The lexer reads it as a name; the parser renames it where the grammar
	// expects the keyword.
	Keyword Kind = lastToken + iota
	// UnitName is the unit of a duration or size: ms, min, MiB.
	UnitName

	firstNode
)

// Node kinds. Each matches a named node of tree-sitter-eyelang/grammar.js.
const (
	SourceFile Kind = firstNode + iota
	ErrorNode

	ImportDeclaration
	ImportPath
	Attribute
	FunctionDeclaration
	FunctionSignature
	Parameters
	Parameter
	UsesClause
	Effect
	Path
	JobDeclaration
	EntityDeclaration
	EntityBody
	StateDeclaration
	Migration
	ViewDeclaration
	TypeDeclaration
	TypeParameters
	VariantList
	Variant
	VariantPayload
	FieldDeclaration
	TraitDeclaration
	TraitBody
	ImplDeclaration
	ImplBody
	ContractDeclaration
	ContractBody
	CommandDeclaration
	QueryDeclaration
	EventDeclaration
	TestDeclaration

	Block
	LetStatement
	VarStatement
	Assignment
	ForStatement
	WhileStatement
	WithStatement
	ReturnStatement
	BreakStatement
	ContinueStatement

	Placeholder
	ParenthesizedExpression
	List
	EmptyBraces
	Record
	FieldInitializer
	ShorthandField
	RecordUpdate
	Map
	MapEntry
	Lambda
	IfExpression
	MatchExpression
	MatchBody
	MatchArm
	StartExpression
	CallExpression
	WaitingCallExpression
	Arguments
	NamedArgument
	FieldExpression
	IndexExpression
	TryExpression
	UnaryExpression
	BinaryExpression

	Boolean
	Unit
	Duration
	Size
	Instant
	ZonedTime
	String
	Interpolation

	Wildcard
	NegativeLiteral
	VariantPattern
	PatternArguments
	NamedFieldPattern
	RecordPattern
	ShorthandFieldPattern
	ListPattern
	RestPattern
	OrPattern

	GenericType
	TypeArguments
	QualifiedType
	UnitType
	RecordType
	FunctionType
	FunctionTypeParameters
)

var nodeNames = map[Kind]string{
	Keyword:                 "keyword",
	UnitName:                "unit_name",
	SourceFile:              "source_file",
	ErrorNode:               "ERROR",
	ImportDeclaration:       "import_declaration",
	ImportPath:              "import_path",
	Attribute:               "attribute",
	FunctionDeclaration:     "function_declaration",
	FunctionSignature:       "function_signature",
	Parameters:              "parameters",
	Parameter:               "parameter",
	UsesClause:              "uses_clause",
	Effect:                  "effect",
	Path:                    "path",
	JobDeclaration:          "job_declaration",
	EntityDeclaration:       "entity_declaration",
	EntityBody:              "entity_body",
	StateDeclaration:        "state_declaration",
	Migration:               "migration",
	ViewDeclaration:         "view_declaration",
	TypeDeclaration:         "type_declaration",
	TypeParameters:          "type_parameters",
	VariantList:             "variant_list",
	Variant:                 "variant",
	VariantPayload:          "variant_payload",
	FieldDeclaration:        "field_declaration",
	TraitDeclaration:        "trait_declaration",
	TraitBody:               "trait_body",
	ImplDeclaration:         "impl_declaration",
	ImplBody:                "impl_body",
	ContractDeclaration:     "contract_declaration",
	ContractBody:            "contract_body",
	CommandDeclaration:      "command_declaration",
	QueryDeclaration:        "query_declaration",
	EventDeclaration:        "event_declaration",
	TestDeclaration:         "test_declaration",
	Block:                   "block",
	LetStatement:            "let_statement",
	VarStatement:            "var_statement",
	Assignment:              "assignment",
	ForStatement:            "for_statement",
	WhileStatement:          "while_statement",
	WithStatement:           "with_statement",
	ReturnStatement:         "return_statement",
	BreakStatement:          "break_statement",
	ContinueStatement:       "continue_statement",
	Placeholder:             "placeholder",
	ParenthesizedExpression: "parenthesized_expression",
	List:                    "list",
	EmptyBraces:             "empty_braces",
	Record:                  "record",
	FieldInitializer:        "field_initializer",
	ShorthandField:          "shorthand_field",
	RecordUpdate:            "record_update",
	Map:                     "map",
	MapEntry:                "map_entry",
	Lambda:                  "lambda",
	IfExpression:            "if_expression",
	MatchExpression:         "match_expression",
	MatchBody:               "match_body",
	MatchArm:                "match_arm",
	StartExpression:         "start_expression",
	CallExpression:          "call_expression",
	WaitingCallExpression:   "waiting_call_expression",
	Arguments:               "arguments",
	NamedArgument:           "named_argument",
	FieldExpression:         "field_expression",
	IndexExpression:         "index_expression",
	TryExpression:           "try_expression",
	UnaryExpression:         "unary_expression",
	BinaryExpression:        "binary_expression",
	Boolean:                 "boolean",
	Unit:                    "unit",
	Duration:                "duration",
	Size:                    "size",
	Instant:                 "instant",
	ZonedTime:               "zoned_time",
	String:                  "string",
	Interpolation:           "interpolation",
	Wildcard:                "wildcard",
	NegativeLiteral:         "negative_literal",
	VariantPattern:          "variant_pattern",
	PatternArguments:        "pattern_arguments",
	NamedFieldPattern:       "field_pattern",
	RecordPattern:           "record_pattern",
	ShorthandFieldPattern:   "shorthand_field_pattern",
	ListPattern:             "list_pattern",
	RestPattern:             "rest_pattern",
	OrPattern:               "or_pattern",
	GenericType:             "generic_type",
	TypeArguments:           "type_arguments",
	QualifiedType:           "qualified_type",
	UnitType:                "unit_type",
	RecordType:              "record_type",
	FunctionType:            "function_type",
	FunctionTypeParameters:  "function_type_parameters",
}

// IsNamed reports whether nodes of this kind are named nodes in the
// tree-sitter grammar. Punctuation, keywords and quotes are anonymous.
func (k Kind) IsNamed() bool {
	switch k {
	case Ident, TypeIdent, Int, Float, Date, Time, StringText, StringEscape, UnitName, Comment, Illegal:
		return true
	}
	return k >= firstNode
}

// Field is the role a node plays in its parent, named as in the grammar's
// field() calls.
type Field uint8

// Fields, named as in tree-sitter-eyelang/grammar.js.
const (
	NoField Field = iota
	FieldAlternative
	FieldArguments
	FieldBase
	FieldBody
	FieldCall
	FieldCondition
	FieldConsequence
	FieldDate
	FieldDefault
	FieldEffects
	FieldField
	FieldFunction
	FieldGuard
	FieldIndex
	FieldItem
	FieldKey
	FieldKind
	FieldLeft
	FieldModule
	FieldName
	FieldOperand
	FieldOperator
	FieldParameter
	FieldParameters
	FieldPath
	FieldPattern
	FieldPayload
	FieldReturnType
	FieldRight
	FieldTarget
	FieldTime
	FieldTiming
	FieldTopic
	FieldTrait
	FieldType
	FieldTypeParameters
	FieldUnit
	FieldValue
	FieldVersion
	FieldZone
)

var fieldNames = [...]string{
	NoField:             "",
	FieldAlternative:    "alternative",
	FieldArguments:      "arguments",
	FieldBase:           "base",
	FieldBody:           "body",
	FieldCall:           "call",
	FieldCondition:      "condition",
	FieldConsequence:    "consequence",
	FieldDate:           "date",
	FieldDefault:        "default",
	FieldEffects:        "effects",
	FieldField:          "field",
	FieldFunction:       "function",
	FieldGuard:          "guard",
	FieldIndex:          "index",
	FieldItem:           "item",
	FieldKey:            "key",
	FieldKind:           "kind",
	FieldLeft:           "left",
	FieldModule:         "module",
	FieldName:           "name",
	FieldOperand:        "operand",
	FieldOperator:       "operator",
	FieldParameter:      "parameter",
	FieldParameters:     "parameters",
	FieldPath:           "path",
	FieldPattern:        "pattern",
	FieldPayload:        "payload",
	FieldReturnType:     "return_type",
	FieldRight:          "right",
	FieldTarget:         "target",
	FieldTime:           "time",
	FieldTiming:         "timing",
	FieldTopic:          "topic",
	FieldTrait:          "trait",
	FieldType:           "type",
	FieldTypeParameters: "type_parameters",
	FieldUnit:           "unit",
	FieldValue:          "value",
	FieldVersion:        "version",
	FieldZone:           "zone",
}

func (f Field) String() string { return fieldNames[f] }
