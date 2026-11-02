package main

import "testing"

func TestParseSkips(t *testing.T) {
	got := parseSkips("# a comment\n\n~/.cache/huggingface # needed to use phonon voices\n/opt/keep/\n", "/Users/me")
	want := []Skip{{"/Users/me/.cache/huggingface", "needed to use phonon voices"}, {"/opt/keep", ""}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestTheShippedSkipListParses(t *testing.T) {
	skips := parseSkips(skipFile, "/Users/me")
	if len(skips) == 0 || skips[0].Note == "" {
		t.Errorf("skip.txt should list folders with notes, got %+v", skips)
	}
}

func TestWithoutSkipped(t *testing.T) {
	skips := []Skip{{Path: "/h/.cache/huggingface"}}
	items := []*Item{
		{Key: "hf", SizePaths: []string{"/h/.cache/huggingface"}, RemovePaths: []string{"/h/.cache/huggingface"}},
		{Key: "hf-inside", SizePaths: []string{"/h/.cache/huggingface/hub"}, RemovePaths: []string{"/h/.cache/huggingface/hub"}},
		{Key: "parent", SizePaths: []string{"/h/.cache"}, EmptyDirs: []string{"/h/.cache"}},
		{Key: "mixed", SizePaths: []string{"/h/.cache/huggingface", "/h/.npm"}, EmptyDirs: []string{"/h/.cache/huggingface", "/h/.npm"}},
		{Key: "tool", SizePaths: []string{"/h/.cache/huggingface"}, Commands: [][]string{{"hf", "cache", "clean"}}},
		{Key: "other", SizePaths: []string{"/h/.cache/uv"}, Commands: [][]string{{"uv", "cache", "clean"}}},
		{Key: "simulators", Commands: [][]string{{"xcrun", "simctl", "delete", "unavailable"}}},
	}
	got := withoutSkipped(items, skips)
	var keys []string
	for _, it := range got {
		keys = append(keys, it.Key)
	}
	if len(got) != 3 || keys[0] != "mixed" || keys[1] != "other" || keys[2] != "simulators" {
		t.Fatalf("kept %v, want [mixed other simulators]", keys)
	}
	if m := got[0]; len(m.SizePaths) != 1 || m.SizePaths[0] != "/h/.npm" || len(m.EmptyDirs) != 1 || m.EmptyDirs[0] != "/h/.npm" {
		t.Errorf("mixed should keep only /h/.npm, got %+v", m)
	}
}
