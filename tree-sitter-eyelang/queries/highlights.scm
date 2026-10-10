; Highlighting for Eyelang, using nvim-treesitter's capture names. More specific patterns come
; later and win, as in Neovim.

; Names
(identifier) @variable
(type_identifier) @type
(type_variable) @type
((identifier) @variable.builtin
  (#eq? @variable.builtin "self"))

; Declarations
(function_declaration name: (identifier) @function)
(function_signature name: (identifier) @function)
(extern_declaration name: (identifier) @function)
(job_declaration name: (identifier) @function)
(view_declaration name: (identifier) @variable)
(entity_declaration name: (type_identifier) @type.definition)
(type_declaration name: (type_identifier) @type.definition)
(trait_declaration name: (type_identifier) @type.definition)
(contract_declaration name: (type_identifier) @type.definition)
(command_declaration name: (identifier) @function)
(query_declaration name: (identifier) @function)
(event_declaration name: (identifier) @property)
(type_parameters (identifier) @type)
(requirement name: (identifier) @type)

(parameter name: (identifier) @variable.parameter)
(lambda parameter: (identifier) @variable.parameter)
(named_argument name: (identifier) @variable.parameter)

(field_declaration name: (identifier) @property)
(state_declaration name: (identifier) @property)
(field_initializer name: (identifier) @property)
(shorthand_field (identifier) @property)
(field_pattern name: (identifier) @property)
(shorthand_field_pattern (identifier) @property)
(field_expression field: (identifier) @property)

(import_path (identifier) @module)
(qualified_type module: (identifier) @module)

; Variants and constructors
(variant name: (type_identifier) @constructor)
(variant_pattern name: (type_identifier) @constructor)
(call_expression function: (type_identifier) @constructor)
(binary_expression [left: (type_identifier) right: (type_identifier)] @constructor)
(match_arm value: (type_identifier) @constructor)
(arguments (type_identifier) @constructor)

; Calls
(call_expression function: (identifier) @function.call)
(call_expression function: (field_expression field: (identifier) @function.method.call))

; Waiting calls are durable steps, so they stand out like `await` does elsewhere.
(waiting_call_expression function: (identifier) @keyword.coroutine)
(waiting_call_expression function: (field_expression field: (identifier) @keyword.coroutine))
(waiting_call_expression "!" @keyword.coroutine)

; Attributes and effects
(attribute "@" @attribute)
(attribute name: (identifier) @attribute)
(effect kind: (identifier) @keyword.modifier)

; Literals
(integer) @number
(float) @number.float
(boolean) @boolean
(char) @character
(string) @string
(escape_sequence) @string.escape
(interpolation ["{" "}"] @punctuation.special)
[(date) (time)] @string.special
(unit_name) @constant.builtin
[(unit) (unit_type) (empty_braces)] @constant.builtin
[(placeholder) (wildcard)] @character.special

; Comments
(comment) @comment @spell
((comment) @comment.documentation
  (#match? @comment.documentation "^##"))

; Keywords
["if" "then" "else" "match"] @keyword.conditional
["for" "while"] @keyword.repeat
[(break_statement) (continue_statement)] @keyword.repeat
"in" @keyword.operator
"return" @keyword.return
"fn" @keyword.function
"import" @keyword.import
["type" "trait" "impl" "entity" "contract" "extern" "dyn"] @keyword.type
["pub" "opaque" "mut"] @keyword.modifier
["let" "var" "state" "job" "view" "test" "with" "where" "uses" "migrate" "from" "command" "query"
  "event" "on"] @keyword

; Operators and punctuation
["+" "-" "*" "/" "%" "==" "!=" "<" "<=" ">" ">=" "&&" "||" "!" "??" "|>" "=" "+=" "-=" "*=" "/="
  "%=" "->" "=>" ".." "..=" "|"] @operator
(try_expression "?" @operator)
["(" ")" "[" "]" "{" "}"] @punctuation.bracket
["," ";" ":" "." ".{"] @punctuation.delimiter
