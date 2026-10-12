## Regular expressions: patterns that find, replace and split text.
##
## A pattern is written in RE2's syntax, the one Go's regexp package reads:
## `\d+`, `[a-z]{2,3}`, `(?i)` to ignore case, `(?P<year>\d{4})` or
## `(?<year>\d{4})` to name a group. Matching takes time linear in the size
## of the text, whatever the pattern, so there are no backreferences and no
## lookaround. Positions are [Offset]s in the text searched.

# SPECS.md §5. Its primitives are provided by the host.

## A pattern that has been checked: only [parse] makes one. Two are equal when
## their patterns are written the same, and one shows as its pattern.
pub opaque type Regex = { pattern: String }

## Why a pattern couldn't be read.
pub type RegexError =
  ## The pattern isn't RE2's syntax, or is too large; reason says where.
  | BadPattern(pattern: String, reason: String)

impl Show for RegexError {
  fn show(self) -> String = match self {
    BadPattern(pattern, reason) => "bad regular expression \"{pattern}\": {reason}"
  }
}

impl Show for Regex {
  fn show(self) -> String = self.pattern
}

## Where a pattern matched, and what its groups took.
pub type Match = {
  ## The text matched.
  text: String,
  ## Where the match is in the text searched: `text.slice(m.range)` is `Some(m.text)`.
  range: Range(Offset),
  ## What each group took, in the order the groups open in the pattern: `None`
  ## for a group that took no part in the match.
  groups: List(Option(String)),
  ## What each named group took, by name. A named group that took no part
  ## isn't in it.
  named: Map(String, String),
}

impl Match {
  ## What group n took: 0 is the whole match, 1 the first group. `None` for a
  ## group that took no part, or that the pattern doesn't have.
  pub fn group(self, n: Int) -> Option(String) =
    if n == 0 then Some(self.text) else self.groups[n - 1] ?? None
}

## Reads a pattern: `regex.parse("\\d+")`, or `Err(BadPattern(…))` when it
## isn't one.
pub fn parse(pattern: String) -> Result(Regex, RegexError) = match problem(pattern) {
  Some(reason) => Err(BadPattern(pattern, reason))
  None => Ok({ pattern })
}

## text with every character that means something in a pattern escaped, so
## that the pattern matches the text as it is: `regex.escape("1.5")` is
## `"1\\.5"`.
@host("regex.escape")
pub extern fn escape(text: String) -> String

impl Regex {
  ## Whether the pattern matches the whole of text. To ask whether it matches
  ## anywhere in it, `re.find(text) != None`.
  pub fn matches(self, text: String) -> Bool = whole(self.pattern, text)

  ## The first match in text, leftmost, or `None`.
  pub fn find(self, text: String) -> Option(Match) = first(matchesIn(self.pattern, text, limit: 1))

  ## Every match in text, left to right, none overlapping. An empty match
  ## right after another match isn't counted.
  pub fn findAll(self, text: String) -> List(Match) = matchesIn(self.pattern, text, limit: -1)

  ## text with every match replaced by with, as a string's `replace` replaces
  ## every occurrence. In with, `$1` is what group 1 took, `$<year>` what the
  ## group named year took, and `$$` is a `$`; a group that took no part, or
  ## that the pattern doesn't have, gives "". `$1x` is group 1, then `x`, and
  ## `$<1>0` is group 1, then `0`.
  pub fn replace(self, text: String, with: String) -> String =
    replaced(self.pattern, text, with, limit: -1)

  ## text with its first match replaced by with, as [Regex.replace] does.
  pub fn replaceFirst(self, text: String, with: String) -> String =
    replaced(self.pattern, text, with, limit: 1)

  ## The parts of text between matches, as a string's `split` gives the parts
  ## between separators: `regex.parse(",\\s*")` splits `"a, b,c"` into
  ## `["a", "b", "c"]`.
  pub fn split(self, text: String) -> List(String) = splitAt(self.pattern, text)
}

# The host's primitives. Each compiles the pattern the first time it sees it
# and keeps it, so a Regex costs one compilation however often it is used.

@host("regex.problem")
extern fn problem(pattern: String) -> Option(String)

@host("regex.whole")
extern fn whole(pattern: String, text: String) -> Bool

@host("regex.matches")
extern fn matchesIn(pattern: String, text: String, limit: Int) -> List(Match)

@host("regex.replace")
extern fn replaced(pattern: String, text: String, with: String, limit: Int) -> String

@host("regex.split")
extern fn splitAt(pattern: String, text: String) -> List(String)
