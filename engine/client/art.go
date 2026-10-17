package client

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// The web version's hand-drawn scenery, landmarks, hazard pools and move art, exported to sprite
// atlases by tools/export_art.py (data/art/). Live effects (glows, smoke, bubbles…) are drawn here.

type artLoc struct {
	Tree, Rock string
	T, R       []string
}

type artLM struct {
	Kind, Img string
}

type artLMK struct {
	Cs [][3]float64
	Fx []any
}

type artData struct {
	Atlas   map[string]map[string][4]int
	Loc     map[string][]artLoc
	LmLoc   map[string][][]artLM
	Sign    string
	Lmk     map[string]artLMK
	ArtCols map[string][3]string
	Rscm    map[string]float64
	sheets  map[string]*ebiten.Image
	subs    map[string]*ebiten.Image
}

var art *artData

func loadArt(f fs.FS) {
	b, err := fs.ReadFile(f, "art/art.json")
	if err != nil {
		return
	}
	a := &artData{sheets: map[string]*ebiten.Image{}, subs: map[string]*ebiten.Image{}}
	if json.Unmarshal(b, a) != nil {
		return
	}
	for name, file := range map[string]string{"types": "types", "scen": "scenery", "lm": "landmarks", "pools": "pools"} {
		pb, err := fs.ReadFile(f, "art/"+file+".png")
		if err != nil {
			return
		}
		im, _, err := image.Decode(bytes.NewReader(pb))
		if err != nil {
			return
		}
		a.sheets[name] = ebiten.NewImageFromImage(im)
	}
	art = a
}

// artImg returns one picture from an atlas ("types" "fire/p", "scen" "s12", "lm" "l3", "pools" "lava/0").
func artImg(atlas, key string) *ebiten.Image {
	if art == nil {
		return nil
	}
	k := atlas + ":" + key
	if im, ok := art.subs[k]; ok {
		return im
	}
	r, ok := art.Atlas[atlas][key]
	var im *ebiten.Image
	if ok {
		im = art.sheets[atlas].SubImage(image.Rect(r[0], r[1], r[0]+r[2], r[1]+r[3])).(*ebiten.Image)
	}
	art.subs[k] = im
	return im
}

// stamp draws im scaled to size sz (its width), centred on (x, y), rotated by rot, with alpha a.
func stamp(dst, im *ebiten.Image, x, y, sz, rot, a float64) {
	if im == nil || a <= 0 {
		return
	}
	w, h := float64(im.Bounds().Dx()), float64(im.Bounds().Dy())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(sz/w, sz/w)
	if rot != 0 {
		op.GeoM.Rotate(rot)
	}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(float32(math.Min(1, a)))
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(im, op)
}

// drawAt draws im with its top-left at (x, y) scaled by s.
func drawAt(dst, im *ebiten.Image, x, y, s, a float64) {
	if im == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	if a < 1 {
		op.ColorScale.ScaleAlpha(float32(a))
	}
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(im, op)
}

// glow: the web glow() — a soft blob, solid to 55% then fading, radius r*1.15.
var glowImg = func() *ebiten.Image {
	const n = 64
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)+.5-32, float64(y)+.5-32) / 32
			a := 1.0
			if d > .55 {
				a = math.Max(0, 1-(d-.55)/.45)
			}
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}()

func glow(dst *ebiten.Image, x, y, r float64, col color.NRGBA, a float64) {
	if r < .5 {
		return
	}
	R := r * 1.15
	texQuad(dst, glowImg, glowImg.Bounds(), x-R, y-R, 2*R, 2*R, alpha(col, math.Min(1, a*1.25)))
}

// ellipsePath adds an ellipse (optionally only the arc a0..a1) to p.
func ellipsePath(p *vector.Path, x, y, rx, ry, a0, a1 float64) {
	n := int(math.Max(12, math.Min(64, (rx+ry)*.6)))
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(n)
		px, py := float32(x+math.Cos(a)*rx), float32(y+math.Sin(a)*ry)
		if i == 0 {
			p.MoveTo(px, py)
		} else {
			p.LineTo(px, py)
		}
	}
}

func strokeEllipse(dst *ebiten.Image, x, y, rx, ry, a0, a1, w float64, col color.NRGBA) {
	var p vector.Path
	ellipsePath(&p, x, y, rx, ry, a0, a1)
	strokePath(dst, &p, w, col)
}

func fillEllipse(dst *ebiten.Image, x, y, rx, ry float64, col color.NRGBA) {
	var p vector.Path
	ellipsePath(&p, x, y, rx, ry, 0, 2*math.Pi)
	p.Close()
	fillPath(dst, &p, col)
}

// rgbaStr parses "#rrggbb" or "rgba(r,g,b,a)".
func rgbaStr(s string) color.NRGBA {
	if strings.HasPrefix(s, "#") {
		return hexC(s)
	}
	return rgbStr(s)
}

// ---- scenery ----

func (g *Game) locArt() *artLoc {
	if art == nil {
		return nil
	}
	l := art.Loc[g.W.Biome]
	if len(l) == 0 {
		l = art.Loc["normal"]
	}
	li := g.W.LI
	if li < 0 || li >= len(l) {
		li = 0
	}
	return &l[li]
}

// drawObst draws a tree, rock or landmark (web drawObst). Falls back to the simple drawings when
// the art atlases are missing.
func (g *Game) drawObst(dst *ebiten.Image, o sim.Obstacle, x, y float64) {
	w := g.W
	if o.Lm != 0 {
		g.drawLandmark(dst, o, x, y)
		return
	}
	la := g.locArt()
	if la == nil {
		drawObstacle(dst, o, x, y, w.Biome)
		return
	}
	v := int(math.Abs(math.Floor(o.X*7.3+o.Y*3.1))) % 3
	if o.Tree {
		s := o.R / 14.5
		if v < len(la.T) {
			drawAt(dst, artImg("scen", la.T[v]), x-64*s, y-104*s, s, 1)
		}
		return
	}
	k := art.Rscm[la.Rock]
	if k == 0 {
		k = 1
	}
	s := o.R / 34 * k
	if v < len(la.R) {
		drawAt(dst, artImg("scen", la.R[v]), x-64*s, y-92*s, s, 1)
	}
}

func (g *Game) drawLandmark(dst *ebiten.Image, o sim.Obstacle, x, y float64) {
	w := g.W
	if art == nil {
		return
	}
	kind := w.LandmarkKind(o)
	img := art.Sign
	if o.Lm != sim.LmSign {
		l := art.LmLoc[w.Biome]
		if w.LI < 0 || w.LI >= len(l) || o.Lm < 1 || o.Lm > len(l[w.LI]) {
			return
		}
		img = l[w.LI][o.Lm-1].Img
	}
	s := o.Sc
	if s == 0 {
		s = 1
	}
	drawAt(dst, artImg("lm", img), x-80*s, y-150*s, s, 1)
	f := art.Lmk[kind].Fx
	if len(f) < 3 {
		return
	}
	fk, _ := f[0].(string)
	if g.opt("fx") == 0 && fk != "text" {
		return
	}
	fxp, _ := f[1].(float64)
	fyp, _ := f[2].(float64)
	fx, fy := x+(fxp-80)*s, y+(fyp-150)*s
	col := white
	if len(f) > 3 {
		if cs, ok := f[3].(string); ok {
			col = rgbaStr(cs)
		}
	}
	T := w.T
	switch fk {
	case "text":
		nm := w.LocName(w.Biome, w.LI)
		fs := 12 * s
		for fs > 6 && textW(nm, int(fs), true) > 78*s {
			fs -= .5
		}
		drawText(dst, nm, fx, fy-fs*.62, int(math.Round(fs)), true, hexC("#3a2410"), text.AlignCenter, false)
	case "glow":
		glow(dst, fx, fy, 18*s*(1+.15*math.Sin(T*3+o.X)), col, .35)
	case "smoke", "steam":
		ka := .5
		if fk == "steam" {
			ka = .35
		}
		for q := 0; q < 4; q++ {
			ph := fmod1(T*.45 + float64(q)*.25 + o.X*.001)
			disc(dst, fx+math.Sin(ph*6+float64(q))*8*s, fy-ph*60*s, (6+ph*14)*s, alpha(col, (1-ph)*ka))
		}
	case "beam":
		a := T * 1.4
		sector(dst, fx, fy, 180*s, a, .18, alpha(col, .22))
	case "spin":
		for i := 0; i < 3; i++ {
			a := -T*2.2 + float64(i)*2.09
			strokeEllipse(dst, fx, fy, (20+float64(i)*14)*s, (9+float64(i)*6)*s, a, a+1.6, 2.5*s, alpha(col, .7))
		}
	case "spark":
		if int(T*8)%3 == 0 {
			px, py := fx, fy
			for i := 0; i < 4; i++ {
				nx := px + (hash01(int(T*8), i, 1)-.5)*24*s
				ny := py + (hash01(int(T*8), i, 2)-.3)*14*s
				line(dst, px, py, nx, ny, 2*s, col)
				px, py = nx, ny
			}
		}
		glow(dst, fx, fy, 14*s, col, .25)
	case "flicker":
		glow(dst, fx, fy-10*s, 30*s*(1+.2*math.Sin(T*13+o.Y)), col, .3)
	case "float":
		glow(dst, fx, fy+math.Sin(T*1.5)*4*s, 40*s, col, .15)
	case "spray":
		for q := 0; q < 5; q++ {
			ph := fmod1(T*1.2 + float64(q)*.2)
			disc(dst, fx+float64(q-2)*6*s*ph, fy-math.Sin(ph*math.Pi)*22*s+ph*20*s, 2.5*s, alpha(col, 1-ph))
		}
	case "neon":
		if math.Sin(T*7) > -.6 {
			glow(dst, fx, fy, 60*s, col, .18)
		}
	case "pulse":
		ph := math.Mod(T*.8, 1)
		strokeEllipse(dst, fx, fy, (14+ph*30)*s, (5+ph*12)*s, 0, 2*math.Pi, 2*s, alpha(col, 1-ph))
	}
}

func fmod1(x float64) float64 { return x - math.Floor(x) }

// ---- hazard pools (web drawPool) ----

func (g *Game) drawPoolArt(dst *ebiten.Image, p sim.Pool, x, y, T float64) {
	v := int(math.Abs(math.Floor(p.X*.13+p.Y*.29))) % 3
	im := artImg("pools", p.K+"/"+string(rune('0'+v)))
	if im == nil {
		drawPool(dst, p, x, y, T)
		return
	}
	s := p.R * 1.02 / 120
	ph := math.Sin(T*3+p.X)*.5 + .5
	switch p.K {
	case "sand":
		stamp(dst, im, x, y, 256*s, T*.6, 1)
		return
	case "web":
		stamp(dst, im, x, y, 256*s, math.Sin(T*.8+p.X)*.03, .85)
		return
	}
	stamp(dst, im, x, y, 256*s, 0, 1)
	switch p.K {
	case "lava":
		glow(dst, x, y, p.R*.55, hexC("#ffd24a"), .18+.14*ph)
		for q := 0; q < 3; q++ {
			pq := fmod1(T*.7 + float64(q)*.33 + p.X*.01)
			a := float64(q)*2.1 + p.Y
			disc(dst, x+math.Cos(a)*p.R*.4, y+math.Sin(a)*p.R*.35, 2+pq*6, alpha(hexC("#fff1a8"), 1-pq))
		}
	case "deep":
		r := p.R * (.3 + .5*ph)
		strokeEllipse(dst, x, y, r, r*.8, 0, 2*math.Pi, 2, c(220, 240, 255, uint8(255*.45*(1-ph))))
	case "ice":
		gp := math.Mod(math.Mod(T*.25+p.X*.001, 1.6)+1.6, 1.6) - .3
		gx := x - p.R + gp*p.R*2
		// the glint is clipped to the pool in the web version; keep it inside the circle
		for i := 0; i < 8; i++ {
			t := float64(i) / 7
			px, py := gx+9-t*p.R, y-p.R+t*2*p.R
			if (px-x)*(px-x)+(py-y)*(py-y) < sq(p.R*.88) {
				disc(dst, px, py, 6, c(255, 255, 255, 60))
			}
		}
	case "toxic":
		for q := 0; q < 4; q++ {
			pq := fmod1(T*.9 + float64(q)*.25 + p.X*.013)
			a := float64(q)*1.7 + p.X
			al := .9
			if pq >= .8 {
				al = (1 - pq) * 4
			}
			bx, by, br := x+math.Cos(a)*p.R*.5, y+math.Sin(a)*p.R*.45, 2+pq*7
			disc(dst, bx, by, br, alpha(hexC("#b060e0"), al))
			ring(dst, bx, by, br, 1.5, alpha(hexC("#ecc0ff"), al))
		}
	}
}

func sq(x float64) float64 { return x * x }

// ---- weather events (web drawBioHz) ----

func (g *Game) drawBioHz(dst *ebiten.Image, h *sim.Hazard, x, y float64) {
	T := g.W.T
	f := 1 - math.Max(0, h.T)/math.Max(h.T0, .01)
	if !h.Gone {
		if h.Bio == "storm" {
			glow(dst, x, y, h.R*1.3, hexC("#1a1a30"), .35)
			// dashed rotating ring
			for i := 0; i < 18; i++ {
				a0 := float64(i)/18*2*math.Pi + T*30/h.R
				strokeEllipse(dst, x, y, h.R, h.R, a0, a0+2*math.Pi/18*.57, 3, c(255, 224, 61, uint8(255*(.5+.4*math.Sin(T*20)))))
			}
			disc(dst, x, y, h.R*f, c(255, 224, 61, 56))
			if f > .6 {
				px, py := x, y-260
				for i := 0; i < 6; i++ {
					nx := x + (hash01(i, int(T*20), 3)-.5)*30
					line(dst, px, py, nx, py+43, 1.5, alpha(hexC("#fff8c0"), (f-.6)*2))
					px, py = nx, py+43
				}
			}
		} else {
			sc := .35 + .65*f
			fillEllipse(dst, x, y, h.R*sc, h.R*sc*.7, c(0, 0, 0, uint8(255*(.15+.3*f))))
			ring(dst, x, y, h.R, 2, c(190, 160, 110, 204))
			fy := y - (1-f)*(1-f)*340
			s := h.R * .8 / 34
			if l := art.Loc["rock"]; art != nil && len(l) > 0 && len(l[0].R) > 0 {
				drawAt(dst, artImg("scen", l[0].R[0]), x-64*s, fy-92*s, s, 1)
			}
		}
		return
	}
	a := math.Max(0, h.Fl/.25)
	if h.Bio == "storm" {
		for _, st := range []struct {
			w   float64
			col color.NRGBA
		}{{9, hexC("#ffe03d")}, {3, white}} {
			px, py := x, y-420
			for i := 1; i <= 8; i++ {
				nx := x
				if i < 8 {
					nx = x + (hash01(i*7, int(h.X), 5)-.5)*46
				}
				ny := y - 420 + float64(i)*52.5
				line(dst, px, py, nx, ny, st.w, alpha(st.col, a))
				px, py = nx, ny
			}
		}
		glow(dst, x, y, h.R*1.2, hexC("#fff8c0"), a*.7)
	} else {
		glow(dst, x, y, h.R, hexC("#d8c8a0"), .5*a)
		for i := 0; i < 8; i++ {
			an := float64(i)*.785 + h.X
			d := h.R * (1.1 - a*.6)
			disc(dst, x+math.Cos(an)*d, y+math.Sin(an)*d*.7, 3+float64(i%3), alpha(hexC("#8a7a5a"), a))
		}
	}
}
