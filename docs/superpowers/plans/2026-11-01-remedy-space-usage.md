# remedy-space-usage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `remedy-space-usage` command that opens a chicle checklist at once, fills in the size of each safe cleanup on this machine, and permanently deletes what you tick after a review screen, then reports the space saved on each item and in total.

**Architecture:** One `package main`. A catalog describes each cleanup as data (`Item`), a discovery pass adds per-project rows, and a scanner measures everything with `du` and streams rows to chicle through `Config.Updates`. Execution happens after the picker closes, on the normal terminal, so the review and results stay in scrollback.

**Tech Stack:** Go 1.26, `github.com/grillermo/chicle` v0.4.0, system `du`, `lsof`, `pgrep`.

**Commits:** The repo has no commits yet. Before Task 1 run `git switch -c feature/tui`. End every commit message with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

## Scope decisions

Source checklist: `remedial-space-saving-measures.html`. "Safe items only" means regrowable caches, build artifacts and logs, on this machine only.

| Starts ticked | Starts unticked | Left out of this version |
|---|---|---|
| DerivedData, XCTestDevices, unavailable simulators, iOS DeviceSupport | Each `~/Library/Caches` and `~/.cache` entry of 100 MB or more (could hold costly downloads such as models) | CleanMyMac's riskier work (see below) |
| Chrome Service Worker, Brave and Safari caches, Slack caches | Trash (holds things you chose to delete, but may still want) | Mail attachments and Mail trash |
| npm, uv, Go build and module caches, pnpm store, Homebrew, Playwright | `node_modules`, Rust `target`, Swift `.build` and `.build-run`, Next.js turbopack cache, per project | Finished projects, media, downloads, models, apps, "gone for good" leftovers |
| Rails dev/test logs, `~/Library/Logs`, AI tool junk, app container caches, temp files older than 7 days | Postgres and Redis logs, system logs (need your sudo password) | Old Claude sessions, plugin clones, pipx venvs, Apple Music tracks, merged worktrees |
| | | `vendor/bundle`, `public/assets`, `tmp/`, Python venvs |

The last row is a deliberate narrowing of the checklist's section 2: the macmini runs real apps from `~/c`, and those folders can be part of a running deployment.

## What this takes over from CleanMyMac

Based on its removal log on the macmini (8,493 paths) and the checklist's section 0.

| CleanMyMac does | This app |
|---|---|
| Xcode junk, caches, Trash | Already covered above |
| User logs (`~/Library/Logs`) | New item, ticked |
| AI junk | New items, ticked: `~/.claude/debug`, `~/.claude/paste-cache`, `~/.cache/chrome-devtools-mcp`, `~/.gemini/tmp`, opencode logs. `~/.claude/shell-snapshots` is its own row, skipped while Claude Code is running because live sessions use it |
| App caches in `~/Library/Containers` | New item, ticked. Reading these may trigger one macOS "access data from other apps" prompt the first time |
| Temp files in `/private/var/folders` | New item, ticked: only files older than 7 days in `$TMPDIR` |
| System logs and crash reports | New item, unticked so routine runs never ask for a password: rotated `system.log` and `wifi.log`, `DiagnosticMessages`, `/Library/Logs/DiagnosticReports`, `/Library/Logs/CrashReporter/CoreCapture` |
| Thinning universal binaries, removing language files (2,318 `.lproj` removals in the log) | **Not included.** It edits apps in `/Applications`, which breaks their code signatures unless they are re-signed, and app updates undo it |
| Mail attachments and trash | **Not included.** Mail owns that store, and deleting files behind its back risks corrupting it |
| Broken login items | **Not included.** Frees no meaningful space |
| Space Lens | **Not included.** It's an interactive explorer; `disk-space-differ` already covers finding large folders |

## Safety rules

- Nothing is deleted until a review screen, printing every path and command, is answered with `y`.
- A project row is skipped when a running process has its working directory inside that project (checked with `lsof`).
- A browser or Slack cache is skipped while that app is running.
- Rails logs: only `development`, `test` and `server_dev` logs. Current logs are truncated and rotated ones deleted. `production.log` and `caddy_access*.log` are never touched.
- Service logs are truncated, never deleted.

## Reporting space saved

After each item runs, the results list shows how much it saved, followed by a total that adds up the items.

- **Items with paths:** the size from the scan minus a new `du` of the same paths. This is accurate even if something else on the machine is writing to disk, and it shows the real result of commands like `pnpm store prune`, whose scan size is only an upper limit.
- **Items with no paths** (unavailable simulators): the change in free disk space while that item ran.
- **Skipped or failed items:** a skipped item counts as nothing saved. A failed item still reports what it did free, because it may have partly worked.
- The final line also gives the change in free disk space, because APFS can hold space back and the two numbers can differ.

## Chicle behaviours the code relies on

- `applyUpdate` ORs old ticks with `Row.Ticked` on incoming rows. An update that repeats `Ticked: true` would re-tick a row you just unticked, so a row may carry its default tick only the **first** time it is sent. `rowsFor` enforces this.
- `Config.Title` is static, so there is no live "selected total" in the list. The total appears on the review screen.
- `enter` toggles a row, `tab` reaches the buttons and printable keys type into the filter, so the hotkey must be a named key. This plan uses `ctrl+d`.
- `Run` draws on `/dev/tty` in the alt screen, so stdout is clean afterwards for the review.

## File structure

| File | Responsibility |
|---|---|
| `item.go` | `Item` type, size formatting |
| `catalog.go` | Fixed list of cleanups |
| `discover.go` | Finds per-project artifacts, Rails logs, large cache entries |
| `measure.go` | Runs `du` |
| `scanner.go` | Measures concurrently, turns items into chicle rows |
| `execute.go` | Guards, deletes, runs commands |
| `main.go` | Picker, review screen, results |
| `install` | Builds into `~/.local/bin` |

---

### Task 1: Item type and size formatting

**Files:**
- Create: `go.mod`, `item.go`, `item_test.go`

- [ ] **Step 1: Create the module**

```bash
go mod init github.com/grillermo/remedy-space-usage
```

- [ ] **Step 2: Write the failing test**

`item_test.go`:

```go
package main

import "testing"

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{0: "0 B", 512: "512 KB", 2048: "2.0 MB", 5 * 1024 * 1024: "5.0 GB"}
	for kb, want := range cases {
		if got := formatSize(kb); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", kb, got, want)
		}
	}
}

func TestSizeLabel(t *testing.T) {
	if got := (&Item{SizePaths: []string{"/x"}}).sizeLabel(); got != "…" {
		t.Errorf("unmeasured = %q", got)
	}
	if got := (&Item{}).sizeLabel(); got != "?" {
		t.Errorf("unmeasurable = %q", got)
	}
	if got := (&Item{SizePaths: []string{"/x"}, Measured: true, SizeKB: 2048}).sizeLabel(); got != "2.0 MB" {
		t.Errorf("measured = %q", got)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./...`
Expected: FAIL, `undefined: formatSize`.

- [ ] **Step 4: Implement**

`item.go`:

```go
package main

import "fmt"

const (
	sectionCaches    = "Caches"
	sectionArtifacts = "Project build artifacts"
	sectionLogs      = "Logs"
)

// Item is one row in the checklist and everything needed to carry it out.
type Item struct {
	Key     string
	Section string
	Label   string
	Detail  string
	// Ticked is the row's default. It is sent to chicle only once; see rowsFor.
	Ticked bool

	// SizePaths are measured with du. Empty means the size can't be known ahead of time.
	SizePaths []string
	// MinSizeKB hides the row once measured if it is smaller than this.
	MinSizeKB int64

	EmptyDirs   []string   // the contents are deleted, the folder stays
	RemovePaths []string   // deleted whole
	Truncate    []string   // emptied in place
	Commands    [][]string // run in order

	App       string // skipped while this process name is running
	InUseRoot string // skipped while a process has its working directory here

	SizeKB   int64
	Measured bool
}

func (i *Item) sizeLabel() string {
	if len(i.SizePaths) == 0 {
		return "?"
	}
	if !i.Measured {
		return "…"
	}
	return formatSize(i.SizeKB)
}

func formatSize(kb int64) string {
	switch {
	case kb == 0:
		return "0 B"
	case kb < 1024:
		return fmt.Sprintf("%d KB", kb)
	case kb < 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(kb)/1024)
	default:
		return fmt.Sprintf("%.1f GB", float64(kb)/(1024*1024))
	}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod item.go item_test.go
git commit -m "Add item type and size formatting"
```

---

### Task 2: Catalog of cleanups

**Files:**
- Create: `catalog.go`, `catalog_test.go`

- [ ] **Step 1: Write the failing test**

`catalog_test.go`:

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./...`
Expected: FAIL, `undefined: outputFunc`.

- [ ] **Step 3: Implement**

`catalog.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// outputFunc runs a command and returns its trimmed stdout. An error means the
// tool isn't usable here, so its item is left out.
type outputFunc func(name string, args ...string) (string, error)

func systemOutput(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
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
```

- [ ] **Step 4: Run the tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add catalog.go catalog_test.go
git commit -m "Add catalog of cleanups"
```

---

### Task 3: Discovery

**Files:**
- Create: `discover.go`, `discover_test.go`

- [ ] **Step 1: Write the failing test**

`discover_test.go`:

```go
package main

import "testing"

func TestDiscoverArtifactsNeedsAProjectMarker(t *testing.T) {
	root := t.TempDir()
	touch(t, root+"/rusty/Cargo.toml")
	touch(t, root+"/rusty/target/a")
	touch(t, root+"/notes/target/a")
	touch(t, root+"/web/package.json")
	touch(t, root+"/web/node_modules/dep/package.json")
	touch(t, root+"/web/node_modules/dep/node_modules/x/a")
	touch(t, root+"/swifty/Package.swift")
	touch(t, root+"/swifty/.build/a")
	touch(t, root+"/next/.next/dev/cache/turbopack/a")
	touch(t, root+"/app/vendor/bundle/a")
	touch(t, root+"/app/tmp/a")

	items := discover(root)

	for _, key := range []string{"artifact:" + root + "/rusty/target", "artifact:" + root + "/web/node_modules",
		"artifact:" + root + "/swifty/.build", "artifact:" + root + "/next/.next/dev/cache/turbopack"} {
		it := find(items, key)
		if it == nil {
			t.Errorf("missing %s", key)
			continue
		}
		if it.Ticked {
			t.Errorf("%s must start unticked", key)
		}
	}
	if len(items) != 4 {
		t.Errorf("got %d items, want 4 (no notes/target, nested node_modules, vendor or tmp)", len(items))
	}
	if got := find(items, "artifact:"+root+"/rusty/target").InUseRoot; got != root+"/rusty" {
		t.Errorf("InUseRoot = %q", got)
	}
	if got := find(items, "artifact:"+root+"/next/.next/dev/cache/turbopack").InUseRoot; got != root+"/next" {
		t.Errorf("turbopack InUseRoot = %q", got)
	}
}

func TestDiscoverRailsLogsLeavesProductionAlone(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"development.log", "development.log.0", "test.log", "server_dev.log", "production.log", "caddy_access.log"} {
		touch(t, root+"/app/log/"+f)
	}
	it := find(discover(root), "rails-logs")
	if it == nil || !it.Ticked {
		t.Fatal("rails-logs should exist and start ticked")
	}
	if len(it.Truncate) != 3 || len(it.RemovePaths) != 1 || it.RemovePaths[0] != root+"/app/log/development.log.0" {
		t.Errorf("truncate=%v remove=%v", it.Truncate, it.RemovePaths)
	}
}

func TestLargeEntriesSkipsClaimedPaths(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir+"/huggingface/a")
	touch(t, dir+"/uv/a")
	touch(t, dir+"/brave/sub/a")
	items := largeEntries(dir, []string{dir + "/uv", dir + "/brave/sub"})
	if len(items) != 1 || items[0].Key != "cache:"+dir+"/huggingface" {
		t.Fatalf("got %+v", items)
	}
	if it := items[0]; it.Ticked || it.MinSizeKB != 100*1024 || it.RemovePaths[0] != dir+"/huggingface" {
		t.Errorf("got %+v", it)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./...`
Expected: FAIL, `undefined: discover`.

- [ ] **Step 3: Implement**

`discover.go`:

```go
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const discoverMaxDepth = 4

var (
	currentLog = regexp.MustCompile(`^(development|test|server_dev)\.log$`)
	rotatedLog = regexp.MustCompile(`^(development|test|server_dev)\.log\.[0-9]+$`)
	// A folder only counts as an artifact next to its project's marker, so a
	// folder that merely happens to be called "target" is never offered.
	artifactMarkers = map[string]string{
		"target":       "Cargo.toml",
		".build":       "Package.swift",
		".build-run":   "Package.swift",
		"node_modules": "package.json",
	}
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func artifactItem(path, project string) *Item {
	name, err := filepath.Rel(project, path)
	if err != nil {
		name = filepath.Base(path)
	}
	return &Item{Key: "artifact:" + path, Section: sectionArtifacts,
		Label: filepath.Base(project) + "/" + name, Detail: path,
		SizePaths: []string{path}, RemovePaths: []string{path}, InUseRoot: project}
}

// discover walks the projects folder for build artifacts and Rails logs.
func discover(root string) []*Item {
	var items []*Item
	logs := &Item{Key: "rails-logs", Section: sectionLogs, Label: "Rails dev/test logs", Ticked: true}

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() || path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.Count(rel, string(filepath.Separator))+1 > discoverMaxDepth {
			return filepath.SkipDir
		}
		name, parent := d.Name(), filepath.Dir(path)
		switch {
		case name == ".git":
			return filepath.SkipDir
		case name == ".next":
			if turbo := filepath.Join(path, "dev/cache/turbopack"); exists(turbo) {
				items = append(items, artifactItem(turbo, parent))
			}
			return filepath.SkipDir
		case name == "log":
			collectLogs(path, logs)
			return filepath.SkipDir
		}
		if marker, ok := artifactMarkers[name]; ok && exists(filepath.Join(parent, marker)) {
			items = append(items, artifactItem(path, parent))
			return filepath.SkipDir
		}
		if name == "node_modules" {
			return filepath.SkipDir
		}
		return nil
	})

	if len(logs.SizePaths) > 0 {
		logs.Detail = fmt.Sprintf("%d files in %s", len(logs.SizePaths), root)
		items = append(items, logs)
	}
	return items
}

func collectLogs(dir string, logs *Item) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
		case currentLog.MatchString(e.Name()):
			logs.Truncate = append(logs.Truncate, path)
			logs.SizePaths = append(logs.SizePaths, path)
		case rotatedLog.MatchString(e.Name()):
			logs.RemovePaths = append(logs.RemovePaths, path)
			logs.SizePaths = append(logs.SizePaths, path)
		}
	}
}

// largeEntries offers each entry of a catch-all cache folder separately, except
// those another item already covers, so no space is counted twice.
func largeEntries(dir string, claimed []string) []*Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var items []*Item
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		isClaimed := false
		for _, c := range claimed {
			if c == path || strings.HasPrefix(c, path+string(filepath.Separator)) {
				isClaimed = true
				break
			}
		}
		if isClaimed {
			continue
		}
		items = append(items, &Item{Key: "cache:" + path, Section: sectionCaches, Label: e.Name(), Detail: path,
			SizePaths: []string{path}, RemovePaths: []string{path}, MinSizeKB: 100 * 1024})
	}
	return items
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add discover.go discover_test.go
git commit -m "Discover project artifacts, Rails logs and large cache entries"
```

---

### Task 4: Measuring and the scanner

**Files:**
- Create: `measure.go`, `scanner.go`, `scanner_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add chicle**

```bash
go get github.com/grillermo/chicle@v0.4.0
```

- [ ] **Step 2: Write the failing test**

`scanner_test.go`:

```go
package main

import (
	"os"
	"testing"
)

func TestMeasure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/f", make([]byte, 64*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if kb := measure([]string{dir, dir + "/missing"}); kb < 64 {
		t.Errorf("measure = %d KB, want at least 64", kb)
	}
	if kb := measure([]string{dir + "/missing"}); kb != 0 {
		t.Errorf("missing path = %d KB, want 0", kb)
	}
}

// Repeating Ticked:true would make chicle re-tick a row the user just unticked.
func TestRowsCarryDefaultTickOnlyOnce(t *testing.T) {
	s := &Scanner{items: []*Item{{Key: "a", Label: "A", Ticked: true, SizePaths: []string{"/x"}}}, sent: map[string]bool{}}
	if rows := s.rows(); !rows[0].Ticked {
		t.Error("first send must carry the default tick")
	}
	if rows := s.rows(); rows[0].Ticked {
		t.Error("later sends must not carry a tick")
	}
}

func TestRowsHideEmptyAndSmallItems(t *testing.T) {
	s := &Scanner{sent: map[string]bool{}, items: []*Item{
		{Key: "empty", SizePaths: []string{"/x"}, Measured: true},
		{Key: "small", SizePaths: []string{"/x"}, Measured: true, SizeKB: 10, MinSizeKB: 100},
		{Key: "big", SizePaths: []string{"/x"}, Measured: true, SizeKB: 500, MinSizeKB: 100},
		{Key: "pending", SizePaths: []string{"/x"}},
		{Key: "unmeasurable"},
	}}
	got := map[string]bool{}
	for _, r := range s.rows() {
		got[r.Key] = true
	}
	if got["empty"] || got["small"] || !got["big"] || !got["pending"] || !got["unmeasurable"] {
		t.Errorf("rows = %v", got)
	}
}

func TestSortKeepsSectionsTogetherLargestFirst(t *testing.T) {
	s := &Scanner{sent: map[string]bool{}, items: []*Item{
		{Key: "log", Section: sectionLogs, SizePaths: []string{"/x"}, Measured: true, SizeKB: 900},
		{Key: "small", Section: sectionCaches, SizePaths: []string{"/x"}, Measured: true, SizeKB: 1},
		{Key: "art", Section: sectionArtifacts, SizePaths: []string{"/x"}, Measured: true, SizeKB: 5},
		{Key: "big", Section: sectionCaches, SizePaths: []string{"/x"}, Measured: true, SizeKB: 9},
	}}
	s.sortItems()
	var keys []string
	for _, it := range s.items {
		keys = append(keys, it.Key)
	}
	want := []string{"big", "small", "art", "log"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("order = %v, want %v", keys, want)
		}
	}
}

func TestScanFinishes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/f", make([]byte, 8192), 0o644); err != nil {
		t.Fatal(err)
	}
	extra := &Item{Key: "found", SizePaths: []string{dir}}
	s := newScanner([]*Item{{Key: "a", SizePaths: []string{dir}}}, func() []*Item { return []*Item{extra} })
	updates := s.start()
	for range updates {
	}
	if !s.finished() {
		t.Error("scan should be finished once updates closes")
	}
	if !extra.Measured || find(s.items, "a").SizeKB == 0 {
		t.Errorf("items not measured: %+v", s.items)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./...`
Expected: FAIL, `undefined: measure`.

- [ ] **Step 4: Implement**

`measure.go`:

```go
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
```

`scanner.go`:

```go
package main

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/grillermo/chicle"
)

const scanWorkers = 4

var sectionOrder = map[string]int{sectionCaches: 0, sectionArtifacts: 1, sectionLogs: 2}

type Scanner struct {
	mu       sync.Mutex
	items    []*Item
	sent     map[string]bool
	discover func() []*Item
	done     atomic.Bool
}

func newScanner(items []*Item, discover func() []*Item) *Scanner {
	return &Scanner{items: items, discover: discover, sent: map[string]bool{}}
}

func (s *Scanner) finished() bool { return s.done.Load() }

// rows renders the current items. A row carries its default tick only the first
// time it is sent: chicle ORs incoming ticks with its own, so repeating a tick
// would undo the user unticking that row.
func (s *Scanner) rows() []chicle.Row {
	var rows []chicle.Row
	for _, it := range s.items {
		if it.Measured && (it.SizeKB == 0 || it.SizeKB < it.MinSizeKB) {
			continue
		}
		rows = append(rows, chicle.Row{
			Key:     it.Key,
			Section: it.Section,
			Cols:    []string{it.sizeLabel(), it.Label, it.Detail},
			Ticked:  it.Ticked && !s.sent[it.Key],
		})
		s.sent[it.Key] = true
	}
	return rows
}

func (s *Scanner) sortItems() {
	sort.SliceStable(s.items, func(a, b int) bool {
		x, y := s.items[a], s.items[b]
		if x.Section != y.Section {
			return sectionOrder[x.Section] < sectionOrder[y.Section]
		}
		return x.SizeKB > y.SizeKB
	})
}

func (s *Scanner) snapshot() []chicle.Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rows()
}

// start discovers and measures in the background. Rows keep their order during
// the scan so the list doesn't jump, then are sorted largest first at the end.
func (s *Scanner) start() <-chan []chicle.Row {
	updates := make(chan []chicle.Row)
	go func() {
		defer close(updates)

		found := s.discover()
		s.mu.Lock()
		s.items = append(s.items, found...)
		s.sortItems()
		pending := append([]*Item(nil), s.items...)
		s.mu.Unlock()
		updates <- s.snapshot()

		work := make(chan *Item)
		var wg sync.WaitGroup
		for range scanWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for it := range work {
					kb := measure(it.SizePaths)
					s.mu.Lock()
					it.SizeKB, it.Measured = kb, true
					rows := s.rows()
					s.mu.Unlock()
					updates <- rows
				}
			}()
		}
		for _, it := range pending {
			if len(it.SizePaths) > 0 {
				work <- it
			}
		}
		close(work)
		wg.Wait()

		s.mu.Lock()
		s.sortItems()
		rows := s.rows()
		s.mu.Unlock()
		s.done.Store(true)
		updates <- rows
	}()
	return updates
}

// selected returns the items for the given row keys, in list order.
func (s *Scanner) selected(keys map[string]bool) []*Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Item
	for _, it := range s.items {
		if keys[it.Key] {
			out = append(out, it)
		}
	}
	return out
}
```

- [ ] **Step 5: Run the tests**

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum measure.go scanner.go scanner_test.go
git commit -m "Scan item sizes in the background and stream chicle rows"
```

---

### Task 5: Executing a selection

**Files:**
- Create: `execute.go`, `execute_test.go`

- [ ] **Step 1: Write the failing test**

`execute_test.go`:

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./...`
Expected: FAIL, `undefined: removeAll`.

- [ ] **Step 3: Implement**

`execute.go`:

```go
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
```

- [ ] **Step 4: Run the tests**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add execute.go execute_test.go
git commit -m "Carry out cleanups with guards for running apps and projects"
```

---

### Task 6: Picker, review screen, install

**Files:**
- Create: `main.go`, `install`, `.gitignore`

- [ ] **Step 1: Implement the command**

`main.go`:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grillermo/chicle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "remedy-space-usage:", err)
		os.Exit(1)
	}
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	items := catalog(home, systemOutput)
	scanner := newScanner(items, func() []*Item {
		var claimed []string
		for _, it := range items {
			claimed = append(claimed, it.SizePaths...)
		}
		found := discover(filepath.Join(home, "c"))
		found = append(found, largeEntries(filepath.Join(home, "Library/Caches"), claimed)...)
		return append(found, largeEntries(filepath.Join(home, ".cache"), claimed)...)
	})

	chosen, err := pick(scanner)
	if err != nil || len(chosen) == 0 {
		return err
	}
	if !review(chosen) {
		fmt.Println("Nothing deleted.")
		return nil
	}
	execute(chosen)
	return nil
}

func pick(scanner *Scanner) ([]*Item, error) {
	initial := scanner.snapshot()
	keys := map[string]bool{}
	result, err := chicle.Run(chicle.Config{
		Title:       "Free up space: enter ticks, tab reaches the buttons, ctrl+d cleans (once the scan ends)",
		MultiSelect: true,
		NoCycle:     true,
		Columns:     []chicle.Column{{Title: "Size", Width: 9}, {Title: "Item", Width: 38}, {Title: "Where"}},
		Rows:        initial,
		Updates:     scanner.start(),
		Actions: []chicle.Action{
			{
				Label: "Clean",
				Key:   "ctrl+d",
				Show:  func(chicle.Selection) bool { return scanner.finished() },
				Run: func(sel chicle.Selection) chicle.Outcome {
					if len(sel.Ticked) == 0 {
						return chicle.Outcome{Status: "Nothing ticked"}
					}
					for _, r := range sel.Ticked {
						keys[r.Key] = true
					}
					return chicle.Outcome{Result: "clean", Done: true}
				},
			},
			{Label: "Quit"},
		},
	})
	if err != nil || result != "clean" {
		return nil, err
	}
	return scanner.selected(keys), nil
}

func review(items []*Item) bool {
	var total int64
	for _, it := range items {
		total += it.SizeKB
		fmt.Printf("\n%s  (%s)\n", it.Label, it.sizeLabel())
		for _, line := range describe(it) {
			fmt.Println("  " + line)
		}
	}
	fmt.Printf("\nThis permanently deletes up to %s. Continue? [y/N] ", formatSize(total))
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(answer), "y")
}

func execute(items []*Item) {
	start := freeKB()
	g := systemGuards()
	failed := 0
	var total int64
	for _, it := range items {
		if reason := g.blockReason(it); reason != "" {
			fmt.Printf("SKIP  %-38s %s\n", it.Label, reason)
			continue
		}
		fmt.Printf("...   %s\n", it.Label)
		before := freeKB()
		err := perform(it)
		saved := savedKB(it, freeKB()-before)
		total += saved
		if err != nil {
			failed++
			fmt.Printf("FAIL  %-38s saved %s: %v\n", it.Label, formatSize(saved), err)
			continue
		}
		fmt.Printf("OK    %-38s saved %s\n", it.Label, formatSize(saved))
	}
	fmt.Printf("\nSaved %s in total (free disk space changed by %s).", formatSize(total), formatSize(max(freeKB()-start, 0)))
	if failed > 0 {
		fmt.Printf(" %d failed, see above.", failed)
	}
	fmt.Println()
}
```

`install`:

```bash
#!/bin/sh
set -e
cd "$(dirname "$0")"
go build -o "$HOME/.local/bin/remedy-space-usage" .
echo "Installed $HOME/.local/bin/remedy-space-usage"
```

`.gitignore`:

```
.code-review-graph/
```

- [ ] **Step 2: Check it builds and tests pass**

```bash
chmod +x install
go vet ./... && go test -race ./... && go build -o /dev/null .
```

Expected: no vet complaints, all tests PASS.

- [ ] **Step 3: Try the whole flow on a throwaway home**

No real data is at risk: all home-based paths resolve inside the temp folder. Postgres, Redis and system log rows point at real files, and temp files use the real `$TMPDIR` unless you untick them. Step 4 below unticks everything first, so only tick the three rows it names.

```bash
go build -o /tmp/rsu . && T=$(mktemp -d) && mkdir -p "$T/.npm/x" "$T/c/web/node_modules/d" "$T/c/app/log" && dd if=/dev/zero of="$T/.npm/x/f" bs=1m count=5 && touch "$T/c/web/package.json" && dd if=/dev/zero of="$T/c/web/node_modules/d/f" bs=1m count=5 && dd if=/dev/zero of="$T/c/app/log/development.log" bs=1m count=5 && HOME="$T" /tmp/rsu; find "$T" -type f -exec ls -la {} +
```

Check by hand:
1. The list appears at once, with sizes filling in.
2. Empty items disappear when measured, `Clean` appears when the scan ends.
3. Untick npm during the scan: it stays unticked as sizes arrive.
4. `ctrl+x` to untick everything, then tick only npm, node_modules and Rails logs; `ctrl+d`.
5. Answer `n`: nothing changes. Run again, answer `y`: `.npm/x` is gone, `node_modules` is gone, `development.log` is 0 bytes, each `OK` line shows about 5.0 MB saved, and the total is about 15.0 MB.

- [ ] **Step 4: Install and commit**

```bash
./install
git add main.go install .gitignore
git commit -m "Add the picker, review screen and install script"
```

- [ ] **Step 5: Confirm the command is found**

```bash
command -v remedy-space-usage
```

Expected: `/Users/grillermo/.local/bin/remedy-space-usage`.
