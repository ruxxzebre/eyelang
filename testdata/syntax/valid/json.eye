pub type Json =
  | Null
  | Bool(Bool)
  | Number(Float)
  | Text(String)
  | Array(List(Json))
  | Object(Map(String, Json))

pub fn render(j: Json) -> String = match j {
  Null => "null"
  Bool(b) => if b { "true" } else { "false" }
  Number(n) => "{n}"
  Text(s) => "\"{escape(s)}\""
  Array(items) => "[{items |> map(render) |> join(",")}]"
  Object(fields) => "\{{fields |> entries |> map(fn (k, v): "\"{k}\":{render(v)}") |> join(",")}\}"
}
