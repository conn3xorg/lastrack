// Package game holds the rules of Last Rack Standing. It knows nothing about
// terminals: the UI feeds it input between fixed-size Steps and draws what it
// finds. Same seed plus same input gives the same game, tick for tick.
package game

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Playfield size in cells. The UI centres it on larger terminals.
const (
	W   = 80
	H   = 21
	TPS = 30 // ticks per second

	Floor     = H - 1 // bottom row of the racks
	RackTop   = H - 2
	NodeY     = H - 5 // edge nodes sit on masts above the floor
	NumRacks  = 10
	RackHP    = 2
	MaxBW     = 10
	bwRegen   = 20 // ticks per bandwidth unit
	cooldown  = 6  // ticks between shots from one node
	rebootFor = 10 * TPS
	shotSpeed = 2.0 // columns per tick
	breakFor  = 3 * TPS

	RebuildEvery = 5000
)

// NodeXs are the columns of the three edge nodes, each in a gap between racks.
var NodeXs = [3]int{16, 40, 64}

// Kind is a type of threat.
type Kind int

const (
	Packet   Kind = iota // straight line, cheap
	Deploy               // splits into fragments partway down
	Fragment             // what a bad deploy turns into
	Page                 // slow, homes on the nearest live rack
)

var baseScore = [...]int{Packet: 10, Deploy: 40, Fragment: 15, Page: 75}

// Points is the base value of a kind before the cascade multiplier.
func Points(k Kind) int { return baseScore[k] }

// Phase is where a game is in its life.
type Phase int

const (
	Playing Phase = iota
	Break         // between waves: bonus counted, nodes rebooted
	Over
)

type Threat struct {
	Kind         Kind
	X, Y, VX, VY float64
	TX, TY       float64 // impact point
	SplitY       float64 // deploys only
	Dead         bool
}

type Shot struct {
	X, Y, TX, TY float64
	Done         bool
}

// Blast is a scrub radius. It grows, holds, then shrinks; anything inside it
// is scrubbed and opens a blast of its own one link further down the chain.
type Blast struct {
	X, Y  float64
	Max   float64
	Age   int
	Chain int  // 0 for a blast the player fired
	Hit   bool // an impact on the floor; scrubs nothing
}

const blastGrow, blastHold, blastFade = 8, 6, 8

// R is the current radius in columns.
func (b *Blast) R() float64 {
	switch {
	case b.Age < blastGrow:
		return b.Max * float64(b.Age+1) / blastGrow
	case b.Age < blastGrow+blastHold:
		return b.Max
	default:
		left := blastGrow + blastHold + blastFade - b.Age
		return b.Max * float64(left) / blastFade
	}
}

func (b *Blast) done() bool { return b.Age >= blastGrow+blastHold+blastFade }

type Rack struct {
	X  int
	HP int // 2 up, 1 degraded, 0 dark
}

type Node struct {
	X      int
	Down   int // ticks until rebooted; 0 means up
	Cool   int
	Firing bool // fired this tick, for the UI
}

func (n *Node) Ready() bool { return n.Down == 0 && n.Cool == 0 }

type Game struct {
	rng *rand.Rand

	Tick, Wave, Score   int
	BW, bwTimer         int
	CX, CY              int // cursor
	Racks               []Rack
	Nodes               []Node
	Threats             []*Threat
	Shots               []*Shot
	Blasts              []*Blast
	Phase               Phase
	BestChain           int
	LastBonus           int
	Msg                 string // latest event line for the UI
	toSpawn, spawnTimer int
	breakTimer          int
	nextRebuild         int
}

// New starts wave 1.
func New(seed uint64) *Game {
	g := &Game{
		rng:         rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		CX:          W / 2,
		CY:          H / 2,
		nextRebuild: RebuildEvery,
	}
	for i := range NumRacks {
		g.Racks = append(g.Racks, Rack{X: (2*i + 1) * W / (2 * NumRacks), HP: RackHP})
	}
	for _, x := range NodeXs {
		g.Nodes = append(g.Nodes, Node{X: x})
	}
	g.startWave(1)
	return g
}

func (g *Game) startWave(n int) {
	g.Wave = n
	g.Phase = Playing
	g.toSpawn = 8 + 3*n
	g.spawnTimer = TPS
	g.BW = MaxBW
	for i := range g.Nodes {
		g.Nodes[i].Down, g.Nodes[i].Cool = 0, 0
	}
	g.Msg = fmt.Sprintf("wave %d: %d incoming", n, g.toSpawn)
}

// RacksUp counts racks with any HP left.
func (g *Game) RacksUp() int {
	n := 0
	for _, r := range g.Racks {
		if r.HP > 0 {
			n++
		}
	}
	return n
}

// MoveCursor nudges the cursor, clamped to the sky.
func (g *Game) MoveCursor(dx, dy int) { g.SetCursor(g.CX+dx, g.CY+dy) }

func (g *Game) SetCursor(x, y int) {
	g.CX = min(max(x, 0), W-1)
	g.CY = min(max(y, 0), NodeY-1)
}

// Fire launches from node i, or from the nearest ready node when i < 0.
// It reports whether a shot left.
func (g *Game) Fire(i int) bool {
	if g.Phase != Playing || g.BW < 1 {
		return false
	}
	if i < 0 {
		best := math.MaxInt
		for j, n := range g.Nodes {
			if d := abs(n.X - g.CX); n.Ready() && d < best {
				i, best = j, d
			}
		}
	}
	if i < 0 || i >= len(g.Nodes) || !g.Nodes[i].Ready() {
		return false
	}
	n := &g.Nodes[i]
	n.Cool, n.Firing = cooldown, true
	g.BW--
	g.Shots = append(g.Shots, &Shot{X: float64(n.X), Y: NodeY - 1, TX: float64(g.CX), TY: float64(g.CY)})
	return true
}

// dist measures in columns; a row is about two columns tall on a terminal.
func dist(ax, ay, bx, by float64) float64 { return math.Hypot(ax-bx, 2*(ay-by)) }

// Step advances one tick.
func (g *Game) Step() {
	if g.Phase == Over {
		return
	}
	g.Tick++
	for i := range g.Nodes {
		n := &g.Nodes[i]
		n.Firing = false
		n.Cool = max(n.Cool-1, 0)
		if n.Down > 0 {
			if n.Down--; n.Down == 0 {
				g.Msg = fmt.Sprintf("edge-%d back online", i+1)
			}
		}
	}
	if g.BW < MaxBW {
		if g.bwTimer++; g.bwTimer >= bwRegen {
			g.bwTimer, g.BW = 0, g.BW+1
		}
	}

	if g.Phase == Break {
		if g.breakTimer--; g.breakTimer <= 0 {
			g.startWave(g.Wave + 1)
		}
		return
	}

	g.spawn()
	g.moveShots()
	g.moveThreats()
	g.scrub()

	if g.RacksUp() == 0 {
		g.Phase = Over
		g.Msg = "all racks dark"
		return
	}
	if g.toSpawn == 0 && len(g.Threats) == 0 && len(g.Shots) == 0 && len(g.Blasts) == 0 {
		g.endWave()
	}
}

func (g *Game) spawn() {
	if g.toSpawn == 0 {
		return
	}
	if g.spawnTimer--; g.spawnTimer > 0 {
		return
	}
	g.spawnTimer = max(10, 50-4*g.Wave) + g.rng.IntN(20)
	g.toSpawn--
	k := Packet
	switch r := g.rng.IntN(100); {
	case g.Wave >= 3 && r < 5+2*g.Wave:
		k = Page
	case g.Wave >= 2 && r < 25+3*g.Wave:
		k = Deploy
	}
	t := &Threat{Kind: k, X: float64(g.rng.IntN(W)), Y: 0}
	if k == Deploy {
		t.SplitY = float64(H) * (0.3 + 0.25*g.rng.Float64())
	}
	g.aim(t)
	g.Threats = append(g.Threats, t)
}

// speed is rows per tick for this wave.
func (g *Game) speed(k Kind) float64 {
	s := min(0.06+0.012*float64(g.Wave), 0.3)
	switch k {
	case Page:
		return s * 0.5
	case Fragment:
		return s * 1.3
	}
	return s
}

// aim picks something to hit: mostly racks still standing, sometimes a node.
func (g *Game) aim(t *Threat) {
	if g.rng.IntN(5) == 0 {
		n := g.Nodes[g.rng.IntN(len(g.Nodes))]
		t.TX, t.TY = float64(n.X), NodeY
	} else {
		var up []int
		for i, r := range g.Racks {
			if r.HP > 0 {
				up = append(up, i)
			}
		}
		i := g.rng.IntN(NumRacks)
		if len(up) > 0 {
			i = up[g.rng.IntN(len(up))]
		}
		t.TX, t.TY = float64(g.Racks[i].X), RackTop
	}
	if t.Kind == Page {
		g.homeTarget(t)
	}
	g.steer(t)
}

func (g *Game) homeTarget(t *Threat) {
	best := math.MaxFloat64
	for _, r := range g.Racks {
		if d := math.Abs(float64(r.X) - t.X); r.HP > 0 && d < best {
			best, t.TX, t.TY = d, float64(r.X), RackTop
		}
	}
}

func (g *Game) steer(t *Threat) {
	dy := t.TY - t.Y
	if dy <= 0 {
		dy = 1
	}
	steps := dy / g.speed(t.Kind)
	t.VX, t.VY = (t.TX-t.X)/steps, (t.TY-t.Y)/steps
}

func (g *Game) moveShots() {
	live := g.Shots[:0]
	for _, s := range g.Shots {
		d := dist(s.X, s.Y, s.TX, s.TY)
		if d <= shotSpeed {
			g.Blasts = append(g.Blasts, &Blast{X: s.TX, Y: s.TY, Max: 4})
			continue
		}
		s.X += (s.TX - s.X) / d * shotSpeed
		s.Y += (s.TY - s.Y) / d * shotSpeed
		live = append(live, s)
	}
	g.Shots = live
}

func (g *Game) moveThreats() {
	var born []*Threat
	for _, t := range g.Threats {
		if t.Dead {
			continue
		}
		if t.Kind == Page && g.Tick%15 == 0 {
			g.homeTarget(t)
			g.steer(t)
		}
		t.X += t.VX
		t.Y += t.VY
		if t.Kind == Deploy && t.Y >= t.SplitY {
			t.Dead = true
			for range 3 {
				f := &Threat{Kind: Fragment, X: t.X, Y: t.Y}
				g.aim(f)
				born = append(born, f)
			}
			g.Msg = "bad deploy is fanning out"
			continue
		}
		if t.Y >= t.TY {
			t.Dead = true
			g.impact(t)
		}
	}
	g.Threats = append(g.Threats, born...)
	g.sweep()
}

func (g *Game) impact(t *Threat) {
	g.Blasts = append(g.Blasts, &Blast{X: t.TX, Y: t.TY, Max: 2, Hit: true})
	for i := range g.Nodes {
		if n := &g.Nodes[i]; t.TY == NodeY && float64(n.X) == t.TX {
			n.Down = rebootFor
			g.Msg = fmt.Sprintf("edge-%d dropped, rebooting", i+1)
			return
		}
	}
	for i := range g.Racks {
		if r := &g.Racks[i]; math.Abs(float64(r.X)-t.TX) < 2 && r.HP > 0 {
			r.HP--
			if t.Kind == Page {
				r.HP = 0 // nobody sleeps through a page
			}
			if r.HP == 0 {
				g.Msg = fmt.Sprintf("rack-%02d is down", i+1)
			} else {
				g.Msg = fmt.Sprintf("rack-%02d degraded", i+1)
			}
			return
		}
	}
}

// scrub ages blasts and chains through anything they touch.
func (g *Game) scrub() {
	for i := 0; i < len(g.Blasts); i++ { // grows while we walk it
		b := g.Blasts[i]
		if b.Hit {
			continue
		}
		r := b.R()
		for _, t := range g.Threats {
			if t.Dead || dist(t.X, t.Y, b.X, b.Y) > r {
				continue
			}
			t.Dead = true
			c := b.Chain + 1
			g.addScore(Points(t.Kind) * c)
			g.BestChain = max(g.BestChain, c)
			if c >= 3 {
				g.Msg = fmt.Sprintf("cascade x%d", c)
			}
			g.Blasts = append(g.Blasts, &Blast{X: t.X, Y: t.Y, Max: 3, Chain: c})
		}
	}
	live := g.Blasts[:0]
	for _, b := range g.Blasts {
		if b.Age++; !b.done() {
			live = append(live, b)
		}
	}
	g.Blasts = live
	g.sweep()
}

func (g *Game) sweep() {
	live := g.Threats[:0]
	for _, t := range g.Threats {
		if !t.Dead {
			live = append(live, t)
		}
	}
	g.Threats = live
}

func (g *Game) endWave() {
	bonus := 0
	for _, r := range g.Racks {
		bonus += r.HP * 50 * min(g.Wave, 5)
	}
	g.LastBonus = bonus
	g.addScore(bonus)
	g.Phase = Break
	g.breakTimer = breakFor
	g.Msg = fmt.Sprintf("wave %d clear, uptime bonus %d", g.Wave, bonus)
}

// addScore also pays out a rack rebuild every RebuildEvery points.
func (g *Game) addScore(n int) {
	g.Score += n
	for g.Score >= g.nextRebuild {
		g.nextRebuild += RebuildEvery
		worst := -1
		for i, r := range g.Racks {
			if r.HP < RackHP && (worst < 0 || r.HP < g.Racks[worst].HP) {
				worst = i
			}
		}
		if worst >= 0 {
			g.Racks[worst].HP = RackHP
			g.Msg = fmt.Sprintf("circuit applied: rack-%02d rebuilt", worst+1)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
