package client

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Hover tooltips for moves and items: what kind of move it is, what it does and its numbers.

type tipLine struct {
	s    string
	col  color.NRGBA
	size int
	bold bool
}

type tipData struct {
	title string
	col   color.NRGBA
	lines []tipLine
	at    [4]float64 // the thing hovered (the tip sits next to it)
}

// shapeKind: a plain-words category for each move shape.
var shapeKind = map[string]string{
	"proj": "Projectile", "twin": "Twin projectiles", "sludge": "Lobbed projectile", "homing": "Homing projectile",
	"boom": "Boomerang", "burst": "Projectile burst", "chain": "Chain attack", "beam": "Beam", "sweep": "Sweeping beam",
	"orbit": "Orbiting", "arc": "Melee swipe", "jab": "Melee jab", "dash": "Dash · button move", "blast": "Close-range blast",
	"aura": "Aura around you", "spin": "Spinning melee", "cone": "Cone", "nova": "Ring blast", "strike": "Strikes from above",
	"erupt": "Eruptions", "mine": "Traps", "rupture": "Ground ruptures", "whirl": "Whirlpools", "trail": "Ground trail",
	"wave": "Status wave (no damage)", "screen": "Shield", "clone": "Clone", "toss": "Grab and throw",
	"fly": "Flight · button move", "rollout": "Roll · button move", "protect": "Shield", "quake": "Earthquake",
	"magnitude": "Random-strength quake", "seed": "Draining seeds", "sleep": "Sleep (no damage)", "sub": "Decoy",
	"charge": "Charge-up blast", "counter": "Counter (stores damage taken)", "bind": "Bind (holds a foe)", "ramp": "Ramp-up (grows stronger)",
	"sac": "Sacrifice (costs HP)", "vanish": "Vanish and strike",
}

func shapeLabel(k string) string {
	if s, ok := shapeKind[k]; ok {
		return s
	}
	return cap1(k)
}

// hoverTip shows tip (built lazily) while the mouse is over r.
func (g *Game) hoverTip(r [4]float64, build func() *tipData) {
	if g.touch.seen && len(ebiten.AppendTouchIDs(nil)) > 0 {
		return
	}
	mx, my := ebiten.CursorPosition()
	if !inR(r, float64(mx), float64(my)) {
		return
	}
	if t := build(); t != nil {
		t.at = r
		g.tip = t
	}
}

// moveTip: a move's tooltip. With a slot it shows that slot's level, bonuses and current numbers;
// without one, the move's starting stats.
func (g *Game) moveTip(id string, s *sim.Slot) *tipData {
	m := g.db.Moves[id]
	if m == nil {
		return nil
	}
	col := tc(m.Type)
	t := &tipData{title: m.Name, col: col}
	add := func(s string, c color.NRGBA, size int, bold bool) {
		t.lines = append(t.lines, tipLine{s, c, size, bold})
	}
	lv, ev := 1, false
	if s != nil {
		lv, ev = s.Lv, s.Evo
		if ev {
			t.title += " (evolved)"
		}
	}
	add(strings.ToUpper(m.Type)+" · "+shapeLabel(m.Shape), col, 12, true)
	if s != nil {
		l := fmt.Sprintf("Level %d", lv)
		if b := s.Bonuses(); len(b) > 0 {
			l += " · " + strings.Join(b, ", ")
		}
		add(l, alpha(webFg, .85), 12, false)
	}
	for _, ln := range wrap(m.Desc, font(12, false), 260) {
		add(ln, webFg, 12, false)
	}
	if w := g.tipWorld(); w != nil {
		head := "Starting stats"
		if s != nil {
			head = "Now"
		}
		add(head, webAcc, 12, true)
		var d, cd float64
		var pr sim.Probe
		if s != nil && g.W != nil && w == g.W {
			d, cd = w.MovePreviewB(id, lv, ev, "")
			pr = w.ProbeMoveB(id, lv, ev, "")
		} else {
			d, cd = w.MovePreview(id, lv, ev)
			pr = w.ProbeMove(id, lv, ev)
		}
		if d > 0 && m.Shape != "sleep" {
			add(fmt.Sprintf("Damage %d", int(math.Round(d))), webFg, 12, false)
		}
		if cd > 0 {
			add(fmt.Sprintf("Cooldown %.2fs", cd), webFg, 12, false)
		}
		if pr.N > 0 && pr.NL != "" {
			add(fmt.Sprintf("%s %d", pr.NL, int(pr.N)), webFg, 12, false)
		}
		if pr.Area > 0 && pr.AL != "" {
			add(fmt.Sprintf("%s %d", pr.AL, int(math.Round(pr.Area))), webFg, 12, false)
		}
		if pr.Du > 0 && pr.DL != "" {
			add(fmt.Sprintf("%s %.1fs", pr.DL, pr.Du), webFg, 12, false)
		}
	}
	if sim.ActiveKind(m.Shape) {
		add("Fire it with Shift / RB (second button move: Ctrl / LB)", hexC("#7fd8ff"), 11, false)
	}
	if !ev {
		if it := sim.ItemByID[m.HeldItem]; it != nil {
			add("Evolves at Lv 10 while holding "+it.Name, alpha(webAcc, .9), 11, false)
		}
	}
	return t
}

// itemTip: a held item's tooltip (lv 0 = not held).
func (g *Game) itemTip(id string, lv int) *tipData {
	it := sim.ItemInfo(id)
	if it == nil {
		return nil
	}
	t := &tipData{title: it.Name, col: webAcc}
	kind := "Held item"
	if sim.IsFused(id) {
		kind, t.col = "Fused item", hexC("#c49bff")
	}
	if lv > 0 {
		kind += fmt.Sprintf(" · Level %d", lv)
		if tr := sim.OverTier(lv); tr > 0 {
			kind += " · " + []string{"", "Bronze", "Silver", "Gold"}[tr] + " overcharge"
		}
	}
	t.lines = append(t.lines, tipLine{kind, alpha(webFg, .85), 12, true})
	for _, ln := range wrap("Each level: "+it.Desc, font(12, false), 260) {
		t.lines = append(t.lines, tipLine{ln, webFg, 12, false})
	}
	if lv > 5 {
		t.lines = append(t.lines, tipLine{"Levels 6–10 count half, overcharge levels (11–20) a quarter", alpha(webFg, .7), 11, false})
	}
	if lv >= sim.ItemMax && !sim.IsFused(id) && g.W != nil {
		t.lines = append(t.lines, tipLine{"Maxed: fuse it with another Lv 10 item from a level-up", hexC("#c49bff"), 11, false})
	}
	var evo []string
	for _, m := range g.db.MoveList {
		if (m.HeldItem == id || (sim.IsFused(id) && strings.Contains(id, m.HeldItem))) && len(evo) < 4 && (g.W == nil || g.W.SlotOf(m.ID) != nil) {
			evo = append(evo, m.Name)
		}
	}
	if len(evo) > 0 && g.W != nil {
		t.lines = append(t.lines, tipLine{"Evolves your " + strings.Join(evo, ", ") + " at Lv 10", alpha(webAcc, .9), 11, false})
	}
	return t
}

// tipWorld: the run in progress, or a scratch world for the selected Pokémon (menus).
func (g *Game) tipWorld() *sim.World {
	if g.W != nil && (g.scr == scPlay || g.scr == scPause) {
		return g.W
	}
	id := g.csel
	if id == "" || g.db.ByID[id] == nil {
		return nil
	}
	if g.tipW == nil || g.tipWID != id {
		w, err := sim.New(g.db, id, g.db.ByID[id].Types[0], 1)
		if err != nil {
			return nil
		}
		g.tipW, g.tipWID = w, id
	}
	return g.tipW
}

// drawTip draws the current tooltip next to what it describes, kept on screen.
func (g *Game) drawTip(dst *ebiten.Image) {
	t := g.tip
	g.tip = nil
	if t == nil {
		return
	}
	sw, sh := float64(g.sw), float64(g.sh)
	w := textW(t.title, 15, true)
	h := 26.0
	for _, l := range t.lines {
		w = math.Max(w, textW(l.s, l.size, l.bold))
		h += float64(l.size) + 5
	}
	w += 24
	h += 8
	x := t.at[0] + t.at[2] + 10
	if x+w > sw-8 {
		x = t.at[0] - w - 10
	}
	if x < 8 {
		x = math.Max(8, math.Min(sw-w-8, t.at[0]))
	}
	y := t.at[1]
	if y+h > sh-8 {
		y = sh - h - 8
	}
	if y < 8 {
		y = 8
	}
	webBox(dst, x, y, w, h, 10, alpha(t.col, .9), alpha(hexC("#0a0d14"), .96))
	drawText(dst, t.title, x+12, y+8, 15, true, t.col, text.AlignStart, false)
	yy := y + 30
	for _, l := range t.lines {
		drawText(dst, l.s, x+12, yy, l.size, l.bold, l.col, text.AlignStart, false)
		yy += float64(l.size) + 5
	}
}

// tierCol: overcharge tier colours (bronze, silver, gold).
func tierCol(t int) color.NRGBA {
	switch t {
	case 1:
		return hexC("#d08a4c")
	case 2:
		return hexC("#d9e2ee")
	case 3:
		return hexC("#ffd84a")
	}
	return webAcc
}

// itemRimCol: a held item's ring on the HUD (fusions purple, overcharged items by tier).
func itemRimCol(id string, lv int) color.NRGBA {
	if t := sim.OverTier(lv); t > 0 {
		return tierCol(t)
	}
	if sim.IsFused(id) {
		return hexC("#c49bff")
	}
	return webAcc
}
