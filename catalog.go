package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// neutralDir is where tools run. From inside a project, a tool can react to that
// project's settings: `go env` may download the Go toolchain its go.mod asks for.
// It must be writable, because pnpm fails when its working folder isn't.
var neutralDir = os.TempDir()

// outputFunc runs a command and returns its trimmed stdout. An error means the
// tool isn't usable here, so its item is left out.
type outputFunc func(name string, args ...string) (string, error)

func systemOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = neutralDir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func catalog(home string, output outputFunc) []*Item {
	h := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }

	dirCache := func(key, label, app string, ticked bool, dirs ...string) *Item {
		return &Item{Key: key, Section: sectionCaches, Label: label, Detail: dirs[0], Ticked: ticked,
			SizePaths: dirs, EmptyDirs: dirs, App: app}
	}
	logFile := func(key, label, path string) *Item {
		return &Item{Key: key, Section: sectionLogs, Label: label, Detail: path,
			SizePaths: []string{path}, Truncate: []string{path}}
	}

	items := []*Item{
		dirCache("derived-data", "Xcode DerivedData", "", true, h("Library/Developer/Xcode/DerivedData")),
		dirCache("xctest-devices", "Xcode XCTestDevices", "", true, h("Library/Developer/XCTestDevices")),
		dirCache("device-support", "iOS DeviceSupport", "", true, h("Library/Developer/Xcode/iOS DeviceSupport")),
		{Key: "simulators", Section: sectionCaches, Label: "Unavailable simulators",
			Detail: "xcrun simctl delete unavailable", Ticked: true,
			Commands: [][]string{{"xcrun", "simctl", "delete", "unavailable"}}},
		dirCache("chrome-sw", "Chrome Service Worker cache", "Google Chrome", true,
			h("Library/Application Support/Google/Chrome/Default/Service Worker/CacheStorage")),
		dirCache("brave", "Brave cache", "Brave Browser", true, h("Library/Caches/BraveSoftware")),
		dirCache("safari", "Safari cache", "Safari", true, h("Library/Containers/com.apple.Safari/Data/Library/Caches")),
		dirCache("slack", "Slack caches", "Slack", true,
			h("Library/Application Support/Slack/Service Worker/CacheStorage"),
			h("Library/Caches/com.tinyspeck.slackmacgap.ShipIt")),
		dirCache("npm", "npm cache", "", true, h(".npm")),
		dirCache("playwright", "Playwright browsers", "", true, h("Library/Caches/ms-playwright")),
		dirCache("trash", "Empty the Trash", "", false, h(".Trash")),
		dirCache("user-logs", "User logs and crash reports", "", true, h("Library/Logs")),
		dirCache("ai-junk", "AI tool junk (Claude, Gemini, opencode, devtools)", "", true,
			h(".claude/debug"), h(".claude/paste-cache"), h(".cache/chrome-devtools-mcp"), h(".gemini/tmp"), h(".local/share/opencode/log")),
		dirCache("shell-snapshots", "Claude shell snapshots", "claude", true, h(".claude/shell-snapshots")),
		logFile("postgres-log", "Postgres log", "/opt/homebrew/var/log/postgresql@18.log"),
		logFile("redis-log", "Redis log", "/opt/homebrew/var/log/redis.log"),
	}

	if dirs := containerCaches(home); len(dirs) > 0 {
		item := dirCache("container-caches", "App container caches", "", true, dirs...)
		item.Detail = filepath.Join(home, "Library/Containers/*/Data/Library/Caches")
		items = append(items, item)
	}
	tmp := filepath.Clean(os.TempDir())
	items = append(items, &Item{Key: "temp-files", Section: sectionCaches, Label: "Temp files older than 7 days",
		Detail: tmp, Ticked: true, SizePaths: []string{tmp},
		Commands: [][]string{{"find", tmp, "-type", "f", "-mtime", "+7", "-delete"}}})
	items = append(items, systemLogs())

	// Tool-owned caches: ask the tool where its cache is, and let it do the cleaning.
	tool := func(key, label, detail string, locate []string, commands ...[]string) {
		path, err := output(locate[0], locate[1:]...)
		if err != nil || path == "" {
			return
		}
		items = append(items, &Item{Key: key, Section: sectionCaches, Label: label, Detail: detail, Ticked: true,
			SizePaths: []string{path}, Commands: commands})
	}
	tool("uv", "uv cache", "uv cache clean", []string{"uv", "cache", "dir"}, []string{"uv", "cache", "clean"})
	tool("go-build", "Go build cache", "go clean -cache", []string{"go", "env", "GOCACHE"}, []string{"go", "clean", "-cache"})
	tool("go-mod", "Go module cache", "go clean -modcache", []string{"go", "env", "GOMODCACHE"}, []string{"go", "clean", "-modcache"})
	tool("pnpm", "pnpm store (unused packages)", "pnpm store prune", []string{"pnpm", "store", "path"}, []string{"pnpm", "store", "prune"})
	tool("brew", "Homebrew downloads and orphans", "brew autoremove; brew cleanup --prune=all -s", []string{"brew", "--cache"},
		[]string{"brew", "autoremove"}, []string{"brew", "cleanup", "--prune=all", "-s"})

	return items
}

// containerCaches lists each sandboxed app's cache folder, except Safari's,
// which has its own row guarded by whether Safari is running.
func containerCaches(home string) []string {
	matches, _ := filepath.Glob(filepath.Join(home, "Library/Containers/*/Data/Library/Caches"))
	var dirs []string
	for _, m := range matches {
		if !strings.Contains(m, "/com.apple.Safari/") {
			dirs = append(dirs, m)
		}
	}
	return dirs
}

// systemLogs needs root, so it starts unticked: a routine run never asks for a
// password. sudo prompts on the terminal itself.
func systemLogs() *Item {
	dirs := []string{"/private/var/log/DiagnosticMessages", "/Library/Logs/DiagnosticReports", "/Library/Logs/CrashReporter/CoreCapture"}
	rotated, _ := filepath.Glob("/private/var/log/system.log.*.gz")
	wifi, _ := filepath.Glob("/private/var/log/wifi.log.*.bz2")
	rotated = append(rotated, wifi...)

	commands := [][]string{append(append([]string{"sudo", "find"}, dirs...), "-type", "f", "-delete")}
	if len(rotated) > 0 {
		commands = append(commands, append([]string{"sudo", "rm", "-f"}, rotated...))
	}
	return &Item{Key: "system-logs", Section: sectionLogs, Label: "System logs and crash reports (sudo)",
		Detail: "/private/var/log, /Library/Logs", SizePaths: append(dirs, rotated...), Commands: commands}
}
