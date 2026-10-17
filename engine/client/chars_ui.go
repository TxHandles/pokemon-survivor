package client

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/meta"
)

// Character select, rebuilt to match the web version (chars()): search, type filter chips, one card
// per evolution line with tabs for its unlocked members, mastery, matchups, stats, the type effect,
// ability cards with the hidden-ability progress and the line-upgrades button.

const ccW = 290.0 // card width incl. border (web: 262 + 2×12 padding + 2×2 border)

var typeFlavor = map[string]string{"fire": "Burn: 20% chance to ignite (damage over time)", "water": "Soak: hits slow enemies",
	"grass": "Entangle: 12% chance to root enemies for 1s", "electric": "Shock: 12% chance to stun", "ground": "Mud: hits slow enemies",
	"rock": "Heavy hits: extra knockback", "flying": "Gust: extra knockback · Flies over obstacles",
	"ice": "Freeze: 10% chance to freeze; thawed enemies are slowed", "poison": "Toxin: 25% chance to poison (damage over time)",
	"psychic": "Seeking projectiles", "dark": "Fear: 8% chance enemies flee for 1s", "bug": "Sticky: hits slow enemies",
	"fairy": "Charm: 10% chance enemies attack allies for 1.2s (not bosses)", "dragon": "Burn: 15% chance to ignite",
	"steel": "Heavy hits: extra knockback", "normal": "No special effect", "ghost": "Phantom: +2 chain jumps · Phases through obstacles",
	"fighting": "Brawler: big knockback"}

// dexNo is the Pokédex number as text (web dexNo).
func (g *Game) dexNo(id string) string {
	if p := g.db.ByID[id]; p != nil && p.Dex > 0 {
		return fmt.Sprintf("%03d", p.Dex)
	}
	for i, p := range g.db.Pokemon {
		if p.ID == id {
			return fmt.Sprintf("%03d", i+1)
		}
	}
	return "000"
}

// srchOK is the web search rule: a number matches the Dex number, text matches unlocked names,
// type names and "legendary".
func (g *Game) srchOK(id, q string) bool {
	q = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(q)), "#")
	if q == "" {
		return true
	}
	p := g.db.ByID[id]
	no := g.dexNo(id)
	if _, err := strconv.Atoi(q); err == nil {
		pad := q
		for len(pad) < min(3, len(q)) {
			pad = "0" + pad
		}
		n, _ := strconv.Atoi(no)
		return strings.Contains(no, pad) || strings.HasPrefix(strconv.Itoa(n), q)
	}
	if g.prof.Unl(id) && strings.Contains(strings.ToLower(p.Name), q) {
		return true
	}
	for _, t := range p.Types {
		if strings.HasPrefix(t, q) {
			return true
		}
	}
	return p.Legendary && strings.HasPrefix("legendary", q)
}

// lineCard is one card on the select screen.
type lineCard struct {
	fam  string
	id   string   // member shown
	mem  []string // unlocked members (tabs)
	nlk  int      // locked members
	h    float64
	x, y float64
}

func (g *Game) charCards() []*lineCard {
	pr := g.prof
	q := g.charQ
	okC := func(id string) bool {
		if !pr.Unl(id) || !g.srchOK(id, q) {
			return false
		}
		if g.charF == "" || g.charF == "all" {
			return true
		}
		for _, t := range g.db.ByID[id].Types {
			if t == g.charF {
				return true
			}
		}
		return false
	}
	var fams []string
	seen := map[string]bool{}
	for _, p := range g.db.Pokemon {
		f := pr.Fam(p.ID)
		if seen[f] {
			continue
		}
		for _, x := range g.db.Pokemon {
			if pr.Fam(x.ID) == f && okC(x.ID) {
				seen[f] = true
				fams = append(fams, f)
				break
			}
		}
	}
	var out []*lineCard
	for _, f := range fams {
		lc := &lineCard{fam: f}
		var pass []string
		for _, x := range g.db.Pokemon {
			if pr.Fam(x.ID) != f {
				continue
			}
			if pr.Unl(x.ID) {
				lc.mem = append(lc.mem, x.ID)
				if okC(x.ID) {
					pass = append(pass, x.ID)
				}
			} else {
				lc.nlk++
			}
		}
		pick := ""
		for _, x := range pass {
			if x == g.csel {
				pick = x
			}
		}
		if pick == "" {
			for _, x := range pass {
				if x == g.cshow[f] {
					pick = x
				}
			}
		}
		if pick == "" {
			pick = pass[len(pass)-1]
		}
		lc.id = pick
		out = append(out, lc)
	}
	return out
}

// cardHit is a clickable region inside a card.
type cardHit struct {
	key      string
	r        [4]float64
	disabled bool
	act      func()
}

func (g *Game) upgReady(fam string) bool {
	pr := g.prof
	for _, u := range meta.Upgrades {
		if !u.Global && pr.UL(u.Key, fam) < u.Max && pr.S.Coins() >= pr.UpCost(u.Key, fam) {
			return true
		}
	}
	t := pr.Training(fam)
	return t < meta.TrainingMax && pr.S.Coins() >= meta.TrainingCost(t)
}

// charCard lays out one line card at (x, y); with dst it also draws it. It returns the height and
// the clickable regions (tabs, upgrades, abilities, the hidden-ability unlock).
func (g *Game) charCard(dst *ebiten.Image, lc *lineCard, x, y float64) (float64, []cardHit) {
	pr := g.prof
	id := lc.id
	p := g.db.ByID[id]
	f := lc.fam
	col := tc(p.Types[0])
	foc := g.ui.focus
	var hits []cardHit
	in := x + 14 // content left (border 2 + padding 12)
	cw := ccW - 28
	yy := y + 14
	sel := id == g.csel
	if dst != nil {
		border := webBorder
		if sel {
			border = webAcc
		} else if foc == "cc:"+f {
			border = webAcc
		}
		webBox(dst, x, y, ccW, lc.h, 12, border, webPanel)
	}
	// tabs for the line's members
	if len(lc.mem) > 1 || lc.nlk > 0 {
		n := len(lc.mem)
		if lc.nlk > 0 {
			n++
		}
		tw := (cw - float64(n-1)*4) / float64(n)
		for i, m := range lc.mem {
			m := m
			r := [4]float64{in + float64(i)*(tw+4), yy - 2, tw, 52}
			hits = append(hits, cardHit{key: "ct:" + m, r: r, act: func() {
				g.cshow[f] = m
				g.csel = m
				g.save()
			}})
			if dst != nil {
				on := m == id
				border, bg := webBorder, hexC("#10131d")
				if on {
					border, bg = webAcc, hexC("#2a2a14")
				} else if foc == "ct:"+m {
					border = mix(webBorder, webAcc, .5)
				}
				webBox(dst, r[0], r[1], r[2], r[3], 8, border, bg)
				if s := g.assets.Sprite(m); s != nil && s.Front != nil {
					im := s.Front.Frame(s.Front.IdleFrame(0))
					bx := s.Front.Box(s.Front.IdleFrame(0))
					k := math.Min(30/float64(bx.Dx()), 30/float64(bx.Dy()))
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(k, k)
					op.GeoM.Translate(r[0]+r[2]/2-float64(bx.Dx())*k/2, r[1]+4+15-float64(bx.Dy())*k/2)
					dst.DrawImage(im.SubImage(bx).(*ebiten.Image), op)
				}
				nm := strings.Replace(g.db.ByID[m].Name, "Alolan ", "A. ", 1)
				drawText(dst, fitText(nm, 10, false, r[2]-6), r[0]+r[2]/2, r[1]+36, 10, false, webFg, text.AlignCenter, false)
			}
		}
		if lc.nlk > 0 && dst != nil {
			r := [4]float64{in + float64(len(lc.mem))*(tw+4), yy - 2, tw, 52}
			webBox(dst, r[0], r[1], r[2], r[3], 8, webBorder, hexC("#10131d"))
			drawGlyph(dst, gLock, r[0]+r[2]/2, r[1]+18, 19, alpha(webFg, .6))
			drawText(dst, fmt.Sprintf("%d more", lc.nlk), r[0]+r[2]/2, r[1]+36, 10, false, alpha(webFg, .7), text.AlignCenter, false)
		}
		yy += 50 + 8
	}
	// upgrades button (top-right corner)
	nu := 0
	for _, u := range meta.Upgrades {
		if !u.Global {
			nu += pr.UL(u.Key, f)
		}
	}
	{
		r := [4]float64{x + ccW - 26, y - 9, 34, 34}
		hits = append(hits, cardHit{key: "cu:" + f, r: r, act: func() {
			g.lineFam = f
			g.lineFrom = scChars
			g.scr = scLine
			g.ui.focus = ""
		}})
		if dst != nil {
			cx, cy := r[0]+17, r[1]+17
			soft(dst, cx, cy+3, 22, 22, c(0, 0, 0, 140))
			ringCol := hexC("#5a6a90")
			if foc == "cu:"+f {
				ringCol = webAcc
			}
			if g.upgReady(f) {
				disc(dst, cx, cy, 20, alpha(webGreen, .33))
				ringCol = webGreen
			}
			disc(dst, cx, cy, 17, ringCol)
			disc(dst, cx, cy, 15, hexC("#1f2a44"))
			drawGlyph(dst, gUp, cx, cy, 15, webAcc)
			if nu > 0 {
				s := strconv.Itoa(nu)
				bw := textW(s, 10, false) + 8
				roundRect(dst, cx+13-bw/2, cy+11, bw, 13, 6, hexC("#5a6a90"))
				roundRect(dst, cx+13-bw/2+1, cy+12, bw-2, 11, 5, webInk)
				drawText(dst, s, cx+13, cy+11, 10, false, white, text.AlignCenter, false)
			}
		}
	}
	// header: portrait, name, type, start move
	if dst != nil {
		g.drawPortrait(dst, id, in, yy, 76, false, sel, true)
		tx := in + 86
		drawText(dst, p.Name, tx, yy+10, 17, true, col, text.AlignStart, false)
		ty := ""
		for i, t := range p.Types {
			if i > 0 {
				ty += "/"
			}
			ty += cap1s(t)
		}
		drawText(dst, fitText(ty+" type"+tagText(p), 12, false, cw-86), tx, yy+34, 12, false, alpha(webFg, .75), text.AlignStart, false)
		drawText(dst, fitText("Starts with "+g.db.Moves[g.db.StartMove(id)].Name, 12, false, cw-86), tx, yy+52, 12, false, alpha(webFg, .75), text.AlignStart, false)
	}
	yy += 76 + 6
	// mastery
	root := pr.LineRoot(id)
	rk, into, need := pr.Mastery(root)
	if dst != nil {
		parts := []any{gStar, "Mastery Rank ", true, strconv.Itoa(rk), false}
		hasEvo := root != id
		for _, x := range g.db.Pokemon {
			if x.EvolvesFrom == root {
				hasEvo = true
			}
		}
		if hasEvo {
			parts = append(parts, " · "+g.db.ByID[root].Name+" line")
		}
		richLine(dst, in, yy, 12, false, webFg, append([]any{webAcc}, append([]any{parts[0], webFg}, parts[1:]...)...)...)
		pbar(dst, in, yy+19, cw, 6, float64(into)/float64(max(1, need)), webAcc)
		drawText(dst, fmt.Sprintf("%s / %s XP to rank %d", commas(into), commas(need), rk+1), in, yy+29, 12, false, alpha(webFg, .7), text.AlignStart, false)
	}
	yy += 50
	// matchups
	yy += 2
	yy += g.matchupRows(dst, p.Types, in, yy, cw, false)
	yy += 4
	// stats
	st := g.db.Stats[id]
	ul := func(k string) float64 { return float64(pr.UL(k, f)) }
	vals := []struct {
		l      string
		v, max float64
		col    color.NRGBA
	}{
		{"HP", math.Round((80 + 14*ul("hp")) * st.HP), 150, webGreen},
		{"ATK", math.Round(100 * st.Atk * (1 + .08*ul("dmg")) * (1 + .02*float64(pr.Training(f)))), 150, hexC("#ff7a4d")},
		{"DEF", math.Round(100 * st.Def / math.Pow(.96, ul("arm"))), 150, hexC("#6fb5ff")},
		{"SPD", math.Round(125 * st.Spd * (1 + .03*ul("spd"))), 170, webAcc},
	}
	for _, v := range vals {
		if dst != nil {
			drawText(dst, v.l, in, yy, 12, false, alpha(webFg, .8), text.AlignStart, false)
			pbar(dst, in+36, yy+5, cw-36-36, 7, v.v/v.max, v.col)
			drawText(dst, strconv.Itoa(int(v.v)), in+cw, yy, 12, false, webFg, text.AlignEnd, false)
		}
		yy += 18
	}
	// type effect
	fl := typeFlavor[p.Types[0]]
	if len(p.Types) > 1 {
		ms := ""
		for i, t := range p.Types {
			if i > 0 {
				ms += " + "
			}
			ms += cap1s(t)
		}
		fl += " · Moves from " + ms + " movesets"
	}
	for i, ln := range wrap(fl, font(12, false), cw-16) {
		if dst != nil {
			if i == 0 {
				drawGlyph(dst, gSparkle, in+6, yy+8, 11, alpha(webFg, .75))
			}
			drawText(dst, ln, in+16*b2f(i == 0, 1, 0), yy, 12, false, alpha(webFg, .75), text.AlignStart, false)
		}
		yy += 15
	}
	yy += 3
	// abilities
	ab := g.db.AbilityPair(id)
	pu := pr.HiddenOwned(id)
	cur := pr.AbilityIdx(id)
	kl := pr.FamKills(id)
	for i, a := range ab {
		i := i
		kind := "Ability"
		if i == 1 {
			kind = "Hidden Ability"
			if a.Tier == 3 {
				kind = "Legendary Ability"
			}
		}
		on := cur == i
		lockedAb := i == 1 && !pu
		title := kind + ": " + a.Name
		lines := wrap(a.Desc, font(12, false), cw-18)
		h := 8 + 17 + float64(len(lines))*15 + 6
		var btn [4]float64
		btnLabel, btnOK := "", false
		var btnParts []any
		if lockedAb {
			h += 12 + 18
			switch {
			case kl < meta.HiddenKills:
				btnLabel, btnParts = "Needs "+commas(meta.HiddenKills)+" kills", []any{gLock}
			case pr.S.Tickets() < 1:
				btnLabel, btnParts = "Needs 1 ticket", []any{gTicket}
			default:
				btnLabel, btnParts, btnOK = "Unlock with 1", []any{}, true
			}
			bw := smallBtnW(btnLabel, btnParts...)
			if btnOK {
				bw += 18
			}
			btn = [4]float64{in + 8, yy + h + 1, bw, 25}
			h += 31
		}
		box := [4]float64{in, yy, cw, h}
		if !lockedAb {
			hits = append(hits, cardHit{key: fmt.Sprintf("ca:%s:%d", f, i), r: box, act: func() {
				pr.SetAbilityIdx(id, i)
				g.csel = id
				g.save()
			}})
		} else {
			hits = append(hits, cardHit{key: "ch:" + f, r: btn, disabled: !btnOK, act: func() {
				if pr.BuyHidden(id) {
					g.toast("Hidden Ability unlocked: " + ab[1].Name)
					g.save()
				}
			}})
		}
		if dst != nil {
			border, bg := webBorder, webPanel
			if on {
				border, bg = webAcc, mix(webPanel, webAcc, .094)
			} else if foc == fmt.Sprintf("ca:%s:%d", f, i) {
				border = mix(webBorder, webAcc, .5)
			}
			roundRect(dst, box[0], box[1], box[2], box[3], 8, border)
			roundRect(dst, box[0]+1, box[1]+1, box[2]-2, box[3]-2, 7, bg)
			ty := box[1] + 7
			parts := []any{}
			if on {
				parts = append(parts, gCheck)
			}
			if a.Tier == 3 && i == 1 {
				parts = append(parts, gStar)
			}
			parts = append(parts, title)
			richLine(dst, box[0]+9, ty, 12, true, webFg, parts...)
			ty += 18
			for _, ln := range lines {
				drawText(dst, ln, box[0]+9, ty, 12, false, alpha(webFg, .9), text.AlignStart, false)
				ty += 15
			}
			if lockedAb {
				pbar(dst, box[0]+9, ty+4, box[2]-18, 6, float64(kl)/meta.HiddenKills, webCyan)
				drawText(dst, fmt.Sprintf("%s / %s kills with this line", commas(min(kl, meta.HiddenKills)), commas(meta.HiddenKills)), box[0]+9, ty+14, 12, false, alpha(webFg, .75), text.AlignStart, false)
				if btnOK {
					smallBtn(dst, btn, "", foc == "ch:"+f, false, "Unlock with 1 ", gTicket)
				} else {
					smallBtn(dst, btn, btnLabel, foc == "ch:"+f, true, btnParts...)
				}
			}
		}
		yy += h + 6
	}
	// footer
	foot := fmt.Sprintf("%d upgrade%s bought", nu, plural(nu))
	if tr := pr.Training(f); tr > 0 {
		foot += fmt.Sprintf(" · Training %d", tr)
	}
	if dst != nil {
		drawText(dst, foot, in, yy, 12, false, alpha(webFg, .75), text.AlignStart, false)
		if sel {
			richLine(dst, in, yy+18, 12, false, alpha(webFg, .75), gCheck, "Selected")
		} else {
			drawText(dst, "Tap to select", in, yy+18, 12, false, alpha(webFg, .75), text.AlignStart, false)
		}
	}
	yy += 36 + 10
	return yy - y, hits
}

// webHeader is the title row of the rebuilt screens: back button, centred title and subtitle,
// tickets on the right.
func (g *Game) webHeader(title, sub string, back screen, right string) {
	sw := float64(g.sw)
	u := &g.ui
	u.label([4]float64{0, 18, sw, 36}, false, textDraw(title, 26, true, webFg, text.AlignCenter))
	if sub != "" {
		u.label([4]float64{0, 56, sw, 18}, false, textDraw(sub, 13, false, alpha(webFg, .85), text.AlignCenter))
	}
	tk := g.prof.S.Tickets()
	u.label([4]float64{sw - 300, 22, 280, 24}, false, func(dst *ebiten.Image, r [4]float64) {
		parts := []any{}
		if right != "" {
			parts = append(parts, right+" · ")
		}
		parts = append(parts, gTicket, white, true, strconv.Itoa(tk))
		w := richLine(nil, 0, 0, 16, false, webFg, parts...)
		richLine(dst, r[0]+r[2]-w, r[1], 16, false, webFg, parts...)
	})
	g.backButton(back)
}

func (g *Game) menuChars() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	if g.cshow == nil {
		g.cshow = map[string]string{}
	}
	if g.csel == "" || g.db.ByID[g.csel] == nil || !g.prof.Unl(g.csel) {
		g.csel = "fire"
	}
	g.webHeader("Pokémon", "Tap a Pokémon to select it. Find locked Pokémon, unlock conditions and ticket unlocks in the Pokédex.", scMenu, "")
	g.searchBox("charq", [4]float64{sw/2 - 210, 84, 420, 40}, &g.charQ, "Search name, type or #number")
	// type chips: All + the types of unlocked Pokémon
	tys := map[string]bool{}
	for _, p := range g.db.Pokemon {
		if g.prof.Unl(p.ID) {
			for _, t := range p.Types {
				tys[t] = true
			}
		}
	}
	type chip struct{ k, l string }
	chips := []chip{{"all", "All"}}
	for _, t := range typeOrder {
		if tys[t] {
			chips = append(chips, chip{t, cap1s(t)})
		}
	}
	// lay chips out in centred rows (max 700 wide)
	var rows [][]int
	var rw []float64
	cw := func(ch chip) float64 {
		if ch.k == "all" {
			return textW(ch.l, 13, false) + 24
		}
		return textW(ch.l, 13, false) + 24 + 20
	}
	for i, ch := range chips {
		w := cw(ch)
		if len(rows) == 0 || rw[len(rw)-1]+w+6 > 700 {
			rows = append(rows, nil)
			rw = append(rw, -6)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], i)
		rw[len(rw)-1] += w + 6
	}
	cy := 134.0
	for ri, row := range rows {
		cx := sw/2 - rw[ri]/2
		for _, i := range row {
			ch := chips[i]
			w := cw(ch)
			on := g.charF == ch.k || (g.charF == "" && ch.k == "all")
			u.button("cf:"+ch.k, [4]float64{cx, cy, w, 30}, false, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
				chipBtn(dst, r, on, hot, func(x, y float64) {
					if ch.k != "all" {
						g.drawTypeIcon(dst, ch.k, x+8, y, 16, false)
						x += 20
					}
					drawText(dst, ch.l, x, y-8, 13, false, webFg, text.AlignStart, false)
				})
			}, func() {
				g.charF = ch.k
				u.scroll = 0
			})
			cx += w + 6
		}
		cy += 36
	}
	// cards
	top := cy + 10
	barH := 66.0
	u.area = [4]float64{0, top, sw, sh - top - barH}
	cards := g.charCards()
	cols := max(1, int((sw-24)/(ccW+10)))
	gx := sw/2 - (float64(cols)*(ccW+10)-10)/2
	y := 12.0 // room for the upgrade buttons poking out
	for i := 0; i < len(cards); i += cols {
		rowH := 0.0
		for j := i; j < min(i+cols, len(cards)); j++ {
			lc := cards[j]
			lc.h, _ = g.charCardMeasure(lc)
			rowH = max(rowH, lc.h)
		}
		for j := i; j < min(i+cols, len(cards)); j++ {
			lc := cards[j]
			lc.h = rowH // web: cards in a row share its height
			lc.x, lc.y = gx+float64(j-i)*(ccW+10), y
		}
		y += rowH + 10
	}
	if len(cards) == 0 {
		u.label([4]float64{0, 20, sw, 20}, true, textDraw("No unlocked Pokémon match that search.", 14, false, alpha(webFg, .7), text.AlignCenter))
	}
	for _, lc := range cards {
		lc := lc
		_, hits := g.charCard(nil, lc, lc.x, lc.y)
		// the card's controls first (the first widget under the pointer gets the click), then the
		// card itself, which draws everything and selects the Pokémon
		for _, h := range hits {
			h := h
			u.button(h.key, h.r, true, h.disabled, nil, h.act)
		}
		u.button("cc:"+lc.fam, [4]float64{lc.x, lc.y, ccW, lc.h}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
			// cards are rendered once into an image and re-rendered only when their state changes
			// (they are made of many anti-aliased shapes, too slow to redraw every frame)
			foc := g.ui.focus
			rel := ""
			if strings.HasSuffix(foc, ":"+lc.fam) || strings.Contains(foc, ":"+lc.fam+":") || (strings.HasPrefix(foc, "ct:") && contains(lc.mem, foc[3:])) {
				rel = foc
			}
			key := fmt.Sprintf("cc|%s|%s|%s|%s|%.0f|%d", lc.fam, lc.id, g.csel, rel, lc.h, g.saveGen)
			im := g.cacheImg(key, int(ccW+24), int(lc.h+24), func(img *ebiten.Image) { g.charCard(img, lc, 12, 12) })
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(r[0]-12, r[1]-12)
			dst.DrawImage(im, op)
			if lc.id == g.csel { // the selected Pokémon's portrait is animated
				py := r[1] + 14
				if len(lc.mem) > 1 || lc.nlk > 0 {
					py += 58
				}
				g.drawPortrait(dst, lc.id, r[0]+14, py, 76, false, true, true)
			}
		}, func() {
			g.csel = lc.id
			g.cshow[lc.fam] = lc.id
			g.save()
		})
	}
	u.scrollMax = math.Max(0, y-u.area[3])
	// bottom bar: selected Pokémon and start
	p := g.db.ByID[g.csel]
	u.label([4]float64{0, sh - barH, sw, barH}, false, func(dst *ebiten.Image, r [4]float64) {
		rect(dst, r[0], r[1], r[2], r[3], alpha(webBg, .96))
		rect(dst, r[0], r[1], r[2], 1, webBorder)
		g.drawPortrait(dst, g.csel, sw/2-330, r[1]+8, 50, false, true, true)
		drawText(dst, p.Name, sw/2-268, r[1]+12, 16, true, tc(p.Types[0]), text.AlignStart, false)
		ab := g.db.AbilityPair(g.csel)[g.prof.AbilityIdx(g.csel)]
		drawText(dst, "Ability: "+ab.Name, sw/2-268, r[1]+34, 12, false, alpha(webFg, .75), text.AlignStart, false)
	})
	u.button("start", [4]float64{sw/2 + 20, sh - barH + 9, 310, 48}, false, false, btnDraw("Start run", g.setupSummary(), gold, false), func() { g.startFromMenu() })
}

// charCardMeasure returns a card's natural height (cached briefly: it wraps a lot of text).
func (g *Game) charCardMeasure(lc *lineCard) (float64, []cardHit) {
	k := fmt.Sprintf("%s|%v|%v|%d|%d|%d|%d", lc.id, lc.id == g.csel, g.prof.HiddenOwned(lc.id), g.prof.AbilityIdx(lc.id), len(lc.mem), lc.nlk, g.prof.S.Tickets())
	if h, ok := g.cardH[k]; ok && g.tick-g.cardT < 120 {
		return h, nil
	}
	if g.cardH == nil || g.tick-g.cardT >= 120 {
		g.cardH = map[string]float64{}
		g.cardT = g.tick
	}
	save := lc.h
	lc.h = 0
	h, hits := g.charCard(nil, lc, 0, 0)
	lc.h = save
	g.cardH[k] = h
	return h, hits
}
