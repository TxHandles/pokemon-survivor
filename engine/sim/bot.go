package sim

import (
	"math"
	"math/rand"
)

// Bot steers like the web balance bot (hbot3 steerHuman): every ~0.13s it scores 24 directions by
// predicted enemy contact, crowding, obstacles and the best nearby gem, and walks the cheapest one.
type Bot struct {
	W          *World
	rng        *rand.Rand
	decT, wang float64
	lx, ly     float64
	mx, my     float64
}

func NewBot(w *World, seed int64) *Bot {
	r := rand.New(rand.NewSource(seed))
	return &Bot{W: w, rng: r, wang: r.Float64() * 6.28, lx: 1}
}

func (b *Bot) Input(dt float64) Input {
	b.decT -= dt
	if b.decT <= 0 {
		b.decT = .1 + b.rng.Float64()*.06
		b.steer()
	}
	return Input{MoveX: b.mx, MoveY: b.my}
}

func (b *Bot) steer() {
	w := b.W
	P := w.P
	sp := P.Sp * (1 + .08*w.pe("boots"))
	const LA = .5
	hpf := P.HP / P.MHP
	riskw := .45 + 3.2*math.Pow(math.Max(0, 1-hpf), 2)
	var near []*Enemy
	cx, cy, cn := 0.0, 0.0, 0
	for _, e := range w.E {
		if e.Dead {
			continue
		}
		dx, dy := e.X-P.X, e.Y-P.Y
		d2 := dx*dx + dy*dy
		if d2 < 330*330 {
			near = append(near, e)
		}
		if d2 < 420*420 {
			cx += e.X
			cy += e.Y
			cn++
		}
	}
	var gt *Gem
	gs := 0.0
	for _, g := range w.Gems {
		d := dist(g.X, g.Y, P.X, P.Y)
		if d > 520 {
			continue
		}
		v := 1.0
		if g.V > 3 {
			v = 3
		}
		v /= 1 + d/90
		if v > gs {
			gs, gt = v, g
		}
	}
	hasC := cn > 0
	ccx, ccy := P.X, P.Y
	if hasC {
		ccx, ccy = cx/float64(cn), cy/float64(cn)
	}
	var oc []Obstacle
	if !has(P.Types, "flying") && !has(P.Types, "ghost") {
		chx, chy := int(math.Floor(P.X/ChunkSize)), int(math.Floor(P.Y/ChunkSize))
		for i := -1; i <= 1; i++ {
			for j := -1; j <= 1; j++ {
				for _, o := range w.Chunk(chx+i, chy+j) {
					if dist(o.X, o.Y, P.X, P.Y) < o.R+sp*LA+30 {
						oc = append(oc, o)
					}
				}
			}
		}
	}
	bx, by, bc := 1.0, 0.0, 1e18
	for k := 0; k < 24; k++ {
		a := float64(k) * math.Pi / 12
		ux, uy := math.Cos(a), math.Sin(a)
		px, py := P.X+ux*sp*LA, P.Y+uy*sp*LA
		c := 0.0
		for _, o := range oc {
			for _, f := range []float64{.2, .5, 1} {
				qx, qy := P.X+ux*sp*LA*f, P.Y+uy*sp*LA*f
				dd := dist(qx, qy, o.X, o.Y) - o.R - 13
				if dd < 0 {
					c += b2f(f < .6, 500, 160)
					break
				} else if dd < 18 && f < .6 {
					c += (18 - dd) * 3
				}
			}
		}
		for _, e := range near {
			esp := e.Sp * LA
			l := math.Max(1, dist(px, py, e.X, e.Y))
			qx, qy := e.X+(px-e.X)/l*esp, e.Y+(py-e.Y)/l*esp
			dd := dist(px, py, qx, qy) - e.R - 12
			Rs := 60 + b2f(e.Boss, 70, 0) + b2f(e.Elite, 30, 0) + e.R*.5
			if dd < Rs {
				wt := b2f(e.Boss, 3.5, b2f(e.Elite, 2.2, 1)) * math.Max(.6, math.Min(3, e.Dm/9))
				c += riskw * wt * math.Pow((Rs-dd)/Rs, 2) * 60
			}
		}
		ahead := 0
		for _, e := range near {
			dx, dy := e.X-P.X, e.Y-P.Y
			d := hyp(dx, dy)
			if d < 230 && (dx*ux+dy*uy)/math.Max(d, 1e-9) > .8 {
				ahead++
			}
		}
		c += float64(ahead) * riskw * 1.6
		if gt != nil {
			d0, d1 := dist(gt.X, gt.Y, P.X, P.Y), dist(gt.X, gt.Y, px, py)
			c += (d1-d0)*.35 + d1*.02
		} else if hasC {
			c += math.Abs(dist(ccx, ccy, px, py)-170) * .05
		} else if ux*math.Cos(b.wang)+uy*math.Sin(b.wang) <= 0 {
			c += 8
		}
		c += (1-(ux*b.lx+uy*b.ly))*3.5 + b.rng.Float64()*2.5
		if c < bc {
			bc, bx, by = c, ux, uy
		}
	}
	if !hasC && gt == nil {
		b.wang += (b.rng.Float64() - .5) * .6
	}
	b.lx, b.ly = bx, by
	b.mx, b.my = bx, by
}
