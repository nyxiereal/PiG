//go:build linux

package parity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MichaelKinsy/PiG/internal/testenv"
)

// The fast run keeps going after a failed group, so one run reports every failing group: test/perfgate runs in the
// serial-exclusive group after the parallel one, and a failure in the parallel group must not hide its own. The serial group
// runs with its own arguments, not the fault group's -tags and -run that ran before it.
func TestGroupedFastRunReportsTheSerialGroupAfterAFailedParallelGroup(t *testing.T) {
	root := pigRepoRoot(t)
	fakeBin := t.TempDir()
	calls := filepath.Join(t.TempDir(), "go-test-calls")
	writeExecutable(t, filepath.Join(fakeBin, "go"), `#!/bin/sh
case "$1" in
  list)
    case "$*" in
      *"./..."*) echo example/fast; echo example/perfgate ;;
      *"./test/perfgate"*) echo example/perfgate ;;
      *) echo example/heavy ;;
    esac
    exit 0
    ;;
  build) exit 0 ;;
  test)
    echo "$*" >>"$GO_TEST_CALLS"
    case "$*" in *"-p 12 "*) exit 1 ;; esac
    exit 0
    ;;
esac
exit 24
`)
	writeExecutable(t, filepath.Join(fakeBin, "cargo"), "#!/bin/sh\nmkdir -p \"$CARGO_TARGET_DIR/release\" && : >\"$CARGO_TARGET_DIR/release/rust-sdk-fixture\"\n")
	scratch := t.TempDir()

	cmd := exec.Command(testenv.Bash(t), filepath.Join(root, "automation", "ci", "test-grouped.sh"), "fast-parallel")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"), "GO_TEST_CALLS="+calls, "TMPDIR="+scratch, "PIG_TEST_FIXTURE_DIR="+filepath.Join(scratch, "fixtures"))
	out, err := cmd.CombinedOutput()
	if exitCode(err) != 1 {
		t.Fatalf("fast-parallel with a failing parallel group exited %v, want 1\n%s", err, out)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatalf("no go test ran: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("go test ran %d times, want the parallel, fault and serial groups:\n%s", len(lines), data)
	}
	parallel, fault, serial := lines[0], lines[1], lines[2]
	if !strings.Contains(parallel, "-p 12 ") || !strings.HasSuffix(parallel, " example/fast") {
		t.Errorf("first group = %q, want the 12-way pool of example/fast alone", parallel)
	}
	if !strings.Contains(fault, "-tags pigfault") || !strings.HasSuffix(fault, " ./durable/core/driver") {
		t.Errorf("second group = %q, want the pigfault group of ./durable/core/driver", fault)
	}
	if !strings.Contains(serial, "-p 1 ") || !strings.HasSuffix(serial, " example/perfgate") || strings.Contains(serial, "-tags") || strings.Contains(serial, "-run") {
		t.Errorf("third group = %q, want example/perfgate alone at -p 1 without the fault group's -tags and -run", serial)
	}
}
