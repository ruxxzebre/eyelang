# SPECS.md section 20, example 3.
@memory(128 MiB)
job thumbnails() {}

test "thumbnails don't fit on a small runner" {
  with memory(64 MiB)
  let run = thumbnails.start()
  expect(run.outcome() == Some(Failed(OutOfMemory)))
}
