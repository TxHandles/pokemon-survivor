package client

import (
	"fmt"
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Collection and Lifetime stats, ported from the web version (coll() / life()).

const (
	scColl screen = iota + 170
	scLife
)

// chipItem is a web ".chips button" with an optional second line.
type chipItem struct {
	key, label, sub string
	col             color.NRGBA
	on              bool
	act             func()
}

// chipFlow lays out chips in centred rows at content y (in the scrolling area); returns the height.
func (g *Game) chipFlow(items []chipItem, y, maxW float64) float64 {
	sw := float64(g.sw)
	h := 30.0
	for _, it := range items {
		if it.sub != "" {
			h = 46
		}
	}
	type pl struct {
		it  chipItem
		w   float64
		row int
	}
	var ps []pl
	var rw []float64
	for _, it := range items {
		w := math.Max(textW(it.label, 13, false), textW(it.sub, 12, false)) + 22
		if len(rw) == 0 || rw[len(rw)-1]+w+6 > maxW {
			rw = append(rw, -6)
		}
		rw[len(rw)-1] += w + 6
		ps = append(ps, pl{it, w, len(rw) - 1})
	}
	x, cur := 0.0, -1
	for _, p := range ps {
		if p.row != cur {
			cur = p.row
			x = sw/2 - rw[cur]/2
		}
		p := p
		r := [4]float64{x, y + float64(p.row)*(h+6), p.w, h}
		g.ui.button(p.it.key, r, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
			border := webBorder
			if p.it.on || hot {
				border = b2c(p.it.on, webAcc, mix(webBorder, webAcc, .5))
			}
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, border, webPanel)
			col := p.it.col
			if col.A == 0 {
				col = webFg
			}
			cx := rr[0] + rr[2]/2
			if p.it.sub == "" {
				drawText(dst, p.it.label, cx, rr[1]+rr[3]/2-8, 13, false, col, text.AlignCenter, false)
				return
			}
			drawText(dst, p.it.label, cx, rr[1]+6, 13, false, col, text.AlignCenter, false)
			drawText(dst, p.it.sub, cx, rr[1]+25, 12, false, alpha(col, .75), text.AlignCenter, false)
		}, p.it.act)
		x += p.w + 6
	}
	return float64(len(rw))*(h+6) - 6
}

// learnable: every move some Pokémon can learn (web LRNALL).
func (g *Game) learnable() map[string]bool {
	out := map[string]bool{}
	for _, l := range g.db.Learnsets {
		for _, m := range l.Moves {
			out[m] = true
		}
		for _, ms := range l.SecondType {
			for _, m := range ms {
				out[m] = true
			}
		}
	}
	return out
}

func itemName(id string) string {
	for _, it := range sim.Items {
		if it.ID == id {
			return it.Name
		}
	}
	return id
}

func (g *Game) menuColl() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	g.webHeaderParts("Collection", "", scMenu, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	if g.collSel == "" {
		g.collSel = g.db.ByID[g.csel].Types[0]
	}
	col, _ := pr.S.M["col"].(map[string]any)
	flag := func(id, f string) bool {
		m, _ := col[id].(map[string]any)
		return num(m[f]) > 0
	}
	all := g.learnable()
	nd, ne, nc := 0, 0, 0
	for id := range col {
		if g.db.Moves[id] == nil {
			continue
		}
		if flag(id, "d") {
			nd++
		}
		if flag(id, "e") {
			ne++
		}
		if flag(id, "c") {
			nc++
		}
	}
	y := 4.0
	y += g.para(fmt.Sprintf("Moves discovered %d/%d · Evolved %d/%d · Capstones %d/%d. Milestone rewards are claimed in Challenges.", nd, len(all), ne, len(all), nc, len(all)), y, 480, 14, false, alpha(webFg, .9)) + 10
	// one chip per type (labelled with the base Pokémon, like the web version)
	var chips []chipItem
	for _, p := range g.db.Pokemon {
		if p.BaseStats == nil {
			continue
		}
		t := p.ID
		n, d := 0, 0
		for _, m := range g.db.MoveList {
			if m.Type == t {
				n++
				if flag(m.ID, "d") {
					d++
				}
			}
		}
		chips = append(chips, chipItem{key: "ct:" + t, label: p.Name, sub: fmt.Sprintf("%d/%d found", d, n), col: tc(t), on: t == g.collSel, act: func() { g.collSel = t }})
	}
	y += g.chipFlow(chips, y, 700) + 16
	y += g.h3("Moves", y)
	y += g.para("A move can evolve at Lv 10 while you hold its synergy item (listed on each card). Evolved moves unlock Lv 11–20 and a capstone at Lv 20. Each level of the synergy item adds +8% evolved damage.", y, 480, 12, false, alpha(webFg, .8)) + 10
	var cards []*webCard
	for _, m := range g.db.MoveList {
		if m.Type != g.collSel {
			continue
		}
		c := &webCard{key: "cm:" + m.ID, minW: 220, maxW: 220}
		if flag(m.ID, "d") {
			c.title, c.titleCol = m.Name, tc(m.Type)
			c.lines = []string{m.Desc, "Evolves at Lv 10 holding " + itemName(m.HeldItem)}
			st := []any{"● Found"}
			if flag(m.ID, "e") {
				st = append(st, " · ", gSparkle, "Evolved")
			}
			if flag(m.ID, "c") {
				st = append(st, " · ", gMedal, "Capstone")
			}
			c.price = st
		} else {
			c.title = "???"
			c.disabled = true
			if all[m.ID] {
				c.lines = []string{"Equip this move during a run to discover it."}
				c.price = []any{"○ Not found"}
			} else {
				c.lines = []string{"No current Pokémon learns this move yet."}
				c.price = []any{"○ Coming later"}
			}
		}
		cards = append(cards, c)
	}
	y += g.flowCards(cards, y, 1150) + 18
	y += g.h3("Held items (all characters)", y)
	ci, _ := pr.S.M["ci"].(map[string]any)
	var ic []*webCard
	for _, it := range sim.Items {
		l := int(num(ci[it.ID]))
		c := &webCard{key: "ci:" + it.ID, minW: 173, maxW: 173}
		if l > 0 {
			c.title, c.lines = it.Name, []string{it.Desc}
			s := fmt.Sprintf("Best level: %d/10", l)
			c.price = []any{s}
			if l >= 10 {
				c.price = []any{s + " ", gCheck}
			}
		} else {
			c.title, c.lines, c.price, c.disabled = "???", []string{"Pick this item during a run."}, []any{"○ Not found"}, true
		}
		ic = append(ic, c)
	}
	y += g.flowCards(ic, y, 1100) + 20
	u.scrollMax = math.Max(0, y-u.area[3])
}

func (g *Game) menuLife() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	g.webHeaderParts("Lifetime stats", "", scMenu, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	if g.lifeSel == "" {
		g.lifeSel = "all"
	}
	cs, _ := pr.S.M["cs"].(map[string]any)
	csOf := func(id string) map[string]any { m, _ := cs[id].(map[string]any); return m }
	y := 4.0
	chips := []chipItem{{key: "lc:all", label: "All characters", sub: fmt.Sprintf("%d played", len(cs)), col: white, on: g.lifeSel == "all", act: func() { g.lifeSel = "all" }}}
	for _, p := range g.db.Pokemon {
		id := p.ID
		chips = append(chips, chipItem{key: "lc:" + id, label: p.Name, sub: fmt.Sprintf("%d runs", int(num(csOf(id)["r"]))), col: tc(p.Types[0]), on: g.lifeSel == id, act: func() { g.lifeSel = id }})
	}
	y += g.chipFlow(chips, y, 720) + 14
	var A []map[string]any
	if g.lifeSel == "all" {
		for _, v := range cs {
			if m, ok := v.(map[string]any); ok {
				A = append(A, m)
			}
		}
	} else if m := csOf(g.lifeSel); m != nil {
		A = append(A, m)
	}
	sum := func(k string) float64 {
		t := 0.0
		for _, c := range A {
			t += num(c[k])
		}
		return t
	}
	mx := func(k string) float64 {
		t := 0.0
		for _, c := range A {
			t = math.Max(t, num(c[k]))
		}
		return t
	}
	fn := func(x float64) string { return commas(int(math.Round(x))) }
	ft := func(x float64) string {
		s := int(math.Round(x))
		if s >= 3600 {
			return fmt.Sprintf("%dh %dm", s/3600, s%3600/60)
		}
		return fmt.Sprintf("%dm %ds", s/60, s%60)
	}
	runs := sum("r")
	vic := fn(sum("w"))
	if runs > 0 {
		vic += fmt.Sprintf(" (%d%%)", int(math.Round(sum("w")/runs*100)))
	}
	avg := "—"
	if runs > 0 {
		avg = ft(sum("tm") / runs)
	}
	rows := [][2]string{{"Runs played", fn(runs)}, {"Victories", vic}, {"Time played", ft(sum("tm"))}, {"Longest run", ft(mx("bt"))}, {"Average run", avg},
		{"Enemies defeated", fn(sum("k"))}, {"Bosses defeated", fn(sum("bk"))}, {"Elites & mini-bosses", fn(sum("ek"))}, {"Coins earned", fn(sum("g"))},
		{"Highest level", fn(mx("lv"))}, {"Damage dealt", fn(sum("dm"))}}
	if g.lifeSel != "all" {
		rows = append(rows, [2]string{"Mastery rank", fmt.Sprintf("%d / %d", pr.Training(pr.Fam(g.lifeSel)), 20)})
	} else {
		col, _ := pr.S.M["col"].(map[string]any)
		d := 0
		for id, v := range col {
			if m, ok := v.(map[string]any); ok && g.db.Moves[id] != nil && num(m["d"]) > 0 {
				d++
			}
		}
		rows = append(rows, [2]string{"Moves discovered", fmt.Sprintf("%d / %d", d, len(g.learnable()))})
	}
	gw := math.Min(760, sw*.94)
	cols := 3
	cw := (gw - 16) / 3
	for i, r := range rows {
		r := r
		x := sw/2 - gw/2 + float64(i%cols)*(cw+8)
		u.label([4]float64{x, y + float64(i/cols)*48, cw, 40}, true, func(dst *ebiten.Image, rr [4]float64) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, webBorder, webPanel)
			drawText(dst, r[0], rr[0]+14, rr[1]+10, 16, false, alpha(webFg, .8), text.AlignStart, false)
			drawText(dst, r[1], rr[0]+rr[2]-14, rr[1]+10, 16, true, webFg, text.AlignEnd, false)
		})
	}
	y += float64((len(rows)+cols-1)/cols)*48 + 16
	y += g.h3("Top attacks by damage", y)
	md := map[string]float64{}
	for _, c := range A {
		if m, ok := c["md"].(map[string]any); ok {
			for id, v := range m {
				md[id] += num(v)
			}
		}
	}
	type mv struct {
		id string
		v  float64
	}
	var top []mv
	for id, v := range md {
		if g.db.Moves[id] != nil {
			top = append(top, mv{id, v})
		}
	}
	sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
	if len(top) > 8 {
		top = top[:8]
	}
	if len(top) == 0 {
		u.label([4]float64{0, y, sw, 20}, true, textDraw("No data yet.", 14, false, alpha(webFg, .7), text.AlignCenter))
		y += 26
	}
	for _, t := range top {
		t := t
		tm := top[0].v
		u.label([4]float64{sw/2 - 245, y, 490, 19}, true, func(dst *ebiten.Image, rr [4]float64) {
			m := g.db.Moves[t.id]
			drawText(dst, fitText(m.Name, 13, false, 150), rr[0]+75, rr[1], 13, false, webFg, text.AlignCenter, false)
			bw := math.Max(2, t.v/tm*200)
			roundRect(dst, rr[0]+160, rr[1]+5, bw, 8, 4, tc(m.Type))
			drawText(dst, commas(int(t.v)), rr[0]+166+bw, rr[1], 13, false, alpha(webFg, .8), text.AlignStart, false)
		})
		y += 19
	}
	y += 12
	y += g.para("Totals include every run logged since your fresh start. Damage and attack totals count each run's top 10 attacks.", y, 480, 12, false, alpha(webFg, .65)) + 20
	u.scrollMax = math.Max(0, y-u.area[3])
}
