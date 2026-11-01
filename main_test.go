package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDryRunReportTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir+"/cache/f")
	items := []*Item{
		{Label: "Cache", EmptyDirs: []string{dir + "/cache"}, SizePaths: []string{dir + "/cache"}, Measured: true, SizeKB: 2048},
		{Label: "Marker", Commands: [][]string{{"touch", dir + "/ran"}}},
		{Label: "Slack", App: "Slack", SizePaths: []string{"/x"}, Measured: true, SizeKB: 4096},
	}
	var out bytes.Buffer
	dryRunReport(&out, items, guards{running: func(app string) bool { return app == "Slack" }})

	if !exists(dir+"/cache/f") || exists(dir+"/ran") {
		t.Error("a dry run must not delete or run anything")
	}
	got := out.String()
	for _, want := range []string{"delete contents of " + dir + "/cache", "run touch", "SKIP  Slack: Slack is running", "nothing was deleted", "up to 2.0 MB"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
