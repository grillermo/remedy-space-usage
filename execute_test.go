package main

import (
	"os"
	"strings"
	"testing"
)

func TestRemoveAllHandlesReadOnlyFolders(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir+"/ro/sub/f")
	os.Chmod(dir+"/ro/sub", 0o555)
	os.Chmod(dir+"/ro", 0o555)
	if err := removeAll(dir + "/ro"); err != nil {
		t.Fatal(err)
	}
	if exists(dir + "/ro") {
		t.Error("folder still there")
	}
}

func TestPerformEmptiesDeletesAndTruncates(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir+"/cache/a/b")
	touch(t, dir+"/old.log.0")
	touch(t, dir+"/current.log")
	it := &Item{
		EmptyDirs:   []string{dir + "/cache", dir + "/missing"},
		RemovePaths: []string{dir + "/old.log.0"},
		Truncate:    []string{dir + "/current.log", dir + "/missing.log"},
		Commands:    [][]string{{"touch", dir + "/ran"}},
	}
	if err := perform(it); err != nil {
		t.Fatal(err)
	}
	if !exists(dir+"/cache") || exists(dir+"/cache/a") {
		t.Error("cache folder should stay, empty")
	}
	if exists(dir + "/old.log.0") {
		t.Error("rotated log should be gone")
	}
	if fi, err := os.Stat(dir + "/current.log"); err != nil || fi.Size() != 0 {
		t.Errorf("current log should exist and be empty: %v %v", fi, err)
	}
	if exists(dir + "/missing.log") {
		t.Error("truncate must not create files")
	}
	if !exists(dir + "/ran") {
		t.Error("command did not run")
	}
}

func TestPerformReportsCommandOutput(t *testing.T) {
	err := perform(&Item{Commands: [][]string{{"sh", "-c", "echo broke >&2; exit 3"}}})
	if err == nil || !strings.Contains(err.Error(), "broke") {
		t.Errorf("err = %v", err)
	}
}

func TestBlockReason(t *testing.T) {
	g := guards{running: func(app string) bool { return app == "Slack" }, cwds: []string{"/c/app/sub"}}
	if g.blockReason(&Item{App: "Slack"}) == "" {
		t.Error("running app should block")
	}
	if g.blockReason(&Item{App: "Safari"}) != "" {
		t.Error("stopped app should not block")
	}
	if g.blockReason(&Item{InUseRoot: "/c/app"}) == "" {
		t.Error("process inside project should block")
	}
	if g.blockReason(&Item{InUseRoot: "/c/ap"}) != "" {
		t.Error("a name prefix is not inside the project")
	}
}

func TestSavedKB(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir+"/kept")
	it := &Item{SizePaths: []string{dir + "/kept", dir + "/gone"}, SizeKB: 100000}
	after := measure(it.SizePaths)
	if got := savedKB(it, 0); got != 100000-after {
		t.Errorf("savedKB = %d, want %d", got, 100000-after)
	}
	if got := savedKB(&Item{SizePaths: []string{dir + "/kept"}, SizeKB: 0}, 0); got != 0 {
		t.Errorf("a path that grew must report 0, got %d", got)
	}
	if got := savedKB(&Item{}, 2048); got != 2048 {
		t.Errorf("item with no paths uses the disk change, got %d", got)
	}
	if got := savedKB(&Item{}, -5); got != 0 {
		t.Errorf("negative disk change must report 0, got %d", got)
	}
}

func TestParseCwds(t *testing.T) {
	got := parseCwds("p1\nfcwd\nn/a/b\np2\nfcwd\nn/\n")
	if len(got) != 1 || got[0] != "/a/b" {
		t.Errorf("got %v (the root cwd must be dropped)", got)
	}
}

func TestDescribeListsEverythingThatWillHappen(t *testing.T) {
	lines := describe(&Item{EmptyDirs: []string{"/d"}, RemovePaths: []string{"/r"}, Truncate: []string{"/t"}, Commands: [][]string{{"go", "clean"}}})
	got := strings.Join(lines, "\n")
	for _, want := range []string{"delete contents of /d", "delete /r", "empty /t", "run go clean"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
