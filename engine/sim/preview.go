package sim

import "math"

// Read-only helpers for the client's HUD and level-up cards. They use no randomness and change no
// state, so calling them never affects the simulation (or lockstep play).

// CoolFrac is how charged slot s is (1 = ready), for the equipped bar.
func (w *World) CoolFrac(s *Slot) float64 {
	if s.Kind == "orbit" {
		return 1
	}
	c := w.cdOf(s) * w.cdM()
	if c <= 0 || c > 1e8 {
		return 1
	}
	return math.Max(0, math.Min(1, 1-s.CD/c))
}

// MovePreview is move id's damage per hit and cooldown at level lv (evolved or not), with the
// current player's items and upgrades. cd is 0 for moves without a cooldown (orbits).
func (w *World) MovePreview(id string, lv int, ev bool) (dmg, cd float64) {
	m := w.DB.Moves[id]
	if m == nil {
		return 0, 0
	}
	q := BuildQ(m)
	sl := 1 + float64(lv-1)*.45
	evm := 1.0
	if ev {
		evm = 1.35 * (1 + .08*w.pe(m.HeldItem))
	}
	bal := 1.0
	tbl := w.DB.Balance.MoveDamage
	if ev {
		tbl = w.DB.Balance.MoveDamageEvolved
	}
	if v, ok := tbl[id]; ok {
		bal = v
	}
	dmg = (q.F("d", 7) + q.F("dp", 3)*sl) * evm * bal * w.dmgM()
	if has(w.P.Types, m.Type) {
		dmg *= 1.5
	}
	if m.Shape != "orbit" {
		s := &Slot{ID: id, Move: m, Kind: m.Shape, Type: m.Type, Lv: lv, Evo: ev, Q: q}
		if c := w.cdOf(s) * w.cdM(); c > 0 && c < 1e8 {
			cd = c
		}
	}
	return
}

// PE is the effective level of held item k (levels above 5 count half), for previews.
func PE(lv int) float64 {
	v := float64(lv)
	if v > 5 {
		return 5 + (v-5)*.5
	}
	return v
}
