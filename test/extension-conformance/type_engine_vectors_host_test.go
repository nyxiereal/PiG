package extensionconformance

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/MichaelKinsy/PiG/coding/extension/host/subprocess"
	"github.com/MichaelKinsy/PiG/internal/typebox/engine"
)

// TestTypeEngineVectorsThroughTheHost runs every vector case the host's type engine answers (about 360 in test/parity/sdk-vectors/pi-ai/Type.json: Pick, Omit, Partial, KeyOf,
// Mapped, Conditional, Extends, Instantiate, Cyclic, Module, Script, ...) through the Python and the Rust SDK's own `Type` members, one case at a time, against Pi's answer: the
// schema's JSON text, its hidden members, the message TypeBox throws, or the JSON.stringify failure of a schema that holds a bigint. Each SDK dials this test's socket (PIG_EXT_SOCKET)
// for every type.engine call, as an extension dials the host's, and the socket is answered by the host's handler for the call (engine.Serve, what the subprocess host runs for
// CallTypeEngine). The Go SDK has the same cases in TestTypeEngineMembersMatchPi (fused path) and TestConformance_TypeEngineMembersAgreeAcrossSDKs covers all three SDKs through the real
// host with one tool; this is the case-by-case proof for Rust and Python through a socket.
func TestTypeEngineVectorsThroughTheHost(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the Rust SDK's tests")
	}
	root := findModuleRoot(t)
	python := testPythonExecutable()
	cargo := requireConformanceTool(t, "cargo")
	cases := []struct {
		name string
		cmd  func(ctx context.Context) *exec.Cmd
	}{
		{"python", func(ctx context.Context) *exec.Cmd {
			cmd := exec.CommandContext(ctx, python, filepath.Join("tests", "run_engine_vectors.py"))
			cmd.Dir = filepath.Join(root, "extensions", "sdk-py")
			cmd.Env = append(os.Environ(), "PYTHONPATH="+cmd.Dir)
			return cmd
		}},
		{"rust", func(ctx context.Context) *exec.Cmd {
			cmd := exec.CommandContext(ctx, cargo, "test", "--quiet", "--lib", "engine_cases_through_the_host", "--", "--nocapture", "--test-threads=1")
			cmd.Dir = filepath.Join(root, "extensions", "sdk-rs")
			return cmd
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "python" {
				if _, err := exec.LookPath(python); err != nil {
					t.Fatalf("%s is required: %v", python, err)
				}
			}
			socket := serveTypeEngineSocket(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			cmd := tc.cmd(ctx)
			cmd.Env = append(append([]string(nil), os.Environ()...), "PIG_EXT_SOCKET="+socket)
			if tc.name == "python" {
				cmd.Env = append(cmd.Env, "PYTHONPATH="+cmd.Dir)
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			match := regexp.MustCompile(`ENGINE_CASES_RUN=(\d+)`).FindSubmatch(out)
			if match == nil {
				t.Fatalf("the run reported no case count:\n%s", out)
			}
			if ran, _ := strconv.Atoi(string(match[1])); ran < 350 {
				t.Fatalf("only %d engine cases ran:\n%s", ran, out)
			}
			t.Logf("%s", match[0])
		})
	}
}

// serveTypeEngineSocket listens on a unix socket that answers every type.engine call of the extension protocol with the host's engine and nothing else (a ping is answered, as the
// SDKs expect while a call waits); it stops with the test.
func serveTypeEngineSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pig-te-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "h.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				for {
					var header [4]byte
					if _, err := io.ReadFull(conn, header[:]); err != nil {
						return
					}
					body := make([]byte, binary.BigEndian.Uint32(header[:]))
					if _, err := io.ReadFull(conn, body); err != nil {
						return
					}
					var env subprocess.Envelope
					if json.Unmarshal(body, &env) != nil || env.Type != subprocess.MsgCall || env.Call == nil || env.Call.Method != subprocess.CallTypeEngine {
						continue
					}
					reply, _ := json.Marshal(subprocess.Envelope{Type: subprocess.MsgCallResult, ID: env.ID, CallResult: &subprocess.CallResultPayload{Result: engine.Serve(env.Call.Args)}})
					var out [4]byte
					binary.BigEndian.PutUint32(out[:], uint32(len(reply)))
					if _, err := conn.Write(append(out[:], reply...)); err != nil {
						return
					}
				}
			}()
		}
	}()
	return path
}
