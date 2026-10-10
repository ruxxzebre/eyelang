# SPECS.md section 14, example 1.
@version(3)
entity Player(id: PlayerId) {
  state name: String
  state devices: Set(DeviceId) = {}          # v3: added; the default fills old rows

  pub fn addDevice(d: DeviceId) -> Result((), DeviceError) {
    if devices |> size >= 5 { return Err(TooManyDevices) }
    devices = devices |> add(d)
    Ok(())
  }

  pub fn retire(ok: Approval) { end(ok) }     # ending an entity needs an Approval

  migrate from 2 (old) {                      # pure function from the old stored shape
    { name: old.nick, devices: {} }
  }
}
