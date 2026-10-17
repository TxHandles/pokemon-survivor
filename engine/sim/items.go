package sim

import "math"

// Item is a held item (passive) offered on level-up.
type Item struct{ ID, Name, Desc string }

// Items supported by this build (same ids, names and numbers as the web version).
var Items = []Item{
	{"band", "Power Band", "+12% damage"},
	{"claw", "Quick Claw", "-8% cooldowns"},
	{"left", "Leftovers", "+0.45 HP/sec"},
	{"lens", "Scope Lens", "+8% crit chance (2x damage)"},
	{"boots", "Swift Boots", "+8% move speed"},
	{"metal", "Metal Coat", "-7% damage taken"},
	{"wide", "Wide Lens", "+20% size for projectiles, orbs, rings and beams; +10% radius for melee"},
	{"magnet", "Magnet", "+25% pickup range"},
	{"exp", "EXP Share", "+10% XP gained"},
	{"soothe", "Soothe Bell", "Heal 0.5% of damage dealt"},
	{"glove", "Punching Glove", "+1 projectile, orb or chain jump (at Lv 1, 3 and 5)"},
	{"focus", "Focus Lens", "+15% range (projectiles, rings, orbits, chains)"},
	{"metro", "Metronome", "Each time a move fires it gains +1% damage (up to 10 stacks); +12% projectile and orbit speed"},
	{"eject", "Eject Button", "Hits knock enemies back"},
	{"belt", "Expert Belt", "+25% damage on super-effective hits; resisted hits lose less damage"},
	{"heat", "Heat Rock", "5% chance to burn; +4% damage to burning or poisoned foes; burn and poison +15% damage"},
	{"helmet", "Rocky Helmet", "Enemies touching you take damage (grows over time)"},
	{"choice", "Choice Band", "+9% damage to your highest-level move; your other moves deal -4%"},
	{"lifeorb", "Life Orb", "+10% damage; drains 0.15% max HP per second (stops at 50% HP)"},
	{"sash", "Focus Sash", "Survive a lethal hit with 1 HP and 2s of invulnerability; recharges in 150s (-12s per level)"},
	{"muscle", "Muscle Band", "+14% damage for melee moves (arcs, jabs, dashes, blasts, auras)"},
	{"wise", "Wise Glasses", "+14% damage for projectiles, beams, seekers and boomerangs"},
	{"razor", "Razor Claw", "+3% crit chance and +20% crit damage"},
}

var ItemByID = func() map[string]*Item {
	m := map[string]*Item{}
	for i := range Items {
		m[Items[i].ID] = &Items[i]
	}
	return m
}()

var meleeK = map[string]bool{"arc": true, "jab": true, "dash": true, "blast": true, "aura": true, "spin": true}
var specK = map[string]bool{"proj": true, "beam": true, "homing": true, "boom": true, "burst": true, "twin": true, "sludge": true, "chain": true, "cone": true}

// pe: effective item level (levels past 5 count half).
func (w *World) pe(k string) float64 {
	v := float64(w.Held[k])
	if v > 5 {
		return 5 + (v-5)*.5
	}
	return v
}

func (w *World) gvBase() int { return int(math.Ceil(w.pe("glove")/2)) + int(w.PS.F("cnt")) }

// gv: extra projectiles / orbs / jumps (glove, ability, and the firing move's rolled bonuses)
func (w *World) gv() int { return w.gvBase() + w.curSlot.BonusCount() }

// rgm: range multiplier (Focus Lens, ability, and the firing move's rolled bonuses)
func (w *World) rgm() float64 {
	return (1 + .15*w.pe("focus")) * (1 + w.PS.F("rng")) * w.curSlot.BonusAreaMul()
}
func (w *World) cdM() float64 {
	return ppow(.92, w.pe("claw")) * b2f(w.P.Bat > 0, .75, 1) * b2f(w.P.Phs > 0, .8, 1) * (1 - w.abCd())
}

// dmgM: player damage multiplier (.66 x items x attack stat; shop/mastery bonuses come with the meta phase).
func (w *World) dmgM() float64 {
	return .66 * (1 + .08*w.up("dmg")) * (1 + .02*float64(w.Cfg.Training)) * b2f(w.P.Bel > 0, 1.25, 1) * (1 + .12*w.pe("band")) * (1 + .1*w.pe("lifeorb")) * b2f(w.P.Pm > 0, 1.3, 1) * b2f(w.P.Pbl > 0, 1.5, 1) * w.psD() * w.P.Stats.Atk
}

// itMv: per-move item multipliers (Muscle Band, Wise Glasses, Choice Band, Metronome).
func (w *World) itMv(s *Slot) float64 {
	x := 1.0
	if v := w.pe("muscle"); v > 0 && meleeK[s.Kind] {
		x *= 1 + .14*v
	}
	if v := w.pe("wise"); v > 0 && specK[s.Kind] {
		x *= 1 + .14*v
	}
	if v := w.pe("choice"); v > 0 {
		var top *Slot
		for _, o := range w.Slots {
			if top == nil || o.Lv > top.Lv {
				top = o
			}
		}
		if s == top {
			x *= 1 + .09*v
		} else {
			x *= math.Max(.5, 1-.04*v)
		}
	}
	if v := w.pe("metro"); v > 0 && w.T-s.metroT < 4 {
		x *= 1 + .01*v*float64(s.metroN)
	}
	return x
}

func (w *World) metroTick(s *Slot) {
	if w.pe("metro") == 0 {
		return
	}
	if w.T-s.metroT > w.cdOf(s)*w.cdM()+1.5 {
		s.metroN = 0
	}
	if s.metroN < 10 {
		s.metroN++
	}
	s.metroT = w.T
}
