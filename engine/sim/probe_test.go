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

// What the client reads while drawing (HUD, level-up cards, map) must not change a co-op world:
// two lockstep worlds, one of them "drawn" every tick, stay identical.
func TestCoopDrawReadsLeaveWorldAlone(t *testing.T) {
	db := loadDB(t)
	specs := []PlayerSpec{{ID: "fire", Name: "A", Kind: "bot"}, {ID: "water", Name: "B", Kind: "bot"}}
	mk := func() *World {
		w, err := NewCoop(db, specs, "grass", 11, Config{LI: -1})
		if err != nil {
			t.Fatal(err)
		}
		w.AutoDash = true
		return w
	}
	a, b := mk(), mk()
	read := func(w *World) {
		for _, s := range w.Slots {
			w.CoolFrac(s)
			w.DashReady()
		}
		cx, cy := int(w.P.X/ChunkSize), int(w.P.Y/ChunkSize)
		for i := -3; i <= 3; i++ {
			for j := -3; j <= 3; j++ {
				for _, o := range w.Chunk(cx+i, cy+j) {
					w.LandmarkKind(o)
				}
				w.Pools(cx+i, cy+j)
			}
		}
		if w.State == StateLevelUp {
			for _, o := range w.Opts {
				if o.Move {
					for _, lv := range []int{1, o.Level, o.Level + 1, 10} {
						w.MovePreview(o.ID, lv, false)
						w.ProbeMove(o.ID, lv, false)
						w.ProbeMove(o.ID, lv, true)
					}
				}
			}
		}
		for _, pc := range w.PL {
			w.DeathRecap(pc.P)
		}
		w.Standings()
	}
	for step := 0; a.T < 200 && a.State != StateDead; step++ {
		read(b)
		if a.Waiting() != b.Waiting() {
			t.Fatalf("t=%.2f waiting differs", a.T)
		}
		if a.Waiting() {
			a.BotDecide()
			b.BotDecide()
			continue
		}
		a.Step(1.0/60, Input{})
		b.Step(1.0/60, Input{})
		if step%60 == 0 && a.Hash() != b.Hash() {
			t.Fatalf("t=%.2f: hashes differ", a.T)
		}
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("end: hashes differ")
	}
}

// Picking a swap card asks which held item to give up; that choice replaces the chosen item.
func TestSwapPick(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "fire", "fire", 3)
	w.Held = map[string]int{"band": 2, "lens": 1}
	w.HeldOrder = []string{"band", "lens"}
	w.State, w.Chooser = StateLevelUp, 0
	w.Opts = []Option{{ID: "metal", New: true, Swap: "band", SwapAsk: true}}
	w.PL[0].Queue = []*Prompt{{Kind: "lv", Opts: w.Opts}}
	w.Choose(0)
	if !w.SwapPick || len(w.Opts) != 2 || w.State != StateLevelUp {
		t.Fatalf("expected a swap pick, got %+v", w.Opts)
	}
	w.Choose(1) // give up lens
	if w.Held["lens"] != 0 || w.Held["metal"] != 1 || w.Held["band"] != 2 || w.SwapPick {
		t.Fatalf("held after swap: %v", w.Held)
	}
}

// Co-op level-ups: every player picks at the same time; play resumes when all are done.
func TestSimultaneousLevelUps(t *testing.T) {
	db := loadDB(t)
	specs := []PlayerSpec{{ID: "fire", Name: "A", Kind: "net"}, {ID: "water", Name: "B", Kind: "net"}}
	w, _ := NewCoop(db, specs, "fire", 5, Config{})
	w.XPS.XP = w.XPS.NX
	w.Step(1.0/60, Input{})
	if w.State != StateLevelUp || w.PendingKind(0) != "lv" || w.PendingKind(1) != "lv" {
		t.Fatalf("state %v pending %q %q", w.State, w.PendingKind(0), w.PendingKind(1))
	}
	w.ChooseFor(1, 0)
	if w.PendingKind(1) != "" || w.State != StateLevelUp {
		t.Fatalf("B done, A still choosing: state %v pending B %q", w.State, w.PendingKind(1))
	}
	w.ChooseFor(0, 0)
	if w.State != StatePlay {
		t.Fatalf("both done: state %v", w.State)
	}
}

// Route map: votes decide, a tie goes to the host.
func TestRouteVote(t *testing.T) {
	db := loadDB(t)
	specs := []PlayerSpec{{ID: "fire", Name: "A", Kind: "net"}, {ID: "water", Name: "B", Kind: "net"}}
	w, _ := NewCoop(db, specs, "fire", 5, Config{})
	w.State = StatePlay
	w.openRoute()
	if len(w.Route) < 2 {
		t.Skip("route too short")
	}
	w.VoteRoute(1, 1)
	if w.RouteGo >= 0 {
		t.Fatal("decided before everyone voted")
	}
	w.VoteRoute(0, 0)
	if w.RouteGo != 0 {
		t.Fatalf("tie should go to the host's pick, got %d", w.RouteGo)
	}
	want := w.Route[0].Type
	for i := 0; i < 200 && w.State == StateRoute; i++ {
		w.Step(1.0/60, Input{})
	}
	if w.State != StatePlay || w.Biome != want {
		t.Fatalf("after travel: state %v biome %s (want %s)", w.State, w.Biome, want)
	}
}
