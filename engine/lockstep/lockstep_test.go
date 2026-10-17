package lockstep

import (
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// memNet joins peers in memory; each message is delivered after lag Update calls, optionally in a
// shuffled order within a batch, to mimic a real network.
type memNet struct {
	peers []*memPeer
	lag   int
	clock int
}

type memPeer struct {
	net   *memNet
	id    int
	inbox []queued
}

type queued struct {
	at int
	b  []byte
}

func newMemNet(n, lag int) *memNet {
	m := &memNet{lag: lag}
	for i := 0; i < n; i++ {
		m.peers = append(m.peers, &memPeer{net: m, id: i})
	}
	return m
}

func (p *memPeer) Send(b []byte) {
	for _, o := range p.net.peers {
		if o != p {
			o.inbox = append(o.inbox, queued{at: p.net.clock + p.net.lag, b: b})
		}
	}
}

func (p *memPeer) Recv() [][]byte {
	var out [][]byte
	keep := p.inbox[:0]
	for _, q := range p.inbox {
		if q.at <= p.net.clock {
			out = append(out, q.b)
		} else {
			keep = append(keep, q)
		}
	}
	p.inbox = keep
	// deliver the batch reversed: order on the wire must not matter
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func loadDB(t *testing.T) *content.DB {
	db, err := content.Load(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func newWorld(t *testing.T, db *content.DB, n int) *sim.World {
	ids := []string{"fire", "water", "grass", "electric"}
	var specs []sim.PlayerSpec
	for i := 0; i < n; i++ {
		specs = append(specs, sim.PlayerSpec{ID: ids[i], Name: "P" + string(rune('1'+i))})
	}
	w, err := sim.NewCoop(db, specs, "fire", 11, sim.Config{LI: -1})
	if err != nil {
		t.Fatal(err)
	}
	w.AutoDash = true
	return w
}

// localFrame drives the local player with a bot and makes the pending menu choice when it is ours.
func localFrame(s *Session, b *sim.Bot) Frame {
	w := s.W
	f := Quantise(b.Input(Tick))
	if w.Waiting() {
		switch {
		case w.State == sim.StateLevelUp && w.Chooser == s.Me:
			f.Cmds = append(f.Cmds, Cmd{Kind: CmdChoose, Arg: int8(w.BotPick())})
		case w.State == sim.StateShrine && w.Chooser == s.Me:
			f.Cmds = append(f.Cmds, Cmd{Kind: CmdBoon, Arg: 0})
		case w.State == sim.StateRoute && s.Me == 0:
			f.Cmds = append(f.Cmds, Cmd{Kind: CmdRoute, Arg: 0})
		}
	}
	return f
}

func runPeers(t *testing.T, n, lag, delay, updates int) []*Session {
	db := loadDB(t)
	net := newMemNet(n, lag)
	var ss []*Session
	var bots []*sim.Bot
	for i := 0; i < n; i++ {
		w := newWorld(t, db, n)
		ss = append(ss, New(w, n, i, delay, net.peers[i]))
		bots = append(bots, sim.NewBot(w, int64(100+i))) // the bot steering player i (only on peer i)
	}
	for u := 0; u < updates; u++ {
		net.clock = u
		for i, s := range ss {
			s.Update(localFrame(s, bots[i]))
		}
	}
	return ss
}

// Two peers, each steering their own player, stay in lockstep: identical hashes, no desync, and the
// run actually progresses (levels, menu picks travelling as commands).
func TestTwoPeersStayInSync(t *testing.T) {
	ss := runPeers(t, 2, 2, 3, 6000)
	a, b := ss[0], ss[1]
	if a.Desync != 0 || b.Desync != 0 {
		t.Fatalf("desync at ticks %d / %d", a.Desync, b.Desync)
	}
	if d := a.Tick - b.Tick; d < -MaxCatch || d > MaxCatch {
		t.Fatalf("peers drifted apart: %d vs %d", a.Tick, b.Tick)
	}
	// step the laggard to the same tick and compare the full state
	for b.Tick < a.Tick {
		b.recv()
		if !b.ready(b.Tick) {
			break
		}
		b.step()
	}
	if a.Tick == b.Tick && a.W.Hash() != b.W.Hash() {
		t.Fatalf("state differs at tick %d", a.Tick)
	}
	if a.W.T < 90 || a.W.XPS.Lv < 4 {
		t.Fatalf("run did not progress: t=%.1f lv=%d state=%v", a.W.T, a.W.XPS.Lv, a.W.State)
	}
	if a.Dropped != 0 || b.Dropped != 0 {
		t.Errorf("dropped messages: %d / %d", a.Dropped, b.Dropped)
	}
	t.Logf("tick %d t=%.1f lv=%d kills=%d stalled=%d/%d", a.Tick, a.W.T, a.W.XPS.Lv, a.W.Kills, a.Stalled, b.Stalled)
}

// Four peers with jittery delivery; input delay smaller than the lag so stalls happen and recover.
func TestFourPeersWithStalls(t *testing.T) {
	ss := runPeers(t, 4, 5, 3, 3000)
	for i, s := range ss {
		if s.Desync != 0 {
			t.Fatalf("peer %d desync at tick %d", i, s.Desync)
		}
		if s.Tick < 2000 {
			t.Fatalf("peer %d only reached tick %d", i, s.Tick)
		}
	}
	t.Logf("ticks %d %d %d %d stalled %d", ss[0].Tick, ss[1].Tick, ss[2].Tick, ss[3].Tick, ss[0].Stalled)
}

// A desync is reported: one peer's world is nudged and the next hash exchange catches it.
func TestDesyncDetected(t *testing.T) {
	db := loadDB(t)
	net := newMemNet(2, 1)
	var ss []*Session
	for i := 0; i < 2; i++ {
		ss = append(ss, New(newWorld(t, db, 2), 2, i, 2, net.peers[i]))
	}
	for u := 0; u < 400; u++ {
		net.clock = u
		if u == 150 {
			ss[1].W.Coins += 1
		}
		for _, s := range ss {
			s.Update(Frame{MX: 127})
		}
	}
	if ss[0].Desync == 0 || ss[1].Desync == 0 {
		t.Fatalf("desync not detected: %d %d", ss[0].Desync, ss[1].Desync)
	}
}

// The same world stepped twice with the same inputs hashes the same every tick (the sim has no
// hidden randomness: no map-order or wall-clock dependence).
func TestSimDeterministic(t *testing.T) {
	db := loadDB(t)
	a, b := newWorld(t, db, 3), newWorld(t, db, 3)
	ba, bb := sim.NewBot(a, 7), sim.NewBot(b, 7)
	for i := 0; i < 9000 && a.State != sim.StateDead; i++ {
		for _, w := range []*sim.World{a, b} {
			if w.Waiting() {
				w.BotDecide()
			}
		}
		a.SetInput(1, sim.Input{MoveX: .7, MoveY: .7})
		b.SetInput(1, sim.Input{MoveX: .7, MoveY: .7})
		a.Step(Tick, ba.Input(Tick))
		b.Step(Tick, bb.Input(Tick))
		if a.Hash() != b.Hash() {
			t.Fatalf("hashes differ at step %d (t=%.2f)", i, a.T)
		}
	}
	t.Logf("t=%.1f lv=%d kills=%d", a.T, a.XPS.Lv, a.Kills)
}

func TestWireRoundTrip(t *testing.T) {
	f := &Frame{MX: -127, MY: 5, Dash: true, Cmds: []Cmd{{CmdChoose, 2}, {CmdBanish, -1}}}
	kind, tk, p, g, _, err := decode(encodeInput(123456, 3, f))
	if err != nil || kind != msgInput || tk != 123456 || p != 3 || g.MX != -127 || g.MY != 5 || !g.Dash || len(g.Cmds) != 2 || g.Cmds[1].Arg != -1 {
		t.Fatalf("bad round trip: %v %d %d %+v %v", kind, tk, p, g, err)
	}
	kind, tk, p, _, h, err := decode(encodeHash(60, 1, 0xdeadbeefcafe))
	if err != nil || kind != msgHash || tk != 60 || p != 1 || h != 0xdeadbeefcafe {
		t.Fatal("bad hash round trip")
	}
	if _, _, _, _, _, err := decode([]byte{1, 2}); err == nil {
		t.Fatal("short message accepted")
	}
}
