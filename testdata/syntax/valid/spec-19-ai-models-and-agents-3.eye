# SPECS.md section 19, example 3.
import agents.{claude, codex}

type Review = { verdict: Verdict, findings: List(Finding) }
type Verdict = | Approve | RequestChanges(reason: String)
type Finding = { file: String, line: Int, message: String }

@on(event(github.pullRequestOpened))
@budget(usd: 5)
@deadline(2 h)
job review(pr: PullRequest) -> Result(Review, agents.AgentError) {
  let session = claude.start!(
    prompt: "Review {pr.url}. Report real bugs only.",
    workdir: checkout!(pr.head),
    tools: [readFiles, runTests],
  )
  let review: Review = session.result!()?
  if review.findings != [] {
    let fixed: String = codex.start!(prompt: "Fix these: {review.findings}",
      workdir: checkout!(pr.head), tools: [editFiles, runTests]).result!()?
    log.info("codex: {fixed}")
  }
  Ok(review)
}
