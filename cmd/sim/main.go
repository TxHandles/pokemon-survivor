// Command sim runs headless bot games with the Go simulation, for balance checks and regression tests.
//
//	go run ./cmd/sim -runs 40 -mins 5            # every starter, 40 runs each
//	go run ./cmd/sim -who fire,water -runs 100
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

func main() {
	runs := flag.Int("runs", 20, "runs per Pokémon")
	mins := flag.Float64("mins", 5, "survival mark in minutes")
	who := flag.String("who", "", "comma-separated Pokémon ids (default: every type base form)")
	biome := flag.String("biome", "", "biome type (default: the Pokémon's own type)")
	workers := flag.Int("j", 2, "parallel workers")
	harness := flag.Bool("harness", true, "use the web balance harness leash (260 / .8 / 1.75) instead of the game's")
	verbose := flag.Bool("v", false, "print each run")
	flag.Parse()

	db, err := content.Load(data.FS)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ids := strings.Split(*who, ",")
	if *who == "" {
		ids = content.Types
	}
	type res struct {
		id        string
		t         float64
		lv, kills int
		dmg       map[string]float64
	}
	jobs := make(chan [2]int)
	out := make(chan res)
	var wg sync.WaitGroup
	start := time.Now()
	for k := 0; k < *workers; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				id := ids[j[0]]
				b := *biome
				if b == "" {
					b = db.ByID[id].Types[0]
				}
				w, err := sim.New(db, id, b, int64(j[1]*7919+j[0]*104729+1))
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				w.AutoDash = true
				if *harness {
					w.LeashFar, w.LeashBias, w.LeashSpread = 260, .8, 1.75
				}
				bot := sim.NewBot(w, int64(j[1]*31+j[0]))
				const dt = 1.0 / 60
				for w.State != sim.StateDead && w.T < *mins*60 {
					if w.State == sim.StateLevelUp {
						w.Choose(w.BotPick())
						continue
					}
					w.Step(dt, bot.Input(dt))
				}
				out <- res{id, w.T, w.P.Lv, w.Kills, w.DMG}
			}
		}()
	}
	go func() {
		for r := 0; r < *runs; r++ {
			for i := range ids {
				jobs <- [2]int{i, r}
			}
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()
	type agg struct {
		n, alive, lv, kills int
		t                   float64
		dmg                 map[string]float64
	}
	A := map[string]*agg{}
	for r := range out {
		a := A[r.id]
		if a == nil {
			a = &agg{dmg: map[string]float64{}}
			A[r.id] = a
		}
		a.n++
		a.t += r.t
		a.lv += r.lv
		a.kills += r.kills
		if r.t >= *mins*60 {
			a.alive++
		}
		for k, v := range r.dmg {
			a.dmg[k] += v
		}
		if *verbose {
			fmt.Printf("%-10s t=%6.1f lv=%2d kills=%d\n", r.id, r.t, r.lv, r.kills)
		}
	}
	tot, alive := 0, 0
	fmt.Printf("%-10s %6s %7s %5s %6s  top moves\n", "pokemon", "alive", "avg t", "lv", "kills")
	for _, id := range ids {
		a := A[id]
		if a == nil {
			continue
		}
		tot += a.n
		alive += a.alive
		type kv struct {
			k string
			v float64
		}
		var mv []kv
		sum := 0.0
		for k, v := range a.dmg {
			mv = append(mv, kv{k, v})
			sum += v
		}
		sort.Slice(mv, func(i, j int) bool { return mv[i].v > mv[j].v })
		top := ""
		for i := 0; i < len(mv) && i < 3; i++ {
			top += fmt.Sprintf(" %s %.0f%%", mv[i].k, mv[i].v/math.Max(sum, 1)*100)
		}
		fmt.Printf("%-10s %5.0f%% %6.0fs %5.1f %6d %s\n", id, float64(a.alive)/float64(a.n)*100, a.t/float64(a.n), float64(a.lv)/float64(a.n), a.kills/a.n, top)
	}
	fmt.Printf("\nsurvived %.0f:00 — %d/%d = %.0f%%   (%.1fs)\n", *mins, alive, tot, float64(alive)/float64(max(tot, 1))*100, time.Since(start).Seconds())
}
