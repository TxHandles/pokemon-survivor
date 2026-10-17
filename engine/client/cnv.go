package client

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"
)

// cnv is a small CPU 2D canvas with the parts of the HTML canvas API the web floor painters use
// (paths with lines, quadratic curves, arcs and ellipses; fill, round-cornered strokes, solid and
// gradient paints, a transform stack and global alpha). It draws into an *image.RGBA, so ground
// tiles can be painted exactly like the web version paints them and then uploaded once.

type cpt struct{ x, y float64 }

type cmat struct{ a, b, c, d, e, f float64 } // x' = a*x + c*y + e, y' = b*x + d*y + f

func (m cmat) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

func (m cmat) mul(n cmat) cmat { // m then n applied first: m·n
	return cmat{
		m.a*n.a + m.c*n.b, m.b*n.a + m.d*n.b,
		m.a*n.c + m.c*n.d, m.b*n.c + m.d*n.d,
		m.a*n.e + m.c*n.f + m.e, m.b*n.e + m.d*n.f + m.f,
	}
}

// cpaint returns a colour for a device pixel.
type cpaint func(x, y float64) color.NRGBA

func solid(c color.NRGBA) cpaint { return func(float64, float64) color.NRGBA { return c } }

type cstate struct {
	m      cmat
	fill   cpaint
	stroke cpaint
	lw     float64
	alpha  float64
	butt   bool
}

type cnv struct {
	img   *image.RGBA
	st    cstate
	stack []cstate
	subs  [][]cpt // current path in device space
	cur   []cpt
	open  bool // cur has a current point
	ras   vector.Rasterizer
	mask  *image.Alpha
}

func newCnv(img *image.RGBA) *cnv {
	return &cnv{img: img, st: cstate{m: cmat{1, 0, 0, 1, 0, 0}, fill: solid(color.NRGBA{0, 0, 0, 255}), stroke: solid(color.NRGBA{0, 0, 0, 255}), lw: 1, alpha: 1}}
}

func (x *cnv) save() { x.stack = append(x.stack, x.st) }
func (x *cnv) restore() {
	if n := len(x.stack); n > 0 {
		x.st = x.stack[n-1]
		x.stack = x.stack[:n-1]
	}
}
func (x *cnv) translate(tx, ty float64) { x.st.m = x.st.m.mul(cmat{1, 0, 0, 1, tx, ty}) }
func (x *cnv) rotate(a float64) {
	s, c := math.Sin(a), math.Cos(a)
	x.st.m = x.st.m.mul(cmat{c, s, -s, c, 0, 0})
}
func (x *cnv) setFill(c color.NRGBA)   { x.st.fill = solid(c) }
func (x *cnv) setStroke(c color.NRGBA) { x.st.stroke = solid(c) }
func (x *cnv) setFillS(s string)       { x.st.fill = solid(cssC(s)) }
func (x *cnv) setStrokeS(s string)     { x.st.stroke = solid(cssC(s)) }
func (x *cnv) lineWidth(w float64)     { x.st.lw = w }
func (x *cnv) globalAlpha(a float64)   { x.st.alpha = a }
func (x *cnv) lineCapButt(b bool)      { x.st.butt = b }

// scale of the current transform (for line widths)
func (x *cnv) mscale() float64 { return math.Sqrt(math.Abs(x.st.m.a*x.st.m.d - x.st.m.b*x.st.m.c)) }

func (x *cnv) beginPath() { x.subs, x.cur, x.open = nil, nil, false }

func (x *cnv) flush() {
	if len(x.cur) > 0 {
		x.subs = append(x.subs, x.cur)
	}
	x.cur = nil
}

func (x *cnv) moveTo(px, py float64) {
	x.flush()
	dx, dy := x.st.m.apply(px, py)
	x.cur = []cpt{{dx, dy}}
	x.open = true
}

func (x *cnv) lineTo(px, py float64) {
	dx, dy := x.st.m.apply(px, py)
	if !x.open {
		x.cur = []cpt{{dx, dy}}
		x.open = true
		return
	}
	x.cur = append(x.cur, cpt{dx, dy})
}

func (x *cnv) lastUser() (float64, float64, bool) {
	if !x.open || len(x.cur) == 0 {
		return 0, 0, false
	}
	p := x.cur[len(x.cur)-1]
	// invert the transform
	m := x.st.m
	det := m.a*m.d - m.b*m.c
	if det == 0 {
		return 0, 0, false
	}
	X, Y := p.x-m.e, p.y-m.f
	return (m.d*X - m.c*Y) / det, (-m.b*X + m.a*Y) / det, true
}

func (x *cnv) quadTo(cx, cy, px, py float64) {
	x0, y0, ok := x.lastUser()
	if !ok {
		x.moveTo(cx, cy)
		x0, y0 = cx, cy
	}
	n := 10
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		x.lineTo(u*u*x0+2*u*t*cx+t*t*px, u*u*y0+2*u*t*cy+t*t*py)
	}
}

func (x *cnv) bezierTo(c1x, c1y, c2x, c2y, px, py float64) {
	x0, y0, ok := x.lastUser()
	if !ok {
		x.moveTo(c1x, c1y)
		x0, y0 = c1x, c1y
	}
	n := 14
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		x.lineTo(u*u*u*x0+3*u*u*t*c1x+3*u*t*t*c2x+t*t*t*px, u*u*u*y0+3*u*u*t*c1y+3*u*t*t*c2y+t*t*t*py)
	}
}

// sweep normalises an arc's angles like the canvas spec.
func sweep(a0, a1 float64, ccw bool) float64 {
	const tau = 2 * math.Pi
	if !ccw {
		if a1-a0 >= tau {
			return tau
		}
		d := math.Mod(a1-a0, tau)
		if d < 0 {
			d += tau
		}
		return d
	}
	if a0-a1 >= tau {
		return -tau
	}
	d := math.Mod(a0-a1, tau)
	if d < 0 {
		d += tau
	}
	return -d
}

func (x *cnv) ellipse(cx, cy, rx, ry, rot, a0, a1 float64, ccw bool) {
	sw := sweep(a0, a1, ccw)
	ds := x.mscale() * math.Max(rx, ry)
	n := int(math.Max(6, math.Min(72, math.Abs(sw)*math.Max(2, ds)/2.2)))
	cr, sr := math.Cos(rot), math.Sin(rot)
	for i := 0; i <= n; i++ {
		a := a0 + sw*float64(i)/float64(n)
		ex, ey := math.Cos(a)*rx, math.Sin(a)*ry
		px, py := cx+ex*cr-ey*sr, cy+ex*sr+ey*cr
		if i == 0 && !x.open {
			x.moveTo(px, py)
		} else {
			x.lineTo(px, py)
		}
	}
}

func (x *cnv) arc(cx, cy, r, a0, a1 float64) { x.ellipse(cx, cy, r, r, 0, a0, a1, false) }

func (x *cnv) closePath() {
	if len(x.cur) > 1 {
		x.cur = append(x.cur, x.cur[0])
	}
	if len(x.cur) > 0 {
		first := x.cur[0]
		x.flush()
		x.cur = []cpt{first}
		x.open = true
		return
	}
	x.flush()
}

func (x *cnv) rectPath(rx, ry, w, h float64) {
	x.moveTo(rx, ry)
	x.lineTo(rx+w, ry)
	x.lineTo(rx+w, ry+h)
	x.lineTo(rx, ry+h)
	x.closePath()
}

func (x *cnv) fillRect(rx, ry, w, h float64) {
	sv := x.subs
	sc, so := x.cur, x.open
	x.beginPath()
	x.rectPath(rx, ry, w, h)
	x.fill()
	x.subs, x.cur, x.open = sv, sc, so
}

func (x *cnv) strokeRect(rx, ry, w, h float64) {
	sv := x.subs
	sc, so := x.cur, x.open
	x.beginPath()
	x.rectPath(rx, ry, w, h)
	x.stroke()
	x.subs, x.cur, x.open = sv, sc, so
}

func (x *cnv) allSubs() [][]cpt {
	out := append([][]cpt(nil), x.subs...)
	if len(x.cur) > 0 {
		out = append(out, x.cur)
	}
	return out
}

// fill fills the current path (each subpath closed).
func (x *cnv) fill() {
	subs := x.allSubs()
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, s := range subs {
		for _, p := range s {
			x0, y0, x1, y1 = math.Min(x0, p.x), math.Min(y0, p.y), math.Max(x1, p.x), math.Max(y1, p.y)
		}
	}
	x.raster(x0, y0, x1, y1, func(add func([]cpt)) {
		for _, s := range subs {
			if len(s) >= 3 {
				add(s)
			}
		}
	}, x.st.fill)
}

// stroke strokes the current path with round joins (and round caps unless butt).
func (x *cnv) stroke() {
	subs := x.allSubs()
	hw := x.st.lw * x.mscale() / 2
	if hw <= 0 {
		return
	}
	hw = math.Max(hw, .35)
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, s := range subs {
		for _, p := range s {
			x0, y0, x1, y1 = math.Min(x0, p.x-hw), math.Min(y0, p.y-hw), math.Max(x1, p.x+hw), math.Max(y1, p.y+hw)
		}
	}
	butt := x.st.butt
	x.raster(x0, y0, x1, y1, func(add func([]cpt)) {
		for _, s := range subs {
			for i := 1; i < len(s); i++ {
				a, b := s[i-1], s[i]
				dx, dy := b.x-a.x, b.y-a.y
				l := math.Hypot(dx, dy)
				if l < 1e-9 {
					continue
				}
				nx, ny := -dy/l*hw, dx/l*hw
				// consistent winding for every piece so overlaps saturate instead of cancelling
				add([]cpt{{a.x + nx, a.y + ny}, {b.x + nx, b.y + ny}, {b.x - nx, b.y - ny}, {a.x - nx, a.y - ny}})
			}
			for i, p := range s {
				if butt && (i == 0 || i == len(s)-1) {
					continue
				}
				if hw < .8 {
					continue
				}
				n := int(math.Max(8, math.Min(24, hw*3)))
				c := make([]cpt, n)
				for k := 0; k < n; k++ {
					a := float64(k) / float64(n) * 2 * math.Pi
					c[k] = cpt{p.x + math.Cos(a)*hw, p.y + math.Sin(a)*hw}
				}
				add(c)
			}
		}
	}, x.st.stroke)
}

// raster rasterises polygons in the device-space box and composites cpaint through the coverage.
func (x *cnv) raster(x0, y0, x1, y1 float64, build func(add func([]cpt)), p cpaint) {
	b := x.img.Bounds()
	ix0, iy0 := int(math.Floor(math.Max(x0, float64(b.Min.X)))), int(math.Floor(math.Max(y0, float64(b.Min.Y))))
	ix1, iy1 := int(math.Ceil(math.Min(x1, float64(b.Max.X)))), int(math.Ceil(math.Min(y1, float64(b.Max.Y))))
	if ix1 <= ix0 || iy1 <= iy0 {
		return
	}
	w, h := ix1-ix0, iy1-iy0
	x.ras.Reset(w, h)
	ox, oy := float32(ix0), float32(iy0)
	pos := false // all pieces are added with positive area so overlaps saturate
	build(func(pts []cpt) {
		area := 0.0
		for i := range pts {
			j := (i + 1) % len(pts)
			area += pts[i].x*pts[j].y - pts[j].x*pts[i].y
		}
		_ = pos
		if area < 0 { // reverse to keep one winding
			for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
				pts[i], pts[j] = pts[j], pts[i]
			}
		}
		x.ras.MoveTo(float32(pts[0].x)-ox, float32(pts[0].y)-oy)
		for _, q := range pts[1:] {
			x.ras.LineTo(float32(q.x)-ox, float32(q.y)-oy)
		}
		x.ras.ClosePath()
	})
	if x.mask == nil || x.mask.Bounds().Dx() < w || x.mask.Bounds().Dy() < h {
		x.mask = image.NewAlpha(image.Rect(0, 0, max(w, 320), max(h, 320)))
	}
	m := x.mask
	for j := 0; j < h; j++ {
		row := m.Pix[j*m.Stride : j*m.Stride+w]
		for i := range row {
			row[i] = 0
		}
	}
	x.ras.Draw(m, image.Rect(0, 0, w, h), image.Opaque, image.Point{})
	ga := x.st.alpha
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			cov := m.Pix[j*m.Stride+i]
			if cov == 0 {
				continue
			}
			px, py := ix0+i, iy0+j
			c := p(float64(px)+.5, float64(py)+.5)
			a := float64(cov) / 255 * float64(c.A) / 255 * ga
			if a <= 0 {
				continue
			}
			o := x.img.PixOffset(px, py)
			d := x.img.Pix[o : o+4 : o+4]
			// src-over onto premultiplied RGBA
			k := 1 - a
			d[0] = uint8(float64(c.R)*a + float64(d[0])*k + .5)
			d[1] = uint8(float64(c.G)*a + float64(d[1])*k + .5)
			d[2] = uint8(float64(c.B)*a + float64(d[2])*k + .5)
			d[3] = uint8(255*a + float64(d[3])*k + .5)
		}
	}
}

// radial gradient cpaint in user space of the current transform (centre cx,cy, radii r0..r1).
type gstop struct {
	o float64
	c color.NRGBA
}

func (x *cnv) radial(cx, cy, r0, r1 float64, st ...gstop) cpaint {
	m := x.st.m
	dcx, dcy := m.apply(cx, cy)
	k := x.mscale()
	R0, R1 := r0*k, r1*k
	return func(px, py float64) color.NRGBA {
		d := math.Hypot(px-dcx, py-dcy)
		t := 0.0
		if R1 > R0 {
			t = (d - R0) / (R1 - R0)
		}
		return gradAt(st, t)
	}
}

func (x *cnv) linear(x0, y0, x1, y1 float64, st ...gstop) cpaint {
	m := x.st.m
	ax, ay := m.apply(x0, y0)
	bx, by := m.apply(x1, y1)
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	return func(px, py float64) color.NRGBA {
		t := 0.0
		if l2 > 0 {
			t = ((px-ax)*dx + (py-ay)*dy) / l2
		}
		return gradAt(st, t)
	}
}

func gradAt(st []gstop, t float64) color.NRGBA {
	if len(st) == 0 {
		return color.NRGBA{}
	}
	if t <= st[0].o {
		return st[0].c
	}
	for i := 1; i < len(st); i++ {
		if t <= st[i].o {
			a, b := st[i-1], st[i]
			k := 0.0
			if b.o > a.o {
				k = (t - a.o) / (b.o - a.o)
			}
			// interpolate in premultiplied space like canvas
			aa, ba := float64(a.c.A)/255, float64(b.c.A)/255
			al := aa + (ba-aa)*k
			if al <= 0 {
				return color.NRGBA{}
			}
			ch := func(x, y uint8) uint8 {
				v := (float64(x)*aa + (float64(y)*ba-float64(x)*aa)*k) / al
				return uint8(math.Max(0, math.Min(255, v+.5)))
			}
			return color.NRGBA{ch(a.c.R, b.c.R), ch(a.c.G, b.c.G), ch(a.c.B, b.c.B), uint8(al*255 + .5)}
		}
	}
	return st[len(st)-1].c
}

// cssC parses "#rgb", "#rrggbb", "#rrggbbaa", "rgb(...)" and "rgba(...)".
func cssC(s string) color.NRGBA {
	if len(s) > 0 && s[0] == '#' {
		if len(s) == 9 {
			c := hexC(s[:7])
			var a int
			for _, ch := range s[7:9] {
				a *= 16
				switch {
				case ch >= '0' && ch <= '9':
					a += int(ch - '0')
				case ch >= 'a' && ch <= 'f':
					a += int(ch-'a') + 10
				case ch >= 'A' && ch <= 'F':
					a += int(ch-'A') + 10
				}
			}
			c.A = uint8(a)
			return c
		}
		return hexC(s)
	}
	return parseRGBA(s)
}

// parseRGBA reads "rgba(r,g,b,a)" / "rgb(r,g,b)" / "r,g,b".
func parseRGBA(s string) color.NRGBA {
	var v [4]float64
	v[3] = 1
	n, cur, frac, dig := 0, 0.0, 0.0, false
	end := func() {
		if dig && n < 4 {
			v[n] = cur
			n++
		}
		cur, frac, dig = 0, 0, false
	}
	for _, ch := range s {
		switch {
		case ch >= '0' && ch <= '9':
			if frac > 0 {
				cur += float64(ch-'0') * frac
				frac /= 10
			} else {
				cur = cur*10 + float64(ch-'0')
			}
			dig = true
		case ch == '.':
			frac = .1
			dig = true
		case ch == ',' || ch == ')':
			end()
		}
	}
	end()
	cl := func(x float64) uint8 { return uint8(math.Max(0, math.Min(255, x+.5))) }
	return color.NRGBA{cl(v[0]), cl(v[1]), cl(v[2]), cl(v[3] * 255)}
}

// fa: the web fa(c, a) — colour c with alpha a.
func fa(s string, a float64) color.NRGBA {
	c := cssC(s)
	c.A = uint8(math.Max(0, math.Min(1, a))*255 + .5)
	return c
}
