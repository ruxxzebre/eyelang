# The morning digest, which Notify's @allow lets call it.

import notify

pub fn whoIs(user: notify.UserId) -> notify.Identity = notify.identity(user)
