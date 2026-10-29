package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// scorePath is $XDG_STATE_HOME/lastrack/highscore, or ~/.local/state/lastrack/highscore.
func scorePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "lastrack", "highscore")
}

func loadHigh() int {
	b, err := os.ReadFile(scorePath())
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return max(n, 0)
}

// saveHigh is best effort: a read-only home should not end anyone's game.
func saveHigh(n int) {
	p := scorePath()
	if p == "" || os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	_ = os.WriteFile(p, []byte(strconv.Itoa(n)+"\n"), 0o600)
}
