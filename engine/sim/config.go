package sim

import (
	"math"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// Config is everything the meta layer feeds into a run: shop upgrades, training, difficulty, modifiers,
// run length and the boss-reward hook.
type Config struct {
	Ability  int
	LI       int            // starting location index (-1 = random)
	Up       map[string]int // shop levels: dmg spd hp reg arm mag xpg cg rr bn m3 m4 h3 h4 ch rt
	Training int            // training rank (+2% damage)
	Tier     int            // difficulty tier 0-10
	Mods     []string       // swift tough brutal horde famine glass titans drought
	Stage    int            // 1-3: 15 / 30 / 45 minute runs (3 / 6 / 9 bosses)
	Lock     bool           // stay in the starting biome (no route map)
	// OnBoss is called when a boss falls (biome, boss type, location, final boss?, biomes beaten this run);
	// the returned messages are shown in the run.
	OnBoss func(bi, et string, li int, fin bool, visited []string) []string
}

// Run holds the difficulty multipliers derived from Config (web RUN).
type Run struct {
	Tier                   int
	Mods                   []string
	HM, SM, DM, RT, BH, XM float64
	NoHeal, Glass          bool
	Mult                   float64 // coin reward multiplier
	Stage, Bosses          int
	Len                    float64
	Lock                   bool
}

var modBonus = map[string]float64{"swift": .10, "tough": .15, "brutal": .10, "horde": .15, "famine": .15, "glass": .20, "titans": .10, "drought": .10}

func makeRun(c Config) Run {
	k := float64(c.Tier)
	hasM := func(m string) bool { return has(c.Mods, m) }
	r := Run{Tier: c.Tier, Mods: c.Mods, HM: 1 + .12*k, DM: 1 + .08*k, SM: 1 + .02*k, RT: 1 + .05*k, BH: 1, XM: 1, Mult: 1 + .12*k, Lock: c.Lock}
	if hasM("tough") {
		r.HM *= 1.4
	}
	if hasM("swift") {
		r.SM *= 1.25
	}
	if hasM("brutal") {
		r.DM *= 1.4
	}
	if hasM("horde") {
		r.RT *= 1.5
	}
	r.NoHeal, r.Glass = hasM("famine"), hasM("glass")
	if hasM("titans") {
		r.BH = 1.6
	}
	if hasM("drought") {
		r.XM = .7
	}
	for _, m := range c.Mods {
		r.Mult += modBonus[m]
	}
	r.Stage = max(1, min(3, c.Stage))
	r.Bosses, r.Len = 3*r.Stage, float64(900*r.Stage)
	return r
}

func (w *World) up(k string) float64 { return float64(w.Cfg.Up[k]) }

// NewWith starts a run with meta progression applied.
func NewWith(db *content.DB, id, biome string, seed int64, cfg Config) (*World, error) {
	if cfg.Up == nil {
		cfg.Up = map[string]int{}
	}
	w, err := newWorld(db, id, biome, seed, cfg)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// spec enemies from difficulty tier 2+ (web SPK / spec)
var specKinds = []struct {
	k    string
	tier int
}{{"spit", 2}, {"chg", 3}, {"bomb", 4}, {"shld", 6}, {"wisp", 8}}

func (w *World) maybeSpec(e *Enemy) {
	if w.T < 100 || w.Run.Tier < 2 || w.rng.Float64() >= math.Min(.3, .04+.025*float64(w.Run.Tier)) {
		return
	}
	var av []string
	for _, s := range specKinds {
		if w.Run.Tier >= s.tier {
			av = append(av, s.k)
		}
	}
	k := av[w.rng.Intn(len(av))]
	e.Sp2 = k
	switch k {
	case "spit", "bomb":
		e.R = 11
	case "chg":
		e.Sp *= .9
	case "shld":
		e.Big, e.R = true, 13
		e.Sh = e.HP * .8
		e.ShMax = e.Sh
	case "wisp":
		e.HP *= 1.5
		e.MHP = e.HP
		e.R = 12
	}
}

// specAI: difficulty specials (web ai0 spit / wisp / chg / bomb).
func (w *World) specAI(e *Enemy, dt float64) float64 {
	P := w.P
	dx, dy := P.X-e.X, P.Y-e.Y
	d := math.Max(hyp(dx, dy), 1)
	ang := math.Atan2(dy, dx)
	m := 1.0
	switch e.Sp2 {
	case "spit", "wisp":
		keep := 210.0
		if e.Sp2 == "wisp" {
			keep = 300
		}
		if d < keep-30 {
			e.steer(ang + math.Pi)
		} else if d < keep+30 {
			m = 0
		}
		if e.st == 0 {
			e.st = w.rnd(.5, 2)
		}
		e.st -= dt
		if e.st <= 0 {
			if e.Sp2 == "wisp" {
				e.st = 6
				if d < 650 && len(w.E) < 500 {
					for i := 0; i < 2; i++ {
						w.minion(e.X+w.rnd(-14, 14), e.Y+w.rnd(-14, 14), e.Type)
					}
				}
			} else {
				e.st = 2.6
				if d < 460 {
					w.EB = append(w.EB, &EBullet{X: e.X, Y: e.Y, VX: dx / d * 200, VY: dy / d * 200, L: 3, R: 6, Dm: e.Dm * .7, Type: e.Type})
				}
			}
		}
	case "chg":
		switch {
		case e.wu > 0:
			e.wu -= dt
			m = 0
			if e.wu <= 0 {
				e.chd, e.ca = .7, ang
			}
		case e.chd > 0:
			e.chd -= dt
			e.steer(e.ca)
			m = 5.5
		default:
			if e.cc == 0 {
				e.cc = 2
			}
			e.cc -= dt
			if e.cc <= 0 && d < 320 {
				e.wu, e.cc = .6, 3.2
			}
		}
	case "bomb":
		m = 1.35
		if d < 44 {
			e.Dead = true
			w.HZ = append(w.HZ, &Hazard{X: e.X, Y: e.Y, R: 70, T: .8, T0: .8, Dm: e.Dm * 1.5, Type: e.Type})
		}
	}
	return m
}

// ---- rerolls and banishes on level-up ----

// Reroll redraws the level-up cards (uses one reroll).
func (w *World) Reroll() bool {
	if w.State != StateLevelUp || w.Rerolls <= 0 {
		return false
	}
	w.Rerolls--
	w.Opts = w.genOpts()
	return true
}

// Banish removes card i from this run's offers for good and redraws (uses one banish).
func (w *World) Banish(i int) bool {
	if w.State != StateLevelUp || w.Banishes <= 0 || i < 0 || i >= len(w.Opts) {
		return false
	}
	w.Banishes--
	o := w.Opts[i]
	w.Ban[banKey(o)] = true
	w.Opts = w.genOpts()
	if len(w.Opts) == 0 {
		w.Opts = nil
		w.State = StatePlay
	}
	return true
}

func banKey(o Option) string {
	if o.Evo {
		return o.ID + "!"
	}
	return o.ID
}
