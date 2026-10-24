// Package execfile writes a file that this process executes afterwards.
//
// Linux, macOS and the BSDs refuse to execute a file that any process holds open for writing: execve fails with ETXTBSY ("text file busy"). The refusal follows the open file, not the path, so renaming the file into place does not clear it. A Go program forks while its other goroutines run, and a child holds a copy of every descriptor that existed when it was forked until it calls exec: close-on-exec closes the copy at exec, not at fork. A child forked while this process had the file open therefore keeps the file busy after this process closed it, and the exec that follows fails (golang.org/issue/22315). The window is the child's time from fork to exec. A loaded host stretches it to milliseconds, so it opens wherever one goroutine writes a binary while others start extensions, builds, or tools.
//
// Write closes the window: this process forks no child from before the file is opened until after it is closed. Go forks under the write side of syscall.ForkLock, so holding it for reading across the open, the copy, and the close leaves no child that holds the descriptor. Nothing retries.
package execfile

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"
)

// Write opens name for writing with the extra flags in flag (os.O_CREATE, os.O_EXCL, os.O_TRUNC), copies content into it, and closes it, while this process starts no child. The umask masks perm, as in os.OpenFile; a caller that needs other bits calls os.Chmod with the name afterwards.
//
// Every goroutine that starts a process waits until Write returns, so content must be memory or a file. A pipe, a socket, or a reader that waits for a goroutine that starts a process can deadlock.
func Write(name string, flag int, perm fs.FileMode, content io.Reader) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	file, err := os.OpenFile(name, flag|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, content)
	return errors.Join(err, file.Close())
}
