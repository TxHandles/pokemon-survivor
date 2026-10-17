package sim

import (
	"math"
	"math/rand"
)

// Read-only helpers for the client's HUD and level-up cards. They use no randomness and change no
// state, so calling them never affects the simulation (or lockstep play).

// CoolFrac is how charged slot s is (1 = ready), for the equipped bar.
func (w *World) CoolFrac(s *Slot) float64 {
	if s.Kind == "orbit" {
		return 1
	}
	c := w.cdOf(s) * w.cdM()
	if c <= 0 || c > 1e8 {
		return 1
	}
	return math.Max(0, math.Min(1, 1-s.CD/c))
}

// MovePreview is move id's damage per hit and cooldown at level lv (evolved or not), with the
// current player's items and upgrades. cd is 0 for moves without a cooldown (orbits).
func (w *World) MovePreview(id string, lv int, ev bool) (dmg, cd float64) {
	return w.MovePreviewB(id, lv, ev, "")
}

// bonusesWith: the current slot's rolled bonuses plus one more (extra may be "").
func (w *World) bonusesWith(id, extra string) map[string]int {
	b := map[string]int{}
	if s := w.slot(id); s != nil {
		for k, v := range s.Bon {
			b[k] = v
		}
	}
	if extra != "" {
		b[extra]++
	}
	return b
}

// MovePreviewB is MovePreview with the move's rolled bonuses (plus extra, a bonus about to be taken).
func (w *World) MovePreviewB(id string, lv int, ev bool, extra string) (dmg, cd float64) {
	m := w.DB.Moves[id]
	if m == nil {
		return 0, 0
	}
	q := BuildQ(m)
	evm := 1.0
	if ev {
		evm = 1.35 * (1 + .08*w.pe(m.HeldItem))
	}
	bal := 1.0
	tbl := w.DB.Balance.MoveDamage
	if ev {
		tbl = w.DB.Balance.MoveDamageEvolved
	}
	if v, ok := tbl[id]; ok {
		bal = v
	}
	bs := &Slot{Bon: w.bonusesWith(id, extra)}
	dmg = (q.F("d", 7) + q.F("dp", 3)*dsl(lv)) * evm * bal * w.dmgM() * bs.BonusDamageMul()
	if has(w.P.Types, m.Type) {
		dmg *= 1.5
	}
	if m.Shape != "orbit" {
		s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: lv, Evo: ev, Q: q, Bon: bs.Bon}
		if c := w.cdOf(s) * w.cdM(); c > 0 && c < 1e8 {
			cd = c
		}
	}
	return
}

// PE is the effective level of held item k (levels above 5 count half), for previews.
func PE(lv int) float64 {
	v := float64(lv)
	if v > 5 {
		return 5 + (v-5)*.5
	}
	return v
}

// Probe is what one use of a move produces at a given level (web probeMv): counts, pierce, sizes,
// area, dash length and status duration, with labels. Zero values are "not shown".
type Probe struct {
	N        float64
	NL       string // label for N
	Pierce   float64
	PSz, OSz float64 // projectile / orb size
	Area     float64
	AL       string // label for Area
	Dist     float64
	Du       float64
	DL       string // label for Du
}

// ProbeMove fires move id once in a scratch copy of the world against dummy targets and measures
// it. It does not touch the real world: the copy has its own effect lists, enemies, player copy and
// random generator, so lockstep worlds stay identical.
func (w *World) ProbeMove(id string, lv int, ev bool) (r Probe) {
	return w.ProbeMoveB(id, lv, ev, "")
}

// ProbeMoveB is ProbeMove with the move's rolled bonuses (plus extra, a bonus about to be taken).
func (w *World) ProbeMoveB(id string, lv int, ev bool, extra string) (r Probe) {
	m := w.DB.Moves[id]
	if m == nil || w.P == nil {
		return
	}
	q := BuildQ(m)
	sl := 1 + float64(lv-1)*.45
	cap_ := ev && lv >= 20
	bs := &Slot{Bon: w.bonusesWith(id, extra)}
	switch m.Shape {
	case "orbit":
		r.N = q.F("n0", 1) + math.Round(sl) + float64(w.gvBase()+b2i(ev)+b2i(cap_)+bs.BonusCount())
		r.NL = "Orbs"
		r.Area = (q.F("rr", 55) + q.F("rrp", 5)*sl) * (1 + .15*w.pe("focus")) * (1 + w.PS.F("rng")) * bs.BonusAreaMul()
		r.AL = "Orbit radius"
		r.OSz = 18 * (1 + .2*w.pe("wide")) * b2f(ev, 1.3, 1) * (1 + .05*float64(lv-1)) * bs.BonusSizeMul()
		return
	case "screen":
		r.N = 1 + math.Round(sl) + float64(w.gvBase()+b2i(ev)+b2i(cap_)+bs.BonusCount())
		r.NL = "Shield charges"
		return
	}
	if probeGo(w, &r, m.Shape, q, lv, sl, ev, bs) {
		return
	}
	defer func() { _ = recover() }() // a probe must never take the game down
	cp := *w
	pc := *w.P
	cp.P = &pc
	cp.rng = rand.New(rand.NewSource(int64(lv)*7919 + 17))
	cp.PR, cp.FX, cp.ZN, cp.DL, cp.EB, cp.HZ, cp.Events = nil, nil, nil, nil, nil, nil, nil
	cp.Items, cp.Gems = nil, nil
	cp.Decoys, cp.Cracks = nil, nil
	cp.DMG = map[string]float64{}
	cp.Orbs = nil
	hits := 0
	cp.probeHits = &hits
	cp.E = nil
	for i := 0; i < 14; i++ {
		a := float64(i) / 14 * 6.283
		d := 22 + float64(i%5)*40
		cp.E = append(cp.E, &Enemy{X: pc.X + math.Cos(a)*d, Y: pc.Y + math.Sin(a)*d, R: 10, HP: 1e9, MHP: 1e9, Type: "normal"})
	}
	s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: lv, Evo: ev, Q: q, Owner: w.CUR, Bon: bs.Bon}
	cp.curSlot = nil
	cp.fire(s)
	for i := 0; i < 240 && cp.P.Dash != nil; i++ { // dash moves hit along the dash
		cp.dashStep(1.0 / 60)
	}
	for g := 0; len(cp.DL) > 0 && g < 300; g++ {
		bi := 0
		for i, d := range cp.DL {
			if d.t < cp.DL[bi].t {
				bi = i
			}
		}
		d := cp.DL[bi]
		cp.DL = append(cp.DL[:bi], cp.DL[bi+1:]...)
		d.fn()
	}
	for _, f := range cp.FX {
		if f.Du > 0 {
			r.Du = f.Du
			r.DL = "Slow duration"
			if f.Sk == "su" {
				r.DL = "Stun duration"
			}
			break
		}
	}
	k := m.Shape
	count := func(kind string) (n int, maxW, maxL float64) {
		for _, f := range cp.FX {
			if f.Kind == kind {
				n++
				maxW, maxL = math.Max(maxW, f.Wd), math.Max(maxL, f.Ln)
			}
		}
		return
	}
	switch {
	case k == "aura":
		r.Area, r.AL = cp.auraR(s), "Radius"
	case len(cp.PR) > 0:
		r.N, r.NL = float64(len(cp.PR)), "Projectiles"
		if k == "boom" {
			r.NL = "Boomerangs"
		}
		p := cp.PR[0]
		if p.P > 0 && p.P < 90 {
			r.Pierce = float64(p.P)
		}
		r.PSz = p.R * 2
	case k == "beam" || k == "dash":
		n, mw, ml := count("beam")
		if k == "beam" {
			r.N, r.NL = float64(n), "Beams"
		}
		r.Area, r.AL = mw, "Width"
		if k == "dash" {
			r.Dist = ml
		}
	case k == "toss":
		for _, e := range cp.E {
			if e.Toss != nil {
				r.N++
				r.PSz = e.Toss.lr * 2
			}
		}
		r.NL = "Foes thrown"
		r.Area, r.AL = (q.F("r", 150)+q.F("rp", 9)*(1+float64(lv-1)*.45))*cp.rgm()*s.BonusAreaMul(), "Grab range"
	case k == "chain":
		r.N, r.NL = float64(hits), "Chain jumps"
	default:
		sk, _, _ := count("sky")
		te, _, _ := count("tele")
		bo, _, _ := count("bolt")
		switch {
		case sk > 0:
			r.N, r.NL = float64(sk), "Strikes"
		case te > 0 && k == "erupt":
			r.N, r.NL = float64(te), "Eruptions"
		case len(cp.ZN) > 0 && k == "whirl":
			r.N, r.NL = float64(len(cp.ZN)), "Whirlpools"
		case k == "jab" && bo > 0:
			r.N, r.NL = float64(bo), "Targets"
		}
		ar := 0.0
		for _, f := range cp.FX {
			v := f.MR
			if v == 0 {
				v = f.R
			}
			if v > 12 {
				ar = math.Max(ar, v)
			}
		}
		for _, z := range cp.ZN {
			if z.R > 12 {
				ar = math.Max(ar, z.R)
			}
		}
		if ar > 0 {
			r.Area, r.AL = ar, "Area"
			if k == "arc" {
				r.AL = "Reach"
			} else if k == "trail" {
				r.AL = "Zone size"
			}
		}
	}
	return
}

// TestFire (screenshots): the current player uses move id once, then the world runs for `after`
// seconds so its effect is mid-animation.
func (w *World) TestFire(id string, lv int, after float64) {
	m := w.DB.Moves[id]
	if m == nil {
		return
	}
	s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: lv, Q: BuildQ(m), Owner: w.CUR}
	w.fire(s)
	for t := 0.0; t < after; t += 1.0 / 60 {
		w.stepFX(1.0 / 60)
		for _, e := range w.E {
			if e.Toss != nil && !e.Dead {
				w.stepToss(e, 1.0/60)
			}
		}
		if w.P.Fly != nil && w.P.Fly.T > 1.0/60 {
			w.P.Fly.T -= 1.0 / 60
		}
		if w.P.Roll != nil {
			w.stepRoll(1.0/60, 0, 0)
		}
		for i := 0; i < len(w.DL); i++ {
			if d := w.DL[i]; d.t-1.0/60 <= 0 {
				w.DL = append(w.DL[:i], w.DL[i+1:]...)
				d.fn()
				i--
			} else {
				d.t -= 1.0 / 60
			}
		}
	}
}

// TestMud (screenshots): drops a few ground-hit mud puddles around the current player.
func (w *World) TestMud() {
	for i := 0; i < 4; i++ {
		a := float64(i) * 1.7
		w.ZN = append(w.ZN, &Zone{X: w.P.X + math.Cos(a)*(70+float64(i)*25), Y: w.P.Y + math.Sin(a)*(60+float64(i)*20), R: 30, L: 1.2 - float64(i)*.2, Life0: 1.2, Tk: 9, Type: "ground", D: 1, Mud: true})
	}
}

// TestBoss (screenshots): spawns the biome boss near the player, makes it use attack k, and runs
// the world for `after` seconds.
func (w *World) TestBoss(k string, after float64) {
	var b *Enemy
	for _, e := range w.E {
		if e.Boss && !e.Dead {
			b = e
		}
	}
	if b == nil {
		w.spawnBoss()
		b = w.E[len(w.E)-1]
	}
	b.X, b.Y = w.P.X+170, w.P.Y-40
	b.Ph = 2
	w.useP(0)
	if !w.bossMove(b, k) {
		w.curHz = k
		switch k {
		case "slam":
			w.hz(w.P.X, w.P.Y, 78, 1.2, 0, b.Type, "")
		case "charge":
			b.K, b.wu, b.ca = "charge", .8, math.Atan2(w.P.Y-b.Y, w.P.X-b.X)
		}
		w.curHz = ""
	}
	inv := w.P.Inv
	for t := 0.0; t < after; t += 1.0 / 60 {
		w.P.Inv = 9
		w.Step(1.0/60, Input{})
	}
	w.P.Inv = inv
}

// TestRouteState (screenshots): sets the route map's votes, and optionally the chosen destination
// with travel time left.
func (w *World) TestRouteState(votes []int, goTo int, left float64) {
	if w.State != StateRoute {
		return
	}
	copy(w.RouteVotes, votes)
	w.RouteGo, w.RouteT = goTo, left
}
