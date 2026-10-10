; Indentation for editors, matching what `eye fmt` produces: two spaces inside each bracket.

[
  (block)
  (entity_body)
  (trait_body)
  (impl_body)
  (contract_body)
  (match_body)
  (arguments)
  (parameters)
  (list)
  (tuple)
  (record)
  (record_update)
  (map)
  (record_type)
  (variant_payload)
] @indent.begin

["}" ")" "]"] @indent.branch @indent.end

(comment) @indent.auto
