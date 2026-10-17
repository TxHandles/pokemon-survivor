package sim

import "math"

// Obstacle is a tree, rock or landmark on the map. Generated per 300px chunk from a seeded hash,
// like the web chunkRaw(), so the client and the sim agree on where they are. Collision uses the
// circles of the drawn picture (web obsShape / SHP), scaled to the obstacle.
type Obstacle struct {
	X, Y, R float64
	Tree    bool
	Lm      int     // landmark: 1 or 2 = the location's landmarks, LmSign = the location sign
	Sc      float64 // landmark scale
	cs      [][3]float64
	bx, by  float64 // bounding circle of cs
	br      float64
}

// LmSign marks the location sign landmark.
const LmSign = 3

const ChunkSize = 300

// mulberry-style generator matching the web rng()
func jsRng(a uint32) func() float64 {
	return func() float64 {
		a += 0x6D2B79F5
		t := a
		t = (t ^ t>>15) * (1 | t)
		t = (t + (t^t>>7)*(61|t)) ^ t
		return float64(t^t>>14) / 4294967296
	}
}

// chunkRaw: the seeded trees, rocks and the odd landmark of chunk (cx, cy) (web chunkRaw).
func (w *World) chunkRaw(cx, cy int) []Obstacle {
	k := [2]int{cx, cy}
	if c, ok := w.obsRaw[k]; ok {
		return c
	}
	if len(w.obsRaw) > 600 {
		w.obsRaw = map[[2]int][]Obstacle{}
	}
	ws := uint32(w.Seed)
	r := jsRng(uint32(int32(cx)*73856093) ^ uint32(int32(cy)*19349663) ^ ws)
	var c []Obstacle
	n := 0
	if r() >= .15 {
		n = 1 + int(r()*3)
	}
	for i := 0; i < n; i++ {
		tree := r() < .6
		rr := 14 + r()*10
		if tree {
			rr = 11 + r()*4
		}
		x := float64(cx)*ChunkSize + r()*ChunkSize
		y := float64(cy)*ChunkSize + r()*ChunkSize
		if hyp(x, y) < 130 {
			continue
		}
		c = append(c, Obstacle{X: x, Y: y, R: rr, Tree: tree})
	}
	r2 := jsRng(uint32(int32(cx)*19349663) ^ uint32(int32(cy)*83492791) ^ ws ^ 0x2f6b1d)
	if r2() < .1 {
		x := float64(cx)*ChunkSize + 40 + r2()*(ChunkSize-80)
		y := float64(cy)*ChunkSize + 60 + r2()*(ChunkSize-90)
		if hyp(x, y) >= 260 {
			lm := 2
			if r2() < .5 {
				lm = 1
			}
			c = append(c, Obstacle{X: x, Y: y, R: 30, Lm: lm, Sc: .9 + r2()*.25})
		}
	}
	w.obsRaw[k] = c
	return c
}

var noTreePools = map[string]bool{"lava": true, "ice": true, "deep": true}

const lmClear = 150 // nothing else grows this close to a landmark

// Chunk returns the obstacles in chunk (cx, cy) for the current location (web chunk): trees are
// kept out of lava / ice / deep water, landmarks out of hazard pools, and the ground around
// landmarks is cleared.
func (w *World) Chunk(cx, cy int) []Obstacle {
	k := [2]int{cx, cy}
	if c, ok := w.obs[k]; ok {
		return c
	}
	if len(w.obs) > 500 {
		w.obs = map[[2]int][]Obstacle{}
	}
	raw := w.chunkRaw(cx, cy)
	c := make([]Obstacle, 0, len(raw)+1)
	near := func(f func(h Pool) bool) bool {
		for i := -1; i <= 1; i++ {
			for j := -1; j <= 1; j++ {
				for _, h := range w.Pools(cx+i, cy+j) {
					if f(h) {
						return true
					}
				}
			}
		}
		return false
	}
	nt := noTreePools[BiomeHazard[w.Biome]]
	for _, b := range raw {
		if nt && b.Tree && near(func(h Pool) bool { m := h.R + b.R*1.6; return sq(b.X-h.X)+sq(b.Y-h.Y) < m*m }) {
			continue
		}
		c = append(c, b)
	}
	for _, o := range w.LMSP {
		if int(math.Floor(o.X/ChunkSize)) == cx && int(math.Floor(o.Y/ChunkSize)) == cy {
			c = append(c, o)
		}
	}
	if BiomeHazard[w.Biome] != "" {
		out := c[:0]
		for _, b := range c {
			if b.Lm != 0 && near(func(h Pool) bool { m := h.R + 70; return sq(b.X-h.X)+sq(b.Y-h.Y) < m*m }) {
				continue
			}
			out = append(out, b)
		}
		c = out
	}
	L := append([]Obstacle(nil), w.LMSP...)
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			for _, b := range w.chunkRaw(cx+i, cy+j) {
				if b.Lm != 0 {
					L = append(L, b)
				}
			}
		}
	}
	if len(L) > 0 {
		out := c[:0]
		for _, b := range c {
			keep := b.Lm != 0
			if !keep {
				keep = true
				for _, l := range L {
					R := lmClear*l.Sc + b.R
					if sq(b.X-l.X)+sq(b.Y-l.Y+40) < R*R {
						keep = false
						break
					}
				}
			}
			if keep {
				out = append(out, b)
			}
		}
		c = out
	}
	for i := range c {
		w.shape(&c[i])
	}
	w.obs[k] = c
	return c
}

func sq(x float64) float64 { return x * x }

// Scenery returns the tree and rock styles of the current location (web curSCB).
func (w *World) Scenery() (tree, rock string) {
	if l, ok := locScenery[w.Biome]; ok && w.LI >= 0 && w.LI < 3 {
		return l[w.LI][0], l[w.LI][1]
	}
	return "oak", "boulder"
}

// LandmarkKind is the kind of landmark o in the current location ("" if none).
func (w *World) LandmarkKind(o Obstacle) string {
	if o.Lm == LmSign {
		return "sign"
	}
	if l, ok := locLandmarks[w.Biome]; ok && w.LI >= 0 && w.LI < 3 && o.Lm >= 1 && o.Lm <= 2 {
		return l[w.LI][o.Lm-1]
	}
	return ""
}

// shape fills o's collision circles (web obsShape).
func (w *World) shape(o *Obstacle) {
	var L [][3]float64
	var s, ox, oy float64
	switch {
	case o.Lm != 0:
		L, s, ox, oy = lmShapes[w.LandmarkKind(*o)], o.Sc, 80, 150
	case o.Tree:
		t, _ := w.Scenery()
		L = shpTree[t]
		if L == nil { // blossom shares the oak's shape
			L = shpTree["oak"]
		}
		s, ox, oy = o.R/14.5, 64, 104
	default:
		_, r := w.Scenery()
		L = shpRock[r]
		if L == nil {
			L = shpRock["boulder"]
		}
		k := rockScale[r]
		if k == 0 {
			k = 1
		}
		s, ox, oy = o.R/34*k, 64, 92
	}
	o.cs = make([][3]float64, len(L))
	o.bx, o.by, o.br = o.X, o.Y, 0
	if len(L) == 0 {
		return
	}
	mx, my := 0.0, 0.0
	for i, c := range L {
		o.cs[i] = [3]float64{o.X + (c[0]-ox)*s, o.Y + (c[1]-oy)*s, c[2] * s}
		mx += o.cs[i][0]
		my += o.cs[i][1]
	}
	mx /= float64(len(L))
	my /= float64(len(L))
	br := 0.0
	for _, c := range o.cs {
		br = math.Max(br, math.Sqrt(sq(c[0]-mx)+sq(c[1]-my))+c[2])
	}
	o.bx, o.by, o.br = mx, my, br
}

// Bounds is the circle around o's whole collision shape.
func (o Obstacle) Bounds() (x, y, r float64) { return o.bx, o.by, o.br }

// Circles is o's collision shape: circles {x, y, r} in world coordinates.
func (o Obstacle) Circles() [][3]float64 { return o.cs }

// placeLandmarks puts the location sign and one of each landmark around the player when a
// location starts (web lmPlace). The angle comes from the run seed, not the gameplay rng.
func (w *World) placeLandmarks() {
	w.LMSP = nil
	w.obs = map[[2]int][]Obstacle{}
	if w.P == nil {
		return
	}
	x0, y0 := w.P.X, w.P.Y
	a0 := jsRng(uint32(w.Seed)^uint32(w.BiomeN)*2654435761)() * 6.283
	pt := func(a, d float64) (float64, float64) { return x0 + math.Cos(a)*d, y0 + math.Sin(a)*d }
	sx, sy := pt(-1.9, 150)
	w.LMSP = append(w.LMSP, Obstacle{X: sx, Y: sy, R: 20, Lm: LmSign, Sc: .9})
	for i, k := range []int{1, 2} {
		qx, qy := pt(a0+float64(i)*2.6, 330+float64(i)*80)
		w.LMSP = append(w.LMSP, Obstacle{X: qx, Y: qy, R: 30, Lm: k, Sc: 1.05})
	}
}

// push moves (x,y) out of any obstacle it overlaps (web push).
func (w *World) push(x, y *float64, rad float64) {
	cx, cy := int(math.Floor(*x/ChunkSize)), int(math.Floor(*y/ChunkSize))
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			for _, b := range w.Chunk(cx+i, cy+j) {
				qx, qy, qm := *x-b.bx, *y-b.by, b.br+rad
				if qx*qx+qy*qy >= qm*qm {
					continue
				}
				for _, c := range b.cs {
					dx, dy := *x-c[0], *y-c[1]
					mm := c[2] + rad
					if d2 := dx*dx + dy*dy; d2 < mm*mm {
						d := math.Sqrt(d2)
						if d == 0 {
							d = .01
						}
						*x, *y = c[0]+dx/d*mm, c[1]+dy/d*mm
					}
				}
			}
		}
	}
}
