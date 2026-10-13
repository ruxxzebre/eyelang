package interp

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestMemoryFS runs a program on a file system held in memory, as the
// online playground does: fs reads the files it was given, relative paths
// being below the program's directory, and what the program writes stays
// in memory, where it reads it back.
func TestMemoryFS(t *testing.T) {
	prog := loadFiles(t, map[string]string{"main.eye": `
import fs

pub fn main() -> Result((), fs.FsError) uses {fs, stdio} {
  print(fs.readText("notes/todo.txt")?)
  fs.makeDir("out")?
  fs.writeText("out/a.txt", "one\n")?
  fs.appendText("out/a.txt", "two\n")?
  print("{fs.lines("out/a.txt")?}")
  print("{fs.list(".")?} {fs.list("out")?}")
  print("{fs.exists("out/a.txt")} {fs.exists("nothing.txt")}")
  fs.remove("out/a.txt")?
  print("{fs.exists("out/a.txt")}")
  match fs.readText("nothing.txt") {
    Ok(_) => print("read nothing.txt")
    Err(e) => print("{e}")
  }
  match fs.remove("notes") {
    Ok(_) => print("removed notes")
    Err(e) => print("{e}")
  }
  Ok(())
}
`})
	dir := prog.Root.Dir
	var out, errs bytes.Buffer
	code := Run(context.Background(), prog, RunOptions{
		Stdout: &out, Stderr: &errs, MemoryLimit: 1 << 30,
		FS:      map[string][]byte{filepath.Join(dir, "notes", "todo.txt"): []byte("buy milk")},
		Virtual: &VirtualClock{Start: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)},
	})
	want := "buy milk\n" +
		"[\"one\", \"two\"]\n" +
		"[\"notes\", \"out\"] [\"a.txt\"]\n" +
		"true false\n" +
		"false\n" +
		"nothing.txt: not found\n" +
		"notes: directory not empty\n"
	if code != 0 || out.String() != want {
		t.Errorf("exit %d, printed:\n%s\nwant:\n%s\nstderr:\n%s", code, out.String(), want, errs.String())
	}
}
