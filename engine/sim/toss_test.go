package sim

import "testing"

func TestSeismicToss(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "fire", "fire", 5)
	w.E = nil
	P := w.P
	for i := 0; i < 5; i++ {
		w.E = append(w.E, &Enemy{X: P.X + 40 + float64(i)*20, Y: P.Y, R: 10, HP: 1e6, MHP: 1e6, Type: "normal", Dm: 50, Sp: 0})
	}
	far := w.E[4]
	m := db.Moves["fighting_seismictoss"]
	if m.Shape != "toss" {
		t.Fatalf("shape %q", m.Shape)
	}
	hp := P.HP
	w.fire(&Slot{ID: m.ID, Move: m, Kind: m.Shape, Type: m.Type, Lv: 1, Q: BuildQ(m)})
	n := 0
	for _, e := range w.E {
		if e.Toss != nil {
			n++
		}
	}
	if n != 3 || far.Toss != nil {
		t.Fatalf("grabbed %d (far grabbed: %v)", n, far.Toss != nil)
	}
	for i := 0; i < 60; i++ {
		w.stepEnemies(1.0 / 60)
	}
	for i, e := range w.E[:3] {
		if e.Toss != nil || e.HP >= 1e6 || e.X > P.X-100 {
			t.Errorf("foe %d: toss %v hp %v x %v (player %v)", i, e.Toss != nil, e.HP, e.X-P.X, P.X)
		}
	}
	if P.HP < hp {
		t.Errorf("held foes hurt the player")
	}
}
