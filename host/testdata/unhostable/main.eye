# Host functions whose types no Go function can take or give (host_test.go).

@host("lab.count")
extern fn count(xs: List(Int)) -> Int

@host("lab.first")
extern fn first(xs: String) -> Option(String)

@host("lab.ask")
extern fn ask(q: String) -> Result(String, Int)
