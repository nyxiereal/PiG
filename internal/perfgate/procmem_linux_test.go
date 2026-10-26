// SPDX-License-Identifier: MIT

//go:build linux

package perfgate

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

var mappedFileSink byte

func TestProcessMemoryReadsThisProcess(t *testing.T) {
	memory := ProcessMemory(t, os.Getpid())
	if memory.Rss <= 0 || memory.Pss <= 0 || memory.Pss > memory.Rss || memory.Anon <= 0 || memory.Anon > memory.Rss {
		t.Fatalf("%+v", memory)
	}
}

// The gated figure is Anon: pages the process wrote must raise it, and file pages it only mapped and read must raise Rss but not Anon.
func TestProcessMemoryAnonCountsWrittenPagesAndNotMappedFiles(t *testing.T) {
	const size = 16 << 20
	page := os.Getpagesize()
	before := ProcessMemory(t, os.Getpid())

	path := filepath.Join(t.TempDir(), "mapped")
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	mapped, err := syscall.Mmap(int(file.Fd()), 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Munmap(mapped)
	for i := 0; i < size; i += page {
		mappedFileSink += mapped[i] // a package variable, so the compiler cannot drop the reads that fault the pages in
	}
	afterRead := ProcessMemory(t, os.Getpid())
	if grew := afterRead.Rss - before.Rss; grew < size*3/4 {
		t.Fatalf("Rss grew %.0f B after reading %d B of a mapped file, want at least three quarters of it", grew, size)
	}
	if grew := afterRead.Anon - before.Anon; grew > size/4 {
		t.Fatalf("Anon grew %.0f B after only reading a mapped file of %d B", grew, size)
	}

	heap, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Munmap(heap)
	for i := 0; i < size; i += page {
		heap[i] = 1
	}
	afterWrite := ProcessMemory(t, os.Getpid())
	if grew := afterWrite.Anon - afterRead.Anon; grew < size*3/4 {
		t.Fatalf("Anon grew %.0f B after writing %d B of anonymous pages, want at least three quarters of it", grew, size)
	}
}

func TestDescendantFindsAGrandchildByName(t *testing.T) {
	named := filepath.Join(t.TempDir(), "perfgate-probe")
	// The probe is a shell under another name; it prints only after it has exec'd under that name, then waits in a child sleep.
	shellPath, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shellPath, named); err != nil {
		t.Fatal(err)
	}
	shell := exec.CommandContext(t.Context(), "sh", "-c", named+" -c 'echo started; sleep 30' & wait")
	shell.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := shell.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := shell.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-shell.Process.Pid, syscall.SIGKILL) // The shell, the probe and its sleep share the group.
		_ = shell.Wait()
	})
	if _, err := bufio.NewReader(stdout).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	found := Descendant(shell.Process.Pid, "perfgate-probe")
	if found == 0 || found == shell.Process.Pid {
		t.Fatalf("Descendant(%d) = %d", shell.Process.Pid, found)
	}
	if Descendant(shell.Process.Pid, "no-such-process") != 0 {
		t.Fatal("found a process that does not exist")
	}
}
