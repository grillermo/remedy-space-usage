package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// measure returns the disk used by paths, in KB. du exits non-zero when a path is
// missing or unreadable but still prints a total for the rest, so the exit
// status is ignored. -c gives one total, and du counts hard links once within it.
func measure(paths []string) int64 {
	out, _ := exec.Command("du", append([]string{"-skc"}, paths...)...).Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) == 0 {
		return 0
	}
	kb, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	return kb
}
