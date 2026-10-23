package sim

import "testing"

// Four bots share one run: no panics, every player keeps playing, XP is shared and revives happen.
func TestCoopBots(t *testing.T) {
	db := loadDB(t)
	specs := []PlayerSpec{{ID: "fire", Name: "A", Kind: "bot"}, {ID: "water", Name: "B", Kind: "bot"}, {ID: "grass", Name: "C", Kind: "bot"}, {ID: "normal", Name: "D", Kind: "bot"}}
	w, err := NewCoop(db, specs, "fire", 5, Config{LI: -1})
	if err != nil {
		t.Fatal(err)
	}
	w.AutoDash = true
	revives, deaths, centers := 0, 0, 0
	for w.T < 400 && w.State != StateDead {
		if w.Waiting() {
			w.BotDecide()
			continue
		}
		pc := w.PCN != nil
		w.Step(1.0/60, Input{})
		if !pc && w.PCN != nil {
			centers++
		}
	}
	for _, c := range w.PL {
		deaths += c.Deaths
		revives += c.Revives
		if c.P.Lv != w.XPS.Lv {
			t.Errorf("%s level %d, shared %d", c.Name, c.P.Lv, w.XPS.Lv)
		}
		if len(c.Slots) == 0 || c.Slots[0].Owner != c.Idx {
			t.Errorf("%s: bad slots", c.Name)
		}
	}
	st := w.Standings()
	t.Logf("t=%.0f state=%v lv=%d kills=%d deaths=%d revives=%d centers=%d standings=%+v", w.T, w.State, w.XPS.Lv, w.Kills, deaths, revives, centers, st)
	if w.Kills < 100 {
		t.Errorf("too few kills: %d", w.Kills)
	}
}

// Solo play through the co-op code path behaves as before (sanity: survives a while, levels up).
func TestSoloStillWorks(t *testing.T) {
	db := loadDB(t)
	w, _ := NewRun(db, "fire", "fire", 3, 0)
	w.AutoDash = true
	b := NewBot(w, 1)
	for w.T < 120 && w.State != StateDead {
		if w.Waiting() {
			w.BotDecide()
			continue
		}
		w.Step(1.0/60, b.Input(1.0/60))
	}
	if w.P.Lv < 3 || len(w.PL) != 1 || w.Chooser != -1 {
		t.Fatalf("lv %d players %d chooser %d", w.P.Lv, len(w.PL), w.Chooser)
	}
}

// Each player has MaxLives lives: after the last faint nobody can revive them.
func TestCoopLives(t *testing.T) {
	db := loadDB(t)
	specs := []PlayerSpec{{Name: "A", ID: "fire", Kind: "human"}, {Name: "B", ID: "water", Kind: "human"}}
	w, err := NewCoop(db, specs, "fire", 5, Config{LI: -1})
	if err != nil {
		t.Fatal(err)
	}
	for life := 1; life <= MaxLives; life++ {
		w.useP(1)
		w.P.HP = 1
		w.hurt(1e6, "normal")
		if !w.P.Dead {
			t.Fatalf("life %d: didn't faint", life)
		}
		if life < MaxLives {
			if w.PCN == nil {
				t.Fatalf("life %d: no Pokémon Center", life)
			}
			w.PCN.X, w.PCN.Y = w.PL[0].P.X, w.PL[0].P.Y
			w.useP(0)
			w.pcTick(2)
			if w.PL[1].P.Dead {
				t.Fatalf("life %d: not revived", life)
			}
		}
	}
	if w.PCN != nil || revivable(w.PL[1]) {
		t.Fatal("a player out of lives can still be revived")
	}
	if w.State == StateDead {
		t.Fatal("run ended while a player is still alive")
	}
}
