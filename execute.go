package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// removeAll deletes a path, making it writable first if needed: node_modules and
// other caches often contain read-only folders, which block os.RemoveAll.
func removeAll(path string) error {
	if err := os.RemoveAll(path); err == nil {
		return nil
	}
	exec.Command("chmod", "-R", "u+w", path).Run()
	return os.RemoveAll(path)
}

func emptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		errs = append(errs, removeAll(filepath.Join(dir, e.Name())))
	}
	return errors.Join(errs...)
}

func perform(it *Item) error {
	var errs []error
	for _, dir := range it.EmptyDirs {
		errs = append(errs, emptyDir(dir))
	}
	for _, path := range it.RemovePaths {
		errs = append(errs, removeAll(path))
	}
	for _, path := range it.Truncate {
		if err := os.Truncate(path, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	for _, c := range it.Commands {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w: %s", strings.Join(c, " "), err, strings.TrimSpace(string(out))))
		}
	}
	return errors.Join(errs...)
}

// describe says exactly what perform will do, for the review screen.
func describe(it *Item) []string {
	var lines []string
	for _, p := range it.EmptyDirs {
		lines = append(lines, "delete contents of "+p)
	}
	for _, p := range it.RemovePaths {
		lines = append(lines, "delete "+p)
	}
	for _, p := range it.Truncate {
		lines = append(lines, "empty "+p)
	}
	for _, c := range it.Commands {
		lines = append(lines, "run "+strings.Join(c, " "))
	}
	return lines
}

type guards struct {
	running func(app string) bool
	cwds    []string
}

func systemGuards() guards {
	out, _ := exec.Command("lsof", "-d", "cwd", "-Fn").Output()
	return guards{
		running: func(app string) bool { return exec.Command("pgrep", "-x", app).Run() == nil },
		cwds:    parseCwds(string(out)),
	}
}

// parseCwds reads `lsof -Fn` output, where path lines start with "n". A process
// sitting in / says nothing about any project, so it is dropped.
func parseCwds(out string) []string {
	var cwds []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "n") && line != "n/" {
			cwds = append(cwds, line[1:])
		}
	}
	return cwds
}

// blockReason says why an item must be skipped right now, or "" if it's clear.
func (g guards) blockReason(it *Item) string {
	if it.App != "" && g.running(it.App) {
		return it.App + " is running"
	}
	if it.InUseRoot != "" {
		for _, cwd := range g.cwds {
			if cwd == it.InUseRoot || strings.HasPrefix(cwd, it.InUseRoot+"/") {
				return "a running process is using " + it.InUseRoot
			}
		}
	}
	return ""
}

// savedKB is what an item saved: its scanned size minus what's left, or the
// change in free disk space when it has no paths to measure.
func savedKB(it *Item, diskChangeKB int64) int64 {
	if len(it.SizePaths) == 0 {
		return max(diskChangeKB, 0)
	}
	return max(it.SizeKB-measure(it.SizePaths), 0)
}

// freeKB is the space available on the startup volume.
func freeKB() int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize) / 1024
}
