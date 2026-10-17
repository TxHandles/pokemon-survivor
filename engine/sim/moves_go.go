package sim

import "math"

// Go-only move mechanics: Fly, Rollout (button moves like dashes), Protect / Detect, Earthquake,
// Magnitude, Leech Seed, Hypnosis / Sing and Substitute.

// FlyState: airborne (Fly). The player steers the shadow and lands on it when T runs out.
type FlyState struct {
	T, Du, R, D float64
	Type        string
	Slot        *Slot
}

// RollState: curled up and rolling (Rollout).
type RollState struct {
	T, Du, A, D, W float64
	N              int
	Type           string
	Slot           *Slot
	hit            map[*Enemy]float64
}

// Decoy: a Substitute that lures foes and soaks up shots, then bursts.
type Decoy struct {
	X, Y, T, Du, R, D float64
	Type              string
	Slot              *Slot
	Owner             int
}

// Crack: an Earthquake fissure (a jagged line) that hurts foes standing on it.
type Crack struct {
	Pts      [][2]float64
	L, L0, D float64
	tk       float64
	Type     string
	Slot     *Slot
}

// ActiveKind: moves fired with an action button instead of automatically.
func ActiveKind(k string) bool { return k == "dash" || k == "fly" || k == "rollout" }

// ActiveSlots: the current player's button moves in slot order. The first goes on the main action
// button (Shift / RB), the second on the other (Left Ctrl / LB), then alternating.
func (w *World) ActiveSlots() []*Slot {
	var out []*Slot
	for _, s := range w.Slots {
		if ActiveKind(s.Kind) {
			out = append(out, s)
		}
	}
	return out
}

// ActiveButton is which action button fires slot s (0 main, 1 second, -1 none).
func (w *World) ActiveButton(s *Slot) int {
	acts := w.ActiveSlots()
	for i, a := range acts {
		if a == s {
			if len(acts) == 1 {
				return 0
			}
			return i % 2
		}
	}
	return -1
}

// ActiveReady is slot s's charge (1 = ready).
func (w *World) ActiveReady(s *Slot) float64 {
	cd := w.cdOf(s) * w.cdM()
	if cd <= 0 {
		return 1
	}
	return 1 - math.Max(0, s.CD)/cd
}

// busy: a button move is already running.
func (P *Player) busy() bool { return P.Dash != nil || P.Fly != nil || P.Roll != nil }

// stepActions fires button moves (and Auto-dash for bots).
func (w *World) stepActions(in Input) {
	P := w.P
	if in.Dash {
		P.DQ = w.T
	}
	if in.Act2 {
		P.DQ2 = w.T
	}
	if !P.busy() {
		acts := w.ActiveSlots()
		pick := func(btn int) *Slot {
			if len(acts) == 1 {
				btn = 0
			}
			for i := btn; i < len(acts); i += 2 {
				if acts[i].CD <= 0 {
					return acts[i]
				}
			}
			return nil
		}
		var s *Slot
		switch {
		case P.DQ >= 0 && w.T-P.DQ < .25 && pick(0) != nil:
			s, P.DQ = pick(0), -1
		case P.DQ2 >= 0 && w.T-P.DQ2 < .25 && pick(1) != nil:
			s, P.DQ2 = pick(1), -1
		case w.AutoDash:
			for _, a := range acts {
				if a.CD <= 0 {
					if e := w.near1(P.X, P.Y, 300); e != nil {
						an := math.Atan2(e.Y-P.Y, e.X-P.X)
						P.adA = &an
						s = a
					}
					break
				}
			}
		}
		if s != nil {
			w.fire(s)
			s.CD = w.cdOf(s) * w.cdM()
		}
	}
	if P.DQ >= 0 && w.T-P.DQ >= .25 {
		P.DQ = -1
	}
	if P.DQ2 >= 0 && w.T-P.DQ2 >= .25 {
		P.DQ2 = -1
	}
}

func init() {
	QD["fly"] = map[string]any{"cd": 5.5, "du": 1.4, "r": 78.0, "rp": 3.0, "d": 40.0, "dp": 14.0}
	QD["rollout"] = map[string]any{"cd": 6.5, "du": 2.2, "w": 20.0, "d": 14.0, "dp": 5.0}
	QD["protect"] = map[string]any{"cd": 8.0, "du": 1.1, "r": 95.0, "rp": 3.0, "d": 12.0, "dp": 4.5}
	QD["quake"] = map[string]any{"cd": 6.5, "r": 170.0, "rp": 7.0, "d": 30.0, "dp": 11.0, "n0": 5.0}
	QD["magnitude"] = map[string]any{"cd": 3.6, "r": 40.0, "rp": 4.0, "d": 22.0, "dp": 8.0}
	QD["seed"] = map[string]any{"cd": 4.5, "n0": 1.0, "du": 6.0, "d": 4.0, "dp": 1.6, "ls": .15}
	QD["sleep"] = map[string]any{"cd": 9.0, "du": 2.6, "r": 130.0, "rp": 5.0, "d": 0.0, "dp": 0.0}
	QD["sub"] = map[string]any{"cd": 10.0, "du": 4.0, "r": 90.0, "rp": 4.0, "d": 22.0, "dp": 7.5}
	for _, k := range []string{"fly", "rollout", "protect", "quake", "magnitude", "seed", "sleep", "sub"} {
		Implemented[k] = true
	}
	dmg := func(c mc) float64 { return (c.q.F("d", 0) + c.q.F("dp", 0)*c.sld) * c.em }

	shapes["fly"] = func(w *World, c mc) {
		q := c.q
		du := q.F("du", 1.4)
		R := (q.F("r", 78) + q.F("rp", 3)*c.sl) * (1 + .2*w.pe("wide")) * c.s.BonusSizeMul() * b2f(c.ev, 1.25, 1)
		w.P.adA = nil
		w.P.Fly = &FlyState{T: du, Du: du, R: R, D: dmg(c), Type: c.t, Slot: c.s}
		w.P.Inv = math.Max(w.P.Inv, du+.1)
		w.sfx("dash")
		w.addFX(&FX{Kind: "puff", X: w.P.X, Y: w.P.Y + 8, R: 34, Life: .35, Type: c.t})
	}
	shapes["rollout"] = func(w *World, c mc) {
		q := c.q
		a := w.faceA()
		if w.P.adA != nil {
			a = *w.P.adA
			w.P.adA = nil
		}
		du := (q.F("du", 2.2) + .12*float64(c.lv-1)) * b2f(c.ev, 1.3, 1)
		W := q.F("w", 20) * (1 + .2*w.pe("wide")) * c.s.BonusSizeMul() * b2f(c.ev, 1.2, 1)
		w.P.Roll = &RollState{T: du, Du: du, A: a, D: dmg(c), W: W, Type: c.t, Slot: c.s, hit: map[*Enemy]float64{}}
		w.sfx("dash")
	}
	shapes["protect"] = func(w *World, c mc) {
		q := c.q
		P := w.P
		P.Prot = (q.F("du", 1.1) + .03*float64(c.lv-1)) * b2f(c.ev, 1.3, 1)
		P.ProtType, P.protD, P.protSlot = c.t, dmg(c), c.s
		P.protR = (q.F("r", 95) + q.F("rp", 3)*c.sl) * w.rgm()
		P.protNext = 0
		// raising the shield shoves nearby foes away
		w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 14, MR: P.protR * .7, Fat: true, Type: c.t, D: P.protD * .5, Hit: map[*Enemy]bool{}, Life: .3, KB: 2, Slot: c.s})
	}
	shapes["quake"] = func(w *World, c mc) {
		q := c.q
		P := w.P
		R := (q.F("r", 170) + q.F("rp", 7)*c.sl) * w.rgm() * b2f(c.ev, 1.2, 1)
		d := dmg(c)
		w.Shake = math.Max(w.Shake, .45)
		w.addFX(&FX{Kind: "quake", X: P.X, Y: P.Y, R: R, Life: 1.1, Type: c.t, Slot: c.s})
		for i, k := range []float64{1, .55, .35} {
			k := k
			w.at(c, float64(i)*.2, func() {
				w.addFX(&FX{Kind: "ring", X: w.P.X, Y: w.P.Y, R: 10, MR: R * (1 + .12*float64(i)), TW: 16, Type: c.t, D: d * k, Hit: map[*Enemy]bool{}, Life: .5, KB: 2, Slot: c.s})
			})
		}
		n := int(q.F("n0", 5)) + w.gv() + b2i(c.lv >= 10) + 2*b2i(c.ev)
		a0 := w.rnd(0, 6.283)
		for i := 0; i < n; i++ {
			a := a0 + float64(i)*6.283/float64(n) + w.rnd(-.25, .25)
			L := R * w.rnd(.75, 1)
			pts := [][2]float64{{P.X + math.Cos(a)*16, P.Y + math.Sin(a)*16}}
			for j := 1; j <= 6; j++ {
				t := float64(j) / 6
				ja := a + w.rnd(-.35, .35)*(1-t*.5)
				pts = append(pts, [2]float64{P.X + math.Cos(ja)*L*t, P.Y + math.Sin(ja)*L*t})
			}
			w.Cracks = append(w.Cracks, &Crack{Pts: pts, L: 3, L0: 3, D: d * .22, tk: .3, Type: c.t, Slot: c.s})
		}
		if len(w.Cracks) > 40 {
			w.Cracks = w.Cracks[len(w.Cracks)-40:]
		}
	}
	shapes["magnitude"] = func(w *World, c mc) {
		q := c.q
		P := w.P
		r := w.rng.Float64()
		m := 4
		for _, t := range []float64{.2, .4, .58, .73, .85, .94} { // 4..10, high magnitudes are rarer
			if r < t {
				break
			}
			m++
		}
		fm := float64(m)
		R := (q.F("r", 40) + q.F("rp", 4)*c.sl + 22*fm) * w.rgm()
		d := dmg(c) * (.4 + .18*(fm-3))
		w.Shake = math.Max(w.Shake, .06+.045*(fm-3))
		w.addFX(&FX{Kind: "mag", X: P.X, Y: P.Y, R: R, Life: .9 + .06*fm, Text: itoa(m), Type: c.t, Slot: c.s, D: fm})
		w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 8, MR: R, Fat: true, Type: c.t, D: d, Hit: map[*Enemy]bool{}, Life: .4, KB: 2, Slot: c.s})
		if m >= 8 { // aftershock
			x, y := P.X, P.Y
			w.at(c, .3, func() {
				w.addFX(&FX{Kind: "ring", X: x, Y: y, R: 8, MR: R * 1.15, Fat: true, Type: c.t, D: d * .5, Hit: map[*Enemy]bool{}, Life: .4, KB: 2, Slot: c.s})
			})
		}
	}
	shapes["seed"] = func(w *World, c mc) {
		q := c.q
		n := int(q.F("n0", 1)) + w.gv() + b2i(c.lv >= 5) + b2i(c.lv >= 10) + b2i(c.ev) + b2i(c.cap)
		du := (q.F("du", 6) + .15*float64(c.lv-1)) * b2f(c.ev, 1.3, 1)
		d := dmg(c)
		P := w.P
		var picks []*Enemy
		for _, e := range w.near(P.X, P.Y, n*3, nil, 380) {
			if len(picks) < n && !(e.LS > 0) {
				picks = append(picks, e)
			}
		}
		for _, e := range w.near(P.X, P.Y, n, nil, 380) { // everything seeded: refresh the nearest
			if len(picks) < n && !inList(picks, e) {
				picks = append(picks, e)
			}
		}
		for _, e := range picks {
			w.lobSeed(P.X, P.Y, e, du, d, c.s, w.CUR)
		}
	}
	shapes["sleep"] = func(w *World, c mc) {
		q := c.q
		P := w.P
		R := (q.F("r", 130) + q.F("rp", 5)*c.sl) * w.rgm() * b2f(c.ev, 1.2, 1)
		du := (q.F("du", 2.6) + .08*float64(c.lv-1)) * b2f(c.ev, 1.3, 1)
		k := 1.5 + .03*float64(c.lv-1) + b2f(c.ev, .3, 0)
		cone := q.F("cone", 0) > 0
		a, hw := 0.0, math.Pi
		if cone {
			a, hw = w.faceA(), .7
			if e := w.near1(P.X, P.Y, R); e != nil {
				a = math.Atan2(e.Y-P.Y, e.X-P.X)
			}
		}
		w.addFX(&FX{Kind: "lull", X: P.X, Y: P.Y, R: R, A: a, Wd: hw, Life: .8, Type: c.t, Slot: c.s})
		for _, e := range w.E {
			if e.Dead || e.Ug || e.Boss {
				continue
			}
			dx, dy := e.X-P.X, e.Y-P.Y
			if hyp(dx, dy) > R+e.R {
				continue
			}
			if cone {
				df := math.Atan2(dy, dx) - a
				if math.Abs(math.Atan2(math.Sin(df), math.Cos(df))) > hw {
					continue
				}
			}
			e.Slp = math.Max(e.Slp, du*b2f(e.Elite, .5, 1))
			e.slpK = k
		}
	}
	shapes["sub"] = func(w *World, c mc) {
		q := c.q
		P := w.P
		du := (q.F("du", 4) + .15*float64(c.lv-1)) * b2f(c.ev, 1.3, 1)
		R := (q.F("r", 90) + q.F("rp", 4)*c.sl) * w.rgm()
		keep := w.Decoys[:0:0]
		for _, d := range w.Decoys {
			if d.Slot != c.s {
				keep = append(keep, d)
			}
		}
		a := w.faceA()
		x, y := P.X-math.Cos(a)*28, P.Y-math.Sin(a)*28
		w.Decoys = append(keep, &Decoy{X: x, Y: y, T: du, Du: du, R: R, D: dmg(c), Type: c.t, Slot: c.s, Owner: w.CUR})
		w.addFX(&FX{Kind: "puff", X: x, Y: y, R: 28, Life: .35, Type: c.t})
	}
}

// lobSeed throws a Leech Seed at e; it takes hold when it lands.
func (w *World) lobSeed(x, y float64, e *Enemy, du, d float64, s *Slot, owner int) {
	const t = .35
	w.addFX(&FX{Kind: "lob", X: x, Y: y, AX: x, AY: y, BX: e.X, BY: e.Y, Life: t, Type: "grass", Slot: s})
	w.DL = append(w.DL, &delay{t: t, fn: func() {
		if e.Dead {
			return
		}
		e.LS = math.Max(e.LS, du)
		e.lsD, e.lsSlot, e.lsOwner = d, s, owner
		if e.lsK <= 0 {
			e.lsK = .5
		}
	}})
}

// stepSeed drains a seeded foe and heals the seed's owner.
func (w *World) stepSeed(e *Enemy, dt float64) {
	e.LS -= dt
	e.lsK -= dt
	if e.lsK > 0 {
		return
	}
	e.lsK = .5
	d := math.Max(1, math.Round(e.lsD*(1+.1*w.pe("heat"))))
	w.hitRaw(e, d, "grass")
	if e.lsSlot != nil {
		w.DMG["_dot"] -= d
		w.DMG[e.lsSlot.ID] += d
	}
	if e.lsOwner >= 0 && e.lsOwner < len(w.PL) && !w.Run.NoHeal {
		if P := w.PL[e.lsOwner].P; !P.Dead {
			ls := .15
			if e.lsSlot != nil {
				ls = e.lsSlot.Q.F("ls", .15)
			}
			P.HP = math.Min(P.MHP, P.HP+math.Min(d*ls, P.MHP*.015))
		}
	}
}

// seedJump: a fainted host's seed jumps to the nearest unseeded foe.
func (w *World) seedJump(e *Enemy) {
	if !(e.LS > .5) {
		return
	}
	var best *Enemy
	bd := 170.0 * 170
	for _, o := range w.E {
		if o == e || o.Dead || o.Ug || o.LS > 0 {
			continue
		}
		if d := sq(o.X-e.X) + sq(o.Y-e.Y); d < bd {
			best, bd = o, d
		}
	}
	if best != nil {
		w.lobSeed(e.X, e.Y, best, e.LS, e.lsD, e.lsSlot, e.lsOwner)
	}
	e.LS = 0
}

// stepFly: the player is airborne; lands on the shadow when time runs out.
func (w *World) stepFly(dt float64) {
	P := w.P
	F := P.Fly
	F.T -= dt
	if F.T > 0 {
		return
	}
	P.Fly = nil
	P.Inv = math.Max(P.Inv, .3)
	w.Shake = math.Max(w.Shake, .25)
	w.addFX(&FX{Kind: "slam", X: P.X, Y: P.Y, R: F.R, Life: .5, Type: F.Type, Slot: F.Slot})
	w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 8, MR: F.R, Fat: true, Type: F.Type, D: F.D, Hit: map[*Enemy]bool{}, Life: .3, KB: 2, Slot: F.Slot})
	w.sfx("hit")
}

// stepRoll moves a rolling player: steerable, faster with every foe hit, ends on time or a crash.
func (w *World) stepRoll(dt, ix, iy float64) {
	P := w.P
	R := P.Roll
	P.X -= P.VX * dt
	P.Y -= P.VY * dt
	if ix != 0 || iy != 0 { // steer towards the stick
		df := math.Atan2(iy, ix) - R.A
		df = math.Atan2(math.Sin(df), math.Cos(df))
		R.A += math.Max(-4*dt, math.Min(4*dt, df))
	}
	sp := 270 * (1 + .07*math.Min(12, float64(R.N))) * R.Slot.BonusSpdMul()
	ox, oy := P.X, P.Y
	mx, my := math.Cos(R.A)*sp*dt, math.Sin(R.A)*sp*dt
	P.X += mx
	P.Y += my
	P.FX, P.FY = math.Cos(R.A), math.Sin(R.A)
	w.push(&P.X, &P.Y, 12)
	moved := hyp(P.X-ox, P.Y-oy)
	R.T -= dt
	P.Inv = math.Max(P.Inv, .05)
	prev := w.curSlot
	w.curSlot = R.Slot
	for _, e := range w.E {
		if e.Dead || e.Ug || R.hit[e] > w.T || dist(e.X, e.Y, P.X, P.Y) > R.W+e.R {
			continue
		}
		R.hit[e] = w.T + .45
		w.hit(e, R.D*(1+.12*math.Min(15, float64(R.N))), R.Type, 2, P.X, P.Y)
		R.N++
	}
	crash := moved < hyp(mx, my)*.35
	if crash || R.T <= 0 {
		if crash { // smashed into a tree or rock: a last burst around you
			w.Shake = math.Max(w.Shake, .2)
			w.addFX(&FX{Kind: "puff", X: P.X, Y: P.Y, R: 50, Life: .35, Type: R.Type})
			for _, e := range w.E {
				if !e.Dead && dist(e.X, e.Y, P.X, P.Y) < 60+e.R {
					w.hit(e, R.D*(1+.12*math.Min(15, float64(R.N))), R.Type, 2, P.X, P.Y)
				}
			}
		}
		P.Roll = nil
		P.Inv = math.Max(P.Inv, .3)
	}
	w.curSlot = prev
}

// protectBlock: Protect is up, so the hit is blocked and a shockwave answers it.
func (w *World) protectBlock() bool {
	P := w.P
	if !(P.Prot > 0) {
		return false
	}
	if P.protNext <= w.T {
		P.protNext = w.T + .35
		w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 14, MR: P.protR, Fat: true, Type: P.ProtType, D: P.protD, Hit: map[*Enemy]bool{}, Life: .35, KB: 2, Slot: P.protSlot})
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 34, Life: .6, Text: "Blocked!", Col: "se"})
	}
	P.Inv = math.Max(P.Inv, .25)
	return true
}

// stepGoMoves: decoys and fissures.
func (w *World) stepGoMoves(dt float64) {
	keep := w.Decoys[:0]
	for _, d := range w.Decoys {
		d.T -= dt
		if d.T > 0 {
			keep = append(keep, d)
			continue
		}
		w.useP(d.Owner)
		w.addFX(&FX{Kind: "puff", X: d.X, Y: d.Y, R: d.R * .6, Life: .4, Type: d.Type})
		w.addFX(&FX{Kind: "ring", X: d.X, Y: d.Y, R: 10, MR: d.R, Fat: true, Type: d.Type, D: d.D, Hit: map[*Enemy]bool{}, Life: .35, KB: 2, Slot: d.Slot})
	}
	w.Decoys = keep
	ck := w.Cracks[:0]
	for _, c := range w.Cracks {
		c.L -= dt
		c.tk -= dt
		if c.tk <= 0 && c.L > 0 {
			c.tk = .4
			if c.Slot != nil {
				w.useP(c.Slot.Owner)
			}
			prev := w.curSlot
			w.curSlot = c.Slot
			for _, e := range w.E {
				if e.Dead || e.Ug {
					continue
				}
				for i := 1; i < len(c.Pts); i++ {
					if segDist(e.X, e.Y, c.Pts[i-1], c.Pts[i]) < 12+e.R {
						w.hit(e, c.D, c.Type, 0, 0, 0)
						break
					}
				}
			}
			w.curSlot = prev
		}
		if c.L > 0 {
			ck = append(ck, c)
		}
	}
	w.Cracks = ck
}

func segDist(x, y float64, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((x-a[0])*dx+(y-a[1])*dy)/l2))
	}
	return hyp(x-(a[0]+dx*t), y-(a[1]+dy*t))
}

// lure: the angle towards a Substitute that draws foe e (ok=false when none does).
func (w *World) lure(e *Enemy, px, py float64) (float64, bool) {
	var best *Decoy
	bd := 330.0 * 330
	for _, d := range w.Decoys {
		if dd := sq(d.X-e.X) + sq(d.Y-e.Y); dd < bd {
			best, bd = d, dd
		}
	}
	if best == nil || math.Sqrt(bd) > dist(e.X, e.Y, px, py)+80 {
		return 0, false
	}
	return math.Atan2(best.Y-e.Y, best.X-e.X), true
}

// decoyEats: a Substitute soaks up an enemy shot.
func (w *World) decoyEats(b *EBullet) bool {
	for _, d := range w.Decoys {
		if dist(b.X, b.Y, d.X, d.Y) < 16+b.R {
			return true
		}
	}
	return false
}

// probeGo fills the level-up card numbers for the Go-only shapes (computed, nothing is fired).
func probeGo(w *World, r *Probe, shape string, q *Q, lv int, sl float64, ev bool, bs *Slot) bool {
	flv := float64(lv - 1)
	rg := (1 + .15*w.pe("focus")) * (1 + w.PS.F("rng")) * bs.BonusAreaMul()
	switch shape {
	case "spin":
		r.Area, r.AL = (q.F("r", 82)+q.F("rp", 7)*sl)*rg*(1+.1*w.pe("wide"))*b2f(ev, 1.25, 1), "Radius"
		r.Du, r.DL = q.F("du", 1.4)*.82*bs.BonusDurMul()*b2f(ev, 1.3, 1), "Spin time"
		r.N, r.NL = float64(2+bs.BonusCount()), "Kick legs"
	case "fly":
		r.Area, r.AL = 2*(q.F("r", 78)+q.F("rp", 3)*sl)*(1+.2*w.pe("wide"))*bs.BonusSizeMul()*b2f(ev, 1.25, 1), "Landing size"
		r.Du, r.DL = q.F("du", 1.4), "Air time"
	case "rollout":
		r.Du, r.DL = (q.F("du", 2.2)+.12*flv)*b2f(ev, 1.3, 1), "Roll time"
		r.Area, r.AL = 2*q.F("w", 20)*(1+.2*w.pe("wide"))*bs.BonusSizeMul()*b2f(ev, 1.2, 1), "Ball size"
	case "protect":
		r.Du, r.DL = (q.F("du", 1.1)+.03*flv)*b2f(ev, 1.3, 1), "Shield time"
		r.Area, r.AL = (q.F("r", 95)+q.F("rp", 3)*sl)*rg, "Shockwave"
	case "quake":
		r.N, r.NL = q.F("n0", 5)+float64(w.gvBase()+bs.BonusCount()+b2i(lv >= 10)+2*b2i(ev)), "Fissures"
		r.Area, r.AL = (q.F("r", 170)+q.F("rp", 7)*sl)*rg*b2f(ev, 1.2, 1), "Radius"
	case "magnitude":
		r.Area, r.AL = (q.F("r", 40)+q.F("rp", 4)*sl+22*7)*rg, "Radius (mag 7)"
	case "seed":
		r.N, r.NL = q.F("n0", 1)+float64(w.gvBase()+bs.BonusCount()+b2i(lv >= 5)+b2i(lv >= 10)+b2i(ev)), "Seeds"
		r.Du, r.DL = (q.F("du", 6)+.15*flv)*b2f(ev, 1.3, 1), "Drain time"
	case "sleep":
		r.Du, r.DL = (q.F("du", 2.6)+.08*flv)*b2f(ev, 1.3, 1), "Sleep time"
		r.Area, r.AL = (q.F("r", 130)+q.F("rp", 5)*sl)*rg*b2f(ev, 1.2, 1), "Reach"
	case "sub":
		r.Du, r.DL = (q.F("du", 4)+.15*flv)*b2f(ev, 1.3, 1), "Decoy time"
		r.Area, r.AL = (q.F("r", 90)+q.F("rp", 4)*sl)*rg, "Burst size"
	default:
		return false
	}
	return true
}
