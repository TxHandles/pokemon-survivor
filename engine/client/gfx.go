// Package client renders the simulation with Ebitengine and turns keyboard / gamepad / mouse input
// into sim.Input. Everything is drawn from the sprite sheets in data/ plus procedural vector shapes.
package client

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type floorEl struct {
	Kind  string
	N     int
	Cols  []color.NRGBA
	Scale float64
}

type floorDef struct {
	A, B, C color.NRGBA
	Els     []floorEl
}

func c(r, g, b, a uint8) color.NRGBA { return color.NRGBA{r, g, b, a} }

func hexC(s string) color.NRGBA {
	h := func(i int) uint8 {
		v := 0
		for _, ch := range s[i : i+2] {
			v *= 16
			switch {
			case ch >= '0' && ch <= '9':
				v += int(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v += int(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				v += int(ch-'A') + 10
			}
		}
		return uint8(v)
	}
	return color.NRGBA{h(1), h(3), h(5), 255}
}

// TypeCol: type colours (web COL).
var TypeCol = map[string]color.NRGBA{}

func init() {
	for k, v := range map[string]string{"fire": "#ff6a3d", "water": "#3d9bff", "grass": "#4fd16a", "electric": "#ffe03d", "ground": "#b88a55",
		"psychic": "#ff5fa2", "rock": "#9a8f7a", "bug": "#9bc931", "dark": "#7a6a8f", "dragon": "#6a4cff", "steel": "#9fb4c7", "fairy": "#ffb3e6",
		"flying": "#a9d6ff", "fighting": "#d6453d", "poison": "#b04fd1", "ghost": "#9c8cff", "ice": "#9ff0ff", "normal": "#d9d2c3"} {
		TypeCol[k] = hexC(v)
	}
}

func tc(t string) color.NRGBA {
	if c, ok := TypeCol[t]; ok {
		return c
	}
	return color.NRGBA{255, 255, 255, 255}
}

func alpha(c color.NRGBA, a float64) color.NRGBA {
	a = math.Max(0, math.Min(1, a))
	c.A = uint8(float64(c.A) * a)
	return c
}

func mix(a, b color.NRGBA, k float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*k) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), l(a.A, b.A)}
}

var (
	whiteImg = func() *ebiten.Image { i := ebiten.NewImage(3, 3); i.Fill(color.White); return i }()
	whiteSub = whiteImg.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
	softImg  = makeSoft(64)
)

// makeSoft: a white radial-gradient disc used for shadows, glows and blotches.
func makeSoft(n int) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := (float64(x)+.5)/float64(n)*2-1, (float64(y)+.5)/float64(n)*2-1
			d := math.Sqrt(dx*dx + dy*dy)
			a := math.Max(0, 1-d)
			a = a * a * (3 - 2*a)
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}

// soft draws the soft disc as an ellipse with radii rx, ry.
func soft(dst *ebiten.Image, x, y, rx, ry float64, col color.NRGBA) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-32, -32)
	op.GeoM.Scale(rx/32, ry/32)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(col)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(softImg, op)
}

// Discs and rounded-rect corners are drawn from pre-rendered smooth disc textures rather than
// anti-aliased vector shapes: every AA vector draw makes Ebitengine resolve a full-screen 2×
// offscreen buffer, so a screen with hundreds of them (menus, chips, HUD) gets slow; texture draws
// batch with everything else.
var discTex = []*ebiten.Image{makeDisc(32), makeDisc(128), makeDisc(512)}

func makeDisc(n int) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	c := float64(n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			d := math.Hypot(float64(x)+.5-c, float64(y)+.5-c)
			a := math.Max(0, math.Min(1, c-d+.5))
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}

func discFor(r float64) *ebiten.Image {
	for _, t := range discTex {
		if float64(t.Bounds().Dx()) >= 2*r*1.2 {
			return t
		}
	}
	return discTex[len(discTex)-1]
}

// texQuad draws part sr of texture t into the rectangle (x, y, w, h).
func texQuad(dst, t *ebiten.Image, sr image.Rectangle, x, y, w, h float64, col color.NRGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w/float64(sr.Dx()), h/float64(sr.Dy()))
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(col)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(t.SubImage(sr).(*ebiten.Image), op)
}

func disc(dst *ebiten.Image, x, y, r float64, col color.NRGBA) {
	if r <= 0 {
		return
	}
	t := discFor(r)
	texQuad(dst, t, t.Bounds(), x-r, y-r, 2*r, 2*r, col)
}

func ring(dst *ebiten.Image, x, y, r, w float64, col color.NRGBA) {
	vector.StrokeCircle(dst, float32(x), float32(y), float32(r), float32(w), col, true)
}

func line(dst *ebiten.Image, x0, y0, x1, y1, w float64, col color.NRGBA) {
	vector.StrokeLine(dst, float32(x0), float32(y0), float32(x1), float32(y1), float32(w), col, true)
}

func rect(dst *ebiten.Image, x, y, w, h float64, col color.NRGBA) {
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), col, false)
}

// fastVec: drawing the game world, where effects come by the hundred — vector shapes skip
// anti-aliasing (each AA draw renders through its own 2× offscreen buffer) and strokes batch.
var fastVec bool

// reusable vertex / index buffers (vector drawing allocates nothing per shape)
var vBuf []ebiten.Vertex
var iBuf []uint16

func fillPath(dst *ebiten.Image, p *vector.Path, col color.NRGBA) {
	vBuf, iBuf = p.AppendVerticesAndIndicesForFilling(vBuf[:0], iBuf[:0])
	paint(dst, vBuf, iBuf, col, false)
}

func strokePath(dst *ebiten.Image, p *vector.Path, w float64, col color.NRGBA) {
	vBuf, iBuf = p.AppendVerticesAndIndicesForStroke(vBuf[:0], iBuf[:0], &vector.StrokeOptions{Width: float32(w), LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound})
	paint(dst, vBuf, iBuf, col, true)
}

func paint(dst *ebiten.Image, vs []ebiten.Vertex, is []uint16, col color.NRGBA, stroke bool) {
	r, g, b, a := col.RGBA()
	for i := range vs {
		vs[i].SrcX, vs[i].SrcY = 1, 1
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = float32(r)/0xffff, float32(g)/0xffff, float32(b)/0xffff, float32(a)/0xffff
	}
	op := &ebiten.DrawTrianglesOptions{AntiAlias: !fastVec, FillRule: ebiten.FillRuleNonZero}
	if fastVec && stroke { // stroke triangles need no stencil: they batch with sprites and discs
		op.FillRule = ebiten.FillRuleFillAll
	}
	dst.DrawTriangles(vs, is, whiteSub, op)
}

// sector: filled pie slice centred on angle a with half-angle hw.
func sector(dst *ebiten.Image, x, y, r, a, hw float64, col color.NRGBA) {
	p := &vector.Path{}
	p.MoveTo(float32(x), float32(y))
	p.Arc(float32(x), float32(y), float32(r), float32(a-hw), float32(a+hw), vector.Clockwise)
	p.Close()
	fillPath(dst, p, col)
}

// orect: rectangle from (x,y) along angle a, length L, half-width hw.
func orect(dst *ebiten.Image, x, y, a, L, hw float64, col color.NRGBA) {
	co, si := math.Cos(a), math.Sin(a)
	px, py := -si*hw, co*hw
	p := &vector.Path{}
	p.MoveTo(float32(x+px), float32(y+py))
	p.LineTo(float32(x+co*L+px), float32(y+si*L+py))
	p.LineTo(float32(x+co*L-px), float32(y+si*L-py))
	p.LineTo(float32(x-px), float32(y-py))
	p.Close()
	fillPath(dst, p, col)
}

func poly(dst *ebiten.Image, pts [][2]float64, col color.NRGBA) {
	if len(pts) < 3 {
		return
	}
	p := &vector.Path{}
	p.MoveTo(float32(pts[0][0]), float32(pts[0][1]))
	for _, q := range pts[1:] {
		p.LineTo(float32(q[0]), float32(q[1]))
	}
	p.Close()
	fillPath(dst, p, col)
}

// hash01: deterministic pseudo-random 0..1 from integers.
func hash01(a, b, c int) float64 {
	h := uint32(a)*374761393 + uint32(b)*668265263 + uint32(c)*2246822519
	h = (h ^ h>>13) * 1274126177
	h ^= h >> 16
	return float64(h) / 4294967296
}

type prng struct{ s uint32 }

func (r *prng) f() float64 {
	r.s += 0x6D2B79F5
	t := r.s
	t = (t ^ t>>15) * (1 | t)
	t = (t + (t^t>>7)*(61|t)) ^ t
	return float64(t^t>>14) / 4294967296
}
func (r *prng) rg(a, b float64) float64 { return a + r.f()*(b-a) }

func rectI(r [4]float64) image.Rectangle {
	return image.Rect(int(r[0]), int(r[1]), int(r[0]+r[2]), int(r[1]+r[3]))
}
