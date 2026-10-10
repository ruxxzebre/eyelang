# SPECS.md section 19, example 1.
# The example is a list of statements, wrapped in a function.
fn example() {
  let token: Secret(String) = secret("telegram-bot-token")
  http.post(url, auth: bearer(token))        # bearer() accepts a Secret
  log.info("token is {token}")               # compile error: Secret has no Show
}
