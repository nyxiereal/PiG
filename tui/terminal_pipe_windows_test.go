//go:build windows

package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procGetThreadTimes = kernel32.NewProc("GetThreadTimes")

// filetimeDuration reads a FILETIME that counts a duration in 100 ns units, as GetThreadTimes reports CPU time.
func filetimeDuration(ft windows.Filetime) time.Duration {
	return time.Duration(int64(ft.HighDateTime)<<32|int64(ft.LowDateTime)) * 100
}

// A Windows pipe handle is signalled from the moment the pipe exists, and again after every read, whether or not input is queued. Waiting on the handle therefore reported every pipe ready, and the reader went into a ReadFile that only input or the end of the pipe could return. The waiter polls a pipe for queued input instead.
func TestWindowsInputWaiterWaitsOnAPipeForInput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	waiter, err := newTerminalInputWaiter(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.close()

	if ready, err := waiter.wait(r, 20); err != nil || ready {
		t.Fatalf("wait on an empty pipe = %v, %v; want a timeout", ready, err)
	}
	if _, err := w.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if ready, err := waiter.wait(r, 10_000); err != nil || !ready {
		t.Fatalf("wait on a pipe with input = %v, %v; want ready", ready, err)
	}
	var one [1]byte
	if _, err := io.ReadFull(r, one[:]); err != nil {
		t.Fatal(err)
	}
	if ready, err := waiter.wait(r, 20); err != nil || ready {
		t.Fatalf("wait on a pipe its reader emptied = %v, %v; want a timeout", ready, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if ready, err := waiter.wait(r, 10_000); err != nil || !ready {
		t.Fatalf("wait on a pipe whose writer closed = %v, %v; want ready, so the read reports the end of input", ready, err)
	}
	if _, err := r.Read(one[:]); !errors.Is(err, io.EOF) {
		t.Fatalf("read after the writer closed = %v, want io.EOF", err)
	}
}

func TestWindowsInputWaiterCancelsAWaitOnAnEmptyPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	waiter, err := newTerminalInputWaiter(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.close()

	type result struct {
		ready bool
		err   error
	}
	started, done := make(chan struct{}), make(chan result, 1)
	go func() {
		close(started)
		ready, err := waiter.wait(r, -1)
		done <- result{ready, err}
	}()
	<-started
	cancel()
	select {
	case got := <-done:
		if got.ready || got.err != nil {
			t.Fatalf("cancelled wait = %v, %v; want neither", got.ready, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancellation did not end an unbounded wait on an empty pipe")
	}
}

// An idle wait on a pipe polls, so it must stay a light sleep: a poll loop without its pause would use a whole core. The wait runs on a locked thread so the thread's CPU time is the wait's alone.
func TestWindowsInputWaiterIdlePipeWaitStaysIdle(t *testing.T) {
	const idle = 500 * time.Millisecond
	const budget = 50 * time.Millisecond // a poll every 5 ms costs about 1 ms; a loop without the pause costs the whole 500 ms
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	waiter, err := newTerminalInputWaiter(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	defer waiter.close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cpu := func() time.Duration {
		var creation, exit, kernel, user windows.Filetime
		if r1, _, err := procGetThreadTimes.Call(uintptr(windows.CurrentThread()), uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r1 == 0 { //nolint:gosec // G103: GetThreadTimes writes four FILETIMEs, and x/sys has no wrapper for it.
			t.Fatal(err)
		}
		return filetimeDuration(kernel) + filetimeDuration(user)
	}
	before := cpu()
	if ready, err := waiter.wait(r, int(idle/time.Millisecond)); err != nil || ready {
		t.Fatalf("idle wait = %v, %v; want a timeout", ready, err)
	}
	if used := cpu() - before; used > budget {
		t.Fatalf("a %v idle wait on a pipe used %v of CPU, want at most %v", idle, used, budget)
	}
}

// Cancelling a reader that waits on an empty pipe ends it without taking input that arrives afterwards: the next reader of the pipe gets it. The framing deadline ends the reader's first wait, which proves the wait is not parked in a read.
func TestReadInputStreamLeavesAPipeForTheNextReader(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var expiredOnce sync.Once
	expired := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- ReadInputStreamUntil(ctx, r,
			func() time.Time { return time.Now().Add(time.Millisecond) },
			func(data []byte) { t.Errorf("the reader took input %q from an empty pipe", data) },
			func() { expiredOnce.Do(func() { close(expired) }) })
	}()
	select {
	case <-expired:
	case <-time.After(10 * time.Second):
		_ = w.Close() // frees the reader, which sits in ReadFile
		t.Fatal("the reader's wait on an empty pipe did not time out: it reported the pipe ready and sits in ReadFile")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("reader ended with %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		_ = w.Close()
		t.Fatal("cancellation did not end the reader")
	}
	if _, err := w.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := io.ReadFull(r, one[:]); err != nil || one[0] != 'x' {
		t.Fatalf("next reader got %q, %v; want the byte written after the first reader ended", one[:], err)
	}
}

// A pipe that ends delivers what it queued, then the end of input.
func TestReadInputStreamDeliversAPipeToItsEnd(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if _, err := w.WriteString("ab"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var got []byte
	err = ReadInputStream(t.Context(), r, func(data []byte) { got = append(got, data...) })
	if !errors.Is(err, io.EOF) || string(got) != "ab" {
		t.Fatalf("ReadInputStream = %q, %v; want \"ab\" then io.EOF", got, err)
	}
}

// A byte that arrives while the reader waits is delivered at once, and the wait costs a few microseconds when the byte is already queued.
func BenchmarkWindowsInputWaiterPipeQueuedInput(b *testing.B) {
	r, w, err := os.Pipe()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	waiter, err := newTerminalInputWaiter(b.Context(), r)
	if err != nil {
		b.Fatal(err)
	}
	defer waiter.close()
	var one [1]byte
	b.ReportAllocs()
	for b.Loop() {
		if _, err := w.Write(one[:]); err != nil {
			b.Fatal(err)
		}
		if ready, err := waiter.wait(r, -1); err != nil || !ready {
			b.Fatalf("wait = %v, %v", ready, err)
		}
		if _, err := io.ReadFull(r, one[:]); err != nil {
			b.Fatal(err)
		}
	}
}
