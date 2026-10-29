package main

import (
	"strings"
	"testing"

	"github.com/conn3xorg/lastrack/game"
	"github.com/gdamore/tcell/v2"
)

func screenText(s tcell.SimulationScreen) string {
	cells, w, h := s.GetContents()
	var b strings.Builder
	for y := range h {
		for x := range w {
			if r := cells[y*w+x].Runes; len(r) > 0 {
				b.WriteRune(r[0])
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// TestDrawEveryMode renders each screen at the minimum size, in both glyph sets.
func TestDrawEveryMode(t *testing.T) {
	for _, gl := range []glyphs{unicodeGlyphs, asciiGlyphs} {
		s := tcell.NewSimulationScreen("UTF-8")
		if err := s.Init(); err != nil {
			t.Fatal(err)
		}
		s.SetSize(minW, minH)
		a := &app{s: s, g: gl, demo: game.New(1)}
		for m, want := range map[mode]string{title: "R A C K", playing: "p pause  q quit", paused: "PAUSED", over: "ALL RACKS DARK"} {
			a.game = game.New(2)
			a.mode = m
			a.draw()
			if a.ox < 0 {
				t.Fatalf("mode %d: playfield off screen", m)
			}
			if txt := screenText(s); !strings.Contains(txt, want) {
				t.Errorf("mode %d: %q missing from\n%s", m, want, txt)
			}
		}
		s.SetSize(60, 20)
		a.draw()
		if !strings.Contains(screenText(s), "needs 80x24") {
			t.Error("no size warning on a small terminal")
		}
		s.Fini()
	}
}

// TestPanelsFit keeps every panel line, plus its frame, inside 80 columns.
func TestPanelsFit(t *testing.T) {
	a := &app{g: unicodeGlyphs, game: game.New(1), high: 9999999, newHigh: true}
	for _, l := range append(a.titleLines(), a.overLines()...) {
		if n := len([]rune(l.s)) + 4; n > game.W {
			t.Errorf("%d columns: %q", n, l.s)
		}
	}
}
