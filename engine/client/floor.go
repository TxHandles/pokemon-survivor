package client

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

const cs = sim.ChunkSize

// Floor caches pre-rendered 300px ground tiles for the current biome.
type Floor struct {
	biome string
	seed  uint32
	tiles map[[2]int]*ebiten.Image
	free  []*ebiten.Image
	wf    *webFloor // the web ground painter (nil = the simple fallback floor)
}

func NewFloor(biome string, li int, seed int64) *Floor {
	f := &Floor{biome: biome, seed: uint32(seed), tiles: map[[2]int]*ebiten.Image{}}
	if d := floorDefFor(biome, li); d != nil {
		f.wf = &webFloor{d: d, bs: int(uint32(seed*2654435761) % 1000000000), dec: map[[2]int][]flDecal{}}
	}
	return f
}

func (f *Floor) def() floorDef {
	if d, ok := floors[f.biome]; ok {
		return d
	}
	return floors["normal"]
}

// Draw paints the ground covering the screen; camX/camY is the world point at the screen's top-left.
func (f *Floor) Draw(dst *ebiten.Image, camX, camY float64) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	if f.wf != nil {
		dst.Fill(cssC(f.wf.d.A))
	} else {
		dst.Fill(f.def().A)
	}
	x0, x1 := int(math.Floor(camX/cs)), int(math.Floor((camX+float64(w))/cs))
	y0, y1 := int(math.Floor(camY/cs)), int(math.Floor((camY+float64(h))/cs))
	built := 0
	t0 := time.Now()
	for j := y0; j <= y1; j++ {
		for i := x0; i <= x1; i++ {
			k := [2]int{i, j}
			t := f.tiles[k]
			if t == nil {
				// build missing tiles, but spread the work when several are missing at once
				if built > 0 && (built >= 6 || time.Since(t0) > 30*time.Millisecond) {
					continue
				}
				built++
				t = f.build(i, j)
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(math.Round(float64(i*cs)-camX), math.Round(float64(j*cs)-camY))
			dst.DrawImage(t, op)
		}
	}
	// prepare one tile just outside the view when nothing had to be built this frame
	if built == 0 {
		cx, cy := (camX+float64(w)/2)/cs, (camY+float64(h)/2)/cs
		best, bd := [2]int{}, math.Inf(1)
		for j := y0 - 1; j <= y1+1; j++ {
			for i := x0 - 1; i <= x1+1; i++ {
				if f.tiles[[2]int{i, j}] != nil {
					continue
				}
				if d := math.Hypot(float64(i)+.5-cx, float64(j)+.5-cy); d < bd {
					best, bd = [2]int{i, j}, d
				}
			}
		}
		if !math.IsInf(bd, 1) {
			f.build(best[0], best[1])
		}
	}
	// evict tiles far from view
	if keep := (x1-x0+3)*(y1-y0+3) + 8; len(f.tiles) > keep {
		for k, t := range f.tiles {
			if k[0] < x0-1 || k[0] > x1+1 || k[1] < y0-1 || k[1] > y1+1 {
				f.free = append(f.free, t)
				delete(f.tiles, k)
			}
		}
	}
}

func (f *Floor) build(cx, cy int) *ebiten.Image {
	var img *ebiten.Image
	if n := len(f.free); n > 0 {
		img = f.free[n-1]
		f.free = f.free[:n-1]
		img.Clear()
	} else {
		img = ebiten.NewImage(cs, cs)
	}
	if f.wf != nil {
		img.WritePixels(f.wf.buildTile(cx, cy).Pix)
		f.tiles[[2]int{cx, cy}] = img
		return img
	}
	d := f.def()
	img.Fill(d.A)
	// large soft patches on a 100px world grid; neighbours' patches are drawn too so tiles join seamlessly
	for gy := -2; gy <= 4; gy++ {
		for gx := -2; gx <= 4; gx++ {
			wx, wy := cx*3+gx, cy*3+gy
			v := hash01(wx, wy, int(f.seed))
			if v < .45 {
				continue
			}
			col := d.B
			if v > .85 {
				col = d.C
			}
			x, y := float64(gx)*100+50+hash01(wx, wy, 7)*40-20, float64(gy)*100+50+hash01(wx, wy, 9)*40-20
			soft(img, x, y, 90+v*40, 70+v*40, alpha(col, .85))
		}
	}
	// faint grid of tile seams adds structure on stone-like floors
	if f.biome == "steel" || f.biome == "fighting" || f.biome == "rock" {
		g := alpha(mix(d.A, color.NRGBA{255, 255, 255, 255}, .08), .7)
		for i := 0; i <= cs; i += 60 {
			line(img, float64(i), 0, float64(i), cs, 1, g)
			line(img, 0, float64(i), cs, float64(i), 1, g)
		}
	}
	// decorations: each chunk's own set, plus the edges of its neighbours' sets
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			r := &prng{s: uint32((cx+i)*73856093) ^ uint32((cy+j)*19349663) ^ f.seed ^ 0x9e3779b9}
			for _, el := range d.Els {
				for k := 0; k < el.N; k++ {
					x, y := r.f()*cs+float64(i*cs), r.f()*cs+float64(j*cs)
					ci := r.f()
					sc := el.Scale * r.rg(.8, 1.25)
					rs := r.s
					if x < -45 || y < -45 || x > cs+45 || y > cs+45 {
						continue
					}
					col := el.Cols[int(ci*float64(len(el.Cols)))%len(el.Cols)]
					sub := &prng{s: rs ^ 0x51ed27}
					drawDeco(img, el.Kind, x, y, sc, col, sub)
				}
			}
		}
	}
	f.tiles[[2]int{cx, cy}] = img
	return img
}

func drawDeco(dst *ebiten.Image, kind string, x, y, s float64, col color.NRGBA, r *prng) {
	switch kind {
	case "tuft", "reed", "weed", "fern", "shrub":
		n := 3 + int(r.f()*3)
		hgt := 7 * s
		if kind == "reed" || kind == "weed" {
			hgt = 11 * s
		}
		for i := 0; i < n; i++ {
			a := -math.Pi/2 + (float64(i)-float64(n-1)/2)*.32 + r.rg(-.1, .1)
			l := hgt * r.rg(.7, 1.1)
			line(dst, x+float64(i-n/2)*1.4, y, x+math.Cos(a)*l, y+math.Sin(a)*l, 1.6*s, col)
		}
	case "flower":
		for i := 0; i < 5; i++ {
			a := float64(i) * 2 * math.Pi / 5
			disc(dst, x+math.Cos(a)*2.4*s, y+math.Sin(a)*2.4*s, 1.9*s, col)
		}
		disc(dst, x, y, 1.5*s, c(240, 200, 70, 255))
	case "clover":
		for i := 0; i < 3; i++ {
			a := float64(i)*2*math.Pi/3 - math.Pi/2
			disc(dst, x+math.Cos(a)*2.2*s, y+math.Sin(a)*2.2*s, 2.2*s, col)
		}
	case "pebble", "stone", "shell", "bone", "bolts":
		rr := 3 * s
		if kind == "stone" {
			rr = 5.5 * s
		}
		soft(dst, x+1, y+rr*.6, rr*1.3, rr*.6, c(0, 0, 0, 70))
		disc(dst, x, y, rr, col)
		disc(dst, x-rr*.3, y-rr*.35, rr*.45, alpha(mix(col, c(255, 255, 255, 255), .35), .8))
	case "dot", "sparkle", "frost", "chisel", "drift":
		disc(dst, x, y, 1.6*s, col)
	case "crack", "root", "twig", "thorn", "bolt", "grate", "stripe":
		px, py := x, y
		a := r.f() * 2 * math.Pi
		for i := 0; i < 4; i++ {
			a += r.rg(-.8, .8)
			l := r.rg(6, 13) * s
			nx, ny := px+math.Cos(a)*l, py+math.Sin(a)*l
			line(dst, px, py, nx, ny, 1.4*s, col)
			px, py = nx, ny
		}
	case "blotch", "scorch", "oil", "dune", "scales", "swirl":
		a := col
		if kind == "scorch" || kind == "oil" {
			a = alpha(col, .25)
		}
		soft(dst, x, y, 34*s, 24*s, a)
	case "leaf", "petal", "feather", "streak":
		a := r.f() * math.Pi
		l := 4 * s
		if kind == "streak" {
			l = 14 * s
		}
		co, si := math.Cos(a), math.Sin(a)
		p := &vector.Path{}
		p.MoveTo(float32(x-co*l), float32(y-si*l))
		p.QuadTo(float32(x-si*l*.45), float32(y+co*l*.45), float32(x+co*l), float32(y+si*l))
		p.QuadTo(float32(x+si*l*.45), float32(y-co*l*.45), float32(x-co*l), float32(y-si*l))
		fillPath(dst, p, col)
	case "ripple", "ring", "bubble", "web":
		ring(dst, x, y, 5*s, 1.2, col)
	case "shroom":
		rect(dst, x-1*s, y-1, 2*s, 4*s, c(230, 220, 200, 255))
		sector(dst, x, y-1, 4*s, -math.Pi/2, math.Pi/2, col)
	case "crystal":
		poly(dst, [][2]float64{{x, y - 7*s}, {x + 3*s, y}, {x, y + 2*s}, {x - 3*s, y}}, col)
	default:
		disc(dst, x, y, 1.5*s, col)
	}
}

// obstacle colours per biome
func obsCols(biome string) (trunk, leaf, leaf2, rock, rock2 color.NRGBA) {
	tcol := tc(biome)
	leaf = mix(c(47, 125, 58, 255), tcol, .45)
	leaf2 = mix(c(61, 163, 74, 255), tcol, .45)
	if biome == "grass" || biome == "normal" || biome == "bug" {
		leaf, leaf2 = c(47, 125, 58, 255), c(61, 163, 74, 255)
	}
	if biome == "fire" || biome == "ground" || biome == "rock" || biome == "steel" {
		leaf, leaf2 = mix(c(90, 80, 60, 255), tcol, .25), mix(c(120, 105, 75, 255), tcol, .25)
	}
	if biome == "ice" {
		leaf, leaf2 = c(150, 190, 210, 255), c(220, 240, 250, 255)
	}
	trunk = c(92, 64, 40, 255)
	rock = mix(c(109, 115, 128, 255), tcol, .3)
	rock2 = mix(c(138, 145, 160, 255), tcol, .3)
	return
}

// drawObstacle draws a tree or rock with its base at (x,y) in screen space.
func drawObstacle(dst *ebiten.Image, o sim.Obstacle, x, y float64, biome string) {
	trunk, leaf, leaf2, rock, rock2 := obsCols(biome)
	if o.Tree {
		r := o.R
		soft(dst, x, y+2, r*1.5, r*.55, c(0, 0, 0, 110))
		rect(dst, x-r*.22, y-r*1.4, r*.44, r*1.4, trunk)
		cy := y - r*2.0
		disc(dst, x-r*.65, cy+r*.3, r*.95, mix(leaf, c(0, 0, 0, 255), .25))
		disc(dst, x+r*.7, cy+r*.35, r*.9, mix(leaf, c(0, 0, 0, 255), .2))
		disc(dst, x, cy-r*.2, r*1.15, leaf)
		disc(dst, x-r*.35, cy-r*.55, r*.55, leaf2)
		disc(dst, x+r*.3, cy-r*.2, r*.35, alpha(leaf2, .8))
		return
	}
	r := o.R
	soft(dst, x, y+r*.25, r*1.35, r*.45, c(0, 0, 0, 110))
	var pts [][2]float64
	n := 7
	for i := 0; i < n; i++ {
		a := math.Pi + float64(i)/float64(n-1)*math.Pi
		k := .8 + .25*math.Sin(float64(i)*2.3+(o.X*.013))
		pts = append(pts, [2]float64{x + math.Cos(a)*r*k, y + r*.25 + math.Sin(a)*r*.95*k})
	}
	poly(dst, pts, rock)
	disc(dst, x-r*.25, y-r*.35, r*.35, alpha(rock2, .9))
	line(dst, x+r*.1, y-r*.1, x+r*.45, y+r*.15, 1.5, alpha(c(0, 0, 0, 255), .3))
}
