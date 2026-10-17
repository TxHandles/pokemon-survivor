package client

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/meta"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

const (
	scDex screen = iota + 140
	scHist
	scDaily
	scBoards
)

// ---- Pokédex ----

func (g *Game) menuDex() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	pr := g.prof
	n, have := 0, 0
	for _, p := range g.db.Pokemon {
		n++
		if pr.Unl(p.ID) {
			have++
		}
	}
	g.header("POKÉDEX", fmt.Sprintf("%d / %d collected · evolutions unlock with Mastery, regional forms with special boss wins", have, n))
	g.backButton(scMenu)
	if g.dexSel == "" {
		g.dexSel = g.csel
	}
	// group by evolution line, in roster order
	lines := map[string][]*content.Pokemon{}
	var order []string
	for _, p := range g.db.Pokemon {
		r := g.db.LineRoot(p.ID)
		if _, ok := lines[r]; !ok {
			order = append(order, r)
		}
		lines[r] = append(lines[r], p)
	}
	gw := sw - 470
	u.area = [4]float64{20, 96, gw, sh - 116}
	cell := 86.0
	y := 0.0
	x := 20.0
	for _, r := range order {
		if x+float64(len(lines[r]))*cell > 20+gw-10 && x > 20 { // keep a line on one row
			x = 20
			y += cell
		}
		for _, p := range lines[r] {
			p := p
			id := p.ID
			lk := !pr.Unl(id)
			u.button("d:"+id, [4]float64{x, y, cell - 6, cell - 6}, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
				g.charCell(id, lk)(dst, rr, hot || id == g.dexSel)
				drawText(dst, fmt.Sprintf("#%d", p.Dex), rr[0]+6, rr[1]+4, 9, true, dim, text.AlignStart, false)
			}, func() { g.dexSel = id })
			x += cell
		}
		x += 14
	}
	if x > 20 {
		y += cell
	}
	u.scrollMax = math.Max(0, y-u.area[3])
	g.dexPanel(sw-440, 96, 420, sh-116)
}

func (g *Game) dexPanel(x, y, w, h float64) {
	u := &g.ui
	pr := g.prof
	id := g.dexSel
	p := g.db.ByID[id]
	if p == nil {
		return
	}
	lk := !pr.Unl(id)
	col := tc(p.Types[0])
	u.label([4]float64{x, y, w, h}, false, panelDraw(c(20, 25, 38, 240)))
	name := p.Name
	if lk {
		name = "???"
	}
	u.label([4]float64{x + 16, y + 14, w - 32, 24}, false, textDraw(fmt.Sprintf("#%d  %s", p.Dex, name), 22, true, col, text.AlignStart))
	types := ""
	for i, t := range p.Types {
		if i > 0 {
			types += " / "
		}
		types += cap1(t)
	}
	u.label([4]float64{x + 16, y + 44, w - 32, 18}, false, textDraw(types+" type"+tagText(p), 12, false, dim, text.AlignStart))
	if s := g.assets.Sprite(id); s != nil {
		u.label([4]float64{x + 16, y + 70, w - 32, 130}, false, func(dst *ebiten.Image, r [4]float64) {
			fr := s.Front.IdleFrame(g.tick * 1000 / 60)
			k := math.Min(130/float64(s.Front.V.CH), 200/float64(s.Front.V.CW)) * .9
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(r[0]+r[2]/2-float64(s.Front.V.CW)*k/2, r[1])
			op.Filter = ebiten.FilterLinear
			if lk {
				op.ColorScale.Scale(.12, .12, .16, 1)
			}
			dst.DrawImage(s.Front.Frame(fr), op)
		})
	}
	yy := y + 212
	// line
	root := g.db.LineRoot(id)
	line := ""
	for _, q := range g.db.Pokemon {
		if g.db.LineRoot(q.ID) == root {
			if line != "" {
				line += " → "
			}
			if pr.Unl(q.ID) {
				line += q.Name
			} else {
				line += "???"
			}
		}
	}
	if p.RegionalOf != "" {
		line = p.Form + " form of " + g.db.ByID[p.RegionalOf].Name
	}
	for _, ln := range wrap("Line: "+line, font(12, false), w-32) {
		u.label([4]float64{x + 16, yy, w - 32, 16}, false, textDraw(ln, 12, false, dim, text.AlignStart))
		yy += 17
	}
	yy += 6
	u.label([4]float64{x + 16, yy, w - 32, 18}, false, textDraw(pr.UnlockText(id), 13, true, b2c(lk, c(255, 200, 120, 255), c(120, 255, 140, 255)), text.AlignStart))
	yy += 24
	if lk {
		if hv, nd := pr.Progress(id); pr.TicketCost(id) == 0 && nd > 0 {
			u.label([4]float64{x + 16, yy, w - 32, 12}, false, func(dst *ebiten.Image, r [4]float64) {
				bar(dst, r[0], r[1], r[2], 8, float64(hv)/float64(nd), gold, c(40, 46, 60, 255))
				drawText(dst, fmt.Sprintf("%d / %d", hv, nd), r[0]+r[2], r[1]+12, 11, false, dim, text.AlignEnd, false)
			})
			yy += 30
		}
		if cst := pr.TicketCost(id); cst > 0 {
			can := pr.S.Tickets() >= cst
			u.button("buy", [4]float64{x + 16, yy, w - 32, 44}, false, !can, btnDraw(fmt.Sprintf("Unlock for %d ticket%s", cst, plural(cst)), fmt.Sprintf("You have %d", pr.S.Tickets()), gold, !can), func() {
				if pr.BuyUnlock(id) {
					g.toast(p.Name + " unlocked!")
					g.save()
				}
			})
			yy += 52
		}
	} else {
		if p.DexNote != "" {
			for _, ln := range wrap(stripHTML(p.DexNote), font(12, false), w-32) {
				u.label([4]float64{x + 16, yy, w - 32, 16}, false, textDraw(ln, 12, false, dim, text.AlignStart))
				yy += 17
			}
		}
		st := g.db.Stats[id]
		u.label([4]float64{x + 16, yy + 6, w - 32, 16}, false, textDraw(fmt.Sprintf("HP %.2f  ATK %.2f  DEF %.2f  SPD %.2f", st.HP, st.Atk, st.Def, st.Spd), 12, true, white, text.AlignStart))
		yy += 28
		ab := g.db.AbilityPair(id)
		for _, a := range ab {
			for i, ln := range wrap(a.Name+": "+a.Desc, font(11, false), w-32) {
				u.label([4]float64{x + 16, yy, w - 32, 15}, false, textDraw(ln, 11, i == 0, b2c(i == 0, white, dim), text.AlignStart))
				yy += 15
			}
			yy += 4
		}
		if cs, ok := pr.S.M["cs"].(map[string]any)[id].(map[string]any); ok {
			u.label([4]float64{x + 16, yy + 4, w - 32, 16}, false, textDraw(fmt.Sprintf("%d runs · %d wins · best %s · %s defeats", int(num(cs["r"])), int(num(cs["w"])), fmtT(num(cs["bt"])), commas(int(num(cs["k"])))), 12, false, gold, text.AlignStart))
		}
		u.button("sel", [4]float64{x + 16, y + h - 60, w - 32, 44}, false, false, btnDraw("Select "+p.Name, "", gold, false), func() {
			g.csel = id
			g.save()
			g.toast(p.Name + " selected")
		})
	}
}

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
		sub = fmt.Sprintf("%d runs logged · %d wins · average %s · best %s · the last 150 runs are kept", len(hist), wins, fmtT(tot/float64(len(hist))), fmtT(best))
	}
	g.header("RUN HISTORY", sub)
	g.backButton(scMenu)
	u.area = [4]float64{40, 100, sw - 80, sh - 120}
	y := 0.0
	// per-move performance
	type ms struct {
		id          string
		n, ev, wins int
		dps         float64
	}
	per := map[string]*ms{}
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
			m := per[id]
			if m == nil {
				m = &ms{id: id}
				per[id] = m
			}
			m.n++
			m.dps += num(d[id]) / math.Max(1, num(h["tm"]))
			if num(l[2]) > 0 {
				m.ev++
			}
			if num(h["w"]) > 0 {
				m.wins++
			}
		}
	}
	var rows []*ms
	for _, m := range per {
		if m.n >= 2 {
			m.dps /= float64(m.n)
			rows = append(rows, m)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].dps > rows[j].dps })
	if len(rows) > 0 {
		u.label([4]float64{40, y, 400, 20}, true, textDraw("Moves across your runs (damage per second, runs used in)", 14, true, white, text.AlignStart))
		y += 26
		med := 1.0
		if len(rows) > 0 {
			l := append([]*ms{}, rows...)
			sort.Slice(l, func(i, j int) bool { return l[i].dps < l[j].dps })
			med = math.Max(.001, l[len(l)/2].dps)
		}
		for _, m := range rows {
			m := m
			mv := g.db.Moves[m.id]
			u.label([4]float64{40, y, sw - 90, 24}, true, func(dst *ebiten.Image, r [4]float64) {
				roundRect(dst, r[0], r[1], r[2], 22, 6, c(26, 32, 48, 220))
				drawText(dst, mv.Name, r[0]+10, r[1]+4, 12, true, tc(mv.Type), text.AlignStart, false)
				drawText(dst, fmt.Sprintf("%d runs · %d%% won · evolved in %d", m.n, m.wins*100/m.n, m.ev), r[0]+200, r[1]+5, 11, false, dim, text.AlignStart, false)
				q := m.dps / med
				col := c(205, 221, 238, 255)
				if q >= 1.6 {
					col = c(255, 154, 107, 255)
				} else if q <= .5 {
					col = c(127, 176, 255, 255)
				}
				drawText(dst, fmt.Sprintf("%.1f/s  x%.2f", m.dps, q), r[0]+r[2]-10, r[1]+5, 12, true, col, text.AlignEnd, false)
			})
			y += 26
		}
		y += 14
	}
	u.label([4]float64{40, y, 400, 20}, true, textDraw("Last 20 runs", 14, true, white, text.AlignStart))
	y += 26
	for i := len(hist) - 1; i >= 0 && i >= len(hist)-20; i-- {
		h := hist[i]
		cid, _ := h["c"].(string)
		name := cid
		col := white
		if p := g.db.ByID[cid]; p != nil {
			name, col = p.Name, tc(p.Types[0])
		}
		ps, _ := h["ps"].(string)
		md, _ := h["md"].([]any)
		top := ""
		if d, ok := h["d"].(map[string]any); ok {
			bk, bv := "", 0.0
			for k, v := range d {
				if num(v) > bv {
					bk, bv = k, num(v)
				}
			}
			if m := g.db.Moves[bk]; m != nil {
				top = " · top: " + m.Name
			}
		}
		when := time.UnixMilli(int64(num(h["ts"]))).Format("Jan 2")
		src := "web"
		if num(h["go"]) > 0 {
			src = "Go"
		}
		l1 := fmt.Sprintf("%s · %s · Difficulty %d · Tier %d · %d mods · %s · %s%s", when, src, int(num(h["tr"])), int(math.Max(1, num(h["sg"]))), len(md), ps, biomeList(h), top)
		r1 := fmt.Sprintf("%s · Lv %d · %s kills · +%s coins%s", fmtT(num(h["tm"])), int(num(h["lv"])), commas(int(num(h["k"]))), commas(int(num(h["g"]))), b2s(num(h["w"]) > 0, "  WIN", ""))
		u.label([4]float64{40, y, sw - 90, 40}, true, func(dst *ebiten.Image, r [4]float64) {
			roundRect(dst, r[0], r[1], r[2], 38, 6, c(26, 32, 48, 220))
			drawText(dst, name, r[0]+10, r[1]+4, 13, true, col, text.AlignStart, false)
			drawText(dst, l1, r[0]+10, r[1]+21, 11, false, dim, text.AlignStart, false)
			drawText(dst, r1, r[0]+r[2]-10, r[1]+11, 12, true, b2c(num(h["w"]) > 0, gold, white), text.AlignEnd, false)
		})
		y += 42
	}
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
	sw := float64(g.sw)
	u := &g.ui
	D := meta.DailyDef(time.Now())
	lb := g.prof.LB()
	g.header("DAILY RUN", fmt.Sprintf("%s · resets in %s (UTC midnight)", D.Key, meta.UntilReset(time.Now())))
	g.backButton(scMenu)
	x := sw/2 - 300
	u.label([4]float64{x, 100, 600, 150}, false, func(dst *ebiten.Image, r [4]float64) {
		roundRect(dst, r[0], r[1], r[2], r[3], 12, panel)
		drawText(dst, biomeName(D.Biome)+"  ·  Difficulty 2  ·  15:00 run", r[0]+20, r[1]+16, 18, true, tc(D.Biome), text.AlignStart, false)
		mods := ""
		for _, m := range meta.Mods {
			if m.Key == D.Mods[0] || m.Key == D.Mods[1] {
				if mods != "" {
					mods += "  +  "
				}
				mods += m.Name + " (" + m.Desc + ")"
			}
		}
		for i, ln := range wrap("Modifiers: "+mods, font(12, false), r[2]-40) {
			drawText(dst, ln, r[0]+20, r[1]+46+float64(i)*16, 12, false, dim, text.AlignStart, false)
		}
		drawText(dst, "Everyone gets the same biome, modifiers and world today. Any Pokémon works.", r[0]+20, r[1]+84, 12, false, dim, text.AlignStart, false)
		drawText(dst, fmt.Sprintf("Best today: %s kills · %s survived", commas(int(lb.Num("dayKills"))), fmtT(lb.Num("dayTime"))), r[0]+20, r[1]+112, 14, true, gold, text.AlignStart, false)
	})
	p := g.db.ByID[g.csel]
	u.button("go", [4]float64{x, 266, 600, 54}, false, false, btnDraw("Start today's Daily Run", "as "+p.Name, gold, false), func() { g.startDaily() })
	u.button("ch", [4]float64{x, 330, 290, 46}, false, false, btnDraw("Change Pokémon", "", c(159, 232, 255, 255), false), func() { g.scr = scChars })
	u.button("lb", [4]float64{x + 310, 330, 290, 46}, false, false, btnDraw("Leaderboards", g.cloud.Status(), c(159, 232, 255, 255), false), func() { g.scr = scBoards; g.lbFetch() })
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
	cfg := sim.Config{Ability: pr.AbilityIdx(id), LI: 0, Up: up, Training: pr.Training(fam), Tier: D.Tier, Mods: D.Mods, Stage: 1}
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
	g.header("LEADERBOARDS", "Only Daily Run attempts count, so everyone plays the same setup · Today: "+biomeName(D.Biome))
	g.backButton(scDaily)
	for i, t := range []string{"day", "week", "all"} {
		t := t
		lab := map[string]string{"day": "Daily", "week": "Weekly", "all": "All-time"}[t]
		u.button("t:"+t, [4]float64{40 + float64(i)*160, 96, 150, 40}, false, false, btnDraw(lab, "", b2c(g.lbT == t, gold, dim), false), func() { g.lbT = t; g.lbFetch() })
	}
	for i, m := range []string{"k", "t"} {
		m := m
		lab := map[string]string{"k": "Most kills", "t": "Longest survived"}[m]
		u.button("m:"+m, [4]float64{sw - 380 + float64(i)*170, 96, 160, 40}, false, false, btnDraw(lab, "", b2c(g.lbM == m, gold, dim), false), func() { g.lbM = m; g.lbFetch() })
	}
	u.label([4]float64{40, 146, sw - 80, 18}, false, textDraw(g.lbNote, 12, false, dim, text.AlignStart))
	u.area = [4]float64{40, 170, sw - 80, sh - 190}
	y := 0.0
	if len(g.lbRows) == 0 {
		u.label([4]float64{40, y, sw - 80, 20}, true, textDraw("No runs yet for this board. Play today's Daily Run to get on it!", 14, false, dim, text.AlignStart))
	}
	for i, r := range g.lbRows {
		r := r
		rank := fmt.Sprint(i + 1)
		p := g.db.ByID[r.Run.C]
		name := r.Run.C
		col := white
		if p != nil {
			name, col = p.Name, tc(p.Types[0])
		}
		mv := ""
		for _, m := range r.Run.MV {
			if mm := g.db.Moves[m.ID]; mm != nil {
				if mv != "" {
					mv += " · "
				}
				mv += fmt.Sprintf("%s %d%s", mm.Name, m.Lv, b2s(m.Evo, "★", ""))
			}
		}
		val := commas(r.V) + " kills"
		sub := fmtT(float64(r.Run.Tm))
		if g.lbM == "t" {
			val, sub = fmtT(float64(r.V)), commas(r.Run.K)+" kills"
		}
		u.label([4]float64{40, y, sw - 90, 46}, true, func(dst *ebiten.Image, rr [4]float64) {
			roundRect(dst, rr[0], rr[1], rr[2], 44, 7, b2c(r.Me, c(40, 50, 40, 240), c(26, 32, 48, 220)))
			drawText(dst, rank, rr[0]+12, rr[1]+12, 16, true, b2c(i < 3, gold, dim), text.AlignStart, false)
			drawText(dst, r.Name, rr[0]+50, rr[1]+6, 14, true, white, text.AlignStart, false)
			drawText(dst, name+" · "+biomeName(r.Run.Bi)+" · "+mv, rr[0]+50, rr[1]+25, 11, false, col, text.AlignStart, false)
			drawText(dst, val, rr[0]+rr[2]-12, rr[1]+6, 15, true, white, text.AlignEnd, false)
			drawText(dst, sub, rr[0]+rr[2]-12, rr[1]+26, 11, false, dim, text.AlignEnd, false)
		})
		y += 50
	}
	u.scrollMax = math.Max(0, y-u.area[3])
}

// lbFetch loads the current board (online when signed in, else your own best).
func (g *Game) lbFetch() {
	if g.lbT == "" {
		g.lbT, g.lbM = "day", "k"
	}
	D := meta.DailyDef(time.Now())
	g.lbNote = "Daily resets in " + meta.UntilReset(time.Now()) + "."
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
