# Reading web pages that have no feed: directly, through the VPN, or through
# a browser (FlareSolverr) behind the VPN, for sites that block scripts.

pub type Route = Direct | Vpn | VpnBrowser

pub type FetchError =
  | Status(code: Int)
  | Blocked
  | TimedOut

contract Web {
  ## A page's text, with its links written as [text](url).
  query page(url: String, via: Route = Direct) -> Result(String, FetchError)
}
