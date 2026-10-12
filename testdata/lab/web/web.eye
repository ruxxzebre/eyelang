# Reading web pages that have no feed: directly, through the VPN's proxy, or
# through a browser behind it (FlareSolverr), for sites that block scripts.

import flaresolverr
import http

pub type Route = Direct | Vpn | VpnBrowser

pub type FetchError =
  | Status(code: Int)
  | NoAnswer(reason: String)
  ## The browser couldn't get past the page's checks.
  | Unsolved(message: String)

## A page's HTML. VPN_PROXY_URL is the address of the VPN's HTTP proxy.
pub fn page(url: String, via: Route = Direct) -> Result(String, FetchError) uses {wait, net, env} {
  let proxy = env.get!("VPN_PROXY_URL")
  if via == VpnBrowser {
    let solved = flaresolverr.request!(
      cmd: "request.get",
      url:,
      proxy: proxy |> map(fn p: { url: p }),
    )
    return match solved {
      Ok({ solution: Some(s), ..}) => Ok(s.response)
      Ok(s) => Err(Unsolved(message: s.message))
      Err(e) => Err(NoAnswer(reason: "{e}"))
    }
  }
  match http.get!(url, headers: { "Accept": "text/html" }, via: if via == Vpn then proxy else None) {
    Ok(r) if r.status == 200 => Ok(r.body)
    Ok(r) => Err(Status(code: r.status))
    Err(e) => Err(NoAnswer(reason: "{e}"))
  }
}
