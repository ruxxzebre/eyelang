# GitHub's pull requests, as the review example uses them.

pub type PullRequest = { url: String, head: String, title: String }

contract GitHub {
  event pullRequestOpened: PullRequest on "github.pull_request.opened"
}
