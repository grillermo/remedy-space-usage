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
