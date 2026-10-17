package client

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/meta"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Menu screens: main menu, Pokémon select, run setup, shops, challenges, save transfer, run results.

const (
	scMenu screen = iota + 100
	scChars
	scSetup
	scShop
	scLine
	scChal
	scXfer
	scResult
)

type endInfo struct {
	gain, mx, tickets int
	rank, into, need  int
	rankUp            int
	unlocked          []string
	win               bool
}

func (g *Game) save() {
	g.saveGen++
	if g.prof != nil {
		g.prof.S.SetSel(g.csel)
		g.prof.S.Store()
		g.cloud.PushSave(g.prof.S)
	}
}

// adoptCloud takes a newer save from the cloud (not during a run; the web game waits the same way).
func (g *Game) adoptCloud(s *meta.Save) {
	if g.scr == scPlay || g.scr == scPause {
		g.cloudPending = s
		return
	}
	if s.Num("ts") <= g.prof.S.Num("ts") {
		if s.Num("ts") < g.prof.S.Num("ts") {
			g.cloud.PushSave(g.prof.S)
		}
		return
	}
	g.prof = meta.NewProfile(s, g.db)
	g.saveGen++
	if sel := s.Sel(); sel != "" && g.db.ByID[sel] != nil && g.prof.Unl(sel) {
		g.csel = sel
	}
	g.prof.S.Store()
	g.toast("Progress loaded from your account")
}

func (g *Game) toast(s string) { g.toasts = append(g.toasts, toastMsg{s, 4}) }

type toastMsg struct {
	s string
	t float64
}

// ---- building the current menu ----

func (g *Game) buildMenu() {
	u := &g.ui
	u.reset()
	u.area = [4]float64{0, 0, 0, 0}
	u.scrollMax = 0
	switch g.scr {
	case scMenu:
		g.menuMain()
	case scChars:
		g.menuChars()
	case scSetup:
		g.menuSetup()
	case scShop:
		g.menuShop()
	case scLine:
		g.menuLine()
	case scChal:
		g.menuChal()
	case scXfer:
		g.menuXfer()
	case scResult:
		g.menuResult()
	case scSettings:
		g.menuSettings()
	case scDex:
		g.menuDex()
	case scHist:
		g.menuHist()
	case scDaily:
		g.menuDaily()
	case scBoards:
		g.menuBoards()
	case scOnline:
		g.menuOnline()
	case scColl:
		g.menuColl()
	case scLife:
		g.menuLife()
	case scFriends:
		g.menuFriends()
	}
	if g.scr != scPlay && g.scr != scPause {
		g.invitePopup()
	}
}

func (g *Game) header(title, sub string) {
	sw := float64(g.sw)
	g.ui.label([4]float64{0, 22, sw, 40}, false, textDraw(title, 32, true, gold, text.AlignCenter))
	if sub != "" {
		g.ui.label([4]float64{0, 64, sw, 20}, false, textDraw(sub, 13, false, dim, text.AlignCenter))
	}
	p := g.prof.S
	g.ui.label([4]float64{sw - 260, 24, 240, 20}, false, textDraw(fmt.Sprintf("%s coins   %d tickets", commas(p.Coins()), p.Tickets()), 14, true, gold, text.AlignEnd))
}

func (g *Game) backButton(to screen) {
	if g.lobbyRet && to == scMenu { // opened from a co-op lobby: go back there
		to = g.lobbyTo
	}
	g.ui.button("back", [4]float64{20, 20, 110, 40}, false, false, btnDraw("< Back", "", dim, false), func() { g.scr = to; g.ui.focus = "" })
	if g.ui.edit != "" || g.ui.swallow {
		return
	}
	if keyJust(ebiten.KeyEscape, ebiten.KeyBackspace) || padBtn(ebiten.StandardGamepadButtonRightRight) {
		g.scr = to
		g.ui.focus = ""
	}
}

func commas(n int) string {
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func (g *Game) setupSummary() string {
	rs := g.prof.S.RunSetup()
	bi, _ := rs["bi"].(string)
	loc := "Random location"
	if bi != "" && bi != "random" && g.prof.BiomeUnlocked(bi) {
		loc = g.locName(bi, int(num(rs["li"])))
	}
	return fmt.Sprintf("%s · Tier %d · Difficulty %d · %d modifiers", loc, g.stage(), g.tier(), len(g.mods()))
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

func (g *Game) locName(t string, li int) string {
	if l := g.db.World.Locations[t]; li >= 0 && li < len(l) {
		return l[li]
	}
	return biomeName(t)
}

// ---- Pokémon select ----

func (g *Game) rosterAll() []string {
	var l []string
	for _, p := range g.db.Pokemon {
		l = append(l, p.ID)
	}
	return l
}

func tagText(p *content.Pokemon) string {
	if p.Tag != "" {
		return " · " + p.Tag
	}
	return ""
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ---- run setup ----

func (g *Game) stage() int {
	rs := g.prof.S.RunSetup()
	bi, _ := rs["bi"].(string)
	return max(1, min(int(num(rs["sg"])), g.prof.StageMax(bi)))
}

func (g *Game) tier() int {
	return min(int(num(g.prof.S.RunSetup()["tier"])), g.prof.Asc())
}

func (g *Game) mods() []string {
	var out []string
	if l, ok := g.prof.S.RunSetup()["mods"].([]any); ok {
		for _, x := range l {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func contains(l []string, x string) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}

func b2s(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

// ---- shops ----

// ---- challenges ----

// ---- save transfer ----

func (g *Game) menuXfer() {
	sw := float64(g.sw)
	u := &g.ui
	g.header("SAVE TRANSFER", "Move your progress between the web game and this version")
	g.backButton(scMenu)
	x := sw/2 - 260
	u.label([4]float64{x, 110, 520, 100}, false, func(dst *ebiten.Image, r [4]float64) {
		for i, ln := range []string{
			"Copy save code: puts a code for all your progress on the clipboard.",
			"Paste it into \"Load save code\" in the web game (or another copy of this one).",
			"Load save code: reads a code from the clipboard and REPLACES this save.",
			"The code from the web game's \"Copy save code\" button works here.",
		} {
			drawText(dst, ln, r[0], r[1]+float64(i)*22, 13, false, dim, text.AlignStart, false)
		}
	})
	u.button("copy", [4]float64{x, 220, 520, 54}, false, false, btnDraw("Copy save code", "To the clipboard", c(159, 232, 255, 255), false), func() {
		if clipWrite(g.prof.S.EncodeCode()) {
			g.xferMsg = "Copied! Paste it into the other game."
		} else {
			g.xferMsg = "Couldn't reach the clipboard."
		}
	})
	label := "Load save code"
	sub := "From the clipboard (replaces this save)"
	if g.xferPending != nil {
		label = "Confirm: replace my save"
		sub = fmt.Sprintf("Found %s coins and %d unlocks", commas(g.xferPending.Coins()), len(g.xferPending.M["unl"].(map[string]any)))
	}
	u.button("load", [4]float64{x, 286, 520, 54}, false, false, btnDraw(label, sub, b2c(g.xferPending != nil, c(255, 140, 140, 255), gold), false), func() {
		if g.xferPending != nil {
			g.prof = meta.NewProfile(g.xferPending, g.db)
			g.saveGen++
			g.xferPending = nil
			if s := g.prof.S.Sel(); s != "" && g.db.ByID[s] != nil {
				g.csel = s
			}
			g.prof.S.Store()
			g.xferMsg = "Save loaded!"
			return
		}
		g.xferMsg = "Reading the clipboard..."
		clipRead(func(s string) { g.clipIn <- s })
	})
	if g.xferMsg != "" {
		u.label([4]float64{0, 360, sw, 20}, false, textDraw(g.xferMsg, 15, true, gold, text.AlignCenter))
	}
	select {
	case s := <-g.clipIn:
		sv, err := meta.DecodeCode(s)
		if err != nil {
			g.xferMsg = "That didn't work: " + err.Error()
		} else {
			g.xferPending = sv
			g.xferMsg = "Save found. Press Load again to replace your progress."
		}
	default:
	}
}

// ---- run results ----

func (g *Game) menuResult() {
	sw := float64(g.sw)
	u := &g.ui
	u.area = [4]float64{0, 0, sw, float64(g.sh)}
	e := g.end
	w := g.W
	title := "Knocked out"
	if e.win {
		title = "Victory!"
		if w.Endless {
			title = "Victory! Endless run over"
		}
	}
	y := 90.0
	u.label([4]float64{0, y, sw, 40}, true, textDraw(title, 32, true, webFg, text.AlignCenter))
	y += 50
	es := fmt.Sprintf("Time %s · Level %d · %d defeated · +%s coins", fmtT(w.T), w.P.Lv, w.Kills, commas(e.gain))
	if w.Run.Mult > 1 {
		es += fmt.Sprintf(" (rewards ×%.2f)", w.Run.Mult)
	}
	es += fmt.Sprintf(" (total %s)", commas(g.prof.S.Coins()))
	if e.tickets > 0 {
		es += fmt.Sprintf(" · +%d ticket(s)", e.tickets)
	}
	if n := g.prof.ReadyChallenges(); n > 0 {
		es += fmt.Sprintf(" · %d challenge%s ready to claim", n, plural(n))
	}
	ls := wrap(es, font(14, false), 480)
	u.label([4]float64{sw/2 - 240, y, 480, float64(len(ls)) * 18}, true, func(dst *ebiten.Image, r [4]float64) {
		for i, l := range ls {
			drawText(dst, l, sw/2, r[1]+float64(i)*18, 14, false, alpha(webFg, .9), text.AlignCenter, true)
		}
	})
	y += float64(len(ls))*18 + 12
	// mastery (web .mrend)
	root := g.prof.LineRoot(w.P.ID)
	mh := 84 + 18*float64(len(e.unlocked))
	u.label([4]float64{sw/2 - 243, y, 486, mh}, true, func(dst *ebiten.Image, r [4]float64) {
		roundRect(dst, r[0], r[1], r[2], r[3], 10, alpha(webAcc, .33))
		roundRect(dst, r[0]+1, r[1]+1, r[2]-2, r[3]-2, 9, mix(hexC("#10131a"), webAcc, .08))
		x := r[0] + 13
		parts := []any{webAcc, gStar, webFg, true, fmt.Sprintf("Mastery +%s XP ", commas(e.mx)), false, alpha(webFg, .85), fmt.Sprintf("(%s survived, %d defeated%s)", fmtT(w.T), w.Kills, b2s(e.win, ", +300 win bonus", ""))}
		richLine(dst, x, r[1]+9, 13, true, webFg, parts...)
		line2 := []any{fmt.Sprintf("%s line · Rank %d", g.db.ByID[root].Name, e.rank)}
		if e.rankUp > 0 {
			line2 = append(line2, webAcc, fmt.Sprintf("  ▲ +%d rank%s!", e.rankUp, plural(e.rankUp)))
		}
		richLine(dst, x, r[1]+27, 13, false, webFg, line2...)
		pbar(dst, x, r[1]+48, r[2]-26, 8, float64(e.into)/math.Max(1, float64(e.need)), webAcc)
		drawText(dst, fmt.Sprintf("%s / %s XP to rank %d", commas(e.into), commas(e.need), e.rank+1), x, r[1]+61, 11, false, alpha(webFg, .85), text.AlignStart, false)
		for i, id := range e.unlocked {
			richLine(dst, x, r[1]+80+float64(i)*18, 13, true, webAcc, gLock, g.db.ByID[id].Name+" unlocked!")
		}
	})
	y += mh + 14
	if rc := w.DeathRecap(w.P); rc != nil && !e.win && w.P.HP <= 0 {
		h := recapH(rc)
		u.label([4]float64{sw/2 - 236, y, 472, h}, true, func(dst *ebiten.Image, r [4]float64) { g.drawRecap(dst, rc, r) })
		y += h + 10
	}
	dh := 17 * float64(min(8, len(w.DMG))+1)
	u.label([4]float64{sw/2 - 230, y, 460, dh}, true, func(dst *ebiten.Image, r [4]float64) {
		g.drawDamageBy(dst, r[0], r[1], r[2])
	})
	y += dh + 6
	x := sw/2 - 300
	if g.coop {
		yy := y + 10
		u.label([4]float64{x, yy, 600, 20 + 20*float64(len(w.PL))}, true, func(dst *ebiten.Image, r [4]float64) {
			g.drawStandings(dst, r[0], r[1], r[2])
		})
		y = yy + 30 + 20*float64(len(w.PL))
	}
	if g.online() || (g.coop && g.on.room != nil) {
		if g.on.note != "" {
			u.label([4]float64{0, y - 4, sw, 20}, true, textDraw(g.on.note, 13, true, c(255, 179, 179, 255), text.AlignCenter))
		}
		u.button("again", [4]float64{sw/2 - 250, y + 20, 240, 50}, true, false, btnDraw("Back to the room", "Play again with the same people", gold, false), func() { g.endOnlineRun() })
		u.button("menu", [4]float64{sw/2 + 10, y + 20, 240, 50}, true, false, btnDraw("Leave", "Main menu", c(159, 232, 255, 255), false), func() { g.leaveRoom(); g.scr = scMenu; g.ui.focus = "" })
		u.scrollMax = math.Max(0, y+90-u.area[3])
		return
	}
	u.button("again", [4]float64{sw/2 - 160, y + 14, 150, 44}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
		webBtnDraw(dst, r, hot, false, false)
		drawText(dst, "Play again", r[0]+r[2]/2, r[1]+13, 15, false, webFg, text.AlignCenter, false)
	}, func() {
		if g.coop {
			g.startCoop()
			return
		}
		g.startFromMenu()
	})
	u.button("menu", [4]float64{sw/2 + 10, y + 14, 150, 44}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
		webBtnDraw(dst, r, hot, false, false)
		drawText(dst, "Main menu", r[0]+r[2]/2, r[1]+13, 15, false, webFg, text.AlignCenter, false)
	}, func() { g.scr = scMenu; g.ui.focus = "" })
	u.scrollMax = math.Max(0, y+80-u.area[3])
}

// ---- starting and finishing runs ----

func (g *Game) startFromMenu() {
	pr := g.prof
	id := g.csel
	if !pr.Unl(id) {
		g.toast("That Pokémon is locked")
		return
	}
	rs := pr.S.RunSetup()
	bi, _ := rs["bi"].(string)
	li := -1
	if bi == "" || bi == "random" || !pr.BiomeUnlocked(bi) {
		bi = content.Types[rand.Intn(len(content.Types))]
	} else {
		li = int(num(rs["li"]))
	}
	fam := pr.Fam(id)
	up := map[string]int{}
	for _, k := range meta.Upgrades {
		up[k.Key] = pr.UL(k.Key, fam)
	}
	lock, _ := rs["lock"].(bool)
	cfg := sim.Config{Ability: pr.AbilityIdx(id), LI: li, Up: up, Training: pr.Training(fam), Tier: g.tier(), Mods: g.mods(), Stage: g.stage(), Lock: lock}
	cfg.OnBoss = func(b, et string, l int, fin bool, visited []string) []string {
		m := pr.BossKill(b, et, l, fin, cfg.Stage, id, visited)
		g.save()
		return m
	}
	g.startCfg(id, bi, rand.Int63(), cfg)
	g.daily = nil
	g.finished = false
}

// finishRun applies rewards once when a run ends.
func (g *Game) finishRun() {
	if g.finished || g.prof == nil || g.auto {
		return
	}
	g.finished = true
	w := g.W
	pr := g.prof
	me := 0
	if g.online() {
		me = g.on.room.Me
	}
	w.UseLocal(me)
	root := pr.LineRoot(w.P.ID)
	r0, _, _ := pr.Mastery(root)
	var moves [][3]any
	for _, s := range w.Slots {
		moves = append(moves, [3]any{s.ID, float64(s.Lv), b2f(s.Evo, 1, 0)})
	}
	var items [][2]any
	for _, k := range w.HeldOrder {
		if w.Held[k] > 0 {
			items = append(items, [2]any{k, float64(w.Held[k])})
		}
	}
	keys := func(m map[string]bool) []string {
		var l []string
		for k := range m {
			l = append(l, k)
		}
		return l
	}
	kills := w.Kills
	if g.coop {
		kills = w.PL[me].Kills
	}
	res := meta.RunResult{CID: w.P.ID, T: w.T, Kills: kills, Lv: w.P.Lv, Win: w.Won, Coins: w.Coins, Tickets: w.Tickets,
		Elites: w.Stats.Elites, MiniBosses: w.Stats.Minis, EliteTk: w.Stats.EliteTk, Cursed: w.Stats.Cursed, Curses: w.Stats.Curses,
		Bosses: w.Bosses, Ability: w.PS.A.ID, AbilityName: w.PS.A.Name, Tier: w.Run.Tier, Mods: w.Run.Mods, Stage: w.Run.Stage, Mult: w.Run.Mult,
		Visited: w.Visited, Dmg: w.DMG, Moves: moves, Items: items, Learned: keys(w.Learned), Evolved: keys(w.Evolved), Capstone: keys(w.Capstone)}
	if g.daily != nil {
		pr.DailyResult(time.Now(), *g.daily, w.P.ID, w.Kills, w.T, w.Won, moves, items, g.lbName())
		g.cloud.PushLB(pr.LB().M)
		g.daily = nil
	}
	gain, mx, unl := pr.Finish(res)
	rk, into, need := pr.Mastery(root)
	g.end = endInfo{gain: gain, mx: mx, tickets: w.Tickets, rank: rk, into: into, need: need, rankUp: rk - r0, unlocked: unl, win: w.Won}
	g.save()
}

// OpenScreen jumps to a menu screen by name: menu chars setup shop line chal xfer.
func (g *Game) OpenScreen(name string) {
	switch name {
	case "chars":
		g.scr = scChars
	case "setup":
		g.scr = scSetup
	case "shop":
		g.scr = scShop
	case "line":
		g.lineFam = g.prof.Fam(g.csel)
		g.scr = scLine
	case "chal":
		g.scr = scChal
	case "xfer":
		g.scr = scXfer
	case "settings":
		g.scr = scSettings
	case "dex":
		g.scr = scDex
	case "hist":
		g.scr = scHist
	case "daily":
		g.scr = scDaily
	case "boards":
		g.scr = scBoards
		g.lbFetch()
	case "lobby":
		g.scr = scLobby
	case "friends":
		g.scr = scFriends
	case "coll":
		g.scr = scColl
	case "life":
		g.scr = scLife
	case "online":
		g.scr = scOnline
		g.on.mode = "menu"
	default:
		g.scr = scMenu
	}
}

// LoadCode replaces the profile with a save code (testing / command line).
func (g *Game) LoadCode(code string) {
	s, err := meta.DecodeCode(code)
	if err != nil {
		g.toast(err.Error())
		return
	}
	g.prof = meta.NewProfile(s, g.db)
	g.saveGen++
	if sel := s.Sel(); sel != "" && g.db.ByID[sel] != nil {
		g.csel = sel
	}
}

// ---- "what knocked you out" (web deathHTML) ----

func recapTip(rc *sim.Recap) []string {
	if rc.Tip == "" {
		return nil
	}
	return wrap(rc.Tip, font(12, false), 424)
}

func recapH(rc *sim.Recap) float64 {
	h := 10 + 22 + 52 + 20 + 18*float64(len(rc.Last10)) + 20 + 18*float64(len(rc.Run)) + 10
	if t := recapTip(rc); len(t) > 0 {
		h += 16*float64(len(t)) + 18
	}
	return h
}

func srcCol(s *sim.DmgSrc) color.NRGBA {
	if s == nil || s.Type == "" {
		return hexC("#99aaaa")
	}
	return tc(s.Type)
}

func (g *Game) drawRecap(dst *ebiten.Image, rc *sim.Recap, r [4]float64) {
	webBox(dst, r[0], r[1], r[2], r[3], 12, alpha(white, .13), c(0, 0, 0, 102))
	x, y := r[0]+12, r[1]+10
	drawText(dst, "What knocked you out", x, y, 14, true, webFg, text.AlignStart, false)
	y += 22
	kc := srcCol(rc.KB.S)
	disc(dst, x+22, y+22, 22, kc)
	disc(dst, x+22, y+22, 20, c(0, 0, 0, 200))
	if rc.KB.S != nil && rc.KB.S.Type != "" {
		g.drawTypeIcon(dst, rc.KB.S.Type, x+22, y+22, 26, false)
	} else {
		drawText(dst, "?", x+22, y+11, 20, true, webFg, text.AlignCenter, false)
	}
	name := "Unknown"
	if rc.KB.S != nil {
		name = rc.KB.S.Name
	}
	drawText(dst, fitText(name, 14, true, r[2]-90), x+54, y+5, 14, true, kc, text.AlignStart, false)
	fin := fmt.Sprintf("Final hit: %d damage at %s", int(math.Round(rc.KB.D)), fmtT(rc.KB.T))
	drawText(dst, fin, x+54, y+25, 11, false, alpha(webFg, .85), text.AlignStart, false)
	y += 52
	rows := func(title string, tot float64, l []sim.DmgAgg) {
		drawText(dst, title, x, y, 12, true, alpha(webFg, .75), text.AlignStart, false)
		y += 20
		mx := 1.0
		if len(l) > 0 {
			mx = math.Max(1, l[0].D)
		}
		for _, o := range l {
			col := srcCol(o.S)
			if o.S.Type != "" {
				g.drawTypeIcon(dst, o.S.Type, x+8, y+7, 15, false)
			} else {
				disc(dst, x+8, y+7, 5, col)
			}
			drawText(dst, fitText(o.S.Name, 12, false, 148), x+22, y, 12, false, webFg, text.AlignStart, false)
			bw := math.Max(3, o.D/mx*120)
			roundRect(dst, x+176, y+3, bw, 8, 4, col)
			pc := int(math.Round(o.D / math.Max(1, tot) * 100))
			v := fmt.Sprintf("%d (%d%%, %d hit%s)", int(math.Round(o.D)), pc, o.Hits, plural(o.Hits))
			drawText(dst, v, x+176+bw+6, y, 11, false, alpha(webFg, .85), text.AlignStart, false)
			y += 18
		}
	}
	rows(fmt.Sprintf("Last 10 seconds: %d damage taken", int(math.Round(rc.Last10Total))), rc.Last10Total, rc.Last10)
	rows(fmt.Sprintf("Whole run: %d damage taken", int(math.Round(rc.RunTotal))), rc.RunTotal, rc.Run)
	if t := recapTip(rc); len(t) > 0 {
		y += 6
		roundRect(dst, x-2, y, r[2]-20, 16*float64(len(t))+10, 8, alpha(webAcc, .1))
		for i, l := range t {
			drawText(dst, l, x+6, y+5+16*float64(i), 12, false, hexC("#ffe9a8"), text.AlignStart, false)
		}
	}
}
