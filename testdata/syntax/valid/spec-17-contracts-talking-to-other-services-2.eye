# SPECS.md section 17, example 2.
impl notify.Notify {
  fn send(to, text, key) = Message(key).queue({ text, to })
  fn receipt(key) = Message(key).delivery()
}
