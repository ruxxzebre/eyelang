# GitHub's issues, as the triage example uses them.

pub type IssueId = | IssueId(Int)

pub type Issue = { id: IssueId, title: String, labels: List(String) }

pub type Labelled = | Added | NoSuchLabel

contract GitHub {
  query search(phrase: String, limit: Int) -> List(Issue)

  @destructive
  command label(issue: IssueId, name: String) -> Labelled
}
