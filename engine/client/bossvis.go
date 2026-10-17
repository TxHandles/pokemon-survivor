package client

import (
	"image/color"
	"math"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
	"github.com/hajimehoshi/ebiten/v2"
)

// Boss and elite attack visuals: every telegraph shows what is coming (falling meteors, a leaping
// shadow, cracks spreading, a portal opening…) and every impact gets its own burst.

// drawBossHz draws one telegraphed hazard by kind; false = not handled (plain circle).
func (g *Game) drawBossHz(dst *ebiten.Image, h *sim.Hazard, x, y float64) bool {
	t := g.W.T
	col := tc(h.Type)
	if h.Type == "" {
		col = c(255, 110, 90, 255)
	}
	if h.Col != "" {
		col = rgbStr(h.Col)
	}
	danger := c(255, 70, 60, 255)
	R := h.R
	if !h.Gone {
		k := 1 - math.Max(0, h.T)/math.Max(h.T0, .01) // 0 → 1 until it lands
		pulse := .65 + .35*math.Sin(t*14)
		// the shared base: tinted zone, closing countdown ring, red edge that flashes faster near the end
		fillEllipse(dst, x, y, R, R*.62, alpha(col, .1+.12*k))
		strokeEllipse(dst, x, y, R*(1-k*.85)+4, (R*(1-k*.85)+4)*.62, 0, 2*math.Pi, 2, alpha(white, .55))
		edge := alpha(danger, .55+.4*b2f(k > .7, pulse, 1))
		strokeEllipse(dst, x, y, R, R*.62, 0, 2*math.Pi, 2.5, edge)
		switch h.K {
		case "rain": // a meteor falling onto the spot
			soft(dst, x, y, R*.6*k+4, R*.35*k+2, c(0, 0, 0, uint8(120*k)))
			my := y - 260*(1-k)
			mx := x + 90*(1-k)
			for i := 1; i <= 5; i++ {
				fi := float64(i)
				disc(dst, mx+fi*9*(1-k*.3), my-fi*22, 9-fi*1.3, alpha(mix(col, white, .2), .5-fi*.08))
			}
			glow(dst, mx, my, 20, col, .7)
			disc(dst, mx, my, 9, mix(col, c(60, 30, 20, 255), .3))
			disc(dst, mx-3, my-3, 4, alpha(white, .8))
		case "quake", "pulse": // cracks race out from the boss
			n := 7
			for i := 0; i < n; i++ {
				a := float64(i)*6.283/float64(n) + float64(int(h.X)%7)*.3
				L := R * k
				pts := [][2]float64{{x, y}}
				for j := 1; j <= 4; j++ {
					f := float64(j) / 4
					ja := a + math.Sin(float64(j)*2.3+float64(i))*.25
					pts = append(pts, [2]float64{x + math.Cos(ja)*L*f, y + math.Sin(ja)*L*f*.62})
				}
				polyline(dst, pts, 4, c(30, 18, 10, 200))
				polyline(dst, pts, 1.6, alpha(mix(col, white, .3), .9))
			}
			if h.K == "pulse" {
				for i := 0; i < 3; i++ {
					kk := math.Mod(k*2+float64(i)/3, 1)
					strokeEllipse(dst, x, y, R*kk, R*kk*.62, 0, 2*math.Pi, 2, alpha(col, .8*(1-kk)))
				}
			}
		case "leap", "slam", "slam2": // the attacker's shadow grows as it comes down, chevrons close in
			soft(dst, x, y, R*.75*k+6, R*.45*k+4, c(0, 0, 0, uint8(60+120*k)))
			for i := 0; i < 4; i++ {
				a := float64(i)*math.Pi/2 + math.Pi/4
				d := R * (1.25 - .55*k)
				cx, cy := x+math.Cos(a)*d, y+math.Sin(a)*d*.62
				ia := a + math.Pi
				poly(dst, [][2]float64{{cx + math.Cos(ia)*12, cy + math.Sin(ia)*12*.62}, {cx + math.Cos(ia+2.4)*9, cy + math.Sin(ia+2.4)*9*.62}, {cx + math.Cos(ia-2.4)*9, cy + math.Sin(ia-2.4)*9*.62}}, alpha(danger, .85))
			}
		case "pillars": // the ground bulges where a pillar is about to burst out
			for i := 0; i < 5; i++ {
				a := float64(i) * 1.26
				d := R * .4 * k
				disc(dst, x+math.Cos(a)*d, y+math.Sin(a)*d*.62, 3+3*k, c(90, 70, 50, uint8(200*k)))
			}
			fillEllipse(dst, x, y-4*k, R*.4*k+2, R*.25*k+1, alpha(mix(col, c(0, 0, 0, 255), .3), .7))
		case "blink": // a portal opens where the boss will appear
			for i := 0; i < 4; i++ {
				rr := R * (.25 + .2*float64(i)) * (.5 + .5*k)
				a := t*(4-float64(i)) + float64(i)
				strokeEllipse(dst, x, y, rr, rr*.62, a, a+3.6, 2.5, alpha(col, .9))
			}
			glow(dst, x, y, R*.5*k, col, .6)
		case "trail": // burning ground left by a charge
			for i := 0; i < 4; i++ {
				fi := float64(i)
				fx := x + math.Sin(fi*2.1+h.X)*R*.4
				fy := y + math.Cos(fi*1.7+h.Y)*R*.25
				fh := 8 + 6*math.Sin(t*12+fi*1.9)
				poly(dst, [][2]float64{{fx - 5, fy}, {fx, fy - fh}, {fx + 5, fy}}, alpha(mix(col, c(255, 220, 120, 255), .4), .8))
			}
		case "expl", "burst": // a sizzling fuse
			glow(dst, x, y, R*.4, c(255, 150, 60, 255), .4+.4*pulse*k)
		}
		return true
	}
	// impacts
	a := math.Max(0, h.Fl/.45)
	k := 1 - a // 0 → 1 over the impact
	switch h.K {
	case "rain", "expl", "burst":
		glow(dst, x, y, R*(.6+.6*k), mix(col, c(255, 200, 120, 255), .4), a)
		for i := 0; i < 10; i++ {
			fi := float64(i)
			an := fi*.628 + h.X*.01
			d := R * (.3 + 1.1*k)
			hh := math.Sin(k*math.Pi) * 26
			disc(dst, x+math.Cos(an)*d, y+math.Sin(an)*d*.62-hh, 3.5*(1-k)+1, alpha(mix(col, c(80, 60, 40, 255), .4), a))
		}
		fillEllipse(dst, x, y, R*.55, R*.33, c(30, 20, 14, uint8(150*a)))
	case "quake", "pulse", "leap", "slam", "slam2":
		for i := 0; i < 2; i++ {
			kk := math.Min(1, k*1.4-float64(i)*.25)
			if kk > 0 {
				strokeEllipse(dst, x, y, R*(.4+.9*kk), R*(.4+.9*kk)*.62, 0, 2*math.Pi, 8*(1-kk)+2, c(200, 175, 135, uint8(220*(1-kk))))
			}
		}
		fillEllipse(dst, x, y, R*.6, R*.36, c(40, 28, 18, uint8(160*a)))
		for i := 0; i < 12; i++ {
			an := float64(i) * .5236
			d0, d1 := R*(.35+.4*k), R*(.75+.6*k)
			line(dst, x+math.Cos(an)*d0, y+math.Sin(an)*d0*.62, x+math.Cos(an)*d1, y+math.Sin(an)*d1*.62, 3, alpha(mix(col, white, .4), a))
		}
	case "pillars": // a pillar erupts from the ground
		hh := 90 * math.Sin(math.Min(1, k*2.2)*math.Pi/2) * (1 - math.Max(0, k-.6)/.4*.6)
		w := R * .55
		rect(dst, x-w/2, y-hh, w, hh, alpha(mix(col, c(0, 0, 0, 255), .25), a))
		rect(dst, x-w/2, y-hh, w*.3, hh, alpha(mix(col, white, .3), .8*a))
		fillEllipse(dst, x, y-hh, w/2, w/4, alpha(mix(col, white, .2), a))
		fillEllipse(dst, x, y, R*.8, R*.45, c(60, 45, 30, uint8(140*a)))
	case "blink":
		glow(dst, x, y, R*(1+k), col, a)
		for i := 0; i < 8; i++ {
			an := float64(i) * .785
			d := R * (.3 + k)
			disc(dst, x+math.Cos(an)*d, y+math.Sin(an)*d*.62, 3, alpha(white, a))
		}
	default:
		soft(dst, x, y, R*1.3, R*1.3, alpha(col, .8*a))
	}
	return true
}

// drawBossWindup: a boss gathering itself for an attack — a glowing aura and, for charges, a lane
// showing exactly where it will run.
func (g *Game) drawBossWindup(dst *ebiten.Image, e *sim.Enemy, x, y float64) {
	k, wu, aim, charging := e.Windup()
	t := g.W.T
	col := tc(e.Type)
	if charging { // afterimages and speed lines while it charges
		for i := 1; i <= 4; i++ {
			fi := float64(i)
			soft(dst, x-math.Cos(aim)*fi*16, y-math.Sin(aim)*fi*16, e.R*(1.3-fi*.15), e.R*(1.3-fi*.15), alpha(col, .35-fi*.07))
		}
		return
	}
	if wu <= 0 {
		return
	}
	glow(dst, x, y, e.R*2.6, col, .35+.25*math.Sin(t*16))
	for i := 0; i < 6; i++ { // energy drawn inwards
		a := float64(i)*1.047 + t*3
		d := e.R * (2.4 - math.Mod(t*2+float64(i)*.17, 1)*1.6)
		disc(dst, x+math.Cos(a)*d, y+math.Sin(a)*d, 3, alpha(mix(col, white, .4), .9))
	}
	if k == "charge" { // the lane it will run down
		L := 520.0
		ca, sa := math.Cos(aim), math.Sin(aim)
		nx, ny := -sa, ca
		wd := e.R + 10
		poly(dst, [][2]float64{{x + nx*wd, y + ny*wd}, {x + ca*L + nx*wd, y + sa*L + ny*wd}, {x + ca*L - nx*wd, y + sa*L - ny*wd}, {x - nx*wd, y - ny*wd}}, c(255, 60, 50, 85))
		for i := 0; i < 6; i++ { // chevrons sliding along it
			d := math.Mod(t*260+float64(i)*L/6, L)
			cx, cy := x+ca*d, y+sa*d
			poly(dst, [][2]float64{{cx + ca*14, cy + sa*14}, {cx + nx*wd*.7, cy + ny*wd*.7}, {cx - nx*wd*.7, cy - ny*wd*.7}}, c(255, 110, 80, 210))
		}
		line(dst, x+nx*wd, y+ny*wd, x+ca*L+nx*wd, y+sa*L+ny*wd, 2, c(255, 80, 70, 200))
		line(dst, x-nx*wd, y-ny*wd, x+ca*L-nx*wd, y+sa*L-ny*wd, 2, c(255, 80, 70, 200))
	}
}

var _ color.NRGBA
