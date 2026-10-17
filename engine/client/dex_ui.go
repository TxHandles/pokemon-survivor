package client

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Pokédex, rebuilt to match the web version (dex()): search, status filters with counts, order and
// type filters, a grid of cards (silhouettes and unlock hints for locked ones) and, for the selected
// Pokémon, a detail panel with forms, how to unlock it, ticket unlocks, stats, matchups, its
// evolution line drawn as a tree with the requirement on each arrow, and related legendaries.

const dexW = 820.0

func (g *Game) dexBase(id string) string {
	if p := g.db.ByID[id]; p != nil && p.RegionalOf != "" {
		return p.RegionalOf
	}
	return id
}

func (g *Game) dexForms(b string) []string {
	out := []string{b}
	for _, p := range g.db.Pokemon {
		if p.RegionalOf == b {
			out = append(out, p.ID)
		}
	}
	return out
}

// lineOf: the evolution line (web lineOf: follows evolvesFrom only).
func (g *Game) lineOf(id string) []string {
	r := id
	for g.db.ByID[r] != nil && g.db.ByID[r].EvolvesFrom != "" {
		r = g.db.ByID[r].EvolvesFrom
	}
	out := []string{r}
	for i := 0; i < len(out); i++ {
		for _, p := range g.db.Pokemon {
			if p.EvolvesFrom == out[i] && !contains(out, p.ID) {
				out = append(out, p.ID)
			}
		}
	}
	return out
}

func (g *Game) dexReady(id string) bool {
	pr := g.prof
	if pr.Unl(id) {
		return false
	}
	c := pr.TicketCost(id)
	return c > 0 && pr.S.Tickets() >= c
}

var masteryRe = func(s string) (int, bool) {
	i := strings.Index(s, "Mastery Rank ")
	if i < 0 {
		return 0, false
	}
	j := i + len("Mastery Rank ")
	k := j
	for k < len(s) && s[k] >= '0' && s[k] <= '9' {
		k++
	}
	n, err := strconv.Atoi(s[j:k])
	return n, err == nil
}

// dexHint is the short unlock hint on a locked card (web dexHint).
func (g *Game) dexHint(id string) (glyph, string) {
	pr := g.prof
	if pr.Unl(id) {
		return 0, ""
	}
	p := g.db.ByID[id]
	if p.BaseStats != nil || pr.TicketCost(id) > 0 {
		if k := pr.TicketCost(id); k > 0 {
			return gTicket, fmt.Sprintf("%d ticket%s", k, plural(k))
		}
		return 0, ""
	}
	if p.EvolvesFrom != "" {
		if n, ok := masteryRe(p.UnlockText); ok {
			s := fmt.Sprintf("Mastery %d", n)
			if p.DexNote != "" {
				s += " + more"
			}
			return gStar, s
		}
	}
	if p.Legendary {
		return gTrophy, "Biome clear"
	}
	return gSparkle, "Special unlock"
}

// unlockInfo: how a Pokémon is unlocked (web unlockInfo): text, progress and its tag.
func (g *Game) unlockInfo(id string) (txt string, have, need int, tag string) {
	pr := g.prof
	p := g.db.ByID[id]
	if p.BaseStats == nil {
		if k := pr.TicketCost(id); k > 0 {
			return fmt.Sprintf("Unlock with %d boss-chest ticket%s", k, plural(k)), 0, 0, "Starter"
		}
		h, n := pr.Progress(id)
		return p.UnlockText, h, n, p.Tag
	}
	if k := pr.TicketCost(id); k > 0 {
		return fmt.Sprintf("Unlock with %d boss-chest ticket%s", k, plural(k)), 0, 0, "Starter"
	}
	return "Available from the start", 0, 0, "Starter"
}

// dexCardH is the height of a Dex card of width w.
func dexCardH(small bool) float64 {
	if small {
		return 124
	}
	return 138
}

// drawDexCard draws a Dex card (web dexCard).
func (g *Game) drawDexCard(dst *ebiten.Image, id string, r [4]float64, small, hot bool) {
	pr := g.prof
	p := g.db.ByID[id]
	lk := !pr.Unl(id)
	border := webBorder
	switch {
	case id == g.dexSel:
		border = webAcc
	case hot:
		border = mix(webBorder, webAcc, .55)
	case g.dexReady(id):
		border = webGreen
	case p.Legendary:
		border = hexC("#c9a64a")
	}
	webBox(dst, r[0], r[1], r[2], r[3], 12, border, webPanel)
	cx := r[0] + r[2]/2
	g.drawPortrait(dst, id, cx-32, r[1]+10, 64, lk, false, true)
	parts := []any{"#" + g.dexNo(id)}
	if p.Legendary {
		parts = append(parts, " ", gStar)
	}
	if n := len(g.dexForms(g.dexBase(id))); n > 1 {
		parts = append(parts, " · ", gSwap, fmt.Sprintf("%d forms", n))
	}
	nw := richLine(nil, 0, 0, 11, false, webFg, parts...)
	richLine(dst, cx-nw/2, r[1]+79, 11, false, alpha(webFg, .7), parts...)
	name := p.Name
	if lk {
		name = "???"
	}
	drawText(dst, fitText(name, 13, true, r[2]-10), cx, r[1]+95, 13, true, alpha(webFg, b2f(lk, .6, 1)), text.AlignCenter, false)
	if lk && !small {
		if gl, h := g.dexHint(id); h != "" {
			col := hexC("#ffd98a")
			w := richLine(nil, 0, 0, 10, false, col, gl, h)
			richLine(dst, cx-w/2, r[1]+114, 10, false, col, gl, h)
		}
	} else if lk && small {
		if gl, h := g.dexHint(id); h != "" {
			col := hexC("#ffd98a")
			w := richLine(nil, 0, 0, 10, false, col, gl, h)
			richLine(dst, cx-w/2, r[1]+108, 10, false, col, gl, h)
		}
	}
}

func (g *Game) dexClick(id string) {
	g.dexSel = id
	if g.dexForm == nil {
		g.dexForm = map[string]string{}
	}
	if b := g.dexBase(id); b != id || g.dexForm[b] != "" {
		g.dexForm[b] = id
	}
	g.ui.scroll = 0
}

// dexDetail lays out (and with dst draws) the detail panel for the selected Pokémon.
func (g *Game) dexDetail(dst *ebiten.Image, x, y, w float64) (float64, []cardHit) {
	pr := g.prof
	id := g.dexSel
	p := g.db.ByID[id]
	if p == nil {
		return 0, nil
	}
	foc := g.ui.focus
	lk := !pr.Unl(id)
	var hits []cardHit
	in := x + 14
	iw := w - 28
	yy := y + 14
	// header
	if dst != nil {
		g.drawPortrait(dst, id, in, yy, 84, lk, false, true)
		col := tc(p.Types[0])
		name := p.Name
		if lk {
			col, name = hexC("#aaaaaa"), "???"
		}
		drawText(dst, "#"+g.dexNo(id)+" "+name, in+94, yy+22, 18, true, col, text.AlignStart, false)
		_, _, _, tag := g.unlockInfo(id)
		ty := "Undiscovered"
		if !lk {
			ty = ""
			for i, t := range p.Types {
				if i > 0 {
					ty += " / "
				}
				ty += cap1s(t)
			}
			ty += " type"
		}
		if tag != "" {
			ty += " · " + tag
		}
		drawText(dst, ty, in+94, yy+48, 12, false, alpha(webFg, .75), text.AlignStart, false)
	}
	yy += 84 + 8
	// forms
	b0 := g.dexBase(id)
	if fs := g.dexForms(b0); len(fs) > 1 {
		if dst != nil {
			drawText(dst, "Forms:", in, yy+6, 12, false, alpha(webFg, .75), text.AlignStart, false)
		}
		fx := in + 50
		for _, f := range fs {
			f := f
			lbl := g.db.ByID[f].Form
			if lbl == "" {
				lbl = "Regular"
			}
			parts := []any{}
			if !pr.Unl(f) {
				parts = append(parts, lbl+" ", gLock)
				lbl = ""
			}
			bw := smallBtnW(lbl, parts...) + 6
			r := [4]float64{fx, yy, bw, 28}
			hits = append(hits, cardHit{key: "df:" + f, r: r, act: func() {
				if g.dexForm == nil {
					g.dexForm = map[string]string{}
				}
				g.dexForm[b0] = f
				g.dexSel = f
			}})
			if dst != nil {
				hot := foc == "df:"+f || f == id
				smallBtn(dst, r, lbl, hot, false, parts...)
			}
			fx += bw + 6
		}
		yy += 28 + 8
	}
	// how to unlock
	txt, have, need, _ := g.unlockInfo(id)
	if dst != nil {
		if lk {
			richLine(dst, in, yy, 16, true, webFg, gLock, "How to unlock")
		} else {
			richLine(dst, in, yy, 16, true, webFg, gCheck, "Unlocked")
		}
	}
	yy += 22
	for _, ln := range wrap(txt, font(12, false), iw) {
		if dst != nil {
			drawText(dst, ln, in, yy, 12, false, alpha(webFg, .8), text.AlignStart, false)
		}
		yy += 16
	}
	if lk && need > 0 {
		if dst != nil {
			pbar(dst, in, yy+4, iw, 6, float64(have)/float64(need), webCyan)
			drawText(dst, fmt.Sprintf("%s / %s", commas(min(have, need)), commas(need)), in, yy+14, 12, false, alpha(webFg, .75), text.AlignStart, false)
		}
		yy += 34
	}
	yy += 6
	// ticket unlock
	if lk {
		if cst := pr.TicketCost(id); cst > 0 {
			hv := pr.S.Tickets()
			ok := hv >= cst
			lbl, parts := fmt.Sprintf("Unlock for %d ", cst), []any{}
			if !ok {
				lbl, parts = fmt.Sprintf(" Need %d ticket%s (you have %d)", cst, plural(cst), hv), []any{gTicket}
			}
			bw := smallBtnW(lbl, parts...) + 20
			r := [4]float64{in, yy, bw, 30}
			hits = append(hits, cardHit{key: "dbuy", r: r, disabled: !ok, act: func() {
				if pr.BuyUnlock(id) {
					g.csel = id
					g.toast(p.Name + " unlocked!")
					g.save()
				}
			}})
			if dst != nil {
				if ok {
					smallBtn(dst, r, "", foc == "dbuy", false, lbl, gTicket)
				} else {
					smallBtn(dst, r, lbl, false, true, parts...)
				}
			}
			yy += 30 + 10
		}
	} else {
		st := g.db.Stats[id]
		ab := g.db.AbilityPair(id)
		hid := ab[1].Name + " (hidden)"
		if ab[1].Tier == 3 {
			hid = "Legendary: " + ab[1].Name + " (hidden)"
		}
		lines := []string{
			fmt.Sprintf("HP ×%.2f · ATK ×%.2f · DEF ×%.2f · SPD ×%.2f", st.HP, st.Atk, st.Def, st.Spd),
			"Abilities: " + ab[0].Name + " / " + hid,
		}
		sm := "Starts with " + g.db.Moves[g.db.StartMove(id)].Name
		if len(p.Types) > 1 {
			ms := ""
			for i, t := range p.Types {
				if i > 0 {
					ms += " + "
				}
				ms += cap1s(t)
			}
			sm += " · Moves from " + ms + " movesets"
		}
		lines = append(lines, sm)
		for _, l := range lines {
			for _, ln := range wrap(l, font(12, false), iw) {
				if dst != nil {
					drawText(dst, ln, in, yy, 12, false, alpha(webFg, .8), text.AlignStart, false)
				}
				yy += 20
			}
		}
		yy += g.matchupRows(dst, p.Types, in, yy, iw, false) + 6
		sel := id == g.csel
		lbl, parts := "Select this character", []any{}
		if sel {
			lbl, parts = "Selected", []any{gCheck}
		}
		bw := smallBtnW(lbl, parts...)
		r := [4]float64{in, yy, bw, 30}
		hits = append(hits, cardHit{key: "dsel", r: r, act: func() {
			g.csel = id
			g.save()
		}})
		if dst != nil {
			smallBtn(dst, r, lbl, foc == "dsel", false, parts...)
		}
		yy += 30 + 10
	}
	// evolution line, drawn as a tree with the requirement on each arrow
	ln := g.lineOf(id)
	if len(ln) > 1 {
		if dst != nil {
			drawText(dst, "Evolution line", in, yy, 16, true, webFg, text.AlignStart, false)
		}
		yy += 24
		cw := 96.0
		chh := dexCardH(false)
		aw := 96.0
		var node func(x string, px, py float64) float64 // returns the height used
		node = func(xid string, px, py float64) float64 {
			xid2 := xid
			r := [4]float64{px, py, cw, chh}
			hits = append(hits, cardHit{key: "de:" + xid2, r: r, act: func() { g.dexClick(xid2) }})
			if dst != nil {
				g.drawDexCard(dst, xid2, r, false, foc == "de:"+xid2)
			}
			var kids []string
			for _, k := range ln {
				if g.db.ByID[k].EvolvesFrom == xid {
					kids = append(kids, k)
				}
			}
			if len(kids) == 0 {
				return chh
			}
			ax := px + cw + 6
			if len(kids) > 1 && dst != nil { // branch: a bracket line
				rect(dst, ax-2, py, 2, float64(len(kids))*(chh+6)-6, alpha(white, .13))
			}
			hy := 0.0
			for i, k := range kids {
				ky := py + hy
				if dst != nil {
					kp := g.db.ByID[k]
					req := stripHTML(kp.DexNote)
					if req == "" {
						req = kp.UnlockText
					}
					drawGlyph(dst, gArrow, ax+aw/2, ky+chh/2-30, 11, alpha(webFg, .85))
					for j, l := range wrap(req, font(11, false), aw-4) {
						if j > 4 {
							break
						}
						drawText(dst, l, ax+aw/2, ky+chh/2-20+float64(j)*14, 11, false, alpha(webFg, .85), text.AlignCenter, false)
					}
				}
				h := node(k, ax+aw+6, ky)
				hy += h
				if i < len(kids)-1 {
					hy += 6
				}
			}
			return math.Max(chh, hy)
		}
		yy += node(ln[0], in, yy) + 10
	}
	// related legendaries (same main type, not in the line)
	var rel []string
	for _, q := range g.db.Pokemon {
		if q.Legendary && q.Types[0] == p.Types[0] && !contains(ln, q.ID) {
			rel = append(rel, q.ID)
		}
	}
	if len(rel) > 0 {
		if dst != nil {
			drawText(dst, "Related legendary", in, yy, 16, true, webFg, text.AlignStart, false)
		}
		yy += 24
		for i, q := range rel {
			q := q
			r := [4]float64{in + float64(i)*102, yy, 96, dexCardH(false)}
			hits = append(hits, cardHit{key: "dl:" + q, r: r, act: func() { g.dexClick(q) }})
			if dst != nil {
				g.drawDexCard(dst, q, r, false, foc == "dl:"+q)
			}
		}
		yy += dexCardH(false) + 10
	}
	return yy - y + 4, hits
}

func (g *Game) menuDex() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	if g.dexForm == nil {
		g.dexForm = map[string]string{}
	}
	if g.dexSt == "" {
		g.dexSt = "all"
	}
	if g.dexTy == "" {
		g.dexTy = "all"
	}
	got := 0
	var all []string
	for _, p := range g.db.Pokemon {
		all = append(all, p.ID)
		if pr.Unl(p.ID) {
			got++
		}
	}
	g.webHeader("Pokédex", "", scMenu, fmt.Sprintf("%d / %d", got, len(all)))
	g.searchBox("dexq", [4]float64{sw/2 - 210, 62, 420, 40}, &g.dexQ, "Search name, type or #number")
	// status filters with counts
	is := func(k, f string) bool {
		switch k {
		case "got":
			return pr.Unl(f)
		case "lock":
			return !pr.Unl(f)
		case "rdy":
			return g.dexReady(f)
		case "leg":
			return g.db.ByID[f].Legendary
		}
		return true
	}
	cnt := func(k string) int {
		n := 0
		for _, id := range all {
			if g.db.ByID[id].RegionalOf != "" {
				continue
			}
			for _, f := range g.dexForms(id) {
				if is(k, f) {
					n++
					break
				}
			}
		}
		return n
	}
	type flt struct {
		k, l string
		gl   glyph
	}
	fl := []flt{{"all", "All", -1}, {"got", "Unlocked", -1}, {"lock", "Locked", -1}, {"rdy", "Can unlock", gTicket}, {"leg", "Legendary", -1}}
	type chipDef struct {
		key   string
		label string
		gl    glyph
		on    bool
		act   func()
	}
	var defs []chipDef
	for _, f := range fl {
		f := f
		defs = append(defs, chipDef{"ds:" + f.k, fmt.Sprintf("%s (%d)", f.l, cnt(f.k)), f.gl, g.dexSt == f.k, func() { g.dexSt = f.k; u.scroll = 0 }})
	}
	ord := "Order: by line"
	if g.dexSo == "num" {
		ord = "Order: Dex #"
	}
	defs = append(defs, chipDef{"dso", ord, -1, false, func() {
		if g.dexSo == "num" {
			g.dexSo = "line"
		} else {
			g.dexSo = "num"
		}
	}})
	tyl := "All types"
	if g.dexTy != "all" {
		tyl = cap1s(g.dexTy)
	}
	defs = append(defs, chipDef{"dty", tyl, -1, g.dexTyOpen, func() { g.dexTyOpen = !g.dexTyOpen }})
	chipWd := func(d chipDef) float64 {
		w := textW(d.label, 13, false) + 24
		if d.gl >= 0 {
			w += 18
		}
		if d.key == "dty" {
			w += 16
			if g.dexTy != "all" {
				w += 20
			}
		}
		return w
	}
	var rows [][]chipDef
	var rw []float64
	for _, d := range defs {
		w := chipWd(d)
		if len(rows) == 0 || rw[len(rw)-1]+w+6 > 700 {
			rows = append(rows, nil)
			rw = append(rw, -6)
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], d)
		rw[len(rw)-1] += w + 6
	}
	cy := 112.0
	var tyBtn [4]float64
	for ri, row := range rows {
		cx := sw/2 - rw[ri]/2
		for _, d := range row {
			d := d
			w := chipWd(d)
			r := [4]float64{cx, cy, w, 30}
			if d.key == "dty" {
				tyBtn = r
			}
			u.button(d.key, r, false, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
				chipBtn(dst, rr, d.on, hot, func(x, y float64) {
					if d.key == "dty" && g.dexTy != "all" {
						g.drawTypeIcon(dst, g.dexTy, x+8, y, 16, false)
						x += 20
					}
					if d.gl >= 0 {
						drawGlyph(dst, d.gl, x+7, y, 14, white)
						x += 18
					}
					drawText(dst, d.label, x, y-8, 13, false, webFg, text.AlignStart, false)
					if d.key == "dty" {
						poly(dst, [][2]float64{{rr[0] + rr[2] - 20, y - 2}, {rr[0] + rr[2] - 12, y - 2}, {rr[0] + rr[2] - 16, y + 3}}, webFg)
					}
				})
			}, d.act)
			cx += w + 6
		}
		cy += 36
	}
	// type picker popup (its buttons first so they get clicks; drawn last, on top)
	var popup [4]float64
	if g.dexTyOpen {
		mx, my := ebiten.CursorPosition()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			pw, ph := 6*104.0+14, float64((len(typeOrder)+6)/6)*34+14
			pr := [4]float64{math.Min(sw-pw-10, math.Max(10, tyBtn[0]+tyBtn[2]/2-pw/2)), tyBtn[1] + 34, pw, ph}
			if !inR(pr, float64(mx), float64(my)) && !inR(tyBtn, float64(mx), float64(my)) {
				g.dexTyOpen = false
			}
		}
		if keyJust(ebiten.KeyEscape) {
			g.dexTyOpen = false
			u.swallow = true
		}
	}
	if g.dexTyOpen {
		opts := append([]string{"all"}, typeOrder...)
		pw, ph := 6*104.0+14, float64((len(opts)+5)/6)*34+14
		popup = [4]float64{math.Min(sw-pw-10, math.Max(10, tyBtn[0]+tyBtn[2]/2-pw/2)), tyBtn[1] + 34, pw, ph}
		for i, t := range opts {
			t := t
			r := [4]float64{popup[0] + 7 + float64(i%6)*104, popup[1] + 7 + float64(i/6)*34, 100, 30}
			u.button("dt:"+t, r, false, false, nil, func() {
				g.dexTy = t
				g.dexTyOpen = false
				u.scroll = 0
			})
		}
	}
	// grid entries
	q := g.dexQ
	pass := func(id string) bool {
		if !g.srchOK(id, q) || !is(g.dexSt, id) {
			return false
		}
		if g.dexTy == "all" {
			return true
		}
		return contains(g.db.ByID[id].Types, g.dexTy)
	}
	var L []string
	for _, b := range all {
		if g.db.ByID[b].RegionalOf != "" {
			continue
		}
		sel := g.dexForm[b]
		if sel == "" {
			sel = b
		}
		f := ""
		if pass(sel) {
			f = sel
		} else {
			for _, x := range g.dexForms(b) {
				if pass(x) {
					f = x
					break
				}
			}
		}
		if f != "" {
			L = append(L, f)
		}
	}
	if g.dexSo == "num" {
		sort.SliceStable(L, func(i, j int) bool {
			a, _ := strconv.Atoi(g.dexNo(L[i]))
			b, _ := strconv.Atoi(g.dexNo(L[j]))
			return a < b
		})
	}
	// scrolling content: detail panel, then the grid
	top := cy + 8
	u.area = [4]float64{0, top, sw, sh - top}
	x0 := sw/2 - dexW/2
	y := 6.0
	if g.dexSel != "" && g.db.ByID[g.dexSel] != nil {
		dh, hits := g.dexDetail(nil, x0, y, dexW)
		for _, h := range hits {
			h := h
			u.button(h.key, h.r, true, h.disabled, nil, h.act)
		}
		yd := y
		u.label([4]float64{x0, yd, dexW, dh}, true, func(dst *ebiten.Image, r [4]float64) {
			foc := g.ui.focus
			rel := ""
			for _, pf := range []string{"df:", "dbuy", "dsel", "de:", "dl:"} {
				if strings.HasPrefix(foc, pf) {
					rel = foc
				}
			}
			key := fmt.Sprintf("dd|%s|%s|%s|%.0f|%d", g.dexSel, g.csel, rel, dh, g.saveGen)
			im := g.cacheImg(key, int(dexW), int(dh), func(img *ebiten.Image) {
				webBox(img, 0, 0, dexW, dh, 12, webBorder, webPanel)
				g.dexDetail(img, 0, 0, dexW)
			})
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(r[0], r[1])
			dst.DrawImage(im, op)
			if g.prof.Unl(g.dexSel) {
				g.drawPortrait(dst, g.dexSel, r[0]+14, r[1]+14, 84, false, true, true)
			}
		})
		y += dh + 12
	}
	cols := int(math.Floor((dexW + 8) / (112 + 8)))
	cw := (dexW - float64(cols-1)*8) / float64(cols)
	ch := dexCardH(false)
	for i, id := range L {
		id := id
		r := [4]float64{x0 + float64(i%cols)*(cw+8), y + float64(i/cols)*(ch+8), cw, ch}
		u.button("dc:"+id, r, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			key := fmt.Sprintf("dc|%s|%v|%v|%.0f|%d", id, id == g.dexSel, hot, cw, g.saveGen)
			im := g.cacheImg(key, int(math.Ceil(cw)), int(ch), func(img *ebiten.Image) {
				g.drawDexCard(img, id, [4]float64{0, 0, cw, ch}, false, hot)
			})
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(rr[0], rr[1])
			dst.DrawImage(im, op)
			if id == g.dexSel && g.prof.Unl(id) {
				g.drawPortrait(dst, id, rr[0]+rr[2]/2-32, rr[1]+10, 64, false, true, true)
			}
		}, func() { g.dexClick(id) })
	}
	if len(L) == 0 {
		u.label([4]float64{0, y + 10, sw, 20}, true, textDraw("No Pokémon match that search.", 14, false, alpha(webFg, .7), text.AlignCenter))
	}
	rows2 := (len(L) + cols - 1) / cols
	y += float64(rows2)*(ch+8) + 10
	u.scrollMax = math.Max(0, y-u.area[3])
	if g.dexTyOpen {
		opts := append([]string{"all"}, typeOrder...)
		pp := popup
		u.label(pp, false, func(dst *ebiten.Image, r [4]float64) {
			soft(dst, r[0]+r[2]/2, r[1]+r[3]/2+6, r[2]*.6, r[3]*.7, c(0, 0, 0, 160))
			webBox(dst, r[0], r[1], r[2], r[3], 12, webBorder, webBg)
			for i, t := range opts {
				br := [4]float64{r[0] + 7 + float64(i%6)*104, r[1] + 7 + float64(i/6)*34, 100, 30}
				on := g.dexTy == t
				chipBtn(dst, br, on, g.ui.focus == "dt:"+t, func(x, y float64) {
					l := "All types"
					if t != "all" {
						g.drawTypeIcon(dst, t, x+8, y, 16, false)
						x += 20
						l = cap1s(t)
					}
					drawText(dst, l, x, y-8, 13, false, webFg, text.AlignStart, false)
				})
			}
		})
	}
}

// DexSelect / ScrollTo set up a screen for screenshots and tests.
func (g *Game) DexSelect(id string) {
	if g.db.ByID[id] != nil {
		g.dexSel = id
	}
}

func (g *Game) ScrollTo(y float64) { g.ui.scroll = y; g.ui.scrollMax = y }
