package meta

import "math"

// Mastery after evolution: each evolution line's Mastery track runs to rank 100. Once the line's
// last evolution is unlocked, every 5 ranks give a reward — alternately a new starting move to
// choose at the Pokémon screen and a small permanent stat boost. At rank 100 the line can prestige:
// Mastery returns to rank 1 and every Pokémon in the line earns a star with its own stat boost.
// Starting moves stay unlocked through a prestige; rank stat boosts are earned again on the way up.

const (
	MasteryMax  = 100
	mrStep      = 5   // ranks between rewards
	mrFirst     = 30  // rewards start no earlier than this (lines with early or no evolutions)
	MrStatBoost = .01 // +1% HP and damage per stat reward
	MrStarBoost = .04 // +4% HP and damage per prestige star
)

type MReward struct {
	Rank int
	Kind string // "move" or "stat"
	Move string
}

// mrRules walks a rule and its "all" parts.
func mrRules(r *rule, f func(*rule)) {
	if r == nil {
		return
	}
	f(r)
	for i := range r.All {
		mrRules(&r.All[i], f)
	}
}

// FinalRank: the Mastery rank that unlocks the line's last evolution (0 if none needs Mastery).
func (p *Profile) FinalRank(root string) int {
	best := 0
	for _, c := range p.DB.Pokemon {
		if p.LineRoot(c.ID) != root {
			continue
		}
		mrRules(p.rule(c.ID), func(r *rule) {
			if r.Mastery != nil && *r.Mastery > best {
				best = *r.Mastery
			}
		})
	}
	return best
}

// MasteryRewards lists the line's rewards in rank order.
func (p *Profile) MasteryRewards(root string) []MReward {
	start := max(p.FinalRank(root), mrFirst)
	start = (start/mrStep + 1) * mrStep
	pool := p.mrMovePool(root)
	var out []MReward
	k := 0
	for r := start; r <= MasteryMax; r += mrStep {
		if k%2 == 0 && k/2 < len(pool) {
			out = append(out, MReward{Rank: r, Kind: "move", Move: pool[k/2]})
		} else {
			out = append(out, MReward{Rank: r, Kind: "stat"})
		}
		k++
	}
	return out
}

// mrMovePool: moves the line can open runs with — basic moves it can learn, in learnset order,
// skipping its normal starting move and moves that can't start a run.
func (p *Profile) mrMovePool(root string) []string {
	def := p.DB.StartMove(root)
	var out []string
	seen := map[string]bool{def: true}
	for _, id := range p.DB.LearnList(root) {
		m := p.DB.Moves[id]
		if m == nil || seen[id] || m.NoStart || m.Tier != "basic" {
			continue
		}
		switch m.Shape {
		case "wave", "sleep", "mine", "screen", "clone", "sub", "protect":
			continue // a run needs a starting move that deals damage
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// Stars: prestiges of the line.
func (p *Profile) Stars(id string) int { return int(num(p.S.obj("pst")[p.LineRoot(id)])) }

// BestRank: the highest Mastery rank the line has ever reached (survives prestige).
func (p *Profile) BestRank(root string) int {
	return max(int(num(p.S.obj("mxb")[root])), p.MasteryRank(root))
}

func (p *Profile) noteBest(root string) {
	if r := p.MasteryRank(root); float64(r) > num(p.S.obj("mxb")[root]) {
		p.S.obj("mxb")[root] = float64(r)
	}
}

// MasteryBonus: the HP / damage bonus from rank stat rewards and prestige stars.
func (p *Profile) MasteryBonus(id string) float64 {
	root := p.LineRoot(id)
	rk := p.MasteryRank(root)
	n := 0
	for _, r := range p.MasteryRewards(root) {
		if r.Kind == "stat" && r.Rank <= rk {
			n++
		}
	}
	return math.Round((MrStatBoost*float64(n)+MrStarBoost*float64(p.Stars(root)))*1000) / 1000
}

// StartMoves: the moves a Pokémon of this line can open a run with (its default first).
func (p *Profile) StartMoves(id string) []string {
	root := p.LineRoot(id)
	out := []string{p.DB.StartMove(id)}
	best := p.BestRank(root)
	for _, r := range p.MasteryRewards(root) {
		if r.Kind == "move" && r.Rank <= best && r.Move != out[0] {
			out = append(out, r.Move)
		}
	}
	return out
}

// StartMove: the chosen starting move for id ("" = the default).
func (p *Profile) StartMove(id string) string {
	v, _ := p.S.obj("smv")[p.LineRoot(id)].(string)
	for _, m := range p.StartMoves(id) {
		if m == v {
			return v
		}
	}
	return ""
}

func (p *Profile) SetStartMove(id, mv string) { p.S.obj("smv")[p.LineRoot(id)] = mv }

func (p *Profile) CanPrestige(id string) bool { return p.MasteryRank(p.LineRoot(id)) >= MasteryMax }

// Prestige resets the line's Mastery to rank 1 for a star. Evolutions stay unlocked.
func (p *Profile) Prestige(id string) bool {
	root := p.LineRoot(id)
	if !p.CanPrestige(root) {
		return false
	}
	p.noteBest(root)
	p.S.obj("mx")[root] = 0.0
	p.S.obj("pst")[root] = float64(p.Stars(root) + 1)
	return true
}

// TestSetMastery (testing only): every line gets xp Mastery XP.
func (p *Profile) TestSetMastery(xp float64) {
	for _, c := range p.DB.Pokemon {
		p.S.obj("mx")[p.LineRoot(c.ID)] = xp
	}
}
