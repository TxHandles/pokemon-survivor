package sim

import (
	"math"
	"testing"
)

func TestActionButtons(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "normal", "normal", 3)
	w.Slots = nil
	d := w.addMove("normal_quickattack")
	f := w.addMove("flying_fly")
	if d.Kind != "dash" || f.Kind != "fly" {
		t.Skipf("kinds %s %s", d.Kind, f.Kind)
	}
	if w.ActiveButton(d) != 0 || w.ActiveButton(f) != 1 {
		t.Fatalf("buttons %d %d", w.ActiveButton(d), w.ActiveButton(f))
	}
	w.Step(1.0/60, Input{Act2: true})
	if w.P.Fly == nil || w.P.Dash != nil {
		t.Fatalf("second button should take off (fly %v dash %v)", w.P.Fly != nil, w.P.Dash != nil)
	}
	for i := 0; i < 120 && w.P.Fly != nil; i++ {
		w.Step(1.0/60, Input{MoveX: 1})
	}
	if w.P.Fly != nil {
		t.Fatal("never landed")
	}
	w.Step(1.0/60, Input{Dash: true})
	if w.P.Dash == nil {
		t.Fatal("main button should dash")
	}
}

func TestRolloutEnds(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "normal", "normal", 4)
	w.Slots = nil
	w.addMove("rock_rollout")
	w.E = nil
	w.Step(1.0/60, Input{Dash: true})
	if w.P.Roll == nil {
		t.Fatal("no roll")
	}
	n := 0
	for ; n < 600 && w.P.Roll != nil; n++ {
		w.E = nil
		w.Step(1.0/60, Input{})
	}
	if w.P.Roll != nil || n > 240 {
		t.Fatalf("roll lasted %d ticks", n)
	}
}

func TestSleepAndSeed(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "normal", "normal", 5)
	P := w.P
	e := &Enemy{X: P.X + 60, Y: P.Y, R: 10, HP: 1e6, MHP: 1e6, Type: "normal", Sp: 50, Dm: 5}
	o := &Enemy{X: P.X + 90, Y: P.Y, R: 10, HP: 1e6, MHP: 1e6, Type: "normal", Sp: 50, Dm: 5}
	w.E = []*Enemy{e, o}
	m := db.Moves["normal_sing"]
	w.fire(&Slot{ID: m.ID, Move: m, Kind: m.Shape, Type: m.Type, Lv: 1, Q: BuildQ(m)})
	if !(e.Slp > 0) {
		t.Fatal("not asleep")
	}
	x := e.X
	w.stepEnemies(.5)
	if e.X != x {
		t.Error("sleeper moved")
	}
	e.HP = 3
	m = db.Moves["grass_leechseed"]
	w.fire(&Slot{ID: m.ID, Move: m, Kind: m.Shape, Type: m.Type, Lv: 1, Q: BuildQ(m)})
	for i := 0; i < 120; i++ {
		w.Step(1.0/60, Input{})
	}
	if !e.Dead || !(o.LS > 0) {
		t.Errorf("seed: host dead %v, jumped %v", e.Dead, o.LS > 0)
	}
}

// Melee attacks grow with range items and Range bonuses.
func TestMeleeRange(t *testing.T) {
	db := loadDB(t)
	for _, id := range []string{"fighting_lowkick", "fighting_karatechop", "fighting_rollingkick", "normal_scratch"} {
		m := db.Moves[id]
		if m == nil {
			continue
		}
		reach := func(focus, area int) float64 {
			w, _ := New(db, "fighting", "fighting", 1)
			w.Held["focus"] = focus
			w.FX = nil
			s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: 3, Q: BuildQ(m), Bon: map[string]int{BonusArea: area}}
			w.fire(s)
			for i := 0; i < 30; i++ {
				w.stepFX(1.0 / 60)
				for _, d := range w.DL {
					d.t = 0
				}
				for len(w.DL) > 0 {
					d := w.DL[0]
					w.DL = w.DL[1:]
					d.fn()
				}
			}
			r := 0.0
			for _, f := range w.FX {
				if f.R > r && f.Kind != "txt" {
					r = f.R
				}
			}
			return r
		}
		a, b, c := reach(0, 0), reach(3, 0), reach(0, 3)
		if !(b > a && c > a) {
			t.Errorf("%s (%s): reach %v, with Focus %v, with Range bonuses %v", id, m.Shape, a, b, c)
		}
	}
}

// The six newer attack types do what they say.
func TestNewAttackTypes(t *testing.T) {
	db := loadDB(t)
	for id, sh := range map[string]string{"fighting_focuspunch": "charge", "fighting_revenge": "counter", "normal_wrap": "bind",
		"normal_rage": "ramp", "psychic_storedpower": "ramp", "normal_explosion": "sac", "water_dive": "vanish"} {
		if m := db.Moves[id]; m == nil || m.Shape != sh {
			t.Fatalf("%s: shape %v, want %s", id, m, sh)
		}
		if _, ok := db.Balance.MoveDamage[id]; ok {
			t.Errorf("%s still has a web balance multiplier", id)
		}
	}
	setup := func(seed int64) (*World, *Enemy) {
		w, _ := New(db, "normal", "normal", seed)
		w.Slots = nil
		e := &Enemy{X: w.P.X + 50, Y: w.P.Y, R: 10, HP: 1e6, MHP: 1e6, Type: "normal", Sp: 50, Dm: 0}
		w.E = []*Enemy{e}
		return w, e
	}
	fire := func(w *World, id string) *Slot {
		m := db.Moves[id]
		s := &Slot{ID: m.ID, Move: m, Kind: m.Shape, Type: m.Type, Lv: 5, Q: BuildQ(m)}
		w.fire(s)
		return s
	}
	run := func(w *World, sec float64) {
		for i := 0; i < int(sec*60); i++ {
			w.Step(1.0/60, Input{})
		}
	}
	// charge: nothing until it lets go, then a big hit
	w, e := setup(1)
	fire(w, "fighting_focuspunch")
	if e.HP != 1e6 || !(w.P.Chg > 0) {
		t.Fatalf("charge hit early or isn't charging (hp %v chg %v)", e.HP, w.P.Chg)
	}
	run(w, 1.4)
	if e.HP >= 1e6 {
		t.Error("charge never released")
	}
	// counter: stored damage makes it hit harder
	w, e = setup(2)
	fire(w, "fighting_revenge")
	run(w, .6)
	base := 1e6 - e.HP
	w2, e2 := setup(2)
	w2.P.Ctr = w2.P.MHP * .5
	fire(w2, "fighting_revenge")
	run(w2, .6)
	if !(1e6-e2.HP > base*2) {
		t.Errorf("counter: plain %v, after taking damage %v", base, 1e6-e2.HP)
	}
	// bind: the foe stays put and keeps taking damage
	w, e = setup(3)
	fire(w, "normal_wrap")
	x, hp := e.X, e.HP
	run(w, 1.5)
	if math.Abs(e.X-x) > 2 || !(e.HP < hp) || len(w.Binds) != 1 {
		t.Errorf("bind: moved %v, damaged %v, binds %d", e.X-x, hp-e.HP, len(w.Binds))
	}
	// ramp: landed uses stack up
	w, e = setup(4)
	s := fire(w, "normal_rage")
	h1 := 1e6 - e.HP
	for i := 0; i < 4; i++ {
		w.fire(s)
	}
	h5 := 1e6 - e.HP
	if s.rampN != 5 || !(h5-h1 > h1*4.5) {
		t.Errorf("ramp: stacks %d, first %v, total %v", s.rampN, h1, h5)
	}
	// sacrifice: costs HP but never knocks you out
	w, e = setup(5)
	w.P.HP = 2
	fire(w, "normal_explosion")
	run(w, .8)
	if w.P.HP < 1 || w.P.Dead || e.HP >= 1e6 {
		t.Errorf("sac: hp %v dead %v hit %v", w.P.HP, w.P.Dead, e.HP < 1e6)
	}
	// vanish: untouchable while under, then bursts up
	w, e = setup(6)
	fire(w, "water_dive")
	hp0 := w.P.HP
	w.hurt(30, "normal")
	if w.P.HP != hp0 {
		t.Error("vanished player took damage")
	}
	run(w, 1)
	if w.P.Van > 0 || e.HP >= 1e6 {
		t.Errorf("vanish: still under %v, hit %v", w.P.Van, e.HP < 1e6)
	}
}
