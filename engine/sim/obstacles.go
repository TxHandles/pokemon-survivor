package sim

import "math"

// Obstacle is a tree or rock on the map (circle collider). Generated per 300px chunk from a seeded
// hash, like the web chunkRaw(), so the client and the sim agree on where they are.
type Obstacle struct {
	X, Y, R float64
	Tree    bool
	Var     float64 // 0..1 visual variation
}

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

// Chunk returns the obstacles in chunk (cx, cy).
func (w *World) Chunk(cx, cy int) []Obstacle {
	k := [2]int{cx, cy}
	if c, ok := w.obs[k]; ok {
		return c
	}
	if len(w.obs) > 600 {
		w.obs = map[[2]int][]Obstacle{}
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
		c = append(c, Obstacle{x, y, rr, tree, r()})
	}
	w.obs[k] = c
	return c
}

// push moves (x,y) out of any obstacle it overlaps.
func (w *World) push(x, y *float64, rad float64) {
	cx, cy := int(math.Floor(*x/ChunkSize)), int(math.Floor(*y/ChunkSize))
	for i := -1; i <= 1; i++ {
		for j := -1; j <= 1; j++ {
			for _, b := range w.Chunk(cx+i, cy+j) {
				br := b.R
				if b.Tree {
					br *= .75 // trunk + lower canopy
				}
				dx, dy := *x-b.X, *y-b.Y
				mm := br + rad
				if d2 := dx*dx + dy*dy; d2 < mm*mm {
					d := math.Sqrt(d2)
					if d == 0 {
						d = .01
					}
					*x, *y = b.X+dx/d*mm, b.Y+dy/d*mm
				}
			}
		}
	}
}
