package sim

import (
	"sort"
	"strings"
)

// Item fusion and overcharge (what to do once held items are maxed).
//
// Overcharge: past Lv 10 an item keeps levelling up to Lv 20; each extra level counts a quarter
// (bronze 11-13, silver 14-16, gold 17-20 on the HUD).
// Fusion: two held items at Lv 10+ fuse into one that takes a single slot (freeing the other), keeps
// both effects at its level, and adds a bonus — a named one for the classic pairs, +6% damage for
// any other pair. A fused item can be overcharged like any other.

const (
	ItemMax      = 10 // normal cap
	ItemOverMax  = 20 // overcharge cap
	fusedPrefix  = "fz:"
	fuseGenericK = .06
)

// Fusion is a named fusion recipe.
type Fusion struct {
	ID, Name, A, B, Bonus string
}

var Fusions = []Fusion{
	{"razorband", "Razor Band", "band", "lens", "critical hits deal +50% damage"},
	{"tempo", "Tempo Claw", "claw", "metro", "-10% cooldowns"},
	{"lullaby", "Lullaby Bell", "left", "soothe", "+50% healing from both"},
	{"barbs", "Iron Barbs", "metal", "helmet", "-10% damage taken; contact damage doubled"},
	{"panorama", "Panorama Lens", "wide", "focus", "+15% range and area"},
	{"volley", "Volley Glasses", "glove", "wise", "+1 projectile"},
	{"inferno", "Inferno Belt", "heat", "belt", "+15% damage to burning or poisoned foes"},
	{"hover", "Hover Magnet", "boots", "magnet", "+50% pickup range and +8% speed"},
	{"champion", "Champion Band", "muscle", "choice", "+15% melee damage"},
	{"secondwind", "Second Wind", "lifeorb", "sash", "Life Orb stops draining; Focus Sash recharges twice as fast"},
	{"keen", "Keen Edge", "razor", "eject", "+5% crit chance; knockback grows"},
	{"study", "Study Bell", "exp", "soothe", "+15% XP"},
}

var fusionByPair = func() map[string]*Fusion {
	m := map[string]*Fusion{}
	for i := range Fusions {
		m[pairKey(Fusions[i].A, Fusions[i].B)] = &Fusions[i]
	}
	return m
}()

func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "+" + b
}

// FusedID is the held-item id two items fuse into.
func FusedID(a, b string) string {
	if f := fusionByPair[pairKey(a, b)]; f != nil {
		return fusedPrefix + f.ID
	}
	return fusedPrefix + pairKey(a, b)
}

// IsFused reports whether id is a fused item.
func IsFused(id string) bool { return strings.HasPrefix(id, fusedPrefix) }

// FusedParts are the two items a fused id was made from.
func FusedParts(id string) (a, b string) {
	k := strings.TrimPrefix(id, fusedPrefix)
	for _, f := range Fusions {
		if f.ID == k {
			return f.A, f.B
		}
	}
	if i := strings.IndexByte(k, '+'); i > 0 {
		return k[:i], k[i+1:]
	}
	return "", ""
}

// ItemInfo describes any held item id, fused ones included (nil if unknown).
func ItemInfo(id string) *Item {
	if !IsFused(id) {
		return ItemByID[id]
	}
	a, b := FusedParts(id)
	ia, ib := ItemByID[a], ItemByID[b]
	if ia == nil || ib == nil {
		return nil
	}
	if f := fusionByPair[pairKey(a, b)]; f != nil {
		return &Item{ID: id, Name: f.Name, Desc: ia.Name + " + " + ib.Name + " · " + f.Bonus}
	}
	last := func(n string) string { f := strings.Fields(n); return f[len(f)-1] }
	return &Item{ID: id, Name: last(ia.Name) + "–" + last(ib.Name) + " Fusion", Desc: ia.Name + " + " + ib.Name + " · +6% damage"}
}

// OverTier: 0 none, 1 bronze, 2 silver, 3 gold.
func OverTier(lv int) int {
	switch {
	case lv >= 17:
		return 3
	case lv >= 14:
		return 2
	case lv >= 11:
		return 1
	}
	return 0
}

// fz: the current player holds named fusion id.
func (w *World) fz(id string) bool { return w.Held[fusedPrefix+id] > 0 }

// fusedInto: the fused item that item k is part of ("" if none).
func (w *World) fusedInto(k string) string {
	for _, h := range w.HeldOrder {
		if IsFused(h) && w.Held[h] > 0 {
			if a, b := FusedParts(h); a == k || b == k {
				return h
			}
		}
	}
	return ""
}

// genericFusions: how many fused items without a named recipe the player holds (+6% damage each).
func (w *World) genericFusions() int {
	n := 0
	for _, h := range w.HeldOrder {
		if IsFused(h) && w.Held[h] > 0 {
			if a, b := FusedParts(h); fusionByPair[pairKey(a, b)] == nil {
				n++
			}
		}
	}
	return n
}

// fusionOptions: fusing pairs of held items that are both at Lv 10+ (named recipes first, at most 2).
func (w *World) fusionOptions() []Option {
	var ready []string
	for _, h := range w.HeldOrder {
		if !IsFused(h) && w.Held[h] >= ItemMax {
			ready = append(ready, h)
		}
	}
	var named, other []Option
	for i := 0; i < len(ready); i++ {
		for j := i + 1; j < len(ready); j++ {
			o := Option{ID: FusedID(ready[i], ready[j]), New: true, Fuse: true}
			if fusionByPair[pairKey(ready[i], ready[j])] != nil {
				named = append(named, o)
			} else {
				other = append(other, o)
			}
		}
	}
	sort.SliceStable(other, func(i, j int) bool { return other[i].ID < other[j].ID })
	out := append(named, other...)
	if len(out) > 2 {
		out = out[:2]
	}
	return out
}

// fuse merges the two parts of fused id f into one held item.
func (w *World) fuse(f string) {
	a, b := FusedParts(f)
	lv := max(w.Held[a], w.Held[b])
	keep := w.HeldOrder[:0:0]
	for _, h := range w.HeldOrder {
		if h != a && h != b {
			keep = append(keep, h)
		}
	}
	w.HeldOrder = append(keep, f)
	w.Held[f], w.Held[a], w.Held[b] = lv, lv, lv
	w.sfx("evolve")
	w.message("FUSED! " + ItemInfo(f).Name)
}

// levelHeld raises held item id by one (a fused item raises its parts with it).
func (w *World) levelHeld(id string) {
	w.Held[id]++
	if IsFused(id) {
		a, b := FusedParts(id)
		w.Held[a], w.Held[b] = w.Held[id], w.Held[id]
	}
}

// dropHeld removes held item id (and a fused item's parts).
func (w *World) dropHeld(id string) {
	w.Held[id] = 0
	if IsFused(id) {
		a, b := FusedParts(id)
		w.Held[a], w.Held[b] = 0, 0
	}
}
