// SPDX-License-Identifier: MIT

//go:build linux

package perfgate

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Memory is a process's resident memory in bytes, as /proc/<pid>/smaps_rollup accounts it.
//
// Rss and Pss count every resident page the process maps, so they include the pages of the binary and of shared libraries the kernel has in its page cache: how many of those are mapped at a given moment depends on readahead and on what else touched the file, not on the process. Anon is the pages that belong to the process alone (its heap, stacks and written data), which does not.
type Memory struct {
	Rss, Pss, Anon float64
}

// ProcessMemory returns the resident, proportional and anonymous memory of process pid, from /proc/<pid>/smaps_rollup.
func ProcessMemory(t testing.TB, pid int) Memory {
	t.Helper()
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/smaps_rollup", pid))
	if err != nil {
		t.Fatal(err)
	}
	var memory Memory
	//portlint:allow pathseparators the kernel writes /proc/<pid>/smaps_rollup with \n line ends only; Go-only memory gate, no Pi counterpart
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "kB" {
			continue
		}
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			t.Fatalf("smaps_rollup %q: %v", line, err)
		}
		switch fields[0] {
		case "Rss:":
			memory.Rss = value * 1024
		case "Pss:":
			memory.Pss = value * 1024
		case "Anonymous:":
			memory.Anon = value * 1024
		}
	}
	if memory.Rss == 0 || memory.Pss == 0 || memory.Anon == 0 {
		t.Fatalf("smaps_rollup of %d has no Rss, Pss or Anonymous:\n%s", pid, data)
	}
	return memory
}

// Descendant returns the first process named comm below pid, searching children breadth-first through /proc/<pid>/task/*/children, or 0.
func Descendant(pid int, comm string) int {
	queue := []int{pid}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		tasks, _ := os.ReadDir(fmt.Sprintf("/proc/%d/task", current))
		for _, task := range tasks {
			data, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%s/children", current, task.Name()))
			if err != nil {
				continue
			}
			for field := range strings.FieldsSeq(string(data)) {
				child, err := strconv.Atoi(field)
				if err != nil {
					continue
				}
				name, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", child))
				if strings.TrimSpace(string(name)) == comm {
					return child
				}
				queue = append(queue, child)
			}
		}
	}
	return 0
}
