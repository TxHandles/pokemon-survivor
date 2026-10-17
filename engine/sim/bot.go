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
	riskw := .45 + 3.2*ppow(math.Max(0, 1-hpf), 2)
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
	// pickups: berries when hurt, magnets, candy
	var it *Pickup
	for _, p := range w.Items {
		d := dist(p.X, p.Y, P.X, P.Y)
		if d > 420 {
			continue
		}
		wt := 3.0
		switch p.K {
		case "berry":
			wt = b2f(hpf < .6, 14, 1)
		case "magnet":
			wt = 4
		case "bomb":
			wt = b2f(len(near) > 60, 5, 1)
		case "candy":
			wt = 5
		}
		if v := wt / (1 + d/90); v > gs {
			gs, gt, it = v, nil, p
		}
	}
	var cx2, cy2 float64
	hasChest := false
	if len(near) < 40 && hpf > .5 {
		for _, m := range w.MC {
			if m.Cursed && hpf < .8 {
				continue
			}
			if d := dist(m.X, m.Y, P.X, P.Y); d < 600 {
				if v := 5 / (1 + d/120); v > gs {
					gs, it, gt, hasChest, cx2, cy2 = v, nil, nil, true, m.X, m.Y
				}
			}
		}
		for _, c := range w.Chests {
			if d := dist(c.X, c.Y, P.X, P.Y); d < 600 {
				if v := 6 / (1 + d/120); v > gs {
					gs, it, gt, hasChest, cx2, cy2 = v, nil, nil, true, c.X, c.Y
				}
			}
		}
	}
	if pc := w.PCN; pc != nil && hpf > .4 && w.n() > 1 { // revive teammates
		if d := dist(pc.X, pc.Y, P.X, P.Y); d < 1400 {
			if v := 9 / (1 + d/300); v > gs {
				gs, it, gt, hasChest, cx2, cy2 = v, nil, nil, true, pc.X, pc.Y
			}
		}
	}
	tx, ty, hasT := 0.0, 0.0, false
	if hasChest && it == nil && gt == nil {
		tx, ty, hasT = cx2, cy2, true
	} else if it != nil {
		tx, ty, hasT = it.X, it.Y, true
	} else if gt != nil {
		tx, ty, hasT = gt.X, gt.Y, true
	}
	pools := BiomeHazard[w.Biome] != "" && w.hzK() > 0 && !has(P.Types, "flying") && !has(P.Types, "ghost")
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
					o.X, o.Y, o.R = o.Bounds()
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
				c += riskw * wt * ppow((Rs-dd)/Rs, 2) * 60
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
		for _, b := range w.EB {
			bx, by := b.X+b.VX*LA, b.Y+b.VY*LA
			if d := math.Min(dist(px, py, bx, by), dist(px, py, b.X, b.Y)); d < 55 {
				c += (55 - d) * (2.2 + b.Dm/10)
			}
		}
		for _, h := range w.HZ {
			if dist(px, py, h.X, h.Y) < h.R+22 {
				c += b2f(h.Gone, 400, 260)
			}
		}
		for _, f := range w.FX {
			if f.Enemy && f.Kind == "tele" && dist(px, py, f.X, f.Y) < f.R+22 {
				c += 260
			}
		}
		for _, e := range near {
			if e.Rk == "charge" && e.cwu > 0 {
				L := e.Sp*5*.45 + 20
				co, si := math.Cos(e.cang), math.Sin(e.cang)
				u, v := (px-e.X)*co+(py-e.Y)*si, -(px-e.X)*si+(py-e.Y)*co
				if u > -10 && u < L+10 && math.Abs(v) < e.R+18 {
					c += 300
				}
			}
		}
		if pools && w.HazAt(px, py) != "" {
			c += 220
		}
		if hasT {
			d0, d1 := dist(tx, ty, P.X, P.Y), dist(tx, ty, px, py)
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
	if !hasC && !hasT {
		b.wang += (b.rng.Float64() - .5) * .6
	}
	b.lx, b.ly = bx, by
	b.mx, b.my = bx, by
}
