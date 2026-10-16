package client

import (
	"fmt"
	"io/fs"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

type screen int

const (
	scTitle screen = iota
	scPlay
	scPause
	scOver
)

// Game implements ebiten.Game.
type Game struct {
	db     *content.DB
	assets *Assets
	W      *sim.World
	floor  *Floor
	pa     playerAnim
	scr    screen
	sw, sh int

	// title
	roster  []string
	sel     int
	biome   int // index into content.Types, -1 = Pokémon's own type
	ability int // 0 starting, 1 hidden
	// Hold keeps demo choice screens open (for screenshots)
	Hold   bool
	floorN int
	// level-up
	lvSel   int
	lvDelay float64
	// over
	overT float64

	auto bool // demo: the bot plays (title screen background / -demo flag)
	bot  *sim.Bot
	tick int
}

func New(dataFS fs.FS, db *content.DB) *Game {
	g := &Game{db: db, assets: NewAssets(dataFS, db), biome: -1, sw: 1280, sh: 720}
	for _, p := range db.Pokemon { // first forms: the roster you pick from in this build
		if p.EvolvesFrom == "" && p.RegionalOf == "" && !p.Legendary {
			g.roster = append(g.roster, p.ID)
		}
	}
	return g
}

// Start begins a run (used by the title screen and by -shot / -demo).
func (g *Game) Start(id, biome string, seed int64) {
	if biome == "" {
		biome = g.db.ByID[id].Types[0]
	}
	w, err := sim.NewRun(g.db, id, biome, seed, g.ability)
	if err != nil {
		panic(err)
	}
	w.ViewW, w.ViewH = float64(g.sw), float64(g.sh)
	g.W = w
	g.floor = NewFloor(biome, seed)
	g.floorN = w.BiomeN
	g.pa = playerAnim{}
	g.scr = scPlay
	g.bot = sim.NewBot(w, seed+1)
	if g.auto {
		w.AutoDash = true
	}
}

// Demo makes the bot play (used for screenshots and an attract mode).
func (g *Game) Demo(on bool) { g.auto = on }

var bioNames = map[string]string{"normal": "Meadow Plains", "fire": "Volcano", "water": "Lagoon", "electric": "Storm Plateau", "grass": "Deep Forest",
	"ice": "Glacier", "fighting": "Dojo Ruins", "poison": "Toxic Swamp", "ground": "Canyon", "flying": "Sky Cliffs", "psychic": "Dream Fields",
	"bug": "Thicket", "rock": "Quarry", "ghost": "Haunted Moor", "dragon": "Dragon Peaks", "dark": "Shadow Wood", "steel": "Foundry", "fairy": "Glimmer Glade"}

func biomeName(t string) string {
	if n, ok := bioNames[t]; ok {
		return n
	}
	return t
}

func (g *Game) Layout(ow, oh int) (int, int) {
	// keep about 720 logical pixels of height so the view matches the web game's zoom
	if oh <= 0 {
		return 1280, 720
	}
	h := 720
	w := int(math.Round(float64(ow) * float64(h) / float64(oh)))
	if w < 640 {
		w = 640
		h = int(math.Round(float64(oh) * 640 / float64(ow)))
	}
	g.sw, g.sh = w, h
	if g.W != nil {
		g.W.ViewW, g.W.ViewH = float64(w), float64(h)
	}
	return w, h
}

// ---- input ----

func pads() []ebiten.GamepadID { return ebiten.AppendGamepadIDs(nil) }

func padBtn(b ebiten.StandardGamepadButton) bool {
	for _, id := range pads() {
		if ebiten.IsStandardGamepadLayoutAvailable(id) && inpututil.IsStandardGamepadButtonJustPressed(id, b) {
			return true
		}
	}
	return false
}

func keyJust(ks ...ebiten.Key) bool {
	for _, k := range ks {
		if inpututil.IsKeyJustPressed(k) {
			return true
		}
	}
	return false
}

func keyDown(ks ...ebiten.Key) bool {
	for _, k := range ks {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	return false
}

func confirm() bool {
	return keyJust(ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyNumpadEnter) || padBtn(ebiten.StandardGamepadButtonRightBottom)
}

// navigation: -1/0/1 per axis, edge-triggered (keys, d-pad, stick)
var stickHeld [2]int

func nav() (int, int) {
	dx, dy := 0, 0
	if keyJust(ebiten.KeyLeft, ebiten.KeyA) || padBtn(ebiten.StandardGamepadButtonLeftLeft) {
		dx = -1
	}
	if keyJust(ebiten.KeyRight, ebiten.KeyD) || padBtn(ebiten.StandardGamepadButtonLeftRight) {
		dx = 1
	}
	if keyJust(ebiten.KeyUp, ebiten.KeyW) || padBtn(ebiten.StandardGamepadButtonLeftTop) {
		dy = -1
	}
	if keyJust(ebiten.KeyDown, ebiten.KeyS) || padBtn(ebiten.StandardGamepadButtonLeftBottom) {
		dy = 1
	}
	for _, id := range pads() {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		lx := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		ly := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		sx, sy := 0, 0
		if lx < -.6 {
			sx = -1
		} else if lx > .6 {
			sx = 1
		}
		if ly < -.6 {
			sy = -1
		} else if ly > .6 {
			sy = 1
		}
		if sx != stickHeld[0] && sx != 0 {
			dx = sx
		}
		if sy != stickHeld[1] && sy != 0 {
			dy = sy
		}
		stickHeld = [2]int{sx, sy}
	}
	return dx, dy
}

func (g *Game) playInput() sim.Input {
	if g.auto {
		return g.bot.Input(1.0 / 60)
	}
	x, y := 0.0, 0.0
	if keyDown(ebiten.KeyD, ebiten.KeyArrowRight) {
		x++
	}
	if keyDown(ebiten.KeyA, ebiten.KeyArrowLeft) {
		x--
	}
	if keyDown(ebiten.KeyS, ebiten.KeyArrowDown) {
		y++
	}
	if keyDown(ebiten.KeyW, ebiten.KeyArrowUp) {
		y--
	}
	if x != 0 && y != 0 {
		x, y = x*.707, y*.707
	}
	for _, id := range pads() {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		lx := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		ly := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if m := math.Hypot(lx, ly); m > .2 {
			k := math.Min(1, (m-.2)/.6) / m
			x, y = lx*k, ly*k
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft) {
			x = -1
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight) {
			x = 1
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop) {
			y = -1
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom) {
			y = 1
		}
	}
	dash := keyJust(ebiten.KeyShiftLeft, ebiten.KeyShiftRight) || padBtn(ebiten.StandardGamepadButtonFrontTopRight)
	return sim.Input{MoveX: x, MoveY: y, Dash: dash}
}

// ---- update ----

func (g *Game) Update() error {
	g.tick++
	switch g.scr {
	case scTitle:
		g.updateTitle()
	case scPlay:
		g.updatePlay()
	case scPause:
		if keyJust(ebiten.KeyEscape, ebiten.KeyP) || padBtn(ebiten.StandardGamepadButtonCenterRight) || confirm() {
			g.scr = scPlay
		}
		if keyJust(ebiten.KeyQ) || padBtn(ebiten.StandardGamepadButtonRightRight) {
			g.scr = scTitle
		}
	case scOver:
		g.overT += 1.0 / 60
		if g.overT > .8 && (confirm() || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)) {
			g.scr = scTitle
		}
	}
	return nil
}

func (g *Game) titleCols() int { return 9 }

func (g *Game) updateTitle() {
	dx, dy := nav()
	n := len(g.roster)
	cols := g.titleCols()
	g.sel = (g.sel + dx + n) % n
	if dy != 0 {
		g.sel = ((g.sel+dy*cols)%n + n) % n
	}
	if keyJust(ebiten.KeyE, ebiten.KeyQ) || padBtn(ebiten.StandardGamepadButtonRightTop) {
		g.ability ^= 1
	}
	if keyJust(ebiten.KeyTab, ebiten.KeyB) || padBtn(ebiten.StandardGamepadButtonFrontTopLeft) {
		g.biome++
		if g.biome >= len(content.Types) {
			g.biome = -1
		}
	}
	if mx, my := ebiten.CursorPosition(); inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if i := g.titleHit(mx, my); i >= 0 {
			if i == g.sel {
				g.startSel()
				return
			}
			g.sel = i
		}
	}
	if confirm() {
		g.startSel()
	}
}

func (g *Game) startSel() {
	b := ""
	if g.biome >= 0 {
		b = content.Types[g.biome]
	}
	g.Start(g.roster[g.sel], b, rand.Int63())
}

func (g *Game) updatePlay() {
	w := g.W
	if keyJust(ebiten.KeyEscape, ebiten.KeyP) || padBtn(ebiten.StandardGamepadButtonCenterRight) {
		g.scr = scPause
		return
	}
	switch w.State {
	case sim.StateLevelUp, sim.StateRoute, sim.StateShrine:
		g.lvDelay += 1.0 / 60
		if g.auto && !g.Hold {
			if g.lvDelay > .6 {
				w.BotDecide()
				g.lvDelay = 0
			}
			return
		}
		n := g.choiceCount()
		dx, dy := nav()
		g.lvSel = (g.lvSel + dx + dy + n) % n
		for i := 0; i < n; i++ {
			if keyJust(ebiten.Key1+ebiten.Key(i)) || keyJust(ebiten.KeyNumpad1+ebiten.Key(i)) {
				g.pick(i)
				return
			}
		}
		if mx, my := ebiten.CursorPosition(); inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if i := g.cardHit(mx, my); i >= 0 {
				g.pick(i)
				return
			}
		}
		if g.lvDelay > .35 && confirm() {
			g.pick(g.lvSel)
		}
		return
	case sim.StateDead:
		g.scr = scOver
		g.overT = 0
		return
	}
	g.lvDelay = 0
	w.Step(1.0/60, g.playInput())
}

// choiceCount: cards on the current choice screen (shrines add "Leave it").
func (g *Game) choiceCount() int {
	switch g.W.State {
	case sim.StateRoute:
		return len(g.W.Route)
	case sim.StateShrine:
		return len(g.W.Boons) + 1
	}
	return len(g.W.Opts)
}

func (g *Game) pick(i int) {
	switch g.W.State {
	case sim.StateRoute:
		g.W.ChooseRoute(i)
	case sim.StateShrine:
		g.W.ChooseBoon(i)
	default:
		g.W.Choose(i)
	}
	g.lvSel = 0
	g.lvDelay = 0
}

func (g *Game) Draw(dst *ebiten.Image) {
	switch g.scr {
	case scTitle:
		g.drawTitle(dst)
	case scPlay, scPause, scOver:
		g.drawWorld(dst)
		g.drawHUD(dst)
		switch g.W.State {
		case sim.StateLevelUp:
			g.drawLevelUp(dst)
		case sim.StateRoute:
			g.drawRoute(dst)
		case sim.StateShrine:
			g.drawShrine(dst)
		}
		if g.scr == scPause {
			g.drawPause(dst)
		}
		if g.scr == scOver {
			g.drawOver(dst)
		}
	}
}

func fmtT(t float64) string { return fmt.Sprintf("%d:%02d", int(t)/60, int(t)%60) }

// FastForward runs the simulation (bot-driven) for t seconds without drawing; used to jump a demo
// into the action for screenshots.
func (g *Game) FastForward(t float64) { g.FastForwardUntil(t, -1) }

// FastForwardUntil runs the bot until time t, or until the run reaches state stop (route / shrine).
func (g *Game) FastForwardUntil(t float64, stop sim.State) {
	for g.W.T < t && g.W.State != sim.StateDead {
		if g.W.State == stop {
			return
		}
		if g.W.Waiting() {
			g.W.BotDecide()
			continue
		}
		g.W.Step(1.0/60, g.bot.Input(1.0/60))
	}
}

// SetAbility picks the starting (0) or hidden (1) ability for the next run.
func (g *Game) SetAbility(i int) { g.ability = i & 1 }
