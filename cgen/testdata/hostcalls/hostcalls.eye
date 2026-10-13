# Host functions the Go runner answers for compiled code (runtime/host.c):
# http.get. cgen's tests answer a test binary's calls with the interpreter's
# host functions, and serve the pages below on a local server whose address
# is in EYE_CGEN_HTTP; without it, there is nothing to get.

import http

fn describe(r: Result(http.Response, http.HttpError)) -> String = match r {
  Ok(page) => "{page.status} {page.body} x-answer={page.headers["x-answer"] ?? "none"}"
  Err(NoAnswer(reason)) => "no answer: {reason}"
  Err(e) => "error: {e}"
}

test "a page" {
  match env.get("EYE_CGEN_HTTP") {
    Some(base) => {
      print(describe(http.get("{base}/hello")))
      print(describe(http.get("{base}/hello", headers: { "Accept": "text/plain" })))
      print(describe(http.get("{base}/missing")))
    }
    None => print("no server")
  }
}

test "a URL nothing answers" {
  print(describe(http.get("ftp://example.invalid/")))
}
