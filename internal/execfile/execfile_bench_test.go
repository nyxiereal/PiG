//go:build unix

package execfile_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/MichaelKinsy/PiG/internal/execfile"
)

// plainWrite is what the writers of a program did before Write: open, copy, close, with no hold on forks.
func plainWrite(name string, flag int, perm os.FileMode, content io.Reader) error {
	file, err := os.OpenFile(name, flag|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, content)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func benchmarkSource(b *testing.B, size int) string {
	b.Helper()
	source := filepath.Join(b.TempDir(), "compiled")
	if err := os.WriteFile(source, make([]byte, size), 0o755); err != nil {
		b.Fatal(err)
	}
	return source
}

// BenchmarkWrite measures one copy of a compiled binary into place, as copyArtifact and copyBuiltBinary do, with and without the hold on forks.
func BenchmarkWrite(b *testing.B) {
	for _, size := range []int{64 << 10, 1 << 20, 16 << 20} {
		for name, write := range map[string]func(string, int, os.FileMode, io.Reader) error{"plain": plainWrite, "execfile": execfile.Write} {
			b.Run(fmt.Sprintf("%s/%dKiB", name, size>>10), func(b *testing.B) {
				source := benchmarkSource(b, size)
				destination := filepath.Join(b.TempDir(), "runner")
				b.ReportAllocs()
				b.SetBytes(int64(size))
				for b.Loop() {
					input, err := os.Open(source)
					if err != nil {
						b.Fatal(err)
					}
					err = write(destination, os.O_CREATE|os.O_EXCL, 0o755, input)
					_ = input.Close()
					if err != nil {
						b.Fatal(err)
					}
					if err := os.Remove(destination); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkForkWhileWriting measures what the hold costs a goroutine that starts a process while another goroutine copies a 16 MiB binary. A copy every 50 ms is a host that builds extensions one after another; a copy with no pause is the worst case. The benchmark starts true(1) as fast as it can and reports the median and the slowest 1% of the starts.
func BenchmarkForkWhileWriting(b *testing.B) {
	trueBinary, err := exec.LookPath("true")
	if err != nil {
		b.Skipf("no true(1) to start: %v", err)
	}
	for _, pause := range []time.Duration{50 * time.Millisecond, 0} {
		for _, impl := range []struct {
			name  string
			write func(string, int, os.FileMode, io.Reader) error
		}{{"plain", plainWrite}, {"execfile", execfile.Write}} {
			b.Run(fmt.Sprintf("%s/pause=%v", impl.name, pause), func(b *testing.B) {
				source := benchmarkSource(b, 16<<20)
				destination := filepath.Join(b.TempDir(), "runner")
				stop := make(chan struct{})
				var writer sync.WaitGroup
				writer.Go(func() {
					for {
						select {
						case <-stop:
							return
						case <-time.After(pause):
						}
						input, err := os.Open(source)
						if err != nil {
							return
						}
						_ = os.Remove(destination)
						_ = impl.write(destination, os.O_CREATE|os.O_EXCL, 0o755, input)
						_ = input.Close()
					}
				})
				defer func() {
					close(stop)
					writer.Wait()
				}()
				starts := make([]time.Duration, 0, 4096)
				b.ReportAllocs()
				for b.Loop() {
					began := time.Now()
					if err := exec.Command(trueBinary).Run(); err != nil {
						b.Fatal(err)
					}
					starts = append(starts, time.Since(began))
				}
				slices.Sort(starts)
				b.ReportMetric(float64(starts[len(starts)/2]), "p50-ns/start")
				b.ReportMetric(float64(starts[len(starts)*99/100]), "p99-ns/start")
			})
		}
	}
}
