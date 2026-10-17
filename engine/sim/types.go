// Package sim is the game simulation: pure logic with no rendering, so it runs the same in the
// Windows/web client and in the headless bot runner. Formulas are ported from the web version (src/game.html).
package sim

import (
	"math/rand"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

type State int

const (
	StatePlay State = iota
	StateLevelUp
	StateDead
	StateRoute  // route map open: choose the next biome (World.Route)
	StateShrine // shrine open: choose a boon (World.Boons)
)

// Input is one frame of player intent.
type Input struct {
	MoveX, MoveY float64 // -1..1 (already normalised by the client)
	Dash         bool    // pressed this frame (Shift / RB / R1)
}

// Q holds a move's merged parameters (shape defaults + moves.json params + tier multipliers).
type Q struct {
	M  map[string]any
	st *Status
}

type Status struct {
	K      string
	P, Dur float64
}

func (q *Q) F(k string, def float64) float64 {
	if q == nil {
		return def
	}
	switch v := q.M[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case bool:
		if v {
			return 1
		}
		return 0
	}
	return def
}

func (q *Q) S(k string) string {
	if q == nil {
		return ""
	}
	s, _ := q.M[k].(string)
	return s
}

func (q *Q) St() *Status {
	if q == nil {
		return nil
	}
	if q.st == nil {
		l, ok := q.M["st"].([]any)
		if !ok || len(l) < 3 {
			return nil
		}
		k, _ := l[0].(string)
		p, _ := l[1].(float64)
		d, _ := l[2].(float64)
		q.st = &Status{k, p, d}
	}
	return q.st
}

// Slot is an equipped move.
type Slot struct {
	ID         string
	Move       *content.Move
	Kind, Type string
	Lv         int
	Evo        bool
	CD         float64 // time until it fires again
	Q          *Q
	metroN     int
	metroT     float64
	orbA       float64
	Owner      int // player index
}

type DashState struct {
	VX, VY, T, W, D float64
	Type            string
	Slot            *Slot
	Cap, Phase      bool
	Hit             map[*Enemy]bool
}

type Player struct {
	ID, Name          string
	Types             []string
	Stats             content.Stats
	X, Y              float64
	VX, VY            float64
	HP, MHP, Sp       float64
	Lv                int
	XP, NX            float64
	Inv, HurtT        float64
	FX, FY            float64 // facing
	Moving, Dead      bool
	Still             float64
	Dash              *DashState
	DQ                float64 // time the dash was requested (-1 = none)
	adA               *float64
	SpdT, SashT       float64
	AtkT              float64 // last time a move fired (for the attack animation)
	Bat, Bel          float64 // battery (-25% cooldowns) / strength belt (+25% damage) timers
	WebT              float64 // webbed: slowed
	WX, WY            float64 // wind push
	gw                float64
	Hazard            string  // hazard under the player this frame
	Pm, Pbl, Phs, Pgr float64 // shrine boons / pacts (seconds left)
}

type Enemy struct {
	PokeID        string
	Type          string
	X, Y, R       float64
	HP, MHP       float64
	Sp, Dm        float64
	Big, Boss     bool
	Elite, Dead   bool
	KX, KY, Flash float64
	Burn, BurnD   float64
	burnK         float64
	Psn, PsnD     float64
	psnK          float64
	Slow, Frz     float64
	Stun, Root    float64
	Fear, Conf    float64
	Charm, Thaw   float64
	zt, cx, th    float64
	orbT          [8]float64
	stk, dtr, ds  float64
	FaceL         bool // facing left (for the sprite)
	Seed          float64
	// roles (shooter, healer, burrower, phaser, charger, webber) and elites
	Rk                 string
	Ug                 bool // underground / phased out: can't be hit or touch you
	rkt, rkw, btv, bwt float64
	bs, ps             int
	cdh, cwu, cang     float64
	pf, pt2, dx2, dy2  float64
	lg, sd             float64
	pmk                bool
	Sh, ShMax          float64 // shield
	Mini, Arm, Vamp    bool
	Expl, Split, Sumn  bool
	Af                 []string
	sut                float64
	ao                 float64
	aoSet              bool
	// bosses
	BD                  *BossDef
	Ph                  int
	K                   string
	at, wu, chd, ca, tt float64
	trl                 bool
	c3                  int
	pb                  float64 // Perish Body countdown
	aslT, aslS          float64 // ability slow aura
	Cursed              bool    // cursed-chest guardian
	Sp2                 string  // difficulty special: spit chg bomb shld wisp
	st, cc              float64
	guard               *Center
}

type Proj struct {
	X, Y, VX, VY, L, R, D, Ex, Tr, Ls float64
	P                                 int
	Type                              string
	Sk                                bool
	Tg                                *Enemy
	Hit                               []*Enemy
	Bm                                *boom
	Slot                              *Slot
}

type boom struct {
	t, out, sp float64
	back       bool
}

// FX is a visual effect; rings also deal damage as they expand.
type FX struct {
	Kind            string // ring arc rect beam bt tele puff sky bolt txt pop
	X, Y, R, MR, TW float64
	A, Wd, Ln       float64 // angle, width (half-angle for arcs), length
	Life, Life0     float64
	Type            string
	D, KB           float64
	Fat             bool
	Sk              string
	Du, Dm          float64
	Rt              bool
	Hit             map[*Enemy]bool
	Slot            *Slot
	Text, Col       string
	AX, AY, BX, BY  float64
	Enemy           bool // telegraph of an enemy attack (the bot avoids these)
}

type Zone struct {
	X, Y, R, L, D, Tk float64
	Type              string
	Ev, Cap           bool
	Pull, Pr          float64
	Mine              bool
	Arm, Age, Br, Md  float64
	Slot              *Slot
	Life0             float64
}

type Gem struct {
	X, Y, V float64
	Mag     bool    // flying to the player (magnet)
	C       float64 // coin value (coins instead of XP)
	Tk, Ed  bool    // ticket (Ed: dropped by an elite)
	ms      float64
}

type Orb struct {
	X, Y, R float64
	Type    string
}

type delay struct {
	t  float64
	fn func()
}

// Option is a level-up card.
type Option struct {
	Move  bool   // move (true) or held item (false)
	ID    string // move id or item id
	New   bool
	Evo   bool
	Swap  string // held item it replaces
	Level int    // current level (0 = new)
}

// World is one run.
type World struct {
	DB           *content.DB
	P            *Player
	E            []*Enemy
	PR           []*Proj
	FX           []*FX
	ZN           []*Zone
	Gems         []*Gem
	Orbs         []Orb
	Slots        []*Slot
	Held         map[string]int
	HeldOrder    []string
	DL           []*delay
	Opts         []Option
	State        State
	T            float64
	Kills        int
	Bosses       int
	Won          bool
	Biome        string
	DMG          map[string]float64
	Shake        float64
	ViewW, ViewH float64
	MoveSlots    int
	HeldSlots    int
	AutoDash     bool // bots: dash at the nearest enemy when ready
	// enemy leash: enemies further than spawn ring + LeashFar are moved back ahead of the player
	LeashFar, LeashBias, LeashSpread float64
	Seed                             int64
	rng                              *rand.Rand
	spawnT                           float64
	nextBoss                         float64
	nextSwarm                        float64
	curSlot                          *Slot
	obs                              map[[2]int][]Obstacle
	relocT                           float64
	Msg                              string
	MsgT                             float64
	EB                               []*EBullet
	HZ                               []*Hazard
	Items                            []*Pickup
	Gust                             *GustState
	pools                            map[[2]int][]Pool
	bioT, elT                        float64
	mbT, candyT                      float64
	spawnHold                        float64
	magnetAll                        bool
	noDrop                           bool
	PS                               *PS // active ability
	ab                               abState
	Ability                          int // 0 starting, 1 hidden
	Coins                            float64
	Endless                          bool
	ET                               float64 // when endless mode began
	Route                            []RouteOpt
	RouteChoices                     int
	LI                               int // location index in the current biome
	BiomeN                           int // biomes visited (the client reloads the floor when it changes)
	routeT                           float64
	chestT, shT                      float64
	MC                               []*MapChest
	Chests                           []Chest
	Boons                            []Boon
	bossesSpawned                    int
	Cfg                              Config
	Run                              Run
	Choices                          int
	Rerolls, Banishes                int
	Ban                              map[string]bool
	Tickets                          int
	Visited                          []string // biomes whose boss was beaten this run
	Notes                            []string // unlock messages from the meta layer (shown by the client)
	Learned, Evolved                 map[string]bool
	Capstone                         map[string]bool
	Stats                            RunStats
	Events                           []string // sound cues since the client last drained them
	// co-op
	PL  []*PlayerCtx
	CUR int
	cur *PlayerCtx
	XPS struct {
		XP, NX float64
		Lv     int
	}
	PCN     *Center
	Chooser int
}

func hyp(x, y float64) float64 { return phyp(x, y) }
func dist(ax, ay, bx, by float64) float64 {
	return phyp(ax-bx, ay-by)
}
func (w *World) rnd(a, b float64) float64 { return a + w.rng.Float64()*(b-a) }

func (w *World) addFX(f *FX) {
	if f.Life0 == 0 {
		f.Life0 = f.Life
	}
	if f.Kind == "txt" && len(w.FX) > 260 {
		return
	}
	w.FX = append(w.FX, f)
}

func (w *World) later(t float64, fn func()) { w.DL = append(w.DL, &delay{t, fn}) }

// RunStats counts things the meta layer records at the end of a run.
type RunStats struct {
	Elites, Minis, EliteTk, Cursed, Curses int
}

// sfx queues a sound cue for the client.
func (w *World) sfx(n string) {
	if len(w.Events) < 64 {
		w.Events = append(w.Events, n)
	}
}

// DrainEvents returns and clears the queued sound cues.
func (w *World) DrainEvents() []string {
	e := w.Events
	w.Events = nil
	return e
}
