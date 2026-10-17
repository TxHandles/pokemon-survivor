// simhash prints a state-hash trace of a fixed co-op run. Built for different targets (amd64, wasm,
// arm64) the traces must be identical, or lockstep play between those builds would desync.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

func main() {
	secs := 240.0
	if len(os.Args) > 1 {
		secs, _ = strconv.ParseFloat(os.Args[1], 64)
	}
	db, err := content.Load(data.FS)
	if err != nil {
		panic(err)
	}
	specs := []sim.PlayerSpec{{ID: "fire", Name: "A"}, {ID: "water", Name: "B", Kind: "bot"}, {ID: "grass", Name: "C", Kind: "bot"}}
	w, err := sim.NewCoop(db, specs, "fire", 11, sim.Config{LI: -1})
	if err != nil {
		panic(err)
	}
	w.AutoDash = true
	b := sim.NewBot(w, 7)
	for i := 0; w.T < secs && w.State != sim.StateDead; i++ {
		if w.Waiting() {
			w.BotDecide()
		}
		w.Step(1.0/60, b.Input(1.0/60))
		if i%60 == 0 {
			fmt.Printf("%d %016x %.3f\n", i, w.Hash(), w.T)
		}
	}
	fmt.Printf("end %016x t=%.2f lv=%d kills=%d\n", w.Hash(), w.T, w.XPS.Lv, w.Kills)
}
