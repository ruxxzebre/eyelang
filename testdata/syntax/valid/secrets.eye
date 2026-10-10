fn post(url: Url) uses {secret telegramBotToken, net Http.post} {
  let token: Secret(String) = secret("telegram-bot-token")
  http.post(url, auth: bearer(token))
}
