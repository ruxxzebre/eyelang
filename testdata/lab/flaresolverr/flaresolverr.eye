# FlareSolverr: a browser that fetches pages for sites that block scripts,
# here through the VPN.

import http

pub type Proxy = { url: String }

pub type Solution = { url: String, status: Int, response: String }

pub type Solved = { status: String, message: String, solution: Option(Solution) }

contract FlareSolverr {
  ## Fetches a page in the browser: cmd "request.get". A POST, but safe to
  ## repeat: it only reads the page.
  @http("POST /v1")
  query request(
    cmd: String,
    url: String,
    proxy: Option(Proxy) = None,
    maxTimeout: Int = 60_000,
  ) -> Result(Solved, http.HttpError)
}
