# SPECS.md section 19, example 2.
## Find open issues that mention a phrase.
fn searchIssues(phrase: String, limit: Int = 10) -> List(Issue) uses {net GitHub.search} {
  github.search(phrase, limit)
}

## Add a label to an issue.
@destructive
fn label(issue: IssueId, name: String) uses {net GitHub.label} {
  github.label(issue, name)
}

type Triage = { duplicateOf: Option(IssueId), labels: List(String) }

job triage(report: String) -> Result(Triage, AiError) {
  ai.generate!(model: "claude-opus-5-5", prompt: report, tools: [searchIssues, label])
}
