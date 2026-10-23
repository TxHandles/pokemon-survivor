package client

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Visuals for the charge / counter / bind / ramp / sacrifice / vanish attack types (moves_go2.go).

// ownerPos: where the player who made an effect is now (charges and auras follow them).
func (g *Game) ownerPos(f *sim.FX) (float64, float64) {
	if f.Slot != nil && f.Slot.Owner >= 0 && f.Slot.Owner < len(g.W.PL) {
		p := g.W.PL[f.Slot.Owner].P
		return p.X, p.Y
	}
	return f.X, f.Y
}

func (g *Game) drawGo2FX(dst *ebiten.Image, f *sim.FX, sx, sy func(float64) float64) bool {
	T := g.W.T
	k := 1 - f.Life/math.Max(f.Life0, 1e-6) // 0 → 1
	al := math.Min(1, f.Life/math.Max(f.Life0*.5, 1e-6))
	L := lookOf(f.Type)
	switch f.Kind {
	case "chg": // charging: energy spirals in, the glow swells and a ring tightens
		ox, oy := g.ownerPos(f)
		x, y := sx(ox), sy(oy)
		e := k * k
		glowAdd(dst, x, y, 26+46*e, L.mid, .25+.55*e)
		glowAdd(dst, x, y, 10+18*e, L.hot, .4+.6*e)
		rr := f.R * (1.3 - 1.05*k)
		ringAdd(dst, x, y, rr, L.mid, .4+.5*k)
		ring(dst, x, y, rr, 2.5+3*k, alpha(L.hot, .5+.4*k))
		for i := 0; i < 10; i++ { // motes pulled inward along spirals
			fi := float64(i)
			ph := math.Mod(T*1.6+fi/10, 1)
			a := fi*.628 + ph*4 + T*2
			d := f.R * 1.2 * (1 - ph)
			glowAdd(dst, x+math.Cos(a)*d, y+math.Sin(a)*d*.8, 5+4*ph, L.hot, .9*ph)
		}
		if math.Mod(T*(6+18*k), 1) < .5 { // flicker faster as it nears release
			disc(dst, x, y, 3+4*k, alpha(white, .8))
		}
		return true
	case "chgx": // the release: a white-hot streak to the target and a huge burst
		x, y := sx(f.X), sy(f.Y)
		ax, ay := sx(f.AX), sy(f.AY)
		if k < .35 {
			a := 1 - k/.35
			line(dst, ax, ay, x, y, 18*a+2, alpha(L.mid, a))
			line(dst, ax, ay, x, y, 6*a+1, alpha(white, a))
			for t := 0.0; t <= 1; t += .12 {
				glowAdd(dst, ax+(x-ax)*t, ay+(y-ay)*t, 18*a, L.mid, a*.5)
			}
		}
		glowAdd(dst, x, y, f.R*(1.1+.8*k), L.mid, al*.4)
		glowAdd(dst, x, y, f.R*(.35+.2*k), white, al*.45)
		ringAdd(dst, x, y, f.R*(.4+1.1*k), L.hot, al)
		ringAdd(dst, x, y, f.R*(.2+.8*k), L.mid, al*.7)
		return true
	case "ctr": // counter: a red-rimmed shockwave, bigger and angrier the more was stored
		ox, oy := g.ownerPos(f)
		x, y := sx(ox), sy(oy)
		fill := f.D
		red := hexC("#ff3a2a")
		r := f.R * (.25 + .85*k)
		glowAdd(dst, x, y, r*1.1, red, al*(.3+.5*fill))
		ringAdd(dst, x, y, r, red, al*(.6+.4*fill))
		ringAdd(dst, x, y, r*.85, L.mid, al*.7)
		for i := 0; i < 12; i++ { // jagged spikes
			a := float64(i)/12*2*math.Pi + T
			l := r * (.18 + .2*fill) * (.6 + .4*hsh(float64(i)))
			px, py := x+math.Cos(a)*r, y+math.Sin(a)*r
			line(dst, px, py, px+math.Cos(a)*l, py+math.Sin(a)*l, 3, alpha(red, al))
		}
		return true
	case "ramp": // a stacking strike: hotter and bigger with every stack
		x, y := sx(f.X), sy(f.Y)
		m := math.Min(1, (f.D-1)/1.5) // 0 at first use → 1 at full power
		c0 := lerpC(L.mid, white, m*.6)
		r := f.R * (.5 + .7*k)
		glowAdd(dst, x, y, f.R*(1+.6*m), c0, al*(.5+.4*m))
		ringAdd(dst, x, y, r, c0, al)
		n := 4 + int(8*m)
		for i := 0; i < n; i++ {
			a := float64(i)/float64(n)*2*math.Pi + hsh(float64(i)+f.X)*.5
			l := f.R * (.5 + .9*k) * (1 + .5*m)
			line(dst, x+math.Cos(a)*l*.35, y+math.Sin(a)*l*.35, x+math.Cos(a)*l, y+math.Sin(a)*l, 2+2*m, alpha(c0, al))
		}
		if f.D > 1.05 && k < .8 {
			drawText(dst, "x"+trim1(f.D), x, y-f.R-18-12*k, 14+int(4*m), true, alpha(lerpC(L.hot, hexC("#ffd84a"), m), al), text.AlignCenter, true)
		}
		return true
	case "sac": // wind-up: the air is sucked in, everything darkens to a point
		ox, oy := g.ownerPos(f)
		x, y := sx(ox), sy(oy)
		r := f.R * (1.2 - 1.1*k)
		soft(dst, x, y, r, r, alpha(hexC("#1a0a10"), .35*k))
		ring(dst, x, y, r, 3+4*k, alpha(L.hot, .5+.5*k))
		ringAdd(dst, x, y, r, L.mid, .6)
		for i := 0; i < 14; i++ {
			fi := float64(i)
			a := fi * .449
			d := f.R * (1.3 - 1.2*math.Mod(k*1.6+fi/14, 1))
			line(dst, x+math.Cos(a)*d, y+math.Sin(a)*d, x+math.Cos(a)*(d+16), y+math.Sin(a)*(d+16), 2, alpha(L.hot, .8))
		}
		glowAdd(dst, x, y, 8+20*k, white, .6+.4*k)
		return true
	case "sacx": // the blast itself
		x, y := sx(f.X), sy(f.Y)
		glowAdd(dst, x, y, f.R*(1.2+.5*k), L.mid, al*.42)
		glowAdd(dst, x, y, f.R*(.4+.3*k), white, al*.4)
		for i := 0; i < 3; i++ {
			kk := math.Min(1, k*1.4+float64(i)*.1)
			ringAdd(dst, x, y, f.R*(.3+1.2*kk), lerpC(L.hot, L.cool, float64(i)/2), al)
		}
		return true
	case "vanx": // bursting up from below: a pillar of light and flung earth
		x, y := sx(f.X), sy(f.Y)
		h := 160 * (1 - k)
		for i := 0; i < 8; i++ {
			t := float64(i) / 8
			glowAdd(dst, x, y-h*t, f.R*(.45-.3*t), L.mid, al*.3*(1-t))
		}
		glowAdd(dst, x, y, f.R*.9, L.mid, al*.4)
		ringAdd(dst, x, y, f.R*(.3+.9*k), L.mid, al)
		strokeEllipse(dst, x, y+4, f.R*(.4+.8*k), f.R*(.4+.8*k)*.5, 0, 2*math.Pi, 4, alpha(L.cool, al*.8))
		return true
	}
	return false
}

func trim1(v float64) string {
	n := int(math.Round(v * 10))
	return itoa10(n/10) + "." + itoa10(n%10)
}

func itoa10(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// drawBinds: coils wrapped around every held foe, squeezing tighter as the hold goes on.
func (g *Game) drawBinds(dst *ebiten.Image, sx, sy func(float64) float64) {
	T := g.W.T
	for _, b := range g.W.Binds {
		e := b.E
		x, y := sx(e.X), sy(e.Y)
		L := lookOf(b.Type)
		k := 1 - b.T/math.Max(b.Du, 1e-6)
		R := e.R * (1.6 - .35*k)
		glowAdd(dst, x, y, R*1.3, L.mid, .25)
		for i := 0; i < 3; i++ {
			fi := float64(i)
			yy := y - e.R*.6 + fi*e.R*.6
			ph := T*3 + fi*2
			strokeEllipse(dst, x, yy, R, R*.32, ph, ph+4.4, 6, alpha(L.cool, .9))
			strokeEllipse(dst, x, yy, R, R*.32, ph, ph+4.4, 2.5, alpha(L.hot, .9))
		}
		if math.Mod(T*3.3+e.Seed, 1) < .25 { // a squeeze pulse
			ringAdd(dst, x, y, R*1.2, L.hot, .5)
		}
	}
}

// drawVanished: the player is under the ground / water / shadow — only a ripple and bubbling
// particles show where they are.
func (g *Game) drawVanished(dst *ebiten.Image, P *sim.Player, x, y float64) {
	T := g.W.T
	L := lookOf(P.VanType)
	k := 1 - P.Van/math.Max(P.VanDu, 1e-6)
	soft(dst, x, y+8, 22, 9, alpha(hexC("#000000"), .35))
	for i := 0; i < 3; i++ {
		ph := math.Mod(T*2.2+float64(i)/3, 1)
		strokeEllipse(dst, x, y+8, 10+26*ph, (10+26*ph)*.45, 0, 2*math.Pi, 2.5, alpha(L.mid, .8*(1-ph)))
	}
	glowAdd(dst, x, y+6, 16+10*k, L.mid, .35+.4*k)
	if rnd() < .3 {
		g.fxs.burst(P.VanType, P.X+rr(-10, 10), P.Y+8, 1, 60, -math.Pi/2, 1)
	}
}

// drawChargeAura / drawCounterAura are drawn under the player's sprite.
func (g *Game) drawCounterAura(dst *ebiten.Image, P *sim.Player, x, y float64) {
	fill := math.Min(1, P.Ctr/math.Max(1, P.MHP))
	if fill < .05 {
		return
	}
	T := g.W.T
	red := hexC("#ff3a2a")
	glowAdd(dst, x, y, 22+18*fill, red, .2+.35*fill*(.7+.3*math.Sin(T*10)))
	for i := 0; i < int(2+6*fill); i++ {
		ph := math.Mod(T*1.5+float64(i)*.27, 1)
		a := float64(i) * 2.4
		glowAdd(dst, x+math.Cos(a)*14, y+10-ph*36, 4*(1-ph)+1, red, .8*(1-ph))
	}
}

func hasCounter(w *sim.World, i int) bool {
	sl := w.Slots
	if i != w.CUR && i < len(w.PL) {
		sl = w.PL[i].Slots
	}
	for _, s := range sl {
		if s.Kind == "counter" {
			return true
		}
	}
	return false
}
