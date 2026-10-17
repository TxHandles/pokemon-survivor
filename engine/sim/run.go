package sim

import (
	"math"
)

// ---- run structure: route map, endless mode, map chests, shrines, coins (web routeOpen / spawnMap / shrine) ----

const (
	RunLength = 900 // 15:00
	RunBosses = 3
)

// RouteOpt is one destination on the route map.
type RouteOpt struct {
	Type string
	LI   int // location index (0-2) into world.json locations
}

// ChestOpenT: seconds standing next to a chest or shrine to open it.
const ChestOpenT = 2.0

// MapChest is a chest, shrine or cursed chest you open by standing next to it for ChestOpenT seconds.
type MapChest struct {
	X, Y, P, T     float64
	Shrine, Cursed bool
}

// Chest is a reward chest you walk over (boss and cursed-guardian chests).
type Chest struct{ X, Y float64 }

// Boon is a shrine choice.
type Boon struct {
	Name, Desc, Curse string
	apply             func(w *World)
}

var safeBoons = []Boon{
	{"Shrine of Might", "+30% damage for 90 seconds.", "", func(w *World) { w.P.Pm = 90 }},
	{"Shrine of Vigor", "+15% max HP and restore HP.", "", func(w *World) {
		w.P.MHP = math.Round(w.P.MHP * 1.15)
		if !w.Run.NoHeal {
			w.P.HP = w.P.MHP
		}
	}},
	{"Shrine of Plenty", "A free level up and bonus coins.", "", func(w *World) { w.freeLevel(); w.Coins += 40 }},
}

var pacts = []Boon{
	{"Pact of Blood", "+50% damage for 90 seconds.", "You take double damage.", func(w *World) { w.P.Pbl = 90 }},
	{"Pact of Haste", "+25% move speed and -20% cooldowns for 90 seconds.", "Enemies move 30% faster.", func(w *World) { w.P.Phs = 90 }},
	{"Pact of Greed", "Enemies drop double coins for 90 seconds.", "New enemies have +50% HP.", func(w *World) { w.P.Pgr = 90 }},
}

// ef: endless-mode scaling (1 before the final boss falls).
func (w *World) ef() float64 {
	if !w.Endless {
		return 1
	}
	return ppow(1.2, (w.T-w.ET)/60)
}

func (w *World) spS() float64 { return math.Min(1.8, ppow(w.ef(), .15)) }

// LocName is a biome location's display name.
func (w *World) LocName(t string, li int) string {
	if l := w.DB.World.Locations[t]; li >= 0 && li < len(l) {
		return l[li]
	}
	return t
}

func (w *World) openRoute() {
	var ts []string
	for _, t := range typesList {
		if t != w.Biome {
			ts = append(ts, t)
		}
	}
	w.rng.Shuffle(len(ts), func(i, j int) { ts[i], ts[j] = ts[j], ts[i] })
	n := w.RouteChoices
	if n < 2 {
		n = 2
	}
	w.Route = nil
	for _, t := range ts[:n] {
		w.Route = append(w.Route, RouteOpt{t, w.rng.Intn(3)})
	}
	w.State = StateRoute
	w.RouteGo, w.RouteT = -1, 0
	w.RouteVotes = make([]int, len(w.PL))
	for i, c := range w.PL {
		w.RouteVotes[i] = -1
		if c.Kind == "bot" { // bots vote at once
			w.RouteVotes[i] = w.rng.Intn(len(w.Route))
		}
	}
	w.tallyRoute()
}

// RouteTravelT: seconds the map shows the party travelling before the next biome loads.
const RouteTravelT = 2.2

// VoteRoute records player p's vote. When everyone still in the run has voted, the most-voted
// destination wins (a tie goes to the host's pick) and the party travels there.
func (w *World) VoteRoute(p, i int) {
	if w.State != StateRoute || w.RouteGo >= 0 || i < 0 || i >= len(w.Route) || p < 0 || p >= len(w.RouteVotes) {
		return
	}
	w.RouteVotes[p] = i
	w.tallyRoute()
}

func (w *World) tallyRoute() {
	cnt := make([]int, len(w.Route))
	host := -1
	for p, c := range w.PL {
		if c.Gone {
			continue
		}
		v := w.RouteVotes[p]
		if v < 0 {
			return // still waiting on someone
		}
		cnt[v]++
		if host < 0 {
			host = v
		}
	}
	best := host
	for i, n := range cnt {
		if best < 0 || n > cnt[best] {
			best = i
		}
	}
	if best < 0 {
		return
	}
	w.RouteGo, w.RouteT = best, RouteTravelT
}

// stepRouteTravel runs the travel animation, then enters the chosen biome.
func (w *World) stepRouteTravel(dt float64) {
	if w.State != StateRoute || w.RouteGo < 0 {
		return
	}
	w.RouteT -= dt
	if w.RouteT <= 0 {
		w.ChooseRouteNow(w.RouteGo)
	}
}

// ChooseRoute votes option i for every player (solo and same-screen co-op).
func (w *World) ChooseRoute(i int) {
	if w.State != StateRoute || w.RouteGo >= 0 || i < 0 || i >= len(w.Route) {
		return
	}
	for p := range w.RouteVotes {
		w.RouteVotes[p] = i
	}
	w.tallyRoute()
}

// ChooseRouteNow travels to route option i at once.
func (w *World) ChooseRouteNow(i int) {
	if w.State != StateRoute || i < 0 || i >= len(w.Route) {
		return
	}
	o := w.Route[i]
	w.Route, w.RouteGo, w.RouteVotes = nil, -1, nil
	w.State = StatePlay
	w.enterBiome(o.Type, o.LI, false)
}

func (w *World) enterBiome(t string, li int, first bool) {
	w.Biome, w.LI = t, li
	w.pools = map[ckey][]Pool{}
	w.Gust = nil
	for _, c := range w.PL {
		c.P.WX, c.P.WY, c.P.gw = 0, 0, 0
	}
	w.bioT = 6
	w.BiomeN++
	w.placeLandmarks()
	if first {
		w.spawnHold = 0
	} else {
		w.spawnHold = 2.5
		w.addFX(&FX{Kind: "flash", Life: .35})
	}
	msg := "Entering: " + w.LocName(t, li) + " (" + t + ")"
	if hz := BiomeHazard[t]; hz != "" {
		note := ""
		switch k := w.hzK(); {
		case k == 0 || w.abHzImm(hz):
			note = " (Your type is immune!)"
		case k < 1:
			note = " (Your type resists it.)"
		case k > 1:
			note = " (Your type is weak to it!)"
		}
		msg += " — " + HazardText[hz] + note
	} else {
		msg += " — a calm biome with no hazards"
	}
	w.message(msg)
}

// afterBoss: boss chest, then the field clears and the route map opens (or victory → endless).
func (w *World) afterBoss(e *Enemy) {
	w.Coins += 25
	w.freeLevel() // boss chest: free level up
	w.bossClear(e)
	// boss loot on the ground: a magnet (pulls in every gem) and a berry
	w.Items = append(w.Items, &Pickup{X: e.X - 22, Y: e.Y, K: "magnet"}, &Pickup{X: e.X + 22, Y: e.Y, K: "berry"})
	w.Visited = appendU(w.Visited, w.Biome)
	w.Tickets++ // boss chest ticket
	if w.Cfg.OnBoss != nil {
		w.Notes = append(w.Notes, w.Cfg.OnBoss(w.Biome, e.Type, w.LI, w.Bosses == w.Run.Bosses && !w.Endless, w.Visited)...)
	}
	if w.Bosses >= w.Run.Bosses && !w.Endless {
		w.Endless, w.Won, w.ET = true, true, w.T
		w.sfx("victory")
		w.message("VICTORY! Endless mode begins...")
	} else {
		w.message("Boss defeated! Choose your next destination...")
	}
	if w.Run.Lock {
		w.spawnHold = 3
		return
	}
	w.spawnHold = 1e9
	w.routeT = 1.3
}

func appendU(l []string, x string) []string {
	for _, y := range l {
		if y == x {
			return l
		}
	}
	return append(l, x)
}

func (w *World) spawnMap(kind string) {
	a, d := w.rnd(0, 6.283), w.rnd(180, 320)
	x, y := w.P.X+math.Cos(a)*d, w.P.Y+math.Sin(a)*d
	w.push(&x, &y, 16)
	switch kind {
	case "c":
		w.Items = append(w.Items, &Pickup{X: x, Y: y, K: "candy"})
	default:
		w.MC = append(w.MC, &MapChest{X: x, Y: y, Shrine: kind == "s", Cursed: kind == "x"})
		if kind == "s" {
			w.message("An ancient shrine appeared nearby!")
		}
		if kind == "x" {
			w.message("A cursed chest appeared nearby...")
		}
	}
}

func (w *World) stepRun(dt float64) {
	w.useP(w.rndAlive())
	_ = w.P
	if w.routeT > 0 {
		w.routeT -= dt
		if w.routeT <= 0 {
			w.routeT = 0
			w.openRoute()
			return
		}
	}
	w.chestT -= dt
	if w.chestT <= 0 {
		w.chestT = 30
		if w.rng.Float64() < .12 && len(w.MC) < 3 {
			w.spawnMap("m")
		}
		if w.T >= 240 && w.rng.Float64() < .04+.01*float64(w.Run.Tier) {
			cur := false
			for _, m := range w.MC {
				cur = cur || m.Cursed
			}
			if !cur {
				w.spawnMap("x")
			}
		}
	}
	w.shT -= dt
	if w.shT <= 0 {
		w.shT = 60
		sh := false
		for _, m := range w.MC {
			sh = sh || m.Shrine
		}
		if w.T >= 200 && w.rng.Float64() < .45 && !sh {
			w.spawnMap("s")
		}
	}
	keep := w.MC[:0]
	for _, c := range w.MC {
		c.T += dt
		if j := w.nearIn(c.X, c.Y, 46); j >= 0 {
			w.useP(j)
			c.P += dt
		} else {
			c.P = math.Max(0, c.P-dt*.5)
		}
		if c.P >= ChestOpenT && w.State == StatePlay {
			w.openChest(c)
			continue
		}
		if w.nearIn(c.X, c.Y, 1800) >= 0 {
			keep = append(keep, c)
		}
	}
	w.MC = keep
	ck := w.Chests[:0]
	for _, c := range w.Chests {
		if j := w.nearIn(c.X, c.Y, 28); j >= 0 {
			w.useP(j)
			P := w.P
			cn := 25.0
			w.Coins += cn
			w.Tickets++
			w.freeLevel()
			w.sfx("chest")
			w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 40, Life: 2.5, Text: "CHEST! +25 coins, free level up", Col: "gold"})
			continue
		}
		ck = append(ck, c)
	}
	w.Chests = ck
	w.stepTrader(dt)
}

func (w *World) openChest(c *MapChest) {
	switch {
	case c.Shrine:
		boons := []Boon{safeBoons[w.rng.Intn(3)]}
		p := []int{0, 1, 2}
		w.rng.Shuffle(3, func(i, j int) { p[i], p[j] = p[j], p[i] })
		boons = append(boons, pacts[p[0]], pacts[p[1]])
		for _, pc := range w.PL {
			if !pc.Gone {
				pc.Queue = append(pc.Queue, &Prompt{Kind: "shrine", Boons: boons})
			}
		}
		w.prompt()
	case c.Cursed:
		w.Stats.Cursed++
		w.spawnEliteAt(true, c.X, c.Y, true)
		for i := 0; i < 18; i++ {
			a := float64(i) / 18 * 6.283
			if len(w.E) < 500 {
				w.minion(w.P.X+math.Cos(a)*260, w.P.Y+math.Sin(a)*260, typesList[w.rng.Intn(18)])
			}
		}
		w.message("CURSED! Defeat the guardian for a bounty!")
		w.Shake = .4
	default:
		for i := 0; i < 5; i++ {
			w.Gems = append(w.Gems, &Gem{X: c.X + w.rnd(-30, 30), Y: c.Y + w.rnd(-30, 30), C: 5})
		}
		for i := 0; i < 6; i++ {
			w.Gems = append(w.Gems, &Gem{X: c.X + w.rnd(-30, 30), Y: c.Y + w.rnd(-30, 30), V: w.XPS.NX * .12})
		}
		w.sfx("chest")
		w.addFX(&FX{Kind: "txt", X: c.X, Y: c.Y - 30, Life: 2, Text: "Chest opened!", Col: "gold"})
	}
}

// ChooseBoon takes shrine boon i (or leaves with -1 / out of range) for the first player choosing.
func (w *World) ChooseBoon(i int) { w.ChooseBoonFor(w.Chooser, i) }

// ChooseBoonFor: player p takes shrine boon i (or leaves).
func (w *World) ChooseBoonFor(p, i int) {
	if w.PendingKind(p) != "shrine" {
		return
	}
	w.useP(p)
	if i >= 0 && i < len(w.Boons) {
		w.applyBoon(w.Boons[i])
	}
	c := w.PL[p]
	c.Queue = c.Queue[1:]
	w.Boons = nil
	w.prompt()
}

// coinDrop: enemies sometimes drop coins (web hit() death drops).
func (w *World) coinDrop(e *Enemy) {
	cm := b2f(w.P.Pgr > 0, 2, 1) * (1 + .1*w.up("cg"))
	if e.Boss {
		return
	}
	if w.rng.Float64() < b2f(e.MHP > 40, .4, .12) {
		w.Gems = append(w.Gems, &Gem{X: e.X + 5, Y: e.Y, C: cm})
	}
	if e.Elite {
		n, v := 5, 2.0
		if e.Mini {
			n, v = 10, 3
		}
		for i := 0; i < n; i++ {
			w.Gems = append(w.Gems, &Gem{X: e.X + w.rnd(-24, 24), Y: e.Y + w.rnd(-24, 24), C: v * cm})
		}
		if e.Cursed {
			w.Chests = append(w.Chests, Chest{e.X, e.Y}, Chest{e.X + 30, e.Y})
		} else if e.Mini && w.rng.Float64() < .25 {
			w.Chests = append(w.Chests, Chest{e.X, e.Y})
		}
	}
}

var typesList = []string{"normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"}

// Waiting reports whether the run is paused on a choice (level-up, route map, shrine).
func (w *World) Waiting() bool {
	return w.State == StateLevelUp || w.State == StateRoute || w.State == StateShrine
}

// BotDecide makes the pending choice the way the web balance bot does: best-scored card, a random
// route, the safe shrine boon.
func (w *World) BotDecide() {
	switch w.State {
	case StateLevelUp:
		w.useP(w.Chooser)
		w.Choose(w.BotPick())
	case StateRoute:
		if w.RouteGo >= 0 { // headless: skip the travel animation
			w.ChooseRouteNow(w.RouteGo)
			return
		}
		w.ChooseRoute(w.rng.Intn(len(w.Route)))
	case StateShrine:
		if w.PendingKind(w.Chooser) == "shop" {
			w.botShop(w.Chooser)
			return
		}
		w.ChooseBoon(0)
	}
}

// OpenRouteNow opens the route map immediately (debug / screenshots).
func (w *World) OpenRouteNow() { w.openRoute() }

// OpenShrineNow opens a shrine immediately (debug / screenshots).
func (w *World) OpenShrineNow() { w.openChest(&MapChest{Shrine: true}) }
