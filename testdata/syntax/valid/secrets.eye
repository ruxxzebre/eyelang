fn post(url: Url) uses {secret "telegram-bot-token", net Http.post} {
  let token: Secret(String) = secret("telegram-bot-token")
  http.post(url, auth: bearer(token))
}
