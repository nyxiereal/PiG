// Package execfile writes a file that this process executes afterwards.
//
// Linux, macOS and the BSDs refuse to execute a file that any process holds open for writing: execve fails with ETXTBSY ("text file busy"). The refusal follows the open file, not the path, so renaming the file into place does not clear it. A Go program forks while its other goroutines run, and a child holds a copy of every descriptor that existed when it was forked until it calls exec: close-on-exec closes the copy at exec, not at fork. A child forked while this process had the file open therefore keeps the file busy after this process closed it, and the exec that follows fails (golang.org/issue/22315). The window is the child's time from fork to exec. A loaded host stretches it to milliseconds, so it opens wherever one goroutine writes a binary while others start extensions, builds, or tools.
//
// Write closes the window: no write descriptor of the file exists while this process can fork. Go forks under the write side of syscall.ForkLock, so Write opens the file, writes one chunk, and closes it while holding the lock for reading, and releases the lock between chunks. A child is forked only while no descriptor is open, so none holds one. The lock is held for one chunk at a time, so a process start waits for at most one chunk of the copy, not the whole file. Nothing retries.
package execfile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// chunkSize is how many bytes Write copies per hold of the fork lock: the longest a process start waits for a write.
var chunkSize int64 = 256 << 10

// Write creates or opens name with the extra flags in flag (os.O_CREATE, os.O_EXCL, os.O_TRUNC) and copies content into it, so that this process starts no child while a descriptor of the file is open. The umask masks perm, as in os.OpenFile; a caller that needs other bits calls os.Chmod with the name afterwards.
//
// The file is closed and opened again between chunks (the later opens seek to the end of the written part), so nothing else may remove or replace name until Write returns, and the file is visible with a partial length meanwhile. Every goroutine that starts a process waits while a chunk is written, so content must be memory or a file. A pipe, a socket, or a reader that waits for a goroutine that starts a process can deadlock.
func Write(name string, flag int, perm fs.FileMode, content io.Reader) error {
	var written int64
	for first := true; ; first = false {
		n, done, err := writeChunk(name, flag, perm, first, written, content)
		written += n
		if err != nil || done {
			return err
		}
	}
}

// writeChunk copies up to chunkSize bytes under the fork lock, starting at offset, and reports how many it wrote and whether content ended. The later chunks are not opened with O_APPEND: the kernel refuses copy_file_range into an append-mode descriptor, and the copy would fall back to a buffer.
func writeChunk(name string, flag int, perm fs.FileMode, first bool, offset int64, content io.Reader) (n int64, done bool, err error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	if !first {
		flag = 0
	}
	file, err := os.OpenFile(name, flag|os.O_WRONLY, perm)
	if err != nil {
		return 0, false, err
	}
	if !first {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return 0, false, errors.Join(err, file.Close())
		}
	}
	n, err = io.CopyN(file, content, chunkSize)
	if errors.Is(err, io.EOF) {
		done, err = true, nil
	}
	return n, done, errors.Join(err, file.Close())
}
