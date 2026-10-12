# A program with a main: what it prints goes to stdout, and an Err from
# main is written to stderr with exit status 1.

type Mood = | Fine | Tired(hours: Int)

impl Show for Mood {
  fn show(self) -> String = match self {
    Fine => "fine"
    Tired(hours) => "tired after {hours} h"
  }
}

pub fn main(args: List(String)) -> Result((), Mood) uses {stdio} {
  print("hello from {args.size()} argument(s)")
  for (i, a) in args |> drop(1) |> enumerate {
    print("{i}: {a}")
  }
  if args.size() > 2 then return Err(Tired(hours: args.size()))
  Ok(())
}
