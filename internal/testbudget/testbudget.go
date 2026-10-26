// Package testbudget sizes the deadlines tests use to bound a hang.
//
// A test that waits for a subprocess, an extension, or a shell command should
// return as soon as the awaited event happens; its deadline only decides how
// long a genuine hang takes to fail. Fixed 5-10 s deadlines turned into
// failures when many test binaries shared a machine (load average 80 on 14
// cores), so waits use [Wait] instead. Only test code imports this package.
//
// The go test -timeout deadline caps every bound, so a test that starts when
// the package run is about to expire reports a timeout of its own, naming a
// wait, a few seconds before the test binary's timeout panic. That test is not
// at fault: the run is out of time. A package whose tests need longer than
// -timeout in one process (coding/cli needs about 25 minutes on 4 CPUs) runs
// as several go test processes in CI (automation/ci/test-grouped.sh).
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

// clipNoted holds the names of the tests whose cut bound Wait already logged.
var clipNoted sync.Map

// Wait returns the hang bound for t: Default or the Env override, capped to
// end before t's deadline. When the deadline cuts the bound, Wait logs that
// once for t, so a failure at that bound names its cause: the package run is
// out of time, and the awaited event is not late.
func Wait(t *testing.T) time.Duration {
	t.Helper()
	deadline, hasDeadline := t.Deadline()
	untilDeadline := time.Until(deadline)
	budget, err := Resolve(os.Getenv(Env), untilDeadline, hasDeadline)
	if err != nil {
		t.Fatal(err)
	}
	if hasDeadline {
		full, _ := Resolve(os.Getenv(Env), 0, false)
		if note := clipNote(full, budget, untilDeadline); note != "" {
			if _, noted := clipNoted.LoadOrStore(t.Name(), struct{}{}); !noted {
				t.Log(note)
			}
		}
	}
	return budget
}

// clipNote describes a bound the go test deadline cut from full to budget, or
// returns "" when the deadline left it whole.
func clipNote(full, budget, untilDeadline time.Duration) string {
	if budget >= full {
		return ""
	}
	return fmt.Sprintf("testbudget: the wait bound is %s, cut from %s because the go test -timeout deadline is %s away; a wait that ends at this bound reports a package run that is out of time, not an awaited event that is late",
		budget.Round(time.Millisecond), full, untilDeadline.Round(time.Millisecond))
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
