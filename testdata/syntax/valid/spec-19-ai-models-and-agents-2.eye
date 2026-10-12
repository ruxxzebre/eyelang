# SPECS.md section 19, example 2.
import ai

## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue. False if there is no such label, or if GitHub's
## reply was lost.
@destructive
fn label(issue: IssueId, name: String) -> Bool uses {net GitHub.label} {
  match github.label(issue, name, approval) {
    Added => true
    NoSuchLabel | Uncertain(_) => false
  }
}

type Triage = { duplicateOf: Option(IssueId), labels: List(String) }

job triage(report: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label],
    approve: fn call: approval!(call, from: maintainer, within: 1 day))
}
