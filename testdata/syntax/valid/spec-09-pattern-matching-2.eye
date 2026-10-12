# SPECS.md section 9, example 2.
type Word = Keyword | Number | Other

fn word(src: Bytes) -> Word = match src {
  "let" | "var" | "continue" => Keyword
  ["0x", _, ..] | ["0b", _, ..] => Number
  _ => Other
}
