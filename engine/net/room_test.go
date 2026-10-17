package net

import (
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/lockstep"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// ---- in-memory Net: pipes with a delivery lag in Update calls ----

type memConn struct {
	peer   *memConn
	q      []Msg
	closed bool
	open   bool
}

func (c *memConn) Send(b []byte) {
	if c.peer != nil && !c.closed {
		c.peer.q = append(c.peer.q, Msg{Bin: append([]byte{}, b...)})
	}
}
func (c *memConn) SendText(s string) {
	if c.peer != nil && !c.closed {
		c.peer.q = append(c.peer.q, Msg{Text: s})
	}
}
func (c *memConn) Recv() []Msg { o := c.q; c.q = nil; return o }
func (c *memConn) Open() bool  { return c.open && !c.closed }
func (c *memConn) Closed() bool {
	return c.closed || (c.peer != nil && c.peer.closed)
}
func (c *memConn) Close() { c.closed = true }

type memNet struct {
	rooms map[string]func(Peer)
}

func (m *memNet) Available() (bool, string) { return true, "" }
func (m *memNet) Host(name string, onCode func(string), onPeer func(Peer), onErr func(string)) func() {
	m.rooms["ABCDE"] = onPeer
	onCode("ABCDE")
	return func() { delete(m.rooms, "ABCDE") }
}
func (m *memNet) Join(code, name string, onConn func(Conn), onErr func(string)) func() {
	h, ok := m.rooms[code]
	if !ok {
		onErr("No room with that code.")
		return func() {}
	}
	a, b := &memConn{open: true}, &memConn{open: true}
	a.peer, b.peer = b, a
	h(Peer{UID: name, Name: name, Conn: b})
	onConn(a)
	return func() {}
}

func loadDB(t *testing.T) *content.DB {
	db, err := content.Load(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func worldFrom(t *testing.T, db *content.DB, st *StartMsg) *sim.World {
	cfg := sim.Config{LI: st.LI, Tier: st.Tier, Mods: st.Mods, Stage: st.Stage, Lock: st.Lock, Up: st.Players[0].Up, Training: st.Players[0].Training}
	w, err := sim.NewCoop(db, st.Players, st.Biome, st.Seed, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w.AutoDash = true
	return w
}

func TestRoomLobbyAndRun(t *testing.T) {
	db := loadDB(t)
	n := &memNet{rooms: map[string]func(Peer){}}
	spec := func(id string) sim.PlayerSpec { return sim.PlayerSpec{ID: id, Up: map[string]int{"hp": 1}} }
	host := HostRoom(n, "h", "Host", spec("fire"), "d1")
	g1 := JoinRoom(n, "ABCDE", "g1", "Guest1", spec("water"), "d1")
	g2 := JoinRoom(n, "ABCDE", "g2", "Guest2", spec("fire"), "d1")
	bad := JoinRoom(n, "ABCDE", "g3", "Old", spec("grass"), "other-data")
	rooms := []*Room{host, g1, g2, bad}
	pump := func(k int) {
		for i := 0; i < k; i++ {
			for _, r := range rooms {
				r.Update()
			}
		}
	}
	pump(4)
	if host.Code != "ABCDE" || len(host.Players) != 3 {
		t.Fatalf("host: code %q players %d", host.Code, len(host.Players))
	}
	if bad.Err == "" {
		t.Fatal("mismatched data was accepted")
	}
	if g2.Me != 2 || !g2.Players[2].Dup {
		t.Fatalf("guest2 should be seat 2 with a duplicate Pokémon: me=%d %+v", g2.Me, g2.Players)
	}
	if ok, why := host.CanStart(); ok {
		t.Fatal("start allowed with a duplicate: " + why)
	}
	g2.SetMe(spec("grass"), true)
	g1.SetMe(spec("water"), true)
	pump(3)
	if ok, why := host.CanStart(); !ok {
		t.Fatal(why)
	}
	st := host.StartRun(42, "fire", -1, 1, nil, 1, false, 3)
	if st == nil {
		t.Fatal("start failed")
	}
	pump(2)
	if g1.Start == nil || g2.Start == nil || g1.Start.Me != 1 || g2.Start.Me != 2 || g1.Start.Seed != 42 {
		t.Fatalf("guests did not get the start: %+v %+v", g1.Start, g2.Start)
	}
	if g1.Start.Players[1].Kind != "net" || g1.Start.Players[2].Up["hp"] != 1 {
		t.Fatalf("bad specs: %+v", g1.Start.Players)
	}

	// run in lockstep over the room transports
	plays := []*Room{host, g1, g2}
	var ss []*lockstep.Session
	var bots []*sim.Bot
	for i, r := range plays {
		w := worldFrom(t, db, r.Start)
		s := lockstep.New(w, 3, r.Start.Me, r.Start.Delay, r)
		ss = append(ss, s)
		bots = append(bots, sim.NewBot(w, int64(50+i)))
	}
	host.OnGone = func(idx int) { ss[0].DropPlayer(idx) }
	frame := func(s *lockstep.Session, b *sim.Bot) lockstep.Frame {
		w := s.W
		f := lockstep.Quantise(b.Input(lockstep.Tick))
		if w.Waiting() {
			switch {
			case w.State == sim.StateLevelUp && w.Chooser == s.Me:
				f.Cmds = append(f.Cmds, lockstep.Cmd{Kind: lockstep.CmdChoose, Arg: int8(w.BotPick())})
			case w.State == sim.StateShrine && w.Chooser == s.Me:
				f.Cmds = append(f.Cmds, lockstep.Cmd{Kind: lockstep.CmdBoon})
			case w.State == sim.StateRoute && s.Me == 0:
				f.Cmds = append(f.Cmds, lockstep.Cmd{Kind: lockstep.CmdRoute})
			}
		}
		return f
	}
	for u := 0; u < 4000; u++ {
		if u == 2500 { // guest 2 drops out mid-run
			g2.Leave()
			plays = plays[:2]
		}
		for i, r := range plays {
			r.Update()
			ss[i].Update(frame(ss[i], bots[i]))
		}
	}
	a, b := ss[0], ss[1]
	if a.Desync != 0 || b.Desync != 0 {
		t.Fatalf("desync %d %d", a.Desync, b.Desync)
	}
	if a.Tick < 3900 || b.Tick < 3900 {
		t.Fatalf("stalled: ticks %d %d", a.Tick, b.Tick)
	}
	if !a.IsGone(2) || !b.IsGone(2) || !a.W.PL[2].Gone || !b.W.PL[2].Gone {
		t.Fatal("guest 2 was not dropped on both peers")
	}
	if !host.Players[2].Gone {
		t.Fatal("host lobby does not show the seat as gone")
	}
	t.Logf("tick %d t=%.1f lv=%d kills=%d", a.Tick, a.W.T, a.W.XPS.Lv, a.W.Kills)

	// back to the lobby: the gone seat is removed, guests unready
	host.EndRun()
	pump(2)
	if len(host.Players) != 2 || len(g1.Players) != 2 || g1.Started || g1.Players[1].Ready {
		t.Fatalf("lobby after the run: host %+v guest %+v", host.Players, g1.Players)
	}
	host.Leave()
	pump(2)
	if g1.Err == "" {
		t.Fatal("guest not told the host left")
	}
}
