package sim

import "math"

// Six more Go-only attack types for moves that the basic archetypes didn't capture:
//   charge  — glow and charge, then one huge blast (Focus Punch, Solar Beam, Skull Bash…)
//   counter — stores the damage you take and fires it back as a shockwave (Revenge, Payback…)
//   bind    — pins the toughest nearby foe in place and squeezes it (Bind, Wrap, Magma Storm…)
//   ramp    — hits grow with each consecutive landed use, or with the run's KOs (Rage, Stored Power…)
//   sac     — a huge explosion that costs some of your HP (Self-Destruct, Explosion…)
//   vanish  — vanish (untouchable) for a moment, keep moving, then burst up (Dive, Dig, Phantom Force…)

// Bind: a foe held by a binding move.
type Bind struct {
	E     *Enemy
	T, Du float64
	D, tk float64
	Type  string
	Slot  *Slot
	Owner int
}

func init() {
	QD["charge"] = map[string]any{"cd": 5.0, "du": .9, "r": 85.0, "rp": 5.0, "d": 62.0, "dp": 24.0}
	QD["counter"] = map[string]any{"cd": 3.4, "r": 110.0, "rp": 6.0, "d": 12.0, "dp": 5.0, "k": 2.6}
	QD["bind"] = map[string]any{"cd": 6.0, "du": 3.0, "n0": 1.0, "d": 8.0, "dp": 3.4}
	QD["ramp"] = map[string]any{"cd": 1.6, "r": 50.0, "rp": 3.0, "d": 18.0, "dp": 7.5, "mx": 10.0, "rk": .15}
	QD["sac"] = map[string]any{"cd": 7.0, "r": 160.0, "rp": 8.0, "d": 85.0, "dp": 32.0, "hpc": .12}
	QD["vanish"] = map[string]any{"cd": 5.5, "du": .7, "r": 95.0, "rp": 4.0, "d": 42.0, "dp": 16.0}
	for _, k := range []string{"charge", "counter", "bind", "ramp", "sac", "vanish"} {
		Implemented[k] = true
	}
	dmg := func(c mc) float64 { return (c.q.F("d", 0) + c.q.F("dp", 0)*c.sld) * c.em }
	area := func(w *World, c mc, r, rp float64) float64 {
		return (c.q.F("r", r) + c.q.F("rp", rp)*c.sl) * w.rgm() * c.s.BonusSizeMul() * b2f(c.ev, 1.25, 1)
	}

	shapes["charge"] = func(w *World, c mc) {
		P := w.P
		du := c.q.F("du", .9) * b2f(c.ev, .8, 1)
		R := area(w, c, 85, 5)
		d := dmg(c)
		P.Chg, P.ChgDu, P.ChgType = du, du, c.t
		w.addFX(&FX{Kind: "chg", X: P.X, Y: P.Y, R: R, Life: du, Life0: du, Type: c.t, Slot: c.s})
		w.at(c, du, func() {
			P := w.P
			if P.Dead {
				return
			}
			a := w.faceA()
			x, y := P.X+math.Cos(a)*70, P.Y+math.Sin(a)*70
			if e := w.near1(P.X, P.Y, 300); e != nil { // aim at whatever is closest when it lets go
				x, y = e.X, e.Y
			}
			w.Shake = math.Max(w.Shake, .35)
			w.addFX(&FX{Kind: "chgx", X: x, Y: y, AX: P.X, AY: P.Y, R: R, Life: .5, Life0: .5, Type: c.t, Slot: c.s})
			w.addFX(&FX{Kind: "ring", X: x, Y: y, R: 10, MR: R, Fat: true, Type: c.t, D: d, Hit: map[*Enemy]bool{}, Life: .3, KB: 3, Slot: c.s})
			if c.cap {
				w.addFX(&FX{Kind: "ring", X: x, Y: y, R: 10, MR: R * 1.6, TW: 14, Type: c.t, D: d * .4, Hit: map[*Enemy]bool{}, Life: .45, KB: 2, Slot: c.s})
			}
		})
	}

	shapes["counter"] = func(w *World, c mc) {
		P := w.P
		st := math.Min(P.Ctr, P.MHP) // stored damage, capped at a full health bar
		fill := st / math.Max(1, P.MHP)
		R := area(w, c, 110, 6) * (1 + .6*fill)
		d := dmg(c) + st*c.q.F("k", 2.6)*c.em
		P.Ctr = 0
		if fill > .25 {
			w.Shake = math.Max(w.Shake, .15+.3*fill)
		}
		w.addFX(&FX{Kind: "ctr", X: P.X, Y: P.Y, R: R, D: fill, Life: .45, Life0: .45, Type: c.t, Slot: c.s})
		w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 10, MR: R, Fat: true, Type: c.t, D: d, Hit: map[*Enemy]bool{}, Life: .32, KB: 2, Slot: c.s})
	}

	shapes["bind"] = func(w *World, c mc) {
		P := w.P
		n := int(c.q.F("n0", 1)) + w.gv() + b2i(c.ev) + b2i(c.cap)
		du := (c.q.F("du", 3) + .1*float64(c.lv-1)) * c.s.BonusDurMul()
		d := dmg(c)
		// toughest first: bosses, then elites, then the healthiest foe in reach
		var picks []*Enemy
		for len(picks) < n {
			var best *Enemy
			bk := -1.0
			for _, e := range w.E {
				if e.Dead || e.Ug || inList(picks, e) || w.bound(e) || dist(P.X, P.Y, e.X, e.Y) > 320 {
					continue
				}
				k := e.HP + b2f(e.Boss, 1e6, 0) + b2f(e.Elite, 1e5, 0)
				if k > bk {
					bk, best = k, e
				}
			}
			if best == nil {
				break
			}
			picks = append(picks, best)
		}
		for _, e := range picks {
			w.Binds = append(w.Binds, &Bind{E: e, T: du * b2f(e.Boss, .5, 1), Du: du, D: d, Type: c.t, Slot: c.s, Owner: w.CUR})
			e.Root = math.Max(e.Root, .12)
			w.hit(e, d*2, c.t, 0, 0, 0) // the grab itself
			w.addFX(&FX{Kind: "puff", X: e.X, Y: e.Y, R: e.R * 2, Life: .3, Type: c.t, Slot: c.s})
		}
	}

	shapes["ramp"] = func(w *World, c mc) {
		P := w.P
		s := c.s
		a := w.faceA()
		x, y := P.X+math.Cos(a)*60, P.Y+math.Sin(a)*60
		if e := w.near1(P.X, P.Y, 280); e != nil {
			x, y = e.X, e.Y
		}
		var mul float64
		if c.q.F("kos", 0) > 0 { // grows with the run's knockouts
			mul = 1 + math.Min(2.5, float64(w.Kills)/400)
		} else {
			mul = 1 + c.q.F("rk", .15)*float64(s.rampN)
		}
		R := area(w, c, 46, 3) * (1 + .04*float64(s.rampN))
		d := dmg(c) * mul
		hits := 0
		for _, e := range w.E {
			if e.Dead || e.Ug || dist(x, y, e.X, e.Y) > R+e.R {
				continue
			}
			w.hit(e, d, c.t, 1, x, y)
			hits++
		}
		if c.q.F("kos", 0) == 0 {
			if hits > 0 {
				s.rampN = min(s.rampN+1, int(c.q.F("mx", 10)))
			} else {
				s.rampN = 0
			}
		}
		w.addFX(&FX{Kind: "ramp", X: x, Y: y, R: R, D: mul, Life: .4, Life0: .4, Type: c.t, Slot: c.s})
	}

	shapes["sac"] = func(w *World, c mc) {
		P := w.P
		R := area(w, c, 160, 8)
		d := dmg(c)
		hpc := c.q.F("hpc", .12)
		const wind = .35
		w.addFX(&FX{Kind: "sac", X: P.X, Y: P.Y, R: R, Life: wind, Life0: wind, Type: c.t, Slot: c.s})
		w.at(c, wind, func() {
			P := w.P
			if P.Dead {
				return
			}
			if !w.Run.NoHeal || P.HP > P.MHP*hpc+1 {
				P.HP = math.Max(1, P.HP-P.MHP*hpc) // never knocks you out
			}
			P.HurtT = w.T
			w.Shake = math.Max(w.Shake, .6)
			w.addFX(&FX{Kind: "sacx", X: P.X, Y: P.Y, R: R, Life: .6, Life0: .6, Type: c.t, Slot: c.s})
			w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 14, MR: R, Fat: true, Type: c.t, D: d, Hit: map[*Enemy]bool{}, Life: .35, KB: 3, Slot: c.s})
		})
	}

	shapes["vanish"] = func(w *World, c mc) {
		P := w.P
		du := c.q.F("du", .7) + .02*float64(c.lv-1)
		R := area(w, c, 95, 4)
		d := dmg(c)
		P.Van, P.VanDu, P.VanType = du, du, c.t
		P.Inv = math.Max(P.Inv, du)
		w.addFX(&FX{Kind: "puff", X: P.X, Y: P.Y + 6, R: 30, Life: .35, Type: c.t, Slot: c.s})
		w.at(c, du, func() {
			P := w.P
			if P.Dead {
				return
			}
			P.Van = 0
			w.Shake = math.Max(w.Shake, .3)
			w.addFX(&FX{Kind: "vanx", X: P.X, Y: P.Y, R: R, Life: .5, Life0: .5, Type: c.t, Slot: c.s})
			w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 10, MR: R, Fat: true, Type: c.t, D: d, Hit: map[*Enemy]bool{}, Life: .3, KB: 3, Slot: c.s})
		})
	}
}

func (w *World) bound(e *Enemy) bool {
	for _, b := range w.Binds {
		if b.E == e {
			return true
		}
	}
	return false
}

// stepBinds: held foes can't move and take a squeeze every .3s.
func (w *World) stepBinds(dt float64) {
	keep := w.Binds[:0]
	for _, b := range w.Binds {
		b.T -= dt
		if b.E.Dead || b.T <= 0 {
			continue
		}
		b.E.Root = math.Max(b.E.Root, .12)
		b.tk -= dt
		if b.tk <= 0 {
			b.tk = .3
			w.useP(b.Owner)
			prev := w.curSlot
			w.curSlot = b.Slot
			w.hit(b.E, b.D, b.Type, 0, 0, 0)
			w.curSlot = prev
		}
		keep = append(keep, b)
	}
	w.Binds = keep
}

// stepPlayerGo2: per-player timers for charge, counter and vanish.
func (w *World) stepPlayerGo2(dt float64) {
	P := w.P
	if P.Chg > 0 {
		P.Chg -= dt
	}
	if P.Van > 0 {
		P.Van -= dt
	}
	if P.Ctr > 0 {
		P.Ctr *= math.Pow(.5, dt/8) // stored damage fades over time
	}
}

func probeGo2(w *World, r *Probe, shape string, q *Q, lv int, sl float64, ev bool, bs *Slot) bool {
	flv := float64(lv - 1)
	rg := (1 + .15*w.pe("focus")) * (1 + w.PS.F("rng")) * bs.BonusAreaMul() * bs.BonusSizeMul() * b2f(ev, 1.25, 1)
	switch shape {
	case "charge":
		r.Du, r.DL = q.F("du", .9)*b2f(ev, .8, 1), "Charge time"
		r.Area, r.AL = (q.F("r", 85)+q.F("rp", 5)*sl)*rg, "Blast radius"
	case "counter":
		r.Area, r.AL = (q.F("r", 110)+q.F("rp", 6)*sl)*rg, "Radius"
	case "bind":
		r.N, r.NL = q.F("n0", 1)+float64(w.gvBase()+bs.BonusCount()+b2i(ev)), "Foes held"
		r.Du, r.DL = (q.F("du", 3)+.1*flv)*bs.BonusDurMul(), "Hold time"
	case "ramp":
		r.Area, r.AL = (q.F("r", 46)+q.F("rp", 3)*sl)*rg, "Radius"
		if q.F("kos", 0) == 0 {
			r.N, r.NL = q.F("mx", 10), "Max stacks"
		}
	case "sac":
		r.Area, r.AL = (q.F("r", 160)+q.F("rp", 8)*sl)*rg, "Blast radius"
	case "vanish":
		r.Du, r.DL = q.F("du", .7)+.02*flv, "Vanish time"
		r.Area, r.AL = (q.F("r", 95)+q.F("rp", 4)*sl)*rg, "Burst radius"
	default:
		return false
	}
	return true
}
