package client

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

var (
	white = c(255, 255, 255, 255)
	dim   = c(180, 188, 205, 255)
	gold  = c(255, 208, 74, 255)
	panel = c(14, 18, 28, 225)
)

func cap1(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func roundRect(dst *ebiten.Image, x, y, w, h, r float64, col color.NRGBA) {
	rect(dst, x+r, y, w-2*r, h, col)
	rect(dst, x, y+r, r, h-2*r, col)
	rect(dst, x+w-r, y+r, r, h-2*r, col)
	disc(dst, x+r, y+r, r, col)
	disc(dst, x+w-r, y+r, r, col)
	disc(dst, x+r, y+h-r, r, col)
	disc(dst, x+w-r, y+h-r, r, col)
}

func bar(dst *ebiten.Image, x, y, w, h, f float64, fg, bg color.NRGBA) {
	roundRect(dst, x, y, w, h, h/2, bg)
	if f > 0 {
		ww := math.Max(h, w*math.Min(1, f))
		roundRect(dst, x, y, ww, h, h/2, fg)
	}
}

func (g *Game) drawHUD(dst *ebiten.Image) {
	w := g.W
	P := w.P
	sw := float64(g.sw)
	// XP bar across the top
	bar(dst, 0, 0, sw, 8, P.XP/P.NX, c(90, 190, 255, 255), c(20, 26, 40, 230))
	// HP
	roundRect(dst, 12, 16, 290, 58, 10, panel)
	if s := g.assets.Sprite(P.ID); s != nil {
		fr := s.Front.Frame(s.Front.IdleFrame(0))
		op := &ebiten.DrawImageOptions{}
		k := 44 / math.Max(float64(s.Front.V.CW), float64(s.Front.V.CH))
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(18, 23)
		op.Filter = ebiten.FilterLinear
		dst.DrawImage(fr, op)
	}
	drawText(dst, fmt.Sprintf("%s  Lv %d", P.Name, P.Lv), 70, 21, 15, true, white, text.AlignStart, false)
	hpf := P.HP / P.MHP
	hc := c(80, 220, 110, 255)
	if hpf < .5 {
		hc = c(255, 200, 70, 255)
	}
	if hpf < .25 {
		hc = c(255, 80, 80, 255)
	}
	bar(dst, 70, 44, 220, 12, hpf, hc, c(40, 46, 60, 255))
	drawText(dst, fmt.Sprintf("%.0f / %.0f", P.HP, P.MHP), 180, 58, 11, true, dim, text.AlignCenter, false)
	// timer / biome / kills
	drawText(dst, fmtT(w.T), sw/2, 16, 26, true, white, text.AlignCenter, true)
	drawText(dst, biomeName(w.Biome)+"  ·  "+fmt.Sprintf("%d KOs", w.Kills), sw/2, 48, 13, false, dim, text.AlignCenter, true)
	if nb := 300 * math.Ceil((w.T+.01)/300); nb-w.T < 20 && w.Bosses < int(nb/300) {
		drawText(dst, fmt.Sprintf("Boss in %.0fs", nb-w.T), sw/2, 68, 13, true, c(255, 120, 120, 255), text.AlignCenter, true)
	}
	// moves (bottom-left)
	x := 12.0
	y := float64(g.sh) - 64
	for _, s := range w.Slots {
		col := tc(s.Type)
		roundRect(dst, x, y, 150, 52, 8, panel)
		rect(dst, x, y+8, 4, 36, col)
		name := s.Move.Name
		if s.Evo {
			name += " (evolved)"
		}
		drawText(dst, name, x+12, y+7, 13, true, white, text.AlignStart, false)
		drawText(dst, fmt.Sprintf("Lv %d · %s", s.Lv, cap1(s.Kind)), x+12, y+25, 11, false, dim, text.AlignStart, false)
		f := 1.0
		if s.Kind == "dash" {
			f = w.DashReady()
		}
		if s.Kind == "dash" {
			bar(dst, x+12, y+41, 126, 5, f, b2c(f >= 1, gold, col), c(40, 46, 60, 255))
			if f >= 1 {
				drawText(dst, "SHIFT / RB", x+138, y+7, 10, true, gold, text.AlignEnd, false)
			}
		} else {
			bar(dst, x+12, y+41, 126, 5, float64(s.Lv)/10, col, c(40, 46, 60, 255))
		}
		x += 158
	}
	// held items (bottom-right)
	x = float64(g.sw) - 12
	for i := len(w.HeldOrder) - 1; i >= 0; i-- {
		id := w.HeldOrder[i]
		if w.Held[id] == 0 {
			continue
		}
		it := sim.ItemByID[id]
		label := fmt.Sprintf("%s  %d", it.Name, w.Held[id])
		tw, _ := text.Measure(label, font(12, true), 0)
		x -= tw + 22
		roundRect(dst, x, float64(g.sh)-40, tw+16, 28, 7, panel)
		drawText(dst, label, x+8, float64(g.sh)-33, 12, true, c(230, 230, 240, 255), text.AlignStart, false)
		x -= 6
	}
	// message
	if w.MsgT > 0 {
		a := math.Min(1, w.MsgT/.6)
		drawText(dst, w.Msg, sw/2, float64(g.sh)*.24, 20, true, alpha(gold, a), text.AlignCenter, true)
	}
	if g.auto {
		drawText(dst, "DEMO · bot playing", sw-12, 16, 12, true, alpha(dim, .8), text.AlignEnd, true)
	}
}

func b2c(b bool, t, f color.NRGBA) color.NRGBA {
	if b {
		return t
	}
	return f
}

// ---- level up ----

func (g *Game) cardRects() [][4]float64 {
	n := len(g.W.Opts)
	cw, ch := 300.0, 260.0
	gap := 22.0
	if tot := float64(n)*cw + float64(n-1)*gap; tot > float64(g.sw)-40 {
		cw = (float64(g.sw) - 40 - float64(n-1)*gap) / float64(n)
	}
	x0 := (float64(g.sw) - (float64(n)*cw + float64(n-1)*gap)) / 2
	y := float64(g.sh)/2 - ch/2 + 20
	var r [][4]float64
	for i := 0; i < n; i++ {
		r = append(r, [4]float64{x0 + float64(i)*(cw+gap), y, cw, ch})
	}
	return r
}

func (g *Game) cardHit(mx, my int) int {
	for i, r := range g.cardRects() {
		if float64(mx) >= r[0] && float64(mx) <= r[0]+r[2] && float64(my) >= r[1] && float64(my) <= r[1]+r[3] {
			return i
		}
	}
	return -1
}

func wrap(s string, f *text.GoTextFace, width float64) []string {
	var out []string
	cur := ""
	for _, wd := range strings.Fields(s) {
		t := wd
		if cur != "" {
			t = cur + " " + wd
		}
		if tw, _ := text.Measure(t, f, 0); tw > width && cur != "" {
			out = append(out, cur)
			cur = wd
		} else {
			cur = t
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func (g *Game) drawLevelUp(dst *ebiten.Image) {
	w := g.W
	rect(dst, 0, 0, float64(g.sw), float64(g.sh), c(5, 8, 15, 170))
	drawText(dst, "LEVEL UP!", float64(g.sw)/2, float64(g.sh)/2-190, 34, true, gold, text.AlignCenter, true)
	drawText(dst, "Choose one  ·  1-3, arrows + Enter, click, or gamepad", float64(g.sw)/2, float64(g.sh)/2-148, 13, false, dim, text.AlignCenter, true)
	mx, my := ebiten.CursorPosition()
	hov := g.cardHit(mx, my)
	for i, r := range g.cardRects() {
		o := w.Opts[i]
		x, y, cw, ch := r[0], r[1], r[2], r[3]
		sel := i == g.lvSel || i == hov
		col := white
		title, sub, tag, desc := "", "", "", ""
		if o.Move {
			m := g.db.Moves[o.ID]
			col = tc(m.Type)
			title = m.Name
			sub = cap1(m.Type) + " · " + cap1(m.Shape) + " · " + cap1(m.Tier)
			desc = m.Desc
			switch {
			case o.Evo:
				tag = "EVOLVE"
				desc = "Held " + sim.ItemByID[m.HeldItem].Name + " evolves this move: stronger, plus a new effect. Unlocks Lv 11–20."
			case o.New:
				tag = "NEW MOVE"
			default:
				tag = fmt.Sprintf("Lv %d → %d", o.Level, o.Level+1)
			}
			if !o.Evo && sim.ItemByID[m.HeldItem] != nil && o.New {
				desc += "  (Evolves at Lv 10 with " + sim.ItemByID[m.HeldItem].Name + ")"
			}
		} else {
			it := sim.ItemByID[o.ID]
			col = c(230, 230, 240, 255)
			title = it.Name
			sub = "Held item"
			desc = it.Desc
			tag = "NEW"
			if o.Level > 0 {
				tag = fmt.Sprintf("Lv %d → %d", o.Level, o.Level+1)
			}
			if o.Swap != "" {
				tag = "SWAP"
				desc += "  (replaces " + sim.ItemByID[o.Swap].Name + ")"
			}
		}
		bg := c(22, 28, 42, 245)
		if sel {
			bg = c(34, 42, 62, 250)
			roundRect(dst, x-3, y-3, cw+6, ch+6, 16, alpha(col, .9))
		}
		roundRect(dst, x, y, cw, ch, 14, bg)
		roundRect(dst, x, y, cw, 8, 4, col)
		drawText(dst, fmt.Sprintf("%d", i+1), x+16, y+18, 14, true, dim, text.AlignStart, false)
		drawText(dst, tag, x+cw-16, y+18, 13, true, b2c(o.Evo || o.New, gold, dim), text.AlignEnd, false)
		drawText(dst, title, x+cw/2, y+48, 22, true, col, text.AlignCenter, false)
		drawText(dst, sub, x+cw/2, y+80, 12, false, dim, text.AlignCenter, false)
		yy := y + 112
		for _, ln := range wrap(desc, font(14, false), cw-36) {
			drawText(dst, ln, x+cw/2, yy, 14, false, c(225, 228, 238, 255), text.AlignCenter, false)
			yy += 20
		}
	}
}

// ---- title ----

func (g *Game) titleGrid() (x0, y0, cell float64) {
	cols := g.titleCols()
	cell = math.Min(104, (float64(g.sw)-80)/float64(cols))
	x0 = (float64(g.sw) - cell*float64(cols)) / 2
	y0 = 170
	return
}

func (g *Game) titleHit(mx, my int) int {
	x0, y0, cell := g.titleGrid()
	cols := g.titleCols()
	i := int((float64(mx)-x0)/cell) + cols*int((float64(my)-y0)/cell)
	if float64(mx) < x0 || float64(my) < y0 || int((float64(mx)-x0)/cell) >= cols || i < 0 || i >= len(g.roster) {
		return -1
	}
	return i
}

func (g *Game) drawTitle(dst *ebiten.Image) {
	t := float64(g.tick) / 60
	dst.Fill(c(16, 20, 32, 255))
	for i := 0; i < 14; i++ {
		x := math.Mod(float64(i)*173+t*12, float64(g.sw)+200) - 100
		soft(dst, x, 120+math.Sin(float64(i)*1.7+t*.3)*90+float64(i%5)*120, 160, 120, alpha(tc(content.Types[i%18]), .07))
	}
	drawText(dst, "POKÉMON SURVIVOR", float64(g.sw)/2, 52, 46, true, gold, text.AlignCenter, true)
	drawText(dst, "Go build · early preview", float64(g.sw)/2, 110, 15, false, dim, text.AlignCenter, true)
	x0, y0, cell := g.titleGrid()
	cols := g.titleCols()
	mx, my := ebiten.CursorPosition()
	hov := g.titleHit(mx, my)
	for i, id := range g.roster {
		cx, cy := x0+float64(i%cols)*cell, y0+float64(i/cols)*cell
		p := g.db.ByID[id]
		col := tc(p.Types[0])
		sel := i == g.sel
		bg := c(26, 32, 48, 255)
		if sel || i == hov {
			bg = mix(c(26, 32, 48, 255), col, .3)
		}
		roundRect(dst, cx+4, cy+4, cell-8, cell-8, 10, bg)
		if sel {
			roundRect(dst, cx+4, cy+cell-12, cell-8, 4, 2, col)
		}
		if s := g.assets.Sprite(id); s != nil {
			fr := s.Front.IdleFrame(int(t * 1000))
			if !sel {
				fr = s.Front.IdleFrame(0)
			}
			k := math.Min((cell-26)/float64(s.Front.V.CW), (cell-34)/float64(s.Front.V.CH))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(cx+cell/2-float64(s.Front.V.CW)*k/2, cy+10)
			op.Filter = ebiten.FilterLinear
			dst.DrawImage(s.Front.Frame(fr), op)
		}
		drawText(dst, p.Name, cx+cell/2, cy+cell-28, 11, true, b2c(sel, white, dim), text.AlignCenter, false)
	}
	rows := (len(g.roster) + cols - 1) / cols
	by := y0 + float64(rows)*cell + 20
	p := g.db.ByID[g.roster[g.sel]]
	types := ""
	for i, t := range p.Types {
		if i > 0 {
			types += " / "
		}
		types += cap1(t)
	}
	st := g.db.Stats[p.ID]
	drawText(dst, fmt.Sprintf("%s  ·  %s  ·  HP %.2f  ATK %.2f  DEF %.2f  SPD %.2f", p.Name, types, st.HP, st.Atk, st.Def, st.Spd), float64(g.sw)/2, by, 16, true, tc(p.Types[0]), text.AlignCenter, true)
	bio := "own type (" + biomeName(p.Types[0]) + ")"
	if g.biome >= 0 {
		bio = biomeName(content.Types[g.biome])
	}
	sm := g.db.StartMove(p.ID)
	drawText(dst, "Starts with "+g.db.Moves[sm].Name+"  ·  Biome: "+bio+"  (Tab / LB to change)", float64(g.sw)/2, by+28, 13, false, dim, text.AlignCenter, true)
	drawText(dst, "Enter / A to start  ·  WASD / stick to move  ·  Shift / RB to dash  ·  Esc to pause", float64(g.sw)/2, float64(g.sh)-34, 13, false, dim, text.AlignCenter, true)
}

func (g *Game) drawPause(dst *ebiten.Image) {
	rect(dst, 0, 0, float64(g.sw), float64(g.sh), c(5, 8, 15, 170))
	drawText(dst, "PAUSED", float64(g.sw)/2, float64(g.sh)/2-60, 40, true, white, text.AlignCenter, true)
	drawText(dst, "Esc / Enter / Start to resume  ·  Q / B to quit to title", float64(g.sw)/2, float64(g.sh)/2, 15, false, dim, text.AlignCenter, true)
}

func (g *Game) drawOver(dst *ebiten.Image) {
	w := g.W
	rect(dst, 0, 0, float64(g.sw), float64(g.sh), c(5, 8, 15, uint8(math.Min(1, g.overT*2)*190)))
	cx := float64(g.sw) / 2
	y := float64(g.sh)/2 - 150
	drawText(dst, "FAINTED", cx, y, 44, true, c(255, 110, 110, 255), text.AlignCenter, true)
	drawText(dst, fmt.Sprintf("Survived %s  ·  Lv %d  ·  %d KOs  ·  %d bosses", fmtT(w.T), w.P.Lv, w.Kills, w.Bosses), cx, y+64, 18, true, white, text.AlignCenter, true)
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
	yy := y + 110
	for i, e := range l {
		if i >= 6 {
			break
		}
		name, col := "Burn / poison", dim
		if m := g.db.Moves[e.k]; m != nil {
			name, col = m.Name, tc(m.Type)
		}
		bar(dst, cx-160, yy+4, 320, 14, e.v/math.Max(tot, 1), alpha(col, .8), c(40, 46, 60, 255))
		drawText(dst, fmt.Sprintf("%s  %.0f (%.0f%%)", name, e.v, e.v/math.Max(tot, 1)*100), cx, yy+3, 12, true, white, text.AlignCenter, true)
		yy += 24
	}
	if g.overT > .8 {
		drawText(dst, "Enter / A / click to return to the title", cx, yy+24, 14, false, dim, text.AlignCenter, true)
	}
}
