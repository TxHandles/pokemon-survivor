package client

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Battle particles: a client-side layer of sparks, embers, droplets, shards, smoke, shockwaves and
// flashes on top of each move's drawn effect. Every hit, KO, projectile impact, ring, beam and bolt
// throws out particles themed by the move's type, and moves of a family (punches, slashes, bites,
// sound moves, draining moves, explosives, powders, wind…) add their own touch. Purely visual:
// nothing here feeds back into the simulation, so co-op stays in lockstep.

type pKind uint8

const (
	pGlow  pKind = iota // soft additive blob
	pSpark              // streak along its velocity (additive)
	pDisc               // solid round drop / pebble
	pShard              // spinning triangle
	pStar               // four-point twinkle (additive)
	pRing               // expanding shockwave ring (additive)
	pSmoke              // soft dark puff (normal blend)
	pArt                // the type's particle art (data/art/types.png "b")
	pFlare              // big additive flash that pops and fades fast
)

type fxPart struct {
	x, y, vx, vy float64
	life, l0     float64
	sz, sz1      float64 // size at birth → at death
	rot, vr      float64
	grav, drag   float64
	col, col1    color.NRGBA // colour at birth → at death
	k            pKind
	art          *ebiten.Image
}

const maxParts = 1400

type fxState struct {
	parts    []fxPart
	seen     map[*sim.FX]bool
	proj     map[*sim.Proj]projSeen
	lastT    float64
	budget   int     // particles left to spawn this frame
	flash    float64 // screen flash strength
	flashCol color.NRGBA
	kickX    float64 // client camera kick (extra shake on heavy hits)
	kickY    float64
	world    *sim.World
	rate     map[rateKey]float64
}

type rateKey struct {
	s *sim.Slot
	k string
}

// often: true if this slot made this kind of effect less than gap seconds ago (spin and aura moves
// spawn effects every tick; their particles are thinned out).
func (s *fxState) often(sl *sim.Slot, k string, gap float64) bool {
	if sl == nil {
		return false
	}
	key := rateKey{sl, k}
	if t, ok := s.rate[key]; ok && s.lastT-t < gap {
		return true
	}
	s.rate[key] = s.lastT
	return false
}

type projSeen struct {
	x, y  float64
	t     string
	slot  *sim.Slot
	r     float64
	fresh bool
}

// ---- textures ----

func makeTex(n int, f func(x, y float64) float64) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			u, v := (float64(x)+.5)/float64(n)*2-1, (float64(y)+.5)/float64(n)*2-1
			a := math.Max(0, math.Min(1, f(u, v)))
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(a * 255)})
		}
	}
	return ebiten.NewImageFromImage(img)
}

var (
	texSpark = makeTex(64, func(u, v float64) float64 { // long soft streak, hot towards the front
		return math.Pow(math.Max(0, 1-math.Abs(v)*4.2), 1.5) * math.Pow(math.Max(0, 1-math.Abs(u)), .7) * (.55 + .45*(u+1)/2)
	})
	texStar = makeTex(64, func(u, v float64) float64 { // four-point twinkle
		d := math.Hypot(u, v)
		s := math.Max(math.Pow(math.Max(0, 1-math.Abs(u)*7), 2)*(1-math.Abs(v)), math.Pow(math.Max(0, 1-math.Abs(v)*7), 2)*(1-math.Abs(u)))
		return math.Max(s, math.Pow(math.Max(0, 1-d*2.2), 2))
	})
	texShard = makeTex(32, func(u, v float64) float64 { // sharp triangle
		if v > .8 || math.Abs(u) > (v+1)/2.2 {
			return 0
		}
		return 1
	})
	texRing = makeTex(128, func(u, v float64) float64 { // soft shockwave ring
		d := math.Hypot(u, v)
		return math.Pow(math.Max(0, 1-math.Abs(d-.86)*9), 1.6) + .35*math.Pow(math.Max(0, 1-math.Abs(d-.8)*4), 2)
	})
)

// ---- spawning ----

func (s *fxState) add(p fxPart) {
	if s.budget <= 0 {
		return
	}
	s.budget--
	if p.l0 == 0 {
		p.l0 = p.life
	}
	if len(s.parts) >= maxParts {
		copy(s.parts, s.parts[len(s.parts)/8:]) // drop the oldest eighth
		s.parts = s.parts[:len(s.parts)-len(s.parts)/8]
	}
	s.parts = append(s.parts, p)
}

// rnd: a cheap client-side random number (visual only; never touches the sim's generator).
var fxSeed uint32 = 2463534242

func rnd() float64 {
	fxSeed ^= fxSeed << 13
	fxSeed ^= fxSeed >> 17
	fxSeed ^= fxSeed << 5
	return float64(fxSeed) / 4294967296
}
func rr(a, b float64) float64 { return a + (b-a)*rnd() }

type typeLook struct {
	hot, mid, cool color.NRGBA // bright core → body → edge colours
	kind           string      // ember drop spark leaf shard rock dust wisp ring smoke glint star bubble speck flame feather impact
}

var typeLooks = map[string]typeLook{
	"fire":     {hexC("#fff2a8"), hexC("#ff9a1f"), hexC("#ff3a0a"), "ember"},
	"water":    {hexC("#e8fbff"), hexC("#4ab8ff"), hexC("#1a5cff"), "drop"},
	"electric": {hexC("#ffffff"), hexC("#ffe83a"), hexC("#ffb000"), "spark"},
	"grass":    {hexC("#eaffb0"), hexC("#6ed64a"), hexC("#1f8a2a"), "leaf"},
	"ice":      {hexC("#ffffff"), hexC("#a8f0ff"), hexC("#4ac0ff"), "shard"},
	"rock":     {hexC("#f2deb0"), hexC("#b08a52"), hexC("#6a4a2a"), "rock"},
	"ground":   {hexC("#f5dca0"), hexC("#c89a52"), hexC("#7a5a2a"), "dust"},
	"ghost":    {hexC("#f0d8ff"), hexC("#9a5aff"), hexC("#4a1a8a"), "wisp"},
	"psychic":  {hexC("#ffe0f4"), hexC("#ff5ab8"), hexC("#c01a8a"), "ring"},
	"dark":     {hexC("#d8b8ff"), hexC("#5a3a7a"), hexC("#1a1020"), "smoke"},
	"steel":    {hexC("#ffffff"), hexC("#cfe0f0"), hexC("#7a8aa0"), "glint"},
	"fairy":    {hexC("#ffffff"), hexC("#ffa8e0"), hexC("#ff5ac0"), "star"},
	"poison":   {hexC("#f4d0ff"), hexC("#c04aff"), hexC("#6a1a9a"), "bubble"},
	"bug":      {hexC("#f4ffb0"), hexC("#a8d81a"), hexC("#5a8a0a"), "speck"},
	"dragon":   {hexC("#e0e8ff"), hexC("#7a6aff"), hexC("#3a1ab0"), "flame"},
	"flying":   {hexC("#ffffff"), hexC("#c8e4ff"), hexC("#7aa8e0"), "feather"},
	"fighting": {hexC("#fff4d0"), hexC("#ff8a3a"), hexC("#c0301a"), "impact"},
	"normal":   {hexC("#ffffff"), hexC("#f0eadc"), hexC("#b8b0a0"), "star"},
}

func lookOf(t string) typeLook {
	if l, ok := typeLooks[t]; ok {
		return l
	}
	return typeLooks["normal"]
}

// burst throws n type-themed particles out of (x, y); sp = speed scale, dir/spread aim them (spread
// 2π = all round).
func (s *fxState) burst(t string, x, y float64, n int, sp, dir, spread float64) {
	L := lookOf(t)
	b := artImg("types", t+"/b")
	for i := 0; i < n; i++ {
		a := dir + (rnd()-.5)*spread
		v := sp * rr(.35, 1)
		vx, vy := math.Cos(a)*v, math.Sin(a)*v
		switch L.kind {
		case "ember", "flame":
			s.add(fxPart{x: x, y: y, vx: vx, vy: vy - 30, life: rr(.35, .8), sz: rr(5, 10), sz1: 1, grav: -140, drag: 2.5, col: L.hot, col1: L.cool, k: pGlow})
		case "drop":
			s.add(fxPart{x: x, y: y, vx: vx, vy: vy - sp*.5, life: rr(.4, .7), sz: rr(2.5, 4.5), sz1: 1.5, grav: 700, drag: .8, col: L.mid, col1: alpha(L.hot, 0), k: pDisc})
		case "spark", "glint":
			s.add(fxPart{x: x, y: y, vx: vx * 1.6, vy: vy * 1.6, life: rr(.15, .35), sz: rr(14, 26), sz1: 4, grav: b2f(L.kind == "glint", 500, 0), drag: 3, col: L.hot, col1: L.cool, k: pSpark})
		case "leaf", "speck", "feather":
			if b != nil && rnd() < .7 {
				s.add(fxPart{x: x, y: y, vx: vx, vy: vy, life: rr(.5, 1), sz: rr(10, 16), sz1: 6, rot: rnd() * 6, vr: rr(-8, 8), grav: 60, drag: 2.2, col: white, col1: alpha(white, 0), k: pArt, art: b})
			} else {
				s.add(fxPart{x: x, y: y, vx: vx, vy: vy, life: rr(.3, .6), sz: rr(4, 7), sz1: 1, drag: 2, col: L.hot, col1: L.cool, k: pGlow})
			}
		case "shard":
			s.add(fxPart{x: x, y: y, vx: vx * 1.2, vy: vy * 1.2, life: rr(.35, .7), sz: rr(6, 11), sz1: 3, rot: rnd() * 6, vr: rr(-14, 14), grav: 300, drag: 1.5, col: L.mid, col1: alpha(L.hot, 0), k: pShard})
		case "rock":
			s.add(fxPart{x: x, y: y, vx: vx, vy: vy - sp*.6, life: rr(.45, .8), sz: rr(4, 8), sz1: 3, rot: rnd() * 6, vr: rr(-10, 10), grav: 900, drag: .6, col: L.mid, col1: alpha(L.cool, 0), k: pShard})
		case "dust", "smoke":
			s.add(fxPart{x: x, y: y, vx: vx * .5, vy: vy*.5 - 15, life: rr(.5, .9), sz: rr(10, 18), sz1: rr(26, 40), drag: 2.5, col: alpha(L.mid, .55), col1: alpha(L.cool, 0), k: pSmoke})
		case "wisp":
			s.add(fxPart{x: x, y: y, vx: vx * .6, vy: vy*.6 - 20, life: rr(.6, 1.1), sz: rr(6, 11), sz1: 2, grav: -60, drag: 1.8, col: L.hot, col1: L.cool, k: pGlow})
		case "ring":
			if i%3 == 0 {
				s.add(fxPart{x: x, y: y, life: rr(.3, .45), sz: 6, sz1: rr(40, 70), col: L.mid, col1: alpha(L.cool, 0), k: pRing})
			} else {
				s.add(fxPart{x: x, y: y, vx: vx, vy: vy, life: rr(.3, .6), sz: rr(8, 14), sz1: 2, drag: 2, col: L.hot, col1: L.mid, k: pStar})
			}
		case "star":
			s.add(fxPart{x: x, y: y, vx: vx, vy: vy, life: rr(.3, .65), sz: rr(9, 16), sz1: 2, vr: rr(-6, 6), drag: 2.4, col: L.hot, col1: L.cool, k: pStar})
		case "bubble":
			s.add(fxPart{x: x, y: y, vx: vx * .6, vy: vy*.6 - 20, life: rr(.5, .9), sz: rr(3, 6), sz1: rr(6, 9), grav: -90, drag: 2, col: L.mid, col1: alpha(L.hot, 0), k: pDisc})
		case "impact":
			s.add(fxPart{x: x, y: y, vx: vx * 1.4, vy: vy * 1.4, life: rr(.15, .3), sz: rr(16, 26), sz1: 4, drag: 4, col: L.hot, col1: L.cool, k: pSpark})
		}
	}
}

func (s *fxState) flare(x, y, r float64, col color.NRGBA, life float64) {
	s.add(fxPart{x: x, y: y, life: life, sz: r, sz1: r * 1.6, col: col, col1: alpha(col, 0), k: pFlare})
}

func (s *fxState) shock(x, y, r0, r1 float64, col color.NRGBA, life float64) {
	s.add(fxPart{x: x, y: y, life: life, sz: r0, sz1: r1, col: col, col1: alpha(col, 0), k: pRing})
}

// ---- move families ----

var famKeys = []struct{ fam, words string }{
	{"drain", "drain absorb leech parabolic horn leech"},
	{"sound", "voice song sing roar howl screech uproar boomburst snarl chatter echoed clang noise aria round growl sonic bug buzz"},
	{"impact", "punch kick fist hammer slam stomp press throw smash crash tackle headbutt body butt mach wood arm knuckle jump bash lariat"},
	{"slash", "slash claw cut blade scratch swipe edge cleave cutter sword razor leaf blade night fury swipes scythe"},
	{"bite", "bite fang crunch jaw chomp"},
	{"explode", "bomb ball shot sphere orb seed egg sludge bullet cannonball blast explosion eruption burst barrage"},
	{"energy", "beam ray cannon laser pulse flash gleam prism"},
	{"powder", "powder spore pollen dust"},
	{"wind", "wind gust storm twister hurricane air tailwind blizzard sand cyclone tornado"},
}

var famCache = map[string]string{}

func moveFam(m *sim.Slot) string {
	if m == nil || m.Move == nil {
		return ""
	}
	if f, ok := famCache[m.ID]; ok {
		return f
	}
	n := " " + strings.ToLower(m.Move.Name) + " "
	f := ""
	for _, k := range famKeys {
		for _, w := range strings.Fields(k.words) {
			if strings.Contains(n, " "+w) || strings.Contains(n, w+" ") {
				f = k.fam
				break
			}
		}
		if f != "" {
			break
		}
	}
	famCache[m.ID] = f
	return f
}

func tierOf(m *sim.Slot) string {
	if m == nil || m.Move == nil {
		return "basic"
	}
	return m.Move.Tier
}

// ---- reacting to the simulation ----

func (g *Game) fxStep() {
	w := g.W
	s := &g.fxs
	if s.seen == nil || s.world != w {
		*s = fxState{seen: map[*sim.FX]bool{}, proj: map[*sim.Proj]projSeen{}, lastT: w.T, world: w, rate: map[rateKey]float64{}}
	}
	lvl := g.opt("fx")
	dt := math.Max(0, math.Min(.05, w.T-s.lastT))
	s.lastT = w.T
	if lvl < 2 {
		s.parts = s.parts[:0]
		s.flash = 0
		return
	}
	s.budget = 260
	if len(w.FX) > 140 || len(w.E) > 260 { // very busy screens: fewer particles per event
		s.budget = 140
	}
	// new effects
	alive := make(map[*sim.FX]bool, len(w.FX))
	for _, f := range w.FX {
		alive[f] = true
		if !s.seen[f] {
			g.fxOn(f)
		}
	}
	s.seen = alive
	// projectiles: trails while flying, an impact burst when they vanish
	cur := make(map[*sim.Proj]projSeen, len(w.PR))
	for _, p := range w.PR {
		ps, had := s.proj[p]
		ps = projSeen{x: p.X, y: p.Y, t: p.Type, slot: p.Slot, r: p.R}
		cur[p] = ps
		if dt > 0 && (len(w.PR) < 60 || rnd() < 60/float64(len(w.PR))) {
			L := lookOf(p.Type)
			sp := math.Hypot(p.VX, p.VY)
			if sp > 0 {
				ux, uy := p.VX/sp, p.VY/sp
				s.add(fxPart{x: p.X - ux*p.R, y: p.Y - uy*p.R, vx: -ux*40 + rr(-25, 25), vy: -uy*40 + rr(-25, 25), life: rr(.2, .4), sz: p.R * rr(1.2, 1.9), sz1: .5, drag: 3, col: L.mid, col1: alpha(L.cool, 0), k: pGlow})
				if rnd() < .35 {
					s.burst(p.Type, p.X, p.Y, 1, 60, math.Atan2(-uy, -ux), 1.2)
				}
			}
		}
		_ = had
	}
	for p, ps := range s.proj {
		if _, ok := cur[p]; !ok {
			g.fxImpact(ps)
		}
	}
	s.proj = cur
	// integrate
	ps := s.parts[:0]
	for _, p := range s.parts {
		p.life -= dt
		if p.life <= 0 {
			continue
		}
		dr := math.Max(0, 1-p.drag*dt)
		p.vx *= dr
		p.vy = p.vy*dr + p.grav*dt
		p.x += p.vx * dt
		p.y += p.vy * dt
		p.rot += p.vr * dt
		ps = append(ps, p)
	}
	s.parts = ps
	s.flash = math.Max(0, s.flash-dt*3.2)
	k := math.Pow(.0005, dt)
	s.kickX *= k
	s.kickY *= k
}

func (g *Game) kick(a float64) {
	if g.opt("shake") <= 0 {
		return
	}
	s := &g.fxs
	ang := rnd() * 6.283
	s.kickX += math.Cos(ang) * a * g.opt("shake")
	s.kickY += math.Sin(ang) * a * g.opt("shake")
}

func (g *Game) screenFlash(col color.NRGBA, a float64) {
	if g.opt("flash") <= 0 {
		a *= .35
	}
	s := &g.fxs
	if a > s.flash {
		s.flash, s.flashCol = a, col
	}
}

func (g *Game) fxOn(f *sim.FX) {
	s := &g.fxs
	switch f.Kind {
	case "txt": // a hit landed
		if f.Text == "" || f.Text[0] < '0' || f.Text[0] > '9' {
			return
		}
		t := f.Type
		if f.Slot != nil {
			t = f.Slot.Type
		}
		if t == "" {
			return
		}
		x, y := f.X, f.Y+10
		big := f.Col == "crit" || f.Col == "se"
		n := 5
		if big {
			n = 10
		}
		L := lookOf(t)
		s.flare(x, y, b2f(big, 22, 14), L.mid, .12)
		s.burst(t, x, y, n, b2f(big, 260, 180), 0, 2*math.Pi)
		if big {
			s.shock(x, y, 6, 46, L.hot, .25)
		}
		if !s.often(f.Slot, "fam", .05) {
			g.fxFamilyHit(f.Slot, t, x, y, big)
		}
	case "pop": // a KO
		L := lookOf(f.Type)
		s.flare(f.X, f.Y, f.R*2.2, L.hot, .18)
		s.shock(f.X, f.Y, f.R*.6, f.R*3.4, L.mid, .32)
		s.burst(f.Type, f.X, f.Y, 9, 240, 0, 2*math.Pi)
		for i := 0; i < 5; i++ {
			s.add(fxPart{x: f.X, y: f.Y, vx: rr(-120, 120), vy: rr(-160, 40), life: rr(.25, .45), sz: rr(8, 14), sz1: 1, vr: rr(-6, 6), drag: 3, col: white, col1: alpha(L.mid, 0), k: pStar})
		}
	case "ring":
		if f.Slot == nil && f.Type == "" {
			return
		}
		if s.often(f.Slot, "ring", .3) {
			return
		}
		t := f.Type
		if f.Slot != nil {
			t = f.Slot.Type
		}
		L := lookOf(t)
		R := math.Max(f.R, f.MR)
		if R < 20 {
			R = 60
		}
		s.shock(f.X, f.Y, R*.25, R*1.15, L.mid, .38)
		s.shock(f.X, f.Y, R*.1, R*.8, L.hot, .26)
		s.flare(f.X, f.Y, R*.55, L.mid, .16)
		n := int(math.Min(22, 8+R/12))
		for i := 0; i < n; i++ {
			a := float64(i) / float64(n) * 2 * math.Pi
			s.burst(t, f.X+math.Cos(a)*R*.4, f.Y+math.Sin(a)*R*.4, 1, R*2.6, a, .4)
		}
		if tierOf(f.Slot) == "final" {
			g.screenFlash(L.mid, .16)
			g.kick(6)
		}
	case "beam":
		t := f.Type
		if f.Slot != nil {
			t = f.Slot.Type
		}
		L := lookOf(t)
		ca, sa := math.Cos(f.A), math.Sin(f.A)
		s.flare(f.X+ca*14, f.Y+sa*14, 34+f.Wd, L.hot, .2)
		for d := 30.0; d < f.Ln; d += 46 {
			s.burst(t, f.X+ca*d, f.Y+sa*d, 1, 220, f.A+math.Pi/2*(1-2*math.Floor(rnd()*2)), .6)
		}
		ex, ey := f.X+ca*f.Ln, f.Y+sa*f.Ln
		s.flare(ex, ey, 30+f.Wd, L.mid, .18)
		s.shock(ex, ey, 8, 70, L.hot, .3)
		g.screenFlash(L.hot, b2f(tierOf(f.Slot) == "final", .2, .08))
		g.kick(b2f(tierOf(f.Slot) == "final", 7, 3))
	case "bolt":
		t := f.Type
		if f.Slot != nil {
			t = f.Slot.Type
		}
		s.flare(f.BX, f.BY, 20, lookOf(t).hot, .1)
		s.burst(t, f.BX, f.BY, 4, 220, 0, 2*math.Pi)
	case "arc":
		if f.Slot == nil || s.often(f.Slot, "arc", .14) {
			return
		}
		t := f.Slot.Type
		for i := 0; i < 5; i++ {
			a := f.A + (rnd()-.5)*f.Wd*2
			s.burst(t, f.X+math.Cos(a)*f.R*.9, f.Y+math.Sin(a)*f.R*.9, 1, 160, a, .7)
		}
	case "sky", "slam", "quake":
		t := f.Type
		if f.Slot != nil {
			t = f.Slot.Type
		}
		if t == "" {
			t = "ground"
		}
		R := math.Max(40, f.R)
		s.shock(f.X, f.Y, R*.2, R*1.2, lookOf(t).mid, .35)
		s.burst("ground", f.X, f.Y, 6, 200, 0, 2*math.Pi)
		s.burst(t, f.X, f.Y, 8, 260, -math.Pi/2, 2.4)
		g.kick(5)
	case "flash":
		g.screenFlash(white, .3)
	}
}

// fxImpact: a projectile ended (hit something or ran out) — a small splash, or an explosion for
// bombs and balls.
func (g *Game) fxImpact(p projSeen) {
	s := &g.fxs
	if p.t == "" {
		return
	}
	fam := moveFam(p.slot)
	L := lookOf(p.t)
	if fam == "explode" || p.r >= 11 {
		R := math.Max(26, p.r*4)
		s.flare(p.x, p.y, R, L.hot, .16)
		s.shock(p.x, p.y, R*.2, R*1.4, L.mid, .3)
		s.burst(p.t, p.x, p.y, 9, 260, 0, 2*math.Pi)
		for i := 0; i < 3; i++ {
			s.add(fxPart{x: p.x + rr(-8, 8), y: p.y + rr(-8, 8), vx: rr(-30, 30), vy: rr(-50, -10), life: rr(.5, .8), sz: rr(10, 16), sz1: rr(26, 36), drag: 2, col: alpha(hexC("#3a3030"), .45), col1: alpha(hexC("#3a3030"), 0), k: pSmoke})
		}
		g.kick(2.5)
		return
	}
	s.flare(p.x, p.y, math.Max(10, p.r*2.2), L.mid, .1)
	s.burst(p.t, p.x, p.y, 3, 150, 0, 2*math.Pi)
}

// fxFamilyHit: the extra touch a move family adds to each hit.
func (g *Game) fxFamilyHit(sl *sim.Slot, t string, x, y float64, big bool) {
	s := &g.fxs
	L := lookOf(t)
	switch moveFam(sl) {
	case "impact":
		s.shock(x, y, 4, b2f(big, 60, 40), L.hot, .2)
		for i := 0; i < 6; i++ {
			a := float64(i) / 6 * 2 * math.Pi
			s.add(fxPart{x: x, y: y, vx: math.Cos(a) * 420, vy: math.Sin(a) * 420, life: .14, sz: 22, sz1: 6, drag: 6, col: white, col1: alpha(L.mid, 0), k: pSpark})
		}
		if big {
			g.kick(3)
		}
	case "slash":
		a := rr(-.9, -.5)
		for i := -1; i <= 1; i++ {
			ox, oy := math.Cos(a+math.Pi/2)*float64(i)*7, math.Sin(a+math.Pi/2)*float64(i)*7
			s.add(fxPart{x: x + ox, y: y + oy, vx: math.Cos(a) * 30, vy: math.Sin(a) * 30, life: .2, sz: 40, sz1: 30, rot: a, col: white, col1: alpha(L.mid, 0), k: pSpark})
		}
	case "bite":
		for _, sd := range []float64{-1, 1} {
			s.add(fxPart{x: x, y: y + sd*16, vy: -sd * 160, life: .14, sz: 30, sz1: 22, rot: 0, drag: 5, col: white, col1: alpha(L.mid, 0), k: pSpark})
		}
		s.burst(t, x, y, 3, 120, 0, 2*math.Pi)
	case "sound":
		for i := 0; i < 3; i++ {
			s.add(fxPart{x: x, y: y, life: .3 + .08*float64(i), sz: 4, sz1: 34 + 14*float64(i), col: L.hot, col1: alpha(L.mid, 0), k: pRing})
		}
	case "drain":
		if pc := g.W.P; pc != nil {
			for i := 0; i < 3; i++ {
				dx, dy := pc.X-x, pc.Y-y
				d := math.Max(1, math.Hypot(dx, dy))
				sp := d / .45
				s.add(fxPart{x: x + rr(-6, 6), y: y + rr(-6, 6), vx: dx / d * sp * rr(.8, 1.1), vy: dy/d*sp*rr(.8, 1.1) - 40, life: .45, sz: 7, sz1: 3, col: hexC("#b8ff8a"), col1: hexC("#4aff6a"), k: pGlow})
			}
		}
	case "explode":
		s.flare(x, y, 30, L.hot, .14)
		s.shock(x, y, 6, 56, L.mid, .26)
	case "energy":
		s.flare(x, y, 26, L.hot, .12)
		s.add(fxPart{x: x, y: y, life: .18, sz: 30, sz1: 8, vr: 8, col: white, col1: alpha(L.mid, 0), k: pStar})
	case "powder":
		for i := 0; i < 4; i++ {
			s.add(fxPart{x: x + rr(-10, 10), y: y + rr(-10, 10), vx: rr(-30, 30), vy: rr(-40, 0), life: rr(.6, 1), sz: rr(8, 12), sz1: rr(22, 30), drag: 2, col: alpha(L.mid, .45), col1: alpha(L.hot, 0), k: pSmoke})
		}
	case "wind":
		a := rr(0, 6.28)
		for i := 0; i < 4; i++ {
			aa := a + float64(i)*1.57
			s.add(fxPart{x: x + math.Cos(aa)*20, y: y + math.Sin(aa)*20, vx: math.Cos(aa+1.57) * 260, vy: math.Sin(aa+1.57) * 260, life: .25, sz: 30, sz1: 10, drag: 3, col: L.hot, col1: alpha(L.cool, 0), k: pSpark})
		}
	}
}

// ---- drawing ----

func (g *Game) drawParts(dst *ebiten.Image, sx, sy func(float64) float64) {
	s := &g.fxs
	if len(s.parts) == 0 {
		return
	}
	sw, sh := float64(dst.Bounds().Dx()), float64(dst.Bounds().Dy())
	// normal-blend pieces first (smoke, drops, shards, art), then the additive light on top
	for pass := 0; pass < 2; pass++ {
		for i := range s.parts {
			p := &s.parts[i]
			add := p.k == pGlow || p.k == pSpark || p.k == pStar || p.k == pRing || p.k == pFlare
			if add != (pass == 1) {
				continue
			}
			x, y := sx(p.x), sy(p.y)
			if x < -90 || y < -90 || x > sw+90 || y > sh+90 {
				continue
			}
			k := 1 - p.life/p.l0 // 0 → 1
			sz := p.sz + (p.sz1-p.sz)*k
			col := lerpC(p.col, p.col1, k)
			if sz <= .3 || col.A == 0 {
				continue
			}
			op := &ebiten.DrawImageOptions{}
			var im *ebiten.Image
			w, h := 1.0, 1.0
			rot := p.rot
			switch p.k {
			case pGlow, pFlare:
				im = glowImg
				w, h = sz*2, sz*2
			case pSmoke:
				im = softImg
				w, h = sz*2, sz*2
			case pDisc:
				im = discTex[0]
				w, h = sz*2, sz*2
			case pShard:
				im = texShard
				w, h = sz*1.4, sz*1.4
			case pStar:
				im = texStar
				w, h = sz*2, sz*2
			case pRing:
				im = texRing
				w, h = sz*2, sz*2
			case pSpark:
				im = texSpark
				sp := math.Hypot(p.vx, p.vy)
				if sp > 1 {
					rot = math.Atan2(p.vy, p.vx)
				}
				w, h = sz*(1+math.Min(2, sp/300)), math.Max(2.5, sz*.28)
			case pArt:
				im = p.art
				w, h = sz, sz
			}
			if im == nil {
				continue
			}
			b := im.Bounds()
			op.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
			op.GeoM.Scale(w/float64(b.Dx()), h/float64(b.Dy()))
			if rot != 0 {
				op.GeoM.Rotate(rot)
			}
			op.GeoM.Translate(x, y)
			op.ColorScale.ScaleWithColor(col)
			op.Filter = ebiten.FilterLinear
			if add {
				op.Blend = ebiten.BlendLighter
			}
			dst.DrawImage(im, op)
		}
	}
}

func (g *Game) drawScreenFlash(dst *ebiten.Image) {
	s := &g.fxs
	if s.flash <= .01 {
		return
	}
	b := dst.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(b.Dx()), float64(b.Dy()))
	op.ColorScale.ScaleWithColor(alpha(s.flashCol, s.flash))
	op.Blend = ebiten.BlendLighter
	dst.DrawImage(whiteSub, op)
}

// glowAdd: an additive light blob (the bloom on beams, rings, swipes and projectiles).
func glowAdd(dst *ebiten.Image, x, y, r float64, col color.NRGBA, a float64) {
	if r < .5 || a <= 0 {
		return
	}
	R := r * 1.15
	b := glowImg.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2*R/float64(b.Dx()), 2*R/float64(b.Dy()))
	op.GeoM.Translate(x-R, y-R)
	op.ColorScale.ScaleWithColor(alpha(col, math.Min(1, a)))
	op.Filter = ebiten.FilterLinear
	op.Blend = ebiten.BlendLighter
	dst.DrawImage(glowImg, op)
}
