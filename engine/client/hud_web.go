package client

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// In-run screens in the web version's layout: the HUD (level pill, segmented XP bar, HP bar, coins,
// location and timer, pause button, equipped bar), level-up cards with stat changes, the pause menu.

// ---- HUD ----

func (g *Game) pauseBtnRect() [4]float64 { return [4]float64{float64(g.sw) - 50, 38, 38, 26} }

// drawWebHUD is the web-style HUD (Settings → HUD layout: Web): bars on top, equipped bar under them.
func (g *Game) drawWebHUD(dst *ebiten.Image) {
	g.drawTopBars(dst)
	g.drawBossWarn(dst)
	if g.opt("eq") != 0 {
		g.drawEquipped(dst, 8, 72)
	}
	g.drawHUDTail(dst)
}

// drawTopBars: level pill + XP bar, then the HP bar (solo), coins, location and time, pause button.
func (g *Game) drawTopBars(dst *ebiten.Image) {
	w := g.W
	P := w.P
	sw := float64(g.sw)
	// level pill + XP bar
	lv := "Lv " + strconv.Itoa(P.Lv)
	if g.coop {
		lv = "Team Lv " + strconv.Itoa(w.XPS.Lv)
	}
	pw := math.Max(56, textW(lv, 14, true)+22)
	soft(dst, 12+pw/2, 19, pw*.7, 16, alpha(hexC("#5ad1ff"), .35))
	webBox(dst, 12, 8, pw, 24, 9, hexC("#5ad1ff"), hexC("#0a0d14"))
	drawText(dst, lv, 12+pw/2, 11, 14, true, white, text.AlignCenter, true)
	bx := 12 + pw + 10
	bw := sw - 12 - bx
	webBox(dst, bx, 8, bw, 24, 11, hexC("#2a3a5c"), hexC("#080b12"))
	f := math.Max(0, math.Min(1, w.XPS.XP/math.Max(1, w.XPS.NX)))
	if f > 0 {
		fw := (bw - 4) * f
		roundRect(dst, bx+2, 10, fw, 20, 9, hexC("#35b4ff"))
		roundRect(dst, bx+2, 10, fw, 8, 4, alpha(hexC("#a8f0ff"), .8))
		rect(dst, bx+2+fw-2, 10, 2, 20, white)
	}
	for i := 1; i < 10; i++ {
		rect(dst, bx+2+(bw-4)*float64(i)/10-1, 10, 1, 20, alpha(white, .15))
	}
	drawText(dst, fmt.Sprintf("%d / %d XP", int(w.XPS.XP), int(w.XPS.NX)), bx+bw/2, 13, 11, true, white, text.AlignCenter, true)
	x := 258.0
	if g.coop {
		x = 12
	} else {
		// HP bar
		hpf := math.Max(0, math.Min(1, P.HP/P.MHP))
		border := hexC("#3a2a2a")
		if hpf < .25 && (g.opt("flash") == 0 || g.tick/20%2 == 0) {
			border = hexC("#ff4d4d")
		}
		webBox(dst, 12, 38, 234, 24, 11, border, hexC("#080b12"))
		if hpf > 0 {
			top, mid := hexC("#8ff5a6"), hexC("#2fc45a")
			if hpf < .25 {
				top, mid = hexC("#ff9a8a"), hexC("#ee3b2b")
			} else if hpf < .5 {
				top, mid = hexC("#ffe58a"), hexC("#f0b429")
			}
			roundRect(dst, 14, 40, 230*hpf, 20, 9, mid)
			roundRect(dst, 14, 40, 230*hpf, 8, 4, alpha(top, .85))
		}
		drawText(dst, fmt.Sprintf("%d / %d", int(math.Ceil(P.HP)), int(P.MHP)), 12+117, 42, 12, true, white, text.AlignCenter, true)
	}
	// coins, location and time, buffs
	x += richLine(dst, x, 41, 15, true, webFg, gCoin, strconv.Itoa(int(w.Coins))) + 14
	tm := w.LocName(w.Biome, w.LI) + " · " + fmtT(w.T) + " / " + fmtT(w.Run.Len)
	if w.Endless {
		tm = w.LocName(w.Biome, w.LI) + " · ENDLESS " + fmtT(w.T)
	}
	drawText(dst, tm, x, 41, 15, true, b2c(w.Endless, c(255, 130, 130, 255), webFg), text.AlignStart, true)
	x += textW(tm, 15, true) + 14
	if bt := buffText(w); bt != "" {
		drawText(dst, bt, x, 42, 13, true, c(127, 224, 255, 255), text.AlignStart, true)
	}
	// pause button
	if g.coop {
		return
	}
	pr := g.pauseBtnRect()
	webBox(dst, pr[0], pr[1], pr[2], pr[3], 8, webBorder, alpha(hexC("#0a0d14"), .8))
	rect(dst, pr[0]+pr[2]/2-5, pr[1]+7, 3.5, 12, webFg)
	rect(dst, pr[0]+pr[2]/2+1.5, pr[1]+7, 3.5, 12, webFg)
}

func (g *Game) drawBossWarn(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	// boss warning and boss bar (centre)
	if nb := 300 * math.Ceil((w.T+.01)/300); nb-w.T < 20 && w.Bosses < int(nb/300) {
		drawText(dst, fmt.Sprintf("Boss in %.0fs", nb-w.T), sw/2, 72, 14, true, c(255, 120, 120, 255), text.AlignCenter, true)
	}
	g.drawBossBar(dst)
}

func (g *Game) drawBossBar(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	for _, e := range w.E {
		if !e.Boss || e.Dead {
			continue
		}
		bw := math.Min(520, sw-360)
		bx := sw/2 - bw/2
		name := e.BD.Name
		if p := g.db.ByID[e.PokeID]; p != nil {
			name += " · " + p.Name
		}
		drawText(dst, fmt.Sprintf("%s  ·  Phase %d", name, max(1, e.Ph)), sw/2, 92, 13, true, tc(e.Type), text.AlignCenter, true)
		webBox(dst, bx, 112, bw, 16, 8, hexC("#5a2a2a"), hexC("#080b12"))
		if f := e.HP / e.MHP; f > 0 {
			roundRect(dst, bx+2, 114, (bw-4)*f, 12, 6, c(235, 70, 70, 255))
		}
		break
	}
}

// itemAbbr: a two-letter label for a held item (the web version shows emoji icons, which this
// build's font does not have).
func itemAbbr(id string) string {
	it := sim.ItemByID[id]
	if it == nil {
		return "?"
	}
	ws := strings.Fields(it.Name)
	if len(ws) >= 2 {
		return strings.ToUpper(ws[0][:1]) + strings.ToUpper(ws[1][:1])
	}
	r := []rune(it.Name)
	if len(r) >= 2 {
		return string(r[0]) + string(r[1])
	}
	return it.Name
}

// drawEquipped is the web "equipped bar": move tiles (type icon, level, charge) and item circles.
func (g *Game) drawEquipped(dst *ebiten.Image, x0, y0 float64) {
	w := g.W
	x := x0
	for _, s := range w.Slots {
		col := tc(s.Type)
		webBox(dst, x, y0, 38, 38, 8, col, alpha(hexC("#0a0d14"), .85))
		if s.Evo {
			soft(dst, x+19, y0+19, 22, 22, alpha(webAcc, .35))
		}
		g.drawTypeIcon(dst, s.Type, x+19, y0+18, 24, false)
		f := w.CoolFrac(s)
		if s.Kind == "dash" {
			f = w.DashReady()
			drawText(dst, "SHIFT", x+4, y0+2, 8, true, webAcc, text.AlignStart, true)
		}
		if f < 1 {
			rect(dst, x+4, y0+33, 30*f, 2.5, alpha(white, .85))
		}
		drawText(dst, strconv.Itoa(s.Lv), x+35, y0+22, 11, true, white, text.AlignEnd, true)
		x += 42
	}
	x = x0
	y := y0 + 42
	for _, id := range w.HeldOrder {
		lv := w.Held[id]
		if lv == 0 {
			continue
		}
		disc(dst, x+19, y+19, 19, webAcc)
		disc(dst, x+19, y+19, 17, alpha(hexC("#0a0d14"), .92))
		drawText(dst, itemAbbr(id), x+19, y+12, 13, true, webFg, text.AlignCenter, false)
		drawText(dst, strconv.Itoa(lv), x+36, y+24, 10, true, white, text.AlignEnd, true)
		x += 42
	}
}

// ---- level-up cards (web choices()) ----

type lvRow struct {
	label, a, b, delta string
	good               bool
}

type lvCard struct {
	num      int
	icon     string // type icon for moves, "" for items
	abbr     string // item label
	title    string
	titleCol color.NRGBA
	badge    string
	tag      string
	tagCol   color.NRGBA
	strong   []string // new move: strong-vs types
	rows     []lvRow
	notes    []struct {
		s   string
		col color.NRGBA
	}
	hl     bool // synergy highlight (gold border)
	w, h   float64
	wrapNs [][]string
}

func (c *lvCard) note(s string, col color.NRGBA) {
	c.notes = append(c.notes, struct {
		s   string
		col color.NRGBA
	}{s, col})
}

var itemFX = map[string][]struct {
	label string
	f     func(p float64) float64
	unit  string
	low   bool
	bad   bool
}{
	"band":    {{"Damage", func(p float64) float64 { return .12 * p }, "%", false, false}},
	"claw":    {{"Cooldowns", func(p float64) float64 { return -(1 - math.Pow(.92, p)) }, "%", true, false}},
	"left":    {{"Regeneration", func(p float64) float64 { return .45 * p }, "hp", false, false}},
	"lens":    {{"Crit chance", func(p float64) float64 { return .08 * p }, "%", false, false}},
	"boots":   {{"Move speed", func(p float64) float64 { return .08 * p }, "%", false, false}},
	"metal":   {{"Damage taken", func(p float64) float64 { return -(1 - math.Pow(.93, p)) }, "%", true, false}},
	"wide":    {{"Size", func(p float64) float64 { return .2 * p }, "%", false, false}, {"Arc/aura/blast radius", func(p float64) float64 { return .1 * p }, "%", false, false}},
	"magnet":  {{"Pickup range", func(p float64) float64 { return .25 * p }, "%", false, false}},
	"exp":     {{"XP gained", func(p float64) float64 { return .1 * p }, "%", false, false}},
	"soothe":  {{"Heal per damage", func(p float64) float64 { return .005 * p }, "%", false, false}},
	"glove":   {{"Extra projectiles", func(p float64) float64 { return math.Ceil(p / 2) }, "n", false, false}},
	"focus":   {{"Range", func(p float64) float64 { return .15 * p }, "%", false, false}},
	"metro":   {{"Max ramp (10 casts)", func(p float64) float64 { return .1 * p }, "%", false, false}, {"Proj./orbit speed", func(p float64) float64 { return .12 * p }, "%", false, false}},
	"eject":   {{"Knockback power", func(p float64) float64 { return p }, "x", false, false}},
	"belt":    {{"Super-effective dmg", func(p float64) float64 { return .25 * p }, "%", false, false}, {"Resisted hits", func(p float64) float64 { return .08 * p }, "%", false, false}},
	"heat":    {{"Burn chance on hit", func(p float64) float64 { return .05 * p }, "%", false, false}, {"Dmg vs burning/poisoned", func(p float64) float64 { return .04 * p }, "%", false, false}, {"Burn/poison damage", func(p float64) float64 { return .15 * p }, "%", false, false}, {"Trail duration", func(p float64) float64 { return .2 * p }, "%", false, false}},
	"helmet":  {{"Contact damage", func(p float64) float64 { return 4 * p }, "n", false, false}},
	"choice":  {{"Top move damage", func(p float64) float64 { return .09 * p }, "%", false, false}, {"Other moves", func(p float64) float64 { return -math.Min(.5, .04*p) }, "%", false, true}},
	"lifeorb": {{"Damage", func(p float64) float64 { return .1 * p }, "%", false, false}, {"HP drain / sec", func(p float64) float64 { return -.0015 * p }, "%", false, true}},
	"sash":    {{"Recharge (sec)", func(p float64) float64 { return math.Max(60, 150-12*p) }, "n", false, false}},
	"muscle":  {{"Melee damage", func(p float64) float64 { return .14 * p }, "%", false, false}},
	"wise":    {{"Ranged damage", func(p float64) float64 { return .14 * p }, "%", false, false}},
	"razor":   {{"Crit chance", func(p float64) float64 { return .03 * p }, "%", false, false}, {"Crit damage", func(p float64) float64 { return .2 * p }, "%", false, false}},
}

func fmtFX(v float64, unit string) string {
	switch unit {
	case "%":
		s := fmt.Sprintf("%+d%%", int(math.Round(v*100)))
		if math.Abs(v*100-math.Round(v*100)) > .05 && math.Abs(v) < .1 {
			s = fmt.Sprintf("%+.1f%%", v*100)
		}
		return s
	case "hp":
		return fmt.Sprintf("+%s HP/s", trimF(math.Round(v*100)/100))
	case "x":
		return fmt.Sprintf("+%d", int(math.Round(v)))
	}
	return trimF(math.Round(v*10) / 10)
}

func (g *Game) lvCards() []*lvCard {
	w := g.W
	var out []*lvCard
	held := func(id string) bool { return w.Held[id] > 0 }
	for i, o := range w.Opts {
		c := &lvCard{num: i + 1, titleCol: webFg, tagCol: webFg}
		if o.Move {
			m := g.db.Moves[o.ID]
			c.icon = m.Type
			c.titleCol = tc(m.Type)
			c.title = m.Name
			it := sim.ItemByID[m.HeldItem]
			switch {
			case o.Evo:
				c.title += " (evolved)"
				c.badge = "Lv 10"
				c.tag, c.tagCol = "EVOLUTION", webAcc
				a, _ := w.MovePreview(o.ID, 10, false)
				b, _ := w.MovePreview(o.ID, 10, true)
				c.rows = append(c.rows, dmgRow(a, b))
				if it != nil {
					c.note(it.Name+" held · each item level: +8% evolved damage", alpha(webFg, .75))
				}
				c.note("Unlocks Lv 11–20 · Capstone at Lv 20", alpha(webFg, .75))
			case o.New || o.Level == 0:
				c.badge = "NEW"
				c.tag, c.tagCol = "NEW MOVE · "+strings.ToUpper(m.Type[:1])+m.Type[1:], hexC("#6fe38f")
				c.strong = sim.SE[m.Type]
				d, cd := w.MovePreview(o.ID, 1, false)
				c.rows = append(c.rows, lvRow{label: "Damage", b: strconv.Itoa(int(math.Round(d)))})
				if cd > 0 {
					c.rows = append(c.rows, lvRow{label: "Cooldown", b: fmt.Sprintf("%.2fs", cd)})
				}
				c.note(m.Desc, alpha(webFg, .75))
				if it != nil {
					col := webAcc
					s := "Evolves at Lv 10 · needs " + it.Name
					if held(m.HeldItem) {
						col, s = hexC("#6fe38f"), s+" (held)"
					}
					c.note(s, col)
				}
			default:
				lv := o.Level
				ev := g.slotEvo(o.ID)
				c.badge = fmt.Sprintf("Lv %d → %d", lv, lv+1)
				if ev && lv+1 == 20 {
					c.tag, c.tagCol = "CAPSTONE PERK", webAcc
				}
				a, ca := w.MovePreview(o.ID, lv, ev)
				b, cb := w.MovePreview(o.ID, lv+1, ev)
				c.rows = append(c.rows, dmgRow(a, b))
				if math.Abs(ca-cb) > .005 {
					c.rows = append(c.rows, lvRow{label: "Cooldown", a: fmt.Sprintf("%.2fs", ca), b: fmt.Sprintf("%.2fs", cb), delta: fmt.Sprintf("%d%%", int(math.Round((cb-ca)/ca*100))), good: cb < ca})
				}
				if !ev && it != nil {
					col := webAcc
					s := "Evolves at Lv 10 · needs " + it.Name
					if held(m.HeldItem) {
						col, s = hexC("#6fe38f"), s+" (held)"
					}
					c.note(s, col)
				}
			}
		} else {
			it := sim.ItemByID[o.ID]
			c.abbr = itemAbbr(o.ID)
			c.title = it.Name
			lv := o.Level
			c.badge = "NEW"
			if lv > 0 {
				c.badge = fmt.Sprintf("Lv %d → %d", lv, lv+1)
			}
			for _, fx := range itemFX[o.ID] {
				a, b := fx.f(sim.PE(lv)), fx.f(sim.PE(lv+1))
				c.rows = append(c.rows, lvRow{label: fx.label, a: fmtFX(a, fx.unit), b: fmtFX(b, fx.unit)})
			}
			if lv > 0 {
				c.note("Each level: "+it.Desc, alpha(webFg, .75))
			} else {
				c.note(it.Desc, alpha(webFg, .75))
			}
			if lv >= 5 {
				c.note("Levels 6–10 give half effect", alpha(webFg, .75))
			}
			if o.Swap != "" {
				c.badge = "SWAP"
				c.note(fmt.Sprintf("Replaces %s (Lv %d)", sim.ItemByID[o.Swap].Name, w.Held[o.Swap]), hexC("#7fd8ff"))
			}
			for _, s := range w.Slots {
				if s.Move.HeldItem == o.ID {
					c.hl = true
					if s.Evo {
						c.note("+8% damage: "+s.Move.Name, webAcc)
					} else if s.Lv >= 10 {
						c.note("Unlocks evolution: "+s.Move.Name+" (ready now!)", webAcc)
					} else {
						c.note("Unlocks evolution: "+s.Move.Name+" (at Lv 10)", webAcc)
					}
				}
			}
		}
		out = append(out, c)
	}
	return out
}

func dmgRow(a, b float64) lvRow {
	r := lvRow{label: "Damage", a: strconv.Itoa(int(math.Round(a))), b: strconv.Itoa(int(math.Round(b))), good: true}
	if a > 0 && math.Abs(b-a) > 1e-9 {
		r.delta = fmt.Sprintf("%+d%%", int(math.Round((b-a)/a*100)))
	}
	return r
}

func (g *Game) slotEvo(id string) bool {
	for _, s := range g.W.Slots {
		if s.ID == id {
			return s.Evo
		}
	}
	return false
}

// lvLayout sizes and places the level-up cards (centred row, web min 205 / max 250 wide).
func (g *Game) lvLayout() ([]*lvCard, [][4]float64) {
	cards := g.lvCards()
	sw, sh := float64(g.sw), float64(g.sh)
	maxH := 0.0
	tot := -10.0
	for _, c := range cards {
		mw := textW(fmt.Sprintf("%d. ", c.num)+c.title, 15, true) + 24 + textW(c.badge, 12, false) + 20
		for _, r := range c.rows {
			mw = math.Max(mw, textW(r.label, 13, false)+textW(r.a+" → "+r.b, 13, true)+textW(r.delta, 12, false)+20)
		}
		c.w = math.Max(205, math.Min(250, mw+28))
		h := 12 + 22.0
		if c.tag != "" {
			h += 16
		}
		if len(c.strong) > 0 {
			h += g.matchupRows(nil, []string{c.icon}, 0, 0, c.w-28, true) + 2
		}
		h += float64(len(c.rows)) * 21
		c.wrapNs = nil
		for _, n := range c.notes {
			ls := wrap(n.s, font(12, false), c.w-28)
			c.wrapNs = append(c.wrapNs, ls)
			h += float64(len(ls))*15 + 3
		}
		c.h = h + 12
		maxH = math.Max(maxH, c.h)
		tot += c.w + 10
	}
	scale := 1.0
	if tot > sw-24 {
		scale = (sw - 24) / tot
	}
	x := sw/2 - tot*scale/2
	y := sh/2 - maxH/2 + 10
	var rs [][4]float64
	for _, c := range cards {
		rs = append(rs, [4]float64{x, y, c.w * scale, maxH})
		x += (c.w + 10) * scale
	}
	return cards, rs
}

func (g *Game) drawWebLevelUp(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	rect(dst, 0, 0, sw, float64(g.sh), c(0, 0, 0, 185))
	cards, rs := g.lvLayout()
	if len(rs) == 0 {
		return
	}
	title := "Level up!"
	if g.banMode {
		title = "Pick a choice to banish"
	}
	if g.coop {
		title = w.ChooserName() + " — " + title
	}
	drawText(dst, title, sw/2, rs[0][1]-40, 24, true, white, text.AlignCenter, true)
	mx, my := ebiten.CursorPosition()
	hov := g.cardHit(mx, my)
	for i, c := range cards {
		r := rs[i]
		sel := i == g.lvSel || i == hov
		border := webBorder
		switch {
		case g.banMode:
			border = hexC("#ff5d5d")
		case sel:
			border = webAcc
		case c.hl:
			border = webAcc
		}
		if c.hl && !g.banMode {
			soft(dst, r[0]+r[2]/2, r[1]+r[3]/2, r[2]*.62, r[3]*.62, alpha(webAcc, .3))
		}
		bg := webPanel
		if sel {
			bg = mix(webPanel, webAcc, .06)
		}
		webBox(dst, r[0], r[1], r[2], r[3], 10, border, bg)
		x := r[0] + 14
		y := r[1] + 12
		iw := r[2] - 28
		// header: "1. [icon] Name" and the badge
		bw := textW(c.badge, 12, false) + 12
		hx := x
		hx += richLine(dst, hx, y+1, 15, true, c.titleCol, fmt.Sprintf("%d. ", c.num))
		if c.icon != "" {
			g.drawTypeIcon(dst, c.icon, hx+8, y+10, 17, false)
			hx += 21
		} else {
			disc(dst, hx+9, y+10, 9.5, webAcc)
			disc(dst, hx+9, y+10, 8, hexC("#0a0d14"))
			drawText(dst, c.abbr, hx+9, y+4, 8, true, webFg, text.AlignCenter, false)
			hx += 22
		}
		drawText(dst, fitText(c.title, 15, true, x+iw-bw-6-hx), hx, y+1, 15, true, c.titleCol, text.AlignStart, false)
		roundRect(dst, x+iw-bw, y+2, bw, 18, 6, alpha(white, .1))
		drawText(dst, c.badge, x+iw-bw/2, y+3, 12, false, webFg, text.AlignCenter, false)
		y += 24
		if c.tag != "" {
			drawText(dst, c.tag, x, y, 11, true, c.tagCol, text.AlignStart, false)
			y += 16
		}
		if len(c.strong) > 0 {
			y += g.matchupRows(dst, []string{c.icon}, x, y, iw, true) + 2
		}
		for _, row := range c.rows {
			rect(dst, x, y, iw, 1, alpha(white, .07))
			drawText(dst, row.label, x, y+4, 13, false, alpha(webFg, .8), text.AlignStart, false)
			rx := x + iw
			if row.delta != "" {
				dc := hexC("#6fe38f")
				if !row.good {
					dc = hexC("#ff8a7a")
				}
				drawText(dst, row.delta, rx, y+5, 12, false, dc, text.AlignEnd, false)
				rx -= textW(row.delta, 12, false) + 4
			}
			v := row.b
			if row.a != "" {
				v = row.a + " → " + row.b
			}
			drawText(dst, v, rx, y+4, 13, true, webFg, text.AlignEnd, false)
			y += 21
		}
		for ni, ls := range c.wrapNs {
			for _, l := range ls {
				drawText(dst, l, x, y+2, 12, false, c.notes[ni].col, text.AlignStart, false)
				y += 15
			}
			y += 3
		}
	}
	// reroll / banish tools
	if w.Rerolls+w.Banishes > 0 || g.prof.UL("rr", "") > 0 || g.prof.UL("bn", "") > 0 {
		ty := rs[0][1] + rs[0][3] + 14
		l1 := fmt.Sprintf("Reroll (%d) [R]", w.Rerolls)
		l2 := fmt.Sprintf("Banish (%d) [X]", w.Banishes)
		if g.banMode {
			l2 = "Cancel banish [X]"
		}
		w1, w2 := smallBtnW(l1, gDice, " "), smallBtnW(l2, gLock, " ")
		x := sw/2 - (w1+w2+8)/2
		smallBtn(dst, [4]float64{x, ty, w1, 30}, l1, false, w.Rerolls <= 0 || g.banMode, gDice, " ")
		smallBtn(dst, [4]float64{x + w1 + 8, ty, w2, 30}, l2, false, !g.banMode && w.Banishes <= 0, gLock, " ")
	}
}

// ---- pause (web #pause) ----

func (g *Game) webPauseRects() [4][4]float64 {
	cx := float64(g.sw) / 2
	y := float64(g.sh)/2 - 40
	return [4][4]float64{{cx - 120, y, 240, 52}, {cx - 120, y + 63, 240, 52}, {cx - 123, y + 126, 246, 52}, {cx - 135, y + 189, 270, 52}}
}

func (g *Game) drawWebPause(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	rect(dst, 0, 0, sw, float64(g.sh), c(0, 0, 0, 185))
	rs := g.webPauseRects()
	top := rs[0][1] - 190
	drawText(dst, "Paused", sw/2, top, 32, true, white, text.AlignCenter, true)
	info := []any{true, w.P.Name, false, fmt.Sprintf(" · Lv %d · %s · %d kills", w.P.Lv, fmtT(w.T), w.Kills)}
	iw := richLine(nil, 0, 0, 13, false, webFg, info...)
	richLine(dst, sw/2-iw/2, top+44, 13, false, webFg, info...)
	drawText(dst, fmt.Sprintf("%s · Difficulty %d", w.LocName(w.Biome, w.LI), w.Run.Tier), sw/2, top+64, 13, false, webFg, text.AlignCenter, true)
	// equipped, centred
	n := len(w.Slots)
	g.drawEquippedCentred(dst, sw/2, top+86, n)
	labels := [4][]any{{gPlay, " Resume"}, {gGear, " Settings"}, {gTrophy, " Retire & bank rewards"}, {"× Abandon run (no rewards)"}}
	if g.pauseQuit {
		labels[3] = []any{"Click again to abandon"}
	}
	mx, my := ebiten.CursorPosition()
	for i, r := range rs {
		hot := inR(r, float64(mx), float64(my)) || g.pauseSel == i
		webBtnDraw(dst, r, hot, false, false)
		lw := richLine(nil, 0, 0, 17, true, webFg, labels[i]...)
		richLine(dst, r[0]+r[2]/2-lw/2, r[1]+16, 17, true, webFg, append([]any{white}, labels[i]...)...)
	}
	drawText(dst, "Esc or P also pauses / resumes.", sw/2, rs[3][1]+66, 12, false, alpha(webFg, .9), text.AlignCenter, true)
}

func (g *Game) drawEquippedCentred(dst *ebiten.Image, cx, y float64, n int) {
	w := g.W
	items := 0
	for _, id := range w.HeldOrder {
		if w.Held[id] > 0 {
			items++
		}
	}
	mw := math.Max(float64(n), float64(items))*42 - 4
	if mw < 0 {
		return
	}
	sub := ebiten.NewImage(int(mw)+2, 84)
	defer sub.Deallocate()
	g.drawEquipped(sub, 0, 0)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cx-mw/2, y)
	dst.DrawImage(sub, op)
}

// ---- results (web #end) ----

func (g *Game) drawDamageBy(dst *ebiten.Image, x, y, w float64) float64 {
	type kv struct {
		k string
		v float64
	}
	var l []kv
	tot := 0.0
	for k, v := range g.W.DMG {
		if v > 0 {
			l = append(l, kv{k, v})
			tot += v
		}
	}
	if len(l) == 0 {
		return 0
	}
	sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v })
	if len(l) > 8 {
		l = l[:8]
	}
	drawText(dst, "Damage by attack", x, y, 12, true, webFg, text.AlignStart, false)
	y += 17
	mx := l[0].v
	for _, d := range l {
		name, col := "Burn, poison & other", hexC("#99aaaa")
		if m := g.db.Moves[d.k]; m != nil {
			name, col = m.Name, tc(m.Type)
		}
		drawText(dst, fitText(name, 12, false, 120), x, y, 12, false, webFg, text.AlignStart, false)
		bw := math.Max(2, d.v/mx*150)
		roundRect(dst, x+126, y+4, bw, 8, 4, col)
		drawText(dst, fmt.Sprintf("%s (%d%%)", commas(int(math.Round(d.v))), int(math.Round(d.v/math.Max(tot, 1)*100))), x+132+bw, y, 12, false, alpha(webFg, .8), text.AlignStart, false)
		y += 17
	}
	return 17 * float64(len(l)+1)
}

// updatePause handles the pause menu: Resume, Settings, Retire & bank rewards, Abandon (press twice).
func (g *Game) updatePause() {
	g.touchInput()
	act := -1
	if keyJust(ebiten.KeyEscape, ebiten.KeyP) || padBtn(ebiten.StandardGamepadButtonCenterRight) || g.touch.pauseTapped {
		act = 0
	}
	if _, dy := nav(); dy != 0 {
		g.pauseSel = (g.pauseSel + dy + 4) % 4
	}
	if confirm() {
		act = g.pauseSel
	}
	if keyJust(ebiten.KeyS) || padBtn(ebiten.StandardGamepadButtonRightTop) {
		act = 1
	}
	if keyJust(ebiten.KeyQ) {
		act = 3
	}
	mx, my := ebiten.CursorPosition()
	for i, r := range g.webPauseRects() {
		if inR(r, float64(mx), float64(my)) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			act = i
		}
	}
	if x, y, ok := g.touchTap(); ok {
		for i, r := range g.webPauseRects() {
			if inR(r, x, y) {
				act = i
			}
		}
	}
	switch act {
	case 0:
		g.scr = scPlay
		g.pauseQuit = false
	case 1:
		g.settingsFrom = scPause
		g.scr = scSettings
		g.ui.focus = ""
		g.ui.scroll = 0
	case 2: // retire: end the run now and keep the rewards (web finish())
		g.scr = scPlay
		g.retired = true
		g.W.State = sim.StateDead
	case 3:
		if !g.pauseQuit {
			g.pauseQuit = true
			g.pauseSel = 3
			return
		}
		g.pauseQuit = false
		g.leaveRoom()
		g.scr = scMenu // abandon: no rewards (same as the web game)
		g.ui.focus = ""
	}
}

// TestPause / TestResults / TestLevelUp open those screens for screenshots (after a bot demo).
func (g *Game) TestPause() { g.auto = false; g.scr = scPause }

func (g *Game) TestResults() {
	g.auto = false
	g.W.State = sim.StateDead
	g.finishRun()
	g.scr = scResult
}

func (g *Game) TestRoute() {
	g.auto = false
	g.Hold = true
	g.W.OpenRouteNow()
}

func (g *Game) TestLevelUp() {
	g.auto = false
	g.Hold = true
	w := g.W
	w.XPS.XP += w.XPS.NX
	for i := 0; i < 600 && w.State != sim.StateLevelUp; i++ {
		w.Step(1.0/60, sim.Input{})
	}
}

// ---- route map (web routeUI) ----

var hazardShort = map[string]string{"lava": "Lava pools", "deep": "Deep water", "ice": "Slippery ice", "toxic": "Toxic pools", "sand": "Quicksand",
	"web": "Sticky webs", "storm": "Lightning", "rockfall": "Rockfall", "gust": "Strong gusts"}

// routeGeom: node positions in the 400×430 map (web routeGeom).
func routeGeom(n int) (cx, cy float64, pos [][2]float64) {
	cx, cy = 200, 372
	for i := 0; i < n; i++ {
		u := .5
		if n > 1 {
			u = float64(i) / float64(n-1)
		}
		x := 50 + u*300
		if n == 1 {
			x = cx
		}
		y := 86 + math.Abs(u-.5)*2*34
		if n >= 4 {
			y = 78
			if i%2 == 1 {
				y = 182
			}
		}
		pos = append(pos, [2]float64{x, y})
	}
	return
}

// routeFrame: where the map is drawn on screen and its scale.
func (g *Game) routeFrame() (x0, y0, k float64) {
	k = math.Min(1.08, (float64(g.sh)-170)/430)
	x0 = float64(g.sw)/2 - 200*k
	y0 = 140
	return
}

// routeRects: the clickable nodes (screen space), in route option order.
func (g *Game) routeRects() [][4]float64 {
	x0, y0, k := g.routeFrame()
	_, _, pos := routeGeom(len(g.W.Route))
	var out [][4]float64
	for _, p := range pos {
		out = append(out, [4]float64{x0 + (p[0]-36)*k, y0 + (p[1]-36)*k, 72 * k, 72 * k})
	}
	return out
}

func (g *Game) drawWebRoute(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	rect(dst, 0, 0, sw, float64(g.sh), c(0, 0, 0, 185))
	x0, y0, k := g.routeFrame()
	tw := richLine(nil, 0, 0, 24, true, webFg, gPin, "Choose your route")
	richLine(dst, sw/2-tw/2, y0-50, 24, true, webFg, gPin, "Choose your route")
	sub := "Boss defeated! Pick your next location."
	if w.Endless {
		sub = fmt.Sprintf("Endless mode · %d bosses defeated · pick your next location.", w.Bosses)
	}
	drawText(dst, sub, sw/2, y0-20, 13, false, alpha(webFg, .9), text.AlignCenter, true)
	X := func(x float64) float64 { return x0 + x*k }
	Y := func(y float64) float64 { return y0 + y*k }
	gold := hexC("#c9b27a")
	roundRect(dst, X(4), Y(4), 392*k, 422*k, 18*k, gold)
	roundRect(dst, X(4)+3, Y(4)+3, 392*k-6, 422*k-6, 16*k, hexC("#1a1f2e"))
	for i := 0; i < 26; i++ {
		disc(dst, X(20+float64((i*97)%360)), Y(30+float64((i*61)%370)), float64(2+i%3)*k, alpha(gold, .12))
	}
	cx, cy, pos := routeGeom(len(w.Route))
	// dotted paths (cubic curves, a dot every 13 px)
	for _, p := range pos {
		pts := [][2]float64{{cx, cy - 30}, {cx, cy - 62}}
		a, c1, c2, b := [2]float64{cx, cy - 62}, [2]float64{cx, (cy + p[1]) / 2}, [2]float64{p[0], (cy+p[1])/2 + 40}, [2]float64{p[0], p[1] + 86}
		for t := .02; t <= 1.0001; t += .02 {
			u := 1 - t
			pts = append(pts, [2]float64{u*u*u*a[0] + 3*u*u*t*c1[0] + 3*u*t*t*c2[0] + t*t*t*b[0], u*u*u*a[1] + 3*u*u*t*c1[1] + 3*u*t*t*c2[1] + t*t*t*b[1]})
		}
		acc := 0.0
		for i := 1; i < len(pts); i++ {
			dx, dy := pts[i][0]-pts[i-1][0], pts[i][1]-pts[i-1][1]
			seg := math.Hypot(dx, dy)
			for acc <= seg {
				f := acc / seg
				disc(dst, X(pts[i-1][0]+dx*f), Y(pts[i-1][1]+dy*f), 2.5*k, alpha(hexC("#e8d9a8"), .75))
				acc += 13
			}
			acc -= seg
		}
	}
	node := func(t string, x, y, r, ir float64, hot bool) {
		disc(dst, X(x), Y(y), (r+5)*k, c(0, 0, 0, 64))
		disc(dst, X(x), Y(y), (r+1.5)*k, b2c(hot, webAcc, white))
		disc(dst, X(x), Y(y), (r-1.5)*k, mix(tc(t), c(0, 0, 0, 255), .25))
		soft(dst, X(x-r*.3), Y(y-r*.35), r*.8*k, r*.7*k, alpha(mix(tc(t), white, .45), .9))
		g.drawTypeIcon(dst, t, X(x), Y(y), ir*k, false)
	}
	mx, my := ebiten.CursorPosition()
	pr := g.prof
	var pt []string
	if w.P != nil {
		pt = w.P.Types
	}
	for i, o := range w.Route {
		p := pos[i]
		hot := i == g.lvSel || inR(g.routeRects()[i], float64(mx), float64(my))
		node(o.Type, p[0], p[1], 33, 54, hot)
		drawText(dst, w.LocName(o.Type, o.LI), X(p[0]), Y(p[1]+40), int(math.Round(12.5*k)), true, white, text.AlignCenter, false)
		hz := "Calm"
		var parts []any
		if h := sim.BiomeHazard[o.Type]; h != "" {
			hz = hazardShort[h]
			parts = append(parts, hz)
			km := sim.TypeMul(o.Type, pt)
			switch {
			case has(pt, o.Type):
				parts = append(parts, " · ", hexC("#7dff9a"), "immune")
			case km > 1:
				parts = append(parts, " · ", hexC("#ff8a8a"), "weak!")
			case km < 1:
				parts = append(parts, " · ", hexC("#9fd8ff"), "resist")
			}
		} else {
			parts = append(parts, hz)
		}
		fs := int(math.Round(10 * k))
		hw := richLine(nil, 0, 0, fs, false, hexC("#cfd6e6"), parts...)
		richLine(dst, X(p[0])-hw/2, Y(p[1]+56), fs, false, hexC("#cfd6e6"), parts...)
		var tag []any
		switch {
		case !pr.BiomeUnlocked(o.Type):
			tag = []any{webAcc, gStar, "NEW"}
		case pr.BiomeStars(o.Type) > 0:
			tag = []any{webAcc}
			for s := 0; s < min(3, pr.BiomeStars(o.Type)); s++ {
				tag = append(tag, gStar)
			}
		default:
			tag = []any{hexC("#9aa4bd"), "visited"}
		}
		tw := richLine(nil, 0, 0, fs, false, webFg, tag...)
		richLine(dst, X(p[0])-tw/2, Y(p[1]+69), fs, false, webFg, tag...)
	}
	node(w.Biome, cx, cy, 27, 44, false)
	drawText(dst, "You are here · "+w.LocName(w.Biome, w.LI), X(cx), Y(cy+36), int(math.Round(11*k)), true, hexC("#e8d9a8"), text.AlignCenter, false)
	if w.P != nil {
		fx, fy := X(cx), Y(cy-44)
		soft(dst, fx, fy+13*k, 13*k, 4*k, c(0, 0, 0, 115))
		disc(dst, fx, fy, 15.5*k, tc(w.P.Types[0]))
		disc(dst, fx, fy, 12.5*k, alpha(webInk, .85))
		if s := g.assets.Sprite(w.P.ID); s != nil && s.Front != nil {
			f0 := s.Front.IdleFrame(0)
			im := s.Front.Frame(f0)
			bx := s.Front.Box(f0)
			kk := math.Min(34*k/float64(bx.Dx()), 34*k/float64(bx.Dy()))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(kk, kk)
			op.GeoM.Translate(fx-float64(bx.Dx())*kk/2, fy+10*k-float64(bx.Dy())*kk)
			dst.DrawImage(im.SubImage(bx).(*ebiten.Image), op)
		}
	}
}

// ---- shrine (web shrineUI: the level-up overlay with plain cards) ----

type shrCard struct {
	title string
	col   color.NRGBA
	desc  []string
	curse []string
	w     float64
}

func (g *Game) shrineLayout() ([]shrCard, [][4]float64) {
	w := g.W
	var cs []shrCard
	mk := func(title string, col color.NRGBA, desc, curse string) shrCard {
		mw := math.Max(textW(title, 15, true), textW(desc, 12, false))
		if curse != "" {
			mw = math.Max(mw, textW(curse, 12, false))
		}
		cw := math.Max(150, math.Min(220, mw+30))
		c := shrCard{title: title, col: col, desc: wrap(desc, font(12, false), cw-28), w: cw}
		if curse != "" {
			c.curse = wrap(curse, font(12, false), cw-28)
		}
		return c
	}
	for i, b := range w.Boons {
		col := hexC("#7fe0ff")
		curse := ""
		if b.Curse != "" {
			col, curse = hexC("#d38aff"), "Curse: "+b.Curse
		}
		cs = append(cs, mk(fmt.Sprintf("%d. %s", i+1, b.Name), col, b.Desc, curse))
	}
	cs = append(cs, mk(fmt.Sprintf("%d. Leave it", len(w.Boons)+1), webFg, "Walk away.", ""))
	h := 0.0
	tot := -10.0
	for _, c := range cs {
		h = math.Max(h, 24+19+float64(len(c.desc)+len(c.curse))*15)
		tot += c.w + 10
	}
	x := float64(g.sw)/2 - tot/2
	y := float64(g.sh)/2 - h/2
	var rs [][4]float64
	for _, c := range cs {
		rs = append(rs, [4]float64{x, y, c.w, h})
		x += c.w + 10
	}
	return cs, rs
}

func (g *Game) drawWebShrine(dst *ebiten.Image) {
	sw := float64(g.sw)
	rect(dst, 0, 0, sw, float64(g.sh), c(0, 0, 0, 185))
	cs, rs := g.shrineLayout()
	tw := richLine(nil, 0, 0, 24, true, webFg, gGift, "Ancient Shrine — choose a boon")
	richLine(dst, sw/2-tw/2, rs[0][1]-44, 24, true, webFg, gGift, "Ancient Shrine — choose a boon")
	mx, my := ebiten.CursorPosition()
	for i, c := range cs {
		r := rs[i]
		hot := i == g.lvSel || inR(r, float64(mx), float64(my))
		webBtnDraw(dst, r, hot, false, false)
		th := 19 + float64(len(c.desc)+len(c.curse))*15
		y := r[1] + r[3]/2 - th/2
		drawText(dst, c.title, r[0]+r[2]/2, y, 15, true, c.col, text.AlignCenter, false)
		y += 19
		for _, l := range c.desc {
			drawText(dst, l, r[0]+r[2]/2, y, 12, false, alpha(webFg, .75), text.AlignCenter, false)
			y += 15
		}
		for _, l := range c.curse {
			drawText(dst, l, r[0]+r[2]/2, y, 12, false, hexC("#ff8a8a"), text.AlignCenter, false)
			y += 15
		}
	}
}
