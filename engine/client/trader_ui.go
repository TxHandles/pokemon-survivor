package client

import (
	"fmt"
	"image/color"
	"math"
	"strconv"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// The travelling trader: the merchant on the map, their shop screen and the rare-item icons.

var rareGold = hexC("#ffcf5a")

// shopLayout: one card per ware plus "Leave".
func (g *Game) shopLayout() [][4]float64 {
	w := g.W
	n := 1
	if w.Trader != nil {
		n += len(w.Trader.Stock)
	}
	const cw, ch, gap = 200.0, 230.0, 12.0
	tot := float64(n)*cw + float64(n-1)*gap
	x := float64(g.sw)/2 - tot/2
	y := float64(g.sh)/2 - ch/2 + 20
	var rs [][4]float64
	for i := 0; i < n; i++ {
		w := cw
		if i == n-1 {
			w = 140
		}
		rs = append(rs, [4]float64{x, y, w, ch})
		x += w + gap
	}
	return rs
}

func (g *Game) drawShop(dst *ebiten.Image) {
	w := g.W
	sw, sh := float64(g.sw), float64(g.sh)
	rect(dst, 0, 0, sw, sh, c(0, 0, 0, 190))
	rs := g.shopLayout()
	top := rs[0][1]
	tw := richLine(nil, 0, 0, 24, true, webFg, "Travelling Trader")
	richLine(dst, sw/2-tw/2, top-92, 24, true, rareGold, "Travelling Trader")
	drawText(dst, "Rare held items you can't find anywhere else · they don't use a held-item slot", sw/2, top-60, 13, false, alpha(webFg, .85), text.AlignCenter, true)
	cl := []any{"You have ", gCoin, strconv.Itoa(int(w.Coins)), " coins · spending them lowers what you bank at the end"}
	cw := richLine(nil, 0, 0, 13, true, webFg, cl...)
	richLine(dst, sw/2-cw/2, top-38, 13, true, webFg, cl...)
	mx, my := ebiten.CursorPosition()
	n := len(rs)
	for i, r := range rs {
		hot := i == g.lvSel || inR(r, float64(mx), float64(my))
		if i == n-1 {
			webBtnDraw(dst, r, hot, false, false)
			drawText(dst, fmt.Sprintf("%d. Leave", i+1), r[0]+r[2]/2, r[1]+r[3]/2-18, 15, true, webFg, text.AlignCenter, false)
			drawText(dst, "Walk away", r[0]+r[2]/2, r[1]+r[3]/2+4, 12, false, alpha(webFg, .7), text.AlignCenter, false)
			continue
		}
		wr := w.Trader.Stock[i]
		it := sim.RareByID[wr.ID]
		owned := w.Rare[wr.ID]
		can := w.CanBuy(i)
		webBtnDraw(dst, r, hot && can, false, !can)
		a := b2f(can || owned, 1, .55)
		cx := r[0] + r[2]/2
		glow(dst, cx, r[1]+52, 34, rareGold, .25*a)
		drawRareIcon(dst, wr.ID, cx, r[1]+52, 54)
		drawText(dst, fmt.Sprintf("%d. %s", i+1, it.Name), cx, r[1]+92, 15, true, alpha(rareGold, a), text.AlignCenter, false)
		y := r[1] + 116
		for _, l := range wrap(it.Desc, font(12, false), r[2]-26) {
			drawText(dst, l, cx, y, 12, false, alpha(webFg, .85*a), text.AlignCenter, false)
			y += 15
		}
		var price []any
		switch {
		case wr.Sold || owned:
			price = []any{gCheck, b2s(owned, " Owned", " Sold")}
		default:
			price = []any{gCoin, strconv.Itoa(wr.Price)}
		}
		pc := webAcc
		if !can && !owned && !wr.Sold {
			pc = hexC("#ff8a8a")
		}
		pw := richLine(nil, 0, 0, 15, true, pc, price...)
		richLine(dst, cx-pw/2, r[1]+r[3]-34, 15, true, pc, price...)
		if !can && !owned && !wr.Sold {
			drawText(dst, "Not enough coins", cx, r[1]+r[3]-14, 11, false, alpha(hexC("#ff8a8a"), .9), text.AlignCenter, false)
		}
	}
}

// drawTrader: the merchant on the map — a hooded traveller with a huge pack, a lantern and a mat
// of wares, with a timer ring showing how long they'll stay.
func (g *Game) drawTrader(dst *ebiten.Image, sx, sy func(float64) float64) {
	w := g.W
	tr := w.Trader
	if tr == nil {
		return
	}
	t := w.T
	x, y := sx(tr.X), sy(tr.Y)
	soft(dst, x, y+18, 40, 12, c(0, 0, 0, 120))
	glow(dst, x, y, 70, rareGold, .16+.06*math.Sin(t*3))
	// mat with wares
	fillEllipse(dst, x+30, y+16, 30, 9, hexC("#7a2f3a"))
	strokeEllipse(dst, x+30, y+16, 30, 9, 0, 2*math.Pi, 1.5, hexC("#d8b26a"))
	for i, wr := range tr.Stock {
		if !wr.Sold {
			drawRareIcon(dst, wr.ID, x+14+float64(i)*16, y+10, 16)
		}
	}
	// pack
	roundRect(dst, x-24, y-34, 26, 34, 6, hexC("#8a5a34"))
	roundRect(dst, x-21, y-31, 20, 8, 3, hexC("#a8743f"))
	rect(dst, x-12, y-38, 3, 6, hexC("#6b4426"))
	// body: hooded cloak
	poly(dst, [][2]float64{{x - 12, y + 14}, {x - 8, y - 14}, {x + 8, y - 14}, {x + 13, y + 14}}, hexC("#3e5a7a"))
	disc(dst, x, y-20, 10, hexC("#3e5a7a"))
	disc(dst, x+1, y-18, 6.5, hexC("#1b2433"))
	disc(dst, x-1, y-18, 1.6, hexC("#ffe08a"))
	disc(dst, x+3, y-18, 1.6, hexC("#ffe08a"))
	rect(dst, x-12, y+2, 25, 3, hexC("#d8b26a"))
	// lantern on a staff
	line(dst, x+16, y+14, x+16, y-34, 2.5, hexC("#6b4426"))
	glow(dst, x+19, y-30, 16, hexC("#ffd27a"), .6+.2*math.Sin(t*7))
	roundRect(dst, x+15, y-35, 8, 10, 2, hexC("#ffe2a0"))
	// coin over the head and the timer ring
	by := y - 52 + math.Sin(t*3)*3
	disc(dst, x, by, 9, hexC("#e6a52a"))
	disc(dst, x, by, 6.5, hexC("#ffd45a"))
	drawText(dst, "$", x, by-7, 10, true, hexC("#8a5a10"), text.AlignCenter, false)
	k := 1 - tr.T/sim.TraderStay
	strokeEllipse(dst, x, y+18, 46, 14, -math.Pi/2, -math.Pi/2+2*math.Pi*k, 2.5, alpha(rareGold, .85))
	drawText(dst, "Trader", x, y+26, 11, true, rareGold, text.AlignCenter, true)
}

// drawTraderArrow: an edge marker pointing at an off-screen trader.
func (g *Game) drawTraderArrow(dst *ebiten.Image) {
	w := g.W
	tr := w.Trader
	if tr == nil {
		return
	}
	sw, sh := float64(g.sw), float64(g.sh)
	cx, cy := g.camCenter()
	dx, dy := tr.X-cx, tr.Y-cy
	if math.Abs(dx) < sw/2-20 && math.Abs(dy) < sh/2-20 {
		return
	}
	mx, my := sw/2-70, sh/2-80
	k := math.Min(mx/math.Max(1e-6, math.Abs(dx)), my/math.Max(1e-6, math.Abs(dy)))
	x, y := sw/2+dx*k, sh/2+10+dy*k
	a := math.Atan2(dy, dx)
	pulse := .75 + .25*math.Sin(w.T*5)
	disc(dst, x, y, 22, c(16, 20, 30, 225))
	ring(dst, x, y, 22, 2.5, alpha(rareGold, pulse))
	disc(dst, x, y, 10, hexC("#e6a52a"))
	disc(dst, x, y, 7, hexC("#ffd45a"))
	drawText(dst, "$", x, y-7, 11, true, hexC("#8a5a10"), text.AlignCenter, false)
	ax, ay := x+math.Cos(a)*30, y+math.Sin(a)*30
	poly(dst, [][2]float64{{ax + math.Cos(a)*9, ay + math.Sin(a)*9}, {ax + math.Cos(a+2.2)*8, ay + math.Sin(a+2.2)*8}, {ax + math.Cos(a-2.2)*8, ay + math.Sin(a-2.2)*8}}, alpha(rareGold, pulse))
	lbl := fmt.Sprintf("Trader · %dm · leaves in %ds", int(math.Hypot(dx, dy)/10), int(sim.TraderStay-tr.T))
	ly := y + 28
	if y > sh/2 {
		ly = y - 44
	}
	drawText(dst, lbl, x, ly, 11, true, rareGold, text.AlignCenter, true)
}

// rareRow: the current player's rare items as gold-rimmed circles.
func (g *Game) rareRow(dst *ebiten.Image, order []string, x, y float64) {
	const d = 30.0
	for _, id := range order {
		id := id
		disc(dst, x+d/2, y+d/2, d/2, rareGold)
		disc(dst, x+d/2, y+d/2, d/2-2, alpha(hexC("#0a0d14"), .92))
		drawRareIcon(dst, id, x+d/2, y+d/2, d*.7)
		g.hoverTip([4]float64{x, y, d, d}, func() *tipData {
			it := sim.RareByID[id]
			t := &tipData{title: it.Name, col: rareGold}
			t.lines = append(t.lines, tipLine{"Rare item · bought from a trader", alpha(webFg, .85), 12, true})
			for _, l := range wrap(it.Desc, font(12, false), 260) {
				t.lines = append(t.lines, tipLine{l, webFg, 12, false})
			}
			return t
		})
		x += d + 6
	}
}

// drawRareIcon draws a rare item's icon centred at (cx, cy), about s pixels across.
func drawRareIcon(dst *ebiten.Image, id string, cx, cy, s float64) {
	k := s / 40
	P := func(pts ...float64) [][2]float64 {
		var out [][2]float64
		for i := 0; i+1 < len(pts); i += 2 {
			out = append(out, [2]float64{cx + pts[i]*k, cy + pts[i+1]*k})
		}
		return out
	}
	shine := func(x, y, r float64) { disc(dst, cx+x*k, cy+y*k, r*k, c(255, 255, 255, 200)) }
	switch id {
	case "souldew": // a teardrop orb, red over blue
		poly(dst, P(0, -16, 11, 2, 0, 15, -11, 2), hexC("#3f6fe0"))
		disc(dst, cx, cy+4*k, 11*k, hexC("#3f6fe0"))
		disc(dst, cx, cy-1*k, 7*k, hexC("#e0485a"))
		shine(-4, -4, 2.5)
	case "vest":
		poly(dst, P(-12, -14, -5, -10, 0, -4, 5, -10, 12, -14, 15, 14, -15, 14), hexC("#c23b3b"))
		poly(dst, P(-5, -10, 0, -4, 5, -10, 3, 14, -3, 14), hexC("#2a2f3a"))
		line(dst, cx-10*k, cy+4*k, cx+10*k, cy+4*k, 2*k, hexC("#ffd45a"))
	case "eviolite": // a faceted crystal
		poly(dst, P(0, -16, 12, -4, 7, 14, -7, 14, -12, -4), hexC("#b06ad8"))
		poly(dst, P(0, -16, 4, -4, 0, 14, -4, -4), hexC("#d8a6f2"))
		shine(-5, -6, 2)
	case "powder": // a pouch with sparkles
		disc(dst, cx, cy+4*k, 11*k, hexC("#e8c14a"))
		poly(dst, P(-6, -6, 6, -6, 9, -12, -9, -12), hexC("#c99a2a"))
		line(dst, cx-7*k, cy-6*k, cx+7*k, cy-6*k, 2*k, hexC("#7a5418"))
		for _, p := range [][2]float64{{12, -12}, {15, -4}, {-14, -10}} {
			disc(dst, cx+p[0]*k, cy+p[1]*k, 2*k, c(255, 255, 255, 230))
		}
	case "luckyegg":
		fillEllipse(dst, cx, cy+2*k, 11*k, 14*k, hexC("#f6eedc"))
		disc(dst, cx-4*k, cy-3*k, 3*k, hexC("#f08aa0"))
		disc(dst, cx+5*k, cy+5*k, 3.5*k, hexC("#f08aa0"))
		disc(dst, cx-2*k, cy+10*k, 2*k, hexC("#f08aa0"))
	case "amulet": // a coin on a ribbon
		poly(dst, P(-8, -16, -2, -2, 2, -2, 8, -16, 3, -16, 0, -8, -3, -16), hexC("#d84a4a"))
		disc(dst, cx, cy+5*k, 11*k, hexC("#e6a52a"))
		disc(dst, cx, cy+5*k, 8*k, hexC("#ffd45a"))
		shine(-4, 1, 2)
	case "shellbell": // a scallop shell
		poly(dst, P(0, 14, -15, -2, -10, -12, 0, -15, 10, -12, 15, -2), hexC("#f2b6c2"))
		for _, a := range []float64{-.9, -.45, 0, .45, .9} {
			line(dst, cx, cy+12*k, cx+math.Sin(a)*13*k, cy-math.Cos(a)*12*k, 1.4*k, hexC("#c9798a"))
		}
	case "kingsrock": // a crown
		poly(dst, P(-14, 10, -14, -8, -7, 0, 0, -12, 7, 0, 14, -8, 14, 10), hexC("#e6b23a"))
		rect(dst, cx-14*k, cy+6*k, 28*k, 5*k, hexC("#c9902a"))
		disc(dst, cx, cy+1*k, 2.5*k, hexC("#3fb0e0"))
	case "zoomlens": // a lens with a crosshair
		disc(dst, cx-2*k, cy-2*k, 12*k, hexC("#3b4458"))
		disc(dst, cx-2*k, cy-2*k, 9*k, hexC("#8fd2ff"))
		line(dst, cx-2*k, cy-9*k, cx-2*k, cy+5*k, 1.5*k, hexC("#e0485a"))
		line(dst, cx-9*k, cy-2*k, cx+5*k, cy-2*k, 1.5*k, hexC("#e0485a"))
		line(dst, cx+7*k, cy+7*k, cx+15*k, cy+15*k, 4*k, hexC("#3b4458"))
	case "policy": // a sealed scroll
		roundRect(dst, cx-12*k, cy-13*k, 24*k, 26*k, 3*k, hexC("#efe2c0"))
		for i := 0; i < 3; i++ {
			line(dst, cx-8*k, cy+(float64(i)*6-8)*k, cx+8*k, cy+(float64(i)*6-8)*k, 1.2*k, hexC("#9a8a6a"))
		}
		disc(dst, cx+6*k, cy+9*k, 5*k, hexC("#d84a4a"))
	default:
		disc(dst, cx, cy, 12*k, rareGold)
	}
}

var _ color.NRGBA

// TestTrader (screenshots): a trader next to the player; shop opens their shop.
func (g *Game) TestTrader(shop bool) {
	w := g.W
	w.TestTrader()
	if shop {
		w.Trader.X, w.Trader.Y = w.P.X+20, w.P.Y
		for i := 0; i < 120 && w.PendingKind(0) != "shop"; i++ {
			w.Step(1.0/60, sim.Input{})
		}
		g.Hold, g.auto = true, false
	}
}
