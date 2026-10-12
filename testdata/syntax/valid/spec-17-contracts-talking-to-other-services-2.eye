# SPECS.md section 17, example 2.
# notify/notify.eye
@port(3001)
@allow(digest, approving)
contract Notify {
  query identity(user: UserId) -> Identity
}

# trains/trains.eye
import notify.{Identity, UserId}

fn whoIs(user: UserId) -> Identity = notify.identity(user)    # error: trains isn't in @allow
