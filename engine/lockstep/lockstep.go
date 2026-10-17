// Package lockstep runs a co-op World in deterministic lockstep between peers: every peer runs the
// same simulation from the same seed and only inputs travel over the wire. A tick advances once the
// inputs of every player for that tick are known; local inputs are scheduled Delay ticks ahead so
// they have time to arrive. Peers exchange state hashes every HashEvery ticks to catch desyncs.
package lockstep

import (
	"encoding/binary"
	"errors"
	"math"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

const (
	Tick      = 1.0 / 60 // simulation step
	HashEvery = 60       // ticks between state hashes
	MaxCatch  = 4        // ticks a peer may owe real time (caught up after a stall; beyond that time is lost)
	Window    = 600      // ticks of input kept ahead / behind (10 s)
)

// Cmd is a menu action that is part of the input stream (level-up pick, reroll, banish, boon, route).
type Cmd struct {
	Kind CmdKind
	Arg  int8
}

type CmdKind uint8

const (
	CmdChoose CmdKind = iota + 1
	CmdReroll
	CmdBanish
	CmdBoon
	CmdRoute
)

// Frame is one player's input for one tick. Movement is quantised so every peer feeds the sim the
// same floats; the local player uses the quantised values too.
type Frame struct {
	MX, MY int8
	Dash   bool
	Cmds   []Cmd
}

// Quantise turns a client input into a Frame.
func Quantise(in sim.Input) Frame {
	return Frame{MX: q8(in.MoveX), MY: q8(in.MoveY), Dash: in.Dash}
}

func q8(v float64) int8 { return int8(math.Round(math.Max(-1, math.Min(1, v)) * 127)) }

func (f Frame) input() sim.Input {
	return sim.Input{MoveX: float64(f.MX) / 127, MoveY: float64(f.MY) / 127, Dash: f.Dash}
}

// Transport carries messages to the other peers. Send broadcasts; Recv returns what arrived.
type Transport interface {
	Send(b []byte)
	Recv() [][]byte
}

// Session is one peer's view of a lockstep game.
type Session struct {
	W     *sim.World
	N, Me int // players, and which one is ours
	Delay int // input delay in ticks
	Tick  int // next tick to simulate
	T     Transport

	frames   map[int][]*Frame // tick -> per-player input (nil = not yet known)
	sentTo   int              // last tick we scheduled a local frame for
	hashes   map[int]uint64   // our hash per hashed tick
	theirs   map[int][]uint64 // remote hashes per tick
	Desync   int              // first tick where a hash disagreed, 0 = none
	Stalled  int              // ticks waited on remote input (stats)
	Dropped  int              // messages ignored (bad / out of window)
	Latest   []int            // per player: latest tick we have input for
	Err      error
	NoRemote bool // true when no remote input has ever arrived (shown as "waiting for players")
	gotAny   bool
	credit   int         // ticks owed to real time (one per Update, capped at MaxCatch)
	gone     map[int]int // player -> tick from which they are gone (no input expected)
}

// New starts a session over w (a co-op world with n players) for local player me.
func New(w *sim.World, n, me, delay int, t Transport) *Session {
	s := &Session{W: w, N: n, Me: me, Delay: delay, T: t, frames: map[int][]*Frame{}, hashes: map[int]uint64{}, theirs: map[int][]uint64{}, sentTo: -1, Latest: make([]int, n), gone: map[int]int{}}
	for i := range s.Latest {
		s.Latest[i] = -1
	}
	s.NoRemote = n > 1
	return s
}

func (s *Session) row(t int) []*Frame {
	r := s.frames[t]
	if r == nil {
		r = make([]*Frame, s.N)
		s.frames[t] = r
	}
	return r
}

// put records a frame; local frames are also sent.
func (s *Session) put(t, p int, f *Frame, send bool) {
	if t < s.Tick || p < 0 || p >= s.N {
		s.Dropped++
		return
	}
	r := s.row(t)
	if r[p] == nil {
		r[p] = f
		if t > s.Latest[p] {
			s.Latest[p] = t
		}
	}
	if send && s.T != nil {
		s.T.Send(encodeInput(t, p, f))
	}
}

// Update is called once per rendered frame with the local input. It schedules the local input,
// takes in remote messages and simulates every tick whose inputs are complete (at most MaxCatch).
// It returns how many ticks were simulated.
func (s *Session) Update(local Frame) int {
	// schedule the local input Delay ticks ahead. While we are stalled waiting for others, keep sending
	// one frame per update (up to a bound) so peers that are waiting on us can go on; the effective
	// input delay then stretches with the lag and shrinks again once the sim catches up.
	target := s.Tick + s.Delay
	if target < s.sentTo+1 {
		target = s.sentTo + 1
	}
	if lim := s.Tick + s.Delay + 2*MaxCatch; target > lim {
		target = lim
	}
	for t := s.sentTo + 1; t <= target; t++ {
		f := local
		s.put(t, s.Me, &f, true)
	}
	if target > s.sentTo {
		s.sentTo = target
	}
	s.recv()
	if s.credit < MaxCatch {
		s.credit++
	}
	n := 0
	for s.credit > 0 && s.ready(s.Tick) {
		s.step()
		s.credit--
		n++
	}
	if n == 0 && s.N > 1 {
		s.Stalled++
	}
	return n
}

func (s *Session) ready(t int) bool {
	r := s.frames[t]
	if r == nil {
		if len(s.gone) == s.N {
			return false
		}
		r = s.row(t)
	}
	for p, f := range r {
		if f == nil {
			if gt, ok := s.gone[p]; ok && t >= gt {
				r[p] = &Frame{}
				continue
			}
			return false
		}
	}
	return true
}

// Gone marks player p as disconnected from tick t on: their input is empty from then and the sim
// drops them at that tick. The host decides t (one past the last input it relayed) and tells the
// others, so every peer drops the player at the same tick.
func (s *Session) Gone(p, t int) {
	if p < 0 || p >= s.N || p == s.Me {
		return
	}
	if t < s.Tick {
		t = s.Tick
	}
	if old, ok := s.gone[p]; ok && old <= t {
		return
	}
	s.gone[p] = t
}

// DropPlayer (host) announces that player p is gone and schedules the drop locally.
func (s *Session) DropPlayer(p int) {
	t := s.Latest[p] + 1
	if t < s.Tick {
		t = s.Tick
	}
	s.Gone(p, t)
	if s.T != nil {
		s.T.Send(encodeGone(t, p))
	}
}

// IsGone reports whether player p has been dropped.
func (s *Session) IsGone(p int) bool { _, ok := s.gone[p]; return ok }

// WaitingOn lists the players whose input for the next tick is missing.
func (s *Session) WaitingOn() []int {
	var out []int
	r := s.frames[s.Tick]
	for p := 0; p < s.N; p++ {
		if _, g := s.gone[p]; g {
			continue
		}
		if r == nil || r[p] == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *Session) step() {
	r := s.frames[s.Tick]
	w := s.W
	// menu commands first (the sim pauses while someone chooses), in player order
	for p, f := range r {
		for _, c := range f.Cmds {
			apply(w, p, c)
		}
	}
	for p, f := range r {
		w.SetInput(p, f.input())
	}
	for p, gt := range s.gone {
		if gt == s.Tick {
			w.Drop(p)
		}
	}
	w.Step(Tick, r[0].input())
	delete(s.frames, s.Tick)
	s.Tick++
	if s.Tick%HashEvery == 0 {
		h := w.Hash()
		s.hashes[s.Tick] = h
		s.check(s.Tick, h, -1)
		if s.T != nil && s.N > 1 {
			s.T.Send(encodeHash(s.Tick, s.Me, h))
		}
		for t := range s.hashes {
			if t < s.Tick-Window {
				delete(s.hashes, t)
				delete(s.theirs, t)
			}
		}
	}
}

func apply(w *sim.World, p int, c Cmd) {
	switch c.Kind {
	case CmdRoute:
		w.ChooseRoute(int(c.Arg))
		return
	}
	if w.Chooser != p {
		return
	}
	switch c.Kind {
	case CmdChoose:
		w.Choose(int(c.Arg))
	case CmdReroll:
		w.Reroll()
	case CmdBanish:
		w.Banish(int(c.Arg))
	case CmdBoon:
		w.ChooseBoon(int(c.Arg))
	}
}

// check compares a hash for tick t against what we have (ours, or other peers').
func (s *Session) check(t int, h uint64, from int) {
	if from >= 0 {
		s.theirs[t] = append(s.theirs[t], h)
	}
	mine, ok := s.hashes[t]
	if !ok {
		return
	}
	for _, o := range s.theirs[t] {
		if o != mine && (s.Desync == 0 || t < s.Desync) {
			s.Desync = t
		}
	}
}

func (s *Session) recv() {
	if s.T == nil {
		return
	}
	for _, b := range s.T.Recv() {
		kind, t, p, f, h, err := decode(b)
		if err != nil {
			s.Dropped++
			continue
		}
		if p == s.Me {
			continue
		}
		switch kind {
		case msgInput:
			if t > s.Tick+Window {
				s.Dropped++
				continue
			}
			s.gotAny = true
			s.NoRemote = false
			s.put(t, p, f, false)
		case msgHash:
			s.check(t, h, p)
		case msgGone:
			s.Gone(p, t)
		}
	}
}

// ---- wire format ----

const (
	msgInput = 1
	msgHash  = 2
	msgGone  = 3
)

// gone: [3][tick u32][player u8]
func encodeGone(t, p int) []byte {
	b := make([]byte, 0, 6)
	b = append(b, msgGone)
	b = binary.LittleEndian.AppendUint32(b, uint32(t))
	return append(b, byte(p))
}

// input: [1][tick u32][player u8][mx i8][my i8][dash u8][ncmd u8]([kind u8][arg i8])*
func encodeInput(t, p int, f *Frame) []byte {
	b := make([]byte, 0, 10+2*len(f.Cmds))
	b = append(b, msgInput)
	b = binary.LittleEndian.AppendUint32(b, uint32(t))
	b = append(b, byte(p), byte(f.MX), byte(f.MY), b2u(f.Dash), byte(len(f.Cmds)))
	for _, c := range f.Cmds {
		b = append(b, byte(c.Kind), byte(c.Arg))
	}
	return b
}

// hash: [2][tick u32][player u8][hash u64]
func encodeHash(t, p int, h uint64) []byte {
	b := make([]byte, 0, 14)
	b = append(b, msgHash)
	b = binary.LittleEndian.AppendUint32(b, uint32(t))
	b = append(b, byte(p))
	return binary.LittleEndian.AppendUint64(b, h)
}

var errBad = errors.New("bad message")

func decode(b []byte) (kind, t, p int, f *Frame, h uint64, err error) {
	if len(b) < 6 {
		return 0, 0, 0, nil, 0, errBad
	}
	kind, t, p = int(b[0]), int(binary.LittleEndian.Uint32(b[1:5])), int(b[5])
	switch kind {
	case msgInput:
		if len(b) < 10 {
			return 0, 0, 0, nil, 0, errBad
		}
		f = &Frame{MX: int8(b[6]), MY: int8(b[7]), Dash: b[8] != 0}
		n := int(b[9])
		if len(b) < 10+2*n {
			return 0, 0, 0, nil, 0, errBad
		}
		for i := 0; i < n; i++ {
			f.Cmds = append(f.Cmds, Cmd{Kind: CmdKind(b[10+2*i]), Arg: int8(b[11+2*i])})
		}
	case msgHash:
		if len(b) < 14 {
			return 0, 0, 0, nil, 0, errBad
		}
		h = binary.LittleEndian.Uint64(b[6:14])
	case msgGone:
	default:
		return 0, 0, 0, nil, 0, errBad
	}
	return
}

func b2u(v bool) byte {
	if v {
		return 1
	}
	return 0
}
