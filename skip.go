package main

import (
	_ "embed"
	"path/filepath"
	"strings"
)

//go:embed skip.txt
var skipFile string

// Skip is a folder that is never cleaned, with the reason it is kept.
type Skip struct {
	Path string
	Note string
}

// parseSkips reads skip.txt: one folder per line, "path # note". Blank lines
// and lines starting with # are ignored.
func parseSkips(text, home string) []Skip {
	var skips []Skip
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, note, _ := strings.Cut(line, " # ")
		path = strings.TrimSpace(path)
		if path == "~" || strings.HasPrefix(path, "~/") {
			path = filepath.Join(home, path[1:])
		}
		skips = append(skips, Skip{Path: filepath.Clean(path), Note: strings.TrimSpace(note)})
	}
	return skips
}

// touches reports whether cleaning path could reach a skipped folder: it is
// the folder, inside it, or contains it.
func touches(path string, skips []Skip) bool {
	path = filepath.Clean(path)
	for _, s := range skips {
		if path == s.Path || strings.HasPrefix(path, s.Path+"/") || strings.HasPrefix(s.Path, path+"/") {
			return true
		}
	}
	return false
}

// withoutSkipped drops the paths that touch a skipped folder from each item,
// and the items left with nothing to clean. An item that runs a command can't
// be narrowed, so it goes whole if any of its paths touches one.
func withoutSkipped(items []*Item, skips []Skip) []*Item {
	if len(skips) == 0 {
		return items
	}
	keep := func(paths []string) []string {
		var kept []string
		for _, p := range paths {
			if !touches(p, skips) {
				kept = append(kept, p)
			}
		}
		return kept
	}
	var out []*Item
	for _, it := range items {
		sized := keep(it.SizePaths)
		if len(sized) == len(it.SizePaths) && len(keep(it.EmptyDirs)) == len(it.EmptyDirs) &&
			len(keep(it.RemovePaths)) == len(it.RemovePaths) && len(keep(it.Truncate)) == len(it.Truncate) {
			out = append(out, it)
			continue
		}
		if len(it.Commands) > 0 {
			continue
		}
		it.SizePaths, it.EmptyDirs, it.RemovePaths, it.Truncate = sized, keep(it.EmptyDirs), keep(it.RemovePaths), keep(it.Truncate)
		if len(it.EmptyDirs)+len(it.RemovePaths)+len(it.Truncate) > 0 {
			out = append(out, it)
		}
	}
	return out
}
