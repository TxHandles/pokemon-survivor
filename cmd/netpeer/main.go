// netpeer is a headless online co-op player: it hosts or joins a room over the real desktop
// transport (pion WebRTC) with a bot steering its Pokémon, and reports the lockstep state. Used to
// test native ↔ browser play without a display:
//
//	netpeer -host TEST1 -signal http://localhost:8773 -secs 60
//	netpeer -join TEST1 -signal http://localhost:8773
//
// Without -signal it uses Firestore like the desktop game (needs network access).
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/lockstep"
	"github.com/TxHandles/pokemon-survivor/engine/net"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

func main() {
	host := flag.String("host", "", "host a room with this code")
	join := flag.String("join", "", "join a room with this code")
	sig := flag.String("signal", "", "HTTP test relay base URL (default: Firestore)")
	secs := flag.Float64("secs", 60, "seconds of play before reporting")
	id := flag.String("id", "fire", "Pokémon")
	name := flag.String("name", "", "player name")
	players := flag.Int("players", 2, "host: start once this many players are in")
	flag.Parse()
	if (*host == "") == (*join == "") {
		fmt.Println("use -host CODE or -join CODE")
		os.Exit(2)
	}
	db, err := content.Load(data.FS)
	if err != nil {
		panic(err)
	}
	mode := ""
	if *sig != "" {
		net.SignalBase = *sig
		mode = "http"
	}
	n := net.NewNet(mode)
	uid, uname := "", ""
	done := make(chan struct{})
	n.Login(func(u, nm, e string) {
		if e != "" {
			fmt.Println("login failed:", e)
			os.Exit(1)
		}
		uid, uname = u, nm
		close(done)
	})
	<-done
	if *name != "" {
		uname = *name
	}
	spec := sim.PlayerSpec{ID: *id, Up: map[string]int{}}
	dataID := net.DataID(data.FS)
	var r *net.Room
	if *host != "" {
		net.FixedCode = *host
		r = net.HostRoom(n, uid, uname, spec, dataID)
	} else {
		r = net.JoinRoom(n, *join, uid, uname, spec, dataID)
	}
	fmt.Println("room:", r.Status)
	var ls *lockstep.Session
	var bot *sim.Bot
	last := ""
	ready := false
	t0 := time.Now()
	for range time.Tick(time.Second / 60) {
		r.Update()
		if r.Err != "" {
			fmt.Println("room error:", r.Err)
			os.Exit(1)
		}
		if r.Status != last {
			fmt.Println("room:", r.Status)
			last = r.Status
		}
		if ls == nil {
			switch {
			case r.Host:
				if ok, _ := r.CanStart(); ok && len(r.Players) >= *players {
					r.StartRun(7, "fire", -1, 1, nil, 1, false, 4)
				}
			case r.Connected() && !ready:
				if r.Me >= 0 && r.Me < len(r.Players) && r.Players[r.Me].Dup {
					spec.ID = "water"
				}
				r.SetMe(spec, true)
				ready = true
			}
			if r.Started && r.Start != nil {
				st := r.Start
				cfg := sim.Config{LI: st.LI, Tier: st.Tier, Mods: st.Mods, Stage: st.Stage, Lock: st.Lock, Up: st.Players[0].Up, Training: st.Players[0].Training}
				w, err := sim.NewCoop(db, st.Players, st.Biome, st.Seed, cfg)
				if err != nil {
					panic(err)
				}
				w.ViewW, w.ViewH = 1280, 720
				w.AutoDash = true
				ls = lockstep.New(w, len(st.Players), st.Me, st.Delay, r)
				bot = sim.NewBot(w, int64(100+st.Me))
				if r.Host {
					r.OnGone = func(i int) { ls.DropPlayer(i) }
				}
				t0 = time.Now()
				fmt.Println("run started: me =", st.Me, "of", len(st.Players), "seed", st.Seed)
			}
			if time.Since(t0) > 60*time.Second {
				fmt.Println("timed out waiting in the lobby")
				os.Exit(1)
			}
			continue
		}
		w := ls.W
		f := lockstep.Quantise(bot.Input(lockstep.Tick))
		if w.Waiting() {
			switch {
			case w.PendingKind(ls.Me) == "lv":
				w.UseLocal(ls.Me)
				f.Cmds = append(f.Cmds, lockstep.Cmd{Kind: lockstep.CmdChoose, Arg: int8(w.BotPick())})
			case w.PendingKind(ls.Me) == "shrine":
				f.Cmds = append(f.Cmds, lockstep.Cmd{Kind: lockstep.CmdBoon})
			case w.State == sim.StateRoute && w.RouteGo < 0 && ls.Me < len(w.RouteVotes) && w.RouteVotes[ls.Me] < 0:
				f.Cmds = append(f.Cmds, lockstep.Cmd{Kind: lockstep.CmdRoute})
			}
		}
		ls.Update(f)
		if ls.Tick%600 == 0 && ls.Tick > 0 {
			fmt.Printf("tick %d  t=%.1f  lv %d  kills %d  stalled %d  waiting %v  desync %d  hash %016x\n", ls.Tick, w.T, w.XPS.Lv, w.Kills, ls.Stalled, ls.WaitingOn(), ls.Desync, w.Hash())
		}
		if ls.Desync != 0 {
			fmt.Println("DESYNC at tick", ls.Desync)
			os.Exit(3)
		}
		if w.T >= *secs || w.State == sim.StateDead {
			fmt.Printf("done: tick %d t=%.1f lv %d kills %d state %d desync %d hash %016x\n", ls.Tick, w.T, w.XPS.Lv, w.Kills, w.State, ls.Desync, w.Hash())
			r.Leave()
			return
		}
	}
}
