---
title: "Grammar sketch"
---

<!-- Generated from SPECS.md by `task generate`. Edit SPECS.md, not this file. -->

# Grammar sketch

A readable summary of [`tree-sitter-eyelang/grammar.js`](https://github.com/ruxxzebre/eyelang/blob/main/tree-sitter-eyelang/grammar.js), which is
the definition. `sep` is `;` or a newline that ends a statement ([section 4](/reference/language/lexical-structure)); `{` marked *block* is
the brace that opens a block. Lists in `( )`, `[ ]` and literal braces allow a trailing comma.
Every `eye` example in this document is a whole file or the statements of a function body, and CI
parses each one with both parsers.

```ebnf
file       = { sep } [ item { sep item } { sep } ] ;
item       = import | fn | extern | job | entity | view | type | trait | impl | contract | test
           | [ "pub" ] let ;
import     = "import" ident { "." ident } [ ".{" name { "," name } "}" ] ;
attribute  = "@" ident [ args ] ;
fn         = { attribute } [ "pub" ] "fn" ident params [ "->" type ] [ uses ] [ where ]
             ( "=" expr | block ) ;
extern     = { attribute } [ "pub" ] "extern" "fn" ident params [ "->" type ] [ uses ] ;
params     = "(" [ param { "," param } ] ")" ;
param      = [ "mut" ] ident [ ":" type ] [ "=" expr ] ;           (* self, mut self *)
uses       = "uses" "{" [ effect { "," effect } ] "}" ;          (* wait, net Llm.complete *)
where      = "where" ident ":" bound { "," ident ":" bound } ;
bound      = typeexpr { "+" typeexpr } ;
job        = { attribute } [ "pub" ] "job" ident params [ "->" type ] [ uses ] block ;
entity     = { attribute } [ "pub" ] "entity" Ident params "{" members "}" ;
member     = { attribute } "state" ident ":" type [ "=" expr ]
           | fn
           | { attribute } "migrate" "from" int "(" ident ")" block ;
view       = { attribute } [ "pub" ] "view" ident "=" expr ;
type       = { attribute } [ "pub" [ "opaque" ] ] "type" Ident [ "(" ident { "," ident } ")" ]
             "=" ( typeexpr | variants ) ;
variants   = [ "|" ] variant "|" variant { "|" variant } | "|" variant ;
variant    = Ident [ "(" ( ident ":" typeexpr | typeexpr ) { "," … } ")" ] ;
trait      = { attribute } [ "pub" ] "trait" Ident [ "(" ident { "," ident } ")" ] "{" { fn | fnsig } "}" ;
impl       = { attribute } "impl" typeexpr [ "for" typeexpr ] "{" { fn } "}" ;
contract   = { attribute } [ "pub" ] "contract" Ident "{" { cmember } "}" ;
cmember    = { attribute } ( "command" | "query" ) ident params "->" typeexpr
           | { attribute } "event" ident ":" typeexpr "on" string ;
test       = { attribute } "test" string block ;

block      = "{"(block) { sep } [ stmt { sep stmt } { sep } ] "}" ;
stmt       = let | "var" ident [ ":" type ] "=" expr | place assign expr
           | "for" pattern "in" expr block | "while" expr block
           | "with" expr | "return" [ expr ] | "break" | "continue" | expr ;
let        = "let" pattern [ ":" typeexpr ] "=" expr [ "else" block ] ;
place      = ident { "." ident } ;
assign     = "=" | "+=" | "-=" | "*=" | "/=" | "%=" ;

expr       = lambda | binary ;                              (* precedence table below *)
lambda     = "fn" [ ident | "_" | params ] ( ":" expr | block ) ;
unary      = ( "-" | "!" ) unary | postfix ;
postfix    = primary { args | "!(" args… | "." name | "[" ( expr | slice ) "]" | "?" } ;
slice      = [ expr ] ( ".." | "..=" ) [ expr ] ;
args       = "(" [ arg { "," arg } ] ")" ;
arg        = [ ( ident | "in" | "for" ) ":" ] ( expr | "mut" place )
           | ident ":" ;                                     (* `key:` is `key: key` *)
primary    = ident | Ident | "_" | literal | "(" expr ")" | "(" expr "," expr { "," expr } ")"
           | "[" [ expr { "," expr } ] "]"
           | "{" "}" | "{" field { "," field } "}" | "{" expr "with" field { "," field } "}"
           | "{" key ":" expr { "," key ":" expr } "}"
           | "if" expr block [ "else" ( block | if ) ]
           | "if" expr "then" branch [ "else" branch ]
           | "match" expr "{" { arm ( "," | sep ) } "}" ;
field      = ident ":" expr | ident ;
key        = string | int | date | time | "(" expr ")" ;
arm        = pattern [ "if" expr ] "=>" ( block | branch ) ;  (* "{" name ":"|","|"with" is a record *)
branch     = expr | place assign expr ;
literal    = int | float | char | string | "true" | "false" | "()" | number unit
           | date | time | date time "in" zone | time "in" zone ;
zone       = string | ident ;
string     = '"' { text | escape | "{" expr "}" } '"'
           | '"""' newline { text | newline | escape | "{" expr "}" } newline indent '"""' ;

pattern    = pattern "|" pattern | ident | "_" | literal | "-" number
           | bound ( ".." | "..=" ) bound                      (* bound: a number or char *)
           | Ident [ "(" [ ppart { "," ppart } ] ")" ]
           | "(" pattern "," pattern { "," pattern } ")"
           | "{" [ ( ident ":" pattern | ident | ".." ) { "," … } ] "}"
           | "[" [ ( pattern | ".." [ ident ] ) { "," … } ] "]" ;
ppart      = pattern | ident ":" pattern | ".." ;
typeexpr   = Ident [ "(" typeexpr { "," typeexpr } ")" ] | ident "." Ident [ "(" … ")" ]
           | ident                                            (* a type variable *)
           | "dyn" typeexpr                                   (* from v0.2 *)
           | "()" | "(" typeexpr "," typeexpr { "," typeexpr } ")"
           | "{" [ ident ":" typeexpr { "," … } ] "}"
           | "fn" "(" [ typeexpr { "," typeexpr } ] ")" [ "->" typeexpr ] ;
```

**Operator precedence**, from tightest to loosest:

| Level | Operators |
|---|---|
| 1 | postfix: call `f(x)`, waiting call `f!(x)`, field `.x`, index `[i]`, slice `[a..b]`, propagate `?` |
| 2 | prefix: `-`, `!` (not) |
| 3 | `*` `/` `%` |
| 4 | `+` `-` |
| 5 | `|>` |
| 6 | `..` `..=` (ranges; they don't chain) |
| 7 | `<` `>` `<=` `>=` |
| 8 | `==` `!=` |
| 9 | `&&` |
| 10 | `||` |
| 11 | `??` (default for `Option`) |
| 12 | lambda body: `fn x: …` takes everything to its right |

`!` directly after an expression and directly before `(` is the waiting marker; `!` before an
expression is logical not. `?` directly after an expression propagates an error; `??` is the
operator. All binary operators associate to the left except `??`, which associates to the right.
