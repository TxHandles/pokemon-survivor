package sim

import (
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// New starts a run as Pokémon id in the given biome (a type name).
func New(db *content.DB, id, biome string, seed int64) (*World, error) {
	return NewRun(db, id, biome, seed, 0)
}

// NewRun starts a run with ability 0 (starting) or 1 (hidden / legendary) and no meta upgrades.
func NewRun(db *content.DB, id, biome string, seed int64, ability int) (*World, error) {
	return NewWith(db, id, biome, seed, Config{Ability: ability, LI: -1})
}

func newWorld(db *content.DB, id, biome string, seed int64, cfg Config) (*World, error) {
	return NewCoop(db, []PlayerSpec{{ID: id, Name: "You", Kind: "local", Ability: cfg.Ability, Up: cfg.Up, Training: cfg.Training}}, biome, seed, cfg)
}

// NewCoop starts a run for one to four players sharing the screen / simulation.
func NewCoop(db *content.DB, players []PlayerSpec, biome string, seed int64, cfg Config) (*World, error) {
	if len(players) == 0 || len(players) > 4 {
		return nil, fmt.Errorf("need 1-4 players")
	}

	if cfg.Up == nil {
		cfg.Up = map[string]int{}
	}
	w := &World{DB: db, Biome: biome, Seed: seed, rng: rand.New(rand.NewSource(seed)),
		ViewW: 1280, ViewH: 720, MoveSlots: 2, HeldSlots: 2, nextBoss: 300, nextSwarm: 120, LeashFar: 500, LeashBias: .5, LeashSpread: 2.2, obs: map[ckey][]Obstacle{}, obsRaw: map[ckey][]Obstacle{},
		pools: map[ckey][]Pool{}, trT: TraderEvery, elT: 100, mbT: 150, candyT: 20, bioT: 6, Cfg: cfg, Run: makeRun(cfg), Chooser: -1}
	n := len(players)
	w.Run.HM *= coC.hp[n]
	w.Run.XM *= coC.xp[n]
	w.Run.DM *= coC.dm[n]
	w.XPS.NX, w.XPS.Lv = 5, 1
	for i, sp := range players {
		c, err := w.addPlayer(db, sp)
		if err != nil {
			return nil, err
		}
		a := float64(i) / float64(n) * 6.283
		if n > 1 {
			c.P.X, c.P.Y = math.Cos(a)*45, math.Sin(a)*45
		}
	}
	w.useP(0)
	// global shop upgrades come from player 0's profile (local co-op shares one save)
	w.RouteChoices = min(5, 2+int(w.up("rt")))
	w.MoveSlots = 2 + int(w.up("m3")) + int(w.up("m4"))
	w.HeldSlots = 2 + int(w.up("h3")) + int(w.up("h4"))
	w.Choices = 3 + int(w.up("ch"))
	w.chestT, w.shT = 30, 150
	li := cfg.LI
	if li < 0 || li > 2 {
		li = w.rng.Intn(3)
	}
	w.enterBiome(biome, li, true)
	if n > 1 {
		names := ""
		for _, c := range w.PL {
			if names != "" {
				names += " · "
			}
			names += c.Name
		}
		w.message("Co-op: " + names)
	}
	return w, nil
}

func (w *World) addMove(id string) *Slot {
	m := w.DB.Moves[id]
	s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: 1, Q: BuildQ(m), Owner: w.CUR}
	w.Slots = append(w.Slots, s)
	return s
}

// SlotOf is the current player's equipped slot for move id (nil if not equipped).
func (w *World) SlotOf(id string) *Slot { return w.slot(id) }

func (w *World) slot(id string) *Slot {
	for _, s := range w.Slots {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (w *World) tS() float64 {
	k := 1.0
	if w.Run.Stage > 1 && !w.Endless {
		k = .6
	}
	return math.Min(w.T, 900) + math.Max(0, w.T-900)*k
}
func (w *World) tsc() float64 { return (1 + w.tS()/60*.35) * w.ef() }
func (w *World) dmS() float64 { return (1 + w.tS()/600) * math.Sqrt(w.ef()) }

func (w *World) spawnDist() float64 { return hyp(w.ViewW, w.ViewH)/2 + 40 }

func (w *World) pickT(p float64) string {
	if w.rng.Float64() < p {
		return w.Biome
	}
	return content.Types[w.rng.Intn(18)]
}

func (w *World) enemySprite(t string, pool string) string {
	var l []string
	if pool == "boss" {
		for _, p := range w.DB.Pokemon {
			if has(p.Types, t) && has(p.EnemyPool, "boss") && !p.Legendary {
				l = append(l, p.ID)
			}
		}
	}
	if len(l) == 0 {
		l = w.DB.EnemyPool(t)
	}
	if len(l) == 0 {
		return t
	}
	return l[w.rng.Intn(len(l))]
}

// spawn adds a regular enemy on the spawn ring (web spawn()).
func (w *World) spawn(a float64, t string) {
	if a < 0 {
		a = w.rnd(0, 6.283)
	}
	if t == "" {
		t = w.pickT(.55)
	}
	d := w.spawnDist()
	big := w.rng.Float64() < .12
	sc := w.tsc()
	q := w.PL[w.rndAlive()].P
	e := &Enemy{X: q.X + math.Cos(a)*d, Y: q.Y + math.Sin(a)*d, Type: t, Big: big, R: 10, Seed: w.rng.Float64()}
	hp, sp, dm := 14.0, w.rnd(50, 85), 6.0
	if big { // slow, tanky bruisers
		e.R, hp, sp, dm = 16, 55, sp*.62, 12
	} else if w.rng.Float64() < .14 { // quick, fragile runners
		e.Swift = true
		e.R, hp, sp, dm = 8, 6, sp*1.6, 4
	}
	e.HP, e.MHP = hp*sc*w.Run.HM, hp*sc*w.Run.HM
	if w.P.Pgr > 0 {
		e.HP, e.MHP = e.HP*1.5, e.MHP*1.5
	}
	e.Sp = sp * w.spS() * w.Run.SM
	e.Dm = dm * w.dmS() * w.Run.DM * math.Min(1, .75+w.T/720)
	e.PokeID = w.enemySprite(t, "")
	w.maybeSpec(e)
	if e.Sp2 == "" {
		w.assignRole(e)
	}
	w.E = append(w.E, e)
}

func (w *World) spawnBoss() {
	w.nextBoss += 300
	w.bossesSpawned++
	n := w.bossesSpawned
	t := w.Biome
	a := w.rnd(0, 6.283)
	bd := Bosses[t]
	if bd == nil {
		bd = Bosses["normal"]
	}
	bh := 640.0
	if n > 1 {
		bh = 800 * float64(n)
	}
	bh *= bd.HP * w.ef() * w.Run.HM * w.Run.BH
	dm := 18 * w.dmS() * bd.Dm * w.Run.DM
	if n == 1 {
		dm *= .85
	}
	q := w.PL[w.rndAlive()].P
	e := &Enemy{X: q.X + math.Cos(a)*500, Y: q.Y + math.Sin(a)*500, Type: t, R: bd.R, HP: bh, MHP: bh, Sp: 60 * bd.Sp * w.spS() * w.Run.SM, Dm: dm, Boss: true, BD: bd, Seed: w.rng.Float64()}
	e.PokeID = w.enemySprite(t, "boss")
	w.E = append(w.E, e)
	name := e.PokeID
	if p := w.DB.ByID[e.PokeID]; p != nil {
		name = p.Name
	}
	w.sfx("boss")
	w.message(name + " (Boss) appears!")
}

func (w *World) swarm() {
	t := w.pickT(.7)
	n := int(math.Max(0, math.Min(math.Round((22+math.Floor(w.T/15))*coC.sw[min(4, w.n())]), math.Min(90, 720+w.coCap()-320-float64(len(w.E))))))
	sp := 6.283
	if w.T < 480 {
		sp = 4.7
	}
	a0 := w.rnd(0, 6.283)
	for i := 0; i < n; i++ {
		w.spawn(a0+float64(i)/float64(n)*sp, t)
	}
	w.message("A swarm of " + t + " Pokémon surrounds you!")
	_ = w.P
}

// Step advances the run by dt seconds.
func (w *World) Step(dt float64, in Input) {
	w.useP(0) // clients may have switched the viewed player; a tick always starts from player 0
	if w.State != StatePlay {
		w.stepRouteTravel(dt)
		return
	}
	w.T += dt
	if w.MsgT > 0 {
		w.MsgT -= dt
	}
	if len(w.PL) > 0 {
		w.PL[0].In = in
	}
	for i := range w.PL {
		if !w.alive(i) {
			continue
		}
		w.useP(i)
		c := w.PL[i]
		pin := c.In
		if c.Kind == "bot" && c.Bot != nil {
			pin = c.Bot.Input(dt)
		}
		w.stepPlayer(dt, pin)
		if w.State != StatePlay {
			return
		}
	}
	w.pcTick(dt)
	// delayed move effects
	for _, d := range w.DL {
		d.t -= dt
	}
	var due []*delay
	keep := w.DL[:0:0]
	for _, d := range w.DL {
		if d.t <= 0 {
			due = append(due, d)
		} else {
			keep = append(keep, d)
		}
	}
	w.DL = keep
	for _, d := range due {
		d.fn()
	}
	// spawning
	if w.spawnHold > 0 {
		w.spawnHold -= dt
	}
	w.spawnT -= dt
	cap := w.coCap()
	if w.Endless {
		cap += (w.T - w.ET) / 3
	}
	if w.spawnT <= 0 && w.spawnHold <= 0 && float64(len(w.E)) < math.Min(520+w.coCap()-320, cap) {
		ts := w.tS()
		x := ts - 90
		if ts < 300 {
			x = ts * .7
		}
		n := 1 + int((x+55)/110)
		for i := 0; i < n; i++ {
			w.spawn(-1, "")
		}
		w.spawnT = math.Max(.1, .55-w.T/1700) / w.Run.RT / w.coRt()
		if w.Endless {
			w.spawnT /= 1 + (w.T-w.ET)/180
		}
	}
	w.relocate(dt)
	if w.T >= w.nextBoss && (w.bossesSpawned < w.Run.Bosses || w.Endless) {
		w.spawnBoss()
	}
	w.stepRun(dt)
	if w.State != StatePlay {
		return
	}
	if w.T >= w.nextSwarm {
		w.nextSwarm += 120
		if w.spawnHold <= 0 {
			w.swarm()
		}
	}
	w.stepEnemies(dt)
	if w.State != StatePlay {
		return
	}
	w.stepEnemyAttacks(dt)
	if w.State != StatePlay {
		return
	}
	w.stepProjectiles(dt)
	w.stepFX(dt)
	w.stepZones(dt)
	w.stepGoMoves(dt)
	w.stepOrbits(dt)
	w.compact()
	w.stepGems(dt)
	w.stepPickups(dt)
	w.Shake = math.Max(0, w.Shake-dt)
	if w.XPS.XP >= w.XPS.NX && w.State == StatePlay {
		w.coopLevel()
	}
}

func (w *World) stepPlayer(dt float64, in Input) {
	P := w.P
	x, y := in.MoveX, in.MoveY
	if l := hyp(x, y); l > 1 {
		x, y = x/l, y/l
	}
	P.Moving = x != 0 || y != 0
	if P.Moving {
		P.Still = 0
		l := hyp(x, y)
		P.FX, P.FY = x/l, y/l
	} else {
		P.Still += dt
	}
	if P.Bat > 0 {
		P.Bat -= dt
	}
	if P.Bel > 0 {
		P.Bel -= dt
	}
	tm, hk := w.playerTerrain(dt)
	P.Hazard = hk
	if w.State != StatePlay {
		return
	}
	for _, t := range []*float64{&P.Pm, &P.Pbl, &P.Phs, &P.Pgr} {
		if *t > 0 {
			*t -= dt
		}
	}
	spm := P.Sp * (1 + .08*w.pe("boots") + b2f(w.fz("hover"), .08, 0) + b2f(w.rare("powder"), .1, 0) + w.abSpd() + b2f(P.SpdT > w.T, .2, 0)) * b2f(w.ab.kbst > 0, 1+w.PS.F("kspd"), 1) * b2f(P.Phs > 0, 1.25, 1) * tm * b2f(P.Fly != nil, 1.45, 1)
	if hk == "ice" {
		q := math.Min(1, dt*(1.6+(1-math.Min(1, w.hzK()))*14))
		P.VX += (x*spm - P.VX) * q
		P.VY += (y*spm - P.VY) * q
	} else {
		P.VX, P.VY = x*spm, y*spm
	}
	P.X += P.VX*dt + P.WX*dt
	P.Y += P.VY*dt + P.WY*dt
	if P.Dash != nil {
		w.dashStep(dt)
	}
	if P.Roll != nil {
		w.stepRoll(dt, x, y)
	}
	if P.Fly != nil {
		w.stepFly(dt)
	} else if !has(P.Types, "flying") && !has(P.Types, "ghost") {
		w.push(&P.X, &P.Y, 12)
	}
	if P.Prot > 0 {
		P.Prot -= dt
	}
	P.Inv = math.Max(0, P.Inv-dt)
	if v := w.pe("lifeorb"); v > 0 && P.HP > P.MHP*.5 && !w.fz("secondwind") {
		P.HP -= P.MHP * .0015 * v * dt
	}
	if !w.Run.NoHeal {
		P.HP = math.Min(P.MHP, P.HP+(.45*w.pe("left")*b2f(w.fz("lullaby"), 1.5, 1)+.3*w.up("reg")+w.abReg())*dt)
	}
	w.abTick(dt)
	// moves
	for _, s := range w.Slots {
		s.CD -= dt
		if ActiveKind(s.Kind) {
			if s.CD < 0 {
				s.CD = 0
			}
			continue
		}
		if s.Kind == "orbit" {
			continue
		}
		if s.CD <= 0 {
			w.fire(s)
			s.CD = w.cdOf(s) * w.cdM()
		}
	}
	w.stepActions(in)
}

func (w *World) dashStep(dt float64) {
	P := w.P
	D := P.Dash
	st := math.Min(dt, D.T)
	P.X -= P.VX * dt
	P.Y -= P.VY * dt
	ox, oy := P.X, P.Y
	P.X += D.VX * st
	P.Y += D.VY * st
	if !D.Phase {
		w.push(&P.X, &P.Y, 12)
	}
	D.T -= dt
	sx, sy := P.X-ox, P.Y-oy
	s2 := sx*sx + sy*sy
	if s2 == 0 {
		s2 = 1
	}
	sp := hyp(D.VX, D.VY)
	ux, uy := D.VX/sp, D.VY/sp
	w.addFX(&FX{Kind: "beam", X: ox, Y: oy, A: math.Atan2(D.VY, D.VX), Ln: math.Sqrt(s2) + 6, Wd: D.W * .75, Life: .22, Type: D.Type})
	prev := w.curSlot
	w.curSlot = D.Slot
	for _, e := range w.E {
		if e.Dead || D.Hit[e] {
			continue
		}
		k := ((e.X-ox)*sx + (e.Y-oy)*sy) / s2
		k = math.Max(0, math.Min(1, k))
		px, py := ox+sx*k, oy+sy*k
		if hyp(e.X-px, e.Y-py) > D.W+e.R {
			continue
		}
		D.Hit[e] = true
		w.hit(e, D.D, D.Type, 0, px, py)
		if !e.Dead {
			nx, ny := e.X-px, e.Y-py
			nl := hyp(nx, ny)
			if nl == 0 {
				nl = 1
			}
			kf := e.kbFactor()
			e.KX = clampK(e.KX + (nx/nl*260+ux*220)*kf)
			e.KY = clampK(e.KY + (ny/nl*260+uy*220)*kf)
		}
	}
	if D.T <= 0 {
		P.Dash = nil
		P.Inv = math.Max(P.Inv, .3)
		if D.Cap {
			w.addFX(&FX{Kind: "ring", X: P.X, Y: P.Y, R: 8, MR: 100, Type: D.Type, D: D.D * .6, Hit: map[*Enemy]bool{}, Life: .35, Fat: true, KB: 2, Slot: D.Slot})
		}
	}
	w.curSlot = prev
}

// relocate moves enemies that fell far behind back onto the spawn ring, biased ahead of the player (web leash).
func (w *World) relocate(dt float64) {
	w.relocT -= dt
	if w.relocT > 0 {
		return
	}
	w.relocT = .25
	sd := w.spawnDist()
	far := sd + w.LeashFar
	n := 0
	for _, e := range w.E {
		if e.Dead || e.Charm > 0 || e.guard != nil || e.cwu > 0 || e.chd > 0 {
			continue
		}
		Q := w.PL[w.nearP(e.X, e.Y)].P
		lim := far
		if e.Boss {
			lim += 400
		}
		dx, dy := e.X-Q.X, e.Y-Q.Y
		if dx*dx+dy*dy < lim*lim {
			continue
		}
		a := w.rnd(0, 6.283)
		if w.n() == 1 && Q.Moving && w.rng.Float64() < w.LeashBias {
			a = math.Atan2(Q.FY, Q.FX) + w.rnd(-w.LeashSpread, w.LeashSpread)
		}
		r := sd + w.rnd(0, 70)
		e.X, e.Y = Q.X+math.Cos(a)*r, Q.Y+math.Sin(a)*r
		if n++; n >= 30 {
			break
		}
	}
}

func (w *World) stepEnemies(dt float64) {
	helm := make([]float64, len(w.PL)) // Rocky Helmet level per player (looked up once per tick)
	for i := range w.PL {
		w.with(i, func() { helm[i] = w.pe("helmet") * b2f(w.fz("barbs"), 2, 1) })
	}
	for _, e := range w.E {
		if e.Dead {
			continue
		}
		if e.Toss != nil { // held / flying: no AI and no contact damage
			w.stepToss(e, dt)
			continue
		}
		w.useP(w.nearP(e.X, e.Y))
		P := w.P
		// heading as a unit vector (cheaper than an angle); custom headings fall back to angles
		ux, uy := P.X-e.X, P.Y-e.Y
		if l := phyp(ux, uy); l > 0 {
			ux, uy = ux/l, uy/l
		} else {
			ux, uy = 1, 0
		}
		if e.Fear > 0 {
			ux, uy = -ux, -uy
		}
		a, angled := 0.0, false
		if e.Charm > 0 {
			var tg *Enemy
			bd := 1e18
			for _, o := range w.E {
				if o == e || o.Dead || o.Charm > 0 {
					continue
				}
				if d := (o.X-e.X)*(o.X-e.X) + (o.Y-e.Y)*(o.Y-e.Y); d < bd {
					bd, tg = d, o
				}
			}
			if tg != nil {
				a, angled = math.Atan2(tg.Y-e.Y, tg.X-e.X), true
				if bd < (e.R+tg.R+4)*(e.R+tg.R+4) && e.cx <= w.T {
					e.cx = w.T + .4
					w.hitRaw(tg, math.Round(e.Dm*3), "charm")
				}
			}
		}
		wasFrozen := e.Frz > 0
		for _, p := range []*float64{&e.Slow, &e.Stun, &e.Frz, &e.Thaw, &e.Fear, &e.Charm, &e.Root, &e.Conf} {
			if *p > 0 {
				*p -= dt
			}
		}
		if wasFrozen && !(e.Frz > 0) {
			e.Thaw = 2
		}
		if e.Burn > 0 {
			e.Burn -= dt
			e.burnK -= dt
			if e.burnK <= 0 {
				e.burnK = .5
				w.hitRaw(e, e.BurnD*.5*(1+.15*w.pe("heat")), "burn")
			}
		}
		if e.Slp > 0 {
			e.Slp -= dt
		}
		if e.LS > 0 {
			w.stepSeed(e, dt)
		}
		if e.Psn > 0 {
			e.Psn -= dt
			e.psnK -= dt
			if e.psnK <= 0 {
				e.psnK = .5
				w.hitRaw(e, e.PsnD*.5*(1+.15*w.pe("heat")), "poison")
			}
		}
		if e.Dead {
			continue
		}
		if g := w.Gust; g != nil && g.T > 0 {
			wk := w.hzKe(e.Type) * b2f(e.Boss, .3, 1)
			e.X += g.VX * wk * dt
			e.Y += g.VY * wk * dt
		}
		px, py := e.X, e.Y
		mv := 0.0
		lureA, lured := 0.0, false
		if len(w.Decoys) > 0 && !(e.Fear > 0) && !e.Boss {
			lureA, lured = w.lure(e, P.X, P.Y)
		}
		if !(e.Stun > 0 || e.Frz > 0 || e.Root > 0 || e.Slp > 0) {
			w.curSrc = w.esrc(e, false)
			sm := w.ai(e, dt) * b2f(P.Phs > 0, 1.3, 1)
			w.curSrc = nil
			if e.Dead || w.State != StatePlay {
				continue
			}
			if e.aslT > w.T && !(e.Slow > 0) {
				sm *= 1 - e.aslS
			}
			if e.Slow > 0 {
				sm = .5
			}
			if e.Thaw > 0 {
				sm *= .5
			}
			aa := a
			if e.aoSet {
				aa, angled = e.ao, true
			}
			if lured { // a Substitute draws it in
				aa, angled = lureA, true
			}
			if e.dtr > 0 {
				e.dtr -= dt
				if !angled {
					aa, angled = math.Atan2(uy, ux), true
				}
				aa += e.ds * 1.3
			}
			hx, hy := ux, uy
			if angled {
				hx, hy = math.Cos(aa), math.Sin(aa)
			}
			mv = e.Sp * sm * dt
			e.X += hx * mv
			e.Y += hy * mv
			if hx < -.05 {
				e.FaceL = true
			} else if hx > .05 {
				e.FaceL = false
			}
		}
		e.Flash = math.Max(0, e.Flash-dt)
		if e.KX != 0 || e.KY != 0 {
			e.X += e.KX * dt
			e.Y += e.KY * dt
			dc := pexp(-6 * dt)
			e.KX *= dc
			e.KY *= dc
			if math.Abs(e.KX)+math.Abs(e.KY) < 2 {
				e.KX, e.KY = 0, 0
			}
		}
		if !e.Boss && !e.Ug && !has([]string{"flying", "ghost"}, e.Type) {
			w.push(&e.X, &e.Y, e.R)
		}
		if g := e.guard; g != nil { // Pokémon Center guard stays near its post
			gx, gy := g.X-e.X, g.Y-e.Y
			if gd := hyp(gx, gy); gd > 230 {
				e.X += gx / gd * e.Sp * 1.3 * dt
				e.Y += gy / gd * e.Sp * 1.3 * dt
			}
		}
		// unstick from obstacles
		if mv > 0 && e.KX == 0 && e.KY == 0 && dist(e.X, e.Y, P.X, P.Y) > e.R+40 {
			if hyp(e.X-px, e.Y-py) < mv*.3 {
				e.stk += dt
				if e.stk > 1 && !(e.dtr > 0) {
					e.dtr = 1 + w.rng.Float64()*.6
					if e.ds == 0 {
						e.ds = 1
						if w.rng.Float64() < .5 {
							e.ds = -1
						}
					}
					e.ds = -e.ds
					e.stk = 0
				}
			} else {
				e.stk = math.Max(0, e.stk-dt*2)
			}
		}
		if v := helm[w.CUR]; v > 0 && e.th <= w.T && dist(e.X, e.Y, P.X, P.Y) < e.R+11 {
			e.th = w.T + .5
			prev := w.curSlot
			w.curSlot = nil
			w.hit(e, math.Round(4*v*(1+w.T/300)), P.Types[0], 0, 0, 0)
			w.curSlot = prev
		}
		if P.Inv <= 0 && !e.Dead && !e.Ug && !(e.Stun > 0 || e.Frz > 0 || e.Fear > 0 || e.Charm > 0 || e.Slp > 0) && dist(e.X, e.Y, P.X, P.Y) < e.R+12 {
			if !w.dodge() {
				w.hurtS(e.Dm, hurtSrc{t: e.Type, contact: true, elite: e.Elite, src: w.esrc(e, true)})
			}
			if th := w.PS.F("thorn"); th > 0 && !e.Dead {
				prev := w.curSlot
				w.curSlot = nil
				w.hit(e, th*(6+w.T/30), P.Types[0], 0, 0, 0)
				w.curSlot = prev
			}
			w.abContact(e)
			if e.Vamp {
				e.HP = math.Min(e.MHP, e.HP+e.Dm*2)
			}
			P.Inv = .5 + w.PS.F("inv")
			w.Shake = .12
			if w.State == StateDead {
				return
			}
		}
	}
}

func (w *World) stepProjectiles(dt float64) {
	claim := map[*Enemy]int{}
	for _, p := range w.PR {
		if p.Sk && p.Tg != nil && !p.Tg.Dead {
			claim[p.Tg]++
		}
	}
	for _, p := range w.PR {
		if p.L <= 0 {
			continue
		}
		if p.Slot != nil {
			w.useP(p.Slot.Owner)
		}
		if p.Sk {
			tg := p.Tg
			if tg != nil && (tg.Dead || inList(p.Hit, tg)) {
				tg = nil
			}
			if tg == nil {
				if p.Tg != nil && claim[p.Tg] > 0 {
					claim[p.Tg]--
				}
				bd := 1e18
				for _, e := range w.E {
					if e.Dead || inList(p.Hit, e) {
						continue
					}
					dd := (e.X-p.X)*(e.X-p.X) + (e.Y-p.Y)*(e.Y-p.Y)
					if dd > 160000 {
						continue
					}
					if sc := dd * (1 + 3*float64(claim[e])); sc < bd {
						bd, tg = sc, e
					}
				}
				p.Tg = tg
				if tg != nil {
					claim[tg]++
				}
			}
			if tg != nil {
				sp := hyp(p.VX, p.VY)
				an := math.Atan2(p.VY, p.VX)
				da := math.Atan2(tg.Y-p.Y, tg.X-p.X) - an
				da = math.Atan2(math.Sin(da), math.Cos(da))
				tr := p.Tr
				if tr == 0 {
					tr = 6
				}
				tn := math.Max(-tr*dt, math.Min(tr*dt, da))
				p.VX, p.VY = math.Cos(an+tn)*sp, math.Sin(an+tn)*sp
			}
		}
		if b := p.Bm; b != nil {
			b.t += dt
			if !b.back {
				if b.t >= b.out {
					b.back = true
					p.Hit = nil
				}
			} else {
				dx, dy := w.P.X-p.X, w.P.Y-p.Y
				dd := hyp(dx, dy)
				if dd == 0 {
					dd = 1
				}
				p.VX, p.VY = dx/dd*b.sp*1.15, dy/dd*b.sp*1.15
				if dd < 16 {
					p.L = 0
				}
			}
		}
		p.X += p.VX * dt
		p.Y += p.VY * dt
		p.L -= dt
		for _, e := range w.E {
			if e.Dead || p.L <= 0 || inList(p.Hit, e) {
				continue
			}
			if dist(p.X, p.Y, e.X, e.Y) < e.R+p.R {
				prev := w.curSlot
				w.curSlot = p.Slot
				w.hit(e, p.D, p.Type, 1, w.P.X, w.P.Y)
				if p.Ls > 0 {
					w.healCapped(lastD * p.Ls)
				}
				p.Hit = append(p.Hit, e)
				if p.Ex > 0 {
					f := &FX{Kind: "ring", X: e.X, Y: e.Y, R: 4, MR: 44 * p.Ex * (1 + .2*w.pe("wide")), Type: p.Type, D: p.D * .4, Hit: map[*Enemy]bool{e: true}, Life: .25, Fat: true, KB: 1, Slot: p.Slot}
					w.addFX(f)
				}
				w.curSlot = prev
				if p.P--; p.P <= 0 {
					p.L = 0
				}
			}
		}
	}
}

func (w *World) stepFX(dt float64) {
	for i := 0; i < len(w.FX); i++ { // rings can add FX while iterating
		f := w.FX[i]
		f.Life -= dt
		switch f.Kind {
		case "ring":
			if f.Slot != nil {
				w.useP(f.Slot.Owner)
			}
			f.R += (f.MR - f.R) * math.Min(1, dt*8)
			if !f.Fat {
				f.X, f.Y = w.P.X, w.P.Y
			}
			prev := w.curSlot
			w.curSlot = f.Slot
			for _, e := range w.E {
				if e.Dead || f.Hit[e] {
					continue
				}
				d := dist(f.X, f.Y, e.X, e.Y)
				extra := 0.0
				if f.Fat {
					extra = f.R
				}
				if math.Abs(d-f.R) < e.R+f.TW+extra && d < f.R+e.R {
					f.Hit[e] = true
					if f.Sk != "" {
						w.applyWave(e, f)
						if f.Dm > 0 {
							w.hit(e, f.Dm, f.Type, 0, 0, 0)
						}
					} else {
						kb := int(f.KB)
						if kb == 0 {
							kb = 2
						}
						w.hit(e, f.D, f.Type, kb, f.X, f.Y)
					}
				}
			}
			w.curSlot = prev
		case "txt":
			f.Y -= 30 * dt
		}
	}
}

func (w *World) applyWave(e *Enemy, f *FX) {
	if f.Sk == "su" {
		if e.Boss {
			e.Slow = math.Max(e.Slow, f.Du)
		} else {
			e.Stun = math.Max(e.Stun, f.Du)
		}
	} else {
		e.Slow = math.Max(e.Slow, f.Du*b2f(e.Boss, .8, 1))
		if f.Rt && !e.Boss {
			e.Root = math.Max(e.Root, .8)
		}
	}
}

var slowT = map[string]bool{"water": true, "ground": true, "bug": true, "ice": true}

func (w *World) stepZones(dt float64) {
	for _, z := range w.ZN {
		if z.Slot != nil {
			w.useP(z.Slot.Owner)
		}
		z.L -= dt
		z.Tk -= dt
		prev := w.curSlot
		w.curSlot = z.Slot
		if z.Mine {
			z.Age += dt
			if z.Age >= z.Arm && z.L > 0 {
				for _, e := range w.E {
					if e.Dead || dist(z.X, z.Y, e.X, e.Y) > z.R+e.R+4 {
						continue
					}
					z.L = 0
					w.addFX(&FX{Kind: "puff", X: z.X, Y: z.Y, R: z.Br, Life: .3, Type: z.Type})
					w.Shake = math.Max(w.Shake, .08)
					for _, o := range w.E {
						if !o.Dead && hyp(o.X-z.X, o.Y-z.Y) < z.Br+o.R {
							w.hit(o, z.Md, z.Type, 2, z.X, z.Y)
						}
					}
					break
				}
			}
			w.curSlot = prev
			continue
		}
		if z.Pull > 0 {
			for _, e := range w.E {
				if e.Dead || e.Boss {
					continue
				}
				dd := dist(z.X, z.Y, e.X, e.Y)
				if dd < z.R*z.Pr && dd > 3 {
					m := math.Min(dd, z.Pull*dt)
					e.X += (z.X - e.X) / dd * m
					e.Y += (z.Y - e.Y) / dd * m
				}
			}
		}
		if z.Mud {
			for _, e := range w.E {
				if !e.Dead && dist(z.X, z.Y, e.X, e.Y) < z.R+e.R {
					e.Slow = math.Max(e.Slow, .35)
				}
			}
		}
		if z.Tk <= 0 {
			z.Tk = .5 * (1 - .07*w.pe("heat")) * b2f(z.Cap, .5, 1)
			if z.Mud {
				z.Tk = .4
			}
			w.noProc = z.Mud
			for _, e := range w.E {
				if !e.Dead && dist(z.X, z.Y, e.X, e.Y) < z.R+e.R && (z.Mud || e.zt <= w.T) {
					if !z.Mud { // mud ticks on its own clock, so it never blocks the move's own zones
						e.zt = w.T + .4*b2f(z.Cap, .5, 1)
					}
					w.hit(e, z.D, z.Type, 0, 0, 0)
					if slowT[z.Type] || z.Ev {
						e.Slow = .6
					}
				}
			}
			w.noProc = false
		}
		w.curSlot = prev
	}
}

func (w *World) stepOrbits(dt float64) {
	c0 := w.CUR
	for i := range w.PL {
		w.useP(i)
		if w.alive(i) {
			w.stepOrbits1(dt)
		} else {
			w.Orbs = nil
		}
	}
	w.useP(c0)
}

func (w *World) stepOrbits1(dt float64) {
	w.Orbs = w.Orbs[:0]
	oi := 0
	for _, s := range w.Slots {
		if s.Kind != "orbit" {
			continue
		}
		q := s.Q
		sl := 1 + float64(s.Lv-1)*.45
		ev := s.Evo
		n := int(q.F("n0", 1)) + int(math.Round(sl)) + w.gvBase() + b2i(ev) + b2i(ev && s.Lv >= 20) + s.BonusCount()
		sz := (1 + .2*w.pe("wide")) * b2f(ev, 1.3, 1) * (1 + .05*float64(s.Lv-1)) * s.BonusSizeMul()
		dir := 1.0
		if oi%2 == 1 {
			dir = -1
		}
		if s.orbA == 0 {
			s.orbA = float64(oi) * 1.1
		}
		s.orbA += dt * (2.4 + .2*sl) * (1 + .12*w.pe("metro")) * dir * s.BonusSpdMul()
		prevS := w.curSlot
		w.curSlot = nil // rgm() without another move's range bonus
		rr := (q.F("rr", 55)+q.F("rrp", 5)*sl)*w.rgm()*s.BonusAreaMul() + float64(oi)*26
		w.curSlot = prevS
		dmg := (q.F("d", 7) + q.F("dp", 3)*dsl(s.Lv)) * b2f(ev, 1.35*(1+.08*w.pe(s.Move.HeldItem)), 1) * s.BonusDamageMul()
		prev := w.curSlot
		w.curSlot = s
		for i := 0; i < n; i++ {
			a := s.orbA + float64(i)*6.283/float64(n)
			o := Orb{X: w.P.X + math.Cos(a)*rr, Y: w.P.Y + math.Sin(a)*rr, R: 9 * sz, Type: s.Type, Mv: s.ID}
			w.Orbs = append(w.Orbs, o)
			for _, e := range w.E {
				if !e.Dead && e.orbT[oi%8] <= w.T && dist(o.X, o.Y, e.X, e.Y) < e.R+o.R {
					e.orbT[oi%8] = w.T + b2f(ev, .25, .4)
					w.hit(e, dmg, o.Type, 2, w.P.X, w.P.Y)
				}
			}
		}
		w.curSlot = prev
		oi++
	}
}

func (w *World) compact() {
	e := w.E[:0]
	for _, x := range w.E {
		if !x.Dead {
			e = append(e, x)
		}
	}
	for i := len(e); i < len(w.E); i++ {
		w.E[i] = nil
	}
	w.E = e
	pr := w.PR[:0]
	for _, x := range w.PR {
		if x.L > 0 {
			pr = append(pr, x)
		}
	}
	w.PR = pr
	fx := w.FX[:0]
	for _, x := range w.FX {
		if x.Life > 0 {
			fx = append(fx, x)
		}
	}
	w.FX = fx
	zn := w.ZN[:0]
	for _, x := range w.ZN {
		if x.L > 0 {
			zn = append(zn, x)
		}
	}
	w.ZN = zn
}

func (w *World) stepGems(dt float64) {
	if w.magnetAll {
		w.magnetAll = false
		for _, g := range w.Gems {
			g.Mag = true
		}
	}
	mags := make([]float64, len(w.PL)) // pickup radius per player (looked up once per tick)
	for i := range w.PL {
		w.with(i, func() {
			mags[i] = 90 * (1 + .12*w.up("mag") + .25*w.pe("magnet") + w.PS.F("mag")) * b2f(w.fz("hover"), 1.5, 1)
		})
	}
	if len(w.Gems) > 260 && int(w.T*60)%120 == 0 {
		w.mergeGems()
	}
	g2 := w.Gems[:0]
	for _, g := range w.Gems {
		pi := w.nearP(g.X, g.Y)
		P := w.PL[pi].P
		mag := mags[pi]
		d := dist(g.X, g.Y, P.X, P.Y)
		if g.Mag || d < mag {
			sp := 260.0
			if g.Mag {
				if g.ms == 0 {
					g.ms = 320
				}
				g.ms = math.Min(1800, g.ms+dt*1100)
				sp = g.ms
			}
			if d < sp*dt {
				g.X, g.Y = P.X, P.Y
			} else {
				g.X += (P.X - g.X) / math.Max(d, 1) * sp * dt
				g.Y += (P.Y - g.Y) / math.Max(d, 1) * sp * dt
			}
		}
		if d < 14 {
			w.useP(pi)
			switch {
			case g.Tk:
				w.sfx("ticket")
			case g.C > 0:
				w.sfx("coin")
			default:
				w.sfx("xp")
			}
			w.XPS.XP += g.V * w.Run.XM * (1 + .05*w.up("xpg") + .1*w.pe("exp") + w.PS.F("xp") + b2f(w.fz("study"), .15, 0) + b2f(w.rare("luckyegg"), .3, 0))
			P.XP = w.XPS.XP
			w.Coins += g.C * coinK * (1 + w.PS.F("coin")) * b2f(w.rare("amulet"), 1.5, 1)
			if g.Tk {
				w.Tickets++
				if g.Ed {
					w.Stats.EliteTk++
				}
				w.addFX(&FX{Kind: "txt", X: P.X, Y: P.Y - 30, Life: 1.5, Text: "+1 ticket!", Col: "gold"})
			}
			continue
		}
		g2 = append(g2, g)
	}
	w.Gems = g2
	// merge far-away gems when there are too many (keeps the count bounded)
	if P := w.P; len(w.Gems) > 400 {
		sort.Slice(w.Gems, func(i, j int) bool {
			return dist(w.Gems[i].X, w.Gems[i].Y, P.X, P.Y) < dist(w.Gems[j].X, w.Gems[j].Y, P.X, P.Y)
		})
		tail := w.Gems[300:]
		v, m := 0.0, false
		for _, g := range tail {
			v += g.V
			m = m || g.Mag
		}
		w.Gems = append(w.Gems[:300], &Gem{X: tail[0].X, Y: tail[0].Y, V: v, Mag: m})
	}
}

// ---- level-up ----

func (w *World) levelUp() { w.coopLevel() }

func (w *World) heldCount() int { // slots in use (a fused item takes one)
	n := 0
	for _, h := range w.HeldOrder {
		if w.Held[h] > 0 {
			n++
		}
	}
	return n
}

// options lists every possible level-up card (web opts_()).
func (w *World) options() []Option {
	var o []Option
	seen := map[string]bool{}
	ids := append([]string{}, w.DB.LearnList(w.P.ID)...)
	for _, s := range w.Slots {
		ids = append(ids, s.ID)
	}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		m := w.DB.Moves[id]
		if m == nil || !Implemented[m.Shape] {
			continue
		}
		s := w.slot(id)
		switch {
		case s == nil:
			if len(w.Slots) < w.MoveSlots {
				o = append(o, Option{Move: true, ID: id, New: true})
			}
		case s.Lv < 10 || (s.Evo && s.Lv < 20):
			o = append(o, Option{Move: true, ID: id, Level: s.Lv})
		case s.Lv == 10 && !s.Evo && w.Held[m.HeldItem] > 0:
			o = append(o, Option{Move: true, ID: id, Evo: true, Level: 10})
		}
	}
	hc := w.heldCount()
	for _, it := range Items {
		if w.fusedInto(it.ID) != "" { // levels with its fusion
			continue
		}
		if v := w.Held[it.ID]; (v > 0 && v < ItemOverMax) || (v == 0 && hc < w.HeldSlots) {
			o = append(o, Option{ID: it.ID, New: v == 0, Level: v})
		}
	}
	for _, h := range w.HeldOrder { // fused items overcharge too
		if v := w.Held[h]; IsFused(h) && v > 0 && v < ItemOverMax {
			o = append(o, Option{ID: h, Level: v})
		}
	}
	o = append(o, w.fusionOptions()...)
	if hc >= w.HeldSlots { // offer swapping an unused held item for one a Lv 10 move needs to evolve
		feeds := func(h string) bool {
			for _, s := range w.Slots {
				if s.Move.HeldItem == h {
					return true
				}
			}
			return false
		}
		done := map[string]bool{}
		for _, s := range w.Slots {
			sy := s.Move.HeldItem
			if s.Lv < 10 || s.Evo || w.Held[sy] > 0 || done[sy] || ItemByID[sy] == nil {
				continue
			}
			var cand string
			for _, h := range w.HeldOrder {
				if w.Held[h] > 0 && !feeds(h) && (cand == "" || w.Held[h] < w.Held[cand]) {
					cand = h
				}
			}
			if cand != "" {
				done[sy] = true
				o = append(o, Option{ID: sy, New: true, Swap: cand, SwapAsk: true})
			}
		}
	}
	return o
}

func (w *World) genOpts() []Option {
	all := w.options()
	o := all[:0]
	for _, x := range all {
		if !w.Ban[banKey(x)] {
			o = append(o, x)
		}
	}
	w.rng.Shuffle(len(o), func(i, j int) { o[i], o[j] = o[j], o[i] })
	need := map[string]bool{}
	for _, s := range w.Slots {
		if s.Lv >= 10 && !s.Evo && w.Held[s.Move.HeldItem] == 0 {
			need[s.Move.HeldItem] = true
		}
	}
	pr := func(x Option) int {
		if x.Evo {
			return 2
		}
		if !x.Move && (need[x.ID] || x.Swap != "" || x.Fuse) {
			return 1
		}
		return 0
	}
	sort.SliceStable(o, func(i, j int) bool { return pr(o[i]) > pr(o[j]) })
	if len(o) > w.Choices {
		o = o[:w.Choices]
	}
	for i := range o { // move level-ups roll their bonus now, so the card can show it
		if o[i].Move && !o[i].New && !o[i].Evo {
			o[i].Bonus = w.rollBonus(w.DB.Moves[o[i].ID].Shape)
		}
	}
	return o
}

// Choose applies level-up card i for the first player choosing (solo, bots, same-screen co-op).
func (w *World) Choose(i int) { w.ChooseFor(w.Chooser, i) }

// ChooseFor applies level-up card i for player p, then serves their next prompt.
func (w *World) ChooseFor(p, i int) {
	if w.PendingKind(p) != "lv" {
		return
	}
	w.useP(p)
	if i < 0 || i >= len(w.Opts) {
		return
	}
	if o := w.Opts[i]; o.SwapAsk { // ask which held item to give up
		var l []Option
		for _, h := range w.HeldOrder {
			if w.Held[h] > 0 {
				l = append(l, Option{ID: o.ID, New: true, Swap: h})
			}
		}
		if len(l) > 0 {
			w.Opts, w.SwapPick = l, true
			return
		}
	}
	w.SwapPick = false
	w.applyOpt(w.Opts[i])
	c := w.PL[p]
	if len(c.Queue) > 0 {
		c.Queue = c.Queue[1:]
	}
	w.Opts = nil
	w.prompt()
}

func (w *World) applyOpt(o Option) {
	switch {
	case o.Evo:
		w.slot(o.ID).Evo = true
		w.Evolved[o.ID] = true
		w.sfx("evolve")
		w.message("EVOLVED! " + w.DB.Moves[o.ID].Name)
	case o.Move:
		if s := w.slot(o.ID); s != nil {
			s.Lv++
			b := o.Bonus
			if b == "" { // free levels (boss chest) roll here
				b = w.rollBonus(s.Kind)
			}
			if s.Bon == nil {
				s.Bon = map[string]int{}
			}
			s.Bon[b]++
			if s.Lv >= 20 {
				w.Capstone[o.ID] = true
			}
		} else {
			w.addMove(o.ID)
			w.Learned[o.ID] = true
		}
	case o.Fuse:
		w.fuse(o.ID)
	default:
		if o.Swap != "" {
			w.dropHeld(o.Swap)
		}
		if w.Held[o.ID] == 0 {
			w.HeldOrder = append(w.HeldOrder, o.ID)
		}
		w.levelHeld(o.ID)
	}
}

// BotPick scores the current options like the web bot (botScore) and returns the best index.
func (w *World) BotPick() int { return w.botPickFrom(w.Opts) }

func (w *World) botPickFrom(opts []Option) int {
	need := map[string]bool{}
	synWant := map[string]bool{}
	for _, s := range w.Slots {
		if s.Lv >= 10 && !s.Evo && w.Held[s.Move.HeldItem] == 0 {
			need[s.Move.HeldItem] = true
		}
		if !s.Evo {
			synWant[s.Move.HeldItem] = true
		}
	}
	missing := 0
	for k := range synWant {
		if w.Held[k] == 0 {
			missing++
		}
	}
	held, mc := w.heldCount(), len(w.Slots)
	pri := map[string]float64{"band": 65, "claw": 58, "lens": 55, "left": 45, "metal": 40, "boots": 42, "magnet": 30, "exp": 35, "glove": 50, "focus": 50, "metro": 44, "wide": 30, "eject": 25, "soothe": 35, "belt": 48, "helmet": 40, "choice": 55, "lifeorb": 52, "sash": 42, "muscle": 50, "wise": 55, "razor": 48}
	score := func(o Option) float64 {
		if o.Evo {
			return 100
		}
		if o.Move {
			if !o.New {
				return 80 - b2f(o.Level >= 10, 25, 0) - b2f(o.Level >= 6, float64(o.Level-5)*2, 0)
			}
			if mc < w.MoveSlots {
				return b2f(mc < 2, 66, 50)
			}
			return 0
		}
		if o.Swap != "" {
			return 93
		}
		if o.Fuse {
			return 90
		}
		if o.Level >= ItemMax { // overcharge: nice to have, below new moves and levels
			return 22
		}
		if w.Held[o.ID] == 0 && need[o.ID] {
			return 95
		}
		var sm []*Slot
		for _, s := range w.Slots {
			if s.Move.HeldItem == o.ID {
				sm = append(sm, s)
			}
		}
		if len(sm) > 0 {
			if w.Held[o.ID] >= 5 {
				return 45
			}
			if w.Held[o.ID] > 0 {
				return 76
			}
			for _, s := range sm {
				if !s.Evo {
					return 84
				}
			}
			return 70
		}
		if w.Held[o.ID] == 0 && w.HeldSlots-held <= missing {
			return 5
		}
		p, ok := pri[o.ID]
		if !ok {
			p = 30
		}
		return p + b2f(w.Held[o.ID] > 0, 3, 0)
	}
	best, bs := 0, -1e9
	for i, o := range opts {
		if s := score(o); s > bs {
			best, bs = i, s
		}
	}
	return best
}

// SlotsView exposes equipped moves for the HUD.
func (w *World) SlotsView() []*Slot { return w.Slots }

// DashReady reports the main action button move's charge (1 = ready), or -1 with no button move.
func (w *World) DashReady() float64 { return w.ButtonReady(0) }

// ButtonReady is action button btn's charge (0 main, 1 second; 1 = ready), or -1 when nothing is on it.
func (w *World) ButtonReady(btn int) float64 {
	best := -1.0
	for _, s := range w.ActiveSlots() {
		if w.ActiveButton(s) == btn {
			best = math.Max(best, w.ActiveReady(s))
		}
	}
	return best
}

// coinK scales coin pickups (a 20-minute run was paying about twice what it should).
const coinK = .5

// mergeGems folds plain XP gems lying close together into one (same total XP), so long runs don't
// carry hundreds of them.
func (w *World) mergeGems() {
	cell := map[ckey]*Gem{}
	out := w.Gems[:0]
	for _, g := range w.Gems {
		if g.C > 0 || g.Tk || g.Mag || g.ms > 0 {
			out = append(out, g)
			continue
		}
		k := ck(int(math.Floor(g.X/48)), int(math.Floor(g.Y/48)))
		if h := cell[k]; h != nil {
			h.V += g.V
			continue
		}
		cell[k] = g
		out = append(out, g)
	}
	for i := len(out); i < len(w.Gems); i++ {
		w.Gems[i] = nil
	}
	w.Gems = out
}
