package client

import (
	"fmt"
	"github.com/TxHandles/pokemon-survivor/engine/lockstep"
	"github.com/TxHandles/pokemon-survivor/engine/net"
	"io/fs"
	"math"
	"math/rand"
	"path/filepath"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/meta"
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
	lobbyRet bool   // browsing Shop / Pokémon / Pokédex from a co-op lobby
	lobbyTo  screen // that lobby (scOnline or scLobby)
	fr       friendsState
	rstArm   bool // Reset progress pressed once
	histCopy string
	amb      ambState
	vigImg   *ebiten.Image
	vigV     float64
	db       *content.DB
	assets   *Assets
	W        *sim.World
	floor    *Floor
	pa       playerAnim
	scr      screen
	sw, sh   int

	// title
	roster  []string
	sel     int
	biome   int // index into content.Types, -1 = Pokémon's own type
	ability int // 0 starting, 1 hidden
	// Hold keeps demo choice screens open (for screenshots)
	Hold   bool
	Freeze bool // testing: the world stops (screenshots of effects mid-animation)
	floorN int
	// level-up
	lvSel   int
	lvDelay float64
	// over
	overT float64

	auto bool // demo: the bot plays (title screen background / -demo flag)
	bot  *sim.Bot
	tick int

	// meta
	prof         *meta.Profile
	ui           uiState
	csel         string // selected Pokémon
	lineFam      string
	lineFrom     screen
	charQ, charF string
	cshow        map[string]string
	cardH        map[string]float64
	cardT        int
	dexQ         string
	dexSt, dexSo string
	dexTy        string
	dexForm      map[string]string
	dexTyOpen    bool
	bsel         string // run setup: type whose locations are shown
	menuBall     int    // main menu: which Poké Ball icon
	prevScr      screen
	collSel      string
	lifeSel      string
	chalOpen     bool
	chalCat      map[string]bool
	specIdx      int        // online co-op: which living teammate a fainted player watches
	tip          *tipData   // hover tooltip for this frame
	dItems       []drawItem // reused depth-sort buffer
	tipW         *sim.World // scratch world for menu tooltips' numbers
	tipWID       string     // challenge groups opened / closed by hand (default: open if something is claimable)
	lbNameEdit   string
	boardsFrom   screen
	pauseSel     int
	pauseQuit    bool
	retired      bool
	saveGen      int // bumped on every save: cached menu images depend on the save
	icache       map[string]*ebiten.Image
	icacheUsed   map[string]int
	toasts       []toastMsg
	clipIn       chan string
	xferMsg      string
	xferPending  *meta.Save
	end          endInfo
	finished     bool
	banMode      bool
	audio        *Audio
	touch        touchState
	settingsFrom screen
	cloud        Cloud
	cloudPending *meta.Save
	daily        *meta.Daily
	dexSel       string
	lbT, lbM     string
	lbNote       string
	lbRows       []meta.LBRow
	// local co-op
	seats     []*seat
	devs      []device
	coop      bool
	pas       map[int]*playerAnim
	on        onlineState
	nodraw    bool
	dataIDv   string
	stickPrev map[ebiten.GamepadID]int
}

func New(dataFS fs.FS, db *content.DB) *Game {
	uiFS = dataFS
	if runtime.GOOS != "js" { // keep the desktop's online account (and friend code) between launches
		net.IdentityFile = filepath.Join(filepath.Dir(meta.SavePath()), "online_id.txt")
	}
	loadArt(dataFS)
	loadFloors(dataFS)
	g := &Game{db: db, assets: NewAssets(dataFS, db), biome: -1, sw: 1280, sh: 720, clipIn: make(chan string, 1)}
	g.prof = meta.NewProfile(meta.LoadStored(), db)
	g.saveGen++
	g.csel = g.prof.S.Sel()
	if g.db.ByID[g.csel] == nil || !g.prof.Unl(g.csel) {
		g.csel = "fire"
	}
	g.scr = scMenu
	g.audio = NewAudio(g.opt)
	g.cloud = newCloud()
	g.dataIDv = net.DataID(dataFS)
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
	g.startCfg(id, biome, seed, sim.Config{Ability: g.ability, LI: -1})
}

// startCfg begins a run with a full meta configuration.
func (g *Game) startCfg(id, biome string, seed int64, cfg sim.Config) {
	if biome == "" {
		biome = g.db.ByID[id].Types[0]
	}
	w, err := sim.NewWith(g.db, id, biome, seed, cfg)
	if err != nil {
		panic(err)
	}
	w.ViewW, w.ViewH = float64(g.sw), float64(g.sh)
	g.W = w
	g.floor = NewFloor(biome, 0, seed)
	g.floorN = w.BiomeN
	g.pa = playerAnim{}
	g.pas = map[int]*playerAnim{}
	g.coop = false
	g.scr = scPlay
	g.bot = sim.NewBot(w, seed+1)
	if g.auto {
		w.AutoDash = true
	}
}

// Demo makes the bot play (used for screenshots and an attract mode).
func (g *Game) Demo(on bool) { g.auto = on }

// NoDraw disables rendering (testing).
func (g *Game) NoDraw(on bool) { g.nodraw = on }

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
	// the view size sets spawn distances, so it is part of the simulation: online, every peer keeps
	// the shared 1280×720 (a different window shape would put enemies in different places = desync)
	if g.W != nil && g.on.ls == nil {
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
	act2 := keyJust(ebiten.KeyControlLeft) || padBtn(ebiten.StandardGamepadButtonFrontTopLeft)
	return sim.Input{MoveX: x, MoveY: y, Dash: dash, Act2: act2}
}

// ---- update ----

func (g *Game) Update() error {
	g.tick++
	if g.scr != g.prevScr {
		g.histCopy, g.rstArm = "", false
		if g.scr == scOnline && g.lobbyRet { // back in the lobby: send any new upgrades / unlocks
			g.lobbyRet = false
			if r := g.on.room; r != nil {
				for i, id := range g.unlockedRoster() { // a Pokémon picked on the Pokémon screen
					if id == g.csel {
						g.on.idx = i
					}
				}
				if !r.Host {
					g.on.ready = false
				}
				r.SetMe(g.mySpec(), g.on.ready)
			}
		}
		if g.scr == scPlay || (g.scr == scLobby && g.lobbyRet) {
			g.lobbyRet = false
		}
		if g.scr == scMenu { // a different Poké Ball on the menu each visit, like the web version
			g.menuBall = rand.Intn(26)
		}
		g.prevScr = g.scr
	}
	defer g.audioTick()
	g.cloud.Poll(g.adoptCloud)
	g.friendsTick()
	g.lobbyKeepAlive()
	if p, ok := g.cloud.(interface{ NeedPush() bool }); ok && p.NeedPush() {
		g.cloud.PushSave(g.prof.S)
	}
	switch g.scr {
	case scTitle:
		g.scr = scMenu
	case scLobby:
		g.updateLobby()
	case scMenu, scChars, scSetup, scShop, scLine, scChal, scXfer, scResult, scSettings, scDex, scHist, scDaily, scBoards, scOnline, scColl, scLife, scFriends:
		g.buildMenu()
		g.ui.update()
		if g.ui.clicked {
			g.audio.Play("click")
		}
	case scPlay:
		// Settings → Auto-pause: pause when the window or tab loses focus (not online: the others keep playing)
		if g.opt("autop") != 0 && !ebiten.IsFocused() && !g.online() && !g.auto && !g.W.Waiting() {
			g.scr = scPause
			return nil
		}
		g.updatePlay()
	case scPause:
		if g.online() {
			g.onlineStep(sim.Input{})
			if g.W.State == sim.StateDead {
				g.finishRun()
				g.scr = scOver
				g.overT = 0
				return nil
			}
		}
		g.updatePause()
	case scOver:
		g.overT += 1.0 / 60
		if g.auto {
			if g.overT > .8 && (confirm() || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)) {
				g.scr = scMenu
			}
		} else if g.overT > 1.2 {
			g.scr = scResult
			g.ui.focus = ""
			if g.cloudPending != nil {
				s := g.cloudPending
				g.cloudPending = nil
				g.adoptCloud(s)
			}
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
	if g.Freeze { // testing: keep the frame still for a screenshot
		return
	}
	w := g.W
	w.UseLocal(g.viewIdx())
	// the HUD's pause button (mouse)
	if mx, my := ebiten.CursorPosition(); inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !w.Waiting() && !g.coop && inR(g.pauseBtnRect(), float64(mx), float64(my)) {
		g.scr = scPause
		g.pauseSel, g.pauseQuit = 0, false
		return
	}
	if g.online() {
		g.updateOnlinePlay()
		return
	}
	tin, tactive := g.touchInput()
	if keyJust(ebiten.KeyEscape, ebiten.KeyP) || padBtn(ebiten.StandardGamepadButtonCenterRight) || (g.touch.pauseTapped && !w.Waiting()) {
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
		g.choiceKeys()
		return
	case sim.StateDead:
		g.finishRun()
		g.scr = scOver
		g.overT = 0
		return
	}
	g.lvDelay = 0
	var in sim.Input
	if g.coop {
		for i, d := range g.devs {
			if d.kind == "bot" {
				continue
			}
			di := g.devInput(d)
			if d.kind == "kb" && tactive {
				di.MoveX, di.MoveY = tin.MoveX, tin.MoveY
				di.Dash = di.Dash || tin.Dash
				di.Act2 = di.Act2 || tin.Act2
			}
			if i == 0 {
				in = di
			} else {
				w.SetInput(i, di)
			}
		}
	} else {
		in = g.playInput()
		if tactive && !g.auto {
			in.MoveX, in.MoveY = tin.MoveX, tin.MoveY
			in.Dash = in.Dash || tin.Dash
			in.Act2 = in.Act2 || tin.Act2
		}
	}
	w.Step(1.0/60, in)
	for _, n := range w.Notes {
		g.toast(n)
	}
	w.Notes = nil
}

// choiceKeys handles the keys, clicks and taps on a choice screen (level-up, route, shrine).
func (g *Game) choiceKeys() {
	w := g.W
	if g.choiceKind() == "lv" {
		if keyJust(ebiten.KeyR) || padBtn(ebiten.StandardGamepadButtonRightLeft) {
			if g.online() {
				g.onlineCmd(lockstep.CmdReroll, 0)
			} else {
				w.Reroll()
			}
			g.banMode = false
		}
		if (keyJust(ebiten.KeyX) || padBtn(ebiten.StandardGamepadButtonRightTop)) && w.Banishes > 0 {
			g.banMode = !g.banMode
		}
	} else {
		g.banMode = false
	}
	n := g.choiceCount()
	if n == 0 {
		return
	}
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
	if tx, ty, ok := g.touchTap(); ok && g.lvDelay > .25 {
		if i := g.cardHit(int(tx), int(ty)); i >= 0 {
			g.pick(i)
			return
		}
	}
	if g.lvDelay > .35 && confirm() {
		g.pick(g.lvSel)
	}
}

// choiceCount: cards on the current choice screen (shrines add "Leave it").
func (g *Game) choiceCount() int {
	switch g.choiceKind() {
	case "route":
		if g.W.RouteGo >= 0 {
			return 0
		}
		return len(g.W.Route)
	case "shrine":
		return len(g.W.Boons) + 1
	case "lv":
		return len(g.W.Opts)
	}
	return 0
}

// viewIdx: the player whose screen this is (online: me; same-screen co-op: whoever is choosing).
func (g *Game) viewIdx() int {
	w := g.W
	if w == nil {
		return 0
	}
	if g.online() {
		return g.on.room.Me
	}
	if (w.State == sim.StateLevelUp || w.State == sim.StateShrine) && w.Chooser >= 0 {
		return w.Chooser
	}
	return 0
}

// choiceKind: which choice the viewed player has open: "lv", "shrine", "route" or "".
func (g *Game) choiceKind() string {
	w := g.W
	if w == nil {
		return ""
	}
	if w.State == sim.StateRoute {
		return "route"
	}
	return w.PendingKind(g.viewIdx())
}

func (g *Game) pick(i int) {
	if g.online() {
		switch g.choiceKind() {
		case "route":
			g.onlineCmd(lockstep.CmdRoute, i)
		case "shrine":
			g.onlineCmd(lockstep.CmdBoon, i)
		default:
			if g.banMode {
				g.onlineCmd(lockstep.CmdBanish, i)
				g.banMode = false
			} else {
				g.onlineCmd(lockstep.CmdChoose, i)
			}
		}
		g.lvSel = 0
		g.lvDelay = 0
		return
	}
	switch g.choiceKind() {
	case "route":
		g.W.ChooseRoute(i)
	case "shrine":
		g.W.ChooseBoon(i)
	default:
		if g.banMode {
			g.W.Banish(i)
			g.banMode = false
			return
		}
		g.W.Choose(i)
	}
	g.lvSel = 0
	g.lvDelay = 0
}

func (g *Game) Draw(dst *ebiten.Image) {
	if g.nodraw { // testing: skip rendering (headless browsers emulate the GPU slowly)
		if g.W != nil {
			drawText(dst, fmt.Sprintf("nodraw  t=%.1f  tick=%d", g.W.T, g.tick), 10, 10, 14, true, white, text.AlignStart, false)
		}
		return
	}
	switch g.scr {
	case scTitle:
		g.drawTitle(dst)
	case scPlay, scPause, scOver:
		g.W.UseLocal(g.viewIdx())
		g.drawWorld(dst)
		g.drawHUD(dst)
		switch g.choiceKind() {
		case "lv":
			g.drawLevelUp(dst)
		case "route":
			g.drawRoute(dst)
		case "shrine":
			g.drawShrine(dst)
		}
		g.drawOnlineOverlay(dst)
		if g.scr == scPlay && !g.W.Waiting() {
			g.drawTouch(dst)
		}
		if g.scr == scPause {
			g.drawPause(dst)
		}
		if g.scr == scOver {
			g.drawOver(dst)
		}
	case scLobby:
		g.drawLobby(dst)
	case scResult:
		g.drawWorld(dst)
		rect(dst, 0, 0, float64(g.sw), float64(g.sh), c(5, 8, 15, 215))
		g.ui.draw(dst)
	default: // menus
		g.drawBackdrop(dst)
		g.ui.draw(dst)
	}
	g.drawTip(dst)
	g.drawToasts(dst)
}

func (g *Game) drawBackdrop(dst *ebiten.Image) {
	t := float64(g.tick) / 60
	dst.Fill(c(16, 20, 32, 255))
	for i := 0; i < 14; i++ {
		x := math.Mod(float64(i)*173+t*12, float64(g.sw)+200) - 100
		soft(dst, x, 120+math.Sin(float64(i)*1.7+t*.3)*90+float64(i%5)*120, 160, 120, alpha(tc(content.Types[i%18]), .07))
	}
}

func (g *Game) drawToasts(dst *ebiten.Image) {
	keep := g.toasts[:0]
	y := 90.0
	for _, t := range g.toasts {
		t.t -= 1.0 / 60
		if t.t <= 0 {
			continue
		}
		a := math.Min(1, t.t/.5)
		tw, _ := text.Measure(t.s, font(14, true), 0)
		x := float64(g.sw) - tw - 40
		roundRect(dst, x, y, tw+24, 30, 8, alpha(c(20, 26, 40, 240), a))
		drawText(dst, t.s, x+12, y+7, 14, true, alpha(gold, a), text.AlignStart, false)
		y += 36
		keep = append(keep, t)
	}
	g.toasts = keep
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
