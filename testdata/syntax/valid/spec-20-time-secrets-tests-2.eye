# SPECS.md section 20, example 2.
# The example is a list of statements, wrapped in a function.
fn example() {
  let token: Secret(String) = secret("telegram-bot-token")  # TELEGRAM_BOT_TOKEN
  log.info("token is {token}")               # compile error: Secret has no Show
}
