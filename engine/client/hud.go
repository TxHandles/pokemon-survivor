package client

import (
	"fmt"
	"image"
	"image/color"
	"math"
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
	if w <= 0 || h <= 0 {
		return
	}
	r = math.Max(0, math.Min(r, math.Min(w, h)/2))
	if r < .5 {
		rect(dst, x, y, w, h, col)
		return
	}
	rect(dst, x+r, y, w-2*r, h, col)
	rect(dst, x, y+r, r, h-2*r, col)
	rect(dst, x+w-r, y+r, r, h-2*r, col)
	t := discFor(r)
	n := t.Bounds().Dx()
	m := n / 2
	texQuad(dst, t, image.Rect(0, 0, m, m), x, y, r, r, col)
	texQuad(dst, t, image.Rect(m, 0, n, m), x+w-r, y, r, r, col)
	texQuad(dst, t, image.Rect(0, m, m, n), x, y+h-r, r, r, col)
	texQuad(dst, t, image.Rect(m, m, n, n), x+w-r, y+h-r, r, r, col)
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
	if g.coop {
		g.drawCoopHUD(dst)
		g.drawHUDCommon(dst)
		return
	}
	g.drawWebHUD(dst)
	_, _ = P, sw
}

// drawHUDCommon: timer, location, boss bar, moves, items, messages.
func (g *Game) drawHUDCommon(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	// timer / biome / kills
	tm := fmtT(w.T) + " / " + fmtT(w.Run.Len)
	if w.Endless {
		tm = "ENDLESS  " + fmtT(w.T)
	}
	drawText(dst, tm, sw/2, 16, 26, true, b2c(w.Endless, c(255, 130, 130, 255), white), text.AlignCenter, true)
	drawText(dst, w.LocName(w.Biome, w.LI)+"  ·  "+fmt.Sprintf("%d KOs", w.Kills), sw/2, 48, 13, false, dim, text.AlignCenter, true)
	if nb := 300 * math.Ceil((w.T+.01)/300); nb-w.T < 20 && w.Bosses < int(nb/300) {
		drawText(dst, fmt.Sprintf("Boss in %.0fs", nb-w.T), sw/2, 68, 13, true, c(255, 120, 120, 255), text.AlignCenter, true)
	}
	// boss bar
	for _, e := range w.E {
		if !e.Boss || e.Dead {
			continue
		}
		bw := math.Min(520, sw-360)
		bx := sw/2 - bw/2
		roundRect(dst, bx-8, 86, bw+16, 40, 8, panel)
		name := e.BD.Name
		if p := g.db.ByID[e.PokeID]; p != nil {
			name += " · " + p.Name
		}
		drawText(dst, fmt.Sprintf("%s  ·  Phase %d", name, max(1, e.Ph)), sw/2, 90, 13, true, tc(e.Type), text.AlignCenter, false)
		bar(dst, bx, 110, bw, 10, e.HP/e.MHP, c(255, 80, 80, 255), c(40, 46, 60, 255))
		break
	}
	if g.opt("eq") == 0 { // Settings → Equipped bar: Hide
		g.drawHUDTail(dst)
		return
	}
	// moves (bottom-left): in co-op, every player's row
	x := 12.0
	y := float64(g.sh) - 64
	if g.coop {
		y = float64(g.sh) - 40 - 44*float64(len(w.PL))
		for i, pc := range w.PL {
			xx := 12.0
			drawText(dst, fmt.Sprintf("P%d", i+1), xx, y+12, 12, true, playerCols[i%4], text.AlignStart, true)
			xx += 28
			for _, s := range pc.Slots {
				col := tc(s.Type)
				roundRect(dst, xx, y, 120, 36, 6, panel)
				rect(dst, xx, y+6, 3, 24, col)
				drawText(dst, s.Move.Name, xx+10, y+4, 11, true, white, text.AlignStart, false)
				drawText(dst, fmt.Sprintf("Lv %d", s.Lv), xx+10, y+19, 10, false, dim, text.AlignStart, false)
				xx += 126
			}
			y += 44
		}
		y = float64(g.sh) + 100 // skip the solo panel
	}
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
	g.drawHUDTail(dst)
}

// drawHUDTail: the centre message and demo label.
func (g *Game) drawHUDTail(dst *ebiten.Image) {
	w := g.W
	sw := float64(g.sw)
	if w.MsgT > 0 && !w.Waiting() {
		a := math.Min(1, w.MsgT/.6)
		drawText(dst, w.Msg, sw/2, float64(g.sh)*.24, 20, true, alpha(gold, a), text.AlignCenter, true)
	}
	if g.auto {
		drawText(dst, "DEMO · bot playing", sw-12, 72, 12, true, alpha(dim, .8), text.AlignEnd, true)
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
	if g.W != nil && g.W.State == sim.StateLevelUp {
		_, rs := g.lvLayout()
		return rs
	}
	if g.W != nil && g.W.State == sim.StateRoute {
		return g.routeRects()
	}
	if g.W != nil && g.W.State == sim.StateShrine {
		_, rs := g.shrineLayout()
		return rs
	}
	n := g.choiceCount()
	cw, ch := 300.0, 260.0
	if n > 3 {
		cw = 250
	}
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
	g.drawWebLevelUp(dst)
}

// ---- title ----

func (g *Game) titleGrid() (x0, y0, cell float64) {
	cols := g.titleCols()
	cell = math.Min(88, (float64(g.sw)-80)/float64(cols))
	x0 = (float64(g.sw) - cell*float64(cols)) / 2
	y0 = 136
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
	drawText(dst, "POKÉMON SURVIVOR", float64(g.sw)/2, 34, 44, true, gold, text.AlignCenter, true)
	drawText(dst, "Go build · early preview", float64(g.sw)/2, 88, 15, false, dim, text.AlignCenter, true)
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
		drawText(dst, p.Name, cx+cell/2, cy+cell-24, 10, true, b2c(sel, white, dim), text.AlignCenter, false)
	}
	rows := (len(g.roster) + cols - 1) / cols
	by := y0 + float64(rows)*cell + 14
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
	ab := g.db.AbilityPair(p.ID)
	for i, a := range ab {
		kind := "Starting ability"
		if a.Tier == 2 {
			kind = "Hidden ability"
		} else if a.Tier == 3 {
			kind = "Legendary ability"
		}
		col := dim
		mark := "   "
		if i == g.ability {
			col, mark = white, "> "
		}
		drawText(dst, mark+kind+": "+a.Name+" — "+a.Desc, float64(g.sw)/2, by+54+float64(i)*20, 13, i == g.ability, col, text.AlignCenter, true)
	}
	drawText(dst, "(E / Y to switch ability)", float64(g.sw)/2, by+96, 12, false, alpha(dim, .8), text.AlignCenter, true)
	drawText(dst, "Enter / A to start  ·  WASD / stick to move  ·  Shift / RB to dash  ·  Esc to pause", float64(g.sw)/2, float64(g.sh)-34, 13, false, dim, text.AlignCenter, true)
}

func (g *Game) pauseRects() [3][4]float64 {
	cx, cy := float64(g.sw)/2, float64(g.sh)/2
	return [3][4]float64{{cx - 140, cy + 10, 280, 46}, {cx - 140, cy + 66, 280, 46}, {cx - 140, cy + 122, 280, 46}}
}

func (g *Game) pauseHit(x, y float64) int {
	for i, r := range g.pauseRects() {
		if inR(r, x, y) {
			return i + 1
		}
	}
	return 0
}

func (g *Game) drawPause(dst *ebiten.Image) {
	g.drawWebPause(dst)
}

func (g *Game) drawOver(dst *ebiten.Image) {
	w := g.W
	rect(dst, 0, 0, float64(g.sw), float64(g.sh), c(0, 0, 0, uint8(math.Min(1, g.overT*2)*185)))
	cx := float64(g.sw) / 2
	y := float64(g.sh)/2 - 60
	title := "Knocked out"
	if w.Won {
		title = "Victory!"
	}
	drawText(dst, title, cx, y, 32, true, webFg, text.AlignCenter, true)
	drawText(dst, fmt.Sprintf("Time %s · Level %d · %d defeated", fmtT(w.T), w.P.Lv, w.Kills), cx, y+50, 15, false, alpha(webFg, .9), text.AlignCenter, true)
	if g.auto && g.overT > .8 {
		drawText(dst, "Enter / A / click to return to the menu", cx, y+80, 13, false, dim, text.AlignCenter, true)
	}
}

func buffText(w *sim.World) string {
	P := w.P
	s := ""
	add := func(t float64, n string) {
		if t > 0 {
			if s != "" {
				s += "  "
			}
			s += fmt.Sprintf("%s %ds", n, int(math.Ceil(t)))
		}
	}
	add(P.Pm, "Might")
	add(P.Pbl, "Blood pact")
	add(P.Phs, "Haste pact")
	add(P.Pgr, "Greed pact")
	add(P.Bat, "Battery")
	add(P.Bel, "Belt")
	return s
}
