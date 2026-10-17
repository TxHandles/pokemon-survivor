// Package net is online co-op: a room (lobby) of up to four players joined by room code, and a
// lockstep transport over the connections. The host accepts guests and relays their messages, so
// every peer sees every input; the host also owns the lobby state and starts the run.
//
// Room logic is platform-independent and tested with in-memory connections; the browser build
// supplies WebRTC connections signalled through the same Firestore documents the web game uses
// (lobbies/{code}/join/{uid}).
package net

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Protocol is bumped when the wire format or the simulation changes incompatibly.
const Protocol = 9 // 4: enemy speed classes, swap pick

const MaxPlayers = 4

// Conn is one connection to a peer. Text carries lobby JSON, binary carries lockstep frames.
type Conn interface {
	Send(b []byte)
	SendText(s string)
	Recv() []Msg
	Open() bool
	Closed() bool
	Close()
}

type Msg struct {
	Bin  []byte
	Text string
}

// Peer is a guest that connected to the host.
type Peer struct {
	UID, Name string
	Conn      Conn
}

// Net makes connections for a platform (WebRTC, signalled through Firestore).
type Net interface {
	Available() (ok bool, why string)
	// Login resolves who we are (the signed-in user, or an anonymous one); cb may run on another goroutine.
	Login(cb func(uid, name, err string))
	// SetRoomOpen marks a hosted room open / in a run (late joiners get a clear message).
	SetRoomOpen(code string, open bool)
	// Host opens a room; onCode gets the room code, onPeer each guest that connects.
	Host(name string, onCode func(code string), onPeer func(Peer), onErr func(string)) (stop func())
	// Join connects to a host's room.
	Join(code, name string, onConn func(Conn), onErr func(string)) (stop func())
}

// LobbyPlayer is one seat in the room as everyone sees it.
type LobbyPlayer struct {
	UID   string         `json:"uid"`
	Name  string         `json:"name"`
	Spec  sim.PlayerSpec `json:"spec"`
	Ready bool           `json:"ready"`
	Dup   bool           `json:"dup,omitempty"` // same Pokémon as another player (must change)
	Host  bool           `json:"host,omitempty"`
	Gone  bool           `json:"gone,omitempty"` // disconnected during the run
}

// StartMsg is what the host sends to begin the run; it is everything needed to build an identical World.
type StartMsg struct {
	Me      int              `json:"me"`
	Seed    int64            `json:"seed"`
	Biome   string           `json:"bi"`
	LI      int              `json:"li"`
	Tier    int              `json:"tier"`
	Mods    []string         `json:"mods"`
	Stage   int              `json:"stage"`
	Lock    bool             `json:"lock"`
	Delay   int              `json:"delay"`
	Players []sim.PlayerSpec `json:"players"`
}

type wire struct {
	T       string          `json:"t"`
	Name    string          `json:"name,omitempty"`
	UID     string          `json:"uid,omitempty"`
	Proto   int             `json:"proto,omitempty"`
	Data    string          `json:"data,omitempty"` // content hash
	Spec    *sim.PlayerSpec `json:"spec,omitempty"`
	Ready   bool            `json:"ready,omitempty"`
	Players []LobbyPlayer   `json:"players,omitempty"`
	Code    string          `json:"code,omitempty"`
	Start   *StartMsg       `json:"start,omitempty"`
	Msg     string          `json:"msg,omitempty"`
	Label   string          `json:"label,omitempty"`
	Ext     map[string]any  `json:"ext,omitempty"`
}

type peer struct {
	Peer
	idx   int // player index once in the lobby, -1 before hello
	hello bool
	last  time.Time
}

// Room is this peer's view of an online lobby and, once started, the lockstep transport.
type Room struct {
	Host    bool
	Code    string
	Status  string // one line for the UI
	Err     string // fatal: the room is gone
	Me      int    // our player index
	MeUID   string
	Players []LobbyPlayer
	Label   string // the host's run setup, as text
	Started bool
	Start   *StartMsg // set when the run starts (both sides)
	Ended   bool      // the host returned everyone to the lobby
	Left    bool
	DataID  string // content hash; peers must match
	// OnGone (host) is called with the player index of a guest that disconnected during a run.
	OnGone func(idx int)

	net    Net
	stop   func()
	peers  []*peer // host: guests
	hc     Conn    // guest: the host
	inbox  [][]byte
	mu     sync.Mutex
	events []func()
	name   string
	spec   sim.PlayerSpec
	now    func() time.Time
	joinT  time.Time
}

func newRoom(n Net, name string, spec sim.PlayerSpec, dataID string) *Room {
	return &Room{net: n, name: name, spec: spec, Me: -1, DataID: dataID, now: time.Now}
}

// HostRoom opens a room. uid names us in the player list.
func HostRoom(n Net, uid, name string, spec sim.PlayerSpec, dataID string) *Room {
	r := newRoom(n, name, spec, dataID)
	r.Host = true
	r.MeUID = uid
	r.Me = 0
	r.Status = "Opening a room…"
	r.Players = []LobbyPlayer{{UID: uid, Name: name, Spec: spec, Ready: true, Host: true}}
	r.stop = n.Host(name, func(code string) {
		r.post(func() { r.Code = code; r.Status = "Room " + code + " · waiting for players" })
	}, func(p Peer) {
		r.post(func() { r.addPeer(p) })
	}, func(e string) {
		r.post(func() { r.fail(e) })
	})
	return r
}

// JoinRoom connects to a room by code.
func JoinRoom(n Net, code, uid, name string, spec sim.PlayerSpec, dataID string) *Room {
	r := newRoom(n, name, spec, dataID)
	r.Code = code
	r.MeUID = uid
	r.Status = "Connecting to " + code + "…"
	r.joinT = r.now()
	r.stop = n.Join(code, name, func(c Conn) {
		r.post(func() {
			r.hc = c
			r.Status = "Connected · joining the lobby…"
			r.sendText(c, wire{T: "hello", Name: name, UID: uid, Proto: Protocol, Data: dataID, Spec: &spec})
		})
	}, func(e string) {
		r.post(func() { r.fail(e) })
	})
	return r
}

func (r *Room) post(f func()) {
	r.mu.Lock()
	r.events = append(r.events, f)
	r.mu.Unlock()
}

func (r *Room) fail(e string) {
	if r.Err == "" {
		r.Err = e
	}
	r.Status = e
}

func (r *Room) sendText(c Conn, m wire) {
	if c == nil {
		return
	}
	b, _ := json.Marshal(m)
	c.SendText(string(b))
}

func (r *Room) broadcastText(m wire) {
	for _, p := range r.peers {
		if p.hello {
			r.sendText(p.Conn, m)
		}
	}
}

// ---- host side ----

func (r *Room) addPeer(p Peer) {
	r.peers = append(r.peers, &peer{Peer: p, idx: -1, last: r.now()})
}

func (r *Room) dropPeer(i int, why string) {
	p := r.peers[i]
	p.Conn.Close()
	r.peers = append(r.peers[:i], r.peers[i+1:]...)
	if p.idx < 0 {
		return
	}
	if r.Started { // indexes are fixed during a run: mark the seat gone and tell the session
		if p.idx < len(r.Players) {
			r.Players[p.idx].Gone = true
		}
		if r.OnGone != nil {
			r.OnGone(p.idx)
		}
		r.Status = p.Name + " disconnected"
		return
	}
	for j := range r.Players {
		if r.Players[j].UID == p.UID {
			r.Players = append(r.Players[:j], r.Players[j+1:]...)
			break
		}
	}
	r.reindex()
	r.sendLobby()
	r.Status = p.Name + " left" + why
}

func (r *Room) reindex() {
	for i := range r.Players {
		for _, p := range r.peers {
			if p.UID == r.Players[i].UID {
				p.idx = i
			}
		}
		if r.Players[i].UID == r.MeUID {
			r.Me = i
		}
	}
}

func (r *Room) hostMsg(pi int, m wire) {
	p := r.peers[pi]
	switch m.T {
	case "hello":
		if p.hello {
			return
		}
		switch {
		case m.Proto != Protocol:
			r.sendText(p.Conn, wire{T: "err", Msg: "Your game version is different from the host's. Update and try again."})
			r.dropPeer(pi, "")
			return
		case m.Data != r.DataID:
			r.sendText(p.Conn, wire{T: "err", Msg: "Your game data differs from the host's. Update and try again."})
			r.dropPeer(pi, "")
			return
		case r.Started || len(r.Players) >= MaxPlayers:
			r.sendText(p.Conn, wire{T: "err", Msg: "The room is full or already in a run."})
			r.dropPeer(pi, "")
			return
		}
		p.hello = true
		p.Name = nonEmpty(m.Name, p.Name, "Trainer")
		if m.UID != "" {
			p.UID = m.UID
		}
		spec := sim.PlayerSpec{}
		if m.Spec != nil {
			spec = *m.Spec
		}
		spec.Kind = "net"
		spec.Name = p.Name
		r.Players = append(r.Players, LobbyPlayer{UID: p.UID, Name: p.Name, Spec: spec})
		r.reindex()
		r.Status = p.Name + " joined"
		r.sendLobby()
	case "pick":
		if !p.hello || p.idx < 0 {
			return
		}
		lp := &r.Players[p.idx]
		if m.Spec != nil {
			lp.Spec = *m.Spec
			lp.Spec.Kind = "net"
			lp.Spec.Name = p.Name
		}
		lp.Ready = m.Ready
		r.sendLobby()
	case "bye":
		r.dropPeer(pi, "")
	}
}

func (r *Room) markDups() {
	for i := range r.Players {
		r.Players[i].Dup = false
	}
	for i := range r.Players {
		for j := 0; j < i; j++ {
			if r.Players[i].Spec.ID == r.Players[j].Spec.ID {
				r.Players[i].Dup = true
				r.Players[i].Ready = false
			}
		}
	}
}

func (r *Room) sendLobby() {
	r.markDups()
	r.broadcastText(wire{T: "lobby", Code: r.Code, Players: r.Players, Label: r.Label})
}

// SetMe updates our own seat (Pokémon / ability / ready) and tells the others.
func (r *Room) SetMe(spec sim.PlayerSpec, ready bool) {
	r.spec = spec
	if r.Host {
		spec.Kind = "local"
		spec.Name = r.name
		r.Players[0].Spec = spec
		r.Players[0].Ready = true
		r.sendLobby()
		return
	}
	if r.Me >= 0 && r.Me < len(r.Players) {
		r.Players[r.Me].Spec = spec
		r.Players[r.Me].Ready = ready
	}
	r.sendText(r.hc, wire{T: "pick", Spec: &spec, Ready: ready})
}

// SetLabel sets the run-setup text shown to guests (host).
func (r *Room) SetLabel(s string) {
	if r.Label != s {
		r.Label = s
		if r.Host {
			r.sendLobby()
		}
	}
}

// CanStart says whether the host may start now.
func (r *Room) CanStart() (bool, string) {
	if !r.Host {
		return false, "Only the host can start."
	}
	if len(r.Players) < 2 {
		return false, "Waiting for at least one more player."
	}
	r.markDups()
	for _, p := range r.Players {
		if p.Dup {
			return false, p.Name + " has the same Pokémon as someone else."
		}
		if !p.Ready {
			return false, "Waiting for " + p.Name + " to ready up."
		}
	}
	return true, ""
}

// StartRun (host) sends the start message to everyone and marks the room started.
func (r *Room) StartRun(seed int64, biome string, li, tier int, mods []string, stage int, lock bool, delay int) *StartMsg {
	ok, why := r.CanStart()
	if !ok {
		r.Status = why
		return nil
	}
	var specs []sim.PlayerSpec
	for _, p := range r.Players {
		s := p.Spec
		s.Name = p.Name
		s.Kind = "net"
		if p.UID == r.MeUID {
			s.Kind = "local"
		}
		specs = append(specs, s)
	}
	base := StartMsg{Seed: seed, Biome: biome, LI: li, Tier: tier, Mods: mods, Stage: stage, Lock: lock, Delay: delay, Players: specs}
	for _, p := range r.peers {
		if p.idx < 0 {
			continue
		}
		m := base
		m.Me = p.idx
		r.sendText(p.Conn, wire{T: "start", Start: &m})
	}
	me := base
	me.Me = r.Me
	r.Started = true
	r.Ended = false
	r.Start = &me
	r.inbox = nil
	r.Status = "In a run"
	return &me
}

// EndRun (host) returns everyone to the lobby after a run.
func (r *Room) EndRun() {
	if !r.Host {
		return
	}
	r.Started = false
	r.Ended = true
	r.Start = nil
	keep := r.Players[:0]
	for _, p := range r.Players {
		if !p.Gone {
			if !p.Host {
				p.Ready = false
			}
			keep = append(keep, p)
		}
	}
	r.Players = keep
	r.reindex()
	r.broadcastText(wire{T: "end"})
	r.sendLobby()
	r.Status = "Room " + r.Code + " · back in the lobby"
}

// ---- guest side ----

func (r *Room) guestMsg(m wire) {
	switch m.T {
	case "lobby":
		r.Players = m.Players
		r.Code = nonEmpty(m.Code, r.Code)
		r.Label = m.Label
		r.Me = -1
		for i, p := range r.Players {
			if p.UID == r.MeUID {
				r.Me = i
			}
		}
		if !r.Started {
			r.Status = "Room " + r.Code + " · " + fmt.Sprint(len(r.Players)) + " players"
		}
	case "start":
		if m.Start != nil {
			r.Started = true
			r.Ended = false
			r.Start = m.Start
			r.Me = m.Start.Me
			r.inbox = nil
			r.Status = "In a run"
		}
	case "end":
		r.Started = false
		r.Ended = true
		r.Start = nil
		r.Status = "Back in the lobby"
	case "err":
		r.fail(m.Msg)
	case "bye":
		r.fail("The host closed the room.")
	}
}

// ---- both ----

// Update handles connection events and lobby messages; call it once per frame before the lockstep session.
func (r *Room) Update() {
	r.mu.Lock()
	ev := r.events
	r.events = nil
	r.mu.Unlock()
	for _, f := range ev {
		f()
	}
	if r.Left {
		return
	}
	if r.Host {
		for i := 0; i < len(r.peers); i++ {
			p := r.peers[i]
			if p.Conn.Closed() {
				r.dropPeer(i, " (connection lost)")
				i--
				continue
			}
			for _, m := range p.Conn.Recv() {
				p.last = r.now()
				if m.Bin != nil {
					if p.idx >= 0 && r.Started {
						r.inbox = append(r.inbox, m.Bin)
						for _, o := range r.peers { // relay to the others
							if o != p && o.idx >= 0 {
								o.Conn.Send(m.Bin)
							}
						}
					}
					continue
				}
				var w wire
				if json.Unmarshal([]byte(m.Text), &w) == nil {
					r.hostMsg(i, w)
					if i >= len(r.peers) || r.peers[i] != p {
						i--
						break
					}
				}
			}
		}
		return
	}
	if r.hc == nil {
		if r.now().Sub(r.joinT) > 20*time.Second && r.Err == "" {
			r.fail("Could not reach the host. Check the code; their network may block direct connections.")
		}
		return
	}
	if r.hc.Closed() {
		r.fail("Lost the connection to the host.")
		return
	}
	for _, m := range r.hc.Recv() {
		if m.Bin != nil {
			if r.Started {
				r.inbox = append(r.inbox, m.Bin)
			}
			continue
		}
		var w wire
		if json.Unmarshal([]byte(m.Text), &w) == nil {
			r.guestMsg(w)
		}
	}
}

// Leave closes everything.
func (r *Room) Leave() {
	if r.Left {
		return
	}
	r.Left = true
	if r.Host {
		r.broadcastText(wire{T: "bye"})
		for _, p := range r.peers {
			p.Conn.Close()
		}
		r.peers = nil
	} else if r.hc != nil {
		r.sendText(r.hc, wire{T: "bye"})
		r.hc.Close()
	}
	if r.stop != nil {
		r.stop()
	}
}

// Connected reports whether we are in the lobby (guest: handshake done).
func (r *Room) Connected() bool {
	if r.Host {
		return r.Code != ""
	}
	return r.hc != nil && r.Me >= 0
}

// N is the number of players in the run (or lobby).
func (r *Room) N() int { return len(r.Players) }

// ---- lockstep.Transport ----

func (r *Room) Send(b []byte) {
	if r.Host {
		for _, p := range r.peers {
			if p.idx >= 0 {
				p.Conn.Send(b)
			}
		}
	} else if r.hc != nil {
		r.hc.Send(b)
	}
}

func (r *Room) Recv() [][]byte {
	out := r.inbox
	r.inbox = nil
	return out
}

func nonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}
