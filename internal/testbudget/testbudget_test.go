package testbudget

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	for _, tc := range []struct {
		name          string
		override      string
		untilDeadline time.Duration
		hasDeadline   bool
		want          time.Duration
		wantErr       bool
	}{
		{name: "default without deadline", want: Default},
		{name: "override", override: "300", want: 300 * time.Second},
		{name: "test deadline caps the budget", untilDeadline: 40 * time.Second, hasDeadline: true, want: 30 * time.Second},
		{name: "distant deadline keeps default", untilDeadline: time.Hour, hasDeadline: true, want: Default},
		{name: "expired deadline keeps a positive budget", untilDeadline: time.Second, hasDeadline: true, want: time.Second},
		{name: "zero override", override: "0", wantErr: true},
		{name: "non-numeric override", override: "10s", wantErr: true},
		{name: "overflow override", override: "9223372037", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.override, tc.untilDeadline, tc.hasDeadline)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("budget = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestDeadlineBound(t *testing.T) {
	for _, tc := range []struct{ until, want time.Duration }{
		{time.Hour, time.Hour - grace},
		{40 * time.Second, 30 * time.Second},
		{5 * time.Second, time.Second},
		{-time.Second, time.Second},
	} {
		if got := deadlineBound(tc.until); got != tc.want {
			t.Errorf("deadlineBound(%s) = %s, want %s", tc.until, got, tc.want)
		}
	}
}

func TestExpiredNamesTheDeadlineOnlyWhenItEndedTheContext(t *testing.T) {
	if got := Expired(context.Background()); got != "" {
		t.Fatalf("Expired(live) = %q, want empty", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	if got := Expired(ctx); !strings.Contains(got, "go test -timeout deadline") {
		t.Fatalf("Expired(ended) = %q, want it to name the go test deadline", got)
	}
}

func TestOnceBuildsOnceAndKeepsTheResult(t *testing.T) {
	var once Once[string]
	calls := 0
	build := func(context.Context) (string, error) {
		calls++
		return "artifact", nil
	}
	for range 3 {
		if got, err := once.Do(t, build); err != nil || got != "artifact" {
			t.Fatalf("Do = %q, %v", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("build ran %d times, want 1", calls)
	}
	var failed Once[string]
	want := errors.New("link failed")
	for range 2 {
		if _, err := failed.Do(t, func(context.Context) (string, error) { return "", want }); !errors.Is(err, want) {
			t.Fatalf("Do error = %v, want %v", err, want)
		}
	}
}
