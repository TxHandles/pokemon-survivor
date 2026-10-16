package sim

import (
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
)

func loadDB(t *testing.T) *content.DB {
	db, err := content.Load(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// Every implemented move fires for 40s at Lv 10 (and evolved) without panicking, and deals damage.
func TestEveryMove(t *testing.T) {
	db := loadDB(t)
	for _, m := range db.MoveList {
		if !Implemented[m.Shape] {
			continue
		}
		for _, evo := range []bool{false, true} {
			w, _ := New(db, "normal", "normal", 7)
			w.AutoDash = true
			w.Slots = nil
			s := w.addMove(m.ID)
			s.Lv, s.Evo = 10, evo
			if evo {
				s.Lv = 20
			}
			b := NewBot(w, 1)
			for w.T < 40 && w.State != StateDead {
				if w.State == StateLevelUp {
					w.State = StatePlay // keep the loadout fixed
					continue
				}
				w.Step(1.0/60, b.Input(1.0/60))
			}
			dmg := w.DMG[m.ID]
			if dmg <= 0 && m.Shape != "wave" {
				t.Errorf("%s (%s, evo=%v) dealt no damage", m.ID, m.Shape, evo)
			}
		}
	}
}

// The same seed and inputs give the same run.
func TestDeterministic(t *testing.T) {
	db := loadDB(t)
	run := func() (float64, int, int) {
		w, _ := New(db, "fire", "fire", 42)
		w.AutoDash = true
		b := NewBot(w, 9)
		for w.T < 120 && w.State != StateDead {
			if w.State == StateLevelUp {
				w.Choose(w.BotPick())
				continue
			}
			w.Step(1.0/60, b.Input(1.0/60))
		}
		return w.P.HP, w.Kills, w.P.Lv
	}
	h1, k1, l1 := run()
	h2, k2, l2 := run()
	if h1 != h2 || k1 != k2 || l1 != l2 {
		t.Fatalf("runs differ: %v/%v/%v vs %v/%v/%v", h1, k1, l1, h2, k2, l2)
	}
}

func TestTypeChart(t *testing.T) {
	cases := []struct {
		a string
		d []string
		m float64
	}{{"water", []string{"fire"}, 1.25}, {"fire", []string{"water"}, .5}, {"ground", []string{"fire", "rock"}, 1.5}, {"normal", []string{"normal"}, .5}, {"grass", []string{"fire", "flying"}, .25}}
	for _, c := range cases {
		if got := TypeMul(c.a, c.d); got != c.m {
			t.Errorf("%s vs %v = %v, want %v", c.a, c.d, got, c.m)
		}
	}
}
