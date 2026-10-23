package client

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/meta"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

const (
	scDex screen = iota + 140
	scHist
	scDaily
	scBoards
)

func stripHTML(s string) string {
	out := []rune{}
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
			out = append(out, ' ')
		case !in:
			out = append(out, r)
		}
	}
	return string(out)
}

// ---- run history ----

func (g *Game) menuHist() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	hist := g.histRuns()
	g.webHeaderParts("Run history", "", scMenu, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	wins, best, tot := 0, 0.0, 0.0
	for _, h := range hist {
		if num(h["w"]) > 0 {
			wins++
		}
		best = math.Max(best, num(h["tm"]))
		tot += num(h["tm"])
	}
	sub := "No runs logged yet. Finish a run and it will appear here."
	if len(hist) > 0 {
		sub = fmt.Sprintf("%d run%s logged · %d win%s · average %s · best %s · last 150 runs are kept", len(hist), plural(len(hist)), wins, plural(wins), fmtT(tot/float64(len(hist))), fmtT(best))
		if g.cloud.Online() {
			sub += " and synced"
		}
	}
	y := 4.0
	y += g.para(sub, y, 480, 14, false, alpha(webFg, .9)) + 12
	y += g.h3("Move performance (damage per second, vs the median move)", y)
	y += g.para("Needs 2+ runs per move. ▲ much stronger than typical, ▼ much weaker. Trend compares your recent runs to earlier ones.", y, 480, 12, false, alpha(webFg, .7)) + 8
	// per-move performance (web hist(): mean dps over runs that used the move, trend = late vs early half)
	type use struct {
		dps float64
		w   bool
		ev  bool
	}
	per := map[string][]use{}
	var order []string
	for _, h := range hist {
		d, _ := h["d"].(map[string]any)
		ld, _ := h["ld"].([]any)
		for _, x := range ld {
			l, _ := x.([]any)
			if len(l) < 3 {
				continue
			}
			id, _ := l[0].(string)
			if g.db.Moves[id] == nil {
				continue
			}
			if per[id] == nil {
				order = append(order, id)
			}
			per[id] = append(per[id], use{num(d[id]) / math.Max(1, num(h["tm"])), num(h["w"]) > 0, num(l[2]) > 0})
		}
	}
	type row struct {
		id    string
		n, ev int
		m, wr float64
		tr    float64
		hasTr bool
	}
	var rows []row
	for _, id := range order {
		a := per[id]
		if len(a) < 2 {
			continue
		}
		r := row{id: id, n: len(a)}
		w := 0
		for _, x := range a {
			r.m += x.dps
			if x.w {
				w++
			}
			if x.ev {
				r.ev++
			}
		}
		r.m /= float64(len(a))
		r.wr = float64(w) / float64(len(a))
		if len(a) >= 4 {
			h := len(a) / 2
			e, l := 0.0, 0.0
			for _, x := range a[:h] {
				e += x.dps
			}
			for _, x := range a[h:] {
				l += x.dps
			}
			e /= float64(h)
			l /= float64(len(a) - h)
			if e > 0 {
				r.tr, r.hasTr = l/e-1, true
			}
		}
		rows = append(rows, r)
	}
	med := 1.0
	if len(rows) > 0 {
		ms := make([]float64, len(rows))
		for i, r := range rows {
			ms[i] = r.m
		}
		sort.Float64s(ms)
		if ms[len(ms)/2] > 0 {
			med = ms[len(ms)/2]
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].m > rows[j].m })
	rw := math.Min(720, sw*.94)
	x0 := sw/2 - rw/2
	if len(rows) == 0 {
		y += g.para("Not enough data yet.", y, 480, 14, false, alpha(webFg, .7)) + 8
	}
	for _, r := range rows {
		r := r
		mv := g.db.Moves[r.id]
		h := 40.0
		if r.hasTr {
			h = 50
		}
		u.label([4]float64{x0, y, rw, h}, true, func(dst *ebiten.Image, rr [4]float64) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, webBorder, webPanel)
			drawText(dst, mv.Name, rr[0]+14, rr[1]+rr[3]/2-9, 16, true, tc(mv.Type), text.AlignStart, false)
			drawText(dst, fmt.Sprintf("%d runs · %d%% won · evolved in %d", r.n, int(math.Round(r.wr*100)), r.ev), rr[0]+174, rr[1]+rr[3]/2-7, 12, false, alpha(webFg, .75), text.AlignStart, false)
			q := r.m / med
			col := hexC("#ccddee")
			var mark []any
			if q >= 1.6 {
				col, mark = hexC("#ff9a6b"), []any{" ", gTriUp}
			} else if q <= .5 {
				col, mark = hexC("#7fb0ff"), []any{" ", gTriDown}
			}
			parts := append([]any{fmt.Sprintf("%.1f/s · ×%.2f", r.m, q)}, mark...)
			ty := rr[1] + rr[3]/2 - 9
			if r.hasTr {
				ty = rr[1] + 8
			}
			w := richLine(nil, 0, 0, 16, false, col, parts...)
			richLine(dst, rr[0]+rr[2]-14-w, ty, 16, false, col, parts...)
			if r.hasTr {
				arrow := "↑"
				if r.tr < 0 {
					arrow = "↓"
				}
				drawText(dst, fmt.Sprintf("%s%d%%", arrow, int(math.Abs(math.Round(r.tr*100)))), rr[0]+rr[2]-14, rr[1]+28, 11, false, alpha(webFg, .75), text.AlignEnd, false)
			}
		})
		y += h + 4
	}
	y += 14
	y += g.h3("Recent runs", y)
	for i := len(hist) - 1; i >= 0 && i >= len(hist)-20; i-- {
		h := hist[i]
		cid, _ := h["c"].(string)
		name := cid
		col := white
		if p := g.db.ByID[cid]; p != nil {
			name, col = p.Name, tc(p.Types[0])
		}
		md, _ := h["md"].([]any)
		top := ""
		if d, ok := h["d"].(map[string]any); ok {
			bk, bv := "", 0.0
			for k, v := range d {
				if num(v) > bv || (num(v) == bv && k < bk) {
					bk, bv = k, num(v)
				}
			}
			if m := g.db.Moves[bk]; m != nil {
				top = " · top: " + m.Name
			}
		}
		when := time.UnixMilli(int64(num(h["ts"]))).Format("1/2/2006")
		l1 := fmt.Sprintf("%s · Difficulty %d", when, int(num(h["tr"])))
		if sg := int(num(h["sg"])); sg > 0 {
			l1 += fmt.Sprintf(" · Tier %d", sg)
		}
		if len(md) > 0 {
			l1 += fmt.Sprintf(" · %d mod%s", len(md), plural(len(md)))
		}
		l1 += top
		if num(h["go"]) > 0 {
			l1 += " · Go build"
		}
		win := num(h["w"]) > 0
		r1 := fmt.Sprintf("%s · Lv %d · %d kills", fmtT(num(h["tm"])), int(num(h["lv"])), int(num(h["k"])))
		u.label([4]float64{x0, y, rw, 40}, true, func(dst *ebiten.Image, rr [4]float64) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, webBorder, webPanel)
			drawText(dst, name, rr[0]+14, rr[1]+11, 15, true, col, text.AlignStart, false)
			drawText(dst, fitText(l1, 12, false, rr[2]-14-110-250), rr[0]+110, rr[1]+13, 12, false, alpha(webFg, .75), text.AlignStart, false)
			parts := []any{r1}
			if win {
				parts = append(parts, " ", webAcc, gTrophy)
			}
			w := richLine(nil, 0, 0, 14, false, webFg, parts...)
			richLine(dst, rr[0]+rr[2]-14-w, rr[1]+11, 14, false, webFg, parts...)
		})
		y += 44
	}
	y += 16
	y += g.h3("Export", y)
	cl := "Copy run data"
	if g.histCopy != "" {
		cl = g.histCopy
	}
	cw := smallBtnW(cl)
	u.button("hcopy", [4]float64{sw/2 - cw/2, y, cw, 30}, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
		smallBtn(dst, rr, cl, hot, false)
	}, func() {
		b, _ := json.Marshal(g.prof.S.M["hist"])
		if len(b) == 0 || string(b) == "null" {
			b = []byte("[]")
		}
		if clipWrite(string(b)) {
			g.histCopy = "Copied!"
		} else {
			g.histCopy = "Couldn't reach the clipboard"
		}
	})
	y += 46
	u.scrollMax = math.Max(0, y-u.area[3])
}

func biomeList(h map[string]any) string {
	bv, _ := h["bv"].([]any)
	s := ""
	for _, b := range bv {
		if t, ok := b.(string); ok {
			if s != "" {
				s += ", "
			}
			s += biomeName(t)
		}
	}
	return s
}

func (g *Game) histRuns() []map[string]any {
	l, _ := g.prof.S.M["hist"].([]any)
	var out []map[string]any
	for _, x := range l {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// ---- daily run and leaderboards ----

func (g *Game) menuDaily() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	D := meta.DailyDef(time.Now())
	lb := g.prof.LB()
	g.webHeaderParts("Daily Run", "", scMenu, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	cw := math.Min(596, sw*.94)
	x := sw/2 - cw/2
	loc := g.locName(D.Biome, 0)
	var modParts []any
	modParts = append(modParts, "Modifiers: ")
	for i, k := range D.Mods {
		for _, m := range meta.Mods {
			if m.Key == k {
				if i > 0 {
					modParts = append(modParts, false, ", ")
				}
				modParts = append(modParts, true, m.Name, false, " ("+m.Desc+")")
			}
		}
	}
	desc := wrap("Everyone gets the same biome, modifiers and world today. Any character works. Your best run counts on the Daily, Weekly and All-time boards.", font(13, false), cw-40)
	ch := 150 + float64(len(desc))*16
	y := 0.0
	u.label([4]float64{x, y, cw, ch}, true, func(dst *ebiten.Image, r [4]float64) {
		webBox(dst, r[0], r[1], r[2], r[3], 12, webBorder, webPanel)
		cx := r[0] + r[2]/2
		center := func(yy float64, size int, parts ...any) {
			w := richLine(nil, 0, 0, size, false, webFg, parts...)
			richLine(dst, cx-w/2, yy, size, false, webFg, parts...)
		}
		center(r[1]+12, 18, gCalendar, true, D.Key)
		drawText(dst, "Resets in "+meta.UntilReset(time.Now())+" (UTC midnight)", cx, r[1]+38, 13, false, alpha(webFg, .9), text.AlignCenter, false)
		center(r[1]+58, 16, gGlobe, tc(D.Biome), true, loc, webFg, false, " · Difficulty 2 · 15:00 run")
		center(r[1]+82, 15, modParts...)
		for i, l := range desc {
			drawText(dst, l, cx, r[1]+106+float64(i)*16, 13, false, alpha(webFg, .9), text.AlignCenter, false)
		}
		center(r[1]+r[3]-30, 16, "Best today: ", true, commas(int(lb.Num("dayKills"))), false, " kills · ", true, fmtT(lb.Num("dayTime")), false, " survived")
	})
	y += ch + 10
	p := g.db.ByID[g.csel]
	u.label([4]float64{x, y, cw, 60}, true, func(dst *ebiten.Image, r [4]float64) {
		webBox(dst, r[0], r[1], r[2], r[3], 12, webBorder, webPanel)
		drawText(dst, "Playing as", r[0]+r[2]/2, r[1]+10, 13, false, alpha(webFg, .9), text.AlignCenter, false)
		drawText(dst, p.Name, r[0]+r[2]/2, r[1]+30, 17, true, tc(p.Types[0]), text.AlignCenter, false)
	})
	y += 70
	bw := math.Min(560, sw*.9)
	big := func(key string, gl any, label string, act func()) {
		u.button(key, [4]float64{sw/2 - bw/2, y, bw, 52}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
			webBtnDraw(dst, r, hot, false, false)
			w := richLine(nil, 0, 0, 17, true, webFg, gl, label)
			richLine(dst, r[0]+r[2]/2-w/2, r[1]+15, 17, true, webFg, white, gl, label)
		}, act)
		y += 61
	}
	big("go", gPlay, "Start today's Daily Run", func() { g.startDaily() })
	big("ch", gPad, "Change character", func() { g.scr = scChars; g.ui.focus = ""; g.ui.scroll = 0 })
	big("lb", gMedal, "Leaderboards", func() { g.boardsFrom = scDaily; g.scr = scBoards; g.ui.scroll = 0; g.lbFetch() })
	u.scrollMax = math.Max(0, y-u.area[3])
}

func (g *Game) startDaily() {
	pr := g.prof
	id := g.csel
	if !pr.Unl(id) {
		return
	}
	D := meta.DailyDef(time.Now())
	fam := pr.Fam(id)
	up := map[string]int{}
	for _, k := range meta.Upgrades {
		up[k.Key] = pr.UL(k.Key, fam)
	}
	cfg := sim.Config{Ability: pr.AbilityIdx(id), LI: 0, Up: up, Training: pr.Training(fam), Mastery: pr.MasteryBonus(id), Start: pr.StartMove(id), Tier: D.Tier, Mods: D.Mods, Stage: 1}
	cfg.OnBoss = func(b, et string, l int, fin bool, visited []string) []string {
		m := pr.BossKill(b, et, l, fin, 1, id, visited)
		g.save()
		return m
	}
	g.startCfg(id, D.Biome, D.Seed, cfg)
	g.daily = &D
	g.finished = false
}

func (g *Game) menuBoards() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	D := meta.DailyDef(time.Now())
	back := g.boardsFrom
	if back == 0 {
		back = scMenu
	}
	g.webHeaderParts("Leaderboards", "", back, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	mods := ""
	for i, k := range D.Mods {
		for _, m := range meta.Mods {
			if m.Key == k {
				if i > 0 {
					mods += " + "
				}
				mods += m.Name
			}
		}
	}
	y := 4.0
	y += g.para("Only Daily Run attempts count, so everyone plays the same setup. Today: "+g.locName(D.Biome, 0)+", "+mods+". "+g.lbNote, y, 480, 13, false, alpha(webFg, .85)) + 10
	var tch []chipItem
	for _, t := range []string{"day", "week", "all"} {
		t := t
		tch = append(tch, chipItem{key: "t:" + t, label: map[string]string{"day": "Daily", "week": "Weekly", "all": "All-time"}[t], on: g.lbT == t, act: func() { g.lbT = t; g.lbFetch() }})
	}
	y += g.chipFlow(tch, y, 600) + 10
	var mch []chipItem
	for _, m := range []string{"k", "t"} {
		m := m
		mch = append(mch, chipItem{key: "m:" + m, label: map[string]string{"k": "Most kills", "t": "Longest survived"}[m], on: g.lbM == m, act: func() { g.lbM = m; g.lbFetch() }})
	}
	y += g.chipFlow(mch, y, 600) + 14
	rw := math.Min(640, sw*.94)
	x0 := sw/2 - rw/2
	if len(g.lbRows) == 0 {
		u.label([4]float64{x0, y, rw, 20}, true, textDraw("No runs yet for this board. Play today's Daily Run to get on it!", 15, false, alpha(webFg, .8), text.AlignStart))
		y += 30
	}
	for i, r := range g.lbRows {
		r := r
		rank := strconv.Itoa(i + 1)
		p := g.db.ByID[r.Run.C]
		name, col := r.Run.C, white
		if p != nil {
			name, col = p.Name, tc(p.Types[0])
		}
		mv := ""
		for _, m := range r.Run.MV {
			if mm := g.db.Moves[m.ID]; mm != nil {
				if mv != "" {
					mv += " · "
				}
				mv += fmt.Sprintf("%s %d%s", mm.Name, m.Lv, b2s(m.Evo, " (evolved)", ""))
			}
		}
		val, sub := commas(r.V)+" kills", fmtT(float64(r.Run.Tm))
		if g.lbM == "t" {
			val, sub = fmtT(float64(r.V)), commas(r.Run.K)+" kills"
		}
		u.label([4]float64{x0, y, rw, 50}, true, func(dst *ebiten.Image, rr [4]float64) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, b2c(r.Me, webAcc, webBorder), webPanel)
			if i < 3 {
				drawGlyph(dst, gMedal, rr[0]+22, rr[1]+25, 20, white)
			} else {
				drawText(dst, rank, rr[0]+22, rr[1]+15, 16, true, alpha(webFg, .7), text.AlignCenter, false)
			}
			drawText(dst, r.Name, rr[0]+46, rr[1]+8, 15, true, webFg, text.AlignStart, false)
			drawText(dst, fitText(name+" · "+g.locName(r.Run.Bi, 0)+" · "+mv, 12, false, rr[2]-46-140), rr[0]+46, rr[1]+28, 12, false, col, text.AlignStart, false)
			drawText(dst, val, rr[0]+rr[2]-14, rr[1]+8, 16, true, webFg, text.AlignEnd, false)
			drawText(dst, sub, rr[0]+rr[2]-14, rr[1]+29, 12, false, alpha(webFg, .75), text.AlignEnd, false)
		})
		y += 56
	}
	u.scrollMax = math.Max(0, y+10-u.area[3])
}

// lbFetch loads the current board (online when signed in, else your own best).
func (g *Game) lbFetch() {
	if g.lbT == "" {
		g.lbT, g.lbM = "day", "k"
	}
	D := meta.DailyDef(time.Now())
	g.lbNote = "Daily resets in " + meta.UntilReset(time.Now()) + "."
	_ = D
	field := g.lbT + map[string]string{"k": "Kills", "t": "Time"}[g.lbM]
	local := func() {
		lb := g.prof.LB()
		g.lbRows = nil
		if r := meta.ParseLBRun(lb.M[field+"Run"]); r != nil && lb.Num(field) > 0 {
			g.lbRows = []meta.LBRow{{Name: g.lbName(), V: int(lb.Num(field)), Run: *r, Me: true}}
		}
		g.lbNote += " · Offline: showing only your own best (" + g.cloud.Status() + ")."
	}
	if !g.cloud.Online() {
		local()
		return
	}
	g.lbRows = nil
	g.lbNote += " · Loading…"
	g.cloud.FetchBoard(g.lbT, field, D.Key, func(rows []meta.LBRow, err error) {
		if err != nil {
			local()
			return
		}
		for i := range rows {
			rows[i].Me = rows[i].UID == g.cloud.UID()
		}
		g.lbRows = rows
		g.lbNote = "Daily resets in " + meta.UntilReset(time.Now()) + "."
	})
}

func (g *Game) lbName() string {
	if n := g.opt("name"); n != 0 {
		_ = n
	}
	if o, ok := g.prof.S.M["opt"].(map[string]any); ok {
		if s, ok := o["name"].(string); ok && s != "" {
			return s
		}
	}
	if u := g.cloud.UID(); u != "" && len(u) >= 4 {
		return "Player " + u[len(u)-4:]
	}
	return "You"
}

var _ = rand.Int
