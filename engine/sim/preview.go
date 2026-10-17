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
	m := w.DB.Moves[id]
	if m == nil {
		return 0, 0
	}
	q := BuildQ(m)
	sl := 1 + float64(lv-1)*.45
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
	dmg = (q.F("d", 7) + q.F("dp", 3)*sl) * evm * bal * w.dmgM()
	if has(w.P.Types, m.Type) {
		dmg *= 1.5
	}
	if m.Shape != "orbit" {
		s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: lv, Evo: ev, Q: q}
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
	m := w.DB.Moves[id]
	if m == nil || w.P == nil {
		return
	}
	q := BuildQ(m)
	sl := 1 + float64(lv-1)*.45
	cap_ := ev && lv >= 20
	switch m.Shape {
	case "orbit":
		r.N = q.F("n0", 1) + math.Round(sl) + float64(w.gv()+b2i(ev)+b2i(cap_))
		r.NL = "Orbs"
		r.Area = (q.F("rr", 55) + q.F("rrp", 5)*sl) * w.rgm()
		r.AL = "Orbit radius"
		r.OSz = 18 * (1 + .2*w.pe("wide")) * b2f(ev, 1.3, 1) * (1 + .05*float64(lv-1))
		return
	case "screen":
		r.N = 1 + math.Round(sl) + float64(w.gv()+b2i(ev)+b2i(cap_))
		r.NL = "Shield charges"
		return
	}
	defer func() { _ = recover() }() // a probe must never take the game down
	cp := *w
	pc := *w.P
	cp.P = &pc
	cp.rng = rand.New(rand.NewSource(int64(lv)*7919 + 17))
	cp.PR, cp.FX, cp.ZN, cp.DL, cp.EB, cp.HZ, cp.Events = nil, nil, nil, nil, nil, nil, nil
	cp.Items, cp.Gems = nil, nil
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
	s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: lv, Evo: ev, Q: q, Owner: w.CUR}
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
