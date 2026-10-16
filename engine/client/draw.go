package client

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// playerAnim keeps the player sprite's view / facing / animation clock between frames.
type playerAnim struct {
	view  string // front | back
	fx    float64
	ani   float64
	rate  float64
	mvEnd float64
	lastT float64
}

type drawItem struct {
	y  float64
	fn func()
}

// drawWorld renders the run. cx, cy: world point at the screen centre.
func (g *Game) drawWorld(dst *ebiten.Image) {
	w := g.W
	sw, sh := float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())
	shx, shy := 0.0, 0.0
	if w.Shake > 0 {
		k := math.Min(1, w.Shake) * 7
		shx, shy = (hash01(int(w.T*60), 1, 3)-.5)*k, (hash01(int(w.T*60), 2, 5)-.5)*k
	}
	camX, camY := math.Round(w.P.X-sw/2+shx), math.Round(w.P.Y-sh/2+shy)
	sx := func(x float64) float64 { return x - camX }
	sy := func(y float64) float64 { return y - camY }
	g.floor.Draw(dst, camX, camY)

	// zones (ground effects)
	for _, z := range w.ZN {
		col := tc(z.Type)
		fade := math.Min(1, z.L/math.Max(.2, math.Min(.6, z.Life0)))
		x, y := sx(z.X), sy(z.Y)
		switch {
		case z.Mine:
			armed := z.Age >= z.Arm
			soft(dst, x, y, z.R*1.4, z.R*1.4, alpha(col, .35))
			pulse := .6 + .4*math.Sin(w.T*8+z.X)
			poly(dst, [][2]float64{{x, y - z.R*.8}, {x + z.R*.8, y}, {x, y + z.R*.8}, {x - z.R*.8, y}}, alpha(col, b2f(armed, pulse, .5)))
			ring(dst, x, y, z.R*.8, 1.5, alpha(c(255, 255, 255, 255), .5))
		case z.Pull > 0:
			soft(dst, x, y, z.R*1.2, z.R*1.2, alpha(col, .35*fade))
			for i := 0; i < 3; i++ {
				a := w.T*5 + float64(i)*2.09
				sector(dst, x, y, z.R*(.5+.2*float64(i)), a, .5, alpha(col, .35*fade))
			}
			ring(dst, x, y, z.R, 2, alpha(col, .7*fade))
		default:
			soft(dst, x, y, z.R*1.3, z.R*1.1, alpha(col, .45*fade))
		}
	}
	// gems
	for _, gm := range w.Gems {
		x, y := sx(gm.X), sy(gm.Y)
		if x < -20 || y < -20 || x > sw+20 || y > sh+20 {
			continue
		}
		col := c(110, 200, 255, 255)
		r := 4.0
		if gm.V >= 4 {
			col, r = c(120, 255, 140, 255), 5
		}
		if gm.V >= 15 {
			col, r = c(255, 110, 110, 255), 6.5
		}
		bob := math.Sin(w.T*4+gm.X*.1) * 1.5
		soft(dst, x, y+2, r*2.2, r*2.2, alpha(col, .3))
		poly(dst, [][2]float64{{x, y - r*1.3 + bob}, {x + r, y + bob}, {x, y + r*1.3 + bob}, {x - r, y + bob}}, col)
		disc(dst, x-r*.25, y-r*.4+bob, r*.3, c(255, 255, 255, 200))
	}

	// depth-sorted: obstacles, enemies, player
	var items []drawItem
	x0, x1 := int(math.Floor((camX-60)/cs)), int(math.Floor((camX+sw+60)/cs))
	y0, y1 := int(math.Floor((camY-20)/cs)), int(math.Floor((camY+sh+120)/cs))
	for j := y0; j <= y1; j++ {
		for i := x0; i <= x1; i++ {
			for _, o := range w.Chunk(i, j) {
				o := o
				items = append(items, drawItem{o.Y, func() { drawObstacle(dst, o, sx(o.X), sy(o.Y), w.Biome) }})
			}
		}
	}
	for _, e := range w.E {
		e := e
		x, y := sx(e.X), sy(e.Y)
		if x < -120 || y < -150 || x > sw+120 || y > sh+60 {
			continue
		}
		items = append(items, drawItem{e.Y + e.R, func() { g.drawEnemy(dst, e, x, y) }})
	}
	items = append(items, drawItem{w.P.Y + 12, func() { g.drawPlayer(dst, sx(w.P.X), sy(w.P.Y)) }})
	sort.SliceStable(items, func(i, j int) bool { return items[i].y < items[j].y })
	for _, it := range items {
		it.fn()
	}

	// orbits, projectiles
	for _, o := range w.Orbs {
		col := tc(o.Type)
		soft(dst, sx(o.X), sy(o.Y), o.R*2.2, o.R*2.2, alpha(col, .45))
		disc(dst, sx(o.X), sy(o.Y), o.R, col)
		disc(dst, sx(o.X)-o.R*.3, sy(o.Y)-o.R*.3, o.R*.35, c(255, 255, 255, 190))
	}
	for _, p := range w.PR {
		x, y := sx(p.X), sy(p.Y)
		if x < -30 || y < -30 || x > sw+30 || y > sh+30 {
			continue
		}
		col := tc(p.Type)
		sp := math.Hypot(p.VX, p.VY)
		if sp > 0 {
			ux, uy := p.VX/sp, p.VY/sp
			line(dst, x, y, x-ux*p.R*2.4, y-uy*p.R*2.4, p.R*1.2, alpha(col, .45))
		}
		soft(dst, x, y, p.R*2.4, p.R*2.4, alpha(col, .5))
		disc(dst, x, y, p.R, col)
		disc(dst, x, y, p.R*.45, c(255, 255, 255, 220))
	}
	// effects
	for _, f := range w.FX {
		g.drawFX(dst, f, sx, sy)
	}
	// floating damage numbers on top
	for _, f := range w.FX {
		if f.Kind != "txt" {
			continue
		}
		a := math.Min(1, f.Life/(f.Life0*.5))
		col := color.NRGBA{255, 255, 255, 255}
		size := 13
		switch f.Col {
		case "se":
			col = c(255, 208, 74, 255)
			size = 15
		case "nve":
			col = c(160, 170, 170, 255)
		case "crit":
			col, size = c(255, 90, 90, 255), 17
		case "burn":
			col = c(255, 138, 61, 255)
		case "poison":
			col = c(196, 107, 255, 255)
		case "gold":
			col, size = c(255, 208, 74, 255), 18
		case "conf":
			col = c(217, 160, 255, 255)
		}
		drawText(dst, f.Text, sx(f.X), sy(f.Y), size, true, alpha(col, a), text.AlignCenter, true)
	}
}

func b2f(b bool, t, f float64) float64 {
	if b {
		return t
	}
	return f
}

func (g *Game) drawPlayer(dst *ebiten.Image, x, y float64) {
	w := g.W
	P := w.P
	a := &g.pa
	dt := math.Max(0, math.Min(.1, w.T-a.lastT))
	a.lastT = w.T
	soft(dst, x, y+11, 15, 6, c(0, 0, 0, 120))
	// marker ring so the player stands out in a crowd of look-alike enemies
	pc := tc(P.Types[0])
	ring(dst, x, y+11, 17, 2.5, alpha(pc, .85))
	ring(dst, x, y+11, 17, 1, alpha(c(255, 255, 255, 255), .7))
	s := g.assets.Sprite(P.ID)
	if s == nil {
		disc(dst, x, y, 12, tc(P.Types[0]))
		return
	}
	mv := P.Moving
	if a.view == "" {
		a.view = "front"
	}
	thr := -.45
	if a.view == "back" {
		thr = -.15
	}
	switch {
	case mv && P.FY < thr:
		a.view = "back"
	case mv || a.view != "back" || w.T-a.mvEnd > .5:
		a.view = "front"
	}
	if mv {
		a.mvEnd = w.T
	}
	sh := s.Front
	if a.view == "back" {
		sh = s.Back
	}
	if math.Abs(P.FX) > .15 && (a.fx == 0 || P.FX*a.fx < -.3) {
		a.fx = b2f(P.FX < 0, -1, 1)
	}
	if a.fx == 0 {
		a.fx = 1
	}
	fl := 1.0
	if sh.V.Faces == "left" {
		fl = b2f(a.fx > 0, -1, 1)
	} else {
		fl = b2f(a.fx < 0, -1, 1)
	}
	if a.rate == 0 {
		a.rate = 1
	}
	a.rate += (b2f(mv, 1.7, 1) - a.rate) * math.Min(1, dt*6)
	a.ani += dt * 1000 * a.rate
	at := w.T - P.AtkT
	aDur := math.Min(1, float64(sh.atot)/1000)
	fr := sh.IdleFrame(int(a.ani))
	if at < aDur && len(sh.V.Act) > 0 {
		fr = sh.ActFrame(int(at / aDur * float64(sh.atot)))
	}
	wk := math.Min(1, math.Max(0, (a.rate-1)/.7))
	cy := math.Sin(w.T * 16)
	bob := -math.Abs(cy) * 2 * wk
	ssx, ssy := 1+(1-math.Abs(cy))*.025*wk, 1-(1-math.Abs(cy))*.04*wk
	if !mv {
		ssy *= 1 + math.Sin(w.T*3)*.012
	}
	alphaV := 1.0
	if P.Inv > 0 && int(w.T*20)%2 == 1 && P.Dash == nil {
		alphaV = .45
	}
	g.blit(dst, sh, fr, x, y+12*.95+bob, s.Scale*fl*ssx, s.Scale*ssy, alphaV, w.T-P.HurtT < .16)
}

// blit draws frame fr of sheet sh with its feet at (x,y), scaled (negative sx flips).
func (g *Game) blit(dst *ebiten.Image, sh *Sheet, fr int, x, y, scx, scy, a float64, flash bool) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(sh.V.OX, sh.V.OY)
	op.GeoM.Scale(scx, scy)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(float32(a))
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(sh.Frame(fr), op)
	if flash {
		op.ColorScale.Reset()
		op.ColorScale.Scale(1, .25, .25, float32(.75*a))
		op.Blend = ebiten.BlendLighter
		dst.DrawImage(sh.Frame(fr), op)
	}
}

func (g *Game) drawEnemy(dst *ebiten.Image, e *sim.Enemy, x, y float64) {
	w := g.W
	s := g.assets.Sprite(e.PokeID)
	soft(dst, x, y+e.R*.85, e.R*1.35, e.R*.5, c(0, 0, 0, 110))
	if e.Boss {
		ring(dst, x, y+e.R*.85, e.R*1.2, 2, alpha(tc(e.Type), .6+.3*math.Sin(w.T*5)))
	}
	if s == nil {
		disc(dst, x, y, e.R, tc(e.Type))
	} else {
		sh := s.Front
		ss := .82
		if p := g.db.ByID[e.PokeID]; p != nil && p.SpriteScale > 0 {
			ss = p.SpriteScale
		}
		sc := s.Scale * e.R / 12 * ss
		fl := 1.0
		if (sh.V.Faces == "left") != e.FaceL {
			fl = -1
		}
		fr := sh.IdleFrame(int(w.T*1000 + e.Seed*5000))
		a := 1.0
		g.blit(dst, sh, fr, x, y+e.R*.95, sc*fl, sc, a, false)
		if e.Flash > 0 {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(sh.V.OX, sh.V.OY)
			op.GeoM.Scale(sc*fl, sc)
			op.GeoM.Translate(x, y+e.R*.95)
			op.ColorScale.Scale(1, 1, 1, .7)
			op.Blend = ebiten.BlendLighter
			op.Filter = ebiten.FilterLinear
			dst.DrawImage(sh.Frame(fr), op)
		}
	}
	// status marks
	switch {
	case e.Frz > 0:
		soft(dst, x, y, e.R*1.6, e.R*1.6, c(159, 240, 255, 110))
	case e.Burn > 0 && int(w.T*6+e.Seed*9)%3 == 0:
		disc(dst, x+(e.Seed-.5)*e.R, y-e.R*.4, 2.2, c(255, 138, 61, 220))
	case e.Psn > 0 && int(w.T*6+e.Seed*9)%3 == 0:
		disc(dst, x+(e.Seed-.5)*e.R, y-e.R*.6, 2.2, c(196, 107, 255, 220))
	case e.Stun > 0:
		line(dst, x-6, y-e.R*1.6, x, y-e.R*1.3, 1.5, c(255, 224, 61, 255))
		line(dst, x, y-e.R*1.3, x+5, y-e.R*1.7, 1.5, c(255, 224, 61, 255))
	case e.Fear > 0 || e.Charm > 0 || e.Conf > 0:
		disc(dst, x, y-e.R*1.5, 2.5, c(255, 179, 230, 230))
	}
	if e.Boss || (e.HP < e.MHP && e.Big) {
		bw := e.R * 2
		if e.Boss {
			bw = 80
		}
		by := y - e.R*2.2
		if e.Boss {
			by = y - e.R*2.6
		}
		rect(dst, x-bw/2, by, bw, 5, c(0, 0, 0, 160))
		rect(dst, x-bw/2, by, bw*math.Max(0, e.HP/e.MHP), 5, c(255, 80, 80, 255))
	}
}

func (g *Game) drawFX(dst *ebiten.Image, f *sim.FX, sx, sy func(float64) float64) {
	col := tc(f.Type)
	k := 1 - f.Life/math.Max(f.Life0, 1e-6) // 0 -> 1 over the effect's life
	a := math.Min(1, f.Life/math.Max(f.Life0*.6, 1e-6))
	x, y := sx(f.X), sy(f.Y)
	switch f.Kind {
	case "ring":
		if f.Sk != "" {
			col = c(255, 240, 140, 255)
			if f.Sk == "sl" {
				col = c(140, 220, 255, 255)
			}
		}
		if f.Fat {
			soft(dst, x, y, f.R*1.15, f.R*1.15, alpha(col, .5*a))
			ring(dst, x, y, f.R, 3, alpha(col, .8*a))
		} else {
			ring(dst, x, y, f.R, math.Max(3, f.TW*.8), alpha(col, .55*a))
			ring(dst, x, y, f.R, 1.5, alpha(c(255, 255, 255, 255), .6*a))
		}
	case "arc":
		sector(dst, x, y, f.R, f.A, f.Wd, alpha(col, .35*a))
		sector(dst, x, y, f.R, f.A, f.Wd*.5, alpha(mix(col, c(255, 255, 255, 255), .4), .3*a))
	case "rect":
		orect(dst, x, y, f.A, f.Ln, f.Wd, alpha(col, .4*a))
	case "beam":
		orect(dst, x, y, f.A, f.Ln, f.Wd, alpha(col, .55*a))
		orect(dst, x, y, f.A, f.Ln, f.Wd*.35, alpha(c(255, 255, 255, 255), .7*a))
	case "bt":
		orect(dst, x, y, f.A, f.Ln, 2, alpha(col, .5))
	case "tele":
		ring(dst, x, y, f.R, 1.5, alpha(col, .8))
		disc(dst, x, y, f.R*k, alpha(col, .22))
	case "puff":
		r := f.R * (.6 + .4*k)
		soft(dst, x, y, r*1.3, r*1.3, alpha(col, .7*a))
		ring(dst, x, y, r, 2.5, alpha(col, .8*a))
	case "sky":
		line(dst, x, y-260, x, y, 6*(1-k)+1, alpha(col, .8*a))
		line(dst, x, y-260, x, y, 2, alpha(c(255, 255, 255, 255), a))
	case "bolt":
		ax, ay, bx, by := sx(f.AX), sy(f.AY), sx(f.BX), sy(f.BY)
		px, py := ax, ay
		dx, dy := bx-ax, by-ay
		l := math.Hypot(dx, dy)
		nx, ny := -dy/math.Max(l, 1), dx/math.Max(l, 1)
		for i := 1; i <= 5; i++ {
			t := float64(i) / 5
			j := 0.0
			if i < 5 {
				j = (hash01(int(f.AX), int(f.BY), i) - .5) * 14
			}
			qx, qy := ax+dx*t+nx*j, ay+dy*t+ny*j
			line(dst, px, py, qx, qy, 4, alpha(col, .6*a))
			line(dst, px, py, qx, qy, 1.5, alpha(c(255, 255, 255, 255), a))
			px, py = qx, qy
		}
	case "pop":
		ring(dst, x, y, f.R*(1+k*1.2), 2, alpha(col, 1-k))
	}
}

func drawText(dst *ebiten.Image, s string, x, y float64, size int, bold bool, col color.NRGBA, align text.Align, outline bool) {
	f := font(size, bold)
	op := &text.DrawOptions{}
	op.PrimaryAlign = align
	if outline {
		o := math.Max(1, float64(size)/12)
		for _, d := range [][2]float64{{-o, 0}, {o, 0}, {0, -o}, {0, o}} {
			op.GeoM.Reset()
			op.GeoM.Translate(x+d[0], y+d[1])
			op.ColorScale.Reset()
			op.ColorScale.ScaleWithColor(color.NRGBA{0, 0, 0, uint8(float64(col.A) * .85)})
			text.Draw(dst, s, f, op)
		}
	}
	op.GeoM.Reset()
	op.GeoM.Translate(x, y)
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(col)
	text.Draw(dst, s, f, op)
}
