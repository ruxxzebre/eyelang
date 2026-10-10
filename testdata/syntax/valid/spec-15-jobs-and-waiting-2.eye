# SPECS.md section 15, example 2.
fn written(day: Date) -> Result(Article, DigestError) =
  match write.start(day).join!() {
    Completed(result) => result
    Failed(fault) => Err(Faulted(fault))
  }
