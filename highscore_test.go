package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHighScoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if loadHigh() != 0 {
		t.Fatal("fresh state should read as zero")
	}
	saveHigh(1234)
	if got := loadHigh(); got != 1234 {
		t.Fatalf("got %d", got)
	}
	for p, want := range map[string]os.FileMode{filepath.Join(dir, "lastrack"): 0o700, scorePath(): 0o600} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s: %v, want %v", p, fi.Mode().Perm(), want)
		}
	}
}
