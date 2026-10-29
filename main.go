// Command lastrack is Last Rack Standing: hold a datacenter floor against
// rogue packets, bad deploys and 3 a.m. pages, from a terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/conn3xorg/lastrack/game"
	"github.com/gdamore/tcell/v2"
)

var version = "dev"

// Screen rows: one HUD row, the playfield, the floor line, one footer row.
const minW, minH = game.W, game.H + 3

type mode int

const (
	title mode = iota
	playing
	paused
	over
)

type app struct {
	s       tcell.Screen
	g       glyphs
	mode    mode
	game    *game.Game
	demo    *game.Game
	seed    uint64
	high    int
	newHigh bool
	ox, oy  int // where the playfield's top-left lands on screen
	buttons tcell.ButtonMask
}

func main() {
	seed := flag.Uint64("seed", 0, "random seed; same seed and same input, same game (0 picks one)")
	ascii := flag.Bool("ascii", false, "plain ASCII glyphs, for fonts and terminals without Unicode")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("lastrack", version)
		return
	}
	if *seed == 0 {
		*seed = uint64(time.Now().UnixNano())
	}
	if err := run(*seed, *ascii); err != nil {
		fmt.Fprintln(os.Stderr, "lastrack:", err)
		os.Exit(1)
	}
}

func run(seed uint64, ascii bool) (err error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	defer func() {
		// Put the terminal back even if something panics mid-frame.
		if r := recover(); r != nil {
			s.Fini()
			panic(r)
		}
		s.Fini()
	}()
	s.EnableMouse()
	s.HideCursor()

	a := &app{s: s, g: unicodeGlyphs, seed: seed, high: loadHigh(), demo: game.New(seed ^ 0xdec0de)}
	if ascii {
		a.g = asciiGlyphs
	}

	events := make(chan tcell.Event, 64)
	go func() {
		for {
			ev := s.PollEvent()
			if ev == nil {
				return
			}
			events <- ev
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)

	tick := time.NewTicker(time.Second / game.TPS)
	defer tick.Stop()
	for {
		select {
		case <-sigs:
			a.finish()
			return nil
		case ev := <-events:
			if !a.handle(ev) {
				a.finish()
				return nil
			}
		case <-tick.C:
			a.step()
			a.draw()
		}
	}
}

// finish records a high score if the player quits mid-game.
func (a *app) finish() {
	if a.game != nil && a.game.Score > a.high {
		saveHigh(a.game.Score)
	}
}

func (a *app) start() {
	a.game = game.New(a.seed)
	a.seed++ // the next game in this session is a different one
	a.newHigh = false
	a.mode = playing
}

func (a *app) step() {
	switch a.mode {
	case title:
		bot(a.demo)
		a.demo.Step()
		if a.demo.Phase == game.Over {
			a.demo = game.New(uint64(a.demo.Tick))
		}
	case playing:
		a.game.Step()
		if a.game.Phase == game.Over {
			a.mode = over
			if a.game.Score > a.high {
				a.high, a.newHigh = a.game.Score, true
				saveHigh(a.high)
			}
		}
	}
}

// bot plays the attract screen: it shoots just under the lowest threat.
func bot(g *game.Game) {
	if g.Tick%9 != 0 || len(g.Threats) == 0 {
		return
	}
	low := g.Threats[0]
	for _, t := range g.Threats {
		if t.Y > low.Y {
			low = t
		}
	}
	g.SetCursor(int(low.X+low.VX*8), int(low.Y+low.VY*8))
	g.Fire(-1)
}

// handle applies one input event and reports whether to keep running.
func (a *app) handle(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		a.s.Sync()
	case *tcell.EventMouse:
		x, y := ev.Position()
		pressed := ev.Buttons()&tcell.Button1 != 0 && a.buttons&tcell.Button1 == 0
		a.buttons = ev.Buttons()
		if a.mode == playing {
			a.game.SetCursor(x-a.ox, y-a.oy)
			if pressed {
				a.game.Fire(-1)
			}
		} else if pressed && a.mode != paused {
			a.start()
		}
	case *tcell.EventKey:
		return a.key(ev)
	}
	return true
}

func (a *app) key(ev *tcell.EventKey) bool {
	k, r := ev.Key(), ev.Rune()
	if k == tcell.KeyCtrlC || k == tcell.KeyRune && (r == 'q' || r == 'Q') {
		return false
	}
	switch a.mode {
	case title, over:
		if k == tcell.KeyEnter || r == ' ' {
			a.start()
		} else if k == tcell.KeyEscape && a.mode == over {
			a.mode = title
		}
	case paused:
		if r == 'p' || r == 'P' || r == ' ' || k == tcell.KeyEscape {
			a.mode = playing
		}
	case playing:
		g := a.game
		switch {
		case k == tcell.KeyLeft || r == 'a' || r == 'h':
			g.MoveCursor(-2, 0)
		case k == tcell.KeyRight || r == 'd' || r == 'l':
			g.MoveCursor(2, 0)
		case k == tcell.KeyUp || r == 'w' || r == 'k':
			g.MoveCursor(0, -1)
		case k == tcell.KeyDown || r == 's' || r == 'j':
			g.MoveCursor(0, 1)
		case r == ' ':
			g.Fire(-1)
		case r >= '1' && r <= '3':
			g.Fire(int(r - '1'))
		case r == 'p' || r == 'P' || k == tcell.KeyEscape:
			a.mode = paused
		}
	}
	return true
}
