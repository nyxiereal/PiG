// Package testbudget sizes the deadlines tests use to bound a hang.
//
// A test that waits for a subprocess, an extension, or a shell command should
// return as soon as the awaited event happens; its deadline only decides how
// long a genuine hang takes to fail. Fixed 5-10 s deadlines turned into
// failures when many test binaries shared a machine (load average 80 on 14
// cores), so waits use [Wait] instead. Only test code imports this package.
package testbudget

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Env overrides [Default] with a positive whole number of seconds.
const Env = "PIG_TEST_WAIT_TIMEOUT_SEC"

// Default matches the extension-conformance connect budget, chosen for -p 12
// runs on shared CI runners where builds and Node startup stall for tens of
// seconds.
const Default = 120 * time.Second

// grace is left before the go test -timeout deadline so a hang fails with the
// test's own diagnostics instead of the test binary's timeout panic.
const grace = 10 * time.Second

// Wait returns the hang bound for t: Default or the Env override, capped to
// end before t's deadline.
func Wait(t *testing.T) time.Duration {
	t.Helper()
	deadline, hasDeadline := t.Deadline()
	budget, err := Resolve(os.Getenv(Env), time.Until(deadline), hasDeadline)
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

// Context returns t.Context() bounded by Wait(t).
func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), Wait(t))
	t.Cleanup(cancel)
	return ctx
}

// DeadlineContext returns t.Context() bounded to end [grace] before t's go test
// deadline; without a deadline it is t.Context(). It bounds work whose length no
// event of the test decides, such as the go build of a shared test binary: the
// time that takes is what the host needs, so no fixed bound fits, and only the
// package timeout says it hangs. Work that fails at this bound should say so
// (see [Expired]).
func DeadlineContext(t *testing.T) context.Context {
	t.Helper()
	deadline, hasDeadline := t.Deadline()
	if !hasDeadline {
		return t.Context()
	}
	ctx, cancel := context.WithTimeout(t.Context(), deadlineBound(time.Until(deadline)))
	t.Cleanup(cancel)
	return ctx
}

// Expired describes ctx from [DeadlineContext] when its deadline ended it, or
// returns "".
func Expired(ctx context.Context) string {
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ""
	}
	return "the go test -timeout deadline arrived (minus " + grace.String() + " left to report): the package run is out of time, and this step is not hung"
}

// Once runs one build per test process and keeps its result for every later
// test: the shape of a shared fixture binary. The build runs under
// [DeadlineContext] of the first test that asks, so it ends at the go test
// deadline and not at a fixed bound, and a build that the deadline cut reports
// that ([Expired]). A failed build is kept, not retried.
type Once[T any] struct {
	once  sync.Once
	value T
	err   error
}

// Do returns the build's result, running build on the first call.
func (o *Once[T]) Do(t *testing.T, build func(ctx context.Context) (T, error)) (T, error) {
	t.Helper()
	o.once.Do(func() {
		ctx := DeadlineContext(t)
		o.value, o.err = build(ctx)
		if o.err != nil {
			if expired := Expired(ctx); expired != "" {
				o.err = fmt.Errorf("%s: %w", expired, o.err)
			}
		}
	})
	return o.value, o.err
}

// deadlineBound is the time left before a deadline, less grace, with a floor of
// one second.
func deadlineBound(untilDeadline time.Duration) time.Duration {
	return max(untilDeadline-grace, time.Second)
}

// Resolve computes a budget from an Env value and the time left before the
// test deadline.
func Resolve(override string, untilDeadline time.Duration, hasDeadline bool) (time.Duration, error) {
	budget := Default
	if override != "" {
		seconds, err := strconv.ParseInt(override, 10, 64)
		if err != nil || seconds <= 0 || seconds > int64((1<<63-1)/time.Second) {
			return 0, fmt.Errorf("%s=%q: want a positive whole number of seconds", Env, override)
		}
		budget = time.Duration(seconds) * time.Second
	}
	if hasDeadline {
		budget = min(budget, deadlineBound(untilDeadline))
	}
	return budget, nil
}
