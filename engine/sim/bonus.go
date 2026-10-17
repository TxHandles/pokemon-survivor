package sim

import "math"

// Level-up bonuses: each move level-up rolls one of these (the card shows which), suited to the
// move's shape: melee attacks never roll projectile count, rings never roll projectile speed…
// Levels themselves still raise the move a little (half the old per-level damage growth plus the
// move's own level scaling); the rolled bonus is the big part.

const (
	BonusDmg  = "dmg"
	BonusCD   = "cd"
	BonusArea = "area"
	BonusSize = "size"
	BonusSpd  = "spd"
	BonusCnt  = "cnt"
)

// per roll
const (
	bonusDmgK  = .15 // +15% damage
	bonusCDK   = .93 // -7% cooldown
	bonusAreaK = .10 // +10% range / area
	bonusSizeK = .12 // +12% size
	bonusSpdK  = .12 // +12% projectile / orbit speed
)

// levelDmgGrowth: the share of the old per-level damage growth that levels still give.
const levelDmgGrowth = .5

// BonusName / BonusText: how the cards show a bonus.
var BonusName = map[string]string{BonusDmg: "Damage", BonusCD: "Cooldown", BonusArea: "Range", BonusSize: "Size", BonusSpd: "Projectile speed", BonusCnt: "Projectiles"}

func BonusText(b string) string {
	switch b {
	case BonusDmg:
		return "+15% damage"
	case BonusCD:
		return "-7% cooldown"
	case BonusArea:
		return "+10% range / area"
	case BonusSize:
		return "+12% size"
	case BonusSpd:
		return "+12% speed"
	case BonusCnt:
		return "+1 projectile"
	}
	return ""
}

// BonusPool: the bonuses a move of this shape can roll.
func BonusPool(shape string) []string {
	switch shape {
	case "proj", "twin", "sludge", "homing", "boom", "burst":
		return []string{BonusDmg, BonusCD, BonusSize, BonusSpd, BonusCnt}
	case "chain":
		return []string{BonusDmg, BonusCD, BonusArea, BonusCnt}
	case "orbit":
		return []string{BonusDmg, BonusSize, BonusSpd, BonusArea, BonusCnt}
	case "toss":
		return []string{BonusDmg, BonusCD, BonusCnt, BonusArea, BonusSize}
	case "strike", "erupt", "mine", "rupture":
		return []string{BonusDmg, BonusCD, BonusArea, BonusCnt}
	case "beam", "sweep":
		return []string{BonusDmg, BonusCD, BonusSize}
	case "screen":
		return []string{BonusCnt, BonusCD}
	case "clone", "wave":
		return []string{BonusCD, BonusArea}
	}
	// melee and area attacks: arc jab spin cone dash nova aura blast whirl trail
	return []string{BonusDmg, BonusCD, BonusArea}
}

func (w *World) rollBonus(shape string) string {
	p := BonusPool(shape)
	return p[w.rng.Intn(len(p))]
}

// slot bonus helpers (nil-safe; s may be nil)
func (s *Slot) bon(k string) float64 {
	if s == nil || s.Bon == nil {
		return 0
	}
	return float64(s.Bon[k])
}

// BonusDamageMul etc.: the multipliers a slot's rolled bonuses give.
func (s *Slot) BonusDamageMul() float64 { return 1 + bonusDmgK*s.bon(BonusDmg) }
func (s *Slot) BonusCDMul() float64     { return math.Pow(bonusCDK, s.bon(BonusCD)) }
func (s *Slot) BonusAreaMul() float64   { return 1 + bonusAreaK*s.bon(BonusArea) }
func (s *Slot) BonusSizeMul() float64   { return 1 + bonusSizeK*s.bon(BonusSize) }
func (s *Slot) BonusSpdMul() float64    { return 1 + bonusSpdK*s.bon(BonusSpd) }
func (s *Slot) BonusCount() int         { return int(s.bon(BonusCnt)) }

// Bonuses lists a slot's rolled bonuses as "Damage ×2" style labels, in a fixed order.
func (s *Slot) Bonuses() []string {
	var out []string
	for _, k := range []string{BonusDmg, BonusCD, BonusArea, BonusSize, BonusSpd, BonusCnt} {
		if n := int(s.bon(k)); n > 0 {
			out = append(out, BonusName[k]+" ×"+itoa(n))
		}
	}
	return out
}

// dsl: the damage level scale (half the old growth per level).
func dsl(lv int) float64 { return 1 + float64(lv-1)*.45*levelDmgGrowth }
