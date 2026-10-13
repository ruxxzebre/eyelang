package codegen

// tsHelpers ends every generated TypeScript module: the types and functions
// that the generated code shares. They follow the same rules as the
// interpreter (interp/json.go, interp/services.go, interp/listen.go).
const tsHelpers = `// ---------------------------------------------------------------- the wire

/**
 * The version of something a service keeps, as the service gave it. A @fresh
 * command takes it back as its basedOn argument.
 */
export type Version = number | string;

/** A person's yes to a @destructive command: what they approved, and who they are. */
export interface Approval {
  what: string;
  by: string;
}

/** A @fresh command's outcome when the state it acts on changed after the version it was based on: it did nothing. */
export interface Stale {
  kind: "Stale";
}

/** A @fresh command's outcome when what it was based on is gone: it can never apply. */
export interface Expired {
  kind: "Expired";
}

/**
 * A command's outcome when its reply was lost: the service may or may not
 * have acted. Sending the command again with the same key finds out without
 * acting twice, when the service keeps what each key did.
 */
export interface Uncertain {
  kind: "Uncertain";
  key: string;
}

/**
 * A reply whose status the operation doesn't take. For a query that returns
 * Result(T, http.HttpError) it is the error the caller handles; for any other
 * operation, a request the service refused. A service's method throws one to
 * answer with its status and body.
 */
export class HttpError extends Error {
  readonly op: string;
  readonly status: number;
  readonly body: string;

  constructor(op: string, status: number, body: string) {
    super(op + " was refused with status " + status + ": " + (body.length > 300 ? body.slice(0, 300) + "…" : body));
    this.op = op;
    this.status = status;
    this.body = body;
  }
}

/** Where a client finds its service, and how it reaches it. */
export interface ClientOptions {
  /** Where the service answers, such as http://notify:3001. */
  baseUrl: string;
  /** The secret the contract's @auth names, for a contract with one. */
  secret?: string;
  /** Makes the requests; the global fetch when left out. */
  fetch?: (url: string, init: RequestInit) => Promise<Response>;
}

// ---------------------------------------------------------------- clients

interface Call {
  op: string;
  method: string;
  path: string[];
  query: URLSearchParams;
  body: Record<string, unknown>;
  /** The body, in place of body's fields: an event's value. */
  whole?: unknown;
  headers: Record<string, string>;
}

interface Reply {
  status: number;
  text: string;
}

interface Auth {
  header: string;
  bearer: boolean;
}

function newCall(op: string, method: string, path: string): Call {
  return { op, method, path: path.split("/"), query: new URLSearchParams(), body: {}, headers: {} };
}

/** A value as a path segment or a query parameter takes it: a string as it is, anything else as JSON. */
function text(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value);
}

/** Adds an argument in the path, the query or the body. In the query, a list is the parameter repeated. */
function put(call: Call, where: "path" | "query" | "body", name: string, value: unknown): void {
  if (value === undefined) {
    return;
  }
  switch (where) {
    case "path":
      call.path = call.path.map((seg) => (seg === ":" + name ? encodeURIComponent(text(value)) : seg));
      return;
    case "query":
      if (value === null) {
        return;
      }
      if (Array.isArray(value)) {
        for (const e of value) {
          call.query.append(name, text(e));
        }
      } else {
        call.query.set(name, text(value));
      }
      return;
    default:
      call.body[name] = value;
  }
}

async function send(options: ClientOptions, auth: Auth | null, call: Call): Promise<Reply> {
  let url = options.baseUrl.replace(/\/$/, "") + call.path.join("/");
  const query = call.query.toString();
  if (query !== "") {
    url += "?" + query;
  }
  const headers: Record<string, string> = { ...call.headers, Accept: "application/json" };
  let body: string | undefined;
  if (call.method !== "GET" && call.method !== "DELETE") {
    body = JSON.stringify(call.whole !== undefined ? call.whole : call.body);
    headers["Content-Type"] = "application/json";
  }
  if (auth !== null) {
    headers[auth.header] = (auth.bearer ? "Bearer " : "") + (options.secret ?? "");
  }
  const response = await (options.fetch ?? fetch)(url, { method: call.method, headers, body });
  return { status: response.status, text: await response.text() };
}

/** Whether a request failed before it reached the service, so that sending it again can't do anything twice. */
function notSent(err: unknown): boolean {
  const cause = err instanceof Error ? (err as { cause?: unknown }).cause : undefined;
  const code = typeof cause === "object" && cause !== null ? (cause as { code?: unknown }).code : undefined;
  return code === "ECONNREFUSED" || code === "ENOTFOUND" || code === "EAI_AGAIN";
}

function parse(call: Call, reply: Reply): unknown {
  if (reply.text.trim() === "") {
    return null;
  }
  try {
    return JSON.parse(reply.text);
  } catch (err) {
    throw new Error(call.op + " answered " + reply.status + " with a body that isn't JSON: " + String(err));
  }
}

/** A variant from a reply whose status names it: the body holds its fields. */
function fieldsOf(call: Call, reply: Reply): Record<string, unknown> {
  const x = parse(call, reply);
  return typeof x === "object" && x !== null && !Array.isArray(x) ? (x as Record<string, unknown>) : {};
}

/** Stale or Expired when a @fresh command's reply says one of them: { "kind": "Stale" }, or the name alone. */
function freshOutcome(reply: Reply, stale: string, expired: string): Stale | Expired | null {
  let x: unknown;
  try {
    x = JSON.parse(reply.text);
  } catch {
    return null;
  }
  const name = typeof x === "string" ? x : typeof x === "object" && x !== null ? (x as { kind?: unknown }).kind : undefined;
  if (name === "Stale" || name === stale) {
    return { kind: "Stale" };
  }
  if (name === "Expired" || name === expired) {
    return { kind: "Expired" };
  }
  return null;
}

// ---------------------------------------------------------------- servers

type TextKind = "string" | "number" | "boolean" | "json";

interface Route {
  op: string;
  method: string;
  path: string;
  httpErr: boolean;
  /** The statuses of results whose variant ` + "`status:`" + ` names, by kind, or by value for a type without fields. */
  statuses: Record<string, number>;
  serve: (r: ServerRequest) => Promise<unknown>;
}

interface ServerAuth {
  header: string;
  bearer: boolean;
  secret: string;
}

/** A request the operation can't take: it answers 400. */
class BadRequest extends Error {}

/** What a request gives an operation. */
class ServerRequest {
  readonly op: string;
  readonly path: Record<string, string>;
  readonly query: URLSearchParams;
  readonly body: Record<string, unknown>;
  readonly headers: Headers;

  constructor(op: string, path: Record<string, string>, query: URLSearchParams, body: Record<string, unknown>, headers: Headers) {
    this.op = op;
    this.path = path;
    this.query = query;
    this.body = body;
    this.headers = headers;
  }

  /** An argument, from the path, a query parameter or the body; undefined when the request leaves it out. */
  arg(where: "path" | "query" | "body", name: string, kind: TextKind, list: boolean, option: boolean): unknown {
    switch (where) {
      case "path":
        return name in this.path ? this.fromText(name, this.path[name], kind) : undefined;
      case "query": {
        const values = this.query.getAll(name);
        if (values.length === 0) {
          return undefined;
        }
        return list ? values.map((v) => this.fromText(name, v, kind)) : this.fromText(name, values[0], kind);
      }
    }
    if (!(name in this.body)) {
      return undefined;
    }
    const v = this.body[name];
    if (v === null && !option) {
      throw new BadRequest(this.op + "'s argument " + name + ": expected a value, found null");
    }
    return v === null ? undefined : v;
  }

  required(name: string, v: unknown): unknown {
    if (v === undefined) {
      throw new BadRequest(this.op + "'s argument " + name + ": it is missing");
    }
    return v;
  }

  /** An argument written as text: a string as it is, anything else as JSON. */
  fromText(name: string, s: string, kind: TextKind): unknown {
    switch (kind) {
      case "string":
        return s;
      case "number": {
        const n = Number(s);
        if (s.trim() === "" || Number.isNaN(n)) {
          throw new BadRequest(this.op + "'s argument " + name + ": expected a number, found " + JSON.stringify(s));
        }
        return n;
      }
      case "boolean":
        if (s !== "true" && s !== "false") {
          throw new BadRequest(this.op + "'s argument " + name + ": expected true or false, found " + JSON.stringify(s));
        }
        return s === "true";
    }
    try {
      return JSON.parse(s);
    } catch {
      throw new BadRequest(this.op + "'s argument " + name + ": expected JSON, found " + JSON.stringify(s));
    }
  }

  /** A command's key, from its header or its body field. */
  key(header: string, field: string): string {
    const k = this.headers.get(header);
    if (k !== null && k !== "") {
      return k;
    }
    const f = field !== "" ? this.body[field] : undefined;
    if (typeof f === "string" && f !== "") {
      return f;
    }
    throw new BadRequest(this.op + " takes a key, in the " + header + " header" + (field !== "" ? " or the body's field " + field : ""));
  }
}

/** What a JSON value is, for messages: a string or a number as it is, a list or an object by its kind. */
function describe(x: unknown): string {
  if (Array.isArray(x)) {
    return "a list";
  }
  if (typeof x === "object" && x !== null) {
    return "an object";
  }
  return JSON.stringify(x);
}

function errorResponse(status: number, message: string): Response {
  return new Response(JSON.stringify({ error: message }) + "\n", { status, headers: { "Content-Type": "application/json" } });
}

function jsonResponse(status: number, value: unknown): Response {
  const body = status === 204 || status === 205 || status === 304 ? null : JSON.stringify(value === undefined ? null : value) + "\n";
  return new Response(body, { status, headers: { "Content-Type": "application/json" } });
}

function match(routes: Route[], method: string, pathname: string): { route?: Route; path?: Record<string, string>; allowed: string[] } {
  const segments = pathname.split("/");
  const allowed: string[] = [];
  for (const route of routes) {
    const want = route.path.split("/");
    if (want.length !== segments.length) {
      continue;
    }
    const path: Record<string, string> = {};
    let ok = true;
    for (let i = 0; i < want.length && ok; i++) {
      if (want[i].startsWith(":")) {
        try {
          path[want[i].slice(1)] = decodeURIComponent(segments[i]);
        } catch {
          ok = false;
        }
        ok = ok && path[want[i].slice(1)] !== "";
      } else {
        ok = want[i] === segments[i];
      }
    }
    if (!ok) {
      continue;
    }
    if (route.method === method) {
      return { route, path, allowed };
    }
    allowed.push(route.method);
  }
  return { allowed };
}

function serve(name: string, auth: ServerAuth | null, routes: Route[]): (request: Request) => Promise<Response> {
  return async (request: Request): Promise<Response> => {
    // The credential is checked first, so a caller without it learns
    // nothing, not even which paths exist.
    if (auth !== null) {
      const want = (auth.bearer ? "Bearer " : "") + auth.secret;
      if (auth.secret === "" || request.headers.get(auth.header) !== want) {
        const how = auth.bearer ? "Bearer and the secret" : "the secret";
        return errorResponse(401, name + " takes only requests with " + how + " in the " + auth.header + " header");
      }
    }
    const url = new URL(request.url);
    const found = match(routes, request.method, url.pathname);
    const route = found.route;
    if (route === undefined || found.path === undefined) {
      if (found.allowed.length > 0) {
        const response = errorResponse(405, decodeURIComponent(url.pathname) + " takes " + found.allowed.join(" or ") + ", not " + request.method);
        response.headers.set("Allow", found.allowed.join(", "));
        return response;
      }
      return errorResponse(404, name + " has no operation at " + request.method + " " + decodeURIComponent(url.pathname));
    }
    let body: Record<string, unknown> = {};
    if (route.method !== "GET" && route.method !== "DELETE") {
      const raw = await request.text();
      if (raw.trim() !== "") {
        let x: unknown;
        try {
          x = JSON.parse(raw);
        } catch (err) {
          return errorResponse(400, "the body isn't JSON: " + String(err));
        }
        if (typeof x !== "object" || x === null || Array.isArray(x)) {
          return errorResponse(400, "the body must be a JSON object of " + route.op + "'s arguments, found " + describe(x));
        }
        body = x as Record<string, unknown>;
      }
    }
    const r = new ServerRequest(route.op, found.path, url.searchParams, body, request.headers);
    try {
      const result = await route.serve(r);
      const kind = typeof result === "string" ? result : typeof result === "object" && result !== null ? (result as { kind?: unknown }).kind : undefined;
      const status = typeof kind === "string" && kind in route.statuses ? route.statuses[kind] : 200;
      return jsonResponse(status, result);
    } catch (err) {
      if (err instanceof BadRequest) {
        return errorResponse(400, err.message);
      }
      if (route.httpErr && err instanceof HttpError) {
        return new Response(err.body, { status: err.status, headers: { "Content-Type": "text/plain; charset=utf-8" } });
      }
      if (route.httpErr) {
        return errorResponse(502, route.op + ": " + String(err));
      }
      return errorResponse(500, route.op + " faulted: " + (err instanceof Error ? err.message : String(err)));
    }
  };
}
`
