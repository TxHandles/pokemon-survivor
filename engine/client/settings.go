package client

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

const scSettings screen = 120

// Settings live in the save under "opt" (same keys and values as the web game).
var optDefaults = map[string]float64{"vol": .3, "sfxv": .7, "musv": .6, "shake": 1, "dnum": 1, "flash": 1, "touch": 2, "eq": 1, "autop": 1, "hud": 0, "fx": 2}

func (g *Game) opt(k string) float64 {
	if g.prof != nil {
		if o, ok := g.prof.S.M["opt"].(map[string]any); ok {
			if v, ok := o[k].(float64); ok {
				return v
			}
		}
	}
	return optDefaults[k]
}

func (g *Game) setOpt(k string, v float64) {
	o, ok := g.prof.S.M["opt"].(map[string]any)
	if !ok {
		o = map[string]any{}
		g.prof.S.M["opt"] = o
	}
	o[k] = v
	g.save()
}

type optRow struct {
	key, name, desc string
	choices         []optChoice
}
type optChoice struct {
	label string
	v     float64
}

var optRows = []optRow{
	{"vol", "Master volume", "Overall game volume", []optChoice{{"Off", 0}, {"Low", .3}, {"Med", .6}, {"High", 1}}},
	{"sfxv", "Sound effects", "Hits, pickups, level-ups and boss cues", []optChoice{{"Off", 0}, {"Low", .35}, {"Med", .7}, {"High", 1}}},
	{"musv", "Music", "Location tracks where they exist, a placeholder loop elsewhere", []optChoice{{"Off", 0}, {"Low", .3}, {"Med", .6}, {"High", 1}}},
	{"shake", "Screen shake", "Camera shake on hits, bosses and explosions", []optChoice{{"Off", 0}, {"Reduced", .4}, {"Full", 1}}},
	{"dnum", "Damage numbers", "Floating numbers when you hit enemies", []optChoice{{"Off", 0}, {"On", 1}}},
	{"fx", "Attack effects", "Extreme: drawn effects plus particles, shockwaves and flashes. Full: drawn effects only. Simple: plain shapes (fastest)", []optChoice{{"Simple", 0}, {"Full", 1}, {"Extreme", 2}}},
	{"flash", "Flashing", "Screen flashes, enemy hit flashes and the low-health pulse", []optChoice{{"Reduced", 0}, {"Normal", 1}}},
	{"hud", "HUD layout", "New: party on the left, attacks and items bottom-left. Web: everything along the top", []optChoice{{"New", 0}, {"Web", 1}}},
	{"eq", "Equipped bar", "Show your attacks and items on screen during a run", []optChoice{{"Hide", 0}, {"Show", 1}}},
	{"autop", "Auto-pause", "Pause automatically when you switch to another window or tab", []optChoice{{"Off", 0}, {"On", 1}}},
	{"touch", "Touch controls", "On-screen stick and dash button", []optChoice{{"Off", 0}, {"On", 1}, {"Auto", 2}}},
}

func (g *Game) menuSettings() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	g.webHeaderParts("Settings", "", g.settingsFrom, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	rw := math.Min(560, sw*.94)
	x0 := sw/2 - rw/2
	y := 0.0
	for _, r := range optRows {
		r := r
		cur := g.opt(r.key)
		// choices right-aligned, title and description on the left (web .sopt)
		var cws []float64
		tot := 0.0
		for _, ch := range r.choices {
			w := textW(ch.label, 13, false) + 24
			cws = append(cws, w)
			tot += w + 6
		}
		dl := wrap(r.desc, font(12, false), rw-28-tot-10)
		h := math.Max(54, 30+float64(len(dl))*15+8)
		u.label([4]float64{x0, y, rw, h}, true, func(dst *ebiten.Image, rr [4]float64) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, webBorder, webPanel)
			drawText(dst, r.name, rr[0]+14, rr[1]+9, 16, true, webFg, text.AlignStart, false)
			for i, l := range dl {
				drawText(dst, l, rr[0]+14, rr[1]+30+float64(i)*15, 12, false, alpha(webFg, .75), text.AlignStart, false)
			}
		})
		bx := x0 + rw - 14 - tot + 6
		for i, ch := range r.choices {
			ch := ch
			on := cur == ch.v
			u.button(fmt.Sprintf("%s:%d", r.key, i), [4]float64{bx, y + h/2 - 15, cws[i], 30}, true, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
				chipBtn(dst, rr, on, hot, func(x, yy float64) {
					drawText(dst, ch.label, rr[0]+rr[2]/2, yy-8, 13, false, webFg, text.AlignCenter, false)
				})
			}, func() {
				g.setOpt(r.key, ch.v)
				if r.key == "sfxv" || r.key == "vol" {
					g.audio.Play("click")
				}
			})
			bx += cws[i] + 6
		}
		y += h + 8
	}
	// leaderboard name (text field)
	if g.lbNameEdit == "" && g.ui.edit != "lbname" {
		g.lbNameEdit = g.optStr("name")
	}
	u.label([4]float64{x0, y, rw, 54}, true, func(dst *ebiten.Image, rr [4]float64) {
		webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, webBorder, webPanel)
		drawText(dst, "Leaderboard name", rr[0]+14, rr[1]+9, 16, true, webFg, text.AlignStart, false)
		drawText(dst, "Shown next to your runs (max 16 characters)", rr[0]+14, rr[1]+30, 12, false, alpha(webFg, .75), text.AlignStart, false)
	})
	was := g.ui.edit == "lbname"
	ar := u.area
	u.area = [4]float64{0, 0, 0, 0}
	fy := ar[1] - u.scroll + y + 11
	if fy > ar[1]-40 && fy < ar[1]+ar[3] {
		g.textField("lbname", [4]float64{x0 + rw - 14 - 190, fy, 190, 32}, &g.lbNameEdit, "Player", false)
	}
	u.area = ar
	if len(g.lbNameEdit) > 16 {
		g.lbNameEdit = g.lbNameEdit[:16]
	}
	if was && g.ui.edit != "lbname" {
		g.setOptStr("name", strings.TrimSpace(g.lbNameEdit))
	}
	y += 62 + 10
	u.scrollMax = math.Max(0, y-u.area[3])
}

func (g *Game) optStr(k string) string {
	if o, ok := g.prof.S.M["opt"].(map[string]any); ok {
		if s, ok := o[k].(string); ok {
			return s
		}
	}
	return ""
}

func (g *Game) setOptStr(k, v string) {
	o, ok := g.prof.S.M["opt"].(map[string]any)
	if !ok {
		o = map[string]any{}
		g.prof.S.M["opt"] = o
	}
	o[k] = v
	if l, ok := g.prof.S.M["lbl"].(map[string]any); ok {
		l["name"] = v
	}
	g.save()
}

// ---- audio per frame ----

func (g *Game) audioTick() {
	a := g.audio
	if a == nil {
		return
	}
	a.Update(1.0 / 60)
	key, biome, boss := "", "", false
	switch g.scr {
	case scPlay, scPause, scOver:
		if g.W != nil && g.scr != scOver {
			biome = g.W.Biome
			key = fmt.Sprintf("%s:%d", biome, g.W.LI)
			for _, e := range g.W.E {
				if e.Boss && !e.Dead {
					boss = true
					break
				}
			}
		}
	case scResult, scSettings:
		if g.scr == scSettings && g.settingsFrom != scPause {
			key = "menu"
		}
	default:
		key = "menu"
	}
	a.SetMusic(g.db, key, biome, boss)
	if g.W != nil {
		for _, e := range g.W.DrainEvents() {
			if g.scr == scPlay || g.scr == scOver {
				a.Play(e)
			}
		}
	}
}

// ---- touch controls ----

type touchState struct {
	seen        bool
	stickID     ebiten.TouchID
	stickOn     bool
	ox, oy      float64 // stick origin
	x, y        float64 // stick vector -1..1
	dash, act2  bool
	tapX, tapY  float64
	tapped      bool
	pauseTapped bool
	forced      bool
}

func (g *Game) touchEnabled() bool {
	switch g.opt("touch") {
	case 0:
		return false
	case 1:
		return true
	}
	return g.touch.seen
}

// touchInput reads the virtual stick and dash button; returns whether a touch is driving the player.
func (g *Game) touchInput() (sim.Input, bool) {
	t := &g.touch
	t.dash, t.act2 = false, false
	t.tapped = false
	t.pauseTapped = false
	ids := ebiten.AppendTouchIDs(nil)
	if len(ids) > 0 {
		t.seen = true
	}
	if !g.touchEnabled() {
		return sim.Input{}, false
	}
	sw := float64(g.sw)
	stickAlive := false
	for _, id := range ids {
		if t.stickOn && id == t.stickID {
			stickAlive = true
			px, py := ebiten.TouchPosition(id)
			dx, dy := float64(px)-t.ox, float64(py)-t.oy
			l := math.Hypot(dx, dy)
			if l > 70 { // drag the origin along so long swipes keep steering
				t.ox, t.oy = float64(px)-dx/l*70, float64(py)-dy/l*70
				l = 70
			}
			k := math.Min(1, l/45)
			if l > 0 {
				t.x, t.y = dx/l*k, dy/l*k
			} else {
				t.x, t.y = 0, 0
			}
		}
	}
	if !stickAlive {
		t.stickOn = false
		t.x, t.y = 0, 0
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		px, py := ebiten.TouchPosition(id)
		fx, fy := float64(px), float64(py)
		switch {
		case fx > sw-80 && fy < 70: // pause button
			t.pauseTapped = true
		case fx > sw*.62 && fy > float64(g.sh)*.45: // action zone (upper part: second action when there are two)
			if fy < float64(g.sh)*.66 && g.W != nil && len(g.W.ActiveSlots()) > 1 {
				t.act2 = true
			} else {
				t.dash = true
			}
		case !t.stickOn && fx < sw*.62:
			t.stickOn, t.stickID = true, id
			t.ox, t.oy = fx, fy
			t.x, t.y = 0, 0
		}
	}
	return sim.Input{MoveX: t.x, MoveY: t.y, Dash: t.dash, Act2: t.act2}, t.stickOn || t.dash || t.act2
}

// touchTap reports a tap this frame (for menus and choice cards) and where.
func (g *Game) touchTap() (float64, float64, bool) {
	for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
		px, py := inpututil.TouchPositionInPreviousTick(id)
		g.touch.seen = true
		return float64(px), float64(py), true
	}
	return 0, 0, false
}

func (g *Game) drawTouch(dst *ebiten.Image) {
	if !g.touchEnabled() || (g.auto && !g.touch.forced) {
		return
	}
	t := &g.touch
	sw, sh := float64(g.sw), float64(g.sh)
	if t.stickOn {
		ring(dst, t.ox, t.oy, 52, 2, c(255, 255, 255, 90))
		disc(dst, t.ox+t.x*45, t.oy+t.y*45, 22, c(255, 255, 255, 110))
	} else {
		ring(dst, 110, sh-120, 52, 2, c(255, 255, 255, 40))
		disc(dst, 110, sh-120, 22, c(255, 255, 255, 40))
	}
	// action buttons (a second one above when two button moves are equipped)
	for btn, by := range []float64{sh - 120, sh - 232} {
		f := 1.0
		lbl := "DASH"
		if g.W != nil {
			f = g.W.ButtonReady(btn)
			for _, s := range g.W.ActiveSlots() {
				if g.W.ActiveButton(s) == btn {
					lbl = strings.ToUpper(fitText(s.Move.Name, 13, true, 84))
					break
				}
			}
		}
		if btn == 1 && f < 0 {
			break
		}
		col := c(255, 255, 255, 60)
		if f >= 1 {
			col = alpha(gold, .55)
		}
		rr := b2f(btn == 0, 46, 38)
		disc(dst, sw-110, by, rr, col)
		if f >= 0 && f < 1 {
			sector(dst, sw-110, by, rr, -math.Pi/2+math.Pi*f, math.Pi*f, alpha(gold, .35))
		}
		drawText(dst, lbl, sw-110, by-8, 13, true, c(255, 255, 255, 230), text.AlignCenter, true)
	}
	// pause
	roundRect(dst, sw-64, 14, 48, 40, 8, c(20, 26, 40, 170))
	rect(dst, sw-50, 24, 7, 20, c(230, 235, 245, 230))
	rect(dst, sw-37, 24, 7, 20, c(230, 235, 245, 230))
}

// ForceTouch shows the touch controls regardless of the setting (testing).
func (g *Game) ForceTouch() { g.touch.seen, g.touch.forced = true, true; g.setOptMem("touch", 1) }

func (g *Game) setOptMem(k string, v float64) {
	o, ok := g.prof.S.M["opt"].(map[string]any)
	if !ok {
		o = map[string]any{}
		g.prof.S.M["opt"] = o
	}
	o[k] = v
}
