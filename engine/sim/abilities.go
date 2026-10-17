package sim

import (
	"math"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// PS is the active ability's parameters (web PS). Every effect is data in abilities.json; the hooks
// below read it at the same points the web game does.
type PS struct {
	A content.Ability
	m map[string]any
}

func (p *PS) F(k string) float64 {
	if p == nil {
		return 0
	}
	switch v := p.m[k].(type) {
	case float64:
		return v
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

func (p *PS) Has(k string) bool {
	if p == nil {
		return false
	}
	_, ok := p.m[k]
	return ok
}

// L returns a list parameter as numbers (non-numbers are 0).
func (p *PS) L(k string) []float64 {
	l, _ := p.m[k].([]any)
	out := make([]float64, len(l))
	for i, x := range l {
		out[i], _ = x.(float64)
	}
	return out
}

// Strs returns a list parameter as strings.
func (p *PS) Strs(k string) []string {
	l, _ := p.m[k].([]any)
	var out []string
	for _, x := range l {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (p *PS) S(k string) string {
	s, _ := p.m[k].(string)
	return s
}

// Map returns a {type: number} parameter.
func (p *PS) Map(k string) map[string]float64 {
	m, _ := p.m[k].(map[string]any)
	out := map[string]float64{}
	for a, b := range m {
		out[a], _ = b.(float64)
	}
	return out
}

func newPS(a content.Ability) *PS { return &PS{A: a, m: a.P} }

// abState: per-run ability counters (web P.* fields used by the hooks).
type abState struct {
	winT, winOn    float64
	ksT, kbT, hdT  float64
	ksN, kbN, kpN  float64
	dfT, dfN       float64
	stkT, stkN     float64
	hsT, hgT, trT  float64
	agT, hbT, sdT  float64
	chT, hvT, shT  float64
	auT, peT, pt   float64
	nse, nbl       int
	pcon, rv       bool
	kbst           float64
	techT          float64
	tech           map[*Slot]bool
	inAb           bool
	hvInit, shInit bool
}

func (w *World) abLoc() bool { return has(w.PS.Strs("loc"), w.Biome) }

func (w *World) abWin() string {
	l := w.PS.m["win"]
	if l == nil || w.ab.winOn <= w.T {
		return ""
	}
	a, _ := l.([]any)
	if len(a) < 3 {
		return ""
	}
	s, _ := a[2].(string)
	return s
}

func (w *World) abDmgAdd() float64 {
	ps, a := w.PS, &w.ab
	s := 0.0
	switch w.abWin() {
	case "psy":
		s += .3
	case "aspect":
		s += .4
	}
	if l := ps.L("kStack"); len(l) == 3 && a.ksT > w.T {
		s += l[0] * a.ksN
	}
	if l := ps.L("kStackBurn"); len(l) == 3 && a.kbT > w.T {
		s += l[0] * a.kbN
	}
	if l := ps.L("kPerm"); len(l) == 2 {
		s += math.Min(l[1], l[0]*a.kpN)
	}
	if l := ps.L("hurtDmg"); len(l) == 2 && a.hdT > w.T {
		s += l[0]
	}
	if l := ps.L("hurtStack"); len(l) == 3 && a.dfT > w.T {
		s += l[0] * a.dfN
	}
	if ps.Has("trace") && a.trT > w.T {
		s += .15
	}
	if a.pcon {
		s += .2
	}
	s += ps.F("plus") // + per co-op ally nearby (solo: none)
	return s
}

// psD: the ability's share of the damage multiplier (web psD).
func (w *World) psD() float64 {
	ps := w.PS
	h := w.P.HP / w.P.MHP
	lowT := ps.F("dmgLowT")
	if lowT == 0 {
		lowT = .35
	}
	m := 1 + ps.F("dmg") + w.abDmgAdd()
	if h < lowT {
		m *= 1 + ps.F("dmgLow")
	}
	if h > .9 {
		m *= 1 + ps.F("dmgFull")
	}
	if w.P.Moving {
		m *= 1 + ps.F("dmgMove")
	} else {
		m *= 1 + ps.F("dmgStill")
	}
	return m
}

func (w *World) abSpd() float64 {
	ps, a := w.PS, &w.ab
	s := ps.F("spd")
	if w.abLoc() {
		s += ps.F("lSpd")
	}
	if w.abWin() == "rain" {
		s += .15
	}
	if l := ps.L("hurtSpd"); len(l) == 2 && a.hsT > w.T {
		s += l[0]
	}
	if q := ps.F("qfeet"); q > 0 && (w.P.WebT > 0 || w.P.Hazard != "") {
		s += q
	}
	if ps.Has("trace") && a.trT > w.T {
		s += .15
	}
	return s
}

func (w *World) abCd() float64 {
	ps := w.PS
	s := ps.F("cd")
	if w.abWin() == "elec" {
		s += .23
	}
	lowT := ps.F("dmgLowT")
	if lowT == 0 {
		lowT = .35
	}
	if c := ps.F("cdLow"); c > 0 && w.P.HP < w.P.MHP*lowT {
		s += c
	}
	if ps.Has("trace") && w.ab.trT > w.T {
		s += .1
	}
	return s
}

func (w *World) abReg() float64 {
	s := w.PS.F("reg")
	if w.abLoc() {
		s += w.PS.F("lReg")
	}
	if w.abWin() == "rain" {
		s += w.P.MHP * .01
	}
	return s
}

func (w *World) abDodge() float64 {
	s := w.PS.F("dodge")
	if w.abLoc() {
		s += w.PS.F("lDodge")
	}
	if l := w.PS.L("hurtDodge"); len(l) == 2 && w.ab.hgT > w.T {
		s += l[0]
	}
	return math.Min(.6, s)
}

func (w *World) abSlowImm() bool { return w.PS.F("slowImm") > 0 || w.abWin() == "mist" }

func (w *World) abHzImm(kind string) bool {
	z := w.PS.Strs("hzImm")
	return has(z, "all") || has(z, kind)
}

func (w *World) abGustImm() bool { return w.PS.F("kbImm") > 0 || w.abHzImm("gust") }

func (w *World) abBase() float64 { return 10 + w.T/25 }

// abHitMul: per-hit multiplier from the ability (web abHitMul).
func (w *World) abHitMul(e *Enemy, t string, em float64) float64 {
	ps := w.PS
	m := 1.0
	m += ps.Map("tdm")[t]
	switch w.abWin() {
	case "sun":
		if t == "fire" {
			m += .3
		}
	case "rain":
		if t == "water" {
			m += .2
		}
	}
	if w.abLoc() {
		m += ps.Map("lTdm")[t]
	}
	m += ps.Map("vsT")[e.Type]
	if v := ps.F("vsElite"); v > 0 && (e.Elite || e.Boss) {
		m += v
	}
	if v := ps.F("seB"); v > 0 && em > 1.15 {
		m += v
	}
	if v := ps.F("stab"); v > 0 && has(w.P.Types, t) {
		m *= (1.5 + v) / 1.5
	}
	if v := ps.F("tech"); v > 0 && w.curSlot != nil {
		if w.ab.techT != w.T {
			w.ab.techT = w.T
			w.ab.tech = map[*Slot]bool{}
			if len(w.Slots) > 2 {
				l := append([]*Slot{}, w.Slots...)
				for i := 0; i < len(l); i++ {
					for j := i + 1; j < len(l); j++ {
						if l[j].Lv < l[i].Lv {
							l[i], l[j] = l[j], l[i]
						}
					}
				}
				w.ab.tech[l[0]], w.ab.tech[l[1]] = true, true
			}
		}
		if w.ab.tech[w.curSlot] {
			m += v
		}
	}
	return m
}

func (w *World) abEm(em float64) float64 {
	if n := int(w.PS.F("nthSE")); n > 0 {
		w.ab.nse++
		if w.ab.nse >= n {
			w.ab.nse = 0
			if em <= 1.15 {
				em = 2
			}
		}
	}
	if w.PS.F("noRes") > 0 && em < 1 {
		em = 1
	}
	return em
}

// abSt: a status applied by an ability.
func (w *World) abSt(e *Enemy, k string, dur float64) {
	if e == nil || e.Dead {
		return
	}
	switch k {
	case "bn":
		if dur == 0 {
			dur = 3
		}
		e.Burn = math.Max(e.Burn, dur)
		e.BurnD = math.Max(e.BurnD, 1.2+w.abBase()*.15)
	case "ps":
		if dur == 0 {
			dur = 4
		}
		e.Psn = math.Max(e.Psn, dur)
		e.PsnD = math.Max(e.PsnD, (1.2+w.abBase()*.12)*b2f(w.PS.F("corr") > 0, 1.3, 1))
		if w.PS.F("puppet") > 0 && !e.Boss {
			e.Conf = math.Max(e.Conf, 1.5)
		}
	case "sl":
		if dur == 0 {
			dur = 1.2
		}
		e.Slow = math.Max(e.Slow, dur)
	case "cm":
		if !e.Boss {
			if dur == 0 {
				dur = 2
			}
			e.Charm = math.Max(e.Charm, dur)
		}
	default:
		if dur == 0 {
			dur = 1
		}
		w.applyStatus(e, k, dur, 0)
	}
}

func (w *World) serene() float64 { return b2f(w.PS.F("serene") > 0, 2, 1) }

// abProc: on-hit procs (web abProc).
func (w *World) abProc(e *Enemy, t string, d float64) {
	ps := w.PS
	if w.ab.inAb || ps.F("sheer") > 0 {
		return
	}
	w.ab.inAb = true
	defer func() { w.ab.inAb = false }()
	sg := w.serene()
	r := w.rng.Float64
	if v := ps.F("pPsn"); v > 0 && r() < v*sg {
		w.abSt(e, "ps", 4)
	}
	if v := ps.F("pFrz"); v > 0 && r() < v*sg {
		w.abSt(e, "fz", 1)
	}
	if v := ps.F("pStun"); v > 0 && r() < v*sg {
		w.abSt(e, "su", .5)
	}
	if v := ps.F("pSleep"); v > 0 && r() < v*sg {
		w.abSt(e, "su", 2)
	}
	if c := ps.L("chain"); len(c) == 3 && r() < c[0] {
		px, py := e.X, e.Y
		done := []*Enemy{e}
		for i := 0; i < int(c[1]); i++ {
			b := w.near(px, py, 1, done, 170)
			if len(b) == 0 {
				break
			}
			done = append(done, b[0])
			w.addFX(&FX{Kind: "bolt", Type: "electric", AX: px, AY: py, BX: b[0].X, BY: b[0].Y, Life: .18})
			w.hitRaw(b[0], math.Max(1, d*c[2]), "")
			px, py = b[0].X, b[0].Y
		}
	}
	if n := int(ps.F("nthBlast")); n > 0 {
		w.ab.nbl++
		if w.ab.nbl >= n {
			w.ab.nbl = 0
			w.addFX(&FX{Kind: "pv", X: e.X, Y: e.Y, R: 70, Life: .3, Type: "fire"})
			for _, q := range append([]*Enemy{}, w.E...) {
				if !q.Dead && (q.X-e.X)*(q.X-e.X)+(q.Y-e.Y)*(q.Y-e.Y) < 70*70 {
					w.hitRaw(q, math.Max(1, d*.45), "burn")
				}
			}
		}
	}
}

// hurtSrc describes where damage came from (web s_ / HURTC / HURTP).
type hurtSrc struct {
	t                    string // damage type
	contact, proj, elite bool
	src                  *DmgSrc
}

// abRed: damage-taken multiplier; 0 when absorbed (web abRed).
func (w *World) abRed(d float64, s hurtSrc, tm float64) float64 {
	ps := w.PS
	m := 1.0
	if at := ps.S("absorbT"); at != "" && s.t == at && !s.contact {
		w.heal(d * .25)
		return 0
	}
	if v := ps.Map("redT")[s.t]; v > 0 && s.t != "" {
		m *= 1 - v
	}
	if v := ps.F("redSE"); v > 0 && tm > 1.15 {
		m *= 1 - v
	}
	if v := ps.F("redLow"); v > 0 && w.P.HP < w.P.MHP*.5 {
		m *= 1 - v
	}
	if v := ps.F("redFull"); v > 0 {
		cd := ps.F("redFullCD")
		if w.P.HP >= w.P.MHP*.999 || (cd > 0 && w.T-w.P.HurtT >= cd) {
			m *= 1 - v
		}
	}
	if l := ps.L("hurtRedStack"); len(l) == 3 && w.ab.stkT > w.T {
		m *= 1 - math.Min(.5, l[0]*w.ab.stkN)
	}
	if s.proj {
		if v := ps.F("projDR"); v > 0 {
			m *= 1 - v
		}
		if w.abWin() == "mist" {
			m *= .5
		}
	}
	if v := ps.F("intim"); v > 0 && s.contact {
		m *= 1 - v
	}
	if v := ps.F("redElite"); v > 0 && s.elite {
		m *= 1 - v
	}
	return m
}

// abHurt: reactions to taking damage dt (web abHurt).
func (w *World) abHurt(dt float64) {
	if dt <= .6 {
		return
	}
	ps, a, P := w.PS, &w.ab, w.P
	if l := ps.L("hurtSpd"); len(l) == 2 {
		a.hsT = w.T + l[1]
	}
	if l := ps.L("hurtDodge"); len(l) == 2 {
		a.hgT = w.T + l[1]
	}
	if l := ps.L("hurtDmg"); len(l) == 2 {
		a.hdT = w.T + l[1]
	}
	if l := ps.L("hurtStack"); len(l) == 3 {
		if a.dfT > w.T {
			a.dfN = math.Min(l[2], a.dfN+1)
		} else {
			a.dfN = 1
		}
		a.dfT = w.T + l[1]
	}
	if l := ps.L("hurtRedStack"); len(l) == 3 {
		if a.stkT > w.T {
			a.stkN = math.Min(l[2], a.stkN+1)
		} else {
			a.stkN = 1
		}
		a.stkT = w.T + l[1]
	}
	if ps.Has("angerPt") && dt > P.MHP*.15 {
		a.agT = w.T + 4
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 46, Life: 1.5, Text: "Anger Point!", Col: "crit"})
	}
	if hb := ps.S("hurtBurst"); hb != "" && w.T-a.hbT > 2.5 {
		a.hbT = w.T
		tox := hb == "toxic"
		w.addFX(&FX{Kind: "pv", X: P.X, Y: P.Y, R: 110, Life: .35, Type: b2s(tox, "poison", "ground")})
		for _, e := range append([]*Enemy{}, w.E...) {
			if !e.Dead && (e.X-P.X)*(e.X-P.X)+(e.Y-P.Y)*(e.Y-P.Y) < 110*110 {
				w.hitRaw(e, w.abBase()*.45, "")
				if !e.Dead {
					w.abSt(e, b2s(tox, "ps", "sl"), b2f(tox, 4, 1.5))
				}
			}
		}
	}
	if ps.Has("pcon") && !a.pcon && P.HP > 0 && P.HP < P.MHP*.5 {
		w.abPcon()
	}
}

func (w *World) abPcon() {
	P := w.P
	w.ab.pcon = true
	P.MHP *= 2
	P.HP = P.MHP
	P.Inv = math.Max(P.Inv, 1.5)
	w.Shake = .4
	w.addFX(&FX{Kind: "flash", Life: .3})
	w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 60, Life: 3, Text: "Power Construct! Complete Forme!", Col: "gold"})
}

// abSave: last-chance saves (Sturdy, Power Construct).
func (w *World) abSave() bool {
	ps, P := w.PS, w.P
	if ps.Has("pcon") && !w.ab.pcon {
		P.HP = 1
		w.abPcon()
		return true
	}
	if v := ps.F("sturdy"); v > 0 && w.T >= w.ab.sdT {
		P.HP = 1
		w.ab.sdT = w.T + v
		P.Inv = math.Max(P.Inv, 1.5)
		w.Shake = .3
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 50, Life: 2, Text: "Sturdy!", Col: "gold"})
		return true
	}
	return false
}

// abContact: an enemy just touched the player.
func (w *World) abContact(e *Enemy) {
	if e.Dead {
		return
	}
	ps := w.PS
	if c, ok := ps.m["cSt"].([]any); ok && len(c) == 3 {
		k, _ := c[0].(string)
		p, _ := c[1].(float64)
		dur, _ := c[2].(float64)
		if w.rng.Float64() < p {
			multi := k == "multi"
			if multi {
				k = []string{"ps", "su", "su"}[w.rng.Intn(3)]
			}
			if k == "su" && multi {
				dur = 1.5
			}
			w.abSt(e, k, dur)
		}
	}
	if ps.Has("perish") && !e.Boss && e.pb == 0 {
		e.pb = 4
	}
	if ps.Has("pecks") {
		dx, dy := e.X-w.P.X, e.Y-w.P.Y
		l := math.Max(hyp(dx, dy), 1)
		ks := b2f(e.Boss, 60, 260)
		e.KX += dx / l * ks
		e.KY += dy / l * ks
	}
}

// abProj: an enemy projectile reached the player; true = negated.
func (w *World) abProj(b *EBullet) bool {
	ps, P := w.PS, w.P
	if has(ps.Strs("projAbsorb"), b.Type) && b.Type != "" {
		w.heal(P.MHP * .01)
		w.addFX(&FX{Kind: "puff", X: b.X, Y: b.Y, R: 12, Life: .25, Type: b.Type})
		return true
	}
	if v := ps.F("projDeflect"); v > 0 && w.rng.Float64() < v {
		w.addFX(&FX{Kind: "puff", X: b.X, Y: b.Y, R: 12, Life: .25, Type: "flying"})
		return true
	}
	if v := ps.F("projReflect"); v > 0 && w.rng.Float64() < v {
		if q := w.near1(P.X, P.Y, 320); q != nil {
			w.addFX(&FX{Kind: "bolt", Type: "psychic", AX: P.X, AY: P.Y, BX: q.X, BY: q.Y, Life: .2})
			w.hitRaw(q, math.Max(w.abBase(), b.Dm*3), "")
		}
		return true
	}
	return false
}

// abKill: kill hooks (web abKill + pKill).
func (w *World) abKill(e *Enemy) {
	ps, a, P := w.PS, &w.ab, w.P
	if v := ps.F("kheal"); v > 0 {
		w.heal(P.MHP * v)
	}
	if ps.F("kspd") > 0 {
		a.kbst = 2
	}
	if l := ps.L("kStack"); len(l) == 3 {
		if a.ksT > w.T {
			a.ksN = math.Min(l[2], a.ksN+1)
		} else {
			a.ksN = 1
		}
		a.ksT = w.T + l[1]
	}
	if l := ps.L("kStackBurn"); len(l) == 3 && e.Burn > 0 {
		if a.kbT > w.T {
			a.kbN = math.Min(l[2], a.kbN+1)
		} else {
			a.kbN = 1
		}
		a.kbT = w.T + l[1]
	}
	if ps.Has("kPerm") {
		a.kpN++
	}
	if v := ps.F("pickCoin"); v > 0 && w.rng.Float64() < v {
		w.Gems = append(w.Gems, &Gem{X: e.X + 6, Y: e.Y - 4, C: 1})
	}
	if ps.F("eliteCoin") > 0 && e.Elite && !e.Boss {
		for i := 0; i < 3; i++ {
			w.Gems = append(w.Gems, &Gem{X: e.X + w.rnd(-20, 20), Y: e.Y + w.rnd(-20, 20), C: 2})
		}
	}
	if ps.Has("trace") && e.Elite && !e.Boss {
		a.trT = w.T + 30
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 50, Life: 1.5, Text: "Trace!", Col: "conf"})
	}
	if ps.Has("shatter") && e.Frz > 0 && !a.inAb {
		a.inAb = true
		w.addFX(&FX{Kind: "pv", X: e.X, Y: e.Y, R: 80, Life: .3, Type: "ice"})
		for _, q := range append([]*Enemy{}, w.E...) {
			if !q.Dead && q != e && (q.X-e.X)*(q.X-e.X)+(q.Y-e.Y)*(q.Y-e.Y) < 80*80 {
				w.hitRaw(q, w.abBase()*1.2, "")
			}
		}
		a.inAb = false
	}
}

var winText = map[string]string{"sun": "Harsh sunlight!", "rain": "Rain!", "elec": "Electric Terrain!", "psy": "Psychic Terrain!", "mist": "Misty Terrain!", "aspect": "Embody Aspect!"}
var winType = map[string]string{"sun": "fire", "rain": "water", "elec": "electric", "psy": "psychic", "mist": "fairy", "aspect": "grass"}

// abTick: per-frame ability timers (web abTick + pasTick).
func (w *World) abTick(dt float64) {
	ps, a, P := w.PS, &w.ab, w.P
	if a.kbst > 0 {
		a.kbst -= dt
	}
	if l, ok := ps.m["win"].([]any); ok && len(l) == 3 && a.winT <= w.T {
		every, _ := l[0].(float64)
		dur, _ := l[1].(float64)
		k, _ := l[2].(string)
		a.winT, a.winOn = w.T+every, w.T+dur
		w.addFX(&FX{Kind: "pv", X: P.X, Y: P.Y, R: 120, Life: .4, Type: winType[k]})
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 56, Life: 1.2, Text: winText[k], Col: "se"})
	}
	if c, ok := ps.m["chip"].([]any); ok && len(c) == 3 {
		every, _ := c[0].(float64)
		k, _ := c[1].(float64)
		kind, _ := c[2].(string)
		if a.chT == 0 {
			a.chT = every
		}
		a.chT -= dt
		if a.chT <= 0 {
			a.chT = every
			vx, vy := w.ViewW/2+40, w.ViewH/2+40
			hail := kind == "hail"
			for _, e := range append([]*Enemy{}, w.E...) {
				if e.Dead || math.Abs(e.X-P.X) > vx || math.Abs(e.Y-P.Y) > vy {
					continue
				}
				w.hitRaw(e, k*w.abBase(), "")
			}
			for i := 0; i < 14; i++ {
				w.addFX(&FX{Kind: "puff", X: P.X + w.rnd(-vx, vx), Y: P.Y + w.rnd(-vy, vy), R: 6, Life: .3, Type: b2s(hail, "ice", "ground")})
			}
		}
	}
	if v := ps.F("harvest"); v > 0 {
		if !a.hvInit {
			a.hvInit, a.hvT = true, v
		}
		a.hvT -= dt
		if a.hvT <= 0 {
			a.hvT = v
			hasB := false
			for _, it := range w.Items {
				if it.K == "harvest" {
					hasB = true
				}
			}
			if !hasB {
				an := w.rnd(0, 6.283)
				w.Items = append(w.Items, &Pickup{X: P.X + math.Cos(an)*140, Y: P.Y + math.Sin(an)*140, K: "harvest"})
				w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 40, Life: 1.2, Text: "A berry grew!", Col: "se"})
			}
		}
	}
	if v := ps.F("shed"); v > 0 {
		if !a.shInit {
			a.shInit, a.shT = true, v
		}
		a.shT -= dt
		if a.shT <= 0 {
			a.shT = v
			P.WebT = 0
			w.heal(P.MHP * .03)
		}
	}
	if r := math.Max(ps.F("auraBurn"), ps.F("dream")); r > 0 {
		a.auT -= dt
		if a.auT <= 0 {
			a.auT = b2f(ps.F("dream") > 0, 1, .8)
			for _, e := range append([]*Enemy{}, w.E...) {
				if e.Dead || (e.X-P.X)*(e.X-P.X)+(e.Y-P.Y)*(e.Y-P.Y) > r*r {
					continue
				}
				if ps.F("auraBurn") > 0 {
					w.abSt(e, "bn", 2)
				}
				if ps.F("dream") > 0 && (e.Stun > 0 || e.Frz > 0 || e.Slow > 0) {
					w.hitRaw(e, w.abBase()*.6, "")
				}
			}
		}
	}
	if ps.Has("perish") {
		a.peT -= dt
		if a.peT <= 0 {
			a.peT = .25
			for _, e := range w.E {
				if e.pb == 0 || e.Dead {
					continue
				}
				e.pb -= .25
				if e.pb <= 0 {
					e.pb = 0
					if e.HP < e.MHP*.25 {
						w.addFX(&FX{Kind: "puff", X: e.X, Y: e.Y, R: e.R * 1.6, Life: .4, Type: "ghost"})
						w.hitRaw(e, e.HP+1, "")
					}
				}
			}
		}
	}
	if ps.Has("pcon") && !a.pcon && P.HP > 0 && P.HP < P.MHP*.5 {
		w.abPcon()
	}
	// slowing aura
	if r := ps.F("slowA"); r > 0 {
		for _, e := range w.E {
			if e.Dead || e.Boss || (e.X-P.X)*(e.X-P.X)+(e.Y-P.Y)*(e.Y-P.Y) >= r*r {
				continue
			}
			if s := ps.F("slowS"); s > 0 {
				if !(e.aslT > w.T) {
					e.aslS = 0
				}
				e.aslT = w.T + .3
				e.aslS = math.Max(e.aslS, s)
			} else {
				e.Slow = math.Max(e.Slow, .3)
			}
		}
	}
	// periodic pulse / heal
	if pl := ps.L("pulse"); len(pl) == 3 {
		a.pt -= dt
		if a.pt <= 0 {
			a.pt = pl[0]
			if pl[1] > 0 {
				w.addFX(&FX{Kind: "pv", X: P.X, Y: P.Y, R: pl[1], Life: .35, Type: P.Types[0]})
				prev := w.curSlot
				w.curSlot = nil
				for _, e := range append([]*Enemy{}, w.E...) {
					if !e.Dead && (e.X-P.X)*(e.X-P.X)+(e.Y-P.Y)*(e.Y-P.Y) < pl[1]*pl[1] {
						w.hit(e, pl[2]*(10+w.T/25), P.Types[0], 1, P.X, P.Y)
					}
				}
				w.curSlot = prev
			}
			if v := ps.F("healp"); v > 0 {
				w.heal(P.MHP * v)
				w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 30, Life: .8, Text: "+" + itoa(int(P.MHP*v)), Col: "heal"})
			}
		}
	}
}
