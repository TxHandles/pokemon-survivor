package client

import (
	"encoding/json"
	"image"
	"image/color"
	"io/fs"
	"math"
)

// Ground tiles painted like the web version (flRows / flPat / flDec / FD): a half-resolution noise
// ground, large structured patterns (paving, plates, dried mud, basalt) where the region noise
// allows, then decals. Biome/location palettes come from data/art/floors.json (exported from the
// web FLB/FLV tables by tools/export_art.py).

type flEl struct {
	kind  string
	n     float64
	col   string   // css colour ("" when the painter takes none)
	cols  []string // palette (leaf, petal)
	bias  float64
	scale float64
	glow  string
}

type flDef struct {
	A, B, C string
	G       float64
	Pat     string
	El      [][]any
	Amb     string
	els     []flEl
	a, b, c [3]float64
}

type flData struct {
	Floors map[string][]*flDef
	Amb    map[string]map[string]any
	Lfx    map[string][]map[string]any
}

var flDB *flData

func loadFloors(f fs.FS) {
	b, err := fs.ReadFile(f, "art/floors.json")
	if err != nil {
		return
	}
	d := &flData{}
	if json.Unmarshal(b, d) != nil {
		return
	}
	for _, locs := range d.Floors {
		for _, fd := range locs {
			if fd == nil {
				continue
			}
			fd.a, fd.b, fd.c = rgb3(fd.A), rgb3(fd.B), rgb3(fd.C)
			for _, e := range fd.El {
				el := flEl{}
				if len(e) < 5 {
					continue
				}
				el.kind, _ = e[0].(string)
				el.n, _ = e[1].(float64)
				switch v := e[2].(type) {
				case string:
					el.col = v
				case []any:
					for _, x := range v {
						if s, ok := x.(string); ok {
							el.cols = append(el.cols, s)
						}
					}
				}
				el.bias, _ = e[3].(float64)
				el.scale, _ = e[4].(float64)
				if len(e) > 5 {
					el.glow, _ = e[5].(string)
				}
				fd.els = append(fd.els, el)
			}
		}
	}
	flDB = d
}

func rgb3(s string) [3]float64 {
	c := cssC(s)
	return [3]float64{float64(c.R), float64(c.G), float64(c.B)}
}

func floorDefFor(biome string, li int) *flDef {
	if flDB == nil {
		return nil
	}
	l := flDB.Floors[biome]
	if len(l) == 0 {
		l = flDB.Floors["normal"]
	}
	if li < 0 || li >= len(l) {
		li = 0
	}
	if len(l) == 0 {
		return nil
	}
	return l[li]
}

// ---- noise (web fvh / fvn / frg / fsm) ----

func fvh(ix, iy, s int) float64 {
	h := uint32(int32(ix))*374761393 + uint32(int32(iy))*668265263 + uint32(int32(s))*1442695041
	h = (h ^ h>>13) * 1274126177
	return float64(h^h>>16) / 4294967296
}

func fvn(x, y float64, s int) float64 {
	fx0, fy0 := math.Floor(x), math.Floor(y)
	ix, iy := int(fx0), int(fy0)
	fx, fy := x-fx0, y-fy0
	u, v := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b, c, d := fvh(ix, iy, s), fvh(ix+1, iy, s), fvh(ix, iy+1, s), fvh(ix+1, iy+1, s)
	return a + (b-a)*u + (c-a)*v + (a-b-c+d)*u*v
}

func fsm(a, b, v float64) float64 {
	t := math.Max(0, math.Min(1, (v-a)/(b-a)))
	return t * t * (3 - 2*t)
}

// srng: the web srng(n) generator (mulberry32 seeded with n·2654435761, computed in doubles).
func srng(n float64) func() float64 {
	v := math.Mod(n*2654435761, 4294967296)
	if v < 0 {
		v += 4294967296
	}
	return jsRngC(uint32(v))
}

func jsRngC(a uint32) func() float64 {
	return func() float64 {
		a += 0x6D2B79F5
		t := a
		t = (t ^ t>>15) * (1 | t)
		t = (t + (t^t>>7)*(61|t)) ^ t
		return float64(t^t>>14) / 4294967296
	}
}

type webFloor struct {
	d   *flDef
	bs  int
	dec map[[2]int][]flDecal
}

type flDecal struct {
	e      *flEl
	px, py float64
	seed   float64
}

func (f *webFloor) frg(x, y float64) float64 {
	return fvn(x/640, y/640, f.bs+11)*.75 + fvn(x/210, y/210, f.bs+12)*.25
}

// decals of one chunk, shared by neighbours so edge decals continue across seams (web flDec)
func (f *webFloor) decals(cx, cy int) []flDecal {
	k := [2]int{cx, cy}
	if l, ok := f.dec[k]; ok {
		return l
	}
	if len(f.dec) > 220 {
		f.dec = map[[2]int][]flDecal{}
	}
	var L []flDecal
	R := jsRngC(uint32(int64(cx)*92837111) ^ uint32(int64(cy)*689287499) ^ uint32(int64(f.bs)*7))
	for i := range f.d.els {
		e := &f.d.els[i]
		n := int(math.Floor(e.n*(.55+R()*.9) + .5))
		for j := 0; j < n; j++ {
			px, py := float64(cx)*cs+R()*cs, float64(cy)*cs+R()*cs
			rv := f.frg(px, py)
			w := .8
			if e.bias > 0 {
				w = fsm(.35, .7, rv)
			} else if e.bias < 0 {
				w = 1 - fsm(.3, .65, rv)
			}
			dn := fvn(px/120, py/120, f.bs+5)
			if R() > w*(.45+dn*.9) {
				continue
			}
			L = append(L, flDecal{e, px, py, math.Trunc(R() * 1e9)})
		}
	}
	f.dec[k] = L
	return L
}

// buildTile paints chunk (cx, cy) into a 300×300 RGBA image.
func (f *webFloor) buildTile(cx, cy int) *image.RGBA {
	d := f.d
	const N = cs/2 + 2
	ox, oy := cx*cs, cy*cs
	// half-res ground noise (web flRows)
	var g [N * N][3]float64
	for j := 0; j < N; j++ {
		for i := 0; i < N; i++ {
			wx, wy := ox+(i-1)*2+1, oy+(j-1)*2+1
			fx, fy := float64(wx), float64(wy)
			fv := fvn(fx/150, fy/150, f.bs+1)*.55 + fvn(fx/48, fy/48, f.bs+2)*.3 + fvn(fx/14, fy/14, f.bs+3)*.15
			r := fsm(.48, .72, f.frg(fx, fy)) * .9
			gr := (fvh(wx>>1, wy>>1, f.bs+4) - .5) * d.G * 2
			for k := 0; k < 3; k++ {
				v := d.a[k] + (d.b[k]-d.a[k])*fv
				g[j*N+i][k] = math.Max(0, math.Min(255, math.Round(v+(d.c[k]-v)*r+gr)))
			}
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, cs, cs))
	// upscale ×2 with bilinear filtering (source pixels 1..N-2)
	for Y := 0; Y < cs; Y++ {
		v := float64(Y)/2 + .75
		y0 := int(v)
		ty := v - float64(y0)
		for X := 0; X < cs; X++ {
			u := float64(X)/2 + .75
			x0 := int(u)
			tx := u - float64(x0)
			o := img.PixOffset(X, Y)
			for k := 0; k < 3; k++ {
				a := g[y0*N+x0][k]*(1-tx) + g[y0*N+x0+1][k]*tx
				b := g[(y0+1)*N+x0][k]*(1-tx) + g[(y0+1)*N+x0+1][k]*tx
				img.Pix[o+k] = uint8(a*(1-ty) + b*ty + .5)
			}
			img.Pix[o+3] = 255
		}
	}
	x := newCnv(img)
	f.pattern(x, cx, cy)
	for dj := -1; dj <= 1; dj++ {
		for di := -1; di <= 1; di++ {
			for _, dc := range f.decals(cx+di, cy+dj) {
				lx, ly := dc.px-float64(ox), dc.py-float64(oy)
				if lx < -50 || ly < -50 || lx > cs+50 || ly > cs+50 {
					continue
				}
				R := srng(dc.seed)
				s := dc.e.scale * (.8 + R()*.45)
				if p := fdPainters[dc.e.kind]; p != nil {
					x.save()
					p(x, R, lx, ly, s, dc.e)
					x.restore()
					x.globalAlpha(1)
				}
			}
		}
	}
	return img
}

func rgbaF(r, g, b int, a float64) color.NRGBA {
	cl := func(v int) uint8 { return uint8(max(0, min(255, v))) }
	return color.NRGBA{cl(r), cl(g), cl(b), uint8(math.Max(0, math.Min(1, a))*255 + .5)}
}

// pattern: the web flPat (worn paving, plates, dried mud, basalt).
func (f *webFloor) pattern(x *cnv, cx, cy int) {
	p := f.d.Pat
	if p == "" {
		return
	}
	ox, oy := float64(cx*cs), float64(cy*cs)
	bs := f.bs
	if p == "flag" || p == "plate" {
		T, t0, t1 := 60.0, .3, .46
		if p == "flag" {
			T, t0, t1 = 44, .42, .62
		}
		for gy := int(math.Floor(oy/T)) - 1; gy <= int(math.Ceil((oy+cs)/T)); gy++ {
			sh := 0.0
			if p == "flag" && gy&1 != 0 {
				sh = T / 2
			}
			for gx := int(math.Floor((ox-sh)/T)) - 1; gx <= int(math.Ceil((ox+cs)/T)); gx++ {
				wx, wy := float64(gx)*T+sh, float64(gy)*T
				h := fvh(gx, gy, bs+41)
				a := fsm(t0, t1, f.frg(wx+T/2, wy+T/2))
				if a < .05 || h < .12 {
					continue
				}
				lx, ly := wx-ox, wy-oy
				if p == "flag" {
					j := (fvh(gx, gy, bs+42) - .5) * 3
					sh2 := (h - .5) * 14
					x.setFill(rgbaF(int(110+sh2), int(100+sh2), int(92+sh2), .16*a))
					x.fillRect(lx+2+j, ly+2, T-4, T-4)
					x.setStroke(rgbaF(0, 0, 0, .4*a))
					x.lineWidth(2)
					x.strokeRect(lx+1+j, ly+1, T-2, T-2)
					x.setFill(rgbaF(255, 255, 255, .05*a))
					x.fillRect(lx+3+j, ly+3, T-6, 2)
					if h > .82 {
						x.setStroke(rgbaF(0, 0, 0, .35*a))
						x.lineWidth(1)
						x.beginPath()
						x.moveTo(lx+T*h*.6, ly+2)
						x.lineTo(lx+T*.5, ly+T*.5)
						x.lineTo(lx+T-3, ly+T*.8)
						x.stroke()
					}
					if h < .3 {
						x.setFill(rgbaF(70, 110, 50, .35*a))
						x.fillRect(lx, ly+T-3, T*.6, 3)
					}
				} else {
					sh2 := (h - .5) * 16
					x.setFill(rgbaF(int(120+sh2), int(132+sh2), int(146+sh2), .09*a))
					x.fillRect(lx+1, ly+1, T-2, T-2)
					x.setStroke(rgbaF(0, 0, 0, .45*a))
					x.lineWidth(2)
					x.strokeRect(lx+1, ly+1, T-2, T-2)
					x.setStroke(rgbaF(255, 255, 255, .06*a))
					x.lineWidth(1)
					x.beginPath()
					x.moveTo(lx+2, ly+T-2)
					x.lineTo(lx+2, ly+2)
					x.lineTo(lx+T-2, ly+2)
					x.stroke()
					x.setFill(rgbaF(150, 160, 175, .4*a))
					for _, q := range [][2]float64{{5, 5}, {T - 5, 5}, {5, T - 5}, {T - 5, T - 5}} {
						x.beginPath()
						x.arc(lx+q[0], ly+q[1], 1.6, 0, 6.283)
						x.fill()
					}
					if h > .85 {
						x.setFill(rgbaF(150, 70, 30, .18*a))
						x.beginPath()
						x.ellipse(lx+T*.6, ly+T*.55, T*.25, T*.18, h*6, 0, 6.283, false)
						x.fill()
					}
				}
			}
		}
		return
	}
	// voronoi-ish cracked cells: mud / basalt
	T := 52.0
	mud := p == "mud"
	if mud {
		T = 38
	}
	for gy := int(math.Floor(oy/T)) - 1; gy <= int(math.Ceil((oy+cs)/T))+1; gy++ {
		for gx := int(math.Floor(ox/T)) - 1; gx <= int(math.Ceil((ox+cs)/T))+1; gx++ {
			cxw := (float64(gx) + .2 + fvh(gx, gy, bs+51)*.6) * T
			cyw := (float64(gy) + .2 + fvh(gx, gy, bs+52)*.6) * T
			lo, hi := .4, .62
			if mud {
				lo, hi = .5, .7
			}
			a := fsm(lo, hi, f.frg(cxw, cyw))
			if a < .05 {
				continue
			}
			for _, dd := range [][2]int{{1, 0}, {0, 1}, {1, -1}} {
				dx, dy := dd[0], dd[1]
				nx := (float64(gx+dx) + .2 + fvh(gx+dx, gy+dy, bs+51)*.6) * T
				ny := (float64(gy+dy) + .2 + fvh(gx+dx, gy+dy, bs+52)*.6) * T
				mx, my := (cxw+nx)/2-ox, (cyw+ny)/2-oy
				vx, vy := -(ny - cyw), nx-cxw
				l := math.Hypot(vx, vy)
				if l == 0 {
					l = 1
				}
				hl := T * .42
				if mud {
					x.setStroke(rgbaF(30, 18, 10, .5*a))
					x.lineWidth(1.6)
				} else {
					x.setStroke(rgbaF(0, 0, 0, .55*a))
					x.lineWidth(2.2)
				}
				x.beginPath()
				x.moveTo(mx-vx/l*hl, my-vy/l*hl)
				x.lineTo(mx+vx/l*hl, my+vy/l*hl)
				x.stroke()
				if !mud && fvh(gx, gy+dx*7, bs+53) > .94 {
					x.setStroke(rgbaF(255, 110, 40, .45*a))
					x.lineWidth(1)
					x.stroke()
				}
			}
			if mud {
				x.setFill(rgbaF(255, 220, 170, .04*a))
				x.beginPath()
				x.arc(cxw-ox, cyw-oy, T*.3, 0, 6.283)
				x.fill()
			}
		}
	}
}

// ---- decal painters (web FD); x = canvas, R = rng, (px,py) position, s scale ----

type fdPainter func(x *cnv, R func() float64, px, py, s float64, e *flEl)

func cosA(a float64) float64 { return math.Cos(a) }
func sinA(a float64) float64 { return math.Sin(a) }
func ri(v float64) int       { return int(v) } // JS (expr)|0 for positive values

var fdPainters = map[string]fdPainter{
	"tuft": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		x.setStrokeS(e.col)
		x.lineWidth(1.4 * s)
		x.beginPath()
		n := ri(4 + R()*4)
		for i := 0; i < n; i++ {
			a := -1.57 + (R()-.5)*1.4
			l := (5 + R()*6) * s
			o := (R() - .5) * 6 * s
			x.moveTo(px+o, py)
			x.quadTo(px+o+cosA(a)*l*.3, py+sinA(a)*l*.6, px+o+cosA(a)*l, py+sinA(a)*l)
		}
		x.stroke()
	},
	"flower": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(2 + R()*5)
		for i := 0; i < n; i++ {
			qx, qy, r := px+(R()-.5)*22*s, py+(R()-.5)*16*s, (1.4+R()*1.2)*s
			x.setFillS(e.col)
			for k := 0; k < 5; k++ {
				a := float64(k)*1.2566 + R()*.3
				x.beginPath()
				x.arc(qx+cosA(a)*r, qy+sinA(a)*r, r*.8, 0, 6.283)
				x.fill()
			}
			x.setFillS("#f6d860")
			x.beginPath()
			x.arc(qx, qy, r*.6, 0, 6.283)
			x.fill()
		}
	},
	"dot": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(6 + R()*14)
		x.setFillS(e.col)
		for i := 0; i < n; i++ {
			r := (R()*R()*2.2 + .6) * s
			x.beginPath()
			x.arc(px+(R()-.5)*40*s, py+(R()-.5)*40*s, r, 0, 6.283)
			x.fill()
		}
	},
	"pebble": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(1 + R()*3)
		for i := 0; i < n; i++ {
			qx, qy := px+(R()-.5)*14*s, py+(R()-.5)*10*s
			rx := (2 + R()*3.5) * s
			ry := rx * (.6 + R()*.3)
			a := R() * 3
			x.setFillS("rgba(0,0,0,.25)")
			x.beginPath()
			x.ellipse(qx+1, qy+1.5, rx, ry, a, 0, 6.283, false)
			x.fill()
			x.setFillS(e.col)
			x.beginPath()
			x.ellipse(qx, qy, rx, ry, a, 0, 6.283, false)
			x.fill()
			x.setFillS("rgba(255,255,255,.12)")
			x.beginPath()
			x.ellipse(qx-rx*.3, qy-ry*.35, rx*.45, ry*.35, a, 0, 6.283, false)
			x.fill()
		}
	},
	"blotch": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*4)
		x.setFillS(e.col)
		for i := 0; i < n; i++ {
			ex, ey := px+(R()-.5)*30*s, py+(R()-.5)*22*s
			rx, ry := (10+R()*16)*s, (7+R()*10)*s
			x.beginPath()
			x.ellipse(ex, ey, rx, ry, R()*3, 0, 6.283, false)
			x.fill()
		}
	},
	"clover": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*5)
		x.setFillS(e.col)
		for i := 0; i < n; i++ {
			qx, qy, r, a0 := px+(R()-.5)*20*s, py+(R()-.5)*14*s, 1.8*s, R()*6
			for k := 0; k < 3; k++ {
				a := a0 + float64(k)*2.094
				x.beginPath()
				x.arc(qx+cosA(a)*r, qy+sinA(a)*r, r, 0, 6.283)
				x.fill()
			}
		}
	},
	"crack": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a := R() * 6.283
		qx, qy := px, py
		pts := [][2]float64{{qx, qy}}
		n := ri(4 + R()*5)
		for i := 0; i < n; i++ {
			a += (R() - .5) * 1.3
			l := (6 + R()*10) * s
			qx += cosA(a) * l
			qy += sinA(a) * l
			pts = append(pts, [2]float64{qx, qy})
		}
		path := func() {
			x.beginPath()
			x.moveTo(pts[0][0], pts[0][1])
			for _, p := range pts {
				x.lineTo(p[0], p[1])
			}
			if R() < .6 {
				b := pts[1+ri(R()*float64(len(pts)-2))]
				d := -1.0
				if R() < .5 {
					d = 1
				}
				ba := a + d*(.8+R()*.6)
				x.moveTo(b[0], b[1])
				x.lineTo(b[0]+cosA(ba)*9*s, b[1]+sinA(ba)*9*s)
			}
		}
		if e.glow != "" {
			x.setStroke(fa(e.glow, .18))
			x.lineWidth(7 * s)
			path()
			x.stroke()
			x.setStrokeS("rgba(0,0,0,.55)")
			x.lineWidth(2.6 * s)
			path()
			x.stroke()
			x.setStrokeS(e.glow)
			x.lineWidth(1.1 * s)
			path()
			x.stroke()
		} else {
			x.setStrokeS(e.col)
			x.lineWidth(1.3 * s)
			path()
			x.stroke()
			x.setStrokeS("rgba(255,255,255,.06)")
			x.lineWidth(1 * s)
			x.save()
			x.translate(1, 1)
			path()
			x.stroke()
			x.restore()
		}
	},
	"leaf": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(2 + R()*5)
		for i := 0; i < n; i++ {
			qx, qy := px+(R()-.5)*26*s, py+(R()-.5)*20*s
			l := (3 + R()*3) * s
			a := R() * 6.283
			cc := e.col
			if len(e.cols) > 0 {
				cc = e.cols[ri(R()*float64(len(e.cols)))]
			} else {
				R()
			}
			x.setFillS(cc)
			x.beginPath()
			x.ellipse(qx, qy, l, l*.45, a, 0, 6.283, false)
			x.fill()
			x.setStrokeS("rgba(0,0,0,.25)")
			x.lineWidth(.7)
			x.beginPath()
			x.moveTo(qx-cosA(a)*l, qy-sinA(a)*l)
			x.lineTo(qx+cosA(a)*l, qy+sinA(a)*l)
			x.stroke()
		}
	},
	"fern": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a0 := R() * 6.283
		l := (12 + R()*10) * s
		x.setStrokeS(e.col)
		x.lineWidth(1.2 * s)
		for f := 0; f < 3; f++ {
			a := a0 + float64(f-1)*1.1
			ex, ey := px+cosA(a)*l, py+sinA(a)*l
			x.beginPath()
			x.moveTo(px, py)
			x.lineTo(ex, ey)
			for k := 1; k < 6; k++ {
				t := float64(k) / 6
				mx, my := px+(ex-px)*t, py+(ey-py)*t
				w := (1 - t) * 5 * s
				x.moveTo(mx, my)
				x.lineTo(mx+cosA(a+1.1)*w, my+sinA(a+1.1)*w)
				x.moveTo(mx, my)
				x.lineTo(mx+cosA(a-1.1)*w, my+sinA(a-1.1)*w)
			}
			x.stroke()
		}
	},
	"root": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		x.setStrokeS(e.col)
		a := R() * 6.283
		qx, qy, w := px, py, 4*s
		x.beginPath()
		x.moveTo(qx, qy)
		for i := 0; i < 6; i++ {
			a += (R() - .5) * .9
			nx, ny := qx+cosA(a)*9*s, qy+sinA(a)*9*s
			x.lineWidth(w)
			x.quadTo(qx+cosA(a+.5)*4, qy+sinA(a+.5)*4, nx, ny)
			x.stroke()
			x.beginPath()
			x.moveTo(nx, ny)
			qx, qy = nx, ny
			w = math.Max(1, w*.78)
		}
	},
	"shroom": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(1 + R()*3)
		for i := 0; i < n; i++ {
			qx, qy, r := px+(R()-.5)*12*s, py+(R()-.5)*8*s, (1.8+R()*1.8)*s
			x.setFillS("#d8d0c0")
			x.fillRect(qx-.6*s, qy, 1.2*s, r*1.2)
			x.setFillS(e.col)
			x.beginPath()
			x.arc(qx, qy, r, 3.14, 0)
			x.fill()
			if len(e.col) == 7 && e.col != "#d8d0c0" {
				x.setFill(fa(e.col, .25))
				x.beginPath()
				x.arc(qx, qy, r*2.6, 0, 6.283)
				x.fill()
			}
		}
	},
	"ripple": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(2 + R()*2)
		r0 := (6 + R()*8) * s
		x.setStrokeS(e.col)
		x.lineWidth(1 * s)
		for i := 0; i < n; i++ {
			x.globalAlpha(1 - float64(i)/float64(n)*.7)
			rr := r0 + float64(i)*6*s
			x.beginPath()
			x.ellipse(px, py, rr, rr*.55, 0, 0, 6.283, false)
			x.stroke()
		}
		x.globalAlpha(1)
	},
	"shell": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a := R() * 6.283
		r := (3 + R()*2.5) * s
		x.save()
		x.translate(px, py)
		x.rotate(a)
		x.setFillS(e.col)
		x.beginPath()
		x.moveTo(0, r*.5)
		x.arc(0, 0, r, 3.6, 5.8)
		x.closePath()
		x.fill()
		x.setStrokeS("rgba(0,0,0,.3)")
		x.lineWidth(.7)
		x.beginPath()
		for k := 0; k < 4; k++ {
			b := 3.8 + float64(k)*.6
			x.moveTo(0, r*.5)
			x.lineTo(cosA(b)*r, sinA(b)*r)
		}
		x.stroke()
		x.restore()
	},
	"weed": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(2 + R()*3)
		x.setStrokeS(e.col)
		x.lineWidth(1.6 * s)
		for i := 0; i < n; i++ {
			qx, qy, ph := px+(R()-.5)*8*s, py, R()*6
			x.beginPath()
			x.moveTo(qx, qy)
			for k := 0; k < 5; k++ {
				qy -= 3.5 * s
				x.lineTo(qx+sinA(ph+float64(k)*1.2)*2.5*s, qy)
			}
			x.stroke()
		}
	},
	"scorch": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		r := (14 + R()*14) * s
		x.st.fill = x.radial(px, py, 0, r, gstop{0, fa(e.col, .3)}, gstop{.6, fa(e.col, .13)}, gstop{1, fa(e.col, 0)})
		x.beginPath()
		ry := r * (.7 + R()*.3)
		x.ellipse(px, py, r, ry, R()*3, 0, 6.283, false)
		x.fill()
	},
	"bolt": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a := R() * 6.283
		qx, qy := px, py
		var seg [][4]float64
		for i := 0; i < 6; i++ {
			a += (R() - .5) * 1.6
			nx, ny := qx+cosA(a)*7*s, qy+sinA(a)*7*s
			seg = append(seg, [4]float64{qx, qy, nx, ny})
			if R() < .3 {
				b := a + (R()-.5)*2.4
				seg = append(seg, [4]float64{nx, ny, nx + cosA(b)*6*s, ny + sinA(b)*6*s})
			}
			qx, qy = nx, ny
		}
		for _, st := range []struct {
			w float64
			c color.NRGBA
		}{{5 * s, fa(e.col, .1)}, {1.1 * s, fa(e.col, .5)}} {
			x.setStroke(st.c)
			x.lineWidth(st.w)
			x.beginPath()
			for _, q := range seg {
				x.moveTo(q[0], q[1])
				x.lineTo(q[2], q[3])
			}
			x.stroke()
		}
	},
	"drift": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*3)
		for i := 0; i < n; i++ {
			qx, qy, rx := px+(R()-.5)*40*s, py+(R()-.5)*18*s, (14+R()*20)*s
			x.st.fill = x.radial(qx, qy, 0, rx, gstop{0, fa(e.col, .22)}, gstop{1, fa(e.col, 0)})
			x.beginPath()
			x.ellipse(qx, qy, rx, rx*.5, 0, 0, 6.283, false)
			x.fill()
		}
	},
	"frost": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(1 + R()*3)
		x.setStrokeS(e.col)
		x.lineWidth(.9 * s)
		for i := 0; i < n; i++ {
			qx, qy, r := px+(R()-.5)*20*s, py+(R()-.5)*20*s, (2.5+R()*3)*s
			x.beginPath()
			for k := 0; k < 3; k++ {
				a := float64(k) * 1.047
				x.moveTo(qx-cosA(a)*r, qy-sinA(a)*r)
				x.lineTo(qx+cosA(a)*r, qy+sinA(a)*r)
			}
			x.stroke()
		}
	},
	"petal": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*6)
		for i := 0; i < n; i++ {
			cc := e.col
			if len(e.cols) > 0 {
				cc = e.cols[ri(R()*float64(len(e.cols)))]
			} else {
				R()
			}
			x.setFillS(cc)
			ex, ey := px+(R()-.5)*34*s, py+(R()-.5)*26*s
			rx, ry := (1.8+R())*s, (1+R()*.6)*s
			x.beginPath()
			x.ellipse(ex, ey, rx, ry, R()*3, 0, 6.283, false)
			x.fill()
		}
	},
	"stone": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(5 + R()*3)
		r := (5 + R()*6) * s
		pts := make([][2]float64, n)
		for i := 0; i < n; i++ {
			a := float64(i)/float64(n)*6.283 + R()*.4
			rr := r * (.7 + R()*.4)
			pts[i] = [2]float64{px + cosA(a)*rr, py + sinA(a)*rr*.75}
		}
		x.setFillS("rgba(0,0,0,.28)")
		x.beginPath()
		for i, p := range pts {
			if i == 0 {
				x.moveTo(p[0]+1.5, p[1]+2)
			} else {
				x.lineTo(p[0]+1.5, p[1]+2)
			}
		}
		x.fill()
		x.setFillS(e.col)
		x.beginPath()
		for i, p := range pts {
			if i == 0 {
				x.moveTo(p[0], p[1])
			} else {
				x.lineTo(p[0], p[1])
			}
		}
		x.closePath()
		x.fill()
		x.setStrokeS("rgba(255,255,255,.1)")
		x.lineWidth(1)
		x.beginPath()
		x.moveTo(pts[n-2][0], pts[n-2][1])
		x.lineTo(pts[n-1][0], pts[n-1][1])
		x.lineTo(pts[0][0], pts[0][1])
		x.stroke()
	},
	"bubble": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(2 + R()*4)
		for i := 0; i < n; i++ {
			r, qx, qy := (1.5+R()*3)*s, px+(R()-.5)*16*s, py+(R()-.5)*12*s
			x.setStrokeS(e.col)
			x.lineWidth(.9 * s)
			x.beginPath()
			x.arc(qx, qy, r, 0, 6.283)
			x.stroke()
			x.setFillS("rgba(255,255,255,.18)")
			x.beginPath()
			x.arc(qx-r*.35, qy-r*.35, r*.3, 0, 6.283)
			x.fill()
		}
	},
	"reed": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*4)
		x.setStrokeS(e.col)
		for i := 0; i < n; i++ {
			qx, l, lean := px+(R()-.5)*10*s, (8+R()*10)*s, (R()-.5)*5*s
			x.lineWidth(1.3 * s)
			x.beginPath()
			x.moveTo(qx, py)
			x.quadTo(qx+lean*.3, py-l*.6, qx+lean, py-l)
			x.stroke()
			if R() < .4 {
				x.lineWidth(2.6 * s)
				x.beginPath()
				x.moveTo(qx+lean*.8, py-l*.8)
				x.lineTo(qx+lean, py-l)
				x.stroke()
			}
		}
	},
	"dune": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*3)
		a := .25 + R()*.3
		l := (30 + R()*30) * s
		x.setStrokeS(e.col)
		x.lineWidth(1.4 * s)
		for i := 0; i < n; i++ {
			oy, ph := float64(i)*6*s, R()*6
			x.beginPath()
			for k := 0; k <= 10; k++ {
				t := float64(k)/10 - .5
				X, Y := px+t*l, py+oy+sinA(ph+t*5)*3*s+t*l*a
				if k == 0 {
					x.moveTo(X, Y)
				} else {
					x.lineTo(X, Y)
				}
			}
			x.stroke()
		}
	},
	"shrub": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		x.setStrokeS(e.col)
		x.lineWidth(1 * s)
		n := ri(5 + R()*5)
		x.beginPath()
		for i := 0; i < n; i++ {
			a, l := R()*6.283, (3+R()*6)*s
			x.moveTo(px, py)
			x.lineTo(px+cosA(a)*l, py+sinA(a)*l*.7)
			x.lineTo(px+cosA(a+.4)*l*1.25, py+sinA(a+.4)*l*.85)
		}
		x.stroke()
	},
	"streak": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(4 + R()*6)
		x.setStrokeS(e.col)
		x.lineWidth(1.1 * s)
		x.beginPath()
		for i := 0; i < n; i++ {
			qx, qy, l := px+(R()-.5)*30*s, py+(R()-.5)*20*s, (5+R()*8)*s
			x.moveTo(qx, qy)
			x.quadTo(qx+l*.6, qy-l*.1, qx+l, qy-l*.45)
		}
		x.stroke()
	},
	"feather": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a, l := R()*6.283, (5+R()*4)*s
		x.save()
		x.translate(px, py)
		x.rotate(a)
		x.setFillS(e.col)
		x.beginPath()
		x.ellipse(0, 0, l, l*.28, 0, 0, 6.283, false)
		x.fill()
		x.setStrokeS("rgba(0,0,0,.3)")
		x.lineWidth(.6)
		x.beginPath()
		x.moveTo(-l*1.2, 0)
		x.lineTo(l, 0)
		x.stroke()
		x.restore()
	},
	"swirl": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		x.setStrokeS(e.col)
		x.lineWidth(1.4 * s)
		d := -1.0
		if R() < .5 {
			d = 1
		}
		a0 := R() * 6.283
		x.beginPath()
		for k := 0; k <= 40; k++ {
			t := float64(k) / 40
			a, r := a0+d*t*9, (1+t*13)*s
			if k == 0 {
				x.moveTo(px+cosA(a)*r, py+sinA(a)*r)
			} else {
				x.lineTo(px+cosA(a)*r, py+sinA(a)*r)
			}
		}
		x.stroke()
	},
	"crystal": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(1 + R()*3)
		for i := 0; i < n; i++ {
			qx, qy := px+(R()-.5)*12*s, py+(R()-.5)*8*s
			h := (4 + R()*6) * s
			w := h * .4
			a := (R() - .5) * .8
			x.save()
			x.translate(qx, qy)
			x.rotate(a)
			x.setFill(fa(e.col, .18))
			x.beginPath()
			x.arc(0, -h*.4, h*1.1, 0, 6.283)
			x.fill()
			x.setFillS(e.col)
			x.beginPath()
			x.moveTo(0, -h)
			x.lineTo(w, -h*.3)
			x.lineTo(w*.6, 0)
			x.lineTo(-w*.6, 0)
			x.lineTo(-w, -h*.3)
			x.closePath()
			x.fill()
			x.setFillS("rgba(255,255,255,.35)")
			x.beginPath()
			x.moveTo(0, -h)
			x.lineTo(w*.15, -h*.25)
			x.lineTo(-w, -h*.3)
			x.closePath()
			x.fill()
			x.restore()
		}
	},
	"ring": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(7 + R()*6)
		r := (9 + R()*8) * s
		x.setFillS(e.col)
		for i := 0; i < n; i++ {
			a := float64(i)/float64(n)*6.283 + R()*.2
			x.beginPath()
			x.arc(px+cosA(a)*r, py+sinA(a)*r*.7, (1+R())*s, 0, 6.283)
			x.fill()
		}
	},
	"twig": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a, l := R()*6.283, (8+R()*10)*s
		x.setStrokeS(e.col)
		x.lineWidth(1.4 * s)
		x.beginPath()
		x.moveTo(px-cosA(a)*l/2, py-sinA(a)*l/2)
		x.lineTo(px+cosA(a)*l/2, py+sinA(a)*l/2)
		b := a + .6
		x.moveTo(px, py)
		x.lineTo(px+cosA(b)*l*.35, py+sinA(b)*l*.35)
		x.stroke()
	},
	"web": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		r := (8 + R()*8) * s
		n := ri(6 + R()*3)
		x.setStrokeS(e.col)
		x.lineWidth(.6)
		x.beginPath()
		for i := 0; i < n; i++ {
			a := float64(i) / float64(n) * 6.283
			x.moveTo(px, py)
			x.lineTo(px+cosA(a)*r, py+sinA(a)*r)
		}
		for k := 1; k <= 3; k++ {
			rr := r * float64(k) / 3.4
			for i := 0; i <= n; i++ {
				a := float64(i) / float64(n) * 6.283
				if i == 0 {
					x.moveTo(px+cosA(a)*rr, py+sinA(a)*rr)
				} else {
					x.lineTo(px+cosA(a)*rr, py+sinA(a)*rr)
				}
			}
		}
		x.stroke()
	},
	"chisel": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a := R() * 3
		n := ri(3 + R()*4)
		x.setStrokeS(e.col)
		x.lineWidth(1 * s)
		x.beginPath()
		for i := 0; i < n; i++ {
			o := (float64(i) - float64(n)/2) * 3 * s
			l := (4 + R()*5) * s
			cx, cy := px+cosA(a+1.57)*o, py+sinA(a+1.57)*o
			x.moveTo(cx, cy)
			x.lineTo(cx+cosA(a)*l, cy+sinA(a)*l)
		}
		x.stroke()
	},
	"bone": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a, l := R()*6.283, (5+R()*4)*s
		x.save()
		x.translate(px, py)
		x.rotate(a)
		x.setFillS(e.col)
		x.fillRect(-l, -.9*s, l*2, 1.8*s)
		for _, ee := range []float64{-l, l} {
			for _, o := range []float64{-1.3, 1.3} {
				x.beginPath()
				x.arc(ee, o*s, 1.5*s, 0, 6.283)
				x.fill()
			}
		}
		x.restore()
	},
	"scales": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		rows, cols := ri(2+R()*3), ri(3+R()*3)
		r := (3.5 + R()*1.5) * s
		x.setStrokeS(e.col)
		x.lineWidth(1 * s)
		x.beginPath()
		for j := 0; j < rows; j++ {
			for i := 0; i < cols; i++ {
				qx := px + (float64(i)-float64(cols)/2+float64(j%2)*.5)*r*2
				qy := py + float64(j)*r*1.2
				x.moveTo(qx+r, qy)
				x.arc(qx, qy, r, 0, 3.14)
			}
		}
		x.stroke()
	},
	"thorn": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		a := R() * 6.283
		qx, qy := px, py
		x.setStrokeS(e.col)
		x.lineWidth(1.3 * s)
		x.beginPath()
		x.moveTo(qx, qy)
		for i := 0; i < 5; i++ {
			a += (R() - .5) * 1
			nx, ny := qx+cosA(a)*7*s, qy+sinA(a)*7*s
			x.lineTo(nx, ny)
			d := -1.0
			if i%2 == 1 {
				d = 1
			}
			b := a + d*.9
			x.lineTo(nx+cosA(b)*3*s, ny+sinA(b)*3*s)
			x.moveTo(nx, ny)
			qx, qy = nx, ny
		}
		x.stroke()
	},
	"grate": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		w, h := (18+R()*10)*s, (12+R()*6)*s
		x.setFillS("rgba(0,0,0,.35)")
		x.fillRect(px-w/2, py-h/2, w, h)
		x.setStrokeS(e.col)
		x.lineWidth(1.5 * s)
		x.strokeRect(px-w/2, py-h/2, w, h)
		x.beginPath()
		for gx := px - w/2 + 4*s; gx < px+w/2-1; gx += 4 * s {
			x.moveTo(gx, py-h/2+2)
			x.lineTo(gx, py+h/2-2)
		}
		x.stroke()
	},
	"oil": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		r := (8 + R()*12) * s
		x.setFillS("rgba(8,8,12,.45)")
		x.beginPath()
		x.ellipse(px, py, r, r*.65, R()*3, 0, 6.283, false)
		x.fill()
		x.st.fill = x.linear(px-r, py, px+r, py, gstop{0, parseRGBA("rgba(120,80,200,0)")}, gstop{.4, parseRGBA("rgba(120,80,200,.12)")}, gstop{.6, parseRGBA("rgba(60,200,160,.12)")}, gstop{1, parseRGBA("rgba(60,200,160,0)")})
		x.beginPath()
		x.ellipse(px, py, r*.6, r*.35, 0, 0, 6.283, false)
		x.fill()
	},
	"bolts": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(2 + R()*3)
		for i := 0; i < n; i++ {
			qx, qy, r := px+(R()-.5)*14*s, py+(R()-.5)*10*s, 1.6*s
			x.setFillS(e.col)
			x.beginPath()
			for k := 0; k < 6; k++ {
				a := float64(k) * 1.047
				if k == 0 {
					x.moveTo(qx+cosA(a)*r, qy+sinA(a)*r)
				} else {
					x.lineTo(qx+cosA(a)*r, qy+sinA(a)*r)
				}
			}
			x.fill()
		}
	},
	"stripe": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		w, h := (26+R()*14)*s, 6*s
		a := (R() - .5) * .5
		x.save()
		x.translate(px, py)
		x.rotate(a)
		x.setFillS("rgba(200,160,40,.28)")
		x.fillRect(-w/2, -h/2, w, h)
		x.setFillS("rgba(20,20,20,.35)")
		for i := -w / 2; i < w/2; i += 6 * s {
			x.beginPath()
			x.moveTo(i, -h/2)
			x.lineTo(i+3*s, -h/2)
			x.lineTo(i+3*s-h, h/2)
			x.lineTo(i-h, h/2)
			x.fill()
		}
		x.restore()
	},
	"sparkle": func(x *cnv, R func() float64, px, py, s float64, e *flEl) {
		n := ri(3 + R()*6)
		for i := 0; i < n; i++ {
			qx, qy, r := px+(R()-.5)*30*s, py+(R()-.5)*24*s, (1+R()*1.6)*s
			x.setFillS(e.col)
			x.beginPath()
			x.moveTo(qx, qy-r*2)
			x.lineTo(qx+r*.5, qy)
			x.lineTo(qx, qy+r*2)
			x.lineTo(qx-r*.5, qy)
			x.fill()
			x.beginPath()
			x.moveTo(qx-r*2, qy)
			x.lineTo(qx, qy+r*.5)
			x.lineTo(qx+r*2, qy)
			x.lineTo(qx, qy-r*.5)
			x.fill()
		}
	},
}
