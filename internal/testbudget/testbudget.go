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
// end before t's deadline. When the deadline cuts the bound short, Wait logs
// [Capped] on t, once: a wait the package's deadline ended fails the test, or
// kills the child process whose context carries the bound ("signal: killed"),
// just as a hang does, and that line is what tells the two apart.
func Wait(t *testing.T) time.Duration {
	t.Helper()
	deadline, hasDeadline := t.Deadline()
	budget, note, err := resolve(os.Getenv(Env), time.Until(deadline), hasDeadline)
	if err != nil {
		t.Fatal(err)
	}
	if note != "" {
		if _, seen := noted.LoadOrStore(t.Name(), true); !seen {
			t.Log(note)
		}
	}
	return budget
}

// noted holds the names of the tests that were told their waits are capped, so
// a test that waits in a loop hears it once.
var noted sync.Map

// resolve is [Resolve] plus the [Capped] note when the deadline lowered the
// budget.
func resolve(override string, untilDeadline time.Duration, hasDeadline bool) (budget time.Duration, note string, err error) {
	budget, err = Resolve(override, untilDeadline, hasDeadline)
	if err != nil {
		return 0, "", err
	}
	if uncapped, _ := Resolve(override, 0, false); budget < uncapped {
		note = Capped(budget, untilDeadline)
	}
	return budget, note, nil
}

// Capped is the note Wait logs when the go test -timeout deadline, until away,
// cuts a wait to budget. In that window nothing is known about the code under
// test: a test that fails after the note may be a hang or may be healthy work
// that the package's own slowness left no time for, and only the package's
// total time tells them apart.
func Capped(budget, until time.Duration) string {
	return fmt.Sprintf("testbudget: the go test -timeout deadline is %s away, so this wait is capped to %s. "+
		"A failure of this test after this line may be the package's own slowness, not a hang: check how long the package had run. "+
		"A package that uses its whole -timeout starves every test that starts in its last %s, healthy or not, "+
		"and needs more shards (automation/ci/test-grouped.sh, make test-cli), not a different test",
		until.Round(time.Millisecond), budget, grace+time.Second)
}

// Context returns t.Context() bounded by Wait(t).
func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), Wait(t))
	t.Cleanup(cancel)
	return ctx
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
		budget = min(budget, max(untilDeadline-grace, time.Second))
	}
	return budget, nil
}
