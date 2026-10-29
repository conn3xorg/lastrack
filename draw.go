package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/conn3xorg/lastrack/game"
	"github.com/gdamore/tcell/v2"
)

type glyphs struct {
	threat                   [4]rune // indexed by game.Kind
	tail, shot, mast         rune
	node                     string
	rack, rackDark           string
	led                      rune
	blastIn, blastOut, spark rune
	bwOn, bwOff              rune
	floor                    rune
}

var unicodeGlyphs = glyphs{
	threat: [4]rune{'◆', '▼', '▾', '!'},
	tail:   '·', shot: '•', mast: '│',
	node: "◢■◣", rack: "▐▪▌", rackDark: "▐ ▌", led: '•',
	blastIn: '▒', blastOut: '░', spark: '✶',
	bwOn: '▮', bwOff: '▯', floor: '▀',
}

var asciiGlyphs = glyphs{
	threat: [4]rune{'*', 'V', 'v', '!'},
	tail:   '.', shot: 'o', mast: '|',
	node: "/#\\", rack: "[=]", rackDark: "[ ]", led: ':',
	blastIn: '#', blastOut: '+', spark: 'x',
	bwOn: '#', bwOff: '-', floor: '=',
}

// Only the 16 basic colours, so 8- and 16-colour terminals look right too.
var (
	stDefault = tcell.StyleDefault
	stDim     = stDefault.Foreground(tcell.ColorGray)
	stHUD     = stDefault.Foreground(tcell.ColorSilver)
	stBright  = stDefault.Foreground(tcell.ColorWhite).Bold(true)
	stTitle   = stDefault.Foreground(tcell.ColorAqua).Bold(true)
	stOK      = stDefault.Foreground(tcell.ColorLime)
	stWarn    = stDefault.Foreground(tcell.ColorYellow)
	stBad     = stDefault.Foreground(tcell.ColorRed)
	stThreat  = [4]tcell.Style{
		stDefault.Foreground(tcell.ColorRed).Bold(true),
		stDefault.Foreground(tcell.ColorFuchsia).Bold(true),
		stDefault.Foreground(tcell.ColorPurple),
		stDefault.Foreground(tcell.ColorYellow).Bold(true),
	}
	stBlast = []tcell.Style{
		stDefault.Foreground(tcell.ColorAqua),
		stDefault.Foreground(tcell.ColorTeal),
		stDefault.Foreground(tcell.ColorLime),
		stDefault.Foreground(tcell.ColorGreen),
	}
)

func (a *app) put(x, y int, r rune, st tcell.Style) {
	a.s.SetContent(a.ox+x, a.oy+y, r, nil, st)
}

func (a *app) text(x, y int, s string, st tcell.Style) {
	for _, r := range s {
		a.put(x, y, r, st)
		x++
	}
}

// centre writes s centred on the playfield row y.
func (a *app) centre(y int, s string, st tcell.Style) {
	a.text((game.W-len([]rune(s)))/2, y, s, st)
}

func (a *app) draw() {
	a.s.Clear()
	sw, sh := a.s.Size()
	if sw < minW || sh < minH {
		msg := fmt.Sprintf("Last Rack Standing needs %dx%d; this terminal is %dx%d.", minW, minH, sw, sh)
		a.ox, a.oy = 0, 0
		a.text(max((sw-len(msg))/2, 0), sh/2, msg, stWarn)
		a.text(max((sw-14)/2, 0), sh/2+1, "q to log off", stDim)
		a.s.Show()
		return
	}
	// Row 0 of the playfield is screen row oy; the HUD sits just above it.
	a.ox, a.oy = (sw-minW)/2, (sh-minH)/2+1

	g := a.game
	if a.mode == title {
		g = a.demo
	}
	a.field(g)
	switch a.mode {
	case title:
		a.box(a.titleLines())
	case paused:
		a.box([]line{{"PAUSED", stBright}, {"", stDim}, {"The pager can wait. p to carry on.", stHUD}})
	case over:
		a.box(a.overLines())
	default:
		a.hud(g)
		a.cursor(g)
	}
	a.s.Show()
}

func (a *app) field(g *game.Game) {
	gl := &a.g
	for x := range game.W {
		a.put(x, game.H, gl.floor, stDim)
	}
	for _, b := range g.Blasts {
		r := b.R()
		for y := int(b.Y - r/2 - 1); y <= int(b.Y+r/2+1); y++ {
			for x := int(b.X - r - 1); x <= int(b.X+r+1); x++ {
				d := math.Hypot(float64(x)-b.X, 2*(float64(y)-b.Y))
				if d > r || x < 0 || x >= game.W || y < 0 || y >= game.H {
					continue
				}
				switch {
				case b.Hit:
					a.put(x, y, gl.spark, stBad)
				case d < r*0.55:
					a.put(x, y, gl.blastIn, stBlast[min(b.Chain, 3)])
				default:
					a.put(x, y, gl.blastOut, stBlast[min(b.Chain, 3)])
				}
			}
		}
	}
	for i, r := range g.Racks {
		st, art := stOK, gl.rack
		switch r.HP {
		case 1:
			st = stWarn
		case 0:
			st, art = stDim, gl.rackDark
		}
		a.text(r.X-1, game.RackTop, art, st)
		a.text(r.X-1, game.Floor, art, st)
		if r.HP > 0 && (g.Tick/6+i)%4 != 0 { // blinkenlights
			a.put(r.X, game.Floor, gl.led, st)
		}
	}
	for _, n := range g.Nodes {
		st := stHUD
		switch {
		case n.Down > 0:
			st = stBad
		case n.Firing:
			st = stBright
		}
		a.text(n.X-1, game.NodeY, gl.node, st)
		for y := game.NodeY + 1; y < game.H; y++ {
			a.put(n.X, y, gl.mast, stDim)
		}
	}
	for _, t := range g.Threats {
		for k := 3.0; k >= 1; k-- {
			a.put(int(t.X-t.VX*k*3), int(t.Y-t.VY*k*3), gl.tail, stDim)
		}
		st := stThreat[t.Kind]
		if t.Kind == game.Page && g.Tick/5%2 == 0 {
			st = stWarn
		}
		a.put(int(t.X), int(t.Y), gl.threat[t.Kind], st)
	}
	for _, s := range g.Shots {
		a.put(int(s.X), int(s.Y), gl.shot, stBright)
	}
}

func (a *app) cursor(g *game.Game) {
	a.put(g.CX-1, g.CY, '[', stBright)
	a.put(g.CX, g.CY, '+', stBright)
	a.put(g.CX+1, g.CY, ']', stBright)
}

func (a *app) hud(g *game.Game) {
	var bw strings.Builder
	for i := range game.MaxBW {
		if i < g.BW {
			bw.WriteRune(a.g.bwOn)
		} else {
			bw.WriteRune(a.g.bwOff)
		}
	}
	a.text(0, -1, fmt.Sprintf("wave %-3d score %07d  hi %07d", g.Wave, g.Score, max(a.high, g.Score)), stHUD)
	a.text(52, -1, "bandwidth "+bw.String(), stHUD)
	nodes := ""
	for i, n := range g.Nodes {
		if n.Down > 0 {
			nodes += fmt.Sprintf(" edge-%d %ds", i+1, n.Down/game.TPS+1)
		}
	}
	a.text(0, game.H+1, g.Msg+nodes, stWarn)
	hint := "space/1/2/3 fire  p pause  q quit"
	a.text(game.W-len(hint), game.H+1, hint, stDim)
}

type line struct {
	s  string
	st tcell.Style
}

// box draws lines centred in a framed panel over the playfield.
func (a *app) box(lines []line) {
	w := 0
	for _, l := range lines {
		w = max(w, len([]rune(l.s)))
	}
	w += 4
	x0, y0 := (game.W-w)/2, (game.H-len(lines)-2)/2
	for y := y0; y < y0+len(lines)+2; y++ {
		for x := x0; x < x0+w; x++ {
			r := ' '
			switch {
			case y == y0 || y == y0+len(lines)+1:
				r = '-'
			case x == x0 || x == x0+w-1:
				r = '|'
			}
			a.put(x, y, r, stDim)
		}
	}
	for i, l := range lines {
		a.centre(y0+1+i, l.s, l.st)
	}
}

func (a *app) titleLines() []line {
	gl := &a.g
	return []line{
		{"L A S T   R A C K   S T A N D I N G", stTitle},
		{"an after-hours game from the Conn3x crew", stDim},
		{"", stDim},
		{fmt.Sprintf("%c rogue packet %d   %c bad deploy %d, splits   %c 3 a.m. page %d, homes in",
			gl.threat[game.Packet], game.Points(game.Packet),
			gl.threat[game.Deploy], game.Points(game.Deploy),
			gl.threat[game.Page], game.Points(game.Page)), stHUD},
		{"Scrub blasts cascade: each link down the chain multiplies the points.", stHUD},
		{fmt.Sprintf("Racks still up pay a bonus. Every %d points, a circuit rebuilds one.", game.RebuildEvery), stHUD},
		{"Fire costs bandwidth. It comes back if you stop spending it.", stHUD},
		{"", stDim},
		{"move: arrows / wasd / hjkl / mouse    fire: space, 1 2 3, click    p pause", stDim},
		{fmt.Sprintf("high score %d", a.high), stWarn},
		{"", stDim},
		{"press space to clock in        q to log off", stBright},
		{"", stDim},
		{"For the cabinet builders and the terminal hackers,", stDim},
		{"who made a grid of characters feel like somewhere to be. Thank you.", stDim},
	}
}

var sendoffs = []string{
	"The postmortem can wait until morning.",
	"Nothing here that a fresh circuit won't fix.",
	"Every rack you saved was somebody's quiet night.",
	"Go get some sleep. The pager's off now.",
}

func (a *app) overLines() []line {
	g := a.game
	rec := line{"", stDim}
	if a.newHigh {
		rec = line{"New high score. Somebody write that down.", stOK}
	}
	return []line{
		{"ALL RACKS DARK", stBad},
		{"", stDim},
		{fmt.Sprintf("You held the floor to wave %d. Score %d.", g.Wave, g.Score), stBright},
		{fmt.Sprintf("Longest cascade: x%d.", g.BestChain), stHUD},
		rec,
		{sendoffs[g.Tick%len(sendoffs)], stHUD},
		{"", stDim},
		{"space for another shift    esc for the lobby    q to log off", stDim},
	}
}
