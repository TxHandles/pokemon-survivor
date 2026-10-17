package sim

import (
	"sort"
	"strings"
)

// Damage sources and the "what knocked you out" log (web esrc / dlog / deathHTML data).

// DmgSrc names what hurt the player.
type DmgSrc struct {
	Name string
	Kind string // contact, shot, area, terrain
	Type string // damage type ("" = none)
	Boss bool
}

// DmgHit is one logged hit.
type DmgHit struct {
	T, D float64
	S    *DmgSrc
}

// DmgAgg sums the hits of one source.
type DmgAgg struct {
	S    *DmgSrc
	D    float64
	Hits int
}

type dmgLog struct {
	DLog []DmgHit
	DTot map[string]*DmgAgg
	KB   *DmgHit // the knockout blow
}

var (
	srcLava     = &DmgSrc{Name: "Lava pool", Kind: "terrain", Type: "fire"}
	srcToxic    = &DmgSrc{Name: "Toxic pool", Kind: "terrain", Type: "poison"}
	srcStorm    = &DmgSrc{Name: "Lightning strike", Kind: "area", Type: "electric"}
	srcRockfall = &DmgSrc{Name: "Falling boulder", Kind: "area", Type: "rock"}
	srcExpl     = &DmgSrc{Name: "Explosive elite blast", Kind: "area"}
	srcUnknown  = &DmgSrc{Name: "Unknown", Kind: "shot"}
)

// KTip: advice for the kind of damage that did the most harm (web KTIP).
var KTip = map[string]string{
	"contact": "Most of that damage came from monsters touching you. Keep moving through gaps in the swarm, and Armor, Vitality or crowd-control attacks help.",
	"shot":    "Projectiles did the damage. They fly in straight lines, so strafe sideways instead of backing straight away.",
	"area":    "Warning circles show where attacks will land. Step out before the circle fills.",
	"terrain": "The biome terrain hurt you. Steer around the pools, or bring a type that resists this biome.",
}

func capWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// esrc names enemy e's attack (contact = touching it).
func (w *World) esrc(e *Enemy, contact bool) *DmgSrc {
	if contact && e.srcC != nil {
		return e.srcC
	}
	if !contact && e.srcA != nil {
		return e.srcA
	}
	nm := capWord(e.Type) + " monster"
	if p := w.DB.ByID[e.PokeID]; p != nil {
		nm = p.Name
	}
	s := &DmgSrc{Type: e.Type}
	add := func(a, b string) string {
		if contact {
			return a
		}
		return b
	}
	switch {
	case e.Boss && e.BD != nil:
		s.Name, s.Kind, s.Boss = e.BD.Name+add(" (contact)", " · attack"), add("contact", "area"), true
	case e.Elite:
		pre := "Elite "
		if e.Mini {
			pre = "Mini-boss "
		}
		s.Name, s.Kind = pre+nm+add("", " attack"), add("contact", "area")
	case e.Rk == "shoot":
		s.Name, s.Kind = nm+add("", " · shot"), add("contact", "shot")
	case e.Rk == "charge":
		s.Name, s.Kind = nm+add(" · charge", ""), "contact"
	case e.Rk == "web":
		s.Name, s.Kind = nm+add("", " · web"), add("contact", "shot")
	case e.Rk == "burrow":
		s.Name, s.Kind = nm+add("", " · ambush"), add("contact", "area")
	case e.Rk == "phase":
		s.Name, s.Kind = nm+add("", " · phase"), "contact"
	case e.Sp2 == "bomb":
		s.Name, s.Kind = nm+add("", " · explosion"), add("contact", "area")
	default:
		s.Name, s.Kind = nm+add("", " attack"), add("contact", "shot")
	}
	if contact {
		e.srcC = s
	} else {
		e.srcA = s
	}
	return s
}

// dlog records damage d from source s against the current player.
func (w *World) dlog(d float64, s *DmgSrc) {
	if !(d > 0) {
		return
	}
	if s == nil {
		s = srcUnknown
	}
	L := &w.P.dl
	L.DLog = append(L.DLog, DmgHit{w.T, d, s})
	if L.DTot == nil {
		L.DTot = map[string]*DmgAgg{}
	}
	a := L.DTot[s.Name]
	if a == nil {
		a = &DmgAgg{S: s}
		L.DTot[s.Name] = a
	}
	a.D += d
	a.Hits++
	if len(L.DLog) > 500 {
		k := L.DLog[:0]
		for _, h := range L.DLog {
			if h.T > w.T-30 {
				k = append(k, h)
			}
		}
		L.DLog = k
	}
}

// Recap is the "what knocked you out" summary of player p.
type Recap struct {
	KB          DmgHit
	Last10      []DmgAgg // top sources of the last 10 seconds
	Last10Total float64
	Run         []DmgAgg // top sources of the whole run
	RunTotal    float64
	Tip         string
}

// DeathRecap returns the knockout summary of player p (nil when nothing hurt them).
func (w *World) DeathRecap(p *Player) *Recap {
	L := &p.dl
	if L.KB == nil && len(L.DLog) == 0 {
		return nil
	}
	r := &Recap{}
	if L.KB != nil {
		r.KB = *L.KB
	} else {
		h := L.DLog[len(L.DLog)-1]
		r.KB = h
	}
	agg := map[string]*DmgAgg{}
	var order []string
	kinds := map[string]float64{}
	for _, h := range L.DLog {
		if h.T <= r.KB.T-10 {
			continue
		}
		a := agg[h.S.Name]
		if a == nil {
			a = &DmgAgg{S: h.S}
			agg[h.S.Name] = a
			order = append(order, h.S.Name)
		}
		a.D += h.D
		a.Hits++
		r.Last10Total += h.D
		kinds[h.S.Kind] += h.D
	}
	for _, k := range order {
		r.Last10 = append(r.Last10, *agg[k])
	}
	sort.SliceStable(r.Last10, func(i, j int) bool { return r.Last10[i].D > r.Last10[j].D })
	if len(r.Last10) > 5 {
		r.Last10 = r.Last10[:5]
	}
	names := make([]string, 0, len(L.DTot))
	for k := range L.DTot {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		r.Run = append(r.Run, *L.DTot[k])
		r.RunTotal += L.DTot[k].D
	}
	sort.SliceStable(r.Run, func(i, j int) bool { return r.Run[i].D > r.Run[j].D })
	if len(r.Run) > 5 {
		r.Run = r.Run[:5]
	}
	top, tv := "", 0.0
	for _, k := range []string{"contact", "shot", "area", "terrain"} {
		if kinds[k] > tv {
			top, tv = k, kinds[k]
		}
	}
	if r.KB.S != nil && r.KB.S.Boss {
		r.Tip = "Bosses hit harder at each phase (66% and 33% HP). "
	}
	r.Tip += KTip[top]
	return r
}

// TestKO knocks the current player out with a hit from the nearest enemy (screenshot tests).
func (w *World) TestKO() {
	var near *Enemy
	bd := 1e18
	for _, e := range w.E {
		if d := dist(e.X, e.Y, w.P.X, w.P.Y); !e.Dead && d < bd {
			near, bd = e, d
		}
	}
	for i := 0; i < 5 && !w.P.Dead && w.P.HP > 0; i++ {
		s := hurtSrc{src: srcUnknown}
		if near != nil {
			s = hurtSrc{t: near.Type, contact: true, src: w.esrc(near, true)}
		}
		w.hurtS(w.P.MHP*3, s)
	}
}
