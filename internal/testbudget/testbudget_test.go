package testbudget

import (
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

func TestClipNote(t *testing.T) {
	for _, tc := range []struct {
		name          string
		full, budget  time.Duration
		untilDeadline time.Duration
		want          string
	}{
		{name: "whole bound", full: Default, budget: Default, untilDeadline: time.Hour},
		{name: "bound cut by the deadline", full: Default, budget: 2400 * time.Millisecond, untilDeadline: 12400 * time.Millisecond,
			want: "testbudget: the wait bound is 2.4s, cut from 2m0s because the go test -timeout deadline is 12.4s away; a wait that ends at this bound reports a package run that is out of time, not an awaited event that is late"},
		{name: "override cut to the floor", full: 300 * time.Second, budget: time.Second, untilDeadline: 3 * time.Second,
			want: "testbudget: the wait bound is 1s, cut from 5m0s because the go test -timeout deadline is 3s away; a wait that ends at this bound reports a package run that is out of time, not an awaited event that is late"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := clipNote(tc.full, tc.budget, tc.untilDeadline); got != tc.want {
				t.Fatalf("clipNote = %q, want %q", got, tc.want)
			}
		})
	}
}
