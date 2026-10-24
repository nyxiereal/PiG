package execfile_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/MichaelKinsy/PiG/internal/execfile"
	"github.com/MichaelKinsy/PiG/internal/testenv"
)

// readerFunc is a reader the test controls while the destination is open.
type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

// A child forked while the file is open for writing holds the descriptor until it calls exec, and the exec of the file fails with ETXTBSY. Go forks under the write side of syscall.ForkLock, so Write holds the read side while the file is open.
func TestWriteStartsNoChildWhileTheFileIsOpen(t *testing.T) {
	name := filepath.Join(t.TempDir(), "runner")
	var chunks int
	content := readerFunc(func(p []byte) (int, error) {
		chunks++
		if chunks == 3 {
			return 0, io.EOF
		}
		// Every read of the content happens with the file open for writing.
		testenv.RequireForkLocked(t)
		return copy(p, "chunk\n"), nil
	})
	if err := execfile.Write(name, os.O_CREATE|os.O_EXCL, 0o755, content); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(name); err != nil || string(got) != "chunk\nchunk\n" {
		t.Fatalf("file = %q, %v", got, err)
	}
	if !syscall.ForkLock.TryLock() {
		t.Fatal("Write returned with the fork lock held")
	}
	syscall.ForkLock.Unlock()
}

func TestWriteReleasesTheForkLockWhenItFails(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := errors.New("content failed")
	cases := map[string]func() error{
		"open fails": func() error {
			return execfile.Write(existing, os.O_CREATE|os.O_EXCL, 0o755, strings.NewReader("new"))
		},
		"content fails": func() error {
			return execfile.Write(filepath.Join(dir, "partial"), os.O_CREATE|os.O_EXCL, 0o755, io.MultiReader(strings.NewReader("half"), readerFunc(func([]byte) (int, error) { return 0, broken })))
		},
	}
	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			err := write()
			if err == nil {
				t.Fatal("Write succeeded")
			}
			if name == "content fails" && !errors.Is(err, broken) {
				t.Fatalf("error = %v, want the content's", err)
			}
			if !syscall.ForkLock.TryLock() {
				t.Fatal("Write returned with the fork lock held")
			}
			syscall.ForkLock.Unlock()
		})
	}
	if got, err := os.ReadFile(existing); err != nil || string(got) != "old" {
		t.Fatalf("an exclusive create replaced the file: %q, %v", got, err)
	}
}

func TestWriteTruncatesWhenAsked(t *testing.T) {
	name := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(name, bytes.Repeat([]byte("old"), 100), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := execfile.Write(name, os.O_CREATE|os.O_TRUNC, 0o755, strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(name); err != nil || string(got) != "new" {
		t.Fatalf("file = %q, %v", got, err)
	}
}
