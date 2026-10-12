# Maps: count n words drawn from a vocabulary of 5,000, then find the most
# common count and how many words reached it.

pub fn main(args: List(String)) uses {stdio} {
  let n = match parseInt(args[1] ?? "") {
    Ok(n) => n
    Err(_) => 5_000_000
  }
  var words: List(String) = []
  for i in 0..5000 {
    words += ["w{i * 7919 % 100_003}"]
  }
  var counts: Map(String, Int) = {}
  var seed = 7
  for _ in 0..n {
    seed = (seed * 1103515245 + 12345) % 2147483648
    let word = words[seed / 16 % 5000] ?? ""
    counts = counts.insert(word, (counts[word] ?? 0) + 1)
  }
  var top = 0
  var atTop = 0
  for count in counts |> values {
    if count > top {
      top = count
      atTop = 1
    } else if count == top {
      atTop += 1
    }
  }
  print("{counts.size()} words; the most common seen {top} times, {atTop} of them")
}
