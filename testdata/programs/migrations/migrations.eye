# Entity versions (SPECS.md §14). A player was saved as { nickname } at
# version 1, as { nick } at version 2, and is { name, devices, joined } now.
# Rows saved at older versions are migrated when they are loaded; the Go test
# in interp/migrate_test.go stores such rows and loads them.

type PlayerId = | PlayerId(String)

type DeviceId = | DeviceId(String)

@version(3)
entity Player(id: PlayerId) {
  state name: String = ""
  state devices: Set(DeviceId) = {}
  ## Added after version 3 with a default: needs no new version.
  state joined: Int = 0

  pub fn rename(n: String) { name = n }

  pub fn join(d: DeviceId) {
    devices = devices |> add(d)
    joined += 1
  }

  ## Version 1 called the name nickname; version 2 calls it nick.
  migrate from 1 (old) {
    { nick: old.nickname }
  }

  ## Version 3 calls it name, and adds devices.
  migrate from 2 (old) {
    { name: old.nick |> trim, devices: {} }
  }
}

test "a new player starts at the current version" {
  let p = Player(PlayerId("ann"))
  p.rename("Ann")
  p.join(DeviceId("phone"))
  expect(p.name == "Ann" && p.joined == 1)
}
