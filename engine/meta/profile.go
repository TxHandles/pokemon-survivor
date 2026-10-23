package meta

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// Profile is a save plus the game data it is read against.
type Profile struct {
	S  *Save
	DB *content.DB
}

func NewProfile(s *Save, db *content.DB) *Profile {
	p := &Profile{S: s, DB: db}
	p.migrate()
	p.ChkUnl()
	p.ColStats()
	return p
}

// migrate fills derived tables old saves may lack (web mrSeed / csInit).
func (p *Profile) migrate() {
	s := p.S
	if s.Num("mxs") == 0 {
		mx := s.obj("mx")
		for _, h := range s.hist() {
			c, _ := h["c"].(string)
			if c == "" {
				continue
			}
			r := p.LineRoot(c)
			mx[r] = num(mx[r]) + float64(MrGain(num(h["tm"]), num(h["k"]), num(h["w"]) > 0))
		}
		s.M["mxs"] = 1.0
	}
	if s.Num("csb") == 0 {
		s.M["cs"] = map[string]any{}
		for _, h := range s.hist() {
			p.addCS(h)
		}
		s.M["csb"] = 1.0
	}
}

func (s *Save) hist() []map[string]any {
	l, _ := s.M["hist"].([]any)
	var out []map[string]any
	for _, x := range l {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// ---- families ----

func (p *Profile) LineRoot(id string) string { return p.DB.LineRoot(id) }

// Fam: the upgrade family (regional forms share their original's line; web fam()).
func (p *Profile) Fam(id string) string {
	r := p.LineRoot(id)
	if c := p.DB.ByID[r]; c != nil && c.RegionalOf != "" {
		return p.LineRoot(c.RegionalOf)
	}
	return r
}

// ---- unlocks ----

type rule struct {
	Tickets   *int     `json:"tickets"`
	Mastery   *int     `json:"mastery"`
	Line      string   `json:"line"`
	Bosses    int      `json:"bosses"`
	Counter   string   `json:"counter"`
	Boss      bool     `json:"boss"`
	Flag      string   `json:"flag"`
	DoneFlag  string   `json:"doneFlag"`
	As        []string `json:"as"`
	AsLine    string   `json:"asLine"`
	BossType  string   `json:"bossType"`
	Biome     string   `json:"biome"`
	ClearTier *int     `json:"clearTier"`
	All       []rule   `json:"all"`
}

func (p *Profile) rule(id string) *rule {
	c := p.DB.ByID[id]
	if c == nil || len(c.Unlock) == 0 {
		return nil
	}
	var r rule
	if json.Unmarshal(c.Unlock, &r) != nil {
		return nil
	}
	return &r
}

// TicketCost: tickets needed to unlock id (0 = not bought with tickets).
func (p *Profile) TicketCost(id string) int {
	c := p.DB.ByID[id]
	if c == nil {
		return 0
	}
	if c.BaseStats != nil {
		return c.Tickets
	}
	if r := p.rule(id); r != nil && r.Tickets != nil {
		return *r.Tickets
	}
	return 0
}

// Unl reports whether a Pokémon is playable (web unl()).
func (p *Profile) Unl(id string) bool {
	c := p.DB.ByID[id]
	if c == nil {
		return false
	}
	if c.BaseStats != nil {
		return c.Tickets == 0 || p.S.Unlocked(id)
	}
	return p.S.Unlocked(id)
}

func (p *Profile) urOK(r rule) bool {
	switch {
	case r.All != nil:
		for _, x := range r.All {
			if !p.urOK(x) {
				return false
			}
		}
		return true
	case r.Tickets != nil:
		return false
	case r.Mastery != nil:
		return p.BestRank(r.Line) >= *r.Mastery
	case r.ClearTier != nil:
		return p.BiomeStars(r.Biome) >= *r.ClearTier
	}
	k := r.Counter
	if k == "" {
		k = r.Flag
	}
	v := p.S.Flag(k)
	if r.Counter != "" {
		return int(v) >= r.Bosses
	}
	return v > 0
}

func (p *Profile) urProg(r rule) (int, int) {
	switch {
	case r.All != nil:
		for _, x := range r.All {
			if !p.urOK(x) {
				return p.urProg(x)
			}
		}
		return p.urProg(r.All[len(r.All)-1])
	case r.Tickets != nil:
		return 0, *r.Tickets
	case r.Mastery != nil:
		return min(p.BestRank(r.Line), *r.Mastery), *r.Mastery
	case r.ClearTier != nil:
		return min(*r.ClearTier, p.BiomeStars(r.Biome)), *r.ClearTier
	}
	k := r.Counter
	if k == "" {
		k = r.Flag
	}
	v := int(p.S.Flag(k))
	if r.Counter != "" {
		return min(v, r.Bosses), r.Bosses
	}
	return min(v, 1), 1
}

// Progress toward a rule-based unlock (have, need).
func (p *Profile) Progress(id string) (int, int) {
	if r := p.rule(id); r != nil {
		return p.urProg(*r)
	}
	return 0, 1
}

// ChkUnl unlocks every Pokémon whose rule is now met; returns the new ones.
func (p *Profile) ChkUnl() []string {
	var out []string
	for _, c := range p.DB.Pokemon {
		if c.BaseStats != nil || p.S.Unlocked(c.ID) {
			continue
		}
		if r := p.rule(c.ID); r != nil && p.urOK(*r) {
			p.S.SetUnlocked(c.ID)
			out = append(out, c.ID)
		}
	}
	return out
}

// BuyUnlock spends tickets on a ticket-priced Pokémon.
func (p *Profile) BuyUnlock(id string) bool {
	c := p.TicketCost(id)
	if c <= 0 || p.Unl(id) || p.S.Tickets() < c {
		return false
	}
	p.S.Add("tickets", -float64(c))
	p.S.SetUnlocked(id)
	p.ColStats()
	return true
}

// UnlockText describes how to get a locked Pokémon.
func (p *Profile) UnlockText(id string) string {
	if c := p.TicketCost(id); c > 0 {
		return fmt.Sprintf("Unlock with %d boss-chest ticket%s", c, plural(c))
	}
	if c := p.DB.ByID[id]; c != nil && c.UnlockText != "" {
		return c.UnlockText
	}
	return "Available from the start"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ---- mastery (XP per line, web mrInfo) ----

func mrNeed(r int) int { return 290 + 47*min(r, 10) }

// MrGain: mastery XP for a run (1/s survived, 0.2 per defeat, +300 for a win, x1.5).
func MrGain(t, kills float64, win bool) int {
	w := 0.0
	if win {
		w = 300
	}
	return int(math.Round((t + kills*.2 + w) * 1.5))
}

// Mastery returns rank, XP into the rank and XP needed for the next one.
func (p *Profile) Mastery(root string) (rank, into, need int) {
	x := int(math.Floor(num(p.S.obj("mx")[root])))
	r := 1
	for x >= mrNeed(r) && r < MasteryMax {
		x -= mrNeed(r)
		r++
	}
	if r >= MasteryMax {
		return MasteryMax, mrNeed(r), mrNeed(r)
	}
	return r, x, mrNeed(r)
}

func (p *Profile) MasteryRank(root string) int { r, _, _ := p.Mastery(root); return r }

// ---- shop (web UP) ----

type Upgrade struct {
	Key, Name, Desc string
	Max, Cost       int
	Req             string
	Lock            *Lock // achievement gate
	Global          bool
}

// LockStat is stat k for an unlock rule. "bm" (bosses beaten in one run) is new, so older saves that
// already survived 15:00 count as having beaten the 10:00 boss.
func (p *Profile) LockStat(k string) float64 {
	v := p.S.Stat(k)
	if k == "bm" && p.S.Stat("t") >= 900 {
		v = math.Max(v, 2)
	}
	return v
}

type Lock struct {
	Stat string
	Need float64
	Text string
}

var Upgrades = []Upgrade{
	{Key: "dmg", Name: "Power", Desc: "+8% damage", Max: 10, Cost: 30},
	{Key: "spd", Name: "Swiftness", Desc: "+3% speed", Max: 10, Cost: 25},
	{Key: "hp", Name: "Vitality", Desc: "+14 max HP", Max: 10, Cost: 30},
	{Key: "reg", Name: "Recovery", Desc: "+0.3 HP/sec", Max: 10, Cost: 40},
	{Key: "arm", Name: "Armor", Desc: "-4% damage taken", Max: 8, Cost: 50},
	{Key: "mag", Name: "Magnet", Desc: "+12% pickup range", Max: 5, Cost: 30},
	{Key: "xpg", Name: "Fast Learner", Desc: "+5% XP gain", Max: 5, Cost: 45},
	{Key: "mxg", Name: "Dedicated Trainer", Desc: "+10% Mastery XP", Max: 5, Cost: 50},
	{Key: "cg", Name: "Lucky Coin", Desc: "+10% coins", Max: 5, Cost: 60},
	{Key: "rr", Name: "Reroll", Desc: "+1 reroll per run", Max: 10, Cost: 120, Global: true},
	{Key: "bn", Name: "Banish", Desc: "+1 banish per run", Max: 10, Cost: 120, Global: true},
	{Key: "m3", Name: "Attack Slot 3", Desc: "Equip a 3rd attack", Max: 1, Cost: 800, Global: true, Lock: &Lock{"k", 5000, "Defeat 5,000 enemies"}},
	{Key: "m4", Name: "Attack Slot 4", Desc: "Equip a 4th attack", Max: 1, Cost: 2000, Global: true, Req: "m3", Lock: &Lock{"k", 10000, "Defeat 10,000 enemies"}},
	{Key: "h3", Name: "Held Item Slot 3", Desc: "Equip a 3rd held item", Max: 1, Cost: 700, Global: true, Lock: &Lock{"bm", 2, "Defeat the 10:00 boss"}},
	{Key: "h4", Name: "Held Item Slot 4", Desc: "Equip a 4th held item", Max: 1, Cost: 1800, Global: true, Req: "h3", Lock: &Lock{"t", 1800, "Survive 30:00 in one run"}},
	{Key: "ch", Name: "Wider Choice", Desc: "+1 level-up choice (3 → 4 → 5)", Max: 2, Cost: 1500, Global: true},
	{Key: "rt", Name: "Route Planner", Desc: "+1 biome to choose from on the route map after each boss", Max: 3, Cost: 3000, Global: true},
}

var upgradeByKey = func() map[string]*Upgrade {
	m := map[string]*Upgrade{}
	for i := range Upgrades {
		m[Upgrades[i].Key] = &Upgrades[i]
	}
	return m
}()

// UL: upgrade level (line upgrades are per family, global ones shared).
func (p *Profile) UL(key, fam string) int {
	u := upgradeByKey[key]
	if u == nil {
		return 0
	}
	if u.Global {
		return int(num(p.S.obj("gu")[key]))
	}
	return int(num(sub(p.S.obj("cu"), fam)[key]))
}

func (p *Profile) UpCost(key, fam string) int {
	u := upgradeByKey[key]
	return int(math.Round(float64(u.Cost) * (1 + float64(p.UL(key, fam))*.8)))
}

// UpState explains whether an upgrade can be bought ("" = yes).
func (p *Profile) UpState(key, fam string) string {
	u := upgradeByKey[key]
	l := p.UL(key, fam)
	switch {
	case l >= u.Max:
		if u.Max == 1 {
			return "OWNED"
		}
		return "MAX"
	case u.Lock != nil && l == 0 && p.LockStat(u.Lock.Stat) < u.Lock.Need:
		return "Locked: " + u.Lock.Text
	case u.Req != "" && p.UL(u.Req, fam) == 0:
		return "Needs " + upgradeByKey[u.Req].Name
	case p.S.Coins() < p.UpCost(key, fam):
		return "Not enough coins"
	}
	return ""
}

func (p *Profile) BuyUpgrade(key, fam string) bool {
	if p.UpState(key, fam) != "" {
		return false
	}
	c := p.UpCost(key, fam)
	p.S.Add("coins", -float64(c))
	u := upgradeByKey[key]
	if u.Global {
		p.S.obj("gu")[key] = float64(p.UL(key, fam) + 1)
	} else {
		sub(p.S.obj("cu"), fam)[key] = float64(p.UL(key, fam) + 1)
	}
	return true
}

// Training rank (SV.ms): +1% HP and +1% damage per rank, up to 20.
const TrainingMax = 20

func (p *Profile) Training(fam string) int { return int(num(p.S.obj("ms")[fam])) }
func TrainingCost(r int) int               { return int(math.Round(2500 * (1 + float64(r)*.6))) }

func (p *Profile) BuyTraining(fam string) bool {
	r := p.Training(fam)
	c := TrainingCost(r)
	if r >= TrainingMax || p.S.Coins() < c {
		return false
	}
	p.S.Add("coins", -float64(c))
	p.S.obj("ms")[fam] = float64(r + 1)
	return true
}

// ---- hidden abilities: 10,000 kills with the line + 1 ticket ----

const HiddenKills = 10000

func (p *Profile) FamKills(id string) int {
	f := p.Fam(id)
	k := 0.0
	cs := p.S.obj("cs")
	for _, c := range p.DB.Pokemon {
		if p.Fam(c.ID) == f {
			if m, ok := cs[c.ID].(map[string]any); ok {
				k += num(m["k"])
			}
		}
	}
	return int(k)
}

func (p *Profile) HiddenOwned(id string) bool { return num(p.S.obj("pu")[p.Fam(id)]) > 0 }

// AbilityIdx: which ability the line uses (0 starting, 1 hidden).
func (p *Profile) AbilityIdx(id string) int {
	f := p.Fam(id)
	if p.HiddenOwned(id) && num(p.S.obj("pp")[f]) == 1 {
		return 1
	}
	return 0
}

func (p *Profile) SetAbilityIdx(id string, i int) {
	if i == 1 && !p.HiddenOwned(id) {
		return
	}
	p.S.obj("pp")[p.Fam(id)] = float64(i)
}

func (p *Profile) BuyHidden(id string) bool {
	if p.HiddenOwned(id) || p.FamKills(id) < HiddenKills || p.S.Tickets() < 1 {
		return false
	}
	p.S.Add("tickets", -1)
	f := p.Fam(id)
	p.S.obj("pu")[f] = 1.0
	p.S.obj("pp")[f] = 1.0
	p.ColStats()
	return true
}

// ---- biomes ----

func (p *Profile) biome(t string) map[string]any {
	b, _ := p.S.obj("bu")[t].(map[string]any)
	return b
}

func (p *Profile) BiomeUnlocked(t string) bool { b := p.biome(t); return b != nil && num(b["u"]) > 0 }
func (p *Profile) BiomeStars(t string) int {
	if b := p.biome(t); b != nil {
		return int(num(b["s"]))
	}
	return 0
}

// ---- difficulty and run setup (web MODS / tierInfo / mkRun) ----

const TierMax = 10

type Mod struct {
	Key, Name, Desc string
	Bonus           float64
}

var Mods = []Mod{
	{"swift", "Swift Foes", "Enemies move 25% faster", .10},
	{"tough", "Tough Foes", "Enemies have +40% HP", .15},
	{"brutal", "Hard Hitters", "Enemies deal +40% damage", .10},
	{"horde", "Horde", "50% more enemies spawn", .15},
	{"famine", "No Healing", "Recovery, Leftovers, Soothe Bell, lifesteal and berries do nothing", .15},
	{"glass", "Glass Cannon", "Your max HP is halved", .20},
	{"titans", "Titans", "Bosses have +60% HP", .10},
	{"drought", "Drought", "30% less XP from gems", .10},
}

// Asc: highest difficulty tier unlocked (win on tier N unlocks N+1).
func (p *Profile) Asc() int { return int(p.S.Num("asc")) }

// StageMax: run length tiers available (15 / 30 / 45 min) for a biome choice ("random" = any).
func (p *Profile) StageMax(bi string) int {
	best := 0
	if bi == "random" || bi == "" {
		for t := range p.S.obj("bu") {
			best = max(best, p.BiomeStars(t))
		}
	} else {
		best = p.BiomeStars(bi)
	}
	return min(3, 1+best)
}

// ---- challenges (web ACH) ----

type Ach struct {
	ID, Stat, Name string
	Target         float64
	Coins, Tickets int
}

func (p *Profile) Challenges() []Ach {
	var out []Ach
	al := func(s string, l [][3]int, scale float64, name func(int) string) {
		for _, x := range l {
			out = append(out, Ach{ID: fmt.Sprintf("%s:%d", s, x[0]), Stat: s, Target: float64(x[0]) * scale, Coins: x[1], Tickets: x[2], Name: name(x[0])})
		}
	}
	mvn := len(p.DB.MoveList)
	al("k", [][3]int{{500, 50, 0}, {1000, 25, 0}, {2000, 25, 0}, {3000, 40, 0}, {5000, 100, 0}, {7500, 40, 0}, {10000, 250, 0}, {15000, 60, 0}, {20000, 80, 0}, {30000, 0, 1}, {40000, 0, 1}, {50000, 0, 2}, {60000, 0, 1}, {75000, 0, 2}, {100000, 0, 3}}, 1, func(t int) string { return "Defeat " + commas(t) + " enemies" })
	al("b", [][3]int{{3, 50, 0}, {6, 30, 0}, {10, 30, 0}, {15, 75, 0}, {20, 40, 0}, {30, 60, 0}, {40, 150, 0}, {60, 0, 1}, {80, 0, 1}, {100, 0, 2}, {150, 0, 2}, {200, 0, 3}}, 1, func(t int) string { return fmt.Sprintf("Defeat %d bosses", t) })
	al("t", [][3]int{{7, 40, 0}, {9, 25, 0}, {11, 25, 0}, {13, 40, 0}, {15, 150, 0}, {18, 40, 0}, {21, 50, 0}, {24, 60, 0}, {27, 0, 1}, {30, 0, 1}, {35, 0, 2}, {40, 0, 1}, {45, 0, 2}, {50, 0, 2}, {60, 0, 3}}, 60, func(t int) string { return fmt.Sprintf("Survive %d minutes in a single run", t) })
	al("w", [][3]int{{3, 75, 0}, {5, 40, 0}, {8, 40, 0}, {12, 50, 0}, {15, 150, 0}, {20, 60, 0}, {25, 0, 1}, {35, 0, 1}, {50, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Win %d runs (beat the 15:00 boss)", t) })
	al("mv", [][3]int{{10, 40, 0}, {25, 60, 0}, {50, 100, 0}, {75, 150, 0}, {100, 0, 1}, {mvn, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Discover %d different moves", t) })
	al("ev", [][3]int{{5, 50, 0}, {15, 80, 0}, {30, 120, 0}, {60, 0, 1}, {100, 0, 1}, {mvn, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Evolve %d different moves", t) })
	al("cp", [][3]int{{1, 60, 0}, {5, 100, 0}, {15, 150, 0}, {30, 0, 1}, {60, 0, 1}, {100, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Reach Lv 20 with %d different moves", t) })
	al("a", [][3]int{{1, 60, 0}, {2, 80, 0}, {3, 100, 0}, {5, 0, 1}, {7, 0, 2}, {10, 0, 3}}, 1, func(t int) string { return fmt.Sprintf("Win a run on Difficulty %d", t) })
	al("bi", [][3]int{{1, 40, 0}, {3, 60, 0}, {6, 100, 0}, {9, 150, 0}, {12, 0, 1}, {18, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Unlock %d biomes", t) })
	al("bs", [][3]int{{3, 60, 0}, {9, 100, 0}, {18, 200, 0}, {27, 0, 1}, {36, 0, 1}, {54, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Clear %d biome tiers (biome × tier)", t) })
	al("el", [][3]int{{10, 40, 0}, {50, 80, 0}, {150, 120, 0}, {400, 200, 0}, {1000, 0, 1}, {2500, 0, 2}}, 1, func(t int) string { return "Defeat " + commas(t) + " elites or mini-bosses" })
	al("lc", [][3]int{{5, 60, 0}, {10, 100, 0}, {20, 200, 0}, {35, 350, 0}, {54, 0, 3}}, 1, func(t int) string {
		if t == 54 {
			return "Beat a boss in all 54 locations"
		}
		return fmt.Sprintf("Beat a boss in %d different locations", t)
	})
	al("ft", [][3]int{{1, 80, 0}, {3, 150, 0}, {6, 250, 0}, {10, 0, 1}, {18, 0, 3}}, 1, func(t int) string {
		switch t {
		case 1:
			return "Clear all 3 locations of a type"
		case 18:
			return "Clear all 3 locations of every type"
		}
		return fmt.Sprintf("Clear all 3 locations of %d types", t)
	})
	al("rt", [][3]int{{3, 100, 0}, {4, 200, 0}, {5, 0, 1}}, 1, func(t int) string { return fmt.Sprintf("Clear %d different locations in a single run", t) })
	al("ha", [][3]int{{1, 60, 0}, {5, 150, 0}, {10, 300, 0}, {20, 0, 2}}, 1, func(t int) string {
		if t == 1 {
			return "Unlock 1 Hidden Ability"
		}
		return fmt.Sprintf("Unlock %d Hidden Abilities", t)
	})
	al("lg", [][3]int{{1, 0, 2}}, 1, func(int) string { return "Legend Awakened: win a run with a legendary ability" })
	al("pk", [][3]int{{10, 100, 0}, {25, 250, 0}, {40, 0, 1}, {60, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Unlock %d Pokémon", t) })
	al("fe", [][3]int{{5, 100, 0}, {10, 250, 0}, {20, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Unlock the final evolution of %d lines", t) })
	alolan := 0
	legs := 0
	for _, c := range p.DB.Pokemon {
		if c.Form == "Alolan" {
			alolan++
		}
		if c.Legendary {
			legs++
		}
	}
	al("al", [][3]int{{alolan, 0, 1}}, 1, func(t int) string { return fmt.Sprintf("Unlock all %d Alolan forms", t) })
	al("mb", [][3]int{{10, 80, 0}, {30, 200, 0}, {75, 0, 1}}, 1, func(t int) string { return fmt.Sprintf("Defeat %d mini-bosses", t) })
	al("td", [][3]int{{5, 100, 0}, {15, 250, 0}, {40, 0, 2}}, 1, func(t int) string { return fmt.Sprintf("Earn %d tickets from elite and mini-boss drops", t) })
	al("cc", [][3]int{{5, 80, 0}, {20, 200, 0}, {50, 0, 1}}, 1, func(t int) string { return fmt.Sprintf("Open %d cursed chests", t) })
	al("pc", [][3]int{{5, 80, 0}, {20, 200, 0}, {50, 0, 1}}, 1, func(t int) string { return fmt.Sprintf("Accept %d shrine curses", t) })
	al("nr", [][3]int{{1, 0, 1}}, 1, func(int) string { return "No Regrets: win a run after accepting 3 or more shrine curses" })
	for _, x := range [][3]int{{1, 0, 1}, {3, 0, 1}, {5, 0, 2}, {10, 0, 3}, {20, 0, 5}} {
		if x[0] <= legs {
			out = append(out, Ach{ID: fmt.Sprintf("lgc:%d", x[0]), Stat: "lgc", Target: float64(x[0]), Tickets: x[2], Name: fmt.Sprintf("Collect %d legendary Pokémon", x[0])})
		}
	}
	return out
}

func commas(n int) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func (p *Profile) Claimed(a Ach) bool { return num(p.S.obj("cl")[a.ID]) > 0 }
func (p *Profile) Claimable(a Ach) bool {
	return !p.Claimed(a) && p.S.Stat(a.Stat) >= a.Target
}

func (p *Profile) Claim(a Ach) bool {
	if !p.Claimable(a) {
		return false
	}
	if a.Tickets > 0 {
		p.S.Add("tickets", float64(a.Tickets))
	} else {
		p.S.Add("coins", float64(a.Coins))
	}
	p.S.obj("cl")[a.ID] = 1.0
	return true
}

func (p *Profile) ReadyChallenges() int {
	n := 0
	for _, a := range p.Challenges() {
		if p.Claimable(a) {
			n++
		}
	}
	return n
}

// ColStats recomputes collection stats (web colStats).
func (p *Profile) ColStats() {
	s := p.S
	var mv, ev, cp float64
	for id, v := range s.obj("col") {
		if p.DB.Moves[id] == nil {
			continue
		}
		c, _ := v.(map[string]any)
		if num(c["d"]) > 0 {
			mv++
		}
		if num(c["e"]) > 0 {
			ev++
		}
		if num(c["c"]) > 0 {
			cp++
		}
	}
	s.SetStat("mv", mv)
	s.SetStat("ev", ev)
	s.SetStat("cp", cp)
	var bi, bs float64
	for t := range s.obj("bu") {
		if p.BiomeUnlocked(t) {
			bi++
		}
		bs += float64(p.BiomeStars(t))
	}
	s.SetStat("bi", bi)
	s.SetStat("bs", bs)
	var lc, ft float64
	for _, v := range s.obj("locb") {
		m := int(num(v))
		lc += float64(m&1 + m>>1&1 + m>>2&1)
		if m&7 == 7 {
			ft++
		}
	}
	s.SetStat("lc", lc)
	s.SetStat("ft", ft)
	var ha, pk, fe, al, lgc float64
	for f, v := range s.obj("pu") {
		if num(v) > 0 && p.DB.ByID[f] != nil {
			ha++
		}
	}
	children := map[string]bool{}
	for _, c := range p.DB.Pokemon {
		if c.EvolvesFrom != "" {
			children[c.EvolvesFrom] = true
		}
	}
	for _, c := range p.DB.Pokemon {
		if !p.Unl(c.ID) {
			continue
		}
		pk++
		if c.EvolvesFrom != "" && !children[c.ID] {
			fe++
		}
		if c.Form == "Alolan" {
			al++
		}
		if c.Legendary {
			lgc++
		}
	}
	for k, v := range map[string]float64{"ha": ha, "pk": pk, "fe": fe, "al": al, "lgc": lgc} {
		s.SetStat(k, v)
	}
}

// ---- boss rewards (web bossUnlock / urBoss) ----

// BossKill records a boss beaten in biome bi (boss type et) as cid; fin = the run's final boss.
// visited are the biomes beaten this run. Returns messages to show.
func (p *Profile) BossKill(bi, et string, li int, fin bool, stage int, cid string, visited []string) []string {
	var msg []string
	s := p.S
	for _, c := range p.DB.Pokemon {
		r := p.rule(c.ID)
		if r == nil {
			continue
		}
		rs := []rule{*r}
		if r.All != nil {
			rs = r.All
		}
		for _, x := range rs {
			if x.Counter == "" && x.Flag == "" {
				continue
			}
			if x.As != nil && !contains(x.As, cid) {
				continue
			}
			if x.AsLine != "" && p.LineRoot(cid) != x.AsLine {
				continue
			}
			if x.BossType != "" && x.BossType != et {
				continue
			}
			if x.Biome != "" && x.Biome != bi {
				continue
			}
			if x.Counter != "" {
				v := s.Flag(x.Counter) + 1
				s.SetFlag(x.Counter, v)
				if int(v) != x.Bosses {
					continue
				}
			} else {
				if s.Flag(x.Flag) > 0 {
					continue
				}
				s.SetFlag(x.Flag, 1)
			}
			if x.DoneFlag != "" {
				s.SetFlag(x.DoneFlag, 1)
			}
			var mm *rule
			ok := true
			for i := range r.All {
				y := r.All[i]
				if y.Counter == x.Counter && y.Flag == x.Flag {
					continue
				}
				if !p.urOK(y) {
					ok = false
				}
				if y.Mastery != nil {
					mm = &r.All[i]
				}
			}
			if ok || mm == nil {
				msg = append(msg, c.Name+" unlocked!")
			} else {
				msg = append(msg, fmt.Sprintf("%s boss requirement complete — reach Mastery %d to evolve", c.Name, *mm.Mastery))
			}
		}
	}
	bu := s.obj("bu")
	b, _ := bu[bi].(map[string]any)
	had := b != nil && num(b["u"]) > 0
	if b == nil {
		b = map[string]any{"u": 1.0, "s": 0.0}
		bu[bi] = b
	}
	b["u"] = 1.0
	if !contains(visited, bi) {
		visited = append(append([]string{}, visited...), bi)
	}
	if !had {
		msg = append(msg, cap1(bi)+" locations unlocked: "+strings.Join(p.DB.World.Locations[bi], ", ")+"!")
	}
	if fin {
		for _, t := range visited {
			q, _ := bu[t].(map[string]any)
			if q == nil {
				q = map[string]any{"u": 1.0, "s": 0.0}
				bu[t] = q
			}
			q["s"] = math.Max(num(q["s"]), float64(stage))
		}
	}
	for _, id := range p.ChkUnl() {
		if c := p.DB.ByID[id]; c != nil && c.Legendary {
			msg = append(msg, c.Name+" unlocked!")
		}
	}
	lb := s.obj("locb")
	lb[bi] = float64(int(num(lb[bi])) | 1<<li)
	s.MaxStat("rt", float64(len(visited)))
	p.ColStats()
	return msg
}

func contains(l []string, x string) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}

func cap1(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---- end of run (web finish / recRun / addCS) ----

// RunResult is what a finished run reports to the profile.
type RunResult struct {
	CID         string
	T           float64
	Kills, Lv   int
	Win         bool
	Coins       float64 // collected during the run
	Tickets     int
	Elites      int
	MiniBosses  int
	EliteTk     int
	Cursed      int
	Curses      int
	Bosses      int
	Ability     string // ability id
	AbilityName string
	Tier        int
	Mods        []string
	Stage       int
	Mult        float64 // reward multiplier from difficulty
	Visited     []string
	Dmg         map[string]float64
	Moves       [][3]any // id, level, evolved
	Items       [][2]any
	Learned     []string
	Evolved     []string
	Capstone    []string
}

// Finish applies a run's rewards. Returns coins gained, mastery XP and new unlocks.
func (p *Profile) Finish(r RunResult) (gain int, mxGain int, unlocked []string) {
	s := p.S
	root := p.LineRoot(r.CID)
	mxGain = int(math.Round(float64(MrGain(r.T, float64(r.Kills), r.Win)) * (1 + .1*float64(p.UL("mxg", p.Fam(r.CID))))))
	mx := s.obj("mx")
	mx[root] = num(mx[root]) + float64(mxGain)
	p.noteBest(root)
	w := 0.0
	if r.Win {
		w = 100
	}
	mult := r.Mult
	if mult == 0 {
		mult = 1
	}
	gain = int(math.Floor((r.Coins + w + r.T/30) * mult))
	s.Add("coins", float64(gain))
	s.Add("tickets", float64(r.Tickets))
	s.AddStat("k", float64(r.Kills))
	s.AddStat("el", float64(r.Elites))
	s.AddStat("mb", float64(r.MiniBosses))
	s.AddStat("td", float64(r.EliteTk))
	s.AddStat("cc", float64(r.Cursed))
	s.AddStat("pc", float64(r.Curses))
	if r.Win && r.Curses >= 3 {
		s.SetStat("nr", 1)
	}
	if r.Win && strings.HasSuffix(r.Ability, ":leg") {
		s.SetStat("lg", 1)
	}
	s.AddStat("b", float64(r.Bosses))
	s.MaxStat("t", r.T)
	s.MaxStat("bm", float64(r.Bosses)) // most bosses beaten in one run
	s.AddStat("r", 1)
	if r.Win {
		s.AddStat("w", 1)
		s.SetNum("asc", math.Max(s.Num("asc"), math.Min(TierMax, float64(r.Tier+1))))
		s.MaxStat("a", float64(r.Tier+1))
	}
	col := s.obj("col")
	mark := func(ids []string, k string) {
		for _, id := range ids {
			sub(col, id)[k] = 1.0
		}
	}
	mark(r.Learned, "d")
	mark(r.Evolved, "e")
	mark(r.Capstone, "c")
	p.recRun(r, gain)
	unlocked = p.ChkUnl()
	p.ColStats()
	return
}

func (p *Profile) recRun(r RunResult, gain int) {
	if r.T < 15 {
		return
	}
	type kv struct {
		k string
		v float64
	}
	var l []kv
	dt := 0.0
	for k, v := range r.Dmg {
		dt += v
		if p.DB.Moves[k] != nil {
			l = append(l, kv{k, v})
		}
	}
	sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
	d := map[string]any{}
	for i, x := range l {
		if i >= 10 {
			break
		}
		d[x.k] = math.Round(x.v)
	}
	ld := []any{}
	for _, m := range r.Moves {
		ld = append(ld, []any{m[0], m[1], m[2]})
	}
	it := []any{}
	for _, m := range r.Items {
		it = append(it, []any{m[0], m[1]})
	}
	mods := []any{}
	for _, m := range r.Mods {
		mods = append(mods, m)
	}
	bv := []any{}
	for _, b := range r.Visited {
		bv = append(bv, b)
	}
	h := map[string]any{"ts": float64(nowMs()), "c": r.CID, "tr": float64(r.Tier), "md": mods, "tm": math.Round(r.T), "ps": r.AbilityName,
		"sg": float64(r.Stage), "bv": bv, "lv": float64(r.Lv), "k": float64(r.Kills), "w": b2n(r.Win), "ek": float64(r.Elites),
		"bk": float64(r.Bosses), "g": float64(gain), "d": d, "ld": ld, "it": it, "dt": math.Round(dt), "go": 1.0}
	hist, _ := p.S.M["hist"].([]any)
	hist = append(hist, h)
	if len(hist) > 150 {
		hist = hist[len(hist)-150:]
	}
	p.S.M["hist"] = hist
	p.addCS(h)
}

func b2n(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (p *Profile) addCS(r map[string]any) {
	id, _ := r["c"].(string)
	if id == "" {
		return
	}
	cs := p.S.obj("cs")
	c, ok := cs[id].(map[string]any)
	if !ok {
		c = map[string]any{"r": 0.0, "w": 0.0, "k": 0.0, "tm": 0.0, "bt": 0.0, "bk": 0.0, "ek": 0.0, "g": 0.0, "lv": 0.0, "dm": 0.0, "md": map[string]any{}}
		cs[id] = c
	}
	add := func(k string, v float64) { c[k] = num(c[k]) + v }
	add("r", 1)
	add("w", num(r["w"]))
	add("k", num(r["k"]))
	add("tm", num(r["tm"]))
	c["bt"] = math.Max(num(c["bt"]), num(r["tm"]))
	add("bk", num(r["bk"]))
	add("ek", num(r["ek"]))
	add("g", num(r["g"]))
	c["lv"] = math.Max(num(c["lv"]), num(r["lv"]))
	md := sub(c, "md")
	sd := 0.0
	if d, ok := r["d"].(map[string]any); ok {
		for k, v := range d {
			sd += num(v)
			md[k] = num(md[k]) + num(v)
		}
	}
	if dt, ok := r["dt"]; ok {
		add("dm", num(dt))
	} else {
		add("dm", sd)
	}
}

func sortStrings(l []string) { sort.Strings(l) }
