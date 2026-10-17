package client

import (
	"math"
	"strconv"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Visuals for the Go-only move mechanics (moves_go.go): Fly, Rollout, Protect / Detect, Earthquake,
// Magnitude, Leech Seed, Hypnosis / Sing and Substitute.

// flyHeight: how high a flying player is drawn (rise, hover, dive).
func flyHeight(F *sim.FlyState) float64 {
	el := F.Du - F.T
	switch {
	case el < .25:
		k := el / .25
		return 70 * (1 - (1-k)*(1-k))
	case F.T < .18:
		return 70 * F.T / .18
	}
	return 70 + 4*math.Sin(el*9)
}

// drawFlyTarget: the landing zone under a flying player, bold enough to aim with.
func (g *Game) drawFlyTarget(dst *ebiten.Image, P *sim.Player, x, y float64) {
	F := P.Fly
	t := g.W.T
	col := tc(F.Type)
	k := 1 - F.T/F.Du // 0 → 1 as the landing nears
	R := F.R
	fillEllipse(dst, x, y, R, R*.62, alpha(col, .16+.14*k))
	soft(dst, x, y, R*.55, R*.35, c(0, 0, 0, uint8(90+80*k)))
	strokeEllipse(dst, x, y, R, R*.62, 0, 2*math.Pi, 3, alpha(col, .9))
	strokeEllipse(dst, x, y, R, R*.62, 0, 2*math.Pi, 1.2, alpha(white, .8))
	// countdown arc closing round the rim
	strokeEllipse(dst, x, y, R+6, R*.62+4, -math.Pi/2, -math.Pi/2+2*math.Pi*k, 4, alpha(white, .9))
	// rotating corner brackets that tighten as you come down
	rr := R * (1.25 - .2*k)
	for i := 0; i < 4; i++ {
		a := t*1.5 + float64(i)*math.Pi/2
		strokeEllipse(dst, x, y, rr, rr*.62, a-.3, a+.3, 3, alpha(col, .95))
	}
	for i := 0; i < 4; i++ { // crosshair ticks
		a := float64(i) * math.Pi / 2
		cx, cy := math.Cos(a), math.Sin(a)*.62
		line(dst, x+cx*R*.25, y+cy*R*.25, x+cx*R*.55, y+cy*R*.55, 2, alpha(white, .85))
	}
	disc(dst, x, y, 3, alpha(white, .9))
}

// drawRollBall: a curled-up, rolling player.
func (g *Game) drawRollBall(dst *ebiten.Image, P *sim.Player, x, y float64) {
	R := P.Roll
	t := g.W.T
	col := tc(R.Type)
	dk := mix(col, c(0, 0, 0, 255), .45)
	ca, sa := math.Cos(R.A), math.Sin(R.A)
	for i := 1; i <= 5; i++ { // dust kicked up behind
		fi := float64(i)
		ph := math.Mod(t*6+fi*.21, 1)
		d := 10 + fi*7 + ph*6
		soft(dst, x-ca*d+math.Sin(fi*3+t*9)*4, y+8-sa*d*.6, 7+fi*1.5, 4+fi, c(190, 165, 125, uint8(110*(1-fi/6))))
	}
	for i := 0; i < 3; i++ { // speed streaks
		off := (float64(i) - 1) * 7
		line(dst, x-ca*16-sa*off, y-sa*16+ca*off, x-ca*(30+float64(R.N)*1.5)-sa*off, y-sa*(30+float64(R.N)*1.5)+ca*off, 2, alpha(white, .4))
	}
	r := 17.0
	soft(dst, x, y+11, 17, 6, c(0, 0, 0, 120))
	by := y - 2 + math.Abs(math.Sin(t*14))*-2
	disc(dst, x, by, r+1.5, dk)
	disc(dst, x, by, r, col)
	rot := t * (12 + float64(R.N)*.6)
	if ca < 0 {
		rot = -rot
	}
	for i := 0; i < 3; i++ { // bands rolling round the ball
		a := rot + float64(i)*2.09
		off := math.Sin(a) * r * .85
		if math.Cos(a) > 0 {
			strokeEllipse(dst, x+off*.15, by, math.Abs(math.Cos(a))*r*.25+1.5, r*.92, 0, 2*math.Pi, 2.2, alpha(dk, .85))
		}
	}
	disc(dst, x-r*.35, by-r*.4, r*.28, alpha(white, .55))
	if R.N > 0 {
		drawText(dst, "x"+strconv.Itoa(R.N), x, by-r-20, 12, true, mix(col, white, .5), text.AlignCenter, true)
	}
	// time left
	k := R.T / R.Du
	strokeEllipse(dst, x, y+11, 19, 8, -math.Pi/2, -math.Pi/2+2*math.Pi*k, 2.5, alpha(white, .8))
}

// drawProtect: the shield bubble of Protect / Detect.
func (g *Game) drawProtect(dst *ebiten.Image, P *sim.Player, x, y float64) {
	t := g.W.T
	col := tc(P.ProtType)
	if P.ProtType == "normal" {
		col = c(150, 235, 190, 255)
	}
	al := math.Min(1, P.Prot*3)
	if P.Prot < .35 && math.Mod(t*20, 2) < 1 {
		al *= .4
	}
	R := 30.0
	cy := y - 4
	glow(dst, x, cy, R*1.2, col, .35*al)
	fillEllipse(dst, x, cy, R, R, alpha(col, .12*al))
	// hexagon facets turning slowly
	for ring_ := 0; ring_ < 2; ring_++ {
		rr := R * (.55 + .4*float64(ring_))
		pts := make([][2]float64, 0, 7)
		for i := 0; i <= 6; i++ {
			a := t*.6*b2f(ring_ == 0, 1, -1) + float64(i)*math.Pi/3
			pts = append(pts, [2]float64{x + math.Cos(a)*rr, cy + math.Sin(a)*rr})
		}
		polyline(dst, pts, 1.4, alpha(white, .35*al))
	}
	strokeEllipse(dst, x, cy, R, R, 0, 2*math.Pi, 2.5, alpha(col, .9*al))
	sa := t * 3
	strokeEllipse(dst, x, cy, R, R, sa, sa+.9, 3, alpha(white, .9*al))
	disc(dst, x-R*.4, cy-R*.45, 4, alpha(white, .6*al))
}

// drawDecoy: Substitute's training dummy.
func (g *Game) drawDecoy(dst *ebiten.Image, d *sim.Decoy, x, y float64) {
	t := g.W.T
	col := tc(d.Type)
	k := d.T / d.Du
	sh := 0.0
	if d.T < .6 {
		sh = math.Sin(t*60) * 2
	}
	x += sh + math.Sin(t*5)*1.2
	soft(dst, x, y+14, 16, 6, c(0, 0, 0, 120))
	glow(dst, x, y-6, 26, col, .25+.15*math.Sin(t*6))
	rect(dst, x-2, y-2, 4, 16, c(110, 76, 44, 255)) // post
	rect(dst, x-10, y+12, 20, 3, c(90, 62, 36, 255))
	// sack body with a painted target
	fillEllipse(dst, x, y-6, 12, 14, c(196, 160, 104, 255))
	strokeEllipse(dst, x, y-6, 12, 14, 0, 2*math.Pi, 1.5, c(120, 86, 48, 255))
	ring(dst, x, y-5, 7, 2, alpha(col, .9))
	disc(dst, x, y-5, 2.5, alpha(col, .9))
	line(dst, x-11, y-12, x+11, y-12, 2, c(140, 100, 56, 255)) // rope
	// head with stitched eyes
	disc(dst, x, y-25, 8, c(206, 172, 116, 255))
	ring(dst, x, y-25, 8, 1.3, c(120, 86, 48, 255))
	for _, ex := range []float64{-3, 3} {
		line(dst, x+ex-1.5, y-27, x+ex+1.5, y-24, 1.2, c(70, 48, 28, 255))
		line(dst, x+ex+1.5, y-27, x+ex-1.5, y-24, 1.2, c(70, 48, 28, 255))
	}
	// time left
	rect(dst, x-12, y+18, 24, 3, c(0, 0, 0, 140))
	rect(dst, x-12, y+18, 24*k, 3, alpha(col, .95))
}

// drawEnemyMarks: sleep and Leech Seed on a foe.
func (g *Game) drawEnemyMarks(dst *ebiten.Image, e *sim.Enemy, x, y float64) {
	t := g.W.T
	if e.Slp > 0 {
		for i := 0; i < 3; i++ {
			ph := math.Mod(t*.8+float64(i)/3+e.Seed, 1)
			zx := x + e.R*.6 + ph*10
			zy := y - e.R - 4 - ph*18
			drawText(dst, "z", zx, zy, 9+int(ph*6), true, c(200, 210, 255, uint8(255*(1-ph))), text.AlignCenter, true)
		}
	}
	if e.LS > 0 {
		gc := c(110, 220, 90, 255)
		sy := y - e.R - 4
		soft(dst, x, y, e.R*1.8, e.R*1.6, alpha(gc, .25+.1*math.Sin(t*5+e.Seed*9)))
		for i := 0; i < 3; i++ { // vines wrapping the foe
			a := t*2 + float64(i)*2.1
			strokeEllipse(dst, x, y+2, e.R+3, e.R*.6+3, a, a+1.3, 3, c(40, 110, 40, 255))
			strokeEllipse(dst, x, y+2, e.R+3, e.R*.6+3, a, a+1.3, 1.5, gc)
			lx, ly := x+math.Cos(a+1.3)*(e.R+3), y+2+math.Sin(a+1.3)*(e.R*.6+3)
			fillEllipse(dst, lx, ly, 3.5, 2, gc)
		}
		line(dst, x, sy+6, x, sy-5, 2.5, c(70, 130, 50, 255))
		fillEllipse(dst, x-5, sy-6, 5.5, 3, gc)
		fillEllipse(dst, x+5, sy-8, 5.5, 3, mix(gc, white, .25))
		if math.Mod(t*2+e.Seed*3, 1) < .5 { // drain sparkle
			ph := math.Mod(t*2+e.Seed*3, .5) * 2
			disc(dst, x+math.Sin(e.Seed*20)*e.R*.6, y-ph*20, 2.2, alpha(c(170, 255, 140, 255), 1-ph))
		}
	}
}

// drawCracks: Earthquake fissures, glowing and fading.
func (g *Game) drawCracks(dst *ebiten.Image, sx, sy func(float64) float64) {
	t := g.W.T
	for _, cr := range g.W.Cracks {
		al := math.Min(1, cr.L/.6)
		hot := math.Max(0, (cr.L-cr.L0+1)/1) // white-hot for the first second
		pts := make([][2]float64, len(cr.Pts))
		for i, p := range cr.Pts {
			pts[i] = [2]float64{sx(p[0]), sy(p[1])}
		}
		polyline(dst, pts, 9, c(20, 12, 6, uint8(200*al)))
		polyline(dst, pts, 5, c(120, 62, 24, uint8(230*al)))
		fl := .7 + .3*math.Sin(t*14+pts[0][0])
		polyline(dst, pts, 2.5, alpha(mix(c(255, 140, 40, 255), c(255, 236, 170, 255), hot), al*fl))
		for i := 1; i < len(pts); i += 2 { // rubble along the edges
			p := pts[i]
			disc(dst, p[0]+4, p[1]+3, 2.5, c(90, 70, 50, uint8(220*al)))
			disc(dst, p[0]-3, p[1]-4, 2, c(120, 95, 70, uint8(220*al)))
		}
	}
}

// drawGoFX draws the new effect kinds; false = not one of them.
func (g *Game) drawGoFX(dst *ebiten.Image, f *sim.FX, sx, sy func(float64) float64) bool {
	t := g.W.T
	x, y := sx(f.X), sy(f.Y)
	k := 1 - f.Life/math.Max(f.Life0, 1e-6)
	al := math.Min(1, f.Life/math.Max(f.Life0*.5, 1e-6))
	col := tc(f.Type)
	switch f.Kind {
	case "quake":
		// ground ripples rolling outward, dust walls, and flying clods
		for i := 0; i < 4; i++ {
			kk := k*1.3 - float64(i)*.12
			if kk <= 0 || kk > 1 {
				continue
			}
			r := f.R * (.15 + .95*kk)
			strokeEllipse(dst, x, y, r, r*.62, 0, 2*math.Pi, 10*(1-kk)+2, c(110, 80, 50, uint8(170*(1-kk))))
			strokeEllipse(dst, x, y-2, r, r*.62, 0, 2*math.Pi, 2, c(230, 200, 150, uint8(200*(1-kk))))
		}
		for i := 0; i < 14; i++ {
			fi := float64(i)
			a := fi*.449 + hsh(fi)*.4
			d := f.R * (.2 + .9*k) * (.7 + .3*hsh(fi+5))
			h := math.Sin(math.Min(1, k*1.4)*math.Pi) * (30 + 30*hsh(fi+2))
			px, py := x+math.Cos(a)*d, y+math.Sin(a)*d*.62-h
			disc(dst, px, py, 3+2*hsh(fi+7), c(120, 88, 56, uint8(255*al)))
			soft(dst, x+math.Cos(a)*d, y+math.Sin(a)*d*.62, 14, 6, c(170, 140, 100, uint8(80*al)))
		}
		if k < .25 { // the first jolt: a bright flash at the centre
			glow(dst, x, y, f.R*.5, c(255, 220, 160, 255), (1-k/.25)*.6)
		}
		return true
	case "mag":
		m := f.D
		mk := (m - 3) / 7
		// screen-edge pulse (shows the quake even with screen shake off)
		sw, sh := float64(g.sw), float64(g.sh)
		pulse := al * (.25 + .5*mk) * (.6 + .4*math.Sin(t*30))
		ec := mix(col, c(255, 200, 120, 255), .4)
		th := 18 + 40*mk
		for i := 0; i < 4; i++ {
			fi := float64(i)
			a := pulse * (1 - fi/4) * .5
			w := th * (fi + 1) / 4
			rect(dst, 0, 0, sw, w, alpha(ec, a))
			rect(dst, 0, sh-w, sw, w, alpha(ec, a))
			rect(dst, 0, 0, w, sh, alpha(ec, a))
			rect(dst, sw-w, 0, w, sh, alpha(ec, a))
		}
		// radial cracks, one per magnitude point
		n := int(m)
		for i := 0; i < n; i++ {
			fi := float64(i)
			a := fi*6.283/m + hsh(fi+m)*.4
			L := f.R * math.Min(1, k*3) * (.6 + .4*hsh(fi+1))
			mx, my := x+math.Cos(a+.15)*L*.5, y+math.Sin(a+.15)*L*.5*.62
			ex, ey := x+math.Cos(a)*L, y+math.Sin(a)*L*.62
			polyline(dst, [][2]float64{{x, y}, {mx, my}, {ex, ey}}, 4, c(40, 26, 12, uint8(220*al)))
			polyline(dst, [][2]float64{{x, y}, {mx, my}, {ex, ey}}, 1.5, alpha(c(255, 190, 90, 255), al))
		}
		for i := 0; i < n-3; i++ { // shock rings, more for bigger quakes
			kk := k*1.5 - float64(i)*.1
			if kk <= 0 || kk > 1 {
				continue
			}
			r := f.R * kk
			strokeEllipse(dst, x, y, r, r*.62, 0, 2*math.Pi, 3, alpha(col, .8*(1-kk)))
		}
		// the number itself
		pop := math.Min(1, k*6)
		sz := int(18 + 14*mk + 6*(1-pop))
		ty := y - 70 - 10*k
		drawText(dst, "MAGNITUDE", x, ty-float64(sz)*.9, 12, true, alpha(white, al), text.AlignCenter, true)
		drawText(dst, f.Text, x, ty, sz, true, alpha(mix(col, c(255, 220, 120, 255), mk), al), text.AlignCenter, true)
		return true
	case "lull":
		// wavy rings of sound / hypnotic waves spreading out (a cone for Hypnosis)
		hyp := f.Wd < 3
		if f.Type == "normal" {
			col = c(255, 170, 220, 255) // a lullaby pink
		}
		for i := 0; i < 3; i++ {
			kk := k*1.3 - float64(i)*.15
			if kk <= 0 || kk > 1 {
				continue
			}
			r := f.R * kk
			const N = 40
			pts := make([][2]float64, 0, N+1)
			for j := 0; j <= N; j++ {
				a := f.A - f.Wd + 2*f.Wd*float64(j)/N
				rr := r + 4*math.Sin(a*12+t*10)
				pts = append(pts, [2]float64{x + math.Cos(a)*rr, y + math.Sin(a)*rr*.75})
			}
			polyline(dst, pts, 5, alpha(col, .8*(1-kk)))
			polyline(dst, pts, 1.6, alpha(white, .75*(1-kk)))
		}
		for i := 0; i < 6; i++ {
			fi := float64(i)
			a := f.A - f.Wd*.8 + 1.6*f.Wd*hsh(fi+3)
			d := f.R * (.2 + .8*k) * (.6 + .4*hsh(fi))
			px, py := x+math.Cos(a)*d, y+math.Sin(a)*d*.75-math.Sin(t*6+fi)*4
			if hyp { // spirals
				for j := 0; j < 3; j++ {
					rr := 2 + float64(j)*2.5
					strokeEllipse(dst, px, py, rr, rr, t*4+float64(j), t*4+float64(j)+4, 1.4, alpha(col, al))
				}
			} else { // music notes
				nc := alpha(mix(col, white, .3), al)
				fillEllipse(dst, px, py, 3.5, 2.6, nc)
				line(dst, px+3, py, px+3, py-11, 1.5, nc)
				line(dst, px+3, py-11, px+7, py-8, 1.5, nc)
			}
		}
		return true
	case "lob": // a seed arcing onto its target
		px := f.AX + (f.BX-f.AX)*k
		py := f.AY + (f.BY-f.AY)*k
		h := math.Sin(k*math.Pi) * 40
		X, Y := sx(px), sy(py)
		soft(dst, X, Y+4, 7, 3.5, c(0, 0, 0, 110))
		for i := 1; i <= 4; i++ { // green trail
			kk := math.Max(0, k-float64(i)*.06)
			tx, ty := sx(f.AX+(f.BX-f.AX)*kk), sy(f.AY+(f.BY-f.AY)*kk)-math.Sin(kk*math.Pi)*40
			disc(dst, tx, ty, 4-float64(i)*.6, c(140, 230, 110, uint8(180-40*i)))
		}
		glow(dst, X, Y-h, 12, c(140, 240, 110, 255), .5)
		fillEllipse(dst, X, Y-h, 6.5, 5, c(120, 90, 45, 255))
		fillEllipse(dst, X-1.5, Y-h-1.5, 3.5, 2.5, c(200, 235, 120, 255))
		return true
	case "slam": // Fly landing: crater, dust ring and radial streaks
		R := f.R
		strokeEllipse(dst, x, y, R*(.6+.5*k), R*(.6+.5*k)*.62, 0, 2*math.Pi, 8*(1-k)+1, c(200, 175, 135, uint8(200*al)))
		fillEllipse(dst, x, y, R*.45, R*.28, c(60, 42, 26, uint8(140*al)))
		for i := 0; i < 10; i++ {
			a := float64(i) * .628
			d0, d1 := R*(.3+.5*k), R*(.6+.6*k)
			line(dst, x+math.Cos(a)*d0, y+math.Sin(a)*d0*.62, x+math.Cos(a)*d1, y+math.Sin(a)*d1*.62, 2.5, alpha(mix(col, white, .5), al))
		}
		return true
	}
	return false
}

// drawPlayerOverlays: Protect bubbles on top of everything at the players' positions.
func (g *Game) drawPlayerOverlays(dst *ebiten.Image, sx, sy func(float64) float64) {
	for _, pc := range g.W.PL {
		if P := pc.P; P != nil && !P.Dead && P.Prot > 0 {
			g.drawProtect(dst, P, sx(P.X), sy(P.Y))
		}
	}
}
