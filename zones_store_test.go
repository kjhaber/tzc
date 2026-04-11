package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadExtraZoneNames(t *testing.T) {
	root := t.TempDir()
	old := zonesTestRoot
	zonesTestRoot = root
	t.Cleanup(func() { zonesTestRoot = old })

	want := []string{"America/Denver", "America/New_York"}
	if err := saveExtraZoneNames(want); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "tzc", "zones")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	got, err := loadExtraZoneNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

func TestLoadExtraZoneNames_skipsCommentsAndBlank(t *testing.T) {
	root := t.TempDir()
	old := zonesTestRoot
	zonesTestRoot = root
	t.Cleanup(func() { zonesTestRoot = old })

	path := filepath.Join(root, "tzc", "zones")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# my zones\n\nAmerica/Chicago\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadExtraZoneNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "America/Chicago" {
		t.Fatalf("got %v", got)
	}
}
