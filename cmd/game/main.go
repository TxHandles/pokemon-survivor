// Command game is the playable build: a Windows .exe (GOOS=windows) or a browser build (GOOS=js GOARCH=wasm).
package main

import (
	"flag"
	"log"
	"math/rand"
	"net/url"
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
	if code := opts["code"]; code != "" { // load a save code (testing)
		g.LoadCode(code)
	}
	if opts["touch"] != "" { // force the touch overlay on (screenshots / testing)
		g.ForceTouch()
	}
	if opts["nodraw"] == "1" { // testing: no rendering
		g.NoDraw(true)
	}
	if m := opts["signal"]; m != "" { // test signalling for online rooms: local (tabs) or http (page origin)
		g.SignalMode(m)
	}
	if c := opts["host"]; c != "" { // host an online room with this code right away (testing; &auto=1 starts by itself)
		g.AutoHost(c, opts["auto"] == "1")
	}
	if c := opts["join"]; c != "" { // join an online room right away (testing; &auto=1 readies up by itself)
		g.AutoJoin(c, opts["auto"] == "1")
	}
	if a, ok := opts["fradd"], opts["fraccept"] == "1"; a != "" || ok { // testing: send a friend request / accept requests
		g.TestFriends(a, ok, opts["name"])
	}
	if sel := opts["sel"]; sel != "" { // testing: pre-select a Pokémon in the Pokédex
		g.DexSelect(sel)
	}
	if sc := opts["screen"]; sc != "" { // open a menu screen directly (screenshots / testing)
		g.OpenScreen(sc)
		if sy, _ := strconv.ParseFloat(opts["scroll"], 64); sy > 0 {
			g.ScrollTo(sy)
		}
	}
	if cp := opts["coop"]; cp != "" { // bot co-op demo: ?coop=fire,water,grass&t=120
		g.DemoCoop(strings.Split(cp, ","), opts["biome"], 0)
		if t, _ := strconv.ParseFloat(opts["t"], 64); t > 0 {
			g.FastForward(t)
		}
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
		switch opts["then"] { // testing: open a screen after fast-forwarding
		case "pause":
			g.TestPause()
		case "results":
			g.TestResults()
		case "levelup":
			g.TestLevelUp()
		case "route":
			g.TestRoute()
		}
	}
	ebiten.SetWindowTitle("Pokémon Survivor")
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(60)
	ebiten.SetRunnableOnUnfocused(true) // online co-op keeps running; Auto-pause pauses solo runs
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
		if u, err := url.QueryUnescape(v); err == nil {
			v = u
		}
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
