package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A program saves players at version 2; the next release of it is at
// version 3, with a migration; eye run on the same journal reads the old
// rows as the new version declares them, and saves them at it.
func TestRunMigratesStoredEntities(t *testing.T) {
	v2 := `type PlayerId = | PlayerId(String)

@version(2)
entity Player(id: PlayerId) {
  state nick: String = ""

  pub fn rename(n: String) { nick = n }

  migrate from 1 (old) {
    { nick: old.nickname }
  }
}

pub fn main(args: List(String)) uses {entity Player, stdio} {
  let ann = Player(PlayerId("ann"))
  ann.rename(" Ann ")
  print("saved {ann.nick}")
}
`
	v3 := `type PlayerId = | PlayerId(String)

type DeviceId = | DeviceId(String)

@version(3)
entity Player(id: PlayerId) {
  state name: String = ""
  state devices: Set(DeviceId) = {}

  pub fn join(d: DeviceId) { devices = devices |> add(d) }

  migrate from 1 (old) {
    { nick: old.nickname }
  }

  migrate from 2 (old) {
    { name: old.nick |> trim, devices: {} }
  }
}

pub fn main(args: List(String)) uses {entity Player, stdio} {
  let ann = Player(PlayerId("ann"))
  ann.join(DeviceId("phone"))
  print("read {ann.name}, {ann.devices |> size} device")
}
`
	dir := program(t, map[string]string{"main.eye": v2})
	eyeRun := func() string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run([]string{"run", dir}, &stdout, &stderr); code != 0 {
			t.Fatalf("eye run: exit %d\n%s%s", code, stdout.String(), stderr.String())
		}
		return stdout.String()
	}
	if out := eyeRun(); out != "saved  Ann \n" {
		t.Fatalf("version 2 printed %q", out)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(v3), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := eyeRun(); out != "read Ann, 1 device\n" {
		t.Fatalf("version 3 printed %q", out)
	}
	journal, err := os.ReadFile(filepath.Join(dir, ".eye", "journal"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(journal), `"Schema":3`) {
		t.Errorf("the journal never saved the player at version 3:\n%s", journal)
	}

	// A release whose oldest migration is from version 4 can't read the
	// rows stored at version 3, and eye run says so before it starts.
	v5 := `type PlayerId = | PlayerId(String)

@version(5)
entity Player(id: PlayerId) {
  state name: String = ""

  migrate from 4 (old) {
    { name: old.title }
  }
}

pub fn main(args: List(String)) uses {entity Player, stdio} {
  print("read {Player(PlayerId("ann")).name}")
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.eye"), []byte(v5), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", dir}, &stdout, &stderr); code == 0 {
		t.Fatalf("eye run started on a program that can't read its rows:\n%s", stdout.String())
	}
	if want := "1 Player saved at version 3, and Player has no migrate from 3"; !strings.Contains(stderr.String(), want) {
		t.Errorf("eye run said %q, want it to say %q", stderr.String(), want)
	}
}
