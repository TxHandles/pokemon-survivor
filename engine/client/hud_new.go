package client

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// The default in-game HUD (Settings → HUD layout: New): the web HP and XP bars along the top,
// the party with portraits and health on the left, and attacks and held items as icon cards
// in the bottom-left corner. In co-op every player gets a party card and an equipment row.

func (g *Game) drawNewHUD(dst *ebiten.Image) {
	g.drawTopBars(dst)
	g.drawParty(dst, 12, 72)
	g.drawBossWarn(dst)
	if g.opt("eq") != 0 {
		if g.coop {
			g.drawCoopEquip(dst)
		} else {
			g.drawEquipCards(dst)
		}
	}
	g.drawCenterArrow(dst)
	g.drawHUDTail(dst)
}

// drawCenterArrow: when the Pokémon Center is off screen, an arrow at the screen edge points to it.
func (g *Game) drawCenterArrow(dst *ebiten.Image) {
	w := g.W
	pc := w.PCN
	if pc == nil {
		return
	}
	sw, sh := float64(g.sw), float64(g.sh)
	cx, cy := w.Centroid()
	if !g.coop {
		cx, cy = w.P.X, w.P.Y
	}
	dx, dy := pc.X-cx, pc.Y-cy
	if math.Abs(dx) < sw/2-20 && math.Abs(dy) < sh/2-20 {
		return // on screen
	}
	// clamp the direction to a box inside the screen edges (clear of the HUD bars)
	mx, my := sw/2-70, sh/2-80
	k := math.Min(mx/math.Max(1e-6, math.Abs(dx)), my/math.Max(1e-6, math.Abs(dy)))
	x, y := sw/2+dx*k, sh/2+10+dy*k
	a := math.Atan2(dy, dx)
	col := c(255, 110, 150, 255)
	pulse := .75 + .25*math.Sin(w.T*5)
	disc(dst, x, y, 24, c(16, 20, 30, 220))
	ring(dst, x, y, 24, 2, alpha(col, pulse))
	// the little centre roof + cross
	rect(dst, x-9, y-3, 18, 11, c(235, 235, 245, 255))
	poly(dst, [][2]float64{{x - 11, y - 3}, {x, y - 12}, {x + 11, y - 3}}, col)
	rect(dst, x-1.5, y-1, 3, 8, col)
	rect(dst, x-4, y+1.5, 8, 3, col)
	ax, ay := x+math.Cos(a)*32, y+math.Sin(a)*32
	poly(dst, [][2]float64{{ax + math.Cos(a)*9, ay + math.Sin(a)*9}, {ax + math.Cos(a+2.2)*8, ay + math.Sin(a+2.2)*8}, {ax + math.Cos(a-2.2)*8, ay + math.Sin(a-2.2)*8}}, alpha(col, pulse))
	d := math.Hypot(dx, dy)
	lbl := fmt.Sprintf("Pokémon Center · %dm", int(d/10))
	ly := y + 30
	if y > sh/2 {
		ly = y - 46
	}
	drawText(dst, lbl, x, ly, 11, true, white, text.AlignCenter, true)
}

// hpCol: the web HP bar colours (top highlight, main) for a health fraction.
func hpCol(f float64) (top, mid color.NRGBA) {
	switch {
	case f < .25:
		return hexC("#ff9a8a"), hexC("#ee3b2b")
	case f < .5:
		return hexC("#ffe58a"), hexC("#f0b429")
	}
	return hexC("#8ff5a6"), hexC("#2fc45a")
}

// drawParty: one card per party member (portrait, name, level, health bar).
func (g *Game) drawParty(dst *ebiten.Image, x, y float64) {
	w := g.W
	const cw, ch = 220.0, 48.0
	for i, pc := range w.PL {
		P := pc.P
		if pc.Gone {
			continue
		}
		border := webBorder
		if g.coop {
			border = playerCols[i%4]
		}
		webBox(dst, x, y, cw, ch, 10, alpha(border, .9), alpha(hexC("#0a0d14"), .78))
		g.drawPortrait(dst, P.ID, x+5, y+5, 38, P.Dead, true, false)
		name := strings.TrimPrefix(pc.Name, fmt.Sprintf("P%d ", i+1))
		if !g.coop || name == "" {
			if p := g.db.ByID[P.ID]; p != nil {
				name = p.Name
			}
		}
		tag := "Lv " + strconv.Itoa(P.Lv)
		if g.coop {
			tag = fmt.Sprintf("P%d", i+1)
		}
		nc := webFg
		if P.Dead {
			nc = c(150, 156, 170, 255)
			tag = "fainted"
		}
		drawText(dst, fitText(name, 13, true, cw-60-textW(tag, 11, true)-12), x+50, y+5, 13, true, nc, text.AlignStart, true)
		drawText(dst, tag, x+cw-8, y+6, 11, true, b2c(g.coop && !P.Dead, border, alpha(webFg, .7)), text.AlignEnd, true)
		// health bar
		bx, by, bw, bh := x+50, y+25, cw-58, 15.0
		hpf := math.Max(0, math.Min(1, P.HP/math.Max(1, P.MHP)))
		bc := hexC("#3a2a2a")
		if hpf < .25 && !P.Dead && (g.opt("flash") == 0 || g.tick/20%2 == 0) {
			bc = hexC("#ff4d4d")
		}
		webBox(dst, bx, by, bw, bh, 7, bc, hexC("#080b12"))
		if hpf > 0 {
			top, mid := hpCol(hpf)
			roundRect(dst, bx+2, by+2, (bw-4)*hpf, bh-4, 5, mid)
			roundRect(dst, bx+2, by+2, (bw-4)*hpf, (bh-4)*.45, 3, alpha(top, .85))
		}
		drawText(dst, fmt.Sprintf("%d / %d", int(math.Ceil(P.HP)), int(P.MHP)), bx+bw/2, by+1.5, 10, true, white, text.AlignCenter, true)
		y += ch + 6
	}
}

// drawEquipCards: solo attacks (type icon, name, level, charge) along the bottom-left, held items
// as icon circles just above them.
func (g *Game) drawEquipCards(dst *ebiten.Image) {
	w := g.W
	sh := float64(g.sh)
	const cw, ch = 164.0, 50.0
	x, y := 12.0, sh-12-ch
	for _, s := range w.Slots {
		g.moveCard(dst, w, s, x, y, cw, ch)
		x += cw + 6
	}
	g.itemRow(dst, w.Held, w.HeldOrder, 12, y-8-34)
}

func (g *Game) moveCard(dst *ebiten.Image, w *sim.World, s *sim.Slot, x, y, cw, ch float64) {
	col := tc(s.Type)
	webBox(dst, x, y, cw, ch, 10, alpha(col, .95), alpha(hexC("#0a0d14"), .82))
	// icon tile
	webBox(dst, x+6, y+6, 38, 38, 8, alpha(col, .6), alpha(col, .18))
	if s.Evo {
		soft(dst, x+25, y+25, 24, 24, alpha(webAcc, .4))
	}
	g.drawTypeIcon(dst, s.Type, x+25, y+25, 26, false)
	name := s.Move.Name
	drawText(dst, fitText(name, 13, true, cw-58), x+51, y+6, 13, true, webFg, text.AlignStart, true)
	lv := "Lv " + strconv.Itoa(s.Lv)
	drawText(dst, lv, x+51, y+23, 11, true, alpha(webFg, .7), text.AlignStart, true)
	f := w.CoolFrac(s)
	if s.Kind == "dash" {
		f = w.DashReady()
		drawText(dst, "SHIFT", x+cw-7, y+23, 10, true, b2c(f >= 1, webAcc, alpha(webFg, .5)), text.AlignEnd, true)
	} else if s.Evo {
		drawText(dst, "EVOLVED", x+cw-7, y+23, 10, true, webAcc, text.AlignEnd, true)
	}
	// charge bar
	bx, by, bw := x+51, y+ch-11, cw-58
	roundRect(dst, bx, by, bw, 5, 2.5, c(40, 46, 60, 255))
	bc := col
	if f >= 1 {
		bc = alpha(col, .95)
	}
	if f > 0 {
		roundRect(dst, bx, by, math.Max(5, bw*f), 5, 2.5, bc)
	}
}

// itemRow: held items as circles with a short label and their level.
func (g *Game) itemRow(dst *ebiten.Image, held map[string]int, order []string, x, y float64) float64 {
	const d = 34.0
	for _, id := range order {
		lv := held[id]
		if lv == 0 {
			continue
		}
		disc(dst, x+d/2, y+d/2, d/2, webAcc)
		disc(dst, x+d/2, y+d/2, d/2-2, alpha(hexC("#0a0d14"), .92))
		drawText(dst, itemAbbr(id), x+d/2, y+9, 12, true, webFg, text.AlignCenter, false)
		disc(dst, x+d-3, y+d-4, 7.5, webAcc)
		drawText(dst, strconv.Itoa(lv), x+d-3, y+d-11, 10, true, hexC("#0a0d14"), text.AlignCenter, false)
		x += d + 6
	}
	return x
}

// drawCoopEquip: one row per player along the bottom-left: attack tiles, then item circles.
func (g *Game) drawCoopEquip(dst *ebiten.Image) {
	w := g.W
	n := 0
	for _, pc := range w.PL {
		if !pc.Gone {
			n++
		}
	}
	y := float64(g.sh) - 12 - 40*float64(n)
	for i, pc := range w.PL {
		if pc.Gone {
			continue
		}
		col := playerCols[i%4]
		drawText(dst, fmt.Sprintf("P%d", i+1), 12, y+10, 13, true, col, text.AlignStart, true)
		x := 40.0
		for _, s := range pc.Slots {
			tc0 := tc(s.Type)
			webBox(dst, x, y, 34, 34, 8, tc0, alpha(hexC("#0a0d14"), .85))
			if s.Evo {
				soft(dst, x+17, y+17, 20, 20, alpha(webAcc, .35))
			}
			g.drawTypeIcon(dst, s.Type, x+17, y+16, 22, false)
			drawText(dst, strconv.Itoa(s.Lv), x+31, y+19, 10, true, white, text.AlignEnd, true)
			x += 38
		}
		g.itemRow(dst, pc.Held, pc.HeldOrder, x+8, y)
		y += 40
	}
}
