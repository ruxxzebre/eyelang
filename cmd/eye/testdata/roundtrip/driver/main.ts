// The TypeScript side of the round trip test of eye gen ts
// (cmd/eye/gen_roundtrip_test.go). The test compiles it with tsc --strict
// beside the modules eye gen ts wrote, and runs it with node against eye run
// serving the lab's Notify, a Miniflux written in Eyelang and the Echo
// contract. It sends every type of the lab's contracts through the generated
// clients and checks that each comes back as it went. It runs the Echo
// checks a second time through the generated handler, called in the same
// process, which passes each call on to eye run with the client.

import { EchoClient, echoHandler, HttpError } from "./echo.js";
import type * as echo from "./echo.js";
import { MinifluxClient } from "./miniflux.js";
import { NotifyClient } from "./notify.js";

declare const process: { env: Record<string, string | undefined>; exit(code: number): never };

let checks = 0;
let failures = 0;

function fail(message: string): void {
  failures++;
  console.log("FAIL: " + message);
}

/** JSON with its objects' keys sorted, to compare values. */
function canon(x: unknown): string {
  return JSON.stringify(x, (_key, v: unknown) =>
    typeof v === "object" && v !== null && !Array.isArray(v)
      ? Object.fromEntries(Object.entries(v as Record<string, unknown>).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)))
      : v,
  );
}

async function same<T>(what: string, got: Promise<T>, want: T): Promise<void> {
  checks++;
  try {
    const g = await got;
    if (canon(g) !== canon(want)) {
      fail(what + ": got " + canon(g) + ", want " + canon(want));
    }
  } catch (err) {
    fail(what + ": " + String(err));
  }
}

async function throwsHttp(what: string, got: Promise<unknown>, status: number, body: string): Promise<void> {
  checks++;
  try {
    const g = await got;
    fail(what + ": got " + canon(g) + ", want HttpError " + status);
  } catch (err) {
    if (!(err instanceof HttpError) || err.status !== status || err.body !== body) {
      fail(what + ": got " + String(err) + ", want HttpError " + status + " " + body);
    }
  }
}

const env = process.env;
const at = "2026-10-12T07:40:02Z";

async function echoChecks(where: string, c: EchoClient): Promise<void> {
  const w = (what: string) => where + ": " + what;
  await same(w("Priority"), c.priority({ value: "High" }), "High");
  await same(w("Format"), c.format({ value: "Markdown" }), "Markdown");
  await same(w("MessageId"), c.messageId({ value: "m-1" }), "m-1");
  const deliveries: echo.Delivery[] = [
    { kind: "Accepted", id: "m-1", deliveries: 2, deduped: true },
    { kind: "Pending", actionId: "a-1" },
    { kind: "Refused", error: "nobody receives digest.ireland" },
  ];
  for (const d of deliveries) {
    await same(w("Delivery " + d.kind), c.delivery({ value: d }), d);
  }
  await same(w("Refusal"), c.refusal({ value: "BadChoices" }), "BadChoices");
  await same(w("Choice"), c.choice({ value: { id: "yes", label: "Yes, take it" } }), { id: "yes", label: "Yes, take it" });
  await same(w("QuestionId"), c.questionId({ value: "q-1" }), "q-1");
  const asked: echo.Asked[] = [
    { kind: "Asking", value: "q-1" },
    { kind: "NotAsked", value: "NotAllowed" },
  ];
  for (const a of asked) {
    await same(w("Asked " + a.kind), c.asked({ value: a }), a);
  }
  const answered: echo.Answered = { question: "q-1", choice: "yes", by: "operator", at };
  await same(w("Answered"), c.answered({ value: answered }), answered);
  await same(w("UserId"), c.userId({ value: "operator" }), "operator");
  await same(w("CategoryId"), c.categoryId({ value: 7 }), 7);
  const feed: echo.Feed = { title: "News" };
  await same(w("Feed"), c.feed({ value: feed }), feed);
  const entry: echo.Entry = { title: "Ireland", url: "https://news.example/1", publishedAt: at, content: '<p>Hello & "bye"</p>', feed };
  await same(w("Entry"), c.entry({ value: entry }), entry);
  await same(w("Entries"), c.entries({ value: { total: 1, entries: [entry] } }), { total: 1, entries: [entry] });
  await same(w("User"), c.user({ value: { id: 1, username: "lab" } }), { id: 1, username: "lab" });
  await same(w("Refreshed"), c.refreshed({ value: "Refreshed" }), "Refreshed");
  const category: echo.Category = { id: 7, title: "News" };
  await same(w("Category"), c.category({ value: category }), category);
  await same(w("Created"), c.created({ value: category }), category);
  const subscription: echo.Subscription = { feedUrl: "https://news.example/feed", title: "News", category, parsingErrorCount: 2, parsingErrorMessage: "timeout" };
  await same(w("Subscription"), c.subscription({ value: subscription }), subscription);
  const subscribed: echo.Subscribed[] = [
    { kind: "Added", feedId: 42 },
    { kind: "NotAdded", errorMessage: "This feed already exists." },
  ];
  for (const s of subscribed) {
    await same(w("Subscribed " + s.kind), c.subscribed({ value: s }), s);
  }
  await same(w("ChatId"), c.chatId({ value: -100123 }), -100123);
  await same(w("UpdateId"), c.updateId({ value: 6 }), 6);
  await same(w("MessageRef"), c.messageRef({ value: 7 }), 7);
  await same(w("Button"), c.button({ value: { id: "no", label: "Not taking it" } }), { id: "no", label: "Not taking it" });
  const bodies: echo.UpdateBody[] = [
    { kind: "Command", name: "start", args: "now" },
    { kind: "Pressed", message: 7, button: "yes" },
    { kind: "Text", value: "hello" },
  ];
  for (const b of bodies) {
    await same(w("UpdateBody " + b.kind), c.updateBody({ value: b }), b);
  }
  const updates: echo.Update[] = [
    { id: 1, chat: 5, private: true, from: 5, sentAt: at, body: bodies[1] },
    { id: 2, chat: -100123, private: false, from: null, sentAt: at, body: bodies[2] },
  ];
  for (const u of updates) {
    await same(w("Update"), c.update({ value: u }), u);
  }
  const sendings: echo.Sending[] = [
    { kind: "Posted", value: 7 },
    { kind: "TooFast", retryAfter: 1500 },
    { kind: "Blocked" },
  ];
  for (const s of sendings) {
    await same(w("Sending " + s.kind), c.sending({ value: s }), s);
  }
  const person: echo.Person = { id: "operator", email: "op@lab.example", active: true, groups: ["admins", "lab"] };
  await same(w("Person"), c.person({ value: person }), person);
  await same(w("Proxy"), c.proxy({ value: { url: "socks5://vpn:1080" } }), { url: "socks5://vpn:1080" });
  const solution: echo.Solution = { url: "https://news.example", status: 200, response: "<html></html>" };
  await same(w("Solution"), c.solution({ value: solution }), solution);
  const solved: echo.Solved[] = [
    { status: "ok", message: "", solution },
    { status: "error", message: "timed out", solution: null },
  ];
  for (const s of solved) {
    await same(w("Solved"), c.solved({ value: s }), s);
  }
  await same(w("TrainCode"), c.trainCode({ value: "E101" }), "E101");
  await same(w("StopId"), c.stopId({ value: "PERSE" }), "PERSE");
  const movement: echo.Movement = {
    train: "E101",
    date: "2026-10-12",
    departures: { PERSE: at, TARA: "2026-10-12T07:45:02Z" },
    arrivals: {},
    delay: 120000,
    cancelled: true,
  };
  await same(w("Movement"), c.movement({ value: movement }), movement);
  await same(w("PeerUserId"), c.peerUserId({ value: "u-1" }), "u-1");
  const peer: echo.PeerUser = { id: "u-1", email: "guest@lab.example", groups: ["guests"], service: false, blocked: true };
  await same(w("PeerUser"), c.peerUser({ value: peer }), peer);
  const pending: echo.NetbirdPending = { id: "u-2", email: "new@lab.example", joinedAt: at };
  await same(w("Pending"), c.pending({ value: pending }), pending);
  await same(w("Change"), c.change({ value: "NoSuchUser" }), "NoSuchUser");
  await same(w("Version 42"), c.version({ value: 42 }), 42);
  await same(w("Version v7"), c.version({ value: "v7" }), "v7");
  await same(w("Approval"), c.approval({ value: { what: "burn shelf 3", by: "operator" } }), { what: "burn shelf 3", by: "operator" });

  // Arguments in the path and the query.
  const ids: echo.IdsArgs = { chat: -100123, user: "operator", priority: "Low", tags: ["a b", "c/d", "e&f"], limit: 10, at };
  await same(w("Ids"), c.ids(ids), { ...ids });
  await same(w("Ids without a limit"), c.ids({ ...ids, limit: null, tags: ["only"] }), { ...ids, limit: null, tags: ["only"] });
  // Arguments left out take their defaults; the client fills them in.
  await same(w("Defaults left out"), c.defaults({ given: "x" }), { given: "x", priority: "High", tags: ["a", "b"], note: null, count: 3 });
  await same(w("Defaults given"), c.defaults({ given: "y", priority: "Low", tags: [], note: "a note", count: 0 }), {
    given: "y",
    priority: "Low",
    tags: [],
    note: "a note",
    count: 0,
  });
  // Result(T, http.HttpError): an error the caller handles.
  await same(w("Page"), c.page({ status: 200 }), "the page");
  await throwsHttp(w("Page that is paused"), c.page({ status: 503 }), 503, "paused");
  // Statuses that `status:` maps to variants.
  await same(w("Grade 80"), c.grade({ score: 80 }), { kind: "Good", score: 80 });
  await same(w("Grade 20"), c.grade({ score: 20 }), { kind: "Poor", score: 20, reason: "below 50" });
  // A fresh and destructive command, and its outcomes.
  const approval: echo.Approval = { what: "clear shelf s3", by: "operator" };
  await same(w("Clear"), c.clear({ id: "s3", basedOn: 5, approval }, "k-1"), { kind: "Cleared", id: "s3", by: "operator", what: "clear shelf s3", key: "k-1" });
  await same(w("Clear that is stale"), c.clear({ id: "s3", basedOn: 1, approval }, "k-2"), { kind: "Stale" });
  await same(w("Clear that expired"), c.clear({ id: "s3", basedOn: 2, approval }, "k-3"), { kind: "Expired" });
  // A command whose impl faults: its reply is lost, so it is Uncertain.
  await same(w("Fail"), c.fail({ why: "on purpose" }, "k-9"), { kind: "Uncertain", key: "k-9" });
}

/** Requests written by hand, which eye run and the handler must answer alike. */
const rawRequests: { method: string; path: string; body?: string; key?: string }[] = [
  { method: "POST", path: "/defaults", body: '{"given": "x"}' },
  { method: "POST", path: "/defaults", body: '{"given": "x", "note": null, "count": 9}' },
  { method: "POST", path: "/defaults", body: "{}" },
  { method: "POST", path: "/defaults", body: "[1]" },
  { method: "GET", path: "/defaults" },
  { method: "GET", path: "/nowhere" },
  { method: "POST", path: "/grade", body: '{"score": 80}' },
  { method: "POST", path: "/grade", body: '{"score": 20}' },
  { method: "POST", path: "/shelves/s1/clear", body: '{"basedOn": 1, "approval": {"what": "clear", "by": "operator"}}' },
  { method: "POST", path: "/shelves/s1/clear", body: '{"basedOn": 1, "approval": {"what": "clear", "by": "operator"}}', key: "k-raw" },
  { method: "GET", path: "/ids/5?user=operator&priority=Low&tags=a&tags=b&at=2026-10-12T07:40:02Z" },
  { method: "GET", path: "/ids/5?user=operator&priority=Low&tags=a&limit=3&at=2026-10-12T07:40:02Z" },
  { method: "GET", path: "/page?status=503" },
  { method: "POST", path: "/update-body", body: '{"value": {"kind": "Text", "value": "hi"}}' },
];

async function answer(handle: (request: Request) => Promise<Response>, base: string, secret: string, r: (typeof rawRequests)[number]): Promise<[number, string]> {
  const headers: Record<string, string> = { Authorization: "Bearer " + secret };
  if (r.key !== undefined) {
    headers["Idempotency-Key"] = r.key;
  }
  const response = await handle(new Request(base + r.path, { method: r.method, headers, body: r.body }));
  return [response.status, await response.text()];
}

function alike(status: number, a: string, b: string): boolean {
  let x: unknown;
  let y: unknown;
  try {
    x = JSON.parse(a);
    y = JSON.parse(b);
  } catch {
    return a === b;
  }
  if (status === 400) {
    return typeof (x as { error?: unknown }).error === "string" && typeof (y as { error?: unknown }).error === "string";
  }
  return canon(x) === canon(y);
}

async function main(): Promise<void> {
  const direct = new EchoClient({ baseUrl: env.ECHO_URL ?? "", secret: env.ECHO_TOKEN });
  await echoChecks("eye run", direct);

  // The handler, with a service that passes each call on to eye run. A lost
  // reply becomes a throw, which the handler answers with 500.
  const forward = new Proxy(
    {},
    {
      get: (_target, name) => async (...args: unknown[]) => {
        const method = (direct as unknown as Record<string, (...a: unknown[]) => Promise<unknown>>)[String(name)];
        const result = await method.apply(direct, args);
        if (typeof result === "object" && result !== null && (result as { kind?: unknown }).kind === "Uncertain") {
          throw new Error("the reply was lost");
        }
        return result;
      },
    },
  ) as echo.EchoService;
  const handler = echoHandler(forward, "stub-secret");
  const viaStub = new EchoClient({ baseUrl: "http://stub.example", secret: "stub-secret", fetch: (url, init) => handler(new Request(url, init)) });
  await echoChecks("the TypeScript handler", viaStub);
  await throwsHttp(
    "the handler with a wrong secret",
    new EchoClient({ baseUrl: "http://stub.example", secret: "guess", fetch: (url, init) => handler(new Request(url, init)) }).priority({ value: "High" }),
    401,
    JSON.stringify({ error: "echo.Echo takes only requests with Bearer and the secret in the Authorization header" }) + "\n",
  );
  for (const r of rawRequests) {
    const [s1, b1] = await answer(fetch, env.ECHO_URL ?? "", env.ECHO_TOKEN ?? "", r);
    const [s2, b2] = await answer(handler, "http://stub.example", "stub-secret", r);
    checks++;
    if (s1 !== s2 || !alike(s1, b1, b2)) {
      fail(r.method + " " + r.path + " answered " + s1 + " " + b1.trim() + " from eye run, but " + s2 + " " + b2.trim() + " from the handler");
    }
  }

  // Miniflux in Eyelang: its answers show what each request carried.
  const miniflux = new MinifluxClient({ baseUrl: env.MINIFLUX_URL ?? "", secret: env.MINIFLUX_KEY });
  await same("miniflux.me", miniflux.me(), { id: 1, username: "lab" });
  const entries = (title: string) => ({
    total: 5,
    entries: [{ title, url: "https://news.example/5", published_at: "2026-10-12T07:40:00Z", content: "<p>Hello</p>", feed: { title: "News" } }],
  });
  await same("miniflux.entries with defaults", miniflux.entries({ category: 3, published_after: 1760000000, limit: 5 }), entries("3 1760000000 unread,read published_at desc"));
  await same(
    "miniflux.entries",
    miniflux.entries({ category: 3, published_after: 1760000000, limit: 5, status: ["unread"], direction: "asc" }),
    entries("3 1760000000 unread published_at asc"),
  );
  await same("miniflux.categories", miniflux.categories(), [{ id: 1, title: "News" }]);
  await same("miniflux.createCategory", miniflux.createCategory({ title: "Lab" }), { id: 7, title: "Lab" });
  await same("miniflux.subscribe, 400", miniflux.subscribe({ feed_url: "https://news.example/feed", category_id: 1 }), {
    kind: "NotAdded",
    error_message: "This feed already exists.",
  });
  await same("miniflux.subscribe, 201", miniflux.subscribe({ feed_url: "https://other.example/feed", category_id: 1 }), { kind: "Added", feed_id: 42 });
  await same("miniflux.refresh, 204", miniflux.refresh(), "refreshed");

  // The lab's notifier.
  const notify = new NotifyClient({ baseUrl: env.NOTIFY_URL ?? "", secret: env.NOTIFY_TOKEN });
  const args = { event: "digest.ireland", title: "Hello", body: "From TypeScript", to: ["operator"] };
  await same("notify.send", notify.send(args, "roundtrip-ts"), { kind: "Accepted", id: "roundtrip-ts", deliveries: 1, deduped: false });
  await same("notify.send again", notify.send(args, "roundtrip-ts"), { kind: "Accepted", id: "roundtrip-ts", deliveries: 1, deduped: true });
  await same("notify.send to nobody, 400", notify.send({ ...args, to: [] }, "roundtrip-ts-nobody"), { kind: "Refused", error: "nobody receives digest.ireland" });

  if (failures > 0) {
    process.exit(1);
  }
  console.log("ok: " + checks + " checks");
}

main().catch((err: unknown) => {
  console.log("FAIL: " + String(err));
  process.exit(1);
});
