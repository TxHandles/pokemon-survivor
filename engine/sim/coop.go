package sim

import (
	"fmt"
	"math"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// Co-op: up to four players share one simulation (web PL / useP). Each player owns a context (stats,
// moves, items, ability...) that is swapped into the World's player fields while that player acts, so
// the single-player code keeps working unchanged. Solo play is co-op with one player.

// PlayerSpec describes a player joining a run.
type PlayerSpec struct {
	ID, Name string
	Kind     string // local | bot | net
	Ability  int
	Up       map[string]int
	Training int
}

// Prompt is a pending choice for one player.
type Prompt struct {
	Kind  string // lv | shrine
	Boons []Boon
	Opts  []Option
}

// PlayerCtx is one player's state.
type PlayerCtx struct {
	Idx                        int
	Name, Kind                 string
	P                          *Player
	Slots                      []*Slot
	Held                       map[string]int
	HeldOrder                  []string
	PS                         *PS
	ab                         abState
	DMG                        map[string]float64
	Ban                        map[string]bool
	Rerolls, Banishes          int
	Learned, Evolved, Capstone map[string]bool
	Orbs                       []Orb
	Up                         map[string]int
	Training, Ability          int
	Bot                        *Bot
	In                         Input
	Queue                      []*Prompt
	Deaths, Revives, Kills     int
	Gone                       bool
	DT                         float64 // time of death
}

// Center is a Pokémon Center that revives fainted teammates (web PCN).
type Center struct {
	X, Y, P, T float64
	Guard      *Enemy
	Who        int
}

// coC: scaling by player count (index = players), from the web bot sweeps.
var coC = struct{ hp, rt, sw, cap, xp, dm [5]float64 }{
	hp: [5]float64{1, 1, 1.4, 1.55, 1.75}, rt: [5]float64{1, 1, 1.9, 3.2, 3.0}, sw: [5]float64{1, 1, .65, .55, .5},
	cap: [5]float64{320, 320, 480, 640, 800}, xp: [5]float64{1, 1, .6, .45, .36}, dm: [5]float64{1, 1, 1.1, 1.15, 1.15},
}

func (w *World) n() int { return len(w.PL) }

// coRt: extra co-op spawn rate, ramping in between 30 and 240 seconds.
func (w *World) coRt() float64 {
	n := w.n()
	if n <= 1 {
		return 1
	}
	u := math.Max(0, math.Min(1, (w.T-30)/210))
	return 1 + (coC.rt[n]-1)*(.3+.7*u)
}

func (w *World) coCap() float64 { return coC.cap[min(4, w.n())] }

// saveCtx writes the World's player fields back into the current context.
func (w *World) saveCtx() {
	c := w.cur
	if c == nil {
		return
	}
	c.P, c.Slots, c.Held, c.HeldOrder, c.PS, c.ab, c.DMG = w.P, w.Slots, w.Held, w.HeldOrder, w.PS, w.ab, w.DMG
	c.Ban, c.Rerolls, c.Banishes = w.Ban, w.Rerolls, w.Banishes
	c.Learned, c.Evolved, c.Capstone, c.Orbs = w.Learned, w.Evolved, w.Capstone, w.Orbs
}

// useP makes player i the current one.
func (w *World) useP(i int) {
	if i < 0 || i >= len(w.PL) || w.PL[i] == w.cur {
		return
	}
	w.saveCtx()
	c := w.PL[i]
	w.cur, w.CUR = c, i
	w.P, w.Slots, w.Held, w.HeldOrder, w.PS, w.ab, w.DMG = c.P, c.Slots, c.Held, c.HeldOrder, c.PS, c.ab, c.DMG
	w.Ban, w.Rerolls, w.Banishes = c.Ban, c.Rerolls, c.Banishes
	w.Learned, w.Evolved, w.Capstone, w.Orbs = c.Learned, c.Evolved, c.Capstone, c.Orbs
	w.Cfg.Up, w.Cfg.Training, w.Ability = c.Up, c.Training, c.Ability
}

// with runs f as player i and switches back.
func (w *World) with(i int, f func()) {
	c := w.CUR
	w.useP(i)
	f()
	w.useP(c)
}

func (w *World) alive(i int) bool { c := w.PL[i]; return !c.Gone && !c.P.Dead }

func (w *World) aliveIdx() []int {
	var l []int
	for i := range w.PL {
		if w.alive(i) {
			l = append(l, i)
		}
	}
	return l
}

// nearP: the alive player nearest to (x, y) (the current one when nobody is alive).
func (w *World) nearP(x, y float64) int {
	b, bd := w.CUR, math.Inf(1)
	for i := range w.PL {
		if !w.alive(i) {
			continue
		}
		q := w.PL[i].P
		if d := (q.X-x)*(q.X-x) + (q.Y-y)*(q.Y-y); d < bd {
			bd, b = d, i
		}
	}
	return b
}

// nearIn: an alive player within r of (x, y), or -1.
func (w *World) nearIn(x, y, r float64) int {
	i := w.nearP(x, y)
	if !w.alive(i) || dist(w.PL[i].P.X, w.PL[i].P.Y, x, y) >= r {
		return -1
	}
	return i
}

func (w *World) rndAlive() int {
	l := w.aliveIdx()
	if len(l) == 0 {
		return w.CUR
	}
	return l[w.rng.Intn(len(l))]
}

// centroid of the alive players (camera / spawn reference).
func (w *World) Centroid() (float64, float64) {
	l := w.aliveIdx()
	if len(l) == 0 {
		return w.P.X, w.P.Y
	}
	x, y := 0.0, 0.0
	for _, i := range l {
		x += w.PL[i].P.X
		y += w.PL[i].P.Y
	}
	return x / float64(len(l)), y / float64(len(l))
}

// addPlayer builds a context for spec and appends it.
func (w *World) addPlayer(db *content.DB, spec PlayerSpec) (*PlayerCtx, error) {
	p := db.ByID[spec.ID]
	if p == nil {
		return nil, fmt.Errorf("unknown Pokémon %q", spec.ID)
	}
	st := db.Stats[spec.ID]
	c := &PlayerCtx{Idx: len(w.PL), Name: spec.Name, Kind: spec.Kind, Held: map[string]int{}, DMG: map[string]float64{}, Ban: map[string]bool{},
		Learned: map[string]bool{}, Evolved: map[string]bool{}, Capstone: map[string]bool{}, Up: spec.Up, Training: spec.Training, Ability: spec.Ability & 1}
	if c.Up == nil {
		c.Up = map[string]int{}
	}
	if c.Name == "" {
		c.Name = fmt.Sprintf("Player %d", len(w.PL)+1)
	}
	if c.Kind == "" {
		c.Kind = "local"
	}
	up := func(k string) float64 { return float64(c.Up[k]) }
	ab := db.AbilityPair(spec.ID)[c.Ability]
	c.PS = newPS(ab)
	if pl := c.PS.L("pulse"); len(pl) == 3 {
		c.ab.pt = pl[0]
	}
	mh := math.Round((80 + 14*up("hp")) * b2f(w.Run.Glass, .5, 1) * (1 + c.PS.F("hpM")) * st.HP)
	c.P = &Player{ID: spec.ID, Name: p.Name, Types: p.Types, Stats: st, HP: mh, MHP: mh, Sp: 125 * st.Spd * (1 + .03*up("spd")), Lv: 1, NX: 5, FX: 1, DQ: -1, AtkT: -9}
	sm := db.StartMove(spec.ID)
	c.Slots = []*Slot{{ID: sm, Move: db.Moves[sm], Kind: db.Moves[sm].Shape, Type: db.Moves[sm].Type, Lv: 1, Q: BuildQ(db.Moves[sm]), Owner: len(w.PL)}}
	c.Learned[sm] = true
	c.Rerolls, c.Banishes = c.Up["rr"], c.Up["bn"]
	if c.Kind == "bot" {
		c.Bot = NewBot(w, w.Seed+int64(len(w.PL))*31+7)
	}
	w.PL = append(w.PL, c)
	return c, nil
}

// SetInput sets player i's input for the next Step (player 0's comes from Step itself).
func (w *World) SetInput(i int, in Input) {
	if i >= 0 && i < len(w.PL) {
		w.PL[i].In = in
	}
}

// ---- deaths and revives ----

// pdie: the current player fainted. Returns true when the run is over.
func (w *World) pdie() bool {
	P := w.P
	if w.n() <= 1 {
		return true
	}
	c := w.cur
	if P.Dead {
		return len(w.aliveIdx()) == 0
	}
	P.Dead, P.HP, P.Inv, P.Dash = true, 0, 0, nil
	c.Deaths++
	c.DT = w.T
	w.Orbs = nil
	w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 50, Life: 3, Text: c.Name + " fainted!", Col: "crit"})
	if len(w.aliveIdx()) == 0 {
		return true
	}
	w.pcSpawn()
	return false
}

func (w *World) pcSpawn() {
	if w.PCN != nil {
		return
	}
	cx, cy := w.Centroid()
	a, d := w.rnd(0, 6.283), w.rnd(650, 900)
	x, y := cx+math.Cos(a)*d, cy+math.Sin(a)*d
	w.push(&x, &y, 30)
	w.PCN = &Center{X: x, Y: y, Who: -1}
	w.spawnEliteAt(false, x+50, y+10, false)
	if g := w.E[len(w.E)-1]; g.Elite {
		g.guard = w.PCN
		w.PCN.Guard = g
	}
	names := ""
	for _, c := range w.PL {
		if c.P.Dead && !c.Gone {
			if names != "" {
				names += " & "
			}
			names += c.Name
		}
	}
	w.message("A Pokémon Center is nearby! Reach it to revive " + names + " (an elite guards it)")
	w.sfx("elite")
}

func (w *World) pcTick(dt float64) {
	pc := w.PCN
	if pc == nil {
		return
	}
	pc.T += dt
	anyDead := false
	for _, c := range w.PL {
		anyDead = anyDead || (c.P.Dead && !c.Gone)
	}
	if !anyDead {
		if pc.Guard != nil {
			pc.Guard.guard = nil
		}
		w.PCN = nil
		return
	}
	i := w.nearIn(pc.X, pc.Y, 60)
	if i >= 0 {
		pc.P += dt
	} else {
		pc.P = math.Max(0, pc.P-dt*.5)
	}
	pc.Who = i
	if pc.P >= 1.5 {
		names := ""
		n := 0
		for _, c := range w.PL {
			if !c.P.Dead || c.Gone {
				continue
			}
			p := c.P
			p.Dead, p.HP, p.Inv = false, math.Round(p.MHP*.5), 3
			p.X, p.Y = pc.X+w.rnd(-30, 30), pc.Y+w.rnd(-30, 30)
			if names != "" {
				names += " & "
			}
			names += c.Name
			n++
		}
		if i >= 0 {
			w.PL[i].Revives += n
		}
		w.addFX(&FX{Kind: "txt", X: pc.X, Y: pc.Y - 60, Life: 3, Text: names + " revived!", Col: "heal"})
		w.sfx("heal")
		if pc.Guard != nil {
			pc.Guard.guard = nil
		}
		w.PCN = nil
	}
}

// ---- shared XP and prompts ----

func (w *World) coopLevel() {
	x := &w.XPS
	x.XP -= x.NX
	x.Lv++
	if x.Lv <= 13 {
		x.NX = math.Round(x.NX*1.22 + 4)
	} else {
		x.NX = math.Round(x.NX*1.075 + 7)
	}
	for i, c := range w.PL {
		w.with(i, func() {
			P := w.P
			P.XP, P.Lv, P.NX = x.XP, x.Lv, x.NX
			if !w.Run.NoHeal && !P.Dead {
				P.HP = math.Min(P.MHP, P.HP+P.MHP*b2f(P.Lv <= 20, .06, .015))
			}
		})
		if !c.Gone {
			c.Queue = append(c.Queue, &Prompt{Kind: "lv"})
		}
	}
	w.sfx("levelup")
	w.prompt()
}

// prompt serves pending choices: bots answer at once; the first local player with a prompt gets the
// screen (Chooser); when nobody is waiting, play resumes.
func (w *World) prompt() {
	for i, c := range w.PL {
		if c.Kind != "bot" && !c.Gone {
			continue
		}
		for len(c.Queue) > 0 {
			pr := c.Queue[0]
			c.Queue = c.Queue[1:]
			if c.Gone {
				continue
			}
			w.with(i, func() {
				switch pr.Kind {
				case "lv":
					if o := w.genOpts(); len(o) > 0 {
						w.applyOpt(o[w.botPickFrom(o)])
					}
				case "shrine":
					w.applyBoon(pr.Boons[0])
				}
			})
		}
	}
	for i, c := range w.PL {
		if c.Gone || len(c.Queue) == 0 {
			continue
		}
		pr := c.Queue[0]
		w.useP(i)
		w.Chooser = i
		switch pr.Kind {
		case "lv":
			if pr.Opts == nil {
				pr.Opts = w.genOpts()
				if len(pr.Opts) == 0 { // nothing to offer: full heal instead (web)
					w.P.HP = w.P.MHP
					c.Queue = c.Queue[1:]
					w.prompt()
					return
				}
			}
			w.Opts = pr.Opts
			w.State = StateLevelUp
		case "shrine":
			w.Boons = pr.Boons
			w.State = StateShrine
		}
		return
	}
	w.Chooser = -1
	if w.State == StateLevelUp || w.State == StateShrine {
		w.State = StatePlay
	}
}

// ChooserName is the player currently choosing on a shared screen.
func (w *World) ChooserName() string {
	if w.Chooser >= 0 && w.Chooser < len(w.PL) {
		return w.PL[w.Chooser].Name
	}
	return ""
}

func (w *World) applyBoon(b Boon) {
	b.apply(w)
	if b.Curse != "" {
		w.Stats.Curses++
	}
	w.addFX(&FX{Kind: "txt", X: w.P.X, Y: w.P.Y - 50, Life: 2, Text: b.Name + "!", Col: "gold"})
}

// Standing is one player's line on the results screen.
type Standing struct {
	Name, ID                   string
	Kills, Lv, Deaths, Revives int
	Dmg                        float64
	Alive                      bool
}

func (w *World) Standings() []Standing {
	var out []Standing
	for _, c := range w.PL {
		d := 0.0
		for _, v := range c.DMG {
			d += v
		}
		if c == w.cur {
			d = 0
			for _, v := range w.DMG {
				d += v
			}
		}
		out = append(out, Standing{Name: c.Name, ID: c.P.ID, Kills: c.Kills, Lv: c.P.Lv, Deaths: c.Deaths, Revives: c.Revives, Dmg: d, Alive: !c.P.Dead && !c.Gone})
	}
	return out
}

// freeLevel grants a level's worth of XP to the shared pool (chests, candy, boss chests).
func (w *World) freeLevel() {
	w.XPS.XP += w.XPS.NX
	for _, c := range w.PL {
		c.P.XP = w.XPS.XP
	}
}

// UseLocal switches the World's player fields to player i (clients call it before reading P / Slots).
func (w *World) UseLocal(i int) { w.useP(i) }

// Drop marks player i as gone (disconnected): they stop playing, their pending choices are
// skipped and, if everyone else is down, the run ends.
func (w *World) Drop(i int) {
	if i < 0 || i >= len(w.PL) || w.PL[i].Gone {
		return
	}
	c := w.PL[i]
	c.Gone = true
	c.Queue = nil
	c.In = Input{}
	if w.Chooser == i && (w.State == StateLevelUp || w.State == StateShrine) {
		w.Opts, w.Boons = nil, nil
		w.State = StatePlay
		w.prompt()
	}
	if len(w.aliveIdx()) == 0 && w.State != StateDead {
		w.State = StateDead
	}
}
