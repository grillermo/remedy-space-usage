package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocationIsStableAcrossRuns(t *testing.T) {
	cases := map[string]*Item{
		"/a":      {SizePaths: []string{"/a", "/b"}, Detail: "other"},
		"/glob/*": {Location: "/glob/*", SizePaths: []string{"/a"}},
		"xcrun":   {Detail: "xcrun"},
	}
	for want, it := range cases {
		if got := it.location(); got != want {
			t.Errorf("location() = %q, want %q", got, want)
		}
	}
}

func TestMemoryRoundTripAndMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "selection.json")
	m, err := loadMemory(path)
	if err != nil || len(m) != 0 {
		t.Fatalf("missing file = %v, %v", m, err)
	}
	m["/a"] = true
	m["/b"] = false
	if err := m.save(path); err != nil {
		t.Fatal(err)
	}
	got, err := loadMemory(path)
	if err != nil || !got["/a"] {
		t.Fatalf("got %v, %v", got, err)
	}
	if v, ok := got["/b"]; !ok || v {
		t.Errorf("an explicit untick must survive: %v", got)
	}
}

func TestLoadMemoryReportsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	os.WriteFile(path, []byte("{nope"), 0o644)
	if m, err := loadMemory(path); err == nil || len(m) != 0 {
		t.Errorf("got %v, %v", m, err)
	}
}

func TestApplyOverridesDefaultsOnlyForKnownLocations(t *testing.T) {
	ticked := &Item{SizePaths: []string{"/ticked"}, Ticked: true}
	unticked := &Item{SizePaths: []string{"/unticked"}}
	fresh := &Item{SizePaths: []string{"/fresh"}, Ticked: true}
	Memory{"/ticked": false, "/unticked": true}.apply([]*Item{ticked, unticked, fresh})
	if ticked.Ticked || !unticked.Ticked || !fresh.Ticked {
		t.Errorf("ticked=%v unticked=%v fresh=%v", ticked.Ticked, unticked.Ticked, fresh.Ticked)
	}
}

func TestRememberKeepsWhatTheUserCouldNotSee(t *testing.T) {
	shown := func(key, path string) *Item {
		return &Item{Key: key, SizePaths: []string{path}, Measured: true, SizeKB: 10}
	}
	empty := &Item{Key: "empty", SizePaths: []string{"/empty"}, Measured: true}
	notDelivered := shown("late", "/late")
	s := &Scanner{sent: map[string]bool{}, items: []*Item{shown("on", "/on"), shown("off", "/off"), empty, notDelivered},
		applied: map[string]bool{"on": true, "off": true, "empty": true}}
	m := Memory{"/empty": true, "/late": true, "/off": true, "/gone": true}

	s.remember(map[string]bool{"on": true}, m)

	if !m["/on"] {
		t.Error("ticked row must be remembered")
	}
	if m["/off"] {
		t.Error("a row the user saw unticked must be remembered as unticked")
	}
	if !m["/empty"] {
		t.Error("a row hidden for being empty keeps its choice for when it regrows")
	}
	if !m["/late"] {
		t.Error("a row the picker never received keeps its choice")
	}
	if !m["/gone"] {
		t.Error("locations absent from this scan stay remembered")
	}
}
