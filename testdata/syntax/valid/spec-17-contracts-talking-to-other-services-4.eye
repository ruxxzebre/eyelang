# SPECS.md section 17, example 4.
@http(names: snake)                       # dedupeKey is dedupe_key, Normal is "normal"
@auth(bearer: "notify-token")             # Authorization: Bearer <secret("notify-token")>
contract Notify {
  @http("POST /v1/send", status: { 200: Accepted, 202: Pending, 400: Refused, 403: Refused })
  @key(required, header: "X-Actions-Idempotency-Key", field: "dedupe_key")
  @reconcile(send)
  command send(event: String, title: String, priority: Priority = Normal,
    to: Option(List(UserId)) = None) -> Delivery
}
