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
