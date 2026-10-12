# SPECS.md section 17, example 4.
impl notify.Notify {
  fn send(to, text, priority, key) = Message(key).queue({ text, to, priority })
  fn receipt(key) = Message(key).delivery()
}
