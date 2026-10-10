@version(3)
entity Player(id: PlayerId) {
  state name: String
  state devices: Set(DeviceId) = {}
  state rank: Int = 0

  pub fn addDevice(d: DeviceId) -> Result((), DeviceError) {
    if devices |> size >= 5 { return Err(TooManyDevices) }
    devices = devices |> add(d)
    Ok(())
  }

  pub fn retire(ok: Approval) { end(ok) }

  @audit
  pub fn rename(to: String) { name = to }

  migrate from 1 (old) {
    { name: old.nick, devices: {}, rank: 0 }
  }

  migrate from 2 (old) {
    { old with rank: 0 }
  }
}
