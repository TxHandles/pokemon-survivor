package sim

import (
	"math"
	"math/rand"
)

// ---- enemy-side state (ported from the web game's roleAI / spawnElite / bossAI / late) ----

// EBullet is an enemy projectile.
type EBullet struct {
	X, Y, VX, VY, L, R, Dm float64
	Type                   string // damage type ("" = neutral)
	Web, Homing            bool
}

// Hazard is a telegraphed area attack: it shows for T0 seconds, then hits once.
type Hazard struct {
	X, Y, R, T, T0, Dm float64
	Type               string // damage type ("" = neutral)
	Col                string // "r,g,b" tint, "" = default red
	Gone               bool
	Fl                 float64
	Bio                string // biome event hazard (storm / rockfall): type immunity applies
}

// Pickup is a map item: magnet, berry, battery, belt, bomb, candy.
type Pickup struct {
	X, Y, T float64
	K       string
}

// Boss definitions (web BOSSES): radius, damage / hp / speed multipliers and the attack kit per phase.
type BossDef struct {
	Name          string
	R, Dm, HP, Sp float64
	Kit           [3][]string
}

var Bosses = map[string]*BossDef{
	"normal":   {"Titan Brute", 36, 1.3, .95, 1, [3][]string{{"slam", "rings"}, {"slam", "rings", "charge"}, {"slam", "charge", "quake", "summon"}}},
	"fire":     {"Magma Warden", 36, 1.3, 1, 1, [3][]string{{"rain", "slam"}, {"rain", "rush", "rings"}, {"rain", "rush", "pulse", "slam2"}}},
	"water":    {"Tide Leviathan", 40, 1.3, 1.1, .9, [3][]string{{"pulse", "slam"}, {"pulse", "barrage", "rings"}, {"pulse", "barrage", "spiral", "charge"}}},
	"grass":    {"Elder Treant", 40, 1.25, 1.15, .8, [3][]string{{"pillars", "slam"}, {"pillars", "summon", "rings"}, {"pillars", "summon", "rain", "slam2"}}},
	"electric": {"Storm Sovereign", 32, .85, .9, 1.2, [3][]string{{"volley", "blink"}, {"volley", "blink", "charge"}, {"volley", "blink", "spiral", "rush"}}},
	"ground":   {"Canyon Colossus", 42, 1.3, 1.2, .8, [3][]string{{"quake", "slam"}, {"quake", "leap", "rings"}, {"quake", "leap", "pillars", "slam2"}}},
	"ice":      {"Frost Empress", 34, 1.3, 1, .95, [3][]string{{"spiral", "pulse"}, {"spiral", "pulse", "volley"}, {"spiral", "pulse", "volley", "rain"}}},
	"fighting": {"Dojo Master", 32, .83, 1, 1.15, [3][]string{{"charge3", "slam"}, {"charge3", "volley", "slam"}, {"charge3", "rush", "volley", "slam2"}}},
	"poison":   {"Bog Hydra", 38, .78, 1.1, .9, [3][]string{{"rain", "summon"}, {"rain", "summon", "barrage"}, {"rain", "summon", "barrage", "pulse"}}},
	"flying":   {"Sky Tyrant", 32, 1, .9, 1.25, [3][]string{{"blink", "spiral"}, {"blink", "spiral", "volley"}, {"blink", "spiral", "volley", "rush"}}},
	"psychic":  {"Mind Oracle", 32, .98, 1, 1, [3][]string{{"barrage", "blink"}, {"barrage", "blink", "pulse"}, {"barrage", "blink", "pulse", "spiral"}}},
	"bug":      {"Hive Queen", 36, .97, 1, 1.05, [3][]string{{"summon", "volley"}, {"summon", "volley", "rush"}, {"summon", "volley", "rush", "spiral"}}},
	"rock":     {"Quarry Golem", 44, .85, 1.25, .75, [3][]string{{"leap", "rain"}, {"leap", "rain", "rings"}, {"leap", "rain", "quake", "slam2"}}},
	"ghost":    {"Phantom Lord", 34, .72, 1, 1, [3][]string{{"blink", "barrage"}, {"blink", "barrage", "pillars"}, {"blink", "barrage", "pillars", "spiral"}}},
	"dragon":   {"Elder Drake", 40, .55, 1.3, 1.05, [3][]string{{"spiral", "rain", "slam"}, {"spiral", "rain", "rush", "pulse"}, {"spiral", "rain", "rush", "pulse", "leap"}}},
	"dark":     {"Shadow Stalker", 32, .6, 1, 1.2, [3][]string{{"blink", "pillars"}, {"blink", "pillars", "charge"}, {"blink", "pillars", "rush", "volley"}}},
	"steel":    {"Iron Juggernaut", 42, 1.05, 1.25, .85, [3][]string{{"charge", "quake"}, {"charge", "quake", "volley"}, {"rush", "quake", "volley", "leap"}}},
	"fairy":    {"Glimmer Queen", 34, 1.3, 1, 1, [3][]string{{"spiral", "summon"}, {"spiral", "summon", "barrage"}, {"spiral", "summon", "barrage", "pulse"}}},
}

// Biome hazards (web BHZ): pools on the ground or timed events.
var BiomeHazard = map[string]string{"fire": "lava", "water": "deep", "ice": "ice", "poison": "toxic", "ground": "sand", "bug": "web",
	"electric": "storm", "rock": "rockfall", "flying": "gust"}

var HazardText = map[string]string{"lava": "Lava pools burn you badly while you stand in them.", "deep": "Deep water slows you down.",
	"ice": "Slippery ice makes you slide.", "toxic": "Toxic pools poison you and slow you a little.", "sand": "Quicksand slows you heavily.",
	"web": "Sticky webs nearly stop you.", "storm": "Lightning strikes land near you (watch the yellow circles).",
	"rockfall": "Boulders crash down near you (watch the circles).", "gust": "Strong gusts shove you around."}

// role spawn chances (web SHOOT_P etc.)
const (
	shootP = .09
	healP  = .3
	burP   = .3
	phaP   = .15
	chgP   = .25
	webP   = .25
	shdP   = .3
	healR  = 130.0
)

var shootT = map[string]bool{"fire": true, "water": true, "dragon": true, "normal": true}

// Elite affixes (web AFX).
var Affixes = []string{"swift", "arm", "shld", "vamp", "expl", "split", "sumn"}
var AffixName = map[string]string{"swift": "Swift", "arm": "Armored", "shld": "Shielded", "vamp": "Vampiric", "expl": "Explosive", "split": "Splitting", "sumn": "Summoner"}

func (w *World) assignRole(e *Enemy) {
	r := w.rng.Float64
	switch {
	case e.Type == "ground" && r() < burP:
		e.Rk, e.rkt = "burrow", w.rnd(2, 5)
	case e.Type == "ghost" && r() < phaP:
		e.Rk, e.rkt = "phase", w.rnd(2, 5)
	case e.Type == "fairy" && r() < healP:
		e.Rk, e.rkt = "heal", w.rnd(1, 3)
		e.HP *= .8
		e.MHP *= .8
	case e.Type == "electric" && r() < chgP:
		e.Rk, e.rkt = "charge", w.rnd(1.5, 4)
	case e.Type == "bug" && r() < webP:
		e.Rk, e.rkt = "web", w.rnd(1.5, 4)
	case e.Type == "steel" && r() < shdP:
		e.Sh = e.HP * .4
		e.ShMax = e.Sh
	case shootT[e.Type] && r() < shootP:
		e.Rk, e.rkt = "shoot", w.rnd(1, 3)
	}
}

func (w *World) minion(x, y float64, t string) {
	sc := w.tsc() * w.ef()
	e := &Enemy{X: x, Y: y, Type: t, R: 9, HP: 12 * sc * w.Run.HM, MHP: 12 * sc * w.Run.HM, Sp: w.rnd(70, 100) * w.spS() * w.Run.SM, Dm: 5 * w.dmS() * w.Run.DM, Seed: w.rng.Float64()}
	e.PokeID = w.enemySprite(t, "")
	w.E = append(w.E, e)
}

func (w *World) spawnElite(mini bool) { w.spawnEliteAt(mini, math.NaN(), 0, false) }

func (w *World) spawnEliteAt(mini bool, x, y float64, cursed bool) {
	t := w.pickT(.5)
	sc := w.tsc()
	a := w.rnd(0, 6.283)
	d := w.spawnDist() + 20
	if math.IsNaN(x) {
		x, y = w.P.X+math.Cos(a)*d, w.P.Y+math.Sin(a)*d
	}
	n := 1
	if mini {
		n = 2
	}
	if w.Run.Tier >= 5 && w.rng.Float64() < .5 {
		n = min(3, n+1)
	}
	keys := append([]string{}, Affixes...)
	w.rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	hp := 130 * sc
	r, dm := 18.0, 14.0
	if mini {
		hp, r, dm = 270*sc, 24, 20
	}
	hp *= w.Run.HM * (1 + .08*float64(w.Run.Tier))
	dm *= w.Run.DM
	e := &Enemy{Elite: true, Mini: mini, Cursed: cursed, Big: true, X: x, Y: y, Type: t, R: r, HP: hp, MHP: hp,
		Sp: w.rnd(55, 72) * w.spS() * w.Run.SM, Dm: dm * w.dmS(), Af: keys[:n], Seed: w.rng.Float64()}
	for _, k := range e.Af {
		switch k {
		case "swift":
			e.Sp *= 1.5
		case "arm":
			e.Arm = true
		case "shld":
			e.Sh = hp * .6
			e.ShMax = e.Sh
		case "vamp":
			e.Vamp = true
		case "expl":
			e.Expl = true
		case "split":
			e.Split = true
		case "sumn":
			e.Sumn = true
		}
	}
	e.PokeID = w.enemySprite(t, b2s(mini, "boss", ""))
	w.E = append(w.E, e)
	if mini {
		w.message("A mini-boss approaches!")
	} else {
		w.message("An elite " + t + " Pokémon appears!")
	}
}

func b2s(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

// onDeath: elite / affix effects when an enemy faints (web eDie).
func (w *World) onDeath(e *Enemy) {
	if e.Expl {
		w.HZ = append(w.HZ, &Hazard{X: e.X, Y: e.Y, R: 85, T: 1, T0: 1, Dm: e.Dm * 1.5, Col: "255,153,85"})
	}
	if e.Split {
		for i := 0; i < 4; i++ {
			w.minion(e.X+w.rnd(-15, 15), e.Y+w.rnd(-15, 15), e.Type)
		}
	}
	if e.Elite {
		w.Stats.Elites++
		if e.Mini {
			w.Stats.Minis++
		}
		if w.rng.Float64() < b2f(e.Mini, .05, .02) {
			w.Gems = append(w.Gems, &Gem{X: e.X, Y: e.Y - 8, Tk: true, Ed: true, Mag: true})
			w.addFX(&FX{Kind: "txt", X: e.X, Y: e.Y - 52, Life: 2, Text: "Ticket dropped!", Col: "gold"})
		}
		n := 3
		if e.Mini {
			n = 5
		}
		for i := 0; i < n; i++ {
			w.Gems = append(w.Gems, &Gem{X: e.X + w.rnd(-24, 24), Y: e.Y + w.rnd(-24, 24), V: w.P.NX * .15})
		}
		w.dropItem(e.X, e.Y)
		if e.Mini {
			w.dropItem(e.X+10, e.Y)
		}
		w.addFX(&FX{Kind: "txt", X: e.X, Y: e.Y - 30, Life: 1.5, Text: b2s(e.Mini, "MINI-BOSS DEFEATED", "Elite defeated!"), Col: "gold"})
	}
	if !e.Boss && !e.Elite && !w.noDrop && w.rng.Float64() < b2f(e.Big, .008, .0015) {
		w.dropItem(e.X, e.Y)
	}
	if !e.Boss && w.rng.Float64() < b2f(e.Big, .0015, .00005) {
		w.Gems = append(w.Gems, &Gem{X: e.X - 5, Y: e.Y, Tk: true})
	}
}

func (w *World) dropItem(x, y float64) {
	r := w.rng.Float64()
	k := "bomb"
	switch {
	case r < .25:
		k = "magnet"
	case r < .5:
		k = "berry"
	case r < .7:
		k = "battery"
	case r < .9:
		k = "belt"
	}
	w.Items = append(w.Items, &Pickup{X: x, Y: y, K: k})
}

var ItemText = map[string]string{"magnet": "Magnet: all XP flies to you", "berry": "Berry: +50% HP", "battery": "Battery: -25% cooldowns for 10s",
	"belt": "Strength Belt: +25% damage for 10s", "bomb": "Bomb: wipes the screen", "candy": "Rare Candy! Level up", "harvest": "Berry: +15% HP"}

func (w *World) useItem(k string) {
	P := w.P
	switch k {
	case "magnet":
		w.magnetAll = true
	case "berry":
		w.heal(P.MHP * .5)
	case "battery":
		P.Bat = 10
	case "belt":
		P.Bel = 10
	case "bomb":
		w.Shake = .7
		prev := w.curSlot
		w.curSlot = nil
		for _, e := range w.E {
			if e.Dead || math.Abs(e.X-P.X) > w.ViewW/2+30 || math.Abs(e.Y-P.Y) > w.ViewH/2+30 {
				continue
			}
			d := e.HP + 1
			if e.Boss {
				d = e.MHP * .1
			}
			w.noDrop = true
			w.hitRaw(e, d, "crit")
			w.noDrop = false
		}
		w.curSlot = prev
		w.addFX(&FX{Kind: "flash", Life: .35})
	case "candy":
		P.XP += P.NX
	case "harvest":
		w.heal(P.MHP * .15)
	}
	w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 40, Life: 2, Text: ItemText[k], Col: "gold"})
}

func (w *World) stepPickups(dt float64) {
	P := w.P
	w.candyT -= dt
	if w.candyT <= 0 {
		w.candyT = 20
		if w.rng.Float64() < .1 {
			n := 0
			for _, it := range w.Items {
				if it.K == "candy" {
					n++
				}
			}
			if n < 3 {
				a, d := w.rnd(0, 6.283), w.rnd(180, 320)
				x, y := P.X+math.Cos(a)*d, P.Y+math.Sin(a)*d
				w.push(&x, &y, 16)
				w.Items = append(w.Items, &Pickup{X: x, Y: y, K: "candy"})
			}
		}
	}
	keep := w.Items[:0]
	for _, it := range w.Items {
		it.T += dt
		if dist(it.X, it.Y, P.X, P.Y) < 24 {
			w.useItem(it.K)
			continue
		}
		life := 40.0
		if it.K == "candy" {
			life = 60
		}
		if it.T < life {
			keep = append(keep, it)
		}
	}
	w.Items = keep
}

// ---- AI ----

// ai returns the enemy's speed multiplier this frame and may steer it (e.ao). Port of web ai0 / roleAI / bossAI.
func (w *World) ai(e *Enemy, dt float64) float64 {
	e.aoSet = false
	if e.Boss {
		return w.bossAI(e, dt)
	}
	if e.Fear > 0 || e.Charm > 0 {
		return 1
	}
	if e.Sumn {
		if e.sut == 0 {
			e.sut = 4
		}
		e.sut -= dt
		if e.sut <= 0 {
			e.sut = 7
			if len(w.E) < 500 {
				for i := 0; i < 3; i++ {
					w.minion(e.X+w.rnd(-20, 20), e.Y+w.rnd(-20, 20), e.Type)
				}
			}
		}
	}
	if e.Sp2 != "" {
		return w.specAI(e, dt)
	}
	if e.Rk != "" {
		return w.roleAI(e, dt)
	}
	return 1
}

func (e *Enemy) steer(a float64) { e.ao, e.aoSet = a, true }

func (w *World) roleAI(e *Enemy, dt float64) float64 {
	P := w.P
	dx, dy := P.X-e.X, P.Y-e.Y
	d := math.Max(hyp(dx, dy), 1)
	ang := math.Atan2(dy, dx)
	m := 1.0
	switch e.Rk {
	case "burrow":
		switch e.bs {
		case 0:
			e.rkt -= dt
			if e.rkt <= 0 {
				if d < 380 && d > 70 {
					e.bs, e.Ug, e.btv = 1, true, 1.4
					w.addFX(&FX{Kind: "puff", X: e.X, Y: e.Y, R: 22, Life: .3, Type: "ground"})
				} else {
					e.rkt = .5
				}
			}
			return 1
		case 1:
			e.btv -= dt
			e.steer(ang)
			if e.btv <= 0 || d < 46 {
				e.bs, e.bwt = 2, .55
				w.addFX(&FX{Kind: "tele", X: e.X, Y: e.Y, R: 46, Life: .55, Type: "ground", Enemy: true})
			}
			return 1.7
		}
		e.bwt -= dt
		if e.bwt <= 0 {
			e.bs, e.Ug, e.rkt = 0, false, w.rnd(5, 7)
			w.Shake = math.Max(w.Shake, .12)
			w.addFX(&FX{Kind: "puff", X: e.X, Y: e.Y, R: 46, Life: .35, Type: "ground"})
			if hyp(P.X-e.X, P.Y-e.Y) < 46+12 {
				w.eHit(e.Dm*1.1, e.Type)
			}
		}
		return 0
	case "charge":
		if e.cdh > 0 {
			e.cdh -= dt
			e.steer(e.cang)
			return 5
		}
		if e.cwu > 0 {
			e.cwu -= dt
			if e.cwu <= 0 {
				e.cdh = .45
			}
			return 0
		}
		e.rkt -= dt
		if e.rkt <= 0 {
			if d < 300 && d > 60 {
				e.cwu, e.cang, e.rkt = .6, ang, w.rnd(3.5, 5)
			} else {
				e.rkt = .4
			}
		}
		return 1
	case "web":
		if e.rkw > 0 {
			e.rkw -= dt
			if e.rkw <= 0 {
				w.EB = append(w.EB, &EBullet{X: e.X, Y: e.Y, VX: dx / d * 170, VY: dy / d * 170, L: 3, R: 9, Dm: e.Dm * .3, Type: e.Type, Web: true})
			}
			return 0
		}
		e.rkt -= dt
		if e.rkt <= 0 {
			if d < 420 {
				e.rkw, e.rkt = .35, w.rnd(4, 6)
			} else {
				e.rkt = .5
			}
		}
		return 1
	case "phase":
		switch e.ps {
		case 0:
			e.rkt -= dt
			if e.lg > 0 {
				e.lg -= dt
				return 1
			}
			if e.rkt <= 0 {
				if d > 220 && d < 600 {
					e.ps, e.pf = 1, .35
				} else {
					e.rkt = .5
				}
			}
			return 1
		case 1:
			e.pf -= dt
			if e.pf <= 0 {
				e.ps, e.Ug, e.pt2 = 2, true, .85
				a, r := w.rnd(0, 6.283), w.rnd(130, 180)
				e.dx2, e.dy2 = P.X+math.Cos(a)*r, P.Y+math.Sin(a)*r
				e.pmk = false
			}
			return 0
		case 2:
			e.pt2 -= dt
			if e.pt2 <= .55 && !e.pmk {
				e.pmk = true
				w.addFX(&FX{Kind: "tele", X: e.dx2, Y: e.dy2, R: 24, Life: .55, Type: "ghost"})
			}
			if e.pt2 <= 0 {
				e.X, e.Y = e.dx2, e.dy2
				e.Ug, e.ps, e.pf, e.rkt = false, 3, .25, w.rnd(7, 9)
			}
			return 0
		}
		e.pf -= dt
		if e.pf <= 0 {
			e.ps, e.lg = 0, .8
		}
		return 0
	case "heal":
		if d < 140 {
			e.steer(ang + math.Pi)
		} else if d < 200 {
			if e.sd == 0 {
				e.sd = 1
				if w.rng.Float64() < .5 {
					e.sd = -1
				}
			}
			e.steer(ang + e.sd*1.57)
			m = .45
		}
	default:
		m = .7
	}
	if e.rkw > 0 {
		e.rkw -= dt
		m = 0
		if e.rkw <= 0 {
			if e.Rk == "shoot" {
				w.EB = append(w.EB, &EBullet{X: e.X, Y: e.Y, VX: dx / d * 210, VY: dy / d * 210, L: 2.6, R: 7, Dm: e.Dm * .5, Type: e.Type})
			} else {
				w.addFX(&FX{Kind: "pv", X: e.X, Y: e.Y, R: healR, Life: .35, Type: "fairy"})
				for _, o := range w.E {
					if o.Dead || o == e || o.Boss || hyp(o.X-e.X, o.Y-e.Y) > healR+o.R {
						continue
					}
					amt := o.MHP * b2f(o.Elite, .06, .12)
					if o.HP < o.MHP {
						o.HP = math.Min(o.MHP, o.HP+amt)
					}
				}
			}
		}
		return m
	}
	e.rkt -= dt
	if e.rkt <= 0 {
		if e.Rk == "shoot" {
			if d < 480 {
				e.rkt, e.rkw = 3.2, .35
			} else {
				e.rkt = .5
			}
		} else {
			hurt := false
			for _, o := range w.E {
				if o != e && !o.Dead && !o.Boss && o.HP < o.MHP && hyp(o.X-e.X, o.Y-e.Y) < healR+o.R {
					hurt = true
					break
				}
			}
			if hurt {
				e.rkt, e.rkw = 3.5, .4
			} else {
				e.rkt = .6
			}
		}
	}
	return m
}

// eHit: an enemy attack (contact, bullet, blast) lands on the player.
func (w *World) eHit(d float64, t string) { w.eHitS(d, hurtSrc{t: t}) }

func (w *World) eHitS(d float64, s hurtSrc) {
	if w.P.Inv > 0 || w.P.Dead || w.State != StatePlay {
		return
	}
	if !w.dodge() {
		w.hurtS(d, s)
	}
	w.P.Inv = .5 + w.PS.F("inv")
	w.Shake = .12
}

// dodge rolls the ability's dodge chance.
func (w *World) dodge() bool {
	if dg := w.abDodge(); dg > 0 && w.rng.Float64() < dg {
		w.addFX(&FX{Kind: "txt", X: w.P.X, Y: w.P.Y - 30, Life: .8, Text: "Dodged!", Col: "se"})
		return true
	}
	return false
}

func (w *World) eRing(e *Enemy, n int, o, sp, k float64) {
	for i := 0; i < n; i++ {
		a := o + float64(i)*6.283/float64(n)
		w.EB = append(w.EB, &EBullet{X: e.X, Y: e.Y, VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, L: 4, R: 6, Dm: e.Dm * k, Type: e.Type})
	}
}

func (w *World) bul(e *Enemy, a, sp, k float64, homing bool) {
	b := &EBullet{X: e.X, Y: e.Y, VX: math.Cos(a) * sp, VY: math.Sin(a) * sp, L: 4, R: 6, Dm: e.Dm * k, Type: e.Type}
	if homing {
		b.Homing, b.L, b.R = true, 6, 9
	}
	w.EB = append(w.EB, b)
}

func (w *World) hz(x, y, r, t, dm float64, typ, col string) {
	w.HZ = append(w.HZ, &Hazard{X: x, Y: y, R: r, T: t, T0: t, Dm: dm, Type: typ, Col: col})
}

// boss attack kit (web NA + atkGo)
func (w *World) bossMove(e *Enemy, k string) bool {
	P := w.P
	ph := float64(e.Ph)
	alive := func(fn func()) func() {
		return func() {
			if !e.Dead {
				fn()
			}
		}
	}
	switch k {
	case "rain":
		n := 6 + e.Ph*2
		for i := 0; i < n; i++ {
			a, d := w.rnd(0, 6.283), w.rnd(0, 230)
			w.hz(P.X+math.Cos(a)*d, P.Y+math.Sin(a)*d, 55, .9+float64(i)*.12, e.Dm*1.2, e.Type, "")
		}
	case "quake":
		w.hz(e.X, e.Y, 115, 1, e.Dm*1.4, e.Type, "")
		w.later(1, alive(func() { w.eRing(e, 14+3*e.Ph, w.rnd(0, 6), 190, .5) }))
	case "pulse":
		w.hz(e.X, e.Y, 150, 1, e.Dm*1.2, e.Type, "80,170,255")
		w.later(1, alive(func() {
			w.eRing(e, 16+2*e.Ph, 0, 170, .45)
			w.later(.35, alive(func() { w.eRing(e, 16+2*e.Ph, .2, 230, .45) }))
		}))
	case "spiral":
		n := 22 + 6*e.Ph
		for i := 0; i < n; i++ {
			i := i
			w.later(float64(i)*.1, alive(func() { w.bul(e, float64(i)*.55, 200, .45, false) }))
		}
	case "volley":
		for s := 0; s < 3; s++ {
			w.later(float64(s)*.45, alive(func() {
				a := math.Atan2(P.Y-e.Y, P.X-e.X)
				n := 4 + e.Ph
				for i := 0; i < n; i++ {
					w.bul(e, a+(float64(i)-float64(n-1)/2)*.2, 270, .5, false)
				}
			}))
		}
	case "barrage":
		n := 2 + e.Ph
		for i := 0; i < n; i++ {
			w.later(float64(i)*.35, alive(func() { w.bul(e, w.rnd(0, 6.283), 95, .6, true) }))
		}
	case "blink":
		a := w.rnd(0, 6.283)
		x, y := P.X+math.Cos(a)*130, P.Y+math.Sin(a)*130
		w.hz(x, y, 42, .8, 0, "", "200,120,255")
		w.later(.8, alive(func() {
			e.X, e.Y = x, y
			w.eRing(e, 10+2*e.Ph, w.rnd(0, 6), 200, .5)
			w.hz(P.X, P.Y, 70, .7, e.Dm*1.5, e.Type, "")
		}))
	case "leap":
		x, y := P.X, P.Y
		w.hz(x, y, 100, 1, e.Dm*1.8, e.Type, "")
		w.later(1, alive(func() {
			e.X, e.Y = x, y
			w.Shake = .5
			w.eRing(e, 10, 0, 160, .5)
		}))
	case "pillars":
		a := math.Atan2(P.Y-e.Y, P.X-e.X)
		for i := 1; i <= 5+e.Ph; i++ {
			fi := float64(i)
			w.hz(e.X+math.Cos(a)*fi*95, e.Y+math.Sin(a)*fi*95, 46, .7+fi*.12, e.Dm*1.3, e.Type, "")
		}
	case "rush":
		e.ca, e.wu, e.K, e.trl = math.Atan2(P.Y-e.Y, P.X-e.X), .8, "charge", true
	case "charge3":
		e.ca, e.wu, e.K, e.c3 = math.Atan2(P.Y-e.Y, P.X-e.X), .8, "charge", 2
	default:
		_ = ph
		return false
	}
	return true
}

func (w *World) atkGo(e *Enemy) {
	switch e.K {
	case "rings":
		w.eRing(e, 12+4*e.Ph, w.rnd(0, 6.28), 170, .5)
	case "charge":
		e.chd = .55
	case "summon":
		for i := 0; i < 6; i++ {
			if len(w.E) < 500 {
				w.minion(e.X+math.Cos(float64(i))*50, e.Y+math.Sin(float64(i))*50, e.Type)
			}
		}
	}
}

func (w *World) bossAI(e *Enemy, dt float64) float64 {
	P := w.P
	if e.Ph == 0 {
		e.Ph, e.at = 1, 3.5
	}
	ph := 1
	if e.HP < e.MHP*.33 {
		ph = 3
	} else if e.HP < e.MHP*.66 {
		ph = 2
	}
	if ph > e.Ph {
		e.Ph = ph
		w.addFX(&FX{Kind: "txt", X: e.X, Y: e.Y - 50, Life: 2, Text: "PHASE " + itoa(ph) + "!", Col: "crit"})
		w.eRing(e, 16+4*ph, 0, 170, .5)
		e.at, e.wu, e.chd = 1.5, 0, 0
		w.Shake = .4
	}
	if e.wu > 0 {
		e.wu -= dt
		if e.wu <= 0 {
			w.atkGo(e)
		}
		return 0
	}
	if e.chd > 0 {
		e.chd -= dt
		e.steer(e.ca)
		if e.trl {
			e.tt -= dt
			if e.tt <= 0 {
				e.tt = .14
				w.hz(e.X, e.Y, 40, .9, e.Dm, e.Type, "")
			}
		}
		if e.chd <= 0 && e.c3 > 0 {
			e.c3--
			w.later(.35, func() {
				if !e.Dead {
					e.ca = math.Atan2(P.Y-e.Y, P.X-e.X)
					e.wu = .5
				}
			})
		}
		return 7
	}
	e.at -= dt
	if e.at <= 0 {
		kit := e.BD.Kit[e.Ph-1]
		e.K = kit[w.rng.Intn(len(kit))]
		e.trl, e.c3 = false, 0
		e.at = math.Max(2, 5.5-float64(e.Ph)*.8-float64(w.Run.Tier)*.12)
		if w.bossMove(e, e.K) {
			return 1 + .15*float64(e.Ph-1)
		}
		switch e.K {
		case "slam":
			w.hz(P.X, P.Y, 78, 1.2, e.Dm*1.6, e.Type, "")
		case "slam2":
			for i := 0; i < 3; i++ {
				fi := float64(i)
				w.hz(P.X+P.FX*140*fi, P.Y+P.FY*140*fi, 70, 1.1+.3*fi, e.Dm*1.4, e.Type, "")
			}
		default:
			e.wu = b2f(e.K == "charge", .8, .6)
			e.ca = math.Atan2(P.Y-e.Y, P.X-e.X)
		}
	}
	return 1 + .15*float64(e.Ph-1)
}

// bossClear: a boss faint clears the field (web bossClear) and holds spawns briefly.
func (w *World) bossClear(b *Enemy) {
	n := 0
	for _, x := range w.E {
		if x.Dead || x == b || x.Boss {
			continue
		}
		x.Dead = true
		if n < 160 && math.Abs(x.X-w.P.X) < 900 && math.Abs(x.Y-w.P.Y) < 700 {
			w.addFX(&FX{Kind: "puff", X: x.X, Y: x.Y, R: x.R * 1.6, Life: .4, Type: x.Type})
		}
		n++
	}
	w.EB = nil
	w.spawnHold = 3
	w.addFX(&FX{Kind: "txt", X: w.P.X, Y: w.P.Y - 150, Life: 2.5, Text: "The area is clear!", Col: "gold"})
}

// ---- enemy bullets and telegraphed hazards (web late()) ----

func (w *World) stepEnemyAttacks(dt float64) {
	P := w.P
	w.elT -= dt
	if w.elT <= 0 && w.T >= 150 && w.spawnHold <= 0 {
		w.elT = math.Max(22, 70-float64(w.Run.Tier)*4-w.T/40)
		n := 0
		for _, e := range w.E {
			if e.Elite && !e.Dead {
				n++
			}
		}
		if n < 4+w.Run.Tier>>1 {
			w.spawnElite(false)
			if w.Run.Tier >= 6 && w.rng.Float64() < .4 {
				w.spawnElite(false)
			}
		}
	}
	if w.T >= w.mbT && w.spawnHold <= 0 {
		w.mbT += 300
		w.spawnElite(true)
	}
	w.biomeEvents(dt)
	keep := w.EB[:0]
	for _, b := range w.EB {
		if b.Homing {
			a := math.Atan2(P.Y-b.Y, P.X-b.X)
			cu := math.Atan2(b.VY, b.VX)
			df := a - cu
			df = math.Atan2(math.Sin(df), math.Cos(df))
			nn := cu + math.Max(-1.5*dt, math.Min(1.5*dt, df))
			sp := hyp(b.VX, b.VY)
			b.VX, b.VY = math.Cos(nn)*sp, math.Sin(nn)*sp
		}
		b.X += b.VX * dt
		b.Y += b.VY * dt
		b.L -= dt
		if b.L > 0 && dist(b.X, b.Y, P.X, P.Y) < b.R+12 {
			b.L = 0
			if !w.abProj(b) {
				if b.Web && !has(P.Types, "flying") && !w.abSlowImm() {
					P.WebT = 1.8
				}
				w.eHitS(b.Dm, hurtSrc{t: b.Type, proj: true})
			}
		}
		if b.L > 0 {
			keep = append(keep, b)
		}
	}
	w.EB = keep
	hk := w.HZ[:0]
	for _, h := range w.HZ {
		h.T -= dt
		if !h.Gone && h.T <= 0 {
			h.Gone, h.Fl = true, .25
			w.Shake = math.Max(w.Shake, .15)
			if dist(h.X, h.Y, P.X, P.Y) < h.R+10 && h.Dm > 0 {
				kz := 1.0
				if h.Bio != "" {
					kz = w.hzK()
					if w.abHzImm(h.Bio) {
						kz = 0
					}
				}
				if h.Bio == "storm" && w.PS.S("hzHeal") == "storm" {
					w.heal(P.MHP * .08)
					w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 30, Life: .9, Text: "Absorbed!", Col: "se"})
				} else if kz > 0 {
					w.eHit(h.Dm*kz, h.Type)
				}
			}
		}
		if h.Gone {
			h.Fl -= dt
		}
		if !h.Gone || h.Fl > 0 {
			hk = append(hk, h)
		}
	}
	w.HZ = hk
}

// ---- biome hazards ----

// hzK: how strongly the biome's hazard affects the player (0 = immune: same type; else the type multiplier).
func (w *World) hzK() float64 {
	if has(w.P.Types, w.Biome) {
		return 0
	}
	return TypeMul(w.Biome, w.P.Types)
}

// hzKe: the same for an enemy of type t (gusts).
func (w *World) hzKe(t string) float64 {
	if t == w.Biome {
		return 0
	}
	return TypeMul(w.Biome, []string{t})
}

// Pool is a ground hazard area.
type Pool struct {
	X, Y, R float64
	K       string
}

// Pools returns the hazard pools in chunk (cx, cy) (web hzc()).
func (w *World) Pools(cx, cy int) []Pool {
	k := BiomeHazard[w.Biome]
	if k == "" || k == "storm" || k == "rockfall" || k == "gust" {
		return nil
	}
	key := [2]int{cx, cy}
	if p, ok := w.pools[key]; ok {
		return p
	}
	if len(w.pools) > 600 {
		w.pools = map[[2]int][]Pool{}
	}
	r := jsRng(uint32(int32(cx)*-1640531535) ^ uint32(int32(cy)*40503) ^ uint32(w.Seed) ^ 0x5bd1e995)
	var out []Pool
	if r() >= .5 {
		rad := 50 + r()*45
		x, y := float64(cx)*ChunkSize+r()*ChunkSize, float64(cy)*ChunkSize+r()*ChunkSize
		if hyp(x, y) >= 190 {
			out = append(out, Pool{x, y, rad, k})
		}
	}
	w.pools[key] = out
	return out
}

// HazAt returns the hazard kind at (x, y), or "".
func (w *World) HazAt(x, y float64) string {
	if BiomeHazard[w.Biome] == "" {
		return ""
	}
	cx, cy := int(math.Floor(x/ChunkSize)), int(math.Floor(y/ChunkSize))
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			for _, h := range w.Pools(cx+i, cy+j) {
				if (x-h.X)*(x-h.X)+(y-h.Y)*(y-h.Y) < h.R*h.R {
					return h.K
				}
			}
		}
	}
	return ""
}

// GustState is the active wind (web gust).
type GustState struct {
	A, T, VX, VY float64
}

func (w *World) biomeEvents(dt float64) {
	P := w.P
	k := BiomeHazard[w.Biome]
	if P.gw > 0 {
		P.gw -= dt
		if P.gw <= 0 && w.Gust != nil {
			g := 30 * math.Min(1, w.hzK())
			if w.abGustImm() {
				g = 0
			}
			P.WX, P.WY = math.Cos(w.Gust.A)*g, math.Sin(w.Gust.A)*g
			w.Gust.T = 2.5
		}
	}
	if w.Gust != nil && w.Gust.T > 0 {
		w.Gust.T -= dt
		if w.Gust.T <= 0 {
			P.WX, P.WY = 0, 0
		}
	}
	if has(P.Types, w.Biome) && k != "gust" {
		return
	}
	w.bioT -= dt
	if w.bioT > 0 {
		return
	}
	switch k {
	case "storm", "rockfall":
		r, dm, d0, d1, t, col, typ := 42.0, 10*w.dmS(), 60.0, 260.0, 1.2, "255,224,61", "electric"
		w.bioT = math.Max(5, 7-.1*float64(w.Run.Tier))
		n := b2i(w.T > 600) + b2i(w.Run.Tier > 5)
		ahead := 40.0
		if k == "rockfall" {
			r, dm, d0, d1, t, col, typ = 52, 13*w.dmS(), 80, 280, 1.5, "190,160,110", "rock"
			w.bioT = math.Max(6.5, 9.5-.15*float64(w.Run.Tier))
			n = 1 + b2i(w.T > 600)
			ahead = 60
		}
		w.hz(P.X+P.FX*ahead, P.Y+P.FY*ahead, r, t, dm, "", col)
		for i := 0; i < n; i++ {
			a, d := w.rnd(0, 6.283), w.rnd(d0, d1)
			w.hz(P.X+math.Cos(a)*d, P.Y+math.Sin(a)*d, r, t, dm, "", col)
		}
		for _, h := range w.HZ[len(w.HZ)-n-1:] {
			h.Bio = k
		}
		_ = typ
	case "gust":
		w.bioT = 9 + w.rnd(0, 4)
		a := w.rnd(0, 6.283)
		w.Gust = &GustState{A: a, VX: math.Cos(a) * 30, VY: math.Sin(a) * 30}
		P.gw = .9
		w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 70, Life: 1.5, Text: "A strong gust is coming!", Col: "se"})
	default:
		w.bioT = 99
	}
}

// playerHazards applies standing-in-pool effects and returns the speed multiplier (web pStep).
func (w *World) playerTerrain(dt float64) (spd float64, hk string) {
	P := w.P
	kk := w.hzK()
	if w.abHzImm(BiomeHazard[w.Biome]) {
		kk = 0
	}
	kc := math.Min(1, kk)
	sl := func(v float64) float64 {
		if w.abSlowImm() {
			return 1
		}
		return 1 - (1-v)*kc
	}
	if w.PS.S("hzHeal") == "deep" && BiomeHazard[w.Biome] == "deep" && w.HazAt(P.X, P.Y) == "deep" {
		w.heal(1.5 * dt)
	}
	if kk != 0 && !has(P.Types, "flying") && !has(P.Types, "ghost") {
		hk = w.HazAt(P.X, P.Y)
	}
	spd = 1
	switch hk {
	case "deep":
		spd = sl(.55)
	case "sand":
		spd = sl(.5)
	case "web":
		spd = sl(.35)
	case "toxic":
		spd = sl(.8)
	case "lava":
		w.hurt((P.MHP*.055*dt+1.5*dt)*kk, "")
	}
	if hk == "toxic" {
		w.hurt(P.MHP*.025*dt*kk, "")
	}
	if P.WebT > 0 {
		if !w.abSlowImm() {
			spd *= .65
		}
		P.WebT -= dt
	}
	return
}

var _ = rand.Int

// ChargingUp: a charger is winding up (drawn as a warning ring).
func (e *Enemy) ChargingUp() bool { return e.cwu > 0 }
