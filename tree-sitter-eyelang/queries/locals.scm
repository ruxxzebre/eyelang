; Scopes and definitions, for highlighting local names and jumping to them.

[
  (source_file)
  (block)
  (function_declaration)
  (job_declaration)
  (lambda)
  (match_arm)
  (for_statement)
] @local.scope

(parameter name: (identifier) @local.definition.parameter)
(lambda parameter: (identifier) @local.definition.parameter)
(let_statement pattern: (identifier) @local.definition.var)
(var_statement name: (identifier) @local.definition.var)
(for_statement pattern: (identifier) @local.definition.var)
(match_arm pattern: (identifier) @local.definition.var)
(pattern_arguments (identifier) @local.definition.var)
(list_pattern (identifier) @local.definition.var)
(tuple_pattern (identifier) @local.definition.var)
(shorthand_field_pattern (identifier) @local.definition.var)
(function_declaration name: (identifier) @local.definition.function)
(job_declaration name: (identifier) @local.definition.function)
(type_declaration name: (type_identifier) @local.definition.type)

(identifier) @local.reference
