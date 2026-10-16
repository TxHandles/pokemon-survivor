package client

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"sort"
	"strings"

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
	if g.prof != nil {
		g.prof.S.SetSel(g.csel)
		g.prof.S.Store()
	}
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
	g.ui.button("back", [4]float64{20, 20, 110, 40}, false, false, btnDraw("< Back", "", dim, false), func() { g.scr = to; g.ui.focus = "" })
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

func (g *Game) menuMain() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	u.label([4]float64{0, 70, sw, 50}, false, textDraw("POKÉMON SURVIVOR", 50, true, gold, text.AlignCenter))
	u.label([4]float64{0, 132, sw, 20}, false, textDraw("Go build · preview", 15, false, dim, text.AlignCenter))
	p := g.prof.S
	u.label([4]float64{0, 160, sw, 20}, false, textDraw(fmt.Sprintf("%s coins  ·  %d tickets  ·  %d runs  ·  %d wins", commas(p.Coins()), p.Tickets(), int(p.Stat("r")), int(p.Stat("w"))), 14, true, c(230, 230, 240, 255), text.AlignCenter))
	bw, bh := 320.0, 54.0
	x := sw/2 - bw/2
	y := 210.0
	ready := g.prof.ReadyChallenges()
	items := []struct {
		k, label, sub string
		to            screen
	}{
		{"play", "Play", "Choose a Pokémon and start a run", scChars},
		{"setup", "Run setup", g.setupSummary(), scSetup},
		{"shop", "Shop", "Global upgrades: rerolls, banishes, slots, choices", scShop},
		{"chal", "Challenges", fmt.Sprintf("%d ready to claim", ready), scChal},
		{"xfer", "Save transfer", "Move progress to and from the web version", scXfer},
		{"settings", "Settings", "Volume, music, screen shake, touch controls", scSettings},
	}
	for _, it := range items {
		it := it
		col := c(159, 232, 255, 255)
		if it.k == "play" {
			col = gold
		}
		if it.k == "chal" && ready > 0 {
			col = c(120, 255, 140, 255)
		}
		u.button(it.k, [4]float64{x, y, bw, bh}, false, false, btnDraw(it.label, it.sub, col, false), func() {
			g.settingsFrom = scMenu
			g.scr = it.to
			g.ui.focus = ""
			g.ui.scroll = 0
		})
		y += bh + 12
	}
	u.label([4]float64{0, sh - 36, sw, 20}, false, textDraw("Arrows / stick to move  ·  Enter / A to choose  ·  Esc / B to go back  ·  Saved to "+meta.SavePath(), 12, false, alpha(dim, .8), text.AlignCenter))
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

func (g *Game) menuChars() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	g.header("CHOOSE YOUR POKÉMON", "Locked Pokémon show how to get them · evolutions unlock with Mastery")
	g.backButton(scMenu)
	if g.csel == "" || g.db.ByID[g.csel] == nil {
		g.csel = "fire"
	}
	// grid (left, scrolling)
	gw := math.Min(sw-460, 760)
	u.area = [4]float64{20, 96, gw, sh - 116}
	cols := int(gw / 84)
	cell := gw / float64(cols)
	ids := g.rosterAll()
	for i, id := range ids {
		id := id
		x := 20 + float64(i%cols)*cell
		y := float64(i/cols) * cell
		lk := !g.prof.Unl(id)
		u.button("c:"+id, [4]float64{x + 3, y + 3, cell - 6, cell - 6}, true, false, g.charCell(id, lk), func() {
			g.csel = id
			g.save()
		})
	}
	rows := (len(ids) + cols - 1) / cols
	u.scrollMax = math.Max(0, float64(rows)*cell-u.area[3])
	g.charPanel(sw-430, 96, 410, sh-116)
}

func (g *Game) charCell(id string, lk bool) func(dst *ebiten.Image, r [4]float64, hot bool) {
	return func(dst *ebiten.Image, r [4]float64, hot bool) {
		p := g.db.ByID[id]
		col := tc(p.Types[0])
		bg := c(26, 32, 48, 255)
		if id == g.csel {
			bg = mix(bg, col, .35)
		} else if hot {
			bg = mix(bg, col, .18)
		}
		roundRect(dst, r[0], r[1], r[2], r[3], 9, bg)
		if hot || id == g.csel {
			roundRect(dst, r[0]+4, r[1]+r[3]-6, r[2]-8, 3, 1.5, col)
		}
		if s := g.assets.Sprite(id); s != nil {
			fr := s.Front.IdleFrame(0)
			if id == g.csel {
				fr = s.Front.IdleFrame(g.tick * 1000 / 60)
			}
			k := math.Min((r[2]-18)/float64(s.Front.V.CW), (r[3]-26)/float64(s.Front.V.CH))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(r[0]+r[2]/2-float64(s.Front.V.CW)*k/2, r[1]+6)
			op.Filter = ebiten.FilterLinear
			if lk {
				op.ColorScale.Scale(.12, .12, .16, 1) // silhouette
			}
			dst.DrawImage(s.Front.Frame(fr), op)
		}
		name := p.Name
		if lk {
			name = "???"
		}
		drawText(dst, name, r[0]+r[2]/2, r[1]+r[3]-20, 10, true, b2c(lk, c(120, 126, 140, 255), white), text.AlignCenter, false)
		if lk && g.prof.TicketCost(id) > 0 && g.prof.S.Tickets() >= g.prof.TicketCost(id) {
			disc(dst, r[0]+r[2]-10, r[1]+10, 5, gold)
		}
	}
}

func (g *Game) charPanel(x, y, w, h float64) {
	u := &g.ui
	id := g.csel
	p := g.db.ByID[id]
	pr := g.prof
	fam := pr.Fam(id)
	lk := !pr.Unl(id)
	col := tc(p.Types[0])
	u.label([4]float64{x, y, w, h}, false, panelDraw(c(20, 25, 38, 240)))
	name := p.Name
	if lk {
		name = "??? (locked)"
	}
	types := ""
	for i, t := range p.Types {
		if i > 0 {
			types += " / "
		}
		types += cap1(t)
	}
	u.label([4]float64{x + 16, y + 14, w - 32, 24}, false, textDraw(name, 22, true, col, text.AlignStart))
	u.label([4]float64{x + 16, y + 44, w - 32, 18}, false, textDraw(types+" type"+tagText(p), 12, false, dim, text.AlignStart))
	yy := y + 70
	if lk {
		u.label([4]float64{x + 16, yy, w - 32, 18}, false, textDraw(pr.UnlockText(id), 13, true, c(255, 200, 120, 255), text.AlignStart))
		yy += 24
		if hv, nd := pr.Progress(id); pr.TicketCost(id) == 0 && nd > 0 {
			u.label([4]float64{x + 16, yy, w - 32, 12}, false, func(dst *ebiten.Image, r [4]float64) {
				bar(dst, r[0], r[1], r[2], 8, float64(hv)/float64(nd), gold, c(40, 46, 60, 255))
				drawText(dst, fmt.Sprintf("%d / %d", hv, nd), r[0]+r[2], r[1]+12, 11, false, dim, text.AlignEnd, false)
			})
			yy += 30
		}
		if c := pr.TicketCost(id); c > 0 {
			can := pr.S.Tickets() >= c
			u.button("buy", [4]float64{x + 16, yy, w - 32, 44}, false, !can, btnDraw(fmt.Sprintf("Unlock for %d ticket%s", c, plural(c)), fmt.Sprintf("You have %d", pr.S.Tickets()), gold, !can), func() {
				if pr.BuyUnlock(id) {
					g.toast(p.Name + " unlocked!")
					g.save()
				}
			})
		}
		return
	}
	// mastery
	root := pr.LineRoot(id)
	rk, into, need := pr.Mastery(root)
	u.label([4]float64{x + 16, yy, w - 32, 30}, false, func(dst *ebiten.Image, r [4]float64) {
		drawText(dst, fmt.Sprintf("Mastery Rank %d  (%s line)", rk, g.db.ByID[root].Name), r[0], r[1], 13, true, gold, text.AlignStart, false)
		bar(dst, r[0], r[1]+20, r[2], 7, float64(into)/float64(need), gold, c(40, 46, 60, 255))
	})
	yy += 38
	// stats with upgrades
	st := g.db.Stats[id]
	ul := func(k string) float64 { return float64(pr.UL(k, fam)) }
	vals := []struct {
		l      string
		v, max float64
		col    color.NRGBA
	}{
		{"HP", math.Round((80 + 14*ul("hp")) * st.HP), 150, c(91, 227, 125, 255)},
		{"ATK", math.Round(100 * st.Atk * (1 + .08*ul("dmg")) * (1 + .02*float64(pr.Training(fam)))), 150, c(255, 122, 77, 255)},
		{"DEF", math.Round(100 * st.Def / math.Pow(.96, ul("arm"))), 150, c(111, 181, 255, 255)},
		{"SPD", math.Round(125 * st.Spd * (1 + .03*ul("spd"))), 170, c(255, 208, 74, 255)},
	}
	for _, v := range vals {
		v := v
		u.label([4]float64{x + 16, yy, w - 32, 16}, false, func(dst *ebiten.Image, r [4]float64) {
			drawText(dst, v.l, r[0], r[1], 12, true, dim, text.AlignStart, false)
			bar(dst, r[0]+40, r[1]+3, r[2]-90, 8, v.v/v.max, v.col, c(40, 46, 60, 255))
			drawText(dst, fmt.Sprint(int(v.v)), r[0]+r[2], r[1], 12, true, white, text.AlignEnd, false)
		})
		yy += 19
	}
	yy += 6
	u.label([4]float64{x + 16, yy, w - 32, 16}, false, textDraw("Starts with "+g.db.Moves[g.db.StartMove(id)].Name, 12, false, dim, text.AlignStart))
	yy += 22
	// abilities
	ab := g.db.AbilityPair(id)
	cur := pr.AbilityIdx(id)
	for i, a := range ab {
		i, a := i, a
		owned := i == 0 || pr.HiddenOwned(id)
		kind := "Ability"
		if a.Tier == 2 {
			kind = "Hidden Ability"
		} else if a.Tier == 3 {
			kind = "Legendary Ability"
		}
		lines := wrap(a.Desc, font(12, false), w-56)
		bh := 30 + float64(len(lines))*16
		extra := ""
		if !owned {
			kl := pr.FamKills(id)
			if kl < meta.HiddenKills {
				extra = fmt.Sprintf("Unlock: %s / %s defeats with this line, then 1 ticket", commas(kl), commas(meta.HiddenKills))
			} else {
				extra = "Ready: unlock for 1 ticket"
			}
			bh += 18
		}
		u.button(fmt.Sprintf("ab%d", i), [4]float64{x + 16, yy, w - 32, bh}, false, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
			bg := c(28, 34, 50, 240)
			if i == cur {
				bg = mix(bg, col, .22)
			}
			if hot {
				roundRect(dst, r[0]-2, r[1]-2, r[2]+4, r[3]+4, 10, alpha(col, .8))
			}
			roundRect(dst, r[0], r[1], r[2], r[3], 8, bg)
			mark := ""
			if i == cur {
				mark = "> "
			}
			drawText(dst, mark+kind+": "+a.Name, r[0]+12, r[1]+8, 13, true, b2c(owned, white, c(150, 156, 170, 255)), text.AlignStart, false)
			for j, ln := range lines {
				drawText(dst, ln, r[0]+12, r[1]+28+float64(j)*16, 12, false, dim, text.AlignStart, false)
			}
			if extra != "" {
				drawText(dst, extra, r[0]+12, r[1]+r[3]-20, 11, true, gold, text.AlignStart, false)
			}
		}, func() {
			if owned {
				pr.SetAbilityIdx(id, i)
				g.save()
			} else if pr.BuyHidden(id) {
				g.toast("Hidden Ability unlocked: " + a.Name)
				g.save()
			}
		})
		yy += bh + 8
	}
	// buttons
	by := y + h - 112
	nu := 0
	for _, k := range meta.Upgrades {
		if !k.Global {
			nu += pr.UL(k.Key, fam)
		}
	}
	u.button("line", [4]float64{x + 16, by, w - 32, 44}, false, false, btnDraw("Line upgrades", fmt.Sprintf("%d bought · Training %d/%d · shared by the whole line", nu, pr.Training(fam), meta.TrainingMax), c(159, 232, 255, 255), false), func() {
		g.lineFam = fam
		g.scr = scLine
		g.ui.focus = ""
	})
	u.button("start", [4]float64{x + 16, by + 52, w - 32, 48}, false, false, btnDraw("Start run", g.setupSummary(), gold, false), func() { g.startFromMenu() })
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

func (g *Game) menuSetup() {
	sw := float64(g.sw)
	u := &g.ui
	pr := g.prof
	rs := pr.S.RunSetup()
	g.header("RUN SETUP", "Where you start, how long the run is, and how hard")
	g.backButton(scMenu)
	bi, _ := rs["bi"].(string)
	if bi == "" {
		bi = "random"
	}
	if bi != "random" && !pr.BiomeUnlocked(bi) {
		bi = "random"
	}
	// biome grid
	x0, y := 40.0, 100.0
	u.label([4]float64{x0, y, 300, 18}, false, textDraw("Starting biome (beat a type's boss anywhere to unlock its locations)", 13, true, white, text.AlignStart))
	y += 24
	bw := (sw - 80 - 18*6) / 19
	bh := 52.0
	types := append([]string{"random"}, content.Types...)
	for i, t := range types {
		t := t
		x := x0 + float64(i)*(bw+6)
		unl := t == "random" || pr.BiomeUnlocked(t)
		on := t == bi
		u.button("b:"+t, [4]float64{x, y, bw, bh}, false, !unl, func(dst *ebiten.Image, r [4]float64, hot bool) {
			col := c(200, 200, 210, 255)
			if t != "random" {
				col = tc(t)
			}
			bg := c(28, 34, 50, 240)
			if on {
				bg = mix(bg, col, .4)
			}
			if hot {
				roundRect(dst, r[0]-2, r[1]-2, r[2]+4, r[3]+4, 9, alpha(col, .9))
			}
			roundRect(dst, r[0], r[1], r[2], r[3], 7, bg)
			label := "?"
			if t != "random" {
				label = strings.ToUpper(t[:3])
				disc(dst, r[0]+r[2]/2, r[1]+16, 9, b2c(unl, col, c(70, 74, 86, 255)))
			} else {
				drawText(dst, "?", r[0]+r[2]/2, r[1]+6, 18, true, white, text.AlignCenter, false)
			}
			drawText(dst, label, r[0]+r[2]/2, r[1]+30, 10, true, b2c(unl, white, c(110, 116, 130, 255)), text.AlignCenter, false)
			if s := pr.BiomeStars(t); s > 0 {
				drawText(dst, strings.Repeat("*", min(3, s)), r[0]+r[2]/2, r[1]+40, 10, true, gold, text.AlignCenter, false)
			}
		}, func() {
			rs["bi"] = t
			if t != "random" {
				rs["li"] = 0.0
			}
			g.save()
		})
	}
	y += bh + 14
	if bi != "random" {
		for i, nm := range g.db.World.Locations[bi] {
			i := i
			on := int(num(rs["li"])) == i
			u.button(fmt.Sprintf("l%d", i), [4]float64{x0 + float64(i)*260, y, 250, 46}, false, false, btnDraw(nm, fmt.Sprintf("%s · tier %d/3 cleared", biomeName(bi), pr.BiomeStars(bi)), b2c(on, gold, tc(bi)), false), func() {
				rs["li"] = float64(i)
				g.save()
			})
		}
		hz := "No hazards here."
		if h := sim.BiomeHazard[bi]; h != "" {
			hz = sim.HazardText[h]
		}
		u.label([4]float64{x0 + 800, y + 14, sw - x0 - 840, 18}, false, textDraw(hz, 12, false, dim, text.AlignStart))
	} else {
		u.label([4]float64{x0, y + 14, 600, 18}, false, textDraw("Random: you start in a random location. After every boss you pick your next location on the route map.", 12, false, dim, text.AlignStart))
	}
	y += 66
	// stage + lock
	u.label([4]float64{x0, y, 300, 18}, false, textDraw("Run length (clear a biome's tiers to unlock longer runs)", 13, true, white, text.AlignStart))
	y += 24
	smax := pr.StageMax(bi)
	for k := 1; k <= 3; k++ {
		k := k
		lk := k > smax
		on := g.stage() == k
		u.button(fmt.Sprintf("sg%d", k), [4]float64{x0 + float64(k-1)*200, y, 190, 46}, false, lk, btnDraw(fmt.Sprintf("Tier %d", k), fmt.Sprintf("%d minutes · %d bosses", 15*k, 3*k), b2c(on, gold, c(159, 232, 255, 255)), lk), func() {
			rs["sg"] = float64(k)
			g.save()
		})
	}
	lock, _ := rs["lock"].(bool)
	u.button("lock", [4]float64{x0 + 620, y, 230, 46}, false, false, btnDraw(map[bool]string{true: "Stay in biome: ON", false: "Stay in biome: OFF"}[lock], "Don't switch after bosses", b2c(lock, gold, dim), false), func() {
		rs["lock"] = !lock
		g.save()
	})
	y += 66
	// difficulty
	asc := pr.Asc()
	u.label([4]float64{x0, y, 300, 18}, false, textDraw(fmt.Sprintf("Difficulty (win a run to unlock the next; unlocked up to %d)", asc), 13, true, white, text.AlignStart))
	y += 24
	tw := (sw - 80 - 10*6) / 11
	for k := 0; k <= meta.TierMax; k++ {
		k := k
		lk := k > asc
		on := g.tier() == k
		u.button(fmt.Sprintf("t%d", k), [4]float64{x0 + float64(k)*(tw+6), y, tw, 44}, false, lk, btnDraw(fmt.Sprint(k), fmt.Sprintf("+%d%%", 12*k), b2c(on, c(255, 120, 120, 255), dim), lk), func() {
			rs["tier"] = float64(k)
			g.save()
		})
	}
	y += 64
	u.label([4]float64{x0, y, 300, 18}, false, textDraw("Modifiers (harder, but more coins)", 13, true, white, text.AlignStart))
	y += 24
	mods := g.mods()
	mw := (sw - 80 - 3*10) / 4
	for i, m := range meta.Mods {
		m := m
		on := contains(mods, m.Key)
		u.button("m:"+m.Key, [4]float64{x0 + float64(i%4)*(mw+10), y + float64(i/4)*56, mw, 48}, false, false, btnDraw(b2s(on, "ON  ", "")+m.Name+fmt.Sprintf("  +%d%%", int(m.Bonus*100)), m.Desc, b2c(on, c(255, 120, 120, 255), dim), false), func() {
			var l []any
			for _, x := range g.mods() {
				if x != m.Key {
					l = append(l, x)
				}
			}
			if !on {
				l = append(l, m.Key)
			}
			if l == nil {
				l = []any{}
			}
			rs["mods"] = l
			g.save()
		})
	}
	y += 120
	mult := 1 + .12*float64(g.tier())
	for _, m := range meta.Mods {
		if contains(mods, m.Key) {
			mult += m.Bonus
		}
	}
	u.label([4]float64{x0, y, sw - 80, 20}, false, textDraw(fmt.Sprintf("Coin rewards x%.2f", mult), 16, true, gold, text.AlignStart))
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

func (g *Game) upButton(key string, k meta.Upgrade, fam string, r [4]float64) {
	pr := g.prof
	l := pr.UL(k.Key, fam)
	state := pr.UpState(k.Key, fam)
	sub := fmt.Sprintf("Lv %d/%d · %s", l, k.Max, k.Desc)
	if k.Max == 1 {
		sub = k.Desc
	}
	price := fmt.Sprintf("%s coins", commas(pr.UpCost(k.Key, fam)))
	if state != "" {
		price = state
	}
	dis := state != ""
	g.ui.button(key, r, false, dis, func(dst *ebiten.Image, rr [4]float64, hot bool) {
		btnDraw(k.Name, sub, c(159, 232, 255, 255), dis && state != "Not enough coins")(dst, rr, hot)
		drawText(dst, price, rr[0]+rr[2]/2, rr[1]+rr[3]-17, 11, true, b2c(state == "", gold, b2c(state == "MAX" || state == "OWNED", c(120, 255, 140, 255), c(255, 140, 140, 255))), text.AlignCenter, false)
	}, func() {
		if pr.BuyUpgrade(k.Key, fam) {
			g.save()
		}
	})
}

func (g *Game) menuShop() {
	sw := float64(g.sw)
	g.header("SHOP", "Global upgrades, shared by every Pokémon")
	g.backButton(scMenu)
	i := 0
	bw := (sw - 80 - 30) / 4
	for _, k := range meta.Upgrades {
		if !k.Global {
			continue
		}
		g.upButton("u:"+k.Key, k, "", [4]float64{40 + float64(i%4)*(bw+10), 120 + float64(i/4)*96, bw, 84})
		i++
	}
}

func (g *Game) menuLine() {
	sw := float64(g.sw)
	pr := g.prof
	fam := g.lineFam
	name := g.db.ByID[fam].Name
	g.header("LINE UPGRADES · "+name+" line", "Shared by every evolution and form in this line")
	g.backButton(scChars)
	i := 0
	bw := (sw - 80 - 30) / 4
	for _, k := range meta.Upgrades {
		if k.Global {
			continue
		}
		g.upButton("u:"+k.Key, k, fam, [4]float64{40 + float64(i%4)*(bw+10), 120 + float64(i/4)*96, bw, 84})
		i++
	}
	r := pr.Training(fam)
	cost := meta.TrainingCost(r)
	mx := r >= meta.TrainingMax
	can := !mx && pr.S.Coins() >= cost
	sub := fmt.Sprintf("+2%% damage and +1%% coins per rank (now +%d%% / +%d%%)", 2*r, r)
	price := commas(cost) + " coins"
	if mx {
		price = "MAX"
	}
	g.ui.button("train", [4]float64{40, 330, sw - 80, 70}, false, !can, func(dst *ebiten.Image, rr [4]float64, hot bool) {
		btnDraw(fmt.Sprintf("Training Rank %d/%d", r, meta.TrainingMax), sub, gold, !can && !mx)(dst, rr, hot)
		drawText(dst, price, rr[0]+rr[2]/2, rr[1]+rr[3]-17, 11, true, b2c(can, gold, c(255, 140, 140, 255)), text.AlignCenter, false)
	}, func() {
		if pr.BuyTraining(fam) {
			g.save()
		}
	})
}

// ---- challenges ----

func (g *Game) menuChal() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	st := pr.S
	g.header("CHALLENGES", fmt.Sprintf("Defeated %s · Bosses %d · Best run %s · Wins %d · Runs %d", commas(int(st.Stat("k"))), int(st.Stat("b")), fmtT(st.Stat("t")), int(st.Stat("w")), int(st.Stat("r"))))
	g.backButton(scMenu)
	all := pr.Challenges()
	sort.SliceStable(all, func(i, j int) bool {
		rank := func(a meta.Ach) int {
			switch {
			case pr.Claimable(a):
				return 0
			case pr.Claimed(a):
				return 2
			}
			return 1
		}
		return rank(all[i]) < rank(all[j])
	})
	u.area = [4]float64{60, 100, sw - 120, sh - 120}
	y := 0.0
	for _, a := range all {
		a := a
		v := math.Min(a.Target, st.Stat(a.Stat))
		cl := pr.Claimed(a)
		ok := pr.Claimable(a)
		rew := fmt.Sprintf("%d coins", a.Coins)
		if a.Tickets > 0 {
			rew = fmt.Sprintf("%d ticket%s", a.Tickets, plural(a.Tickets))
		}
		lab := fmt.Sprintf("%s / %s", commas(int(v)), commas(int(a.Target)))
		if a.Stat == "t" {
			lab = fmtT(v) + " / " + fmtT(a.Target)
		}
		u.button("a:"+a.ID, [4]float64{60, y, sw - 130, 52}, true, !ok, func(dst *ebiten.Image, r [4]float64, hot bool) {
			bg := c(26, 32, 48, 240)
			if ok {
				bg = c(30, 56, 40, 245)
			}
			if hot {
				roundRect(dst, r[0]-2, r[1]-2, r[2]+4, r[3]+4, 9, alpha(b2c(ok, c(120, 255, 140, 255), dim), .8))
			}
			roundRect(dst, r[0], r[1], r[2], r[3], 7, bg)
			drawText(dst, a.Name, r[0]+14, r[1]+8, 14, true, b2c(cl, c(130, 136, 150, 255), white), text.AlignStart, false)
			drawText(dst, lab, r[0]+14, r[1]+30, 11, false, dim, text.AlignStart, false)
			bar(dst, r[0]+160, r[1]+33, r[2]-400, 6, v/a.Target, b2c(cl, c(90, 100, 110, 255), gold), c(40, 46, 60, 255))
			status := rew
			if cl {
				status = "Claimed"
			} else if ok {
				status = "Claim " + rew
			}
			drawText(dst, status, r[0]+r[2]-14, r[1]+17, 13, true, b2c(ok, c(120, 255, 140, 255), b2c(cl, c(120, 126, 140, 255), gold)), text.AlignEnd, false)
		}, func() {
			if pr.Claim(a) {
				g.toast("Claimed: " + rew)
				g.save()
			}
		})
		y += 58
	}
	u.scrollMax = math.Max(0, y-u.area[3])
}

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
	e := g.end
	w := g.W
	title := "FAINTED"
	col := c(255, 110, 110, 255)
	if e.win {
		title, col = "VICTORY", gold
	}
	u.label([4]float64{0, 40, sw, 44}, false, textDraw(title, 40, true, col, text.AlignCenter))
	u.label([4]float64{0, 96, sw, 20}, false, textDraw(fmt.Sprintf("%s · %s · Lv %d · %d defeated · %d bosses", w.P.Name, fmtT(w.T), w.P.Lv, w.Kills, w.Bosses), 16, true, white, text.AlignCenter))
	x := sw/2 - 300
	u.label([4]float64{x, 130, 600, 160}, false, func(dst *ebiten.Image, r [4]float64) {
		roundRect(dst, r[0], r[1], r[2], r[3], 12, panel)
		drawText(dst, fmt.Sprintf("+%s coins   (total %s)", commas(e.gain), commas(g.prof.S.Coins())), r[0]+20, r[1]+16, 16, true, gold, text.AlignStart, false)
		if e.tickets > 0 {
			drawText(dst, fmt.Sprintf("+%d ticket%s", e.tickets, plural(e.tickets)), r[0]+r[2]-20, r[1]+16, 16, true, c(255, 157, 232, 255), text.AlignEnd, false)
		}
		up := ""
		if e.rankUp > 0 {
			up = fmt.Sprintf("   ▲ +%d rank%s!", e.rankUp, plural(e.rankUp))
		}
		drawText(dst, fmt.Sprintf("Mastery +%s XP · Rank %d%s", commas(e.mx), e.rank, up), r[0]+20, r[1]+50, 14, true, white, text.AlignStart, false)
		bar(dst, r[0]+20, r[1]+74, r[2]-40, 8, float64(e.into)/float64(e.need), gold, c(40, 46, 60, 255))
		drawText(dst, fmt.Sprintf("%s / %s XP to rank %d", commas(e.into), commas(e.need), e.rank+1), r[0]+20, r[1]+88, 12, false, dim, text.AlignStart, false)
		for i, id := range e.unlocked {
			drawText(dst, g.db.ByID[id].Name+" unlocked!", r[0]+20, r[1]+112+float64(i)*20, 14, true, gold, text.AlignStart, false)
		}
		if n := g.prof.ReadyChallenges(); n > 0 {
			drawText(dst, fmt.Sprintf("%d challenge%s ready to claim", n, plural(n)), r[0]+r[2]-20, r[1]+r[3]-26, 13, true, c(120, 255, 140, 255), text.AlignEnd, false)
		}
	})
	// damage breakdown
	type kv struct {
		k string
		v float64
	}
	var l []kv
	tot := 0.0
	for k, v := range w.DMG {
		l = append(l, kv{k, v})
		tot += v
	}
	sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
	y := 310.0
	for i, d := range l {
		if i >= 6 {
			break
		}
		d := d
		name, col := "Burn, poison & other", dim
		if m := g.db.Moves[d.k]; m != nil {
			name, col = m.Name, tc(m.Type)
		}
		u.label([4]float64{x, y, 600, 20}, false, func(dst *ebiten.Image, r [4]float64) {
			bar(dst, r[0], r[1]+3, r[2], 14, d.v/math.Max(tot, 1), alpha(col, .8), c(40, 46, 60, 255))
			drawText(dst, fmt.Sprintf("%s  %s (%d%%)", name, commas(int(d.v)), int(d.v/math.Max(tot, 1)*100)), r[0]+r[2]/2, r[1]+2, 12, true, white, text.AlignCenter, true)
		})
		y += 24
	}
	u.button("again", [4]float64{sw/2 - 250, y + 20, 240, 50}, false, false, btnDraw("Play again", "Same Pokémon and setup", gold, false), func() { g.startFromMenu() })
	u.button("menu", [4]float64{sw/2 + 10, y + 20, 240, 50}, false, false, btnDraw("Main menu", "", c(159, 232, 255, 255), false), func() { g.scr = scMenu; g.ui.focus = "" })
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
	res := meta.RunResult{CID: w.P.ID, T: w.T, Kills: w.Kills, Lv: w.P.Lv, Win: w.Won, Coins: w.Coins, Tickets: w.Tickets,
		Elites: w.Stats.Elites, MiniBosses: w.Stats.Minis, EliteTk: w.Stats.EliteTk, Cursed: w.Stats.Cursed, Curses: w.Stats.Curses,
		Bosses: w.Bosses, Ability: w.PS.A.ID, AbilityName: w.PS.A.Name, Tier: w.Run.Tier, Mods: w.Run.Mods, Stage: w.Run.Stage, Mult: w.Run.Mult,
		Visited: w.Visited, Dmg: w.DMG, Moves: moves, Items: items, Learned: keys(w.Learned), Evolved: keys(w.Evolved), Capstone: keys(w.Capstone)}
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
	default:
		g.scr = scMenu
	}
}
