package sim

import "math"

// Seismic Toss: grabs the nearest foes (they can't hurt you while held), pulls them in and throws
// them over your shoulder; each lands for damage and hurts everything where it lands.

// Toss is a foe being grabbed and thrown.
type Toss struct {
	Ph             int     // 0 pulled in, 1 held, 2 flying
	T              float64 // time in the phase
	Z              float64 // height while flying (for drawing)
	sx, sy, tx, ty float64
	d, lr          float64
	slot           *Slot
	owner          int
}

const (
	tossPull = .28
	tossHold = .08
	tossFly  = .45
)

func init() {
	QD["toss"] = map[string]any{"cd": 3.2, "n0": 3.0, "r": 150.0, "rp": 9.0, "d": 22.0, "dp": 9.0, "lr": 55.0, "dist": 190.0}
	Implemented["toss"] = true
	shapes["toss"] = func(w *World, c mc) {
		q := c.q
		n := int(q.F("n0", 3)) + w.gv() + 2*b2i(c.ev) + 2*b2i(c.cap)
		R := (q.F("r", 150) + q.F("rp", 9)*c.sl) * w.rgm() * b2f(c.ev, 1.2, 1)
		d := (q.F("d", 22) + q.F("dp", 9)*c.sld) * c.em
		lr := q.F("lr", 55) * (1 + .2*w.pe("wide")) * c.s.BonusSizeMul() * b2f(c.ev, 1.3, 1) * (1 + .04*float64(c.lv-1))
		P := w.P
		picked := 0
		for picked < n {
			var best *Enemy
			bd := R * R
			for _, e := range w.E {
				if e.Dead || e.Boss || e.Ug || e.Toss != nil {
					continue
				}
				if dd := sq(e.X-P.X) + sq(e.Y-P.Y); dd < bd {
					best, bd = e, dd
				}
			}
			if best == nil {
				break
			}
			best.Toss = &Toss{sx: best.X, sy: best.Y, d: d, lr: lr, slot: c.s, owner: w.CUR}
			picked++
		}
		if picked > 0 {
			w.addFX(&FX{Kind: "pop", X: P.X, Y: P.Y, R: R * .5, Life: .3, Type: c.t, Slot: c.s})
		}
	}
}

// stepToss moves a grabbed foe; true while it is still in the toss.
func (w *World) stepToss(e *Enemy, dt float64) {
	ts := e.Toss
	w.useP(ts.owner)
	P := w.P
	ts.T += dt
	switch ts.Ph {
	case 0: // pulled in towards the thrower
		k := math.Min(1, ts.T/tossPull)
		k = k * k
		ax, ay := ts.sx-P.X, ts.sy-P.Y
		l := math.Max(1, hyp(ax, ay))
		hx, hy := P.X+ax/l*18, P.Y+ay/l*18
		e.X, e.Y = ts.sx+(hx-ts.sx)*k, ts.sy+(hy-ts.sy)*k
		if ts.T >= tossPull {
			ts.Ph, ts.T = 1, 0
			td := 190.0 * (1 + .1*ts.slot.bon(BonusArea))
			ts.tx, ts.ty = P.X-ax/l*td, P.Y-ay/l*td // over the shoulder
			ts.sx, ts.sy = e.X, e.Y
		}
	case 1:
		if ts.T >= tossHold {
			ts.Ph, ts.T = 2, 0
			ts.sx, ts.sy = e.X, e.Y
		}
	case 2: // flying
		k := math.Min(1, ts.T/tossFly)
		e.X, e.Y = ts.sx+(ts.tx-ts.sx)*k, ts.sy+(ts.ty-ts.sy)*k
		ts.Z = math.Sin(math.Pi*k) * 70
		if k >= 1 {
			e.Toss = nil
			prev := w.curSlot
			w.curSlot = ts.slot
			w.Shake = math.Max(w.Shake, .12)
			w.addFX(&FX{Kind: "puff", X: e.X, Y: e.Y, R: ts.lr, Life: .35, Type: "fighting"})
			for _, o := range w.E {
				if o != e && !o.Dead && !o.Ug && sq(o.X-e.X)+sq(o.Y-e.Y) < sq(ts.lr+o.R) {
					w.hit(o, ts.d*.6, "fighting", 2, e.X, e.Y)
				}
			}
			w.hit(e, ts.d, "fighting", 0, 0, 0)
			w.curSlot = prev
		}
	}
}

// Tossed reports whether e is in a toss, and its phase / height (for drawing).
func (e *Enemy) Tossed() (bool, int, float64) {
	if e.Toss == nil {
		return false, 0, 0
	}
	return true, e.Toss.Ph, e.Toss.Z
}
