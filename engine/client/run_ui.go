package client

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// card draws one choice card; returns nothing. sel highlights it.
func (g *Game) card(dst *ebiten.Image, r [4]float64, i int, sel bool, col color.NRGBA, tag, title, sub, desc, warn string) {
	x, y, cw, ch := r[0], r[1], r[2], r[3]
	bg := c(22, 28, 42, 245)
	if sel {
		bg = c(34, 42, 62, 250)
		roundRect(dst, x-3, y-3, cw+6, ch+6, 16, alpha(col, .9))
	}
	roundRect(dst, x, y, cw, ch, 14, bg)
	roundRect(dst, x, y, cw, 8, 4, col)
	drawText(dst, fmt.Sprintf("%d", i+1), x+16, y+18, 14, true, dim, text.AlignStart, false)
	drawText(dst, tag, x+cw-16, y+18, 13, true, gold, text.AlignEnd, false)
	drawText(dst, title, x+cw/2, y+48, 21, true, col, text.AlignCenter, false)
	drawText(dst, sub, x+cw/2, y+80, 12, false, dim, text.AlignCenter, false)
	yy := y + 112
	for _, ln := range wrap(desc, font(14, false), cw-36) {
		drawText(dst, ln, x+cw/2, yy, 14, false, c(225, 228, 238, 255), text.AlignCenter, false)
		yy += 20
	}
	if warn != "" {
		yy += 6
		for _, ln := range wrap(warn, font(13, true), cw-36) {
			drawText(dst, ln, x+cw/2, yy, 13, true, c(255, 138, 138, 255), text.AlignCenter, false)
			yy += 19
		}
	}
}

func (g *Game) choiceHeader(dst *ebiten.Image, title, sub string, col color.NRGBA) {
	rect(dst, 0, 0, float64(g.sw), float64(g.sh), c(5, 8, 15, 175))
	drawText(dst, title, float64(g.sw)/2, float64(g.sh)/2-190, 32, true, col, text.AlignCenter, true)
	drawText(dst, sub, float64(g.sw)/2, float64(g.sh)/2-148, 13, false, dim, text.AlignCenter, true)
}

func (g *Game) drawRoute(dst *ebiten.Image) {
	g.drawWebRoute(dst)
}

func (g *Game) drawShrine(dst *ebiten.Image) { g.drawWebShrine(dst) }

// drawChests: map chests / shrines (with open progress) and reward chests.
func (g *Game) drawChests(dst *ebiten.Image, sx, sy func(float64) float64) {
	w := g.W
	for _, m := range w.MC {
		x, y := sx(m.X), sy(m.Y)
		col := c(255, 208, 74, 255)
		switch {
		case m.Shrine:
			col = c(127, 224, 255, 255)
		case m.Cursed:
			col = c(211, 138, 255, 255)
		}
		soft(dst, x, y+12, 26, 9, c(0, 0, 0, 120))
		soft(dst, x, y, 46, 46, alpha(col, .18+.08*math.Sin(w.T*3)))
		ring(dst, x, y, 46, 1.5, alpha(col, .45))
		if m.Shrine {
			rect(dst, x-16, y-18, 32, 5, col)
			rect(dst, x-19, y-24, 38, 5, col)
			rect(dst, x-12, y-14, 5, 26, col)
			rect(dst, x+7, y-14, 5, 26, col)
		} else {
			body := c(150, 95, 50, 255)
			if m.Cursed {
				body = c(70, 40, 90, 255)
			}
			rect(dst, x-15, y-8, 30, 18, body)
			rect(dst, x-16, y-14, 32, 8, mix(body, c(255, 255, 255, 255), .15))
			rect(dst, x-16, y-6, 32, 3, col)
			rect(dst, x-3, y-9, 6, 8, col)
		}
		if m.P > 0 {
			sector(dst, x, y-36, 9, -math.Pi/2+math.Pi*m.P/5, math.Pi*m.P/5, alpha(col, .95))
			ring(dst, x, y-36, 9, 1.5, c(255, 255, 255, 160))
		}
	}
	for _, ch := range w.Chests {
		x, y := sx(ch.X), sy(ch.Y)
		soft(dst, x, y, 30, 30, c(255, 208, 74, uint8(80+40*math.Sin(w.T*5))))
		rect(dst, x-14, y-8, 28, 16, c(200, 140, 40, 255))
		rect(dst, x-15, y-13, 30, 7, c(255, 208, 74, 255))
		rect(dst, x-3, y-9, 6, 8, c(255, 240, 180, 255))
	}
}

func has(l []string, x string) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}
