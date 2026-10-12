# What SPECS.md §8's dyn example assumes (context=dyn): two traits, and
# types that implement them.

type Config = { telegram: Bool, token: String }

trait Notifier {
  fn notify(self, text: String) -> Bool
}

type Telegram = { token: String }

type Console = { prefix: String }

impl Telegram {
  fn new(token: String) -> Telegram = { token }
}

impl Console {
  fn new() -> Console = { prefix: "> " }
}

impl Notifier for Telegram {
  fn notify(self, text) = text != ""
}

impl Notifier for Console {
  fn notify(self, text) = true
}

trait Sink {
  fn take(self, line: String) -> Int
}

type Logger = { lines: Int }

type Metrics = { port: Int }

impl Logger {
  fn new() -> Logger = { lines: 0 }
}

impl Metrics {
  fn new(port: Int) -> Metrics = { port }
}

impl Sink for Logger {
  fn take(self, line) = self.lines + 1
}

impl Sink for Metrics {
  fn take(self, line) = size(line)
}
