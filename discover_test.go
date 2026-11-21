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
	if got := it.location(); got != root+"/**/log" {
		t.Errorf("location must not depend on which logs exist: %q", got)
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

func TestDiscoverXcodeProjectBuildFolders(t *testing.T) {
	root := t.TempDir()
	touch(t, root+"/mac/Mac.xcodeproj/project.pbxproj")
	touch(t, root+"/mac/.build-run/DerivedData/a")
	touch(t, root+"/mac/build/DerivedData/a")
	touch(t, root+"/ios/App.xcodeproj/project.pbxproj")
	touch(t, root+"/ios/build/dsyms/a")
	touch(t, root+"/plain/build/DerivedData/a")
	touch(t, root+"/plain/.build-run/a")

	items := discover(root)
	for _, key := range []string{"artifact:" + root + "/mac/.build-run", "artifact:" + root + "/mac/build/DerivedData"} {
		it := find(items, key)
		if it == nil {
			t.Errorf("missing %s", key)
		} else if it.InUseRoot != root+"/mac" {
			t.Errorf("%s InUseRoot = %q", key, it.InUseRoot)
		}
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2 (dSYMs and folders without an Xcode project stay)", len(items))
	}
}
