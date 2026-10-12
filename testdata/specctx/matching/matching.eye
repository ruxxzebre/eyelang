# What SPECS.md §9's statements assume (context=matching): a grant and what
# to do with it, a user, bytes to lex, and the tokens they make.

type Revision = | Revision(Int)

type ReceiptId = | ReceiptId(String)

type Reason = | NoSeats | Banned

type Grant =
  | Admitted(revision: Revision)
  | Pending(receipt: ReceiptId)
  | Refused(reason: Reason)

type User = { name: String, age: Int }

type Token = | Arrow | Number(Bytes) | End | Illegal

let grant = Pending(receipt: ReceiptId("r-41"))

let user: User = { name: "Ann", age: 3 }

let items = bytes("->")

let entry = ("lang", "eyelang")

fn use(revision: Revision) -> Bool = true

fn waited(receipt: ReceiptId) -> Duration = 2 h

fn escalate(receipt: ReceiptId) -> Bool = true

fn wait(receipt: ReceiptId) -> Bool = false

fn report(grant: Grant) -> Bool = false

fn number(src: Bytes) -> Token = Number(src)
