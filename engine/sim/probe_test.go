package sim

import (
	"sort"
	"testing"
)

// ProbeMove must leave the world untouched: a probed world and an unprobed twin stay identical.
func TestProbeLeavesWorldAlone(t *testing.T) {
	db := loadDB(t)
	ids := make([]string, 0, len(db.Moves))
	for id := range db.Moves {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	run := func(probe bool) uint64 {
		w, _ := New(db, "fire", "fire", 42)
		w.AutoDash = true
		b := NewBot(w, 9)
		probed := false
		for w.T < 90 && w.State != StateDead {
			if w.Waiting() {
				w.BotDecide()
				continue
			}
			if probe && !probed && w.T > 30 {
				probed = true
				for _, id := range ids {
					for _, lv := range []int{1, 5, 20} {
						w.ProbeMove(id, lv, false)
						w.ProbeMove(id, lv, true)
					}
				}
			}
			w.Step(1.0/60, b.Input(1.0/60))
		}
		return w.Hash()
	}
	if a, b := run(false), run(true); a != b {
		t.Fatalf("probing changed the run: %x vs %x", a, b)
	}
}

func TestProbeCounts(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "fire", "fire", 1)
	for id, m := range db.Moves {
		a, b := w.ProbeMove(id, 1, false), w.ProbeMove(id, 10, false)
		if m.Shape == "proj" && a.N == 0 {
			t.Errorf("%s: no projectiles counted", id)
		}
		_ = b
	}
}
