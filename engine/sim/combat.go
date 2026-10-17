package sim

import "math"

// SE[attacker] = types it is strong against (same chart as the web version).
var SE = map[string][]string{
	"fire": {"grass", "ice", "bug", "steel"}, "water": {"fire", "ground", "rock"}, "grass": {"water", "ground", "rock"},
	"electric": {"water", "flying"}, "ground": {"electric", "fire", "poison", "rock", "steel"}, "ice": {"grass", "ground", "flying", "dragon"},
	"fighting": {"normal", "ice", "rock", "dark", "steel"}, "poison": {"grass", "fairy"}, "flying": {"grass", "fighting", "bug"},
	"psychic": {"fighting", "poison"}, "bug": {"grass", "psychic", "dark"}, "rock": {"fire", "ice", "flying", "bug"},
	"ghost": {"psychic", "ghost"}, "dragon": {"dragon"}, "dark": {"psychic", "ghost"}, "steel": {"ice", "rock", "fairy"},
	"fairy": {"fighting", "dragon", "dark"}, "normal": {},
}

func has(l []string, x string) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}

func tStep(a, d string) int {
	if a == "" || d == "" {
		return 0
	}
	if has(SE[a], d) {
		return 1
	}
	if has(SE[d], a) || a == d {
		return -1
	}
	return 0
}

var tmult = map[int]float64{2: 1.5, 1: 1.25, 0: 1, -1: .5, -2: .25}

// TypeMul is the multiplier for attack type a against defender types ds.
func TypeMul(a string, ds []string) float64 {
	n := 0
	for _, d := range ds {
		n += tStep(a, d)
	}
	if n > 2 {
		n = 2
	}
	if n < -2 {
		n = -2
	}
	return tmult[n]
}

func eff(a, d string) float64 {
	m := TypeMul(a, []string{d})
	if m == 1 && a == "normal" {
		return 1.1
	}
	return m
}

// passive type effects on hit
type stEff struct {
	k    string
	p, v float64
}

var typeStatus = map[string]stEff{
	"fire": {"bn", .2, 3}, "dragon": {"bn", .15, 3}, "poison": {"ps", .25, 4}, "water": {"sl", 1, 1.4}, "ground": {"sl", 1, 1},
	"bug": {"sl", 1, 1}, "electric": {"su", .12, .6}, "ice": {"fz", .1, 1}, "grass": {"rt", .12, 1}, "dark": {"fe", .08, 1},
	"fairy": {"cm", .1, 1.2},
}
var knockType = map[string]float64{"rock": 2, "steel": 2, "flying": 1.5, "fighting": 3}

func bossK(e *Enemy) float64 {
	if e.Boss {
		return .5
	}
	return 1
}

func (w *World) applyStatus(e *Enemy, k string, dur, d float64) {
	switch k {
	case "bn":
		e.Burn = math.Max(e.Burn, dur)
		e.BurnD = math.Max(e.BurnD, math.Max(1.5, d*.3))
	case "ps":
		e.Psn = math.Max(e.Psn, dur)
		e.PsnD = math.Max(e.PsnD, math.Max(1.5, d*.22)*b2f(w.PS.F("corr") > 0, 1.3, 1))
		if w.PS.F("puppet") > 0 && !e.Boss {
			e.Conf = math.Max(e.Conf, 1.5)
		}
	case "sl":
		e.Slow = math.Max(e.Slow, dur)
	case "cm":
		if !e.Boss {
			e.Charm = dur
		}
	case "fz":
		e.Frz = math.Max(e.Frz, dur*bossK(e))
	case "su":
		e.Stun = math.Max(e.Stun, dur*bossK(e))
	case "rt":
		e.Root = math.Max(e.Root, dur*bossK(e))
	case "fe":
		e.Fear = math.Max(e.Fear, dur*bossK(e))
	case "cf":
		e.Conf = math.Max(e.Conf, dur*bossK(e))
	}
}

func (e *Enemy) kbFactor() float64 {
	if e.Boss {
		return .25
	}
	if e.R >= 16 {
		return .6
	}
	return 1
}

func clampK(v float64) float64 { return math.Max(-700, math.Min(700, v)) }

// knock pushes an enemy away from (fx,fy) by ks (scaled for big enemies / bosses).
func (e *Enemy) knock(fx, fy, ks float64) {
	dx, dy := e.X-fx, e.Y-fy
	l := math.Hypot(dx, dy)
	if l == 0 {
		l = 1
	}
	ks *= e.kbFactor()
	e.KX, e.KY = clampK(e.KX+dx/l*ks), clampK(e.KY+dy/l*ks)
}

// lastD is the damage of the most recent hit (for lifesteal).
var lastD float64

// hit deals move damage to an enemy (port of the web hit()). kb: 0 none, 1 light, 2 strong;
// (fx,fy) is where the knockback comes from. The move is w.curSlot.
func (w *World) hit(e *Enemy, base float64, t string, kb int, fx, fy float64) {
	if e.Dead || e.Ug {
		return
	}
	s := w.curSlot
	var q *Q
	if s != nil {
		q = s.Q
	}
	if q != nil {
		if v := q.F("vps", 0); v > 0 && e.Psn > 0 {
			base *= v
		}
		if v := q.F("vst", 0); v > 0 && (e.Burn > 0 || e.Psn > 0 || e.Stun > 0 || e.Frz > 0 || e.Slow > 0 || e.Conf > 0) {
			base *= v
		}
		if v := q.F("vlow", 0); v > 0 && e.HP < e.MHP*.5 {
			base *= v
		}
		if vt := q.S("vty"); vt != "" && e.Type == vt {
			base *= 2
		}
		if q.F("hpx", 0) > 0 {
			base *= .6 + .6*w.P.HP/w.P.MHP
		}
		if q.F("shb", 0) > 0 {
			e.Sh = 0
		}
	}
	em := w.abEm(eff(t, e.Type))
	m := em
	if em > 1.15 {
		m = em * (1 + .25*w.pe("belt"))
	} else if em < 1 {
		m = math.Min(1, em+.08*w.pe("belt"))
	}
	if has(w.P.Types, t) {
		m *= 1.5
	}
	mv := 1.0
	if s != nil {
		tbl := w.DB.Balance.MoveDamage
		if s.Evo {
			tbl = w.DB.Balance.MoveDamageEvolved
		}
		if v, ok := tbl[s.ID]; ok {
			mv = v
		}
		mv *= w.itMv(s)
	}
	cc := .08*w.pe("lens") + .03*w.pe("razor") + q.F("cr", 0) + w.PS.F("crit")
	if e.Psn > 0 {
		cc += w.PS.F("critVsPsn")
	}
	cr := w.ab.agT > w.T || w.rng.Float64() < cc
	crit := 1.0
	if cr {
		crit = 2 + .2*w.pe("razor") + w.PS.F("critMul")
	}
	heat := 1.0
	if w.pe("heat") > 0 && (e.Burn > 0 || e.Psn > 0) {
		heat = 1 + .04*w.pe("heat")
	}
	d := math.Round(base * m * w.dmgM() * mv * crit * heat * w.abHitMul(e, t, em))
	if v := q.F("pct", 0); v > 0 && !e.Boss && !e.Elite {
		d = math.Max(d, math.Round(e.HP*v))
	}
	if d < 1 {
		d = 1
	}
	if e.Arm && q.F("iarm", 0) == 0 && w.PS.F("ignArm") == 0 {
		d = math.Max(1, math.Round(d*.6))
	}
	if e.Sh > 0 && w.PS.F("ignArm") == 0 {
		a := math.Min(e.Sh, d)
		e.Sh -= a
		d -= a
	}
	w.damage(e, d, em, cr)
	if v := w.pe("soothe"); v > 0 {
		w.P.HP = math.Min(w.P.MHP, w.P.HP+math.Min(d, math.Max(0, e.HP+d))*.005*v)
	}
	if s != nil {
		if v := q.F("lsx", 0); v > 0 {
			w.healCapped(d * v)
		}
	}
	if kl := w.pe("eject") + knockType[t]; kb > 0 && kl > 0 {
		ks := 40.0
		if kb == 2 {
			ks = 140
		}
		e.knock(fx, fy, ks*kl)
	}
	if e.HP > 0 {
		sheer := w.PS.F("sheer") > 0
		if st, ok := typeStatus[t]; ok && !sheer && w.rng.Float64() < st.p*w.serene() {
			w.applyStatus(e, st.k, st.v, d)
		}
		if st := q.St(); st != nil && !sheer && w.rng.Float64() < st.P*w.serene() {
			w.applyStatus(e, st.K, st.Dur, d)
		}
		w.abProc(e, t, d)
		if v := w.pe("heat"); v > 0 && w.rng.Float64() < .05*v {
			w.applyStatus(e, "bn", 3, d)
		}
	} else {
		w.kill(e)
	}
}

// hitRaw: damage over time and other flat hits (no type, STAB or item multipliers).
func (w *World) hitRaw(e *Enemy, base float64, col string) {
	if e.Dead || e.Ug {
		return
	}
	d := math.Max(1, math.Round(base))
	e.HP -= d
	e.Flash = .08
	w.DMG["_dot"] += d
	if w.rng.Float64() < .3 {
		w.addFX(&FX{Kind: "txt", X: e.X, Y: e.Y - e.R, Life: .6, Text: itoa(int(d)), Col: col})
	}
	if e.HP <= 0 {
		w.kill(e)
	}
}

func (w *World) damage(e *Enemy, d, em float64, crit bool) {
	w.sfx("hit")
	lastD = d
	if w.curSlot != nil {
		w.DMG[w.curSlot.ID] += math.Min(d, math.Max(0, e.HP))
	}
	e.HP -= d
	e.Flash = .08
	col := ""
	if em > 1.15 {
		col = "se"
	} else if em < 1 {
		col = "nve"
	}
	if crit {
		col = "crit"
	}
	if len(w.FX) < 90 || w.rng.Float64() < .3 {
		w.addFX(&FX{Kind: "txt", X: e.X, Y: e.Y - e.R, Life: .6, Text: itoa(int(d)), Col: col})
	}
}

func (w *World) kill(e *Enemy) {
	if e.Dead {
		return
	}
	e.Dead = true
	w.Kills++
	if w.cur != nil {
		w.cur.Kills++
	}
	switch {
	case e.Boss:
		w.sfx("bossdown")
	case e.Elite:
		w.sfx("elitedown")
	default:
		w.sfx("kill")
	}
	v := 1.0
	if e.MHP > 40 {
		v = 2
	}
	if e.Boss {
		v = 20
	}
	w.Gems = append(w.Gems, &Gem{X: e.X, Y: e.Y, V: v * (1 + w.T/300)})
	w.addFX(&FX{Kind: "pop", X: e.X, Y: e.Y, R: e.R, Life: .3, Type: e.Type})
	w.onDeath(e)
	w.abKill(e)
	w.coinDrop(e)
	if e.Boss {
		w.Bosses++
		w.Shake = .5
		w.afterBoss(e)
	}
}

func (w *World) message(s string) { w.Msg, w.MsgT = s, 3 }

func (w *World) heal(v float64) {
	if w.Run.NoHeal {
		return
	}
	w.P.HP = math.Min(w.P.MHP, w.P.HP+v)
}

func (w *World) healCapped(v float64) { w.heal(math.Min(v, w.P.MHP*.03)) }

// hurtK is the global damage-taken multiplier (web HURTK).
const hurtK = .93

// hurt applies damage of type t to the player.
func (w *World) hurt(d float64, t string) { w.hurtS(d, hurtSrc{t: t}) }

// hurtS applies damage to the player (web hurt): type multiplier, ability reductions, items, then
// last-chance saves (Focus Sash, Sturdy, Power Construct, revive).
func (w *World) hurtS(d float64, s hurtSrc) {
	P := w.P
	if P.Dead {
		return
	}
	h0 := P.HP
	tm := TypeMul(s.t, P.Types)
	d *= tm * hurtK * w.abRed(d, s, tm)
	red := w.PS.F("red")
	if P.Still > 1 {
		red += w.PS.F("redStill")
	}
	P.HP -= d * math.Pow(.96, w.up("arm")) * math.Pow(.93, w.pe("metal")) * b2f(P.Pbl > 0, 2, 1) * math.Max(.2, 1-red) / P.Stats.Def
	w.abHurt(h0 - P.HP)
	if h0-P.HP > .6 {
		P.HurtT = w.T
		w.sfx("hurt")
	}
	if P.HP <= 0 && w.pe("sash") > 0 && P.SashT <= w.T {
		P.HP, P.Inv = 1, 2
		P.SashT = w.T + math.Max(60, 150-12*w.pe("sash"))
		w.Shake = .3
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 50, Life: 2, Text: "Focus Sash!", Col: "gold"})
	}
	if P.HP <= 0 {
		w.abSave()
	}
	if P.HP <= 0 && w.PS.F("revive") > 0 && !w.ab.rv {
		w.ab.rv = true
		P.HP, P.Inv = P.MHP*.5, 2.5
		w.Shake = .4
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 50, Life: 2.5, Text: "Spirit Return!", Col: "conf"})
	}
	if P.HP <= 0 {
		P.HP = 0
		w.sfx("death")
		if w.pdie() {
			P.Dead = true
			w.State = StateDead
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
