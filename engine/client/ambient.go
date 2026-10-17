package client

import (
	"image"
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
)

// Ambient life (web drawAmb): a light layer of drifting particles in world space — embers,
// fireflies, snow, fog, cloud shadows… — and the per-location tint / cave darkness (drawLocFX).

type ambK struct {
	n, r, vx, vy, a, l          float64
	c                           string
	fl, tw, wob, pet, bub, soft bool
	grav                        bool
}

type ambP struct{ x, y, vx, vy, t, l, p, s float64 }

type ambState struct {
	key  string
	ps   []ambP
	last float64
	tex  map[string]*ebiten.Image
}

func ambKind(name string) *ambK {
	if flDB == nil {
		return nil
	}
	m, ok := flDB.Amb[name]
	if !ok {
		return nil
	}
	f := func(k string) float64 { v, _ := m[k].(float64); return v }
	b := func(k string) bool { return f(k) != 0 }
	c, _ := m["c"].(string)
	return &ambK{n: f("n"), r: f("r"), vx: f("vx"), vy: f("vy"), a: f("a"), l: f("l"), c: c,
		fl: b("fl"), tw: b("tw"), wob: b("wob"), pet: b("pet"), bub: b("bub"), soft: b("soft"), grav: b("grav")}
}

// ambTex: the web ambSpr — radial gradient of colour c (soft = wide fog/cloud blobs).
func (a *ambState) ambTex(K *ambK) *ebiten.Image {
	k := K.c
	if K.soft {
		k += "s"
	}
	if t, ok := a.tex[k]; ok {
		return t
	}
	const n = 64
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	x := newCnv(img)
	mid := .35
	ma := .9
	if K.soft {
		mid, ma = .15, .6
	}
	x.st.fill = x.radial(n/2, n/2, 0, n/2, gstop{0, cssC(K.c)}, gstop{mid, fa(K.c, ma)}, gstop{1, fa(K.c, 0)})
	x.fillRect(0, 0, n, n)
	t := ebiten.NewImageFromImage(img)
	if a.tex == nil {
		a.tex = map[string]*ebiten.Image{}
	}
	a.tex[k] = t
	return t
}

func (g *Game) drawAmbient(dst *ebiten.Image, camX, camY float64) {
	w := g.W
	d := floorDefFor(w.Biome, w.LI)
	if d == nil {
		return
	}
	K := ambKind(d.Amb)
	a := &g.amb
	key := w.Biome + string(rune('0'+w.LI))
	if a.key != key {
		a.key, a.ps = key, nil
	}
	if K == nil {
		return
	}
	dt := math.Max(0, math.Min(.05, w.T-a.last))
	a.last = w.T
	sw, sh := float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())
	cx, cy := camX+sw/2, camY+sh/2
	hw, hh := sw/2+80, sh/2+80
	out := a.ps[:0]
	for _, p := range a.ps {
		p.t += dt
		if p.t > p.l || math.Abs(p.x-cx) > hw+K.r || math.Abs(p.y-cy) > hh+K.r {
			continue
		}
		p.x += p.vx * dt
		p.y += p.vy * dt
		if K.grav {
			p.vy += 60 * dt
		}
		out = append(out, p)
	}
	a.ps = out
	for float64(len(a.ps)) < K.n {
		big := K.r > 50
		kx := 1.0
		if big {
			kx = 1.2
		}
		vx := K.vx * (.5 + rand.Float64())
		if K.grav {
			vx *= (rand.Float64() - .5) * 4
		}
		a.ps = append(a.ps, ambP{x: cx + (rand.Float64()*2-1)*hw*kx, y: cy + (rand.Float64()*2-1)*hh,
			vx: vx, vy: K.vy * (.6 + rand.Float64()*.8), l: K.l * (.6 + rand.Float64()*.8), p: rand.Float64() * 6.283, s: .6 + rand.Float64()*.7})
	}
	tex := a.ambTex(K)
	col := cssC(K.c)
	now := w.T
	for _, p := range a.ps {
		u := p.t / p.l
		fade := math.Min(1, math.Min(u*5, (1-u)*4))
		al := K.a * fade
		if K.tw {
			al *= .5 + .5*math.Sin(p.p+p.t*7)
		}
		if K.fl {
			al *= .75 + .25*math.Sin(p.p+p.t*19)
		}
		if al <= .01 {
			continue
		}
		x, y := p.x-camX, p.y-camY
		if K.wob {
			x += math.Sin(p.p+now*1.7) * 6
			y += math.Cos(p.p*1.3+now*1.3) * 3
		}
		r := K.r * p.s
		switch {
		case K.bub:
			ring(dst, x, y, 2+u*3, 1, alpha(col, al))
		case K.pet:
			fillRotEllipse(dst, x, y, r*.7, r*.35, p.p+now*2, alpha(col, al))
		case K.soft:
			texQuad(dst, tex, tex.Bounds(), x-r*1.6, y-r, r*3.2, r*2, color.NRGBA{255, 255, 255, uint8(al * 255)})
		default:
			texQuad(dst, tex, tex.Bounds(), x-r, y-r, r*2, r*2, color.NRGBA{255, 255, 255, uint8(al * 255)})
		}
	}
}

func fillRotEllipse(dst *ebiten.Image, x, y, rx, ry, rot float64, col color.NRGBA) {
	pts := make([][2]float64, 16)
	cr, sr := math.Cos(rot), math.Sin(rot)
	for i := range pts {
		a := float64(i) / 16 * 2 * math.Pi
		ex, ey := math.Cos(a)*rx, math.Sin(a)*ry
		pts[i] = [2]float64{x + ex*cr - ey*sr, y + ex*sr + ey*cr}
	}
	poly(dst, pts, col)
}

// drawLocFX: the location's tint and cave darkness over the world (web drawLocFX).
func (g *Game) drawLocFX(dst *ebiten.Image) {
	if flDB == nil {
		return
	}
	l := flDB.Lfx[g.W.Biome]
	if g.W.LI < 0 || g.W.LI >= len(l) || l[g.W.LI] == nil {
		return
	}
	f := l[g.W.LI]
	sw, sh := float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())
	if t, ok := f["tint"].(string); ok {
		rect(dst, 0, 0, sw, sh, cssC(t))
	}
	if v, ok := f["vig"].(float64); ok && v > 0 {
		g.vignette(dst, v)
	}
}

// vignette: radial darkening from 28% to 102% of the half-diagonal (cached per strength & size).
func (g *Game) vignette(dst *ebiten.Image, v float64) {
	sw, sh := dst.Bounds().Dx(), dst.Bounds().Dy()
	if g.vigImg == nil || g.vigV != v || g.vigImg.Bounds().Dx() != sw/4 || g.vigImg.Bounds().Dy() != sh/4 {
		w, h := sw/4, sh/4
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		r := math.Hypot(float64(w), float64(h)) / 2
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				d := math.Hypot(float64(x)+.5-float64(w)/2, float64(y)+.5-float64(h)/2)
				t := math.Max(0, math.Min(1, (d-r*.28)/(r*1.02-r*.28)))
				img.Pix[img.PixOffset(x, y)+3] = uint8(t * v * 255)
			}
		}
		g.vigImg, g.vigV = ebiten.NewImageFromImage(img), v
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(g.vigImg.Bounds().Dx()), float64(sh)/float64(g.vigImg.Bounds().Dy()))
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(g.vigImg, op)
}
