package sim

import "testing"

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
