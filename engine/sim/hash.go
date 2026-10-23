package sim

import "math"

// Hash summarises the simulation state in one number. Two worlds stepped with the same seed and
// inputs must hash the same every tick; lockstep play compares hashes to detect a desync, and the
// determinism tests use it to check that the sim has no hidden randomness.
func (w *World) Hash() uint64 {
	h := hasher(14695981039346656037)
	h.f(w.T)
	h.i(int(w.State))
	h.i(w.Kills)
	h.i(w.Bosses)
	h.f(w.Coins)
	h.f(w.XPS.XP)
	h.i(w.XPS.Lv)
	h.i(w.Chooser)
	h.i(w.LI)
	h.i(w.BiomeN)
	h.s(w.Biome)
	for _, c := range w.PL {
		p := c.P
		h.f(p.X)
		h.f(p.Y)
		h.f(p.HP)
		h.f(p.MHP)
		h.i(p.Lv)
		h.b(p.Dead)
		h.b(c.Gone)
		h.i(c.Kills)
		h.i(c.Deaths)
		h.i(len(c.Queue))
	}
	// the current player's live fields (the ctx copies are refreshed on switch)
	h.i(len(w.E))
	for _, e := range w.E {
		h.f(e.X)
		h.f(e.Y)
		h.f(e.HP)
	}
	h.i(len(w.PR))
	for _, p := range w.PR {
		h.f(p.X)
		h.f(p.Y)
	}
	h.i(len(w.Gems))
	h.i(len(w.Items))
	h.i(len(w.EB))
	h.i(len(w.HZ))
	h.i(len(w.FX))
	h.i(len(w.ZN))
	h.i(len(w.Binds))
	h.i(len(w.DL))
	h.i(len(w.MC))
	h.i(len(w.Chests))
	h.i(len(w.Opts))
	h.i(len(w.Route))
	h.i(len(w.Boons))
	if w.PCN != nil {
		h.f(w.PCN.X)
		h.f(w.PCN.Y)
		h.f(w.PCN.T)
	}
	return uint64(h)
}

type hasher uint64

func (h *hasher) u(v uint64) {
	for i := 0; i < 8; i++ {
		*h ^= hasher(v & 0xff)
		*h *= 1099511628211
		v >>= 8
	}
}
func (h *hasher) f(v float64) { h.u(math.Float64bits(v)) }
func (h *hasher) i(v int)     { h.u(uint64(int64(v))) }
func (h *hasher) b(v bool) {
	if v {
		h.u(1)
	} else {
		h.u(0)
	}
}
func (h *hasher) s(v string) {
	for _, c := range []byte(v) {
		h.u(uint64(c))
	}
}
