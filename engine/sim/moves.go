package sim

import (
	"math"
	"sort"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

type P = map[string]any

// QD: shape defaults (web QD after the attack rework).
var QD = map[string]P{
	"arc":     {"cd": 1.1, "r": 62.0, "rp": 11.0, "hw": .62, "hp": .07, "d": 12.0, "dp": 5.0, "ks": 110.0, "kp": 55.0, "hits": 1.0, "hm": 1.0, "ls": 0.0, "shape": "arc"},
	"beam":    {"cd": 5.5, "d": 38.0, "dp": 16.0, "w": 12.0, "wp": .9, "L": 600.0, "tele": .3},
	"dash":    {"cd": 4.2, "L": 230.0, "d": 26.0, "dp": 10.0, "w": 16.0, "wp": .6},
	"strike":  {"cd": 2.2, "n0": 1.0, "d": 20.0, "dp": 8.0, "r": 22.0, "rp": 1.2, "tele": 0.0},
	"erupt":   {"cd": 2.6, "n0": 1.0, "r": 52.0, "rp": 7.0, "d": 14.0, "dp": 6.0, "tele": .4},
	"homing":  {"cd": 3.0, "n0": 1.0, "sp": 115.0, "l": 4.0, "d": 11.0, "dp": 5.0, "p": 2.0, "r": 8.0, "ls": 0.0, "tr": 6.0},
	"boom":    {"cd": 1.7, "n0": 1.0, "sp": 300.0, "out": .5, "d": 9.0, "dp": 4.0, "r": 9.0, "coin": 0.0},
	"whirl":   {"cd": 4.8, "d": 3.0, "dp": 1.4, "r": 34.0, "rp": 3.0, "l": 3.5, "pull": 100.0, "pr": 1.9},
	"aura":    {"cd": .7, "d": 6.0, "dp": 2.4, "r": 95.0, "rp": 6.0},
	"wave":    {"cd": 3.4, "r": 44.0, "rp": 3.5, "du": 1.1, "dup": .05, "sk": "su"},
	"screen":  {"cd": 1e9},
	"clone":   {"cd": 1.1, "dm": .5},
	"jab":     {"cd": .85, "n0": 2.0, "r": 110.0, "rp": 3.0, "d": 7.0, "dp": 3.0},
	"burst":   {"cd": 2.2, "n0": 5.0, "d": 7.5, "dp": 3.1, "sp": 480.0, "gap": .055},
	"twin":    {"cd": 1.3, "n0": 2.0, "sp": 420.0, "d": 6.0, "dp": 2.8, "r": 6.0, "p": 2.0, "l": .9},
	"sludge":  {"cd": 1.4, "n0": 4.0, "sp": 230.0, "l": .55, "d": 7.0, "dp": 3.0, "r": 8.0, "p": 1.0},
	"blast":   {"cd": 3.6, "off": 52.0, "r": 58.0, "rp": 5.0, "d": 50.0, "dp": 20.0, "tele": .28},
	"proj":    {"cd": 1.1, "n0": 2.0, "sp": 360.0, "d": 7.0, "dp": 3.2, "r": 7.0, "p": 1.0, "l": 1.2, "spr": .22, "mode": "near"},
	"nova":    {"cd": 2.2, "r0": 100.0, "rp": 22.0, "d": 10.0, "dp": 3.5, "fat": 0.0},
	"trail":   {"cd": .3, "r": 15.0, "rp": 2.0, "l": 1.2, "lp": .35, "d": 4.0, "dp": 2.0},
	"orbit":   {"cd": 1e9, "n0": 1.0, "d": 7.0, "dp": 3.0, "rr": 55.0, "rrp": 5.0},
	"cone":    {"cd": 2.4, "du": 1.0, "tk": .12, "r": 120.0, "rp": 8.0, "hw": .45, "d": 3.2, "dp": 1.3},
	"sweep":   {"cd": 11.0, "d": 6.0, "dp": 2.4, "w": 46.0, "sp": 560.0, "push": 160.0},
	"chain":   {"cd": 1.6, "n0": 3.0, "d": 9.0, "dp": 3.5, "rng": 260.0, "jr": 170.0, "dec": .9},
	"spin":    {"cd": 3.6, "du": 1.4, "tk": .2, "r": 82.0, "rp": 7.0, "d": 3.5, "dp": 1.4, "ks": 60.0},
	"mine":    {"cd": 2.0, "n0": 1.0, "d": 22.0, "dp": 9.0, "r": 14.0, "br": 58.0, "brp": 4.0, "arm": .5, "l": 10.0, "max": 8.0},
	"rupture": {"cd": 2.8, "n0": 5.0, "gap": 44.0, "stp": .07, "r": 30.0, "rp": 2.5, "d": 12.0, "dp": 5.0},
}

// Implemented shapes in this build; moves of other shapes are not offered yet.
var Implemented = map[string]bool{}

func init() {
	for k := range QD {
		if k != "screen" && k != "clone" {
			Implemented[k] = true
		}
	}
}

var tierMul = map[string][2]float64{"basic": {1, 1}, "evolved": {1.35, 1.08}, "final": {1.75, 1.2}}

// BuildQ merges shape defaults, move params and tier multipliers (web MV[x].q).
func BuildQ(m *content.Move) *Q {
	q := P{}
	for k, v := range QD[m.Shape] {
		q[k] = v
	}
	for k, v := range m.Params {
		q[k] = v
	}
	Q := &Q{M: q}
	tm := tierMul[m.Tier]
	dm, cdm := Q.F("dm", 1)*tm[0], Q.F("cdm", 1)*tm[1]
	if _, ok := q["d"]; ok {
		q["d"] = Q.F("d", 0) * dm
		q["dp"] = Q.F("dp", 0) * dm
	}
	if Q.F("cd", 1) < 1e8 {
		q["cd"] = Q.F("cd", 1) * cdm
	}
	if rm := Q.F("rm", 0); rm > 0 {
		if _, ok := q["r0"]; ok {
			q["r0"] = Q.F("r0", 0) * rm
		} else if _, ok := q["r"]; ok {
			q["r"] = Q.F("r", 0) * rm
		}
	}
	return Q
}

func (w *World) cdOf(s *Slot) float64 {
	if s.Kind == "aura" {
		c := s.Q.F("cd", 1) * (1 - .025*float64(s.Lv))
		if s.Evo {
			c *= .85
		}
		return math.Max(.22, c)
	}
	return s.Q.F("cd", 1)
}

// ---- targeting ----

func ew(e *Enemy) float64 {
	if e.Boss {
		return .5
	}
	if e.Elite {
		return .85
	}
	return 1
}

// near returns up to n enemies nearest to (x,y) within rng, skipping ex (bosses count as closer).
func (w *World) near(x, y float64, n int, ex []*Enemy, rng float64) []*Enemy {
	type c struct {
		d float64
		e *Enemy
	}
	var o []c
	for _, e := range w.E {
		if e.Dead || e.Ug || (len(ex) > 0 && inList(ex, e)) {
			continue
		}
		d := hyp(e.X-x, e.Y-y)
		if d < rng {
			o = append(o, c{d * ew(e), e})
		}
	}
	sort.Slice(o, func(i, j int) bool { return o[i].d < o[j].d })
	if len(o) > n {
		o = o[:n]
	}
	r := make([]*Enemy, len(o))
	for i := range o {
		r[i] = o[i].e
	}
	return r
}

func (w *World) near1(x, y float64, rng float64) *Enemy {
	var be *Enemy
	bk := 1e18
	for _, e := range w.E {
		if e.Dead || e.Ug {
			continue
		}
		dx, dy := e.X-x, e.Y-y
		d2 := dx*dx + dy*dy
		if d2 >= rng*rng {
			continue
		}
		k := d2 * ew(e) * ew(e)
		if k < bk {
			bk, be = k, e
		}
	}
	return be
}

func inList(l []*Enemy, e *Enemy) bool {
	for _, x := range l {
		if x == e {
			return true
		}
	}
	return false
}

// tgt: n random enemies within rng.
func (w *World) tgt(n int, rng float64) []*Enemy {
	var a []*Enemy
	for _, e := range w.E {
		if !e.Dead && !e.Ug && dist(w.P.X, w.P.Y, e.X, e.Y) < rng {
			a = append(a, e)
		}
	}
	w.rng.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	if len(a) > n {
		a = a[:n]
	}
	return a
}

func (w *World) faceA() float64 { return math.Atan2(w.P.FY, w.P.FX) }

func (w *World) aimA() float64 {
	if e := w.near1(w.P.X, w.P.Y, 1e9); e != nil {
		return math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
	}
	return w.faceA()
}

// aimWithin aims at the nearest enemy if it is within r, else the facing direction.
func (w *World) aimWithin(r float64) float64 {
	if e := w.near1(w.P.X, w.P.Y, 1e9); e != nil && dist(w.P.X, w.P.Y, e.X, e.Y) < r {
		return math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
	}
	return w.faceA()
}

func b2f(b bool, t, f float64) float64 {
	if b {
		return t
	}
	return f
}
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- firing ----

type mc struct {
	lv      int
	sl, em  float64
	t       string
	ev, cap bool
	q       *Q
	s       *Slot
}

func (w *World) cn3(c mc) int {
	return int(c.q.F("n0", 1)) + b2i(c.lv >= 5) + b2i(c.lv >= 10) + w.gv() + b2i(c.ev) + b2i(c.cap)
}

// pj adds a projectile (web pj()).
func (w *World) pj(c mc, o *Proj) {
	o.R *= 1 + .05*float64(c.lv-1)
	sp := 1 + .12*w.pe("metro")
	o.VX *= sp
	o.VY *= sp
	o.R *= 1 + .2*w.pe("wide")
	o.L *= (1 + .15*w.pe("focus")) / sp
	if o.Type == "psychic" {
		o.Sk = true
	}
	o.Slot = c.s
	w.PR = append(w.PR, o)
}

func (w *World) ring(c mc, x, y, mr, tw, d, life float64, fat bool, kb float64) *FX {
	f := &FX{Kind: "ring", X: x, Y: y, R: 8, MR: mr, TW: tw, Type: c.t, D: d, Hit: map[*Enemy]bool{}, Life: life, Fat: fat, KB: kb, Slot: c.s}
	w.addFX(f)
	return f
}

// fire uses a move once (web use()).
func (w *World) fire(s *Slot) {
	if w.T-w.P.AtkT > 1.6 {
		w.P.AtkT = w.T
	}
	w.metroTick(s)
	q := s.Q
	ev := s.Evo
	em := 1.0
	if ev {
		em = 1.35 * (1 + .08*w.pe(s.Move.HeldItem))
	}
	c := mc{lv: s.Lv, sl: 1 + float64(s.Lv-1)*.45, em: em, t: s.Type, ev: ev, cap: ev && s.Lv >= 20, q: q, s: s}
	if v := q.F("hpc", 0); v > 0 && w.P.HP > w.P.MHP*.3 {
		w.P.HP -= w.P.MHP * v
	}
	if q.F("spd", 0) > 0 {
		w.P.SpdT = w.T + 2.5
	}
	prev := w.curSlot
	w.curSlot = s
	defer func() { w.curSlot = prev }()
	if f := shapes[s.Kind]; f != nil {
		f(w, c)
	}
}

// later2 schedules fn with the move attributed to c's slot.
func (w *World) at(c mc, t float64, fn func()) {
	s := c.s
	w.later(t, func() {
		if s != nil {
			w.useP(s.Owner)
		}
		prev := w.curSlot
		w.curSlot = s
		fn()
		w.curSlot = prev
	})
}

var shapes map[string]func(w *World, c mc)

func init() {
	shapes = map[string]func(w *World, c mc){
		"arc": func(w *World, c mc) {
			q := c.q
			a0 := w.aimWithin(170)
			sh := q.S("shape")
			hits := int(q.F("hits", 1))
			go_ := func() {
				w.swing(c, a0, 1)
				for i := 1; i < hits; i++ {
					w.at(c, .12*float64(i), func() { w.swing(c, a0, q.F("hm", 1)) })
				}
				if q.F("pj", 0) > 0 || (c.ev && q.F("nw", 0) == 0) {
					d := (10 + 4*c.sl) * c.em * b2f(q.F("pj", 0) > 0, .9, .8)
					w.pj(c, &Proj{X: w.P.X, Y: w.P.Y, VX: math.Cos(a0) * 320, VY: math.Sin(a0) * 320, Type: c.t, D: d, P: 3, L: .7, R: b2f(q.F("pj", 0) > 0, 11, 14)})
				}
				if c.cap {
					w.at(c, b2f(hits > 1, .4, .2), func() { w.swing(c, b2f(sh == "360", a0, a0+math.Pi), .8) })
				}
			}
			if tl := q.F("tele", 0); tl > 0 {
				R := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * b2f(c.ev, 1.25, 1)
				w.addFX(&FX{Kind: "tele", X: w.P.X, Y: w.P.Y, R: R, Life: tl, Type: c.t})
				w.at(c, tl, go_)
			} else {
				go_()
			}
		},
		"beam": func(w *World, c mc) {
			q := c.q
			n := 1 + b2i(c.ev) + b2i(c.cap) + w.gv()
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			for i, e := range w.tgt(n, 620) {
				a := math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
				wd := (q.F("w", 0) + q.F("wp", 0)*c.sl) * b2f(c.ev, 1.3, 1) * b2f(c.cap, 1.2, 1) * (1 + .2*w.pe("wide"))
				L := q.F("L", 600) * w.rgm()
				tl := q.F("tele", .3)
				w.addFX(&FX{Kind: "bt", X: w.P.X, Y: w.P.Y, A: a, Ln: L, Life: tl, Type: c.t})
				w.at(c, tl+float64(i)*.1, func() {
					ox, oy := w.P.X, w.P.Y
					w.addFX(&FX{Kind: "beam", X: ox, Y: oy, A: a, Ln: L, Wd: wd, Life: .28, Type: c.t})
					w.Shake = math.Max(w.Shake, .1)
					w.rectHit(ox, oy, a, L, wd, func(o *Enemy) { w.hit(o, d, c.t, 1, ox, oy) })
				})
			}
		},
		"dash": func(w *World, c mc) {
			q := c.q
			a := w.faceA()
			if w.P.adA != nil {
				a = *w.P.adA
				w.P.adA = nil
			}
			L := q.F("L", 230) * w.rgm() * b2f(c.ev, 1.3, 1)
			wd := (q.F("w", 0) + q.F("wp", 0)*c.sl) * b2f(c.ev, 1.3, 1) * (1 + .2*w.pe("wide"))
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			const sp = 1150
			du := L / sp
			w.sfx("dash")
			w.P.Dash = &DashState{VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, T: du, W: wd, D: d, Type: c.t, Slot: c.s, Cap: c.cap,
				Phase: has(w.P.Types, "flying") || has(w.P.Types, "ghost"), Hit: map[*Enemy]bool{}}
			w.P.Inv = math.Max(w.P.Inv, du+.15)
		},
		"strike": func(w *World, c mc) {
			q := c.q
			n := w.cn3(c)
			r := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * b2f(c.ev, 1.2, 1)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			for i, e := range w.tgt(n, 560) {
				x, y := e.X, e.Y
				go_ := func() {
					w.addFX(&FX{Kind: "sky", X: x, Y: y, Life: .22, Type: c.t})
					w.addFX(&FX{Kind: "puff", X: x, Y: y, R: r, Life: .25, Type: c.t})
					w.Shake = math.Max(w.Shake, .05)
					var hs []*Enemy
					for _, o := range w.E {
						if o.Dead || hyp(o.X-x, o.Y-y) > r+o.R {
							continue
						}
						hs = append(hs, o)
						w.hit(o, d*b2f(q.F("vs", 0) > 0 && o.Psn > 0, 1.7, 1), c.t, 1, x, y)
					}
					if c.ev {
						cx, cy := x, y
						for j := 0; j < 2+2*b2i(c.cap); j++ {
							nx := w.near(cx, cy, 1, hs, 170*w.rgm())
							if len(nx) == 0 {
								break
							}
							o := nx[0]
							w.addFX(&FX{Kind: "bolt", Type: c.t, AX: cx, AY: cy, BX: o.X, BY: o.Y, Life: .15})
							w.hit(o, d*.7, c.t, 1, cx, cy)
							hs = append(hs, o)
							cx, cy = o.X, o.Y
						}
					}
				}
				if tl := q.F("tele", 0); tl > 0 {
					w.addFX(&FX{Kind: "tele", X: x, Y: y, R: r, Life: tl, Type: c.t})
					w.at(c, tl+float64(i)*.05, go_)
				} else {
					w.at(c, float64(i)*.06, go_)
				}
			}
		},
		"erupt": func(w *World, c mc) {
			q := c.q
			n := w.cn3(c)
			r := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(c.ev, 1.25, 1)
			boomF := func(x, y, mu float64) {
				f := w.ring(c, x, y, r*b2f(mu > .9, 1, .8), 0, (q.F("d", 0)+q.F("dp", 0)*c.sl)*.8*c.em*mu, .3, true, 1)
				f.R = 6
				w.Shake = math.Max(w.Shake, q.F("shake", .12))
				if c.ev && mu > .9 {
					w.ZN = append(w.ZN, &Zone{X: x, Y: y, R: r * .75, L: 3, Life0: 3, Type: c.t, D: (4 + 2*c.sl) * c.em, Slot: c.s})
				}
			}
			tl := q.F("tele", .4)
			for i, e := range w.tgt(n, 460) {
				x, y := e.X, e.Y
				w.addFX(&FX{Kind: "tele", X: x, Y: y, R: r, Life: tl, Type: c.t})
				fi := float64(i)
				w.at(c, tl+fi*.08, func() { boomF(x, y, 1) })
				if c.cap {
					w.at(c, tl+.5+fi*.08, func() { boomF(x, y, .6) })
				}
			}
		},
		"homing": func(w *World, c mc) {
			q := c.q
			n := w.cn3(c)
			a0 := w.aimA()
			tl := w.near(w.P.X, w.P.Y, n, nil, 520)
			for i := 0; i < n; i++ {
				off := float64(i) - float64(n-1)/2
				var te *Enemy
				a := a0 + off*.9
				if i < len(tl) {
					te = tl[i]
					a = math.Atan2(te.Y-w.P.Y, te.X-w.P.X) + off*.25
				}
				sp := q.F("sp", 115)
				ex := 0.0
				if c.ev {
					ex = b2f(c.cap, 1.5, 1)
				}
				w.pj(c, &Proj{Tg: te, X: w.P.X, Y: w.P.Y, VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, Type: c.t, D: (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em,
					P: int(q.F("p", 2)) + b2i(c.lv >= 7) + b2i(c.ev), L: q.F("l", 4) * b2f(c.ev, 1.375, 1), R: q.F("r", 8) * b2f(c.ev, 1.25, 1), Sk: true, Tr: q.F("tr", 6), Ls: q.F("ls", 0), Ex: ex})
			}
		},
		"boom": func(w *World, c mc) {
			q := c.q
			n := w.cn3(c)
			a0 := w.aimA()
			for i := 0; i < n; i++ {
				a := a0 + (float64(i)-float64(n-1)/2)*.42
				sp := q.F("sp", 300)
				w.pj(c, &Proj{X: w.P.X, Y: w.P.Y, VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, Type: c.t, D: (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em, P: 99, L: 3.4,
					R: q.F("r", 9) * b2f(c.ev, 1.3, 1), Bm: &boom{out: q.F("out", .5) * w.rgm(), sp: sp * (1 + .12*w.pe("metro"))}})
			}
		},
		"whirl": func(w *World, c mc) {
			q := c.q
			n := 1 + b2i(c.ev) + b2i(c.cap)
			for _, e := range w.tgt(n, 460) {
				l := q.F("l", 3.5) * b2f(c.ev, 1.4, 1)
				w.ZN = append(w.ZN, &Zone{X: e.X, Y: e.Y, R: (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * b2f(c.ev, 1.3, 1), L: l, Life0: l, Type: c.t,
					D: (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em, Pull: q.F("pull", 100) * b2f(c.ev, 1.4, 1), Pr: q.F("pr", 1.9), Slot: c.s})
			}
		},
		"aura": func(w *World, c mc) {
			R := w.auraR(c.s)
			d := (c.q.F("d", 0) + c.q.F("dp", 0)*c.sl) * c.em * b2f(c.cap, 1.3, 1)
			for _, e := range w.E {
				if !e.Dead && dist(w.P.X, w.P.Y, e.X, e.Y) < R+e.R {
					w.hit(e, d, c.t, 0, 0, 0)
				}
			}
		},
		"wave": func(w *World, c mc) {
			q := c.q
			mr := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * b2f(c.ev, 1.3, 1)
			sk := q.S("sk")
			mk := func(mu float64) {
				f := w.ring(c, w.P.X, w.P.Y, mr*mu, 14, 0, .45, false, 0)
				f.Sk = sk
				f.Du = (q.F("du", 0) + q.F("dup", 0)*c.sl) * b2f(c.ev, 1.4, 1) * b2f(mu < 1, .8, 1)
				if c.ev || q.F("dmg", 0) > 0 {
					f.Dm = (5 + 2*c.sl) * c.em * b2f(c.ev, 1, .7)
				}
				f.Rt = c.ev && sk == "sl"
			}
			mk(1)
			if c.cap {
				w.at(c, .45, func() { mk(.85) })
			}
		},
		"orbit": func(w *World, c mc) {},
		"proj": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 2)) + b2i(c.lv >= 4) + b2i(c.lv >= 10) + w.gv() + b2i(c.ev) + b2i(c.cap)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			ex := 0.0
			if q.F("ex", 0) > 0 || c.ev {
				ex = b2f(c.cap, 1.5, 1)
			}
			pp := int(q.F("p", 1)) + b2i(c.ev)
			R := q.F("r", 7) * b2f(c.ev, 1.25, 1)
			sp := q.F("sp", 360)
			mk := func(a float64) {
				w.pj(c, &Proj{X: w.P.X, Y: w.P.Y, VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, Type: c.t, D: d, P: pp, L: q.F("l", 1.2), R: R, Ex: ex})
			}
			if q.S("mode") == "fan" {
				a0 := w.aimA()
				for i := 0; i < n; i++ {
					mk(a0 + (float64(i)-float64(n-1)/2)*q.F("spr", .22))
				}
				return
			}
			ts := w.near(w.P.X, w.P.Y, n, nil, 560)
			if len(ts) == 0 {
				mk(w.faceA())
				return
			}
			for i := 0; i < n; i++ {
				e := ts[i%len(ts)]
				a := math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
				if i >= len(ts) {
					sg := -1.0
					if i%2 == 1 {
						sg = 1
					}
					a += float64(i-len(ts)+1) * .2 * sg
				}
				mk(a)
			}
		},
		"nova": func(w *World, c mc) {
			q := c.q
			fat := q.F("fat", 0) > 0
			kb := 1.0
			if q.F("kbx", 0) > 0 {
				kb = 2
			}
			mk := func(mu float64) {
				f := w.ring(c, w.P.X, w.P.Y, (q.F("r0", 100)+q.F("rp", 0)*c.sl)*w.rgm()*b2f(c.ev, 1.15, 1), 12*(1+.2*w.pe("wide")), (q.F("d", 0)+q.F("dp", 0)*c.sl)*c.em*mu, b2f(fat, .4, .6), fat, kb)
				f.R = 10
				if fat {
					w.Shake = math.Max(w.Shake, .2)
				}
			}
			mk(1)
			if c.ev {
				w.at(c, .35, func() { mk(.6) })
				if c.cap {
					w.at(c, .7, func() { mk(.6) })
				}
			}
		},
		"trail": func(w *World, c mc) {
			q := c.q
			if w.P.Moving {
				l := (q.F("l", 0) + q.F("lp", 0)*c.sl) * w.rgm() * (1 + .2*w.pe("heat")) * b2f(c.ev, 1.5, 1)
				w.ZN = append(w.ZN, &Zone{X: w.P.X, Y: w.P.Y, R: (q.F("r", 0) + q.F("rp", 0)*c.sl) * (1 + .2*w.pe("wide")) * b2f(c.ev, 1.3, 1), L: l, Life0: l, Type: c.t, D: (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em, Ev: c.ev, Cap: c.cap, Slot: c.s})
			}
		},
		"cone": func(w *World, c mc) {
			q := c.q
			R := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(c.ev, 1.2, 1)
			hw := q.F("hw", .45) * b2f(c.ev, 1.25, 1)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			du := q.F("du", 1) * b2f(c.ev, 1.4, 1)
			tk := q.F("tk", .12)
			n := int(math.Max(1, math.Round(du/tk)))
			tick := func() {
				a0 := w.faceA()
				if e := w.near1(w.P.X, w.P.Y, R*1.4); e != nil {
					a0 = math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
				}
				w.addFX(&FX{Kind: "arc", X: w.P.X, Y: w.P.Y, A: a0, Wd: hw, R: R, Life: tk * 1.3, Type: c.t, Slot: c.s})
				w.sectorHit(w.P.X, w.P.Y, a0, hw, R, func(e *Enemy, dx, dy, dd float64) {
					w.hit(e, d, c.t, 0, 0, 0)
					if q.F("kbx", 0) > 0 && !e.Boss {
						e.KX, e.KY = clampK(e.KX+dx/dd*90), clampK(e.KY+dy/dd*90)
					}
				})
			}
			for i := 0; i < n; i++ {
				w.at(c, float64(i)*tk, tick)
			}
			if c.cap {
				for i := 0; i < n; i++ {
					w.at(c, du+.25+float64(i)*tk, tick)
				}
			}
		},
		"sweep": func(w *World, c mc) {
			q := c.q
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			wd := q.F("w", 46) * b2f(c.ev, 1.35, 1) * (1 + .2*w.pe("wide"))
			go_ := func(dir float64) {
				a := dir + w.faceA()
				if e := w.near1(w.P.X, w.P.Y, 700); e != nil {
					a = dir + math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
				}
				co, si := math.Cos(a), math.Sin(a)
				cx, cy := w.P.X, w.P.Y
				const span, d0 = 1500.0, 650.0
				steps := int(math.Ceil(2 * d0 / q.F("sp", 560) / .05))
				hs := map[*Enemy]bool{}
				for i := 0; i <= steps; i++ {
					u := -d0 + float64(i)/float64(steps)*2*d0
					w.at(c, float64(i)*.05, func() {
						wx, wy := cx+co*u, cy+si*u
						w.addFX(&FX{Kind: "rect", X: wx + si*span/2, Y: wy - co*span/2, A: a + math.Pi/2, Ln: span, Wd: wd, Life: .09, Type: c.t, Slot: c.s})
						for _, e := range w.E {
							if e.Dead || hs[e] {
								continue
							}
							dx, dy := e.X-wx, e.Y-wy
							if math.Abs(dx*co+dy*si) > wd+e.R || math.Abs(-dx*si+dy*co) > span/2 {
								continue
							}
							hs[e] = true
							w.hit(e, d, c.t, 0, 0, 0)
							if !e.Boss {
								kf := b2f(e.R >= 16, .6, 1)
								e.KX, e.KY = clampK(e.KX+co*q.F("push", 160)*kf), clampK(e.KY+si*q.F("push", 160)*kf)
							}
						}
					})
				}
			}
			go_(0)
			if c.cap {
				w.at(c, 1.3, func() { go_(math.Pi) })
			}
		},
		"chain": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 3)) + b2i(c.lv >= 5) + b2i(c.lv >= 10) + w.gv() + 2*b2i(c.ev) + 2*b2i(c.cap)
			dec := q.F("dec", .9)
			if c.ev {
				dec = math.Min(.97, dec+.05)
			}
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			cx, cy := w.P.X, w.P.Y
			var ex []*Enemy
			for i := 0; i < n; i++ {
				r := q.F("rng", 260)
				if i > 0 {
					r = q.F("jr", 170)
				}
				l := w.near(cx, cy, 1, ex, r*w.rgm())
				if len(l) == 0 {
					break
				}
				e := l[0]
				w.addFX(&FX{Kind: "bolt", Type: c.t, AX: cx, AY: cy, BX: e.X, BY: e.Y, Life: .18})
				w.hit(e, d, c.t, 1, cx, cy)
				ex = append(ex, e)
				cx, cy = e.X, e.Y
				d *= dec
			}
		},
		"spin": func(w *World, c mc) {
			q := c.q
			R := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(c.ev, 1.25, 1)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			du := q.F("du", 1.4) * b2f(c.ev, 1.3, 1)
			tk := q.F("tk", .2)
			n := int(math.Max(1, math.Round(du/tk)))
			for i := 0; i < n; i++ {
				w.at(c, float64(i)*tk, func() {
					w.addFX(&FX{Kind: "arc", X: w.P.X, Y: w.P.Y, A: w.T * 14, Wd: 2.4, R: R, Life: tk * 1.15, Type: c.t, Slot: c.s})
					for _, e := range w.E {
						if e.Dead {
							continue
						}
						dx, dy := e.X-w.P.X, e.Y-w.P.Y
						dd := hyp(dx, dy)
						if dd > R+e.R {
							continue
						}
						w.hit(e, d, c.t, 0, 0, 0)
						kf := 1.0
						if e.Boss {
							kf = .2
						} else if e.R >= 16 {
							kf = .6
						}
						if dd == 0 {
							dd = 1
						}
						e.KX, e.KY = clampK(e.KX+dx/dd*q.F("ks", 60)*kf), clampK(e.KY+dy/dd*q.F("ks", 60)*kf)
					}
				})
			}
			if c.cap {
				w.at(c, du, func() { f := w.ring(c, w.P.X, w.P.Y, R*1.8, 0, d*2, .35, true, 2); f.R = 8 })
			}
		},
		"mine": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 1)) + b2i(c.lv >= 5) + b2i(c.lv >= 10) + b2i(c.ev) + b2i(c.cap)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			br := (q.F("br", 58) + q.F("brp", 4)*c.sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(c.ev, 1.25, 1)
			var own []*Zone
			for _, z := range w.ZN {
				if z.Mine && z.Slot == c.s && z.L > 0 {
					own = append(own, z)
				}
			}
			if mx := int(q.F("max", 8)); len(own)+n > mx {
				for _, z := range own[:len(own)+n-mx] {
					z.L = 0
				}
			}
			for i := 0; i < n; i++ {
				a := w.rnd(0, 6.283)
				rr := w.rnd(0, 14)
				if i > 0 {
					rr = w.rnd(24, 64)
				}
				w.ZN = append(w.ZN, &Zone{X: w.P.X + math.Cos(a)*rr, Y: w.P.Y + math.Sin(a)*rr, R: q.F("r", 14), L: q.F("l", 10), Life0: q.F("l", 10), Type: c.t, Tk: 9e9,
					Mine: true, Arm: b2f(c.cap, 0, q.F("arm", .5)), Br: br, Md: d, Slot: c.s})
			}
		},
		"rupture": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 5)) + b2i(c.lv >= 5) + b2i(c.lv >= 10) + 2*b2i(c.ev)
			r := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(c.ev, 1.2, 1)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			gap := q.F("gap", 44) * w.rgm()
			line := func(dir float64) {
				a := dir + w.faceA()
				if e := w.near1(w.P.X, w.P.Y, 600); e != nil {
					a = dir + math.Atan2(e.Y-w.P.Y, e.X-w.P.X)
				}
				co, si, ox, oy := math.Cos(a), math.Sin(a), w.P.X, w.P.Y
				for i := 0; i < n; i++ {
					x, y := ox+co*(36+float64(i)*gap), oy+si*(36+float64(i)*gap)
					w.at(c, float64(i)*q.F("stp", .07), func() {
						w.addFX(&FX{Kind: "puff", X: x, Y: y, R: r, Life: .28, Type: c.t})
						w.Shake = math.Max(w.Shake, .05)
						for _, o := range w.E {
							if !o.Dead && hyp(o.X-x, o.Y-y) < r+o.R {
								w.hit(o, d, c.t, 1, x, y)
							}
						}
					})
				}
			}
			line(0)
			if c.cap {
				line(math.Pi)
			}
		},
		"jab": func(w *World, c mc) {
			q := c.q
			n := w.cn3(c)
			R := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * b2f(c.ev, 1.25, 1)
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			for _, e := range w.near(w.P.X, w.P.Y, n, nil, R) {
				e := e
				w.addFX(&FX{Kind: "bolt", Type: c.t, AX: w.P.X, AY: w.P.Y, BX: e.X, BY: e.Y, Life: .1})
				w.hit(e, d, c.t, 1, w.P.X, w.P.Y)
				if c.ev {
					w.at(c, .08, func() {
						if !e.Dead {
							w.hit(e, d*.5, c.t, 0, 0, 0)
						}
					})
				}
			}
		},
		"burst": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 5)) + int(math.Round(c.sl/2)) + w.gv() + 3*b2i(c.ev) + 3*b2i(c.cap)
			a0 := w.aimA()
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			sp := q.F("sp", 480)
			for i := 0; i < n; i++ {
				w.at(c, float64(i)*q.F("gap", .055), func() {
					w.pj(c, &Proj{X: w.P.X, Y: w.P.Y, VX: math.Cos(a0) * sp, VY: math.Sin(a0) * sp, Type: c.t, D: d, P: 1 + b2i(c.ev), L: .9, R: 5})
				})
			}
		},
		"twin": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 2)) + b2i(c.lv >= 7) + b2i(c.lv >= 10) + w.gv() + b2i(c.ev) + b2i(c.cap)
			a0 := w.aimA()
			px, py := -math.Sin(a0), math.Cos(a0)
			sp := q.F("sp", 420)
			for i := 0; i < n; i++ {
				off := (float64(i) - float64(n-1)/2) * 13
				w.pj(c, &Proj{X: w.P.X + px*off, Y: w.P.Y + py*off, VX: math.Cos(a0) * sp, VY: math.Sin(a0) * sp, Type: c.t, D: (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em,
					P: int(q.F("p", 2)) + b2i(c.ev), L: q.F("l", .9), R: q.F("r", 6) * b2f(c.ev, 1.25, 1)})
			}
		},
		"sludge": func(w *World, c mc) {
			q := c.q
			n := int(q.F("n0", 4)) + int(math.Round(c.sl/2)) + w.gv() + 3*b2i(c.ev) + 3*b2i(c.cap)
			a0 := w.aimA()
			ex := 0.0
			if c.ev {
				ex = b2f(c.cap, 1.5, 1)
			}
			for i := 0; i < n; i++ {
				a := w.rnd(0, 6.283)
				if i%2 == 0 {
					a = a0 + w.rnd(-.45, .45)
				}
				sp := q.F("sp", 230) * w.rnd(.7, 1.1)
				w.pj(c, &Proj{X: w.P.X, Y: w.P.Y, VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, Type: c.t, D: (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em,
					P: int(q.F("p", 1)) + b2i(c.ev), L: q.F("l", .55), R: q.F("r", 8) * b2f(c.ev, 1.3, 1), Ex: ex})
			}
		},
		"blast": func(w *World, c mc) {
			q := c.q
			a0 := w.aimWithin(q.F("off", 52) + q.F("r", 58)*2.2 + 40)
			R := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(c.ev, 1.25, 1)
			off := q.F("off", 52) * b2f(c.ev, 1.15, 1) * w.rgm()
			d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em
			co, si := math.Cos(a0), math.Sin(a0)
			bm := func(mu float64) {
				x, y := w.P.X+co*off, w.P.Y+si*off
				w.addFX(&FX{Kind: "puff", X: x, Y: y, R: R, Life: .3, Type: c.t})
				w.Shake = math.Max(w.Shake, .15)
				for _, o := range w.E {
					if !o.Dead && hyp(o.X-x, o.Y-y) < R+o.R {
						w.hit(o, d*mu, c.t, 2, x, y)
					}
				}
			}
			tl := q.F("tele", .28)
			w.addFX(&FX{Kind: "tele", X: w.P.X + co*off, Y: w.P.Y + si*off, R: R, Life: tl, Type: c.t})
			w.at(c, tl, func() {
				bm(1)
				if c.ev {
					w.at(c, .3, func() { bm(.6) })
				}
				if c.cap {
					w.at(c, .6, func() { bm(.6) })
				}
			})
		},
	}
}

func (w *World) auraR(s *Slot) float64 {
	q := s.Q
	sl := 1 + float64(s.Lv-1)*.45
	return (q.F("r", 0) + q.F("rp", 0)*sl) * w.rgm() * (1 + .1*w.pe("wide")) * b2f(s.Evo, 1.25, 1) * b2f(s.Evo && s.Lv >= 20, 1.15, 1)
}

// swing: one melee arc (web swingAt).
func (w *World) swing(c mc, a0, mu float64) {
	q := c.q
	sh := q.S("shape")
	R := (q.F("r", 0) + q.F("rp", 0)*c.sl) * w.rgm() * (1 + .1*w.pe("wide"))
	if c.ev {
		R *= b2f(sh == "360", 1.25, 1.2)
	}
	hw := (q.F("hw", 0) + q.F("hp", 0)*c.sl) * b2f(c.ev, 1.25, 1)
	if sh == "rect" {
		hw *= 1 + .1*w.pe("wide")
	} else {
		hw = math.Min(math.Pi, hw)
	}
	d := (q.F("d", 0) + q.F("dp", 0)*c.sl) * c.em * mu
	ks := (q.F("ks", 0) + q.F("kp", 0)*c.sl) * b2f(c.ev, 1.3, 1)
	ox, oy := w.P.X, w.P.Y
	co, si := math.Cos(a0), math.Sin(a0)
	if sh == "rect" {
		w.addFX(&FX{Kind: "rect", X: ox, Y: oy, A: a0, Ln: R, Wd: hw, Life: .22, Type: c.t, Slot: c.s})
	} else {
		w.addFX(&FX{Kind: "arc", X: ox, Y: oy, A: a0, Wd: hw, R: R, Life: .22, Type: c.t, Slot: c.s})
	}
	ls := q.F("ls", 0)
	for _, e := range w.E {
		if e.Dead {
			continue
		}
		dx, dy := e.X-ox, e.Y-oy
		dd := hyp(dx, dy)
		if sh == "rect" {
			u, v := dx*co+dy*si, -dx*si+dy*co
			if u < -e.R || u > R+e.R || math.Abs(v) > hw+e.R {
				continue
			}
		} else {
			if dd > R+e.R {
				continue
			}
			if sh != "360" {
				da := math.Atan2(dy, dx) - a0
				da = math.Abs(math.Atan2(math.Sin(da), math.Cos(da)))
				if da > hw+math.Atan2(e.R, math.Max(dd, 1)) {
					continue
				}
			}
		}
		if cf := q.F("cf", 0); cf > 0 {
			e.Conf = math.Max(e.Conf, (cf+.08*c.sl)*bossK(e))
			w.addFX(&FX{Kind: "txt", X: e.X, Y: e.Y - e.R, Text: "?", Col: "conf", Life: .5})
			continue
		}
		w.hit(e, d, c.t, 0, 0, 0)
		if ls > 0 {
			w.healCapped(lastD * ls)
		}
		if dd == 0 {
			dd = 1
		}
		kf := e.kbFactor()
		e.KX, e.KY = clampK(e.KX+dx/dd*ks*kf), clampK(e.KY+dy/dd*ks*kf)
	}
}

func (w *World) rectHit(ox, oy, a, L, wd float64, f func(*Enemy)) {
	co, si := math.Cos(a), math.Sin(a)
	for _, o := range w.E {
		if o.Dead {
			continue
		}
		dx, dy := o.X-ox, o.Y-oy
		u, v := dx*co+dy*si, -dx*si+dy*co
		if u < -o.R || u > L+o.R || math.Abs(v) > wd+o.R {
			continue
		}
		f(o)
	}
}

func (w *World) sectorHit(ox, oy, a0, hw, R float64, f func(e *Enemy, dx, dy, dd float64)) {
	for _, e := range w.E {
		if e.Dead {
			continue
		}
		dx, dy := e.X-ox, e.Y-oy
		dd := hyp(dx, dy)
		if dd > R+e.R {
			continue
		}
		da := math.Atan2(dy, dx) - a0
		da = math.Abs(math.Atan2(math.Sin(da), math.Cos(da)))
		if da > hw+math.Atan2(e.R, math.Max(dd, 1)) {
			continue
		}
		if dd == 0 {
			dd = 1
		}
		f(e, dx, dy, dd)
	}
}
