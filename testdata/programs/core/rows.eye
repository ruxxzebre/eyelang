# A record literal in a lambda takes its type from where the call's result
# goes (SPECS.md §5).

type Fan = { name: String, age: Int }

type Town = { city: String }

type Line = { name: String, age: Int, home: Town }

type Admission = Guest | Member | Refuse

type Button = { id: String, label: String }

type Sheet = { lines: List(Line) }

let members: List(Fan) = [{ name: "Ann", age: 70 }, { name: "Bo", age: 12 }]

fn lines(xs: List(Fan)) -> List(Line) =
  xs |> map(fn m: { name: m.name, age: m.age, home: { city: "Cork" } })

fn adults(xs: List(Fan)) -> List(Line) =
  xs |> map(fn m: { name: m.name, age: m.age, home: { city: "Cork" } }) |> filter(fn l: l.age >= 18)

fn lineCount(ls: List(Line)) -> Int = ls |> size

fn oldest(xs: List(Fan)) -> Town =
  xs |> reduce({ city: "" }, fn (acc, m): if m.age > 60 then { city: m.name } else acc)

fn towns(xs: List(Fan)) -> List(Town) = xs |> map(fn m: match m.age {
  0 => { city: "new" }
  _ => if m.age > 60 then { city: "old" } else { city: m.name }
})

test "a lambda's record literal takes the type its call's result is used as" {
  let rows: List(Line) = members |> map(fn m: { name: m.name, age: m.age, home: { city: "Cork" } })
  expect(rows == lines(members))
  expect(adults(members) |> map(fn l: l.name) == ["Ann"])
  expect(lineCount(members |> map(fn m: { name: m.name, age: 0, home: { city: m.name } })) == 2)
  let sheet: Sheet = { lines: members |> map(fn m: { name: m.name, age: m.age, home: { city: "x" } }) }
  expect(sheet.lines |> size == 2)
  expect(oldest(members) == { city: "Ann" })
  expect(towns(members) == [{ city: "old" }, { city: "Bo" }])
  let buttons: List(Button) = Admission.variants() |> map(fn a: { id: "{a}", label: "{a}" })
  expect(buttons |> map(fn b: b.id) == ["Guest", "Member", "Refuse"])
}
