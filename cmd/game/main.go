// Command game is the playable build: a Windows .exe (GOOS=windows) or a browser build (GOOS=js GOARCH=wasm).
package main

import (
	"flag"
	"log"
	"math/rand"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/client"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

func main() {
	opts := options() // flags on desktop, ?query=params in the browser
	db, err := content.Load(data.FS)
	if err != nil {
		log.Fatal(err)
	}
	g := client.New(data.FS, db)
	if sc := opts["screen"]; sc != "" { // open a menu screen directly (screenshots / testing)
		g.OpenScreen(sc)
	}
	if opts["demo"] != "" { // the bot plays: ?demo=fire  (optionally &biome=water&seed=3&t=40 to fast-forward)
		g.Demo(true)
		if opts["ab"] == "1" {
			g.SetAbility(1)
		}
		seed, _ := strconv.ParseInt(opts["seed"], 10, 64)
		if seed == 0 {
			seed = rand.Int63()
		}
		g.Start(opts["demo"], opts["biome"], seed)
		if opts["hold"] != "" {
			g.Hold = true
		}
		if t, _ := strconv.ParseFloat(opts["t"], 64); t > 0 {
			switch opts["until"] {
			case "route":
				g.FastForwardUntil(t, sim.StateRoute)
			case "shrine":
				g.FastForward(t)
				g.W.OpenShrineNow()
			default:
				g.FastForward(t)
			}
		}
	}
	ebiten.SetWindowTitle("Pokémon Survivor")
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func parseKV(s string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(strings.TrimPrefix(s, "?"), "&") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		if v == "" {
			v = "1"
		}
		m[k] = v
	}
	return m
}

func flagOptions() map[string]string {
	demo := flag.String("demo", "", "let the bot play as this Pokémon id")
	biome := flag.String("biome", "", "biome type")
	seed := flag.String("seed", "", "random seed")
	t := flag.String("t", "", "fast-forward the demo to this many seconds")
	flag.Parse()
	return map[string]string{"demo": *demo, "biome": *biome, "seed": *seed, "t": *t}
}
