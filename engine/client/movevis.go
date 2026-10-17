package client

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Per-move battlefield visuals (web genVis + VIS): every move draws its projectiles, orbs, ground
// zones, rings, arcs, beams… with its type's hand-drawn art (data/art/types.png: p = projectile,
// o = orb / spinning piece, b = particle, d = ground decal) in one of three styles picked from the
// move id, and a few fire / electric moves have their own custom drawings.

type mvis struct {
	id, t      string
	c0, c1, c2 color.NRGBA
	h, sty     int
	kind       string
	spin, big  bool
}

var mvisCache = map[string]*mvis{}

func idh(id string) int {
	h := 0
	for _, ch := range []byte(id) {
		h = (h*31 + int(ch)) % 9973
	}
	return h
}

// hsh: the web hsh(n), a 0..1 hash of a number.
func hsh(n float64) float64 {
	x := math.Sin(n*127.1+311.7) * 43758.5453
	return x - math.Floor(x)
}

func (g *Game) visOf(s *sim.Slot) *mvis {
	if s == nil {
		return nil
	}
	return g.visOfID(s.ID)
}

func (g *Game) visOfID(id string) *mvis {
	if id == "" || art == nil || g.opt("fx") == 0 || g.W == nil {
		return nil
	}
	if v, ok := mvisCache[id]; ok {
		return v
	}
	m := g.W.DB.Moves[id]
	if m == nil {
		mvisCache[id] = nil
		return nil
	}
	t := m.Type
	cols, ok := art.ArtCols[t]
	if !ok {
		t = "normal"
		cols = art.ArtCols[t]
	}
	h := idh(id)
	k := m.Shape
	v := &mvis{id: id, t: t, c0: cssC(cols[0]), c1: cssC(cols[1]), c2: cssC(cols[2]), h: h, sty: h % 3, kind: k,
		spin: k == "boom" || (k == "orbit" && h%2 == 1), big: k == "homing" || k == "blast"}
	mvisCache[id] = v
	return v
}

// G: stamp one of the type's art pieces.
func (v *mvis) G(dst *ebiten.Image, piece string, x, y, sz, rot, a float64) {
	stamp(dst, artImg("types", v.t+"/"+piece), x, y, sz, rot, a)
}

// ---- projectiles ----

func (g *Game) visProj(dst *ebiten.Image, v *mvis, p *sim.Proj, x, y float64) {
	T := g.W.T
	switch v.id {
	case "fire_proj":
		fireProj(dst, p, x, y, T)
		return
	case "fire_wisp":
		fireWisp(dst, p, x, y, T)
		return
	case "electric_electroball":
		electroball(dst, p, x, y, T)
		return
	}
	a := math.Atan2(p.VY, p.VX)
	r := p.R
	ca, sa := math.Cos(a), math.Sin(a)
	k := 3.6
	if v.big {
		k = 4
	}
	sz := math.Max(20, r*k)
	if p.Ex > 0 {
		sz *= 1.15
	}
	glow(dst, x, y, sz*.62, v.c1, .18)
	switch v.sty {
	case 0:
		for i := 1; i <= 4; i++ {
			c := v.c2
			if i%2 == 1 {
				c = v.c1
			}
			glow(dst, x-ca*r*float64(i)*1.3, y-sa*r*float64(i)*1.3, r*(1-float64(i)*.17), c, .42-float64(i)*.08)
		}
	case 1:
		for i := 1; i <= 3; i++ {
			fi := float64(i)
			w := math.Sin(T*12+fi*2) * 3
			v.G(dst, "b", x-ca*sz*.32*fi+sa*w, y-sa*sz*.32*fi-ca*w, sz*(.62-fi*.12), T*4+fi, .85-fi*.2)
		}
	default: // fading streak
		L := sz * 1.4
		for i := 0; i < 6; i++ {
			t0, t1 := float64(i)/6, float64(i+1)/6
			c := mix(v.c2, v.c1, t0)
			line(dst, x-ca*L*t0, y-sa*L*t0, x-ca*L*t1, y-sa*L*t1, r*1.3, alpha(c, .55*(1-t0)))
		}
	}
	if v.kind == "boom" {
		v.G(dst, "o", x, y, sz*1.05, T*14, 1)
	} else {
		v.G(dst, "p", x, y, sz*1.5, a, 1)
	}
}

// ---- orbs ----

func (g *Game) visOrb(dst *ebiten.Image, v *mvis, o sim.Orb, x, y, ox, oy float64) {
	T := g.W.T
	if v.id == "fire_orbit" {
		tg := math.Atan2(y-oy, x-ox) + math.Pi/2
		glow(dst, x, y, o.R*2, hexC("#ff7a1f"), .25)
		drop(dst, x, y, o.R*1.05, tg, hexC("#ff4a1a"), hexC("#ffd24a"), o.X*.07, 3.2, T)
		return
	}
	glow(dst, x, y, o.R*1.8, v.c1, .22)
	rot := math.Atan2(y-oy, x-ox) + math.Pi/2
	if v.spin {
		rot = T * 6
	}
	v.G(dst, "o", x, y, math.Max(20, o.R*3.1), rot, 1)
}

// ---- zones ----

func (g *Game) visZone(dst *ebiten.Image, v *mvis, z *sim.Zone, x, y float64) {
	T := g.W.T
	if v.id == "fire_trail" {
		n := int(math.Min(14, math.Max(5, math.Round(z.R/5))))
		al := math.Min(.4, z.L)
		glow(dst, x, y, z.R*.85, hexC("#ff5a1f"), al*.8)
		glow(dst, x, y, z.R*.5, hexC("#ffb733"), al)
		a1 := math.Min(1, z.L*2)
		for i := 0; i < n; i++ {
			a := float64(i) * 2.4
			d := z.R * math.Sqrt((float64(i)+.5)/float64(n)) * .8
			sz := 3.5 + 3*(.5+.5*math.Sin(T*8+float64(i)*1.7))
			dropA(dst, x+math.Cos(a)*d, y+math.Sin(a)*d*.8+4, sz, math.Pi/2, hexC("#ff4a1a"), hexC("#ffd24a"), float64(i)*3, 3.2, T, a1)
		}
		return
	}
	A := math.Min(1, z.L*2)
	glow(dst, x, y, z.R*.9, v.c1, math.Min(.26, z.L))
	glow(dst, x, y, z.R*.45, v.c2, math.Min(.2, z.L))
	sd := int(math.Floor(z.X*.37 + z.Y*.13))
	if z.Pull > 0 {
		for i := 0; i < 3; i++ {
			a := T*5 + float64(i)*2.094
			rr := z.R * (.5 + .14*float64(i))
			strokeEllipse(dst, x, y, rr, rr, a, a+1.6, 3, alpha(v.c2, A))
		}
		for i := 0; i < 6; i++ {
			a := -T*3 + float64(i)*6.283/6
			d := z.R * (.35 + .4*math.Mod(T*.7+float64(i)/6, 1))
			v.G(dst, "b", x+math.Cos(a)*d, y+math.Sin(a)*d, 26, a, A*.9)
		}
		v.G(dst, "d", x, y, z.R*.9, T*2, A*.6)
		return
	}
	v.G(dst, "d", x, y, math.Min(90, z.R*1.5), float64(sd%7)*.9, A*.55)
	n := int(math.Min(6, math.Max(2, math.Round(z.R/14))))
	for i := 0; i < n; i++ {
		a := float64(i)*2.4 + hsh(float64(i+sd))*2
		d := z.R * math.Sqrt((float64(i)+.5)/float64(n)) * .75
		b := math.Sin(T*5+float64(i)*1.9) * 3
		v.G(dst, "b", x+math.Cos(a)*d, y+math.Sin(a)*d*.8+b, 22+8*hsh(float64(i+9)), math.Sin(T*2+float64(i))*.4, A*.9)
	}
}

// ---- effects (ring arc rect beam sky puff bolt tele); false = draw the plain version ----

func (g *Game) visFX(dst *ebiten.Image, v *mvis, f *sim.FX, sx, sy func(float64) float64) bool {
	T := g.W.T
	x, y := sx(f.X), sy(f.Y)
	h := float64(v.h)
	switch v.id {
	case "fire_nova":
		if f.Kind == "ring" {
			fireNova(dst, f, x, y, T)
			return true
		}
	case "fire_erupt":
		if f.Kind == "tele" {
			fireEruptTele(dst, f, x, y)
			return true
		}
		if f.Kind == "ring" {
			fireEruptRing(dst, f, x, y)
			return true
		}
	case "fire_punch":
		if f.Kind == "arc" {
			firePunch(dst, f, x, y, T, g.typeIcon("fighting"))
			return true
		}
	}
	switch f.Kind {
	case "ring":
		al := math.Max(0, math.Min(1, f.Life*2.4))
		if v.kind == "erupt" || v.kind == "blast" {
			glow(dst, x, y, f.R*.9, v.c1, al*.6)
			glow(dst, x, y, f.R*.45, v.c2, al*.6)
			v.G(dst, "d", x, y, math.Min(160, f.R*1.4), h, al*.8)
			n := int(math.Min(10, math.Max(5, math.Round(f.R/14))))
			for i := 0; i < n; i++ {
				a := float64(i)/float64(n)*6.283 + h
				d := f.R * (.55 + .3*hsh(float64(i)+h))
				v.G(dst, "b", x+math.Cos(a)*d, y+math.Sin(a)*d, 26, a, al)
			}
			return true
		}
		lw := 6.0
		if f.Fat {
			lw = 9
		}
		ring(dst, x, y, f.R, lw, alpha(v.c1, al))
		ring(dst, x, y, math.Max(1, f.R-3), 2.5, alpha(v.c2, al))
		n := 0
		if f.R > 24 {
			n = int(math.Min(14, math.Max(6, math.Round(f.R/13))))
		}
		rot := float64(v.sty-1) * f.R * .01
		for i := 0; i < n; i++ {
			a := float64(i)/float64(n)*6.283 + rot
			if v.sty == 2 {
				v.G(dst, "p", x+math.Cos(a)*f.R, y+math.Sin(a)*f.R, 26, a, al)
			} else {
				v.G(dst, "b", x+math.Cos(a)*f.R, y+math.Sin(a)*f.R, 24, a+math.Pi/2, al)
			}
		}
		return true
	case "arc":
		al := math.Max(0, math.Min(1, f.Life*4))
		gradSector(dst, x, y, f.R*.15, f.R, f.A, f.Wd, alpha(v.c1, 0), alpha(v.c1, .8), v.c2, al)
		strokeEllipse(dst, x, y, f.R, f.R, f.A-f.Wd, f.A+f.Wd, 2, alpha(v.c0, al))
		for i := 0; i < 2; i++ {
			rr := f.R * (.7 + .14*float64(i))
			strokeEllipse(dst, x, y, rr, rr, f.A-f.Wd*.8, f.A+f.Wd*.8, 3, alpha(v.c2, al))
		}
		n := int(math.Max(3, math.Min(7, math.Round(f.Wd*5))))
		for i := 0; i <= n; i++ {
			a := f.A - f.Wd + 2*f.Wd*float64(i)/float64(n)
			v.G(dst, "b", x+math.Cos(a)*f.R, y+math.Sin(a)*f.R, 22, a, al)
		}
		if v.sty == 1 {
			v.G(dst, "p", x+math.Cos(f.A)*f.R*.7, y+math.Sin(f.A)*f.R*.7, math.Min(46, 22+f.R*.2), f.A, al)
		} else {
			v.G(dst, "o", x+math.Cos(f.A)*f.R*.7, y+math.Sin(f.A)*f.R*.7, math.Min(46, 22+f.R*.2), 0, al)
		}
		return true
	case "rect":
		if f.Ln > 600 {
			g.visSweep(dst, v, f, x, y)
		} else {
			g.visLash(dst, v, f, x, y)
		}
		return true
	case "beam":
		al := math.Min(1, f.Life*5)
		co, si := math.Cos(f.A), math.Sin(f.A)
		ex, ey := x+co*f.Ln, y+si*f.Ln
		for _, st := range []struct {
			w float64
			c color.NRGBA
		}{{f.Wd*2 + 3, v.c0}, {f.Wd * 2, v.c1}, {f.Wd * .8, v.c2}} {
			line(dst, x, y, ex, ey, st.w, alpha(st.c, al))
			disc(dst, x, y, st.w/2, alpha(st.c, al))
			disc(dst, ex, ey, st.w/2, alpha(st.c, al))
		}
		n := int(math.Max(4, math.Round(f.Ln/70)))
		for i := 0; i < n; i++ {
			d := (float64(i) + math.Mod(T*3, 1)) / float64(n) * f.Ln
			v.G(dst, "b", x+co*d, y+si*d, 16+f.Wd*.7, f.A+T*3, al*.9)
		}
		v.G(dst, "o", x, y, 34, T*4, al)
		return true
	case "sky":
		fall := math.Max(0, f.Life/.22)
		al := math.Min(1, f.Life*6)
		for i := 0; i < 8; i++ {
			t0, t1 := float64(i)/8, float64(i+1)/8
			c := mix(v.c1, v.c2, t1)
			line(dst, x, y-460+460*t0, x, y-460+460*t1, 8, alpha(c, al*t1))
		}
		v.G(dst, "p", x, y-300*fall, 44, math.Pi/2, 1)
		if fall < .35 {
			v.G(dst, "d", x, y, 70, h, 1-fall*2)
		}
		return true
	case "puff":
		k := f.Life / math.Max(f.Life0, 1e-6)
		al := math.Max(0, k)
		glow(dst, x, y, f.R*(1-.4*k)*.85, v.c2, al*.45)
		v.G(dst, "d", x, y, math.Min(110, f.R*1.5), h+T, al*.8)
		for i := 0; i < 4; i++ {
			a := float64(i)*1.57 + h
			d := f.R * (1 - k) * .7
			v.G(dst, "b", x+math.Cos(a)*d, y+math.Sin(a)*d, 22, a, al)
		}
		return true
	case "bolt":
		ax, ay, bx, by := sx(f.AX), sy(f.AY), sx(f.BX), sy(f.BY)
		dx, dy := bx-ax, by-ay
		L := math.Hypot(dx, dy)
		if L == 0 {
			L = 1
		}
		nx, ny := -dy/L, dx/L
		n := int(math.Max(2, math.Round(L/25)))
		jag := 7.0
		if v.t == "electric" {
			jag = 16
		}
		pts := [][2]float64{{ax, ay}}
		for i := 1; i < n; i++ {
			o := (hsh(float64(i)+math.Floor(T*30)+h) - .5) * jag
			pts = append(pts, [2]float64{ax + dx*float64(i)/float64(n) + nx*o, ay + dy*float64(i)/float64(n) + ny*o})
		}
		pts = append(pts, [2]float64{bx, by})
		for _, st := range []struct {
			w float64
			c color.NRGBA
		}{{7, v.c0}, {5, v.c1}, {2, v.c2}} {
			polyline(dst, pts, st.w, st.c)
		}
		v.G(dst, "b", bx, by, 28, T*8, .95)
		return true
	}
	return false
}

func polyline(dst *ebiten.Image, pts [][2]float64, w float64, col color.NRGBA) {
	if len(pts) < 2 {
		return
	}
	var p vector.Path
	p.MoveTo(float32(pts[0][0]), float32(pts[0][1]))
	for _, q := range pts[1:] {
		p.LineTo(float32(q[0]), float32(q[1]))
	}
	strokePath(dst, &p, w, col)
}

// gradSector: a pie slice with a radial gradient (inner radius r0 colour a → 65% b → edge c).
func gradSector(dst *ebiten.Image, x, y, r0, r1, a, hw float64, ca, cb, cc color.NRGBA, al float64) {
	const bands = 7
	for i := 0; i < bands; i++ {
		t0, t1 := float64(i)/bands, float64(i+1)/bands
		tm := (t0 + t1) / 2
		var c color.NRGBA
		if tm < .65 {
			c = lerpC(ca, cb, tm/.65)
		} else {
			c = lerpC(cb, cc, (tm-.65)/.35)
		}
		ra, rb := r0+(r1-r0)*t0, r0+(r1-r0)*t1
		if i == 0 {
			ra = 0
		}
		band(dst, x, y, ra, rb, a, hw, alpha(c, al*float64(c.A)/255))
	}
}

func lerpC(a, b color.NRGBA, k float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*k + .5) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), l(a.A, b.A)}
}

// band: annular wedge between radii r0 and r1.
func band(dst *ebiten.Image, x, y, r0, r1, a, hw float64, col color.NRGBA) {
	n := int(math.Max(4, hw*12))
	pts := make([][2]float64, 0, 2*n+2)
	for i := 0; i <= n; i++ {
		t := a - hw + 2*hw*float64(i)/float64(n)
		pts = append(pts, [2]float64{x + math.Cos(t)*r1, y + math.Sin(t)*r1})
	}
	for i := n; i >= 0; i-- {
		t := a - hw + 2*hw*float64(i)/float64(n)
		pts = append(pts, [2]float64{x + math.Cos(t)*r0, y + math.Sin(t)*r0})
	}
	poly(dst, pts, col)
}

// ---- the custom fire / electric drawings (web VIS) ----

// drop: the web teardrop flame (round head at the front, tail of length len·r behind).
func drop(dst *ebiten.Image, x, y, r, ang float64, c1, c2 color.NRGBA, seed, ln, T float64) {
	dropA(dst, x, y, r, ang, c1, c2, seed, ln, T, 1)
}

func dropA(dst *ebiten.Image, x, y, r, ang float64, c1, c2 color.NRGBA, seed, ln, T, al float64) {
	if ln == 0 {
		ln = 3
	}
	L := r * ln * (.85 + .15*math.Sin(T*30+seed))
	ca, sa := math.Cos(ang), math.Sin(ang)
	tr := func(px, py float64) [2]float64 { return [2]float64{x + px*ca - py*sa, y + px*sa + py*ca} }
	for _, st := range []struct {
		sc float64
		c  color.NRGBA
	}{{1, c1}, {.55, c2}} {
		rr, ll := r*st.sc, L*st.sc
		var pts [][2]float64
		for i := 0; i <= 10; i++ { // front half circle from -90° to 90°
			a := -math.Pi/2 + math.Pi*float64(i)/10
			pts = append(pts, tr(math.Cos(a)*rr, math.Sin(a)*rr))
		}
		q := func(x0, y0, cx, cy, x1, y1 float64) {
			for i := 1; i <= 8; i++ {
				t := float64(i) / 8
				u := 1 - t
				pts = append(pts, tr(u*u*x0+2*u*t*cx+t*t*x1, u*u*y0+2*u*t*cy+t*t*y1))
			}
		}
		q(0, rr, -ll*.45, rr*.7, -ll, 0)
		q(-ll, 0, -ll*.45, -rr*.7, 0, -rr)
		poly(dst, pts, alpha(st.c, al))
	}
}

func fireProj(dst *ebiten.Image, p *sim.Proj, x, y, T float64) {
	a := math.Atan2(p.VY, p.VX)
	r := p.R * 1.1
	glow(dst, x, y, r*2.2, hexC("#ff7a1f"), .25)
	drop(dst, x, y, r, a, hexC("#ff4a1a"), hexC("#ffd24a"), p.X*.1, 3.4, T)
	if p.Ex > 0 {
		disc(dst, x+math.Cos(a)*r*.2, y+math.Sin(a)*r*.2, r*.35, white)
		for i := 0; i < 3; i++ {
			q := hsh(float64(i) + math.Floor(T*12))
			d := r * (2 + q*3)
			glow(dst, x-math.Cos(a)*d+math.Sin(a)*(q-.5)*r*3, y-math.Sin(a)*d-math.Cos(a)*(q-.5)*r*3, 1.8, hexC("#ffd24a"), .9)
		}
	}
}

func fireWisp(dst *ebiten.Image, p *sim.Proj, x, y, T float64) {
	a := math.Atan2(p.VY, p.VX)
	r := p.R * 1.15
	ca, sa := math.Cos(a), math.Sin(a)
	fk := math.Sin(T*26 + p.X*.3)
	glow(dst, x, y, r*2.6, hexC("#ff6a1f"), .22)
	for i := 1; i <= 4; i++ {
		fi := float64(i)
		w := math.Sin(T*14+fi*1.7+p.Y*.05) * r * .55
		c := hexC("#ffcf3a")
		if i%2 == 1 {
			c = hexC("#ff8a1f")
		}
		glow(dst, x-ca*r*fi*1.15-sa*w, y-sa*r*fi*1.15+ca*w, r*(1-fi*.18), c, .5-fi*.09)
	}
	for i := 0; i < 3; i++ {
		fi := float64(i)
		ph := fmod1(T*2.5 + fi/3 + p.X*.01)
		ex := x - ca*r*(1+ph*4) + math.Sin(fi*2.1+T*9)*r*.8
		ey := y - sa*r*(1+ph*4) - ph*r*1.4
		c := hexC("#ff7a1f")
		if i%2 == 1 {
			c = hexC("#ffe08a")
		}
		rect(dst, ex-1, ey-1, 2.2, 2.2, alpha(c, 1-ph))
	}
	drop(dst, x, y, r*(1+.08*fk), a, hexC("#e8380a"), hexC("#ffb733"), p.Y*.1, 2.4, T)
	drop(dst, x+ca*r*.15, y+sa*r*.15, r*.62, a, hexC("#ffd24a"), hexC("#fff6c8"), p.X*.1, 2.1, T)
	if p.Ex > 0 {
		disc(dst, x+ca*r*.25, y+sa*r*.25, r*.28, white)
	}
}

func electroball(dst *ebiten.Image, p *sim.Proj, x, y, T float64) {
	r := p.R * 1.25
	q := math.Floor(T * 24)
	glow(dst, x, y, r*2.8, hexC("#ffe03d"), .22)
	glow(dst, x, y, r*1.7, hexC("#fff27a"), .35)
	for i := 0; i < 5; i++ {
		fi := float64(i)
		a0 := hsh(fi+q*7+p.X) * 6.283
		L := r * (1.5 + hsh(fi*3+q)*1.1)
		c, w := hexC("#ffd21f"), 2.0
		if i%2 == 1 {
			c, w = hexC("#fffbd0"), 1.2
		}
		pts := [][2]float64{{x + math.Cos(a0)*r*.8, y + math.Sin(a0)*r*.8}}
		for j := 1; j <= 3; j++ {
			d := r*.8 + (L-r*.8)*float64(j)/3
			aa := a0 + (hsh(fi+float64(j)*5+q)-.5)*.9
			pts = append(pts, [2]float64{x + math.Cos(aa)*d, y + math.Sin(aa)*d})
		}
		polyline(dst, pts, w, c)
	}
	// the ball (radial gradient approximated with layered discs)
	disc(dst, x, y, r, hexC("#d89a00"))
	disc(dst, x-r*.06, y-r*.06, r*.86, hexC("#ffcc1a"))
	disc(dst, x-r*.16, y-r*.16, r*.55, hexC("#fff27a"))
	disc(dst, x-r*.26, y-r*.26, r*.25, white)
	ring(dst, x, y, r, 1.5, hexC("#8a6a00"))
	strokeEllipse(dst, x, y, r*.62, r*.62, T*9, T*9+2.2, 1.4, c(255, 255, 255, 204))
	strokeEllipse(dst, x, y, r*.62, r*.62, T*9+3.14, T*9+4.6, 1.4, c(255, 255, 255, 204))
	an := math.Atan2(p.VY, p.VX)
	ca, sa := math.Cos(an), math.Sin(an)
	for i := 0; i < 4; i++ {
		fi := float64(i)
		ph := fmod1(T*3 + fi/4 + p.X*.01)
		cl := white
		if i%2 == 1 {
			cl = hexC("#fff27a")
		}
		rect(dst, x-ca*r*(1+ph*3)+math.Sin(fi*2.3+T*11)*r*.7-1, y-sa*r*(1+ph*3)+math.Cos(fi*1.7+T*13)*r*.7-1, 2.2, 2.2, alpha(cl, 1-ph))
	}
}

func fireNova(dst *ebiten.Image, f *sim.FX, x, y, T float64) {
	al := math.Max(0, math.Min(1, f.Life*2.2))
	ring(dst, x, y, f.R, 9, alpha(hexC("#ff5a1f"), al))
	ring(dst, x, y, f.R-2, 4, alpha(hexC("#ffd24a"), al))
	n := int(math.Max(8, math.Round(f.R/7)))
	for i := 0; i < n; i++ {
		fi := float64(i)
		a := fi/float64(n)*6.283 + hsh(fi)*.2
		L := 10 + 12*hsh(fi+math.Floor(T*14))
		px, py := x+math.Cos(a)*f.R, y+math.Sin(a)*f.R
		c := hexC("#ffb733")
		if i%2 == 1 {
			c = hexC("#ff7a1f")
		}
		poly(dst, [][2]float64{{px + math.Cos(a+1.57)*5, py + math.Sin(a+1.57)*5}, {px + math.Cos(a)*L, py + math.Sin(a)*L}, {px + math.Cos(a-1.57)*5, py + math.Sin(a-1.57)*5}}, alpha(c, al))
	}
}

func fireEruptTele(dst *ebiten.Image, f *sim.FX, x, y float64) {
	a := .22 + .18*math.Sin(f.Life*30)
	glow(dst, x, y, f.R*.9, hexC("#d62a0a"), a)
	glow(dst, x, y, f.R*.5, hexC("#ffb733"), a)
	for i := 0; i < 7; i++ {
		fi := float64(i)
		an := fi*.9 + hsh(fi)*.5
		l := f.R * (.5 + .5*hsh(fi+3))
		line(dst, x, y, x+math.Cos(an)*l, y+math.Sin(an)*l, 2, alpha(hexC("#ffb733"), .8))
	}
}

func fireEruptRing(dst *ebiten.Image, f *sim.FX, x, y float64) {
	al := math.Max(0, math.Min(1, f.Life*3))
	r := math.Max(1, f.R)
	disc(dst, x, y, r, alpha(hexC("#a81a0a"), al*.8))
	disc(dst, x, y, r*.85, alpha(hexC("#ff4a1a"), al*.8))
	disc(dst, x, y, r*.55, alpha(hexC("#ffb733"), al*.8))
	disc(dst, x, y, r*.25, alpha(hexC("#fff6c8"), al*.8))
	for i := 0; i < 10; i++ {
		fi := float64(i)
		a := fi*.63 + hsh(fi)*.4
		d0 := f.R * .4
		d1 := f.R * (.9 + .8*hsh(fi+7)) * math.Min(1, f.R/40)
		line(dst, x+math.Cos(a)*d0, y+math.Sin(a)*d0-4, x+math.Cos(a)*d1, y+math.Sin(a)*d1-f.R*.4, 2, alpha(hexC("#ffd24a"), al))
	}
}

func firePunch(dst *ebiten.Image, f *sim.FX, x, y, T float64, ic *ebiten.Image) {
	al := math.Max(0, math.Min(1, f.Life*4))
	gradSector(dst, x, y, f.R*.2, f.R, f.A, f.Wd, alpha(hexC("#ff4a1a"), 0), hexC("#ff7a1f"), hexC("#ffd24a"), al)
	n := int(math.Max(5, math.Round(f.Wd*7)))
	for i := 0; i <= n; i++ {
		fi := float64(i)
		a := f.A - f.Wd + (2*f.Wd)*fi/float64(n)
		L := 8 + 14*hsh(fi+math.Floor(T*16))
		px, py := x+math.Cos(a)*f.R, y+math.Sin(a)*f.R
		c := hexC("#ffd24a")
		if i%2 == 1 {
			c = hexC("#ff5a1f")
		}
		poly(dst, [][2]float64{{px + math.Cos(a+1.57)*5, py + math.Sin(a+1.57)*5}, {px + math.Cos(a)*L, py + math.Sin(a)*L}, {px + math.Cos(a-1.57)*5, py + math.Sin(a-1.57)*5}}, alpha(c, al))
	}
	// the web version draws a fist emoji here; the fighting-type icon stands in
	if ic != nil {
		stamp(dst, ic, x+math.Cos(f.A)*f.R*.7, y+math.Sin(f.A)*f.R*.7, 26, 0, al)
	}
}

// visLash: whip attacks (Fire Lash, Vine Whip…): a tapered whip uncoils along the swing, cracks
// at the tip, and sheds the type's particles along its length.
func (g *Game) visLash(dst *ebiten.Image, v *mvis, f *sim.FX, x, y float64) {
	T := g.W.T
	p := 1 - f.Life/math.Max(f.Life0, 1e-6) // 0 → 1
	ext := math.Min(1, p*3)                 // the whip reaches full length in the first third
	al := math.Min(1, f.Life/math.Max(f.Life0, 1e-6)*2.5)
	ca, sa := math.Cos(f.A), math.Sin(f.A)
	nx, ny := -sa, ca
	side := 1.0
	if v.h%2 == 1 {
		side = -1
	}
	const N = 18
	pts := make([][2]float64, N+1)
	wid := make([]float64, N+1)
	for i := 0; i <= N; i++ {
		t := float64(i) / N
		along := t * f.Ln * ext
		// a travelling S-curve that straightens as the crack lands
		lat := side * (math.Sin(t*math.Pi)*f.Wd*1.8*(1-p) + math.Sin(t*9-p*22)*f.Wd*.35*(1-t))
		pts[i] = [2]float64{x + ca*along + nx*lat, y + sa*along + ny*lat}
		wid[i] = math.Max(1.2, f.Wd*.6*(1-t*.85))
	}
	strip := func(k float64, col color.NRGBA) {
		var l, r [][2]float64
		for i := range pts {
			j := min(i+1, N)
			i0 := max(i-1, 0)
			dx, dy := pts[j][0]-pts[i0][0], pts[j][1]-pts[i0][1]
			d := math.Hypot(dx, dy)
			if d == 0 {
				d = 1
			}
			ox, oy := -dy/d*wid[i]*k, dx/d*wid[i]*k
			l = append(l, [2]float64{pts[i][0] + ox, pts[i][1] + oy})
			r = append([][2]float64{{pts[i][0] - ox, pts[i][1] - oy}}, r...)
		}
		poly(dst, append(l, r...), col)
	}
	glow(dst, pts[N/2][0], pts[N/2][1], f.Ln*.35, v.c1, .12*al)
	strip(1.35, alpha(v.c0, al))
	strip(1, alpha(v.c1, al))
	strip(.45, alpha(v.c2, al))
	// crack at the tip
	tip := pts[N]
	if p > .25 {
		k := math.Min(1, (p-.25)*3)
		glow(dst, tip[0], tip[1], 26*k+8, v.c2, .5*al)
		for i := 0; i < 5; i++ {
			a := f.A + (float64(i)-2)*.55 + math.Sin(T*20+float64(i))*.1
			d := 10 + 16*k
			v.G(dst, "b", tip[0]+math.Cos(a)*d, tip[1]+math.Sin(a)*d, 16+6*k, a, al*(1-k*.4))
		}
	}
	// particles shed along the whip
	for i := 3; i < N; i += 4 {
		q := pts[i]
		v.G(dst, "b", q[0]+math.Sin(T*14+float64(i))*3, q[1]+math.Cos(T*12+float64(i))*3, 14, f.A+T*6, al*.85)
	}
	if v.t == "fire" {
		for i := 2; i < N; i += 3 {
			q := pts[i]
			dropA(dst, q[0], q[1]-2, 3+2*hsh(float64(i)), -math.Pi/2, hexC("#ff4a1a"), hexC("#ffd24a"), float64(i), 3, T, al)
		}
	}
}

// visSweep: the big sweeping walls (Surf, Muddy Water, Sacred Fire…): a rolling crest with the
// type's art riding along it instead of a flat bar.
func (g *Game) visSweep(dst *ebiten.Image, v *mvis, f *sim.FX, x, y float64) {
	T := g.W.T
	al := math.Max(0, math.Min(1, f.Life/math.Max(f.Life0, 1e-6)*1.6))
	ca, sa := math.Cos(f.A), math.Sin(f.A)
	nx, ny := -sa, ca // the wall's facing (direction of travel is ±n)
	sw, sh := float64(g.sw), float64(g.sh)
	// only the on-screen part of the 1500px wall
	t0, t1 := 0.0, f.Ln
	step := 28.0
	var crest [][2]float64
	for t := t0; t <= t1; t += step {
		px, py := x+ca*t, y+sa*t
		if px < -80 || py < -80 || px > sw+80 || py > sh+80 {
			if len(crest) > 1 {
				break
			}
			continue
		}
		wob := math.Sin(t*.025+T*6) * f.Wd * .35
		crest = append(crest, [2]float64{px + nx*wob, py + ny*wob})
	}
	if len(crest) < 2 {
		return
	}
	// body: a soft band behind the crest
	for _, q := range crest {
		glow(dst, q[0], q[1], f.Wd*1.1, v.c1, .16*al)
	}
	polyline(dst, crest, f.Wd*.9, alpha(v.c1, .55*al))
	polyline(dst, crest, math.Max(3, f.Wd*.35), alpha(v.c2, .85*al))
	for i, q := range crest {
		if i%3 == 0 {
			v.G(dst, "b", q[0], q[1], 18+6*hsh(float64(i)), f.A+T*3+float64(i), al)
		}
		if i%5 == 2 {
			v.G(dst, "d", q[0], q[1], 34, T*2+float64(i), al*.6)
		}
	}
}
