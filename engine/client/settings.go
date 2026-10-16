package client

import (
	"fmt"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

const scSettings screen = 120

// Settings live in the save under "opt" (same keys and values as the web game).
var optDefaults = map[string]float64{"vol": .6, "sfxv": .7, "musv": .6, "shake": 1, "dnum": 1, "flash": 1, "touch": 2}

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
	{"flash", "Flashing", "Screen flashes and enemy hit flashes", []optChoice{{"Reduced", 0}, {"Normal", 1}}},
	{"touch", "Touch controls", "On-screen stick and dash button", []optChoice{{"Off", 0}, {"On", 1}, {"Auto", 2}}},
}

func (g *Game) menuSettings() {
	sw := float64(g.sw)
	u := &g.ui
	g.header("SETTINGS", "Saved with your progress")
	g.backButton(g.settingsFrom)
	y := 104.0
	for _, r := range optRows {
		r := r
		u.label([4]float64{60, y, 420, 40}, false, func(dst *ebiten.Image, rr [4]float64) {
			drawText(dst, r.name, rr[0], rr[1]+2, 16, true, white, text.AlignStart, false)
			drawText(dst, r.desc, rr[0], rr[1]+24, 12, false, dim, text.AlignStart, false)
		})
		cur := g.opt(r.key)
		bw := math.Min(130, (sw-540)/float64(len(r.choices))-8)
		for i, ch := range r.choices {
			ch := ch
			on := cur == ch.v
			u.button(fmt.Sprintf("%s:%d", r.key, i), [4]float64{500 + float64(i)*(bw+8), y, bw, 44}, false, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
				if on {
					roundRect(dst, rr[0]-2, rr[1]-2, rr[2]+4, rr[3]+4, 10, alpha(gold, .9))
					roundRect(dst, rr[0], rr[1], rr[2], rr[3], 8, mix(c(28, 34, 50, 240), gold, .25))
					drawText(dst, ch.label, rr[0]+rr[2]/2, rr[1]+rr[3]/2-9, 15, true, white, text.AlignCenter, false)
					if hot {
						roundRect(dst, rr[0], rr[1]+rr[3]-4, rr[2], 4, 2, white)
					}
					return
				}
				btnDraw(ch.label, "", dim, false)(dst, rr, hot)
			}, func() {
				g.setOpt(r.key, ch.v)
				if r.key == "sfxv" || r.key == "vol" {
					g.audio.Play("click")
				}
			})
		}
		y += 62
	}
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
	dash        bool
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
	t.dash = false
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
		case fx > sw*.62 && fy > float64(g.sh)*.45: // dash zone
			t.dash = true
		case !t.stickOn && fx < sw*.62:
			t.stickOn, t.stickID = true, id
			t.ox, t.oy = fx, fy
			t.x, t.y = 0, 0
		}
	}
	return sim.Input{MoveX: t.x, MoveY: t.y, Dash: t.dash}, t.stickOn || t.dash
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
	// dash button
	f := 1.0
	if g.W != nil {
		f = g.W.DashReady()
	}
	col := c(255, 255, 255, 60)
	if f >= 1 {
		col = alpha(gold, .55)
	}
	disc(dst, sw-110, sh-120, 46, col)
	if f >= 0 && f < 1 {
		sector(dst, sw-110, sh-120, 46, -math.Pi/2+math.Pi*f, math.Pi*f, alpha(gold, .35))
	}
	drawText(dst, "DASH", sw-110, sh-129, 15, true, c(255, 255, 255, 230), text.AlignCenter, true)
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
