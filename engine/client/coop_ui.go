package client

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/meta"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Local co-op: up to four players on one screen. Each player owns an input device (the keyboard, or
// one gamepad); bots can fill empty seats.

const scLobby screen = 130

type device struct {
	kind string // kb | pad | bot
	pad  ebiten.GamepadID
}

type seat struct {
	dev     device
	id      string // Pokémon
	ability int
	ready   bool
	idx     int  // roster index
	fresh   bool // joined this frame: ignore its confirm press
}

var playerCols = [4]color.NRGBA{c(255, 90, 90, 255), c(79, 163, 255, 255), c(91, 227, 125, 255), c(255, 208, 74, 255)}

func (g *Game) devJust(d device, b ebiten.StandardGamepadButton, keys ...ebiten.Key) bool {
	switch d.kind {
	case "kb":
		return keyJust(keys...)
	case "pad":
		return ebiten.IsStandardGamepadLayoutAvailable(d.pad) && inpututil.IsStandardGamepadButtonJustPressed(d.pad, b)
	}
	return false
}

// devNav: left/right presses on a device (-1, 0, 1).
func (g *Game) devNav(d device) int {
	switch d.kind {
	case "kb":
		if keyJust(ebiten.KeyLeft, ebiten.KeyA) {
			return -1
		}
		if keyJust(ebiten.KeyRight, ebiten.KeyD) {
			return 1
		}
	case "pad":
		if !ebiten.IsStandardGamepadLayoutAvailable(d.pad) {
			return 0
		}
		if inpututil.IsStandardGamepadButtonJustPressed(d.pad, ebiten.StandardGamepadButtonLeftLeft) {
			return -1
		}
		if inpututil.IsStandardGamepadButtonJustPressed(d.pad, ebiten.StandardGamepadButtonLeftRight) {
			return 1
		}
		lx := ebiten.StandardGamepadAxisValue(d.pad, ebiten.StandardGamepadAxisLeftStickHorizontal)
		sx := 0
		if lx < -.6 {
			sx = -1
		} else if lx > .6 {
			sx = 1
		}
		prev := g.stickPrev[d.pad]
		g.stickPrev[d.pad] = sx
		if sx != 0 && sx != prev {
			return sx
		}
	}
	return 0
}

func (g *Game) seatHas(d device) bool {
	for _, s := range g.seats {
		if s.dev == d {
			return true
		}
	}
	return false
}

func (g *Game) unlockedRoster() []string {
	var l []string
	for _, id := range g.rosterAll() {
		if g.prof.Unl(id) {
			l = append(l, id)
		}
	}
	return l
}

func (g *Game) addSeat(d device) {
	if len(g.seats) >= 4 || g.seatHas(d) {
		return
	}
	r := g.unlockedRoster()
	idx := len(g.seats) % len(r)
	if d.kind == "kb" {
		for i, id := range r {
			if id == g.csel {
				idx = i
			}
		}
	}
	s := &seat{dev: d, idx: idx, id: r[idx], ability: g.prof.AbilityIdx(r[idx]), fresh: true}
	if d.kind == "bot" {
		s.ready = true
	}
	g.seats = append(g.seats, s)
	g.audio.Play("click")
}

func (g *Game) updateLobby() {
	if g.stickPrev == nil {
		g.stickPrev = map[ebiten.GamepadID]int{}
	}
	if keyJust(ebiten.KeyEscape) && len(g.seats) == 0 {
		g.scr = scMenu
		return
	}
	// joining
	if keyJust(ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyNumpadEnter) && !g.seatHas(device{kind: "kb"}) {
		g.addSeat(device{kind: "kb"})
	}
	for _, id := range pads() {
		if ebiten.IsStandardGamepadLayoutAvailable(id) && inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom) && !g.seatHas(device{kind: "pad", pad: id}) {
			g.addSeat(device{kind: "pad", pad: id})
		}
	}
	if keyJust(ebiten.KeyB, ebiten.KeyEqual, ebiten.KeyKPAdd) {
		g.addSeat(device{kind: "bot", pad: ebiten.GamepadID(100 + len(g.seats))})
	}
	r := g.unlockedRoster()
	// per-seat choices
	keep := g.seats[:0]
	for _, s := range g.seats {
		if s.fresh {
			s.fresh = false
			keep = append(keep, s)
			continue
		}
		if s.dev.kind != "bot" {
			if !s.ready {
				if d := g.devNav(s.dev); d != 0 {
					s.idx = ((s.idx+d)%len(r) + len(r)) % len(r)
					s.id = r[s.idx]
					s.ability = g.prof.AbilityIdx(s.id)
					g.audio.Play("click")
				}
				if g.devJust(s.dev, ebiten.StandardGamepadButtonRightTop, ebiten.KeyE, ebiten.KeyQ) && g.prof.HiddenOwned(s.id) {
					s.ability ^= 1
				}
			}
			if g.devJust(s.dev, ebiten.StandardGamepadButtonRightBottom, ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyNumpadEnter) {
				s.ready = !s.ready
				g.audio.Play("click")
			}
			if g.devJust(s.dev, ebiten.StandardGamepadButtonRightRight, ebiten.KeyEscape, ebiten.KeyBackspace) {
				if s.ready {
					s.ready = false
				} else {
					continue // leave
				}
			}
		}
		keep = append(keep, s)
	}
	g.seats = keep
	// mouse: remove bots / go back / start
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		fx, fy := float64(mx), float64(my)
		if inR(g.lobbyBtn(0), fx, fy) {
			g.scr = scMenu
		}
		if inR(g.lobbyBtn(1), fx, fy) {
			g.addSeat(device{kind: "bot", pad: ebiten.GamepadID(100 + len(g.seats))})
		}
		if inR(g.lobbyBtn(2), fx, fy) && len(g.seats) > 0 {
			g.seats = g.seats[:len(g.seats)-1]
		}
	}
	humans, ready := 0, true
	for _, s := range g.seats {
		if s.dev.kind != "bot" {
			humans++
			ready = ready && s.ready
		}
	}
	if len(g.seats) > 0 && humans > 0 && ready {
		g.startCoop()
	}
}

func (g *Game) lobbyBtn(i int) [4]float64 {
	sw, sh := float64(g.sw), float64(g.sh)
	switch i {
	case 0:
		return [4]float64{20, 20, 110, 40}
	case 1:
		return [4]float64{sw/2 - 230, sh - 70, 220, 44}
	}
	return [4]float64{sw/2 + 10, sh - 70, 220, 44}
}

func (g *Game) drawLobby(dst *ebiten.Image) {
	g.drawBackdrop(dst)
	sw, sh := float64(g.sw), float64(g.sh)
	drawText(dst, "LOCAL CO-OP", sw/2, 26, 32, true, gold, text.AlignCenter, true)
	drawText(dst, "Keyboard: Enter to join  ·  Gamepad: A to join  ·  B key: add a bot  ·  Up to 4 players share the screen", sw/2, 68, 13, false, dim, text.AlignCenter, true)
	btnDraw("< Back", "", dim, false)(dst, g.lobbyBtn(0), false)
	btnDraw("Add bot", "", c(159, 232, 255, 255), false)(dst, g.lobbyBtn(1), false)
	btnDraw("Remove last", "", c(255, 140, 140, 255), len(g.seats) == 0)(dst, g.lobbyBtn(2), false)
	cw := math.Min(300, (sw-80)/4)
	x0 := sw/2 - (cw*4+30)/2
	for i := 0; i < 4; i++ {
		x, y := x0+float64(i)*(cw+10), 110.0
		h := sh - 210
		col := playerCols[i]
		if i >= len(g.seats) {
			roundRect(dst, x, y, cw, h, 14, c(20, 25, 38, 160))
			drawText(dst, fmt.Sprintf("Player %d", i+1), x+cw/2, y+h/2-30, 18, true, alpha(dim, .6), text.AlignCenter, false)
			drawText(dst, "Press Enter / A to join", x+cw/2, y+h/2, 12, false, alpha(dim, .6), text.AlignCenter, false)
			continue
		}
		s := g.seats[i]
		p := g.db.ByID[s.id]
		roundRect(dst, x-2, y-2, cw+4, h+4, 16, alpha(col, b2f(s.ready, .95, .45)))
		roundRect(dst, x, y, cw, h, 14, c(22, 28, 42, 245))
		dev := "Keyboard"
		switch s.dev.kind {
		case "pad":
			dev = "Gamepad " + fmt.Sprint(int(s.dev.pad)+1)
		case "bot":
			dev = "Bot"
		}
		drawText(dst, fmt.Sprintf("Player %d · %s", i+1, dev), x+cw/2, y+14, 14, true, col, text.AlignCenter, false)
		if sp := g.assets.Sprite(s.id); sp != nil {
			fr := sp.Front.IdleFrame(g.tick * 1000 / 60)
			k := math.Min((cw-60)/float64(sp.Front.V.CW), 150/float64(sp.Front.V.CH))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(x+cw/2-float64(sp.Front.V.CW)*k/2, y+50)
			op.Filter = ebiten.FilterLinear
			dst.DrawImage(sp.Front.Frame(fr), op)
		}
		drawText(dst, p.Name, x+cw/2, y+215, 20, true, tc(p.Types[0]), text.AlignCenter, false)
		ab := g.db.AbilityPair(s.id)[s.ability]
		drawText(dst, ab.Name, x+cw/2, y+244, 13, true, white, text.AlignCenter, false)
		for j, ln := range wrap(ab.Desc, font(11, false), cw-30) {
			if j > 3 {
				break
			}
			drawText(dst, ln, x+cw/2, y+264+float64(j)*15, 11, false, dim, text.AlignCenter, false)
		}
		st := ""
		if s.dev.kind == "bot" {
			st = "Plays by itself"
		} else if s.ready {
			st = "READY  (B / Esc to change)"
		} else {
			st = "Left / Right: Pokémon  ·  E / Y: ability  ·  Enter / A: ready"
		}
		for j, ln := range wrap(st, font(12, true), cw-24) {
			drawText(dst, ln, x+cw/2, y+h-50+float64(j)*16, 12, true, b2c(s.ready, c(120, 255, 140, 255), dim), text.AlignCenter, false)
		}
	}
	humans := 0
	for _, s := range g.seats {
		if s.dev.kind != "bot" {
			humans++
		}
	}
	if len(g.seats) > 0 && humans == 0 {
		drawText(dst, "Add at least one human player to start", sw/2, sh-84, 13, true, c(255, 140, 140, 255), text.AlignCenter, true)
	} else if len(g.seats) > 0 {
		drawText(dst, "The run starts when every player is ready", sw/2, sh-84, 13, false, dim, text.AlignCenter, true)
	}
}

func (g *Game) startCoop() {
	pr := g.prof
	rs := pr.S.RunSetup()
	bi, _ := rs["bi"].(string)
	li := -1
	if bi == "" || bi == "random" || !pr.BiomeUnlocked(bi) {
		bi = typesList()[rand.Intn(18)]
	} else {
		li = int(num(rs["li"]))
	}
	var specs []sim.PlayerSpec
	g.devs = g.devs[:0]
	for i, s := range g.seats {
		fam := pr.Fam(s.id)
		up := map[string]int{}
		for _, k := range meta.Upgrades {
			up[k.Key] = pr.UL(k.Key, fam)
		}
		kind := "local"
		if s.dev.kind == "bot" {
			kind = "bot"
		}
		specs = append(specs, sim.PlayerSpec{ID: s.id, Name: fmt.Sprintf("P%d %s", i+1, g.db.ByID[s.id].Name), Kind: kind, Ability: s.ability, Up: up, Training: pr.Training(fam)})
		g.devs = append(g.devs, s.dev)
	}
	lock, _ := rs["lock"].(bool)
	cfg := sim.Config{LI: li, Tier: g.tier(), Mods: g.mods(), Stage: g.stage(), Lock: lock, Up: specs[0].Up, Training: specs[0].Training}
	id0 := specs[0].ID
	cfg.OnBoss = func(b, et string, l int, fin bool, visited []string) []string {
		m := pr.BossKill(b, et, l, fin, cfg.Stage, id0, visited)
		g.save()
		return m
	}
	w, err := sim.NewCoop(g.db, specs, bi, rand.Int63(), cfg)
	if err != nil {
		g.toast(err.Error())
		return
	}
	w.ViewW, w.ViewH = float64(g.sw), float64(g.sh)
	g.W = w
	g.floor = NewFloor(bi, w.LI, w.Seed)
	g.floorN = w.BiomeN
	g.pas = map[int]*playerAnim{}
	g.scr = scPlay
	g.bot = sim.NewBot(w, w.Seed+1)
	g.finished = false
	g.coop = true
}

func typesList() []string {
	return []string{"normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"}
}

// devInput reads one player's device.
func (g *Game) devInput(d device) sim.Input {
	x, y := 0.0, 0.0
	dash := false
	switch d.kind {
	case "kb":
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
		dash = keyJust(ebiten.KeyShiftLeft, ebiten.KeyShiftRight)
	case "pad":
		id := d.pad
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			break
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
		dash = inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonFrontTopRight)
	}
	return sim.Input{MoveX: x, MoveY: y, Dash: dash}
}

// drawCenter draws the Pokémon Center (co-op revive point).
func (g *Game) drawCenter(dst *ebiten.Image, sx, sy func(float64) float64) {
	pc := g.W.PCN
	if pc == nil {
		return
	}
	x, y := sx(pc.X), sy(pc.Y)
	col := c(255, 110, 150, 255)
	soft(dst, x, y+14, 40, 14, c(0, 0, 0, 120))
	soft(dst, x, y, 70, 70, alpha(col, .2+.08*math.Sin(g.W.T*3)))
	ring(dst, x, y, 60, 1.5, alpha(col, .5))
	rect(dst, x-26, y-10, 52, 30, c(235, 235, 245, 255))
	poly(dst, [][2]float64{{x - 30, y - 10}, {x, y - 32}, {x + 30, y - 10}}, col)
	rect(dst, x-4, y-4, 8, 20, col)
	rect(dst, x-10, y+2, 20, 8, col)
	if pc.P > 0 {
		sector(dst, x, y-48, 10, -math.Pi/2+math.Pi*pc.P/1.5, math.Pi*pc.P/1.5, alpha(c(120, 255, 140, 255), .95))
		ring(dst, x, y-48, 10, 1.5, c(255, 255, 255, 160))
	}
	drawText(dst, "Pokémon Center", x, y+26, 11, true, white, text.AlignCenter, true)
}

// drawCoopHUD: compact bars for every player.
func (g *Game) drawCoopHUD(dst *ebiten.Image) {
	w := g.W
	y := 16.0
	for i, pc := range w.PL {
		P := pc.P
		col := playerCols[i%4]
		roundRect(dst, 12, y, 230, 42, 8, panel)
		rect(dst, 12, y+6, 4, 30, col)
		if s := g.assets.Sprite(P.ID); s != nil {
			fr := s.Front.Frame(s.Front.IdleFrame(0))
			op := &ebiten.DrawImageOptions{}
			k := 30 / math.Max(float64(s.Front.V.CW), float64(s.Front.V.CH))
			op.GeoM.Scale(k, k)
			op.GeoM.Translate(22, y+6)
			op.Filter = ebiten.FilterLinear
			if P.Dead {
				op.ColorScale.Scale(.4, .4, .5, 1)
			}
			dst.DrawImage(fr, op)
		}
		name := pc.Name
		if P.Dead {
			name += "  (fainted)"
		}
		drawText(dst, name, 60, y+5, 12, true, b2c(P.Dead, c(150, 156, 170, 255), white), text.AlignStart, false)
		hpf := P.HP / P.MHP
		hc := c(80, 220, 110, 255)
		if hpf < .5 {
			hc = c(255, 200, 70, 255)
		}
		if hpf < .25 {
			hc = c(255, 80, 80, 255)
		}
		bar(dst, 60, y+24, 170, 9, hpf, hc, c(40, 46, 60, 255))
		y += 48
	}
	bar(dst, 0, 0, float64(g.sw), 8, w.XPS.XP/w.XPS.NX, c(90, 190, 255, 255), c(20, 26, 40, 230))
	drawText(dst, fmt.Sprintf("Team Lv %d  ·  %d coins", w.XPS.Lv, int(w.Coins)), 22, y+2, 13, true, gold, text.AlignStart, true)
}

// drawStandings: the co-op results table.
func (g *Game) drawStandings(dst *ebiten.Image, x, y, w float64) float64 {
	st := g.W.Standings()
	drawText(dst, "Player", x, y, 12, true, dim, text.AlignStart, false)
	drawText(dst, "KOs", x+w*.45, y, 12, true, dim, text.AlignEnd, false)
	drawText(dst, "Damage", x+w*.65, y, 12, true, dim, text.AlignEnd, false)
	drawText(dst, "Faints", x+w*.82, y, 12, true, dim, text.AlignEnd, false)
	drawText(dst, "Revives", x+w, y, 12, true, dim, text.AlignEnd, false)
	y += 20
	for i, s := range st {
		col := playerCols[i%4]
		drawText(dst, s.Name, x, y, 13, true, col, text.AlignStart, false)
		drawText(dst, commas(s.Kills), x+w*.45, y, 13, false, white, text.AlignEnd, false)
		drawText(dst, commas(int(s.Dmg)), x+w*.65, y, 13, false, white, text.AlignEnd, false)
		drawText(dst, fmt.Sprint(s.Deaths), x+w*.82, y, 13, false, white, text.AlignEnd, false)
		drawText(dst, fmt.Sprint(s.Revives), x+w, y, 13, false, white, text.AlignEnd, false)
		y += 20
	}
	return y
}

// DemoCoop starts a bot-only co-op run (screenshots / testing).
func (g *Game) DemoCoop(ids []string, biome string, seed int64) {
	g.seats = nil
	for i, id := range ids {
		if g.db.ByID[id] == nil {
			continue
		}
		g.seats = append(g.seats, &seat{dev: device{kind: "bot", pad: ebiten.GamepadID(100 + i)}, id: id, ready: true})
	}
	g.auto = true
	g.startCoop()
	if biome != "" && g.W != nil {
		_ = biome
	}
	if seed != 0 && g.W != nil {
		_ = seed
	}
}
