package client

import (
	"fmt"
	"image/color"
	"io/fs"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/meta"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Main menu, shop, line upgrades and run setup rebuilt to match the web version's screens.

// ---- shared pieces ----

// webBtnDraw is the web's plain button: panel background, 2px border, gold border when focused.
func webBtnDraw(dst *ebiten.Image, r [4]float64, hot, on, disabled bool) {
	border, bg := webBorder, webPanel
	if on {
		border = webAcc
	}
	if hot && !disabled {
		border, bg = webAcc, hexC("#2a2a14")
	}
	webBox(dst, r[0], r[1], r[2], r[3], 10, border, bg)
}

// centredPara lays out wrapped, centred text; with dst it draws. Returns the height.
func centredPara(dst *ebiten.Image, s string, cx, y, w float64, size int, bold bool, col color.NRGBA) float64 {
	lh := float64(size) + 4
	ls := wrap(s, font(size, bold), w)
	if dst != nil {
		for i, l := range ls {
			drawText(dst, l, cx, y+float64(i)*lh, size, bold, col, text.AlignCenter, false)
		}
	}
	return float64(len(ls)) * lh
}

// para adds a centred paragraph to the scrolling area at content y; returns its height.
func (g *Game) para(s string, y, w float64, size int, bold bool, col color.NRGBA) float64 {
	sw := float64(g.sw)
	h := centredPara(nil, s, sw/2, 0, w, size, bold, col)
	g.ui.label([4]float64{sw/2 - w/2, y, w, h}, true, func(dst *ebiten.Image, r [4]float64) {
		centredPara(dst, s, r[0]+r[2]/2, r[1], r[2], size, bold, col)
	})
	return h
}

// webCard is a web "button" card in a flowing row (shop items, modifiers, ...).
type webCard struct {
	key      string
	title    string
	titleCol color.NRGBA
	lines    []string
	price    []any // last line, rich (coin glyph + cost, lock + text)
	disabled bool
	on       bool
	minW     float64
	maxW     float64
	act      func()
	extra    []string // wrapped continuation of the last line
}

func (c *webCard) size() (w float64, wrapped []string) {
	mx := textW(c.title, 15, true)
	for _, l := range c.lines {
		mx = math.Max(mx, textW(l, 12, false))
	}
	if len(c.price) > 0 {
		mx = math.Max(mx, richLine(nil, 0, 0, 12, false, webFg, c.price...))
	}
	minW, maxW := c.minW, c.maxW
	if minW == 0 {
		minW = 150
	}
	if maxW == 0 {
		maxW = 220
	}
	w = math.Max(minW, math.Min(maxW, mx+30))
	for _, l := range c.lines {
		wrapped = append(wrapped, wrap(l, font(12, false), w-28)...)
	}
	// a last line too long for the card wraps like the web (glyph + text)
	if len(c.price) > 0 && richLine(nil, 0, 0, 12, false, webFg, c.price...) > w-24 {
		txt := ""
		var gl []any
		for _, p := range c.price {
			switch v := p.(type) {
			case string:
				txt += v
			case glyph:
				gl = append(gl, v)
			}
		}
		ls := wrap(txt, font(12, false), w-28-16)
		if len(ls) > 0 {
			c.price = append(gl, ls[0])
			c.extra = ls[1:]
		}
	}
	return
}

// flowCards lays cards out in centred rows (max row width maxW) at content y; returns the height.
func (g *Game) flowCards(cards []*webCard, y, maxW float64) float64 {
	sw := float64(g.sw)
	type placed struct {
		c   *webCard
		w   float64
		ls  []string
		h   float64
		row int
	}
	var ps []placed
	var rowW []float64
	for _, c := range cards {
		w, ls := c.size()
		h := 12 + 19 + float64(len(ls))*15 + 12
		if len(c.price) > 0 {
			h += 17 + float64(len(c.extra))*15
		}
		if len(rowW) == 0 || rowW[len(rowW)-1]+w+10 > maxW {
			rowW = append(rowW, -10)
		}
		rowW[len(rowW)-1] += w + 10
		ps = append(ps, placed{c, w, ls, h, len(rowW) - 1})
	}
	// equal heights per row (web flex rows stretch)
	rowH := make([]float64, len(rowW))
	for _, p := range ps {
		rowH[p.row] = math.Max(rowH[p.row], p.h)
	}
	cy := y
	x := 0.0
	cur := -1
	for _, p := range ps {
		if p.row != cur {
			if cur >= 0 {
				cy += rowH[cur] + 10
			}
			cur = p.row
			x = sw/2 - rowW[cur]/2
		}
		p := p
		r := [4]float64{x, cy, p.w, rowH[p.row]}
		g.ui.button(p.c.key, r, true, p.c.disabled, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			webBtnDraw(dst, rr, hot, p.c.on, p.c.disabled)
			a := 1.0
			if p.c.disabled {
				a = .5
			}
			n := 1 + len(p.ls)
			if len(p.c.price) > 0 {
				n++
			}
			th := 19 + float64(len(p.ls))*15
			if len(p.c.price) > 0 {
				th += 17 + float64(len(p.c.extra))*15
			}
			ty := rr[1] + rr[3]/2 - th/2
			tcol := p.c.titleCol
			if tcol.A == 0 {
				tcol = webFg
			}
			drawText(dst, p.c.title, rr[0]+rr[2]/2, ty, 15, true, alpha(tcol, a), text.AlignCenter, false)
			ty += 19
			for _, l := range p.ls {
				drawText(dst, l, rr[0]+rr[2]/2, ty, 12, false, alpha(webFg, .75*a), text.AlignCenter, false)
				ty += 15
			}
			if len(p.c.price) > 0 {
				w := richLine(nil, 0, 0, 12, false, webFg, p.c.price...)
				richLine(dst, rr[0]+rr[2]/2-w/2, ty+2, 12, false, alpha(webFg, .75*a), p.c.price...)
				for i, l := range p.c.extra {
					drawText(dst, l, rr[0]+rr[2]/2, ty+2+float64(i+1)*15, 12, false, alpha(webFg, .75*a), text.AlignCenter, false)
				}
			}
			_ = n
		}, p.c.act)
		x += p.w + 10
	}
	if cur >= 0 {
		cy += rowH[cur]
	}
	return cy - y
}

// h3 adds a web section heading at content y; returns its height.
func (g *Game) h3(s string, y float64) float64 {
	sw := float64(g.sw)
	g.ui.label([4]float64{0, y, sw, 20}, true, textDraw(s, 15, true, alpha(webFg, .85), text.AlignCenter))
	return 30
}

// webHeaderParts is the header row with arbitrary rich content on the right.
func (g *Game) webHeaderParts(title, sub string, back screen, right []any) {
	sw := float64(g.sw)
	u := &g.ui
	u.label([4]float64{0, 18, sw, 36}, false, textDraw(title, 26, true, webFg, text.AlignCenter))
	if sub != "" {
		u.label([4]float64{0, 56, sw, 18}, false, textDraw(sub, 13, false, alpha(webFg, .85), text.AlignCenter))
	}
	if len(right) > 0 {
		u.label([4]float64{sw - 300, 22, 280, 24}, false, func(dst *ebiten.Image, r [4]float64) {
			w := richLine(nil, 0, 0, 16, false, webFg, right...)
			richLine(dst, r[0]+r[2]-w, r[1], 16, false, webFg, right...)
		})
	}
	g.backButton(back)
}

func (g *Game) coinsRight() []any {
	return []any{gCoin, true, commas(g.prof.S.Coins())}
}

// ---- logo (the web version's pixel-art SVG, drawn from its rectangles) ----

var logoImg *ebiten.Image

var rectRe = regexp.MustCompile(`<rect x="([\d.]+)" y="([\d.]+)" width="([\d.]+)" height="([\d.]+)" fill="(#[0-9a-fA-F]{6})"/>`)

func (g *Game) logo() *ebiten.Image {
	if logoImg != nil {
		return logoImg
	}
	b, err := readUI("logo.svg")
	if err != nil {
		return nil
	}
	im := ebiten.NewImage(512, 182)
	for _, m := range rectRe.FindAllStringSubmatch(string(b), -1) {
		f := func(i int) float64 { v, _ := strconv.ParseFloat(m[i], 64); return v }
		rect(im, f(1), f(2), f(3), f(4), hexC(m[5]))
	}
	logoImg = im
	return im
}

// ---- main menu ----

// runLabel is the web version's one-line run summary.
func (g *Game) runLabel() string {
	pr := g.prof
	rs := pr.S.RunSetup()
	bi, _ := rs["bi"].(string)
	pre := "Random location"
	if bi != "" && bi != "random" && pr.BiomeUnlocked(bi) {
		pre = g.locName(bi, int(num(rs["li"])))
	}
	sg := g.stage()
	pre += fmt.Sprintf(" · Tier %d (%d min) · ", sg, 15*sg)
	mods := g.mods()
	if g.tier() > 0 || len(mods) > 0 {
		return pre + fmt.Sprintf("Difficulty %d · %d modifier%s · rewards ×%.2f", g.tier(), len(mods), plural(len(mods)), g.rewardMult())
	}
	return pre + "standard difficulty"
}

func (g *Game) rewardMult() float64 {
	mult := 1 + .12*float64(g.tier())
	mods := g.mods()
	for _, m := range meta.Mods {
		if contains(mods, m.Key) {
			mult += m.Bonus
		}
	}
	return mult
}

func (g *Game) menuMain() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	if g.csel == "" || g.db.ByID[g.csel] == nil || !pr.Unl(g.csel) {
		g.csel = "fire"
	}
	u.area = [4]float64{0, 0, sw, sh}
	y := 14.0
	// logo
	lw := math.Min(440, sw*.88)
	lh := lw * 182 / 512
	u.label([4]float64{sw/2 - lw/2, y, lw, lh}, true, func(dst *ebiten.Image, r [4]float64) {
		if im := g.logo(); im != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(r[2]/512, r[3]/182)
			op.GeoM.Translate(r[0], r[1])
			dst.DrawImage(im, op)
		}
	})
	y += lh + 14
	// coins and tickets
	cur := []any{gCoin, webAcc, true, commas(pr.S.Coins()), webFg, false, "     ", gTicket, true, strconv.Itoa(pr.S.Tickets())}
	u.label([4]float64{0, y, sw, 24}, true, func(dst *ebiten.Image, r [4]float64) {
		w := richLine(nil, 0, 0, 18, false, webFg, cur...)
		richLine(dst, sw/2-w/2, r[1], 18, false, webFg, cur...)
	})
	y += 34
	// selected character card
	p := g.db.ByID[g.csel]
	tys := ""
	for i, t := range p.Types {
		if i > 0 {
			tys += "/"
		}
		tys += cap1s(t)
	}
	l1 := tys + " type · starts with " + g.db.Moves[g.db.StartMove(g.csel)].Name
	l2 := typeFlavor[p.Types[0]]
	cw := math.Max(220, math.Max(textW(l1, 13, false), textW(l2, 13, false)+18)+36)
	ch := 180.0
	u.button("selcard", [4]float64{sw/2 - cw/2, y, cw, ch}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
		border := webBorder
		if hot {
			border = mix(webBorder, webAcc, .6)
		}
		webBox(dst, r[0], r[1], r[2], r[3], 12, border, webPanel)
		if s := g.assets.Sprite(g.csel); s != nil && s.Front != nil {
			fr := s.Front.IdleFrame(g.tick * 1000 / 60)
			im := s.Front.Frame(fr)
			f0 := s.Front.IdleFrame(0)
			bx := s.Front.Box(f0).Sub(s.Front.Frame(f0).Bounds().Min).Add(im.Bounds().Min)
			k := math.Min(84/float64(bx.Dx()), 84/float64(bx.Dy()))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(r[0]+r[2]/2-float64(bx.Dx())*k/2, r[1]+8+84-float64(bx.Dy())*k)
			dst.DrawImage(im.SubImage(bx).(*ebiten.Image), op)
		}
		cx := r[0] + r[2]/2
		drawText(dst, "Selected character", cx, r[1]+96, 13, false, alpha(webFg, .85), text.AlignCenter, false)
		drawText(dst, p.Name, cx, r[1]+115, 18, true, tc(p.Types[0]), text.AlignCenter, false)
		drawText(dst, l1, cx, r[1]+139, 13, false, alpha(webFg, .85), text.AlignCenter, false)
		w := richLine(nil, 0, 0, 13, false, webFg, gSparkle, l2)
		richLine(dst, cx-w/2, r[1]+157, 13, false, alpha(webFg, .85), gSparkle, l2)
	}, func() { g.scr = scChars; g.ui.focus = ""; g.ui.scroll = 0 })
	y += ch + 12
	// menu: two columns like the web version
	ready := pr.ReadyChallenges()
	chal := "Challenges"
	if ready > 0 {
		chal = fmt.Sprintf("Challenges (%d ready)", ready)
	}
	ball := uiIcon(fmt.Sprintf("ball%d.png", g.menuBall))
	type item struct {
		key   string
		label string
		icon  any
		to    screen
	}
	items := []item{
		{"play", "Play now", gPlay, scPlay},
		{"setup", "Run setup", gGear, scSetup},
		{"chars", "Pokémon", ball, scChars},
		{"shop", "Shop", gCart, scShop},
		{"coll", "Collection", gBook, scColl},
		{"hist", "Run history", gBars, scHist},
		{"life", "Lifetime stats", gChart, scLife},
		{"chal", chal, gTrophy, scChal},
		{"dex", "Pokédex", uiIcon("dex.png"), scDex},
		{"daily", "Daily Run", gCalendar, scDaily},
		{"boards", "Leaderboards", gMedal, scBoards},
		{"settings", "Settings", gGear, scSettings},
		{"coop", "Local co-op", gPad, scLobby},
		{"online", "Online co-op", gGlobe, scOnline},
	}
	mw := math.Min(460, sw*.94)
	bw := (mw - 8) / 2
	for i, it := range items {
		it := it
		r := [4]float64{sw/2 - mw/2 + float64(i%2)*(bw+8), y + float64(i/2)*48, bw, 40}
		u.button(it.key, r, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			webBtnDraw(dst, rr, hot, false, false)
			col := webFg
			if it.key == "chal" && ready > 0 {
				col = hexC("#ffd04a")
			}
			parts := []any{white, it.icon, col, true, it.label}
			w := richLine(nil, 0, 0, 15, true, col, parts...)
			richLine(dst, rr[0]+rr[2]/2-w/2, rr[1]+rr[3]/2-9, 15, true, col, parts...)
		}, func() {
			g.settingsFrom = scMenu
			switch it.to {
			case scPlay:
				g.startFromMenu()
				return
			case scLobby:
				g.seats = nil
			case scOnline:
				if g.on.room == nil {
					g.on.mode = "menu"
				}
			case scBoards:
				g.boardsFrom = scMenu
				g.lbFetch()
			}
			g.scr = it.to
			g.ui.focus = ""
			g.ui.scroll = 0
		})
	}
	y += float64((len(items)+1)/2)*48 + 4
	y += g.para(g.runLabel(), y, 560, 12, false, alpha(webFg, .8)) + 10
	y += g.para("Move with WASD / arrows (or a gamepad, or drag on touch). Attacks fire automatically. Pick moves on level-up, exploit type matchups, survive 15:00 and beat the final boss to start endless mode. Swarms hit every 2 minutes.", y, 480, 14, false, alpha(webFg, .9)) + 10
	if g.cloud.Available() {
		y += g.para(g.cloud.Status(), y, 560, 12, false, alpha(webFg, .8)) + 8
	} else {
		y += g.para("Saved to "+meta.SavePath(), y, 560, 12, false, alpha(webFg, .7)) + 8
	}
	// small buttons (web: save codes; plus sign-in where the build has it)
	type sb struct {
		key, label string
		icon       any
		act        func()
	}
	rl := "Reset progress"
	if g.rstArm {
		rl = "Click again to confirm"
	}
	sbs := []sb{
		{"reset", rl, nil, func() {
			if !g.rstArm {
				g.rstArm = true
				return
			}
			g.rstArm = false
			g.prof = meta.NewProfile(meta.NewSave(), g.db)
			g.csel = "fire"
			g.save()
			g.toast("Progress reset")
		}},
		{"xferc", "Copy save code", gUpload, func() { g.scr = scXfer; g.ui.focus = "" }},
		{"xferl", "Load save code", gDownload, func() { g.scr = scXfer; g.ui.focus = "" }},
	}
	if g.cloud.Available() {
		if g.cloud.Online() {
			sbs = append(sbs, sb{"acct", "Sign out", gGlobe, func() { g.cloud.SignOut(); g.toast("Signed out") }})
		} else {
			sbs = append(sbs, sb{"acct", "Sign in with Google", gGlobe, func() { g.cloud.SignIn() }})
		}
	}
	parts := func(b sb) []any {
		if b.icon == nil {
			return nil
		}
		return []any{b.icon, " "}
	}
	tot := -8.0
	for _, b := range sbs {
		tot += smallBtnW(b.label, parts(b)...) + 8
	}
	x := sw/2 - tot/2
	for _, b := range sbs {
		b := b
		w := smallBtnW(b.label, parts(b)...)
		u.button(b.key, [4]float64{x, y, w, 30}, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			smallBtn(dst, rr, b.label, hot, false, parts(b)...)
		}, b.act)
		x += w + 8
	}
	y += 30 + 16
	u.scrollMax = math.Max(0, y-sh)
}

// ---- shop ----

func (g *Game) upCard(k meta.Upgrade, fam string) *webCard {
	pr := g.prof
	l := pr.UL(k.Key, fam)
	c := &webCard{key: "u:" + k.Key, title: k.Name}
	if k.Max == 1 {
		c.lines = []string{k.Desc}
	} else {
		c.lines = []string{fmt.Sprintf("Lv %d/%d · %s", l, k.Max, k.Desc)}
	}
	cost := pr.UpCost(k.Key, fam)
	mx := l >= k.Max
	lk := k.Lock != nil && l == 0 && pr.S.Stat(k.Lock.Stat) < k.Lock.Need
	rq := k.Req != "" && pr.UL(k.Req, fam) == 0
	switch {
	case mx:
		c.price = []any{b2s(k.Max == 1, "OWNED", "MAX")}
	case lk:
		have := math.Min(k.Lock.Need, pr.S.Stat(k.Lock.Stat))
		prog := commas(int(have)) + "/" + commas(int(k.Lock.Need))
		if k.Lock.Stat == "t" {
			prog = fmt.Sprintf("%d/%d min", int(have/60), int(k.Lock.Need/60))
		}
		c.lines = append(c.lines, "")
		c.lines[len(c.lines)-1] = k.Lock.Text + " (" + prog + ")"
	case rq:
		c.price = []any{"Needs " + upName(k.Req)}
	default:
		c.price = []any{gCoin, strconv.Itoa(cost)}
	}
	c.disabled = mx || lk || rq || pr.S.Coins() < cost
	c.act = func() {
		if pr.BuyUpgrade(k.Key, fam) {
			g.save()
		}
	}
	return c
}

func upName(key string) string {
	for _, u := range meta.Upgrades {
		if u.Key == key {
			return u.Name
		}
	}
	return key
}

func (g *Game) menuShop() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	g.webHeaderParts("Shop", "", scMenu, g.coinsRight())
	u.area = [4]float64{0, 60, sw, sh - 60}
	y := 4.0
	y += g.para("Loadout slots and level-up tools are shared by every Pokémon. Stat upgrades and Training are bought per evolution line: tap the arrow on a card in the Pokémon screen.", y, 480, 14, false, alpha(webFg, .9)) + 14
	byKey := func(keys ...string) []*webCard {
		var out []*webCard
		for _, k := range keys {
			for _, up := range meta.Upgrades {
				if up.Key == k {
					c := g.upCard(up, "")
					if up.Lock != nil && strings.HasPrefix(c.lines[len(c.lines)-1], up.Lock.Text) {
						c.price = []any{gLock, c.lines[len(c.lines)-1]}
						c.lines = c.lines[:len(c.lines)-1]
					}
					out = append(out, c)
				}
			}
		}
		return out
	}
	y += g.h3("Loadout slots (all Pokémon)", y)
	y += g.flowCards(byKey("m3", "m4", "h3", "h4"), y, 900) + 18
	y += g.h3("Level-up tools (all Pokémon)", y)
	y += g.flowCards(byKey("rr", "bn", "ch", "rt"), y, 900) + 18
	u.scrollMax = math.Max(0, y-u.area[3])
}

// ---- line upgrades (web "Upgrade Shop") ----

func (g *Game) menuLine() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	fam := g.lineFam
	back := g.lineFrom
	if back == 0 {
		back = scChars
	}
	g.webHeaderParts("", "", back, g.coinsRight())
	u.label([4]float64{0, 18, sw, 36}, false, func(dst *ebiten.Image, r [4]float64) {
		parts := []any{uiIcon("candy.png"), " Upgrade Shop ", uiIcon("candy.png")}
		w := richLine(nil, 0, 0, 26, true, webFg, parts...)
		richLine(dst, sw/2-w/2, r[1], 26, true, webFg, parts...)
	})
	name := g.db.ByID[fam].Name
	u.label([4]float64{0, 56, sw, 18}, false, textDraw(name+" line · shared by every evolution and form in this line.", 13, false, alpha(webFg, .85), text.AlignCenter))
	u.area = [4]float64{0, 80, sw, sh - 80}
	y := 4.0
	var mem []string
	for _, p := range g.db.Pokemon {
		if pr.Fam(p.ID) == fam && pr.Unl(p.ID) {
			mem = append(mem, p.ID)
		}
	}
	mw := float64(len(mem))*72 - 8
	for i, m := range mem {
		m := m
		x := sw/2 - mw/2 + float64(i)*72
		u.label([4]float64{x, y, 64, 64}, true, func(dst *ebiten.Image, r [4]float64) {
			g.drawPortrait(dst, m, r[0], r[1], 64, false, false, true)
		})
	}
	y += 64 + 16
	y += g.h3("Stats", y)
	var cards []*webCard
	for _, k := range meta.Upgrades {
		if !k.Global {
			cards = append(cards, g.upCard(k, fam))
		}
	}
	y += g.flowCards(cards, y, 1180) + 18
	y += g.h3("Training — a late-game coin sink", y)
	r := pr.Training(fam)
	cost := meta.TrainingCost(r)
	mx := r >= meta.TrainingMax
	tc := &webCard{key: "train", title: fmt.Sprintf("Training Rank %d/%d", r, meta.TrainingMax), lines: []string{fmt.Sprintf("+2%% damage and +1%% coins per rank (now +%d%% / +%d%%)", 2*r, r)}, minW: 200}
	if mx {
		tc.price = []any{"MAX"}
	} else {
		tc.price = []any{gCoin, strconv.Itoa(cost)}
	}
	tc.disabled = mx || pr.S.Coins() < cost
	tc.act = func() {
		if pr.BuyTraining(fam) {
			g.save()
		}
	}
	y += g.flowCards([]*webCard{tc}, y, 900) + 18
	u.scrollMax = math.Max(0, y-u.area[3])
}

// ---- run setup ----

func (g *Game) menuSetup() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	rs := pr.S.RunSetup()
	g.webHeaderParts("Run setup", "", scMenu, []any{"Rewards ×", true, fmt.Sprintf("%.2f", g.rewardMult())})
	u.area = [4]float64{0, 60, sw, sh - 60}
	cur, _ := rs["bi"].(string)
	if cur == "" || (cur != "random" && !pr.BiomeUnlocked(cur)) {
		cur = "random"
	}
	if g.bsel == "" {
		g.bsel = cur
	}
	li := int(num(rs["li"]))
	y := 2.0
	y += g.h3("Biome", y)
	note := "Random: you start in a random location. "
	if cur != "random" {
		hz := "No hazards here. "
		if h := sim.BiomeHazard[cur]; h != "" {
			hz = sim.HazardText[h] + " "
		}
		note = g.locName(cur, li) + " (" + cap1s(cur) + "): " + hz
	}
	note += "Each type has 3 locations that share unlocks and tier stars. Tap a type to see its locations. After every boss you pick your next location on the route map (unless Stay in biome is on)."
	y += g.para(note, y, 480, 14, false, alpha(webFg, .9)) + 10
	// type grid (web .tgrid: 8 per row at this width)
	opts := append([]string{"random"}, typeOrder...)
	cols := 8
	cw, chh := 64.0, 62.0
	gw := float64(cols)*(cw+7) - 7
	for i, t := range opts {
		t := t
		r := [4]float64{sw/2 - gw/2 + float64(i%cols)*(cw+7), y + float64(i/cols)*(chh+7), cw, chh}
		unl := t == "random" || pr.BiomeUnlocked(t)
		on := t == cur
		open := t == g.bsel
		u.button("b:"+t, r, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			border, bg := webBorder, webPanel
			if open {
				bg = hexC("#2a3350")
			}
			if on {
				soft(dst, rr[0]+rr[2]/2, rr[1]+rr[3]/2, rr[2]*.75, rr[3]*.75, alpha(webAcc, .35))
				border = webAcc
			} else if hot {
				border = mix(webBorder, webAcc, .5)
			}
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, border, bg)
			cx := rr[0] + rr[2]/2
			if t == "random" {
				drawGlyph(dst, gDice, cx, rr[1]+22, 24, white)
				drawText(dst, "Random", cx, rr[1]+41, 10, false, alpha(webFg, .85), text.AlignCenter, false)
				return
			}
			g.drawTypeIcon(dst, t, cx, rr[1]+22, 30, !unl)
			drawText(dst, cap1s(t), cx, rr[1]+41, 10, false, alpha(webFg, .85), text.AlignCenter, false)
			if !unl {
				drawGlyph(dst, gLock, rr[0]+rr[2]-9, rr[1]+9, 11, alpha(webFg, .85))
			} else if s := pr.BiomeStars(t); s > 0 {
				for k := 0; k < min(3, s); k++ {
					drawGlyph(dst, gStar, rr[0]+7+float64(k)*8, rr[1]+7, 8, webAcc)
				}
			}
		}, func() {
			g.bsel = t
			if t == "random" {
				rs["bi"] = "random"
				g.save()
			}
		})
	}
	y += float64((len(opts)+cols-1)/cols)*(chh+7) + 6
	// locations of the opened type
	if g.bsel != "" && g.bsel != "random" {
		t := g.bsel
		unl := pr.BiomeUnlocked(t)
		var cards []*webCard
		for i, nm := range g.db.World.Locations[t] {
			i := i
			c := &webCard{key: fmt.Sprintf("l:%s:%d", t, i), title: nm, titleCol: tc(t), minW: 180, maxW: 260, disabled: !unl, on: cur == t && li == i}
			if unl {
				c.price = []any{fmt.Sprintf("Tier %d/3 cleared", pr.BiomeStars(t))}
			} else {
				c.price = []any{gLock, "Beat a boss in any " + cap1s(t) + " location"}
			}
			c.act = func() {
				rs["bi"] = t
				rs["li"] = float64(i)
				g.save()
			}
			cards = append(cards, c)
		}
		y += g.flowCards(cards, y, 900) + 10
	}
	y += 12
	// tier and stay-in-biome
	y += g.h3("Tier (run length)", y)
	smax := pr.StageMax(cur)
	var tcards []*webCard
	for k := 1; k <= 3; k++ {
		k := k
		lk := k > smax
		c := &webCard{key: fmt.Sprintf("sg%d", k), title: fmt.Sprintf("Tier %d", k), lines: []string{fmt.Sprintf("%d minutes · %d bosses", 15*k, 3*k)}, minW: 140, disabled: lk, on: g.stage() == k}
		if lk {
			c.title = "Tier " + strconv.Itoa(k) + " (locked)"
		}
		c.act = func() { rs["sg"] = float64(k); g.save() }
		tcards = append(tcards, c)
	}
	lock, _ := rs["lock"].(bool)
	tcards = append(tcards, &webCard{key: "lock", title: b2s(lock, "Stay in biome: ON", "Stay in biome: OFF"), lines: []string{"Don't switch after bosses"}, minW: 150, on: lock, act: func() { rs["lock"] = !lock; g.save() }})
	y += g.flowCards(tcards, y, 900) + 16
	// difficulty
	y += g.h3("Difficulty", y)
	asc := pr.Asc()
	tn := "Difficulty 0 is the standard game. Win a run (beat the final boss) to unlock Difficulty 1. Every difficulty level makes enemies tougher and pays more."
	if t := g.tier(); t > 0 {
		k := float64(t) // web tierInfo
		tn = fmt.Sprintf("Difficulty %d: enemies +%d%% HP, +%d%% damage, +%d%% speed, %d%% more spawns. Win a run on your highest difficulty to unlock the next one.", t, int(math.Round(12*k)), int(math.Round(8*k)), int(math.Round(2*k)), int(math.Round(5*k)))
	}
	y += g.para(tn, y, 480, 14, false, alpha(webFg, .9)) + 10
	// difficulty chips in centred rows
	var chips []struct {
		k int
		w float64
	}
	for k := 0; k <= meta.TierMax; k++ {
		w := textW(fmt.Sprintf("Difficulty %d", k), 13, false) + 24
		if k > asc {
			w += 18
		}
		chips = append(chips, struct {
			k int
			w float64
		}{k, w})
	}
	var rows [][]int
	var rw []float64
	for i, c := range chips {
		if len(rows) == 0 || rw[len(rw)-1]+c.w+6 > 640 {
			rows = append(rows, nil)
			rw = append(rw, -6)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], i)
		rw[len(rw)-1] += c.w + 6
	}
	for ri, row := range rows {
		x := sw/2 - rw[ri]/2
		for _, i := range row {
			c := chips[i]
			k := c.k
			lk := k > asc
			on := g.tier() == k
			u.button(fmt.Sprintf("t%d", k), [4]float64{x, y, c.w, 30}, true, lk, func(dst *ebiten.Image, rr [4]float64, hot bool) {
				chipBtn(dst, rr, on, hot && !lk, func(x, yy float64) {
					a := b2f(lk, .5, 1)
					parts := []any{}
					if lk {
						parts = append(parts, gLock)
					}
					parts = append(parts, fmt.Sprintf("Difficulty %d", k))
					richLine(dst, x, yy-8, 13, false, alpha(webFg, a), parts...)
				})
			}, func() { rs["tier"] = float64(k); g.save() })
			x += c.w + 6
		}
		y += 36
	}
	y += 14
	// modifiers
	y += g.h3("Modifiers (optional, each boosts coin rewards)", y)
	mods := g.mods()
	var mc []*webCard
	for _, m := range meta.Mods {
		m := m
		on := contains(mods, m.Key)
		c := &webCard{key: "m:" + m.Key, title: m.Name, lines: []string{m.Desc}, minW: 150, maxW: 240, on: on}
		if on {
			c.price = []any{gCheck, fmt.Sprintf("ON · +%d%% rewards", int(math.Round(m.Bonus*100)))}
		} else {
			c.price = []any{fmt.Sprintf("+%d%% rewards", int(math.Round(m.Bonus*100)))}
		}
		c.act = func() {
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
		}
		mc = append(mc, c)
	}
	y += g.flowCards(mc, y, 1200) + 20
	u.scrollMax = math.Max(0, y-u.area[3])
}

func readUI(name string) ([]byte, error) { return fs.ReadFile(uiFS, "ui/"+name) }

// ---- challenges (web chal()) ----

var chalCats = []struct {
	k, name string
	gl      glyph
}{
	{"k", "Defeat enemies", gSwords}, {"b", "Defeat bosses", gTrophy}, {"t", "Survive in a single run", gSparkle}, {"w", "Win runs", gTrophy},
	{"a", "Conquer difficulty tiers", gStar}, {"mv", "Discover moves", gBook}, {"ev", "Evolve moves", gSparkle}, {"cp", "Capstone moves", gMedal},
	{"el", "Defeat elites & mini-bosses", gSwords}, {"bi", "Unlock biomes", gGlobe}, {"bs", "Clear biome tiers", gTriUp}, {"lc", "Explore locations", gPin},
	{"ft", "Full tours", gPin}, {"rt", "Road trips", gPin}, {"cb", "Co-op bosses", gPad}, {"cw", "Co-op wins", gTrophy}, {"rv", "Pokémon Center revives", gHand},
	{"fp", "Full party", gPad}, {"mvp", "Co-op MVP", gMedal}, {"ha", "Hidden Abilities", gSparkle}, {"lg", "Legendary abilities", gStar},
	{"pk", "Pokémon collection", gBook}, {"fe", "Final forms", gStar}, {"al", "Regional forms", gSparkle}, {"mb", "Mini-bosses", gSwords},
	{"td", "Lucky finds", gTicket}, {"cc", "Cursed chests", gLock}, {"pc", "Shrine curses", gGift}, {"nr", "No regrets", gShield},
	{"lgc", "Legendary collection", gStar}, {"sh", "Shiny collection", gSparkle},
}

func (g *Game) menuChal() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	st := pr.S
	g.webHeaderParts("Challenges", "", scMenu, g.coinsRight())
	u.area = [4]float64{0, 60, sw, sh - 60}
	y := 4.0
	y += g.para(fmt.Sprintf("Defeated %s · Bosses %d · Best run %s · Wins %d · Runs %d", commas(int(st.Stat("k"))), int(st.Stat("b")), fmtT(st.Stat("t")), int(st.Stat("w")), int(st.Stat("r"))), y, 700, 14, false, alpha(webFg, .9)) + 8
	y += g.para("Progress is saved when a run ends. Claim rewards here.", y, 700, 14, false, alpha(webFg, .9)) + 8
	all := pr.Challenges()
	ready := 0
	for _, a := range all {
		if pr.Claimable(a) {
			ready++
		}
	}
	if ready > 0 {
		s := fmt.Sprintf("%d reward%s ready to claim", ready, plural(ready))
		u.label([4]float64{0, y, sw, 20}, true, func(dst *ebiten.Image, r [4]float64) {
			w := richLine(nil, 0, 0, 14, true, webAcc, gGift, s)
			richLine(dst, sw/2-w/2, r[1], 14, true, webAcc, gGift, s)
		})
		y += 28
	}
	rw := math.Min(560, sw*.94)
	row := func(a meta.Ach) {
		v := math.Min(a.Target, st.Stat(a.Stat))
		cl := pr.Claimed(a)
		ok := pr.Claimable(a)
		lab := commas(int(v)) + " / " + commas(int(a.Target))
		if a.Stat == "t" {
			lab = fmtT(v) + " / " + fmtT(a.Target)
		}
		var btn []any
		var bcol color.NRGBA
		switch {
		case cl:
			btn = []any{gCheck, "Claimed"}
		case ok && a.Tickets > 0:
			btn, bcol = []any{"Claim ", gTicket, strconv.Itoa(a.Tickets)}, hexC("#ff9de8")
		case ok:
			btn, bcol = []any{"Claim ", gCoin, strconv.Itoa(a.Coins)}, webAcc
		case a.Tickets > 0:
			btn = []any{gTicket, strconv.Itoa(a.Tickets)}
		default:
			btn = []any{gCoin, strconv.Itoa(a.Coins)}
		}
		bw := richLine(nil, 0, 0, 12, false, webFg, btn...) + 22
		r := [4]float64{sw/2 - rw/2, y, rw, 64}
		u.button("a:"+a.ID, r, true, !ok, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, b2c(hot && ok, webAcc, webBorder), webPanel)
			am := b2f(cl, .55, 1)
			drawText(dst, a.Name, rr[0]+14, rr[1]+10, 16, true, alpha(webFg, am), text.AlignStart, false)
			drawText(dst, lab, rr[0]+14, rr[1]+30, 12, false, alpha(webFg, .75*am), text.AlignStart, false)
			pbar(dst, rr[0]+14, rr[1]+48, rr[2]-28-bw-12, 6, v/a.Target, webCyan)
			br := [4]float64{rr[0] + rr[2] - 14 - bw, rr[1] + rr[3]/2 - 15, bw, 30}
			border, a2 := webBorder, .6
			if ok {
				border, a2 = bcol, 1
			}
			webBox(dst, br[0], br[1], br[2], br[3], 8, border, webPanel)
			w := richLine(nil, 0, 0, 12, false, webFg, btn...)
			richLine(dst, br[0]+br[2]/2-w/2, br[1]+7, 12, false, alpha(webFg, a2*am), btn...)
		}, func() {
			if pr.Claim(a) {
				g.save()
			}
		})
		y += 72
	}
	open := 0
	for _, c := range chalCats {
		var cat, act []meta.Ach
		for _, a := range all {
			if a.Stat == c.k {
				cat = append(cat, a)
				if !pr.Claimed(a) {
					act = append(act, a)
				}
			}
		}
		if len(act) == 0 {
			continue
		}
		open += len(act)
		h := fmt.Sprintf("%s (%d/%d)", c.name, len(cat)-len(act), len(cat))
		gl := c.gl
		u.label([4]float64{0, y, sw, 20}, true, func(dst *ebiten.Image, r [4]float64) {
			w := richLine(nil, 0, 0, 15, true, webFg, gl, h)
			richLine(dst, sw/2-w/2, r[1], 15, true, alpha(webFg, .85), gl, h)
		})
		y += 28
		for _, a := range act {
			row(a)
		}
	}
	if open == 0 {
		y += g.para("Every challenge is complete!", y, 600, 14, false, webFg) + 10
	}
	done := 0
	for _, a := range all {
		if pr.Claimed(a) {
			done++
		}
	}
	if done > 0 {
		r := [4]float64{sw/2 - rw/2, y, rw, 40}
		lbl := fmt.Sprintf("%d / %d challenges", done, len(all))
		u.button("chdone", r, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			webBtnDraw(dst, rr, hot, false, false)
			poly(dst, b2a(g.chalOpen, [][2]float64{{rr[0] + 14, rr[1] + 16}, {rr[0] + 24, rr[1] + 16}, {rr[0] + 19, rr[1] + 23}}, [][2]float64{{rr[0] + 15, rr[1] + 14}, {rr[0] + 22, rr[1] + 20}, {rr[0] + 15, rr[1] + 26}}), webFg)
			richLine(dst, rr[0]+32, rr[1]+11, 14, true, webFg, gCheck, "Completed", false, alpha(webFg, .75), "  "+lbl)
		}, func() { g.chalOpen = !g.chalOpen })
		y += 48
		if g.chalOpen {
			for _, c := range chalCats {
				var dn []meta.Ach
				for _, a := range all {
					if a.Stat == c.k && pr.Claimed(a) {
						dn = append(dn, a)
					}
				}
				if len(dn) == 0 {
					continue
				}
				gl, nm := c.gl, c.name
				u.label([4]float64{0, y, sw, 20}, true, func(dst *ebiten.Image, r [4]float64) {
					w := richLine(nil, 0, 0, 15, true, webFg, gl, nm)
					richLine(dst, sw/2-w/2, r[1], 15, true, alpha(webFg, .85), gl, nm)
				})
				y += 28
				for _, a := range dn {
					row(a)
				}
			}
		}
	}
	u.scrollMax = math.Max(0, y+10-u.area[3])
}

func b2a(b bool, t, f [][2]float64) [][2]float64 {
	if b {
		return t
	}
	return f
}
