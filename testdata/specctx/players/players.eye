# What SPECS.md §14's entity assumes (context=players): the identities of
# players and their devices, and why a device can't be added.

type PlayerId = | PlayerId(String)

type DeviceId = | DeviceId(String)

type DeviceError = | TooManyDevices
