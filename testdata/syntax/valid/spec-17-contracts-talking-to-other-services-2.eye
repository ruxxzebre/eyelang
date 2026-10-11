# SPECS.md section 17, example 2.
# notify/notify.eye
@port(3001)
@allow(digest, approving)
contract Notify {
  query identity(user: UserId) -> Identity
}

# trains/trains.eye: an error, since trains isn't in Notify's @allow
fn whoIs(user: UserId) -> Identity = notify.identity(user)
