package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeOutput(missing ...string) outputFunc {
	return func(name string, args ...string) (string, error) {
		for _, m := range missing {
			if m == name {
				return "", errors.New("not found")
			}
		}
		return "/tools/" + name + "/" + strings.Join(args, "-"), nil
	}
}

func find(items []*Item, key string) *Item {
	for _, it := range items {
		if it.Key == key {
			return it
		}
	}
	return nil
}

func TestCatalogDefaults(t *testing.T) {
	items := catalog("/h", fakeOutput())
	for _, key := range []string{"derived-data", "xctest-devices", "npm", "go-build"} {
		if it := find(items, key); it == nil || !it.Ticked {
			t.Errorf("%s should exist and start ticked", key)
		}
	}
	for _, key := range []string{"trash", "postgres-log", "redis-log"} {
		if it := find(items, key); it == nil || it.Ticked {
			t.Errorf("%s should exist and start unticked", key)
		}
	}
}

func TestCatalogOmitsMissingTools(t *testing.T) {
	items := catalog("/h", fakeOutput("uv", "pnpm"))
	if find(items, "uv") != nil || find(items, "pnpm") != nil {
		t.Error("items for missing tools must be left out")
	}
	if find(items, "derived-data") == nil {
		t.Error("items with no tool must stay")
	}
}

func TestCatalogCleanMyMacItems(t *testing.T) {
	home := t.TempDir()
	touch(t, home+"/Library/Containers/com.example.app/Data/Library/Caches/f")
	touch(t, home+"/Library/Containers/com.apple.Safari/Data/Library/Caches/f")
	items := catalog(home, fakeOutput())

	for _, key := range []string{"user-logs", "ai-junk", "shell-snapshots", "container-caches", "temp-files"} {
		if it := find(items, key); it == nil || !it.Ticked {
			t.Errorf("%s should exist and start ticked", key)
		}
	}
	if got := find(items, "shell-snapshots").App; got != "claude" {
		t.Errorf("shell-snapshots app guard = %q", got)
	}
	dirs := find(items, "container-caches").EmptyDirs
	if len(dirs) != 1 || !strings.Contains(dirs[0], "com.example.app") {
		t.Errorf("container caches must leave out Safari, which has its own row: %v", dirs)
	}
	system := find(items, "system-logs")
	if system == nil || system.Ticked {
		t.Fatal("system-logs should exist and start unticked")
	}
	for _, c := range system.Commands {
		if c[0] != "sudo" {
			t.Errorf("system log command must use sudo: %v", c)
		}
	}
}

func TestCatalogGuardsRunningApps(t *testing.T) {
	if got := find(catalog("/h", fakeOutput()), "chrome-sw").App; got != "Google Chrome" {
		t.Errorf("chrome app guard = %q", got)
	}
}

func TestNeutralDirIsWritable(t *testing.T) {
	f, err := os.CreateTemp(neutralDir, "probe")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())
}
