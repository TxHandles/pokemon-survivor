package sim

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

// PERF=1 go test -run TestPerfLong ./engine/sim -cpuprofile cpu.out
// A long two-player bot run (players can't die) with per-minute tick timings.
func TestPerfLong(t *testing.T) {
	if os.Getenv("PERF") == "" {
		t.Skip("set PERF=1")
	}
	db := loadDB(t)
	specs := []PlayerSpec{{ID: "fire", Name: "A", Kind: "bot"}, {ID: "water", Name: "B", Kind: "bot"}}
	w, _ := NewCoop(db, specs, "fire", 7, Config{})
	w.AutoDash = true
	mins := 22.0
	var ticks []time.Duration
	minute := 0
	for w.T < mins*60 && w.State != StateDead {
		for _, c := range w.PL {
			c.P.HP = c.P.MHP
		}
		if w.Waiting() {
			w.BotDecide()
			continue
		}
		t0 := time.Now()
		w.Step(1.0/60, Input{})
		ticks = append(ticks, time.Since(t0))
		if int(w.T/60) > minute {
			sort.Slice(ticks, func(i, j int) bool { return ticks[i] < ticks[j] })
			n := len(ticks)
			fmt.Printf("min %2d: enemies %4d fx %4d proj %4d gems %4d zones %3d eb %3d | p50 %v p99 %v max %v\n", minute+1, len(w.E), len(w.FX), len(w.PR), len(w.Gems), len(w.ZN), len(w.EB), ticks[n/2], ticks[n*99/100], ticks[n-1])
			ticks = ticks[:0]
			minute = int(w.T / 60)
		}
	}
}
