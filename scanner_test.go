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
