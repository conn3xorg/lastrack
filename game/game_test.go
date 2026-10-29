package game

import (
	"fmt"
	"hash/fnv"
	"testing"
)

// quiet returns a game with nothing left to spawn, so tests place threats by hand.
func quiet() *Game {
	g := New(1)
	g.toSpawn = 0
	return g
}

func run(g *Game, ticks int) {
	for range ticks {
		g.Step()
	}
}

func TestBlastChains(t *testing.T) {
	g := quiet()
	// A line of packets 3 columns apart: only the first is in the player's
	// blast, the rest go by cascade.
	for i := range 4 {
		g.Threats = append(g.Threats, &Threat{Kind: Packet, X: 10 + 3*float64(i), Y: 5, TX: 10, TY: RackTop})
	}
	g.Blasts = append(g.Blasts, &Blast{X: 8, Y: 5, Max: 4})
	run(g, 30)
	if len(g.Threats) != 0 {
		t.Fatalf("%d threats survived the cascade", len(g.Threats))
	}
	if g.BestChain != 4 {
		t.Errorf("best chain %d, want 4", g.BestChain)
	}
	if want := 10 * (1 + 2 + 3 + 4); g.Score < want {
		t.Errorf("score %d, want at least %d", g.Score, want)
	}
}

func TestBlastMissesOutsideRadius(t *testing.T) {
	g := quiet()
	// Two rows down is four columns of distance; outside a radius-3 blast.
	g.Threats = append(g.Threats, &Threat{Kind: Packet, X: 20, Y: 7, TX: 20, TY: RackTop, VY: 0})
	g.Blasts = append(g.Blasts, &Blast{X: 20, Y: 5, Max: 3})
	g.Threats[0].TY = 100 // never lands during the test
	run(g, 25)
	if len(g.Threats) != 1 {
		t.Fatal("threat outside the radius was scrubbed")
	}
}

func TestImpactDegradesThenDowns(t *testing.T) {
	g := quiet()
	x := float64(g.Racks[3].X)
	for range 2 {
		g.Threats = append(g.Threats, &Threat{Kind: Packet, X: x, Y: RackTop - 0.5, VY: 1, TX: x, TY: RackTop})
		g.Step()
	}
	if g.Racks[3].HP != 0 {
		t.Fatalf("rack HP %d after two hits", g.Racks[3].HP)
	}
}

func TestPageTakesRackStraightDown(t *testing.T) {
	g := quiet()
	x := float64(g.Racks[0].X)
	g.Threats = append(g.Threats, &Threat{Kind: Page, X: x, Y: RackTop - 0.1, VY: 1, TX: x, TY: RackTop})
	g.Step()
	if g.Racks[0].HP != 0 {
		t.Fatalf("rack HP %d after a page", g.Racks[0].HP)
	}
}

func TestNodeImpactReboots(t *testing.T) {
	g := quiet()
	g.Threats = append(g.Threats, &Threat{Kind: Packet, X: 40, Y: NodeY - 0.1, VY: 1, TX: 40, TY: NodeY})
	g.Step()
	if g.Nodes[1].Down == 0 {
		t.Fatal("node still up after a hit")
	}
	g.SetCursor(40, 3)
	g.Nodes[0].Cool, g.Nodes[2].Cool = 99, 99
	if g.Fire(1) {
		t.Error("fired from a node that is down")
	}
	run(g, rebootFor)
	if g.Nodes[1].Down != 0 {
		t.Error("node did not reboot")
	}
}

func TestFireNearestAndBandwidth(t *testing.T) {
	g := quiet()
	g.SetCursor(70, 3)
	if !g.Fire(-1) || g.Nodes[2].Cool == 0 {
		t.Fatal("nearest node to column 70 should be edge-3")
	}
	g.BW = 0
	if g.Fire(0) {
		t.Fatal("fired with no bandwidth")
	}
}

func TestSplitDeploy(t *testing.T) {
	g := quiet()
	g.Threats = append(g.Threats, &Threat{Kind: Deploy, X: 30, Y: 4.9, VY: 0.2, TX: 30, TY: RackTop, SplitY: 5})
	g.Step()
	n := 0
	for _, th := range g.Threats {
		if th.Kind == Fragment {
			n++
		}
	}
	if n != 3 || len(g.Threats) != 3 {
		t.Fatalf("got %d fragments of %d threats, want 3", n, len(g.Threats))
	}
}

func TestWaveProgressionAndBonus(t *testing.T) {
	g := quiet()
	g.Racks[0].HP = 1
	g.Racks[1].HP = 0
	g.Step()
	if g.Phase != Break {
		t.Fatalf("phase %v, want Break", g.Phase)
	}
	if want := (8*2 + 1) * 50; g.LastBonus != want || g.Score != want {
		t.Errorf("bonus %d score %d, want %d", g.LastBonus, g.Score, want)
	}
	run(g, breakFor)
	if g.Wave != 2 || g.Phase != Playing || g.toSpawn != 14 {
		t.Errorf("wave %d phase %v spawn %d", g.Wave, g.Phase, g.toSpawn)
	}
	if g.speed(Packet) <= New(1).speed(Packet) {
		t.Error("wave 2 is not faster than wave 1")
	}
}

func TestRebuildEveryN(t *testing.T) {
	g := quiet()
	g.Racks[4].HP = 0
	g.Racks[6].HP = 1
	g.addScore(RebuildEvery)
	if g.Racks[4].HP != RackHP || g.Racks[6].HP != 1 {
		t.Fatalf("rebuild picked wrong rack: %+v", g.Racks)
	}
	g.addScore(RebuildEvery)
	if g.Racks[6].HP != RackHP {
		t.Fatal("second rebuild missing")
	}
}

func TestGameOver(t *testing.T) {
	g := quiet()
	for i := range g.Racks {
		g.Racks[i].HP = 0
	}
	g.toSpawn = 1
	g.Step()
	if g.Phase != Over {
		t.Fatal("game not over with every rack dark")
	}
}

// fingerprint hashes the visible state of a game.
func fingerprint(g *Game) uint64 {
	h := fnv.New64a()
	fmt.Fprint(h, g.Tick, g.Wave, g.Score, g.BW, g.Phase, g.Racks, g.Nodes)
	for _, t := range g.Threats {
		fmt.Fprint(h, *t)
	}
	for _, b := range g.Blasts {
		fmt.Fprint(h, *b)
	}
	return h.Sum64()
}

// play drives a game with a fixed, dumb script of input.
func play(seed uint64) *Game {
	g := New(seed)
	for i := range 3000 {
		if i%7 == 0 && len(g.Threats) > 0 {
			t := g.Threats[0]
			g.SetCursor(int(t.X), int(t.Y)+2)
			g.Fire(-1)
		}
		g.Step()
	}
	return g
}

func TestSeededDeterminism(t *testing.T) {
	a, b := play(42), play(42)
	if fingerprint(a) != fingerprint(b) {
		t.Fatal("same seed and input gave different games")
	}
	if fingerprint(a) == fingerprint(play(43)) {
		t.Error("different seeds gave the same game")
	}
	t.Logf("scripted player: wave %d score %d racks %d chain %d", a.Wave, a.Score, a.RacksUp(), a.BestChain)
	if a.Wave < 2 {
		t.Errorf("scripted player never cleared wave 1 (score %d)", a.Score)
	}
}
