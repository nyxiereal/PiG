//go:build !race

package ai

import (
	"context"
	"testing"
)

// upstream: packages/ai/test/streaming-tool-arguments.test.ts "allocates linearly in the argument text". The race detector inflates allocations, so the measurement runs without it.

func TestStreamingToolArgumentsAllocateLinearlyInTheArgumentText(t *testing.T) {
	for _, path := range streamingArgumentsPaths {
		t.Run(path.name, func(t *testing.T) {
			run := func(size int) uint64 {
				arguments, text := streamingWriteCall(t, size)
				deltas := streamingDeltas(text, 32)
				body := path.body(deltas, arguments, text)
				// The server and the body are built outside the measured run; the run streams the body once.
				server := sequentialToolCallServer(body)
				defer server.Close()
				return allocatedBytes(func() {
					stream := sequentialToolCallStream(t, path.provider(server.URL))
					for range stream.Events(context.Background()) {
					}
					_ = stream.Result()
				})
			}
			run(5_000) // warm the provider's connection and caches
			small := run(25_000)
			large := run(250_000)
			// Ten times the text: linear growth is about 10x, re-parsing the accumulated text about 100x.
			if ratio := float64(large) / float64(small); ratio >= 25 {
				t.Fatalf("allocation grew %.1fx for ten times the text (%d to %d bytes)", ratio, small, large)
			}
			_, text := streamingWriteCall(t, 250_000)
			if perByte := float64(large) / float64(len(text)); perByte >= 5_000 {
				t.Fatalf("%.0f bytes allocated per byte of arguments", perByte)
			}
		})
	}
}

func TestBedrockStreamingToolArgumentsAllocateLinearlyInTheArgumentText(t *testing.T) {
	run := func(size int) uint64 {
		_, text := streamingWriteCall(t, size)
		events := bedrockStreamStateEvents(text)
		return allocatedBytes(func() { runBedrockEvents(t, events...) })
	}
	run(5_000)
	small := run(25_000)
	large := run(250_000)
	if ratio := float64(large) / float64(small); ratio >= 25 {
		t.Fatalf("allocation grew %.1fx for ten times the text (%d to %d bytes)", ratio, small, large)
	}
	_, text := streamingWriteCall(t, 250_000)
	if perByte := float64(large) / float64(len(text)); perByte >= 5_000 {
		t.Fatalf("%.0f bytes allocated per byte of arguments", perByte)
	}
}
