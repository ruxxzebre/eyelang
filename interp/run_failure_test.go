package interp

import (
	"strings"
	"testing"
)

// A job's run that stops on a bug, such as a fake with no answer, is shown
// before the expect it broke, since it is the reason.
func TestAJobsBugComesBeforeTheExpectItBroke(t *testing.T) {
	prog := loadFiles(t, map[string]string{"main.eye": `entity Box(id: Int) {
  state n: Int = 0

  pub fn set(v: Int) { n = v }
}

job fill() {
  unreachable("the cause")
  Box(1).set!(1)
}

test "filled" {
  fill.start()
  expect(Box(1).n == 1)
}
`})
	results := RunTests(prog, "")
	if len(results) != 1 {
		t.Fatalf("%d results", len(results))
	}
	lines := strings.Split(results[0].Failure, "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "job fill: unreachable: the cause") || !strings.Contains(lines[1], "expected Box(1).n == 1") {
		t.Errorf("the test failed with:\n%s\nwant the job's fault, then the expect", results[0].Failure)
	}
}
