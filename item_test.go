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
