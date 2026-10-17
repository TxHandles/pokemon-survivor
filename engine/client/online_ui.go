package client

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/lockstep"
	"github.com/TxHandles/pokemon-survivor/engine/meta"
	"github.com/TxHandles/pokemon-survivor/engine/net"
	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Online co-op: a room joined by code; the run itself is simulated in lockstep on every peer.

const scOnline screen = 160

const inputDelay = 4 // ticks (~67 ms) of input delay for online runs

type onlineState struct {
	mode      string // menu | join | lobby
	code      string // code being typed
	room      *net.Room
	ls        *lockstep.Session
	cmds      []lockstep.Cmd
	idx       int // my roster index
	ability   int
	ready     bool
	stallT    float64 // seconds the sim has been waiting on others
	note      string  // shown on the results screen (disconnects, desync)
	sigMode   string  // "" (Firestore) | local | http (testing)
	net       net.Net
	uid       string
	name      string
	loginBusy bool
	loginErr  string
	loginAt   time.Time
	mu        sync.Mutex
	events    []func()
	started   *net.StartMsg // the start we already acted on
	autoGo    bool          // testing: ready up / start as soon as possible
	autoKind  string
	autoCode  string
}

func (g *Game) online() bool { return g.on.room != nil && g.on.ls != nil }

func (g *Game) mySpec() sim.PlayerSpec {
	r := g.unlockedRoster()
	if len(r) == 0 {
		return sim.PlayerSpec{ID: "fire"}
	}
	g.on.idx = ((g.on.idx % len(r)) + len(r)) % len(r)
	id := r[g.on.idx]
	pr := g.prof
	fam := pr.Fam(id)
	up := map[string]int{}
	for _, k := range meta.Upgrades {
		up[k.Key] = pr.UL(k.Key, fam)
	}
	return sim.PlayerSpec{ID: id, Ability: g.on.ability & 1, Up: up, Training: pr.Training(fam)}
}

func (g *Game) onlineName() string { return nonEmptyStr(g.on.name, "Player") }
func (g *Game) onlineUID() string  { return g.on.uid }

func nonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// onlineNet is the platform transport (one instance; it keeps the sign-in).
func (g *Game) onlineNet() net.Net {
	if g.on.net == nil {
		g.on.net = net.NewNet(g.on.sigMode)
	}
	return g.on.net
}

// onlineLogin resolves our uid / name once (async); the screen shows progress until it is done.
func (g *Game) onlineLogin() {
	if g.on.uid != "" || g.on.loginBusy || (g.on.loginErr != "" && time.Since(g.on.loginAt) < 30*time.Second) {
		return
	}
	g.on.loginAt = time.Now()
	n := g.onlineNet()
	if ok, why := n.Available(); !ok {
		g.on.loginErr = why
		return
	}
	g.on.loginBusy = true
	g.on.loginErr = ""
	n.Login(func(uid, name, err string) {
		g.on.mu.Lock()
		g.on.events = append(g.on.events, func() {
			g.on.loginBusy = false
			g.on.uid, g.on.name, g.on.loginErr = uid, name, err
		})
		g.on.mu.Unlock()
	})
}

// onlinePoll runs events posted from other goroutines (call once per frame on online screens).
func (g *Game) onlinePoll() {
	g.on.mu.Lock()
	ev := g.on.events
	g.on.events = nil
	g.on.mu.Unlock()
	for _, f := range ev {
		f()
	}
}

func (g *Game) hostRoom() {
	n := g.onlineNet()
	if ok, why := n.Available(); !ok {
		g.toast(why)
		return
	}
	if g.on.uid == "" {
		g.toast(nonEmptyStr(g.on.loginErr, "Still signing in…"))
		return
	}
	g.leaveRoom()
	for i, id := range g.unlockedRoster() {
		if id == g.csel {
			g.on.idx = i
		}
	}
	g.on.ability = g.prof.AbilityIdx(g.csel)
	g.on.room = net.HostRoom(n, g.onlineUID(), g.onlineName(), g.mySpec(), g.dataIDv)
	g.on.room.SetLabel(g.setupSummary())
	g.on.mode = "lobby"
	g.on.ready = true
	g.ui.focus = ""
}

func (g *Game) joinRoom(code string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 5 {
		g.toast("Room codes are 5 characters")
		return
	}
	n := g.onlineNet()
	if ok, why := n.Available(); !ok {
		g.toast(why)
		return
	}
	if g.on.uid == "" {
		g.toast(nonEmptyStr(g.on.loginErr, "Still signing in…"))
		return
	}
	g.leaveRoom()
	for i, id := range g.unlockedRoster() {
		if id == g.csel {
			g.on.idx = i
		}
	}
	g.on.ability = g.prof.AbilityIdx(g.csel)
	g.on.ready = false
	g.on.room = net.JoinRoom(n, code, g.onlineUID(), g.onlineName(), g.mySpec(), g.dataIDv)
	g.on.mode = "lobby"
	g.ui.focus = ""
}

func (g *Game) leaveRoom() {
	if g.on.room != nil {
		if g.on.room.Host {
			g.onlineNet().SetRoomOpen(g.on.room.Code, false)
		}
		g.on.room.Leave()
	}
	g.on.room, g.on.ls = nil, nil
	g.on.cmds = nil
}

// ---- menu screen ----

func (g *Game) menuOnline() {
	sw := float64(g.sw)
	u := &g.ui
	g.onlinePoll()
	r := g.on.room
	switch g.on.mode {
	case "join":
		g.header("JOIN A ROOM", "Type the 5-character code your friend sees in their lobby")
		// back returns to the Online menu (same screen, different mode); Backspace only deletes
		u.button("back", [4]float64{20, 20, 110, 40}, false, false, btnDraw("< Back", "", dim, false), func() { g.on.mode = "menu"; g.ui.focus = "" })
		if keyJust(ebiten.KeyEscape) || padBtn(ebiten.StandardGamepadButtonRightRight) {
			g.on.mode = "menu"
			g.ui.focus = ""
			return
		}
		for _, ch := range ebiten.AppendInputChars(nil) {
			if len(g.on.code) < 5 && (ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
				g.on.code += strings.ToUpper(string(ch))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.on.code) > 0 {
			g.on.code = g.on.code[:len(g.on.code)-1]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && len(g.on.code) == 5 {
			g.joinRoom(g.on.code)
		}
		x := sw/2 - 200
		u.label([4]float64{x, 150, 400, 80}, false, func(dst *ebiten.Image, r [4]float64) {
			roundRect(dst, r[0], r[1], r[2], r[3], 12, panel)
			s := g.on.code
			if len(s) < 5 && g.tick/30%2 == 0 {
				s += "_"
			}
			drawText(dst, s, r[0]+r[2]/2, r[1]+22, 36, true, gold, text.AlignCenter, false)
		})
		u.button("go", [4]float64{x, 250, 400, 50}, false, len(g.on.code) != 5, btnDraw("Connect", "", gold, len(g.on.code) != 5), func() { g.joinRoom(g.on.code) })
		return
	case "lobby":
		if r == nil {
			g.on.mode = "menu"
			return
		}
		g.menuRoomLobby()
		return
	}
	g.header("ONLINE CO-OP", "Play with friends over the internet · up to 4 players · everyone needs the same version")
	g.backButton(scMenu)
	x := sw/2 - 300
	ok, why := g.onlineNet().Available()
	if ok {
		g.onlineLogin()
		switch {
		case g.on.uid != "":
			why = "Ready · playing as " + g.onlineName()
			if g.on.autoKind != "" { // testing: host / join right away
				k, c := g.on.autoKind, g.on.autoCode
				g.on.autoKind = ""
				if k == "host" {
					g.hostRoom()
				} else {
					g.joinRoom(c)
				}
				return
			}
		case g.on.loginErr != "":
			ok, why = false, g.on.loginErr
		default:
			ok, why = false, "Signing in…"
		}
	}
	u.label([4]float64{x, 100, 600, 118}, false, func(dst *ebiten.Image, rr [4]float64) {
		roundRect(dst, rr[0], rr[1], rr[2], rr[3], 12, panel)
		lines := []string{
			"One player hosts a room and shares its 5-character code; the others join with it.",
			"Every player's game runs the same simulation in lockstep, so nobody has a host advantage.",
			"Each player picks a different Pokémon and uses their own upgrades. The host picks the run setup.",
		}
		for i, ln := range lines {
			for j, w := range wrap(ln, font(12, false), rr[2]-40) {
				drawText(dst, w, rr[0]+20, rr[1]+16+float64(i)*30+float64(j)*14, 12, false, dim, text.AlignStart, false)
			}
		}
		drawText(dst, why, rr[0]+20, rr[1]+rr[3]-24, 12, true, b2c(ok, c(120, 255, 140, 255), c(255, 140, 140, 255)), text.AlignStart, false)
	})
	u.button("host", [4]float64{x, 236, 290, 54}, false, !ok, btnDraw("Host a room", "Get a code to share", gold, !ok), func() { g.hostRoom() })
	u.button("join", [4]float64{x + 310, 236, 290, 54}, false, !ok, btnDraw("Join a room", "Enter a friend's code", c(159, 232, 255, 255), !ok), func() { g.on.mode = "join"; g.on.code = ""; g.ui.focus = "" })
	if !ok && !g.cloud.Online() && g.cloud.Available() {
		u.button("signin", [4]float64{x, 300, 600, 46}, false, false, btnDraw("Sign in with Google", "Needed to find each other's rooms", c(159, 232, 255, 255), false), func() { g.cloud.SignIn() })
	}
	u.button("setup", [4]float64{x, 356, 600, 46}, false, false, btnDraw("Run setup", g.setupSummary(), c(159, 232, 255, 255), false), func() { g.scr = scSetup; g.settingsFrom = scOnline; g.ui.focus = "" })
}

// lobbyKeepAlive keeps the room running while its player browses other screens, and starts
// the run if the host starts it meanwhile.
func (g *Game) lobbyKeepAlive() {
	r := g.on.room
	if r == nil || g.on.ls != nil || g.scr == scOnline || !g.lobbyRet {
		return
	}
	r.Update()
	if r.Started && r.Start != nil && r.Start != g.on.started {
		g.lobbyRet = false
		g.startOnline(r.Start)
	}
}

func (g *Game) menuRoomLobby() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	r := g.on.room
	r.Update()
	if r.Started && r.Start != nil && r.Start != g.on.started && g.on.ls == nil { // the host started
		g.startOnline(r.Start)
		return
	}
	title := "ROOM " + r.Code
	if r.Code == "" {
		title = "ONLINE CO-OP"
	}
	g.header(title, r.Status)
	// the lobby handles its own keys (left / right pick a Pokémon, Enter readies / starts), so its
	// buttons are plain labels with manual click handling rather than focusable widgets
	leaveR, mainR := [4]float64{20, 20, 110, 40}, [4]float64{sw/2 - 160, sh - 80, 320, 50}
	mx, my := ebiten.CursorPosition()
	click := func(rr [4]float64) bool {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && inR(rr, float64(mx), float64(my)) {
			return true
		}
		if tx, ty, ok := g.touchTap(); ok && inR(rr, tx, ty) {
			return true
		}
		return false
	}
	u.label(leaveR, false, func(dst *ebiten.Image, rr [4]float64) {
		btnDraw("< Leave", "", dim, false)(dst, rr, inR(rr, float64(mx), float64(my)))
	})
	// shop, Pokémon and Pokédex without leaving the room
	bx := leaveR[0] + leaveR[2] + 12
	for _, b := range []struct {
		label string
		icon  any
		to    screen
	}{{"Shop", gCart, scShop}, {"Pokémon", uiIcon("ball0.png"), scChars}, {"Pokédex", uiIcon("dex.png"), scDex}} {
		b := b
		w := smallBtnW(b.label, b.icon, " ")
		br := [4]float64{bx, 25, w, 30}
		u.label(br, false, func(dst *ebiten.Image, rr [4]float64) {
			smallBtn(dst, rr, b.label, inR(rr, float64(mx), float64(my)), false, b.icon, " ")
		})
		if click(br) {
			g.lobbyRet, g.lobbyTo = true, scOnline
			g.scr = b.to
			g.ui.focus = ""
			g.ui.scroll = 0
			return
		}
		bx += w + 8
	}
	if click(leaveR) || keyJust(ebiten.KeyEscape) || padBtn(ebiten.StandardGamepadButtonRightRight) {
		g.leaveRoom()
		g.on.mode = "menu"
		g.ui.focus = ""
		return
	}
	if r.Err != "" {
		u.label([4]float64{sw/2 - 300, 110, 600, 60}, false, func(dst *ebiten.Image, rr [4]float64) {
			roundRect(dst, rr[0], rr[1], rr[2], rr[3], 12, panel)
			for i, ln := range wrap(r.Err, font(13, true), rr[2]-40) {
				drawText(dst, ln, rr[0]+rr[2]/2, rr[1]+14+float64(i)*18, 13, true, c(255, 140, 140, 255), text.AlignCenter, false)
			}
		})
		return
	}
	// my seat controls
	roster := g.unlockedRoster()
	changed := false
	if dx := nav1(); dx != 0 && len(roster) > 0 && !(g.on.ready && !r.Host) {
		g.on.idx = ((g.on.idx+dx)%len(roster) + len(roster)) % len(roster)
		changed = true
	}
	if (keyJust(ebiten.KeyE, ebiten.KeyQ) || padBtn(ebiten.StandardGamepadButtonRightTop)) && !(g.on.ready && !r.Host) {
		g.on.ability ^= 1
		changed = true
	}
	if changed {
		g.on.ready = r.Host
		r.SetMe(g.mySpec(), g.on.ready)
	}
	if r.Host {
		r.SetLabel(g.setupSummary())
	}
	// seats
	cw := math.Min(300, (sw-80)/4)
	x0 := sw/2 - (cw*4+30)/2
	for i := 0; i < 4; i++ {
		i := i
		x, y, h := x0+float64(i)*(cw+10), 110.0, sh-230
		u.label([4]float64{x, y, cw, h}, false, func(dst *ebiten.Image, rr [4]float64) {
			col := playerCols[i]
			if i >= len(r.Players) {
				roundRect(dst, x, y, cw, h, 14, c(20, 25, 38, 160))
				ty := y + h/2
				if r.Host && i == len(r.Players) {
					ty = y + h - 70 // the friends list sits above
				}
				drawText(dst, "Open seat", x+cw/2, ty-10, 18, true, alpha(dim, .6), text.AlignCenter, false)
				if r.Host && r.Code != "" {
					drawText(dst, "Share the code "+r.Code, x+cw/2, ty+16, 12, false, alpha(dim, .6), text.AlignCenter, false)
				}
				return
			}
			p := r.Players[i]
			pk := g.db.ByID[p.Spec.ID]
			roundRect(dst, x-2, y-2, cw+4, h+4, 16, alpha(col, b2f(p.Ready, .95, .45)))
			roundRect(dst, x, y, cw, h, 14, c(22, 28, 42, 245))
			who := p.Name
			if p.Host {
				who += " (host)"
			}
			if i == r.Me {
				who += " · you"
			}
			drawText(dst, who, x+cw/2, y+14, 14, true, col, text.AlignCenter, false)
			if pk != nil {
				if sp := g.assets.Sprite(pk.ID); sp != nil {
					fr := sp.Front.IdleFrame(g.tick * 1000 / 60)
					k := math.Min((cw-60)/float64(sp.Front.V.CW), 150/float64(sp.Front.V.CH))
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(k, k)
					op.GeoM.Translate(x+cw/2-float64(sp.Front.V.CW)*k/2, y+50)
					op.Filter = ebiten.FilterLinear
					dst.DrawImage(sp.Front.Frame(fr), op)
				}
				drawText(dst, pk.Name, x+cw/2, y+215, 20, true, tc(pk.Types[0]), text.AlignCenter, false)
				ab := g.db.AbilityPair(pk.ID)[p.Spec.Ability&1]
				drawText(dst, ab.Name, x+cw/2, y+244, 13, true, white, text.AlignCenter, false)
				for j, ln := range wrap(ab.Desc, font(11, false), cw-30) {
					if j > 3 {
						break
					}
					drawText(dst, ln, x+cw/2, y+264+float64(j)*15, 11, false, dim, text.AlignCenter, false)
				}
			}
			st, sc := "", dim
			switch {
			case p.Dup:
				st, sc = "Same Pokémon as another player — pick a different one", c(255, 140, 140, 255)
			case p.Host:
				st, sc = "Host", c(120, 255, 140, 255)
			case p.Ready:
				st, sc = "READY", c(120, 255, 140, 255)
			case i == r.Me:
				st = "Left / Right: Pokémon  ·  E / Y: ability  ·  Enter / A: ready"
			default:
				st = "Choosing…"
			}
			for j, ln := range wrap(st, font(12, true), cw-24) {
				drawText(dst, ln, x+cw/2, y+h-50+float64(j)*16, 12, true, sc, text.AlignCenter, false)
			}
		})
	}
	// my seat: click the arrows to change Pokémon, click the ability to switch it
	if r.Me >= 0 && r.Me < len(r.Players) && len(roster) > 0 {
		x, y := x0+float64(r.Me)*(cw+10), 110.0
		can := !(g.on.ready && !r.Host)
		lr, rr := [4]float64{x + 6, y + 100, 34, 56}, [4]float64{x + cw - 40, y + 100, 34, 56}
		ar := [4]float64{x + 10, y + 236, cw - 20, 26}
		arrow := func(dst *ebiten.Image, b [4]float64, dir float64) {
			hot := inR(b, float64(mx), float64(my))
			col := alpha(white, b2f(can, b2f(hot, 1, .75), .25))
			roundRect(dst, b[0], b[1], b[2], b[3], 8, alpha(c(40, 48, 70, 255), b2f(hot && can, .95, .7)))
			cx, cy := b[0]+b[2]/2, b[1]+b[3]/2
			poly(dst, [][2]float64{{cx + 7*dir, cy}, {cx - 6*dir, cy - 11}, {cx - 6*dir, cy + 11}}, col)
		}
		u.label([4]float64{x, y, cw, 300}, false, func(dst *ebiten.Image, _ [4]float64) {
			arrow(dst, lr, -1)
			arrow(dst, rr, 1)
			if can && inR(ar, float64(mx), float64(my)) {
				webBox(dst, ar[0], ar[1], ar[2], ar[3], 8, alpha(webAcc, .8), c(0, 0, 0, 0))
			}
			hint := "Click the arrows to change Pokémon · click the ability to switch"
			if !can {
				hint = "Click Not ready to change your Pokémon"
			}
			for j, ln := range wrap(hint, font(11, false), cw-20) {
				drawText(dst, ln, x+cw/2, y+(sh-230)-92+float64(j)*14, 11, false, alpha(webFg, .75), text.AlignCenter, false)
			}
		})
		if can {
			n := len(roster)
			switch {
			case click(lr):
				g.on.idx = (g.on.idx - 1 + n) % n
				g.on.ready = r.Host
				r.SetMe(g.mySpec(), g.on.ready)
			case click(rr):
				g.on.idx = (g.on.idx + 1) % n
				g.on.ready = r.Host
				r.SetMe(g.mySpec(), g.on.ready)
			case click(ar):
				g.on.ability ^= 1
				g.on.ready = r.Host
				r.SetMe(g.mySpec(), g.on.ready)
			}
		}
	}
	if r.Host && r.Code != "" && len(r.Players) < 4 { // invite friends into the first open seat
		g.lobbyInvites(x0+float64(len(r.Players))*(cw+10), 150, cw, click)
	}
	// bottom: run setup label + start / ready
	u.label([4]float64{0, sh - 108, sw, 20}, false, textDraw("Run setup (host): "+r.Label, 13, false, dim, text.AlignCenter))
	if g.on.autoGo {
		if r.Host {
			if ok, _ := r.CanStart(); ok {
				g.hostStart()
				return
			}
		} else if r.Connected() && (!g.on.ready || r.Players[r.Me].Dup) {
			if r.Players[r.Me].Dup {
				g.on.idx++
			}
			g.on.ready = true
			r.SetMe(g.mySpec(), true)
		}
	}
	if r.Host {
		ok, why := r.CanStart()
		u.label(mainR, false, func(dst *ebiten.Image, rr [4]float64) {
			btnDraw("Start the run", b2s(ok, "Everyone is ready", why), gold, !ok)(dst, rr, inR(rr, float64(mx), float64(my)))
		})
		if ok && (click(mainR) || confirm()) {
			g.hostStart()
		}
	} else {
		on := r.Connected()
		u.label(mainR, false, func(dst *ebiten.Image, rr [4]float64) {
			btnDraw(b2s(g.on.ready, "Not ready", "Ready"), "The host starts when everyone is ready", gold, !on)(dst, rr, inR(rr, float64(mx), float64(my)))
		})
		if on && (click(mainR) || confirm()) {
			g.on.ready = !g.on.ready
			r.SetMe(g.mySpec(), g.on.ready)
		}
	}
}

// nav1: a single left / right press (keyboard or pad).
func nav1() int {
	if keyJust(ebiten.KeyLeft, ebiten.KeyA) || padBtn(ebiten.StandardGamepadButtonLeftLeft) {
		return -1
	}
	if keyJust(ebiten.KeyRight, ebiten.KeyD) || padBtn(ebiten.StandardGamepadButtonLeftRight) {
		return 1
	}
	return 0
}

func (g *Game) hostStart() {
	r := g.on.room
	pr := g.prof
	rs := pr.S.RunSetup()
	bi, _ := rs["bi"].(string)
	li := -1
	if bi == "" || bi == "random" || !pr.BiomeUnlocked(bi) {
		bi = typesList()[rand.Intn(18)]
	} else {
		li = int(num(rs["li"]))
	}
	lock, _ := rs["lock"].(bool)
	st := r.StartRun(rand.Int63(), bi, li, g.tier(), g.mods(), g.stage(), lock, inputDelay)
	if st == nil {
		return
	}
	g.onlineNet().SetRoomOpen(r.Code, false)
	g.startOnline(st)
}

// startOnline builds the shared world from the start message and begins the lockstep session.
func (g *Game) startOnline(st *net.StartMsg) {
	pr := g.prof
	r := g.on.room
	cfg := sim.Config{LI: st.LI, Tier: st.Tier, Mods: st.Mods, Stage: st.Stage, Lock: st.Lock, Up: st.Players[0].Up, Training: st.Players[0].Training}
	me := st.Me
	if me >= 0 && me < len(st.Players) {
		id := st.Players[me].ID
		cfg.OnBoss = func(b, et string, l int, fin bool, visited []string) []string {
			m := pr.BossKill(b, et, l, fin, cfg.Stage, id, visited)
			g.save()
			return m
		}
	}
	w, err := sim.NewCoop(g.db, st.Players, st.Biome, st.Seed, cfg)
	if err != nil {
		g.toast(err.Error())
		return
	}
	// every peer must simulate the same view size (it sets spawn distances)
	w.ViewW, w.ViewH = 1280, 720
	g.W = w
	g.floor = NewFloor(st.Biome, 0, w.Seed)
	g.floorN = w.BiomeN
	g.pas = map[int]*playerAnim{}
	g.coop = true
	g.devs = nil
	g.seats = nil
	g.scr = scPlay
	g.bot = sim.NewBot(w, w.Seed+1)
	g.finished = false
	g.daily = nil
	g.on.ls = lockstep.New(w, len(st.Players), me, st.Delay, r)
	g.on.started = st
	g.on.cmds = nil
	g.on.stallT = 0
	g.on.note = ""
	if r.Host {
		ls := g.on.ls
		r.OnGone = func(idx int) { ls.DropPlayer(idx) }
	}
}

// onlineCmd queues a menu action for the lockstep stream (instead of applying it directly).
func (g *Game) onlineCmd(k lockstep.CmdKind, arg int) {
	g.on.cmds = append(g.on.cmds, lockstep.Cmd{Kind: k, Arg: int8(arg)})
}

// myTurn: the pending choice is ours to make.
func (g *Game) myTurn() bool {
	w := g.W
	if !g.online() {
		return true
	}
	me := g.on.room.Me
	switch w.State {
	case sim.StateRoute: // everyone votes until the party sets off
		return w.RouteGo < 0 && me < len(w.RouteVotes) && w.RouteVotes[me] < 0
	case sim.StateLevelUp, sim.StateShrine: // everyone picks their own cards at the same time
		return w.PendingKind(me) != ""
	}
	return true
}

// onlineStep advances the lockstep session with this frame's local input. It runs every frame of
// an online run (also while a choice screen is up), returns true when the run is over.
func (g *Game) onlineStep(in sim.Input) {
	r, ls := g.on.room, g.on.ls
	r.Update()
	f := lockstep.Quantise(in)
	f.Cmds = g.on.cmds
	g.on.cmds = nil
	n := ls.Update(f)
	if g.nodraw && g.tick%600 == 0 { // testing: progress in the console
		gone := ""
		for i, c := range g.W.PL {
			if c.Gone {
				gone += fmt.Sprint(" gone:", i)
			}
		}
		println("net: tick", ls.Tick, "t", int(g.W.T), "lv", g.W.XPS.Lv, "kills", g.W.Kills, "stalled", ls.Stalled, "waiting", fmt.Sprint(ls.WaitingOn()), "desync", ls.Desync, "state", int(g.W.State), gone)
	}
	if n == 0 && len(ls.WaitingOn()) > 0 {
		g.on.stallT += 1.0 / 60
	} else {
		g.on.stallT = 0
	}
	w := g.W
	for _, nt := range w.Notes {
		g.toast(nt)
	}
	w.Notes = nil
	if ls.Desync != 0 && g.on.note == "" {
		g.on.note = "The games went out of sync — the run was ended."
		g.toast("Out of sync with the other players")
		w.State = sim.StateDead
	}
	if !r.Host && r.Err != "" && g.on.note == "" {
		g.on.note = "The host disconnected · your rewards were banked"
		w.State = sim.StateDead
	}
	if r.Host && len(ls.WaitingOn()) > 0 && g.on.stallT > 25 { // a peer vanished without the connection closing
		for _, p := range ls.WaitingOn() {
			ls.DropPlayer(p)
		}
	}
}

// waitingNames lists who we're waiting on (for the HUD).
func (g *Game) waitingNames() string {
	if !g.online() || g.on.stallT < .5 {
		return ""
	}
	var l []string
	for _, p := range g.on.ls.WaitingOn() {
		if p < len(g.W.PL) {
			l = append(l, g.W.PL[p].Name)
		}
	}
	if len(l) == 0 {
		return ""
	}
	return "Waiting for " + strings.Join(l, ", ") + "…"
}

// drawOnlineOverlay: stall notice and, on choice screens that are someone else's, a waiting panel.
func (g *Game) drawOnlineOverlay(dst *ebiten.Image) {
	if !g.online() {
		return
	}
	sw, sh := float64(g.sw), float64(g.sh)
	if s := g.waitingNames(); s != "" {
		roundRect(dst, sw/2-200, 80, 400, 34, 10, c(5, 8, 15, 200))
		drawText(dst, s, sw/2, 88, 14, true, c(255, 208, 74, 255), text.AlignCenter, false)
	}
	if s := g.spectating(); s >= 0 && !g.W.Waiting() {
		roundRect(dst, sw/2-230, sh-70, 460, 44, 10, c(5, 8, 15, 210))
		drawText(dst, "You fainted · watching "+g.W.PL[s].Name, sw/2, sh-64, 15, true, gold, text.AlignCenter, false)
		drawText(dst, "Shift / RB next · Ctrl / LB previous · a Pokémon Center can revive you", sw/2, sh-44, 11, false, dim, text.AlignCenter, false)
	}
	if g.W.Waiting() && !g.myTurn() {
		if g.W.State == sim.StateRoute { // the map stays up: show who we're waiting on
			if g.W.RouteGo < 0 {
				var l []string
				for i, v := range g.W.RouteVotes {
					if v < 0 && i < len(g.W.PL) && !g.W.PL[i].Gone {
						l = append(l, g.W.PL[i].Name)
					}
				}
				if len(l) > 0 {
					roundRect(dst, sw/2-220, sh-64, 440, 34, 10, c(5, 8, 15, 210))
					drawText(dst, "Waiting for "+strings.Join(l, ", ")+" to vote…", sw/2, sh-56, 14, true, gold, text.AlignCenter, false)
				}
			}
			return
		}
		rect(dst, 0, 0, sw, sh, c(5, 8, 15, 170))
		who := strings.Join(g.W.Choosing(), ", ")
		drawText(dst, "Waiting for "+who+"…", sw/2, sh/2-20, 26, true, gold, text.AlignCenter, true)
		drawText(dst, "The run continues when everyone has picked", sw/2, sh/2+20, 13, false, dim, text.AlignCenter, true)
	}
}

// endOnlineRun returns everyone to the room after the results screen.
func (g *Game) endOnlineRun() {
	r := g.on.room
	g.on.ls = nil
	g.on.cmds = nil
	if r == nil {
		g.scr = scMenu
		return
	}
	if r.Err != "" {
		g.leaveRoom()
		g.on.mode = "menu"
		g.scr = scOnline
		return
	}
	if r.Host {
		r.EndRun()
		g.onlineNet().SetRoomOpen(r.Code, true)
	}
	g.on.ready = r.Host
	g.on.mode = "lobby"
	g.scr = scOnline
	g.ui.focus = ""
}

// updateOnlinePlay is updatePlay for lockstep runs: the session advances every frame, choice
// screens only take input from the player whose turn it is.
func (g *Game) updateOnlinePlay() {
	w := g.W
	tin, tactive := g.touchInput()
	if keyJust(ebiten.KeyEscape, ebiten.KeyP) || padBtn(ebiten.StandardGamepadButtonCenterRight) || (g.touch.pauseTapped && !w.Waiting()) {
		g.scr = scPause
		g.onlineStep(sim.Input{})
		return
	}
	if g.spectating() >= 0 { // fainted: cycle through the teammates still up
		if keyJust(ebiten.KeyShiftLeft, ebiten.KeyShiftRight, ebiten.KeyTab) || padBtn(ebiten.StandardGamepadButtonFrontTopRight) {
			g.specIdx++
		}
		if keyJust(ebiten.KeyControlLeft) || padBtn(ebiten.StandardGamepadButtonFrontTopLeft) {
			g.specIdx--
		}
	}
	var in sim.Input
	if w.Waiting() {
		g.lvDelay += 1.0 / 60
		if g.myTurn() {
			if g.auto && g.lvDelay > .6 { // demo / testing: pick like the bot
				switch g.choiceKind() {
				case "lv":
					g.onlineCmd(lockstep.CmdChoose, w.BotPick())
				case "shrine":
					g.onlineCmd(lockstep.CmdBoon, 0)
				case "route":
					g.onlineCmd(lockstep.CmdRoute, 0)
				}
				g.lvDelay = 0
			} else {
				g.choiceKeys()
			}
		}
	} else {
		g.lvDelay = 0
		g.banMode = false
		in = g.playInput()
		if tactive {
			in.MoveX, in.MoveY = tin.MoveX, tin.MoveY
			in.Dash = in.Dash || tin.Dash
			in.Act2 = in.Act2 || tin.Act2
		}
	}
	g.onlineStep(in)
	if w.State == sim.StateDead {
		g.finishRun()
		g.scr = scOver
		g.overT = 0
	}
}

// SignalMode switches online rooms to a test signalling path (?signal=local or http).
func (g *Game) SignalMode(m string) { g.on.sigMode = m }

// AutoHost / AutoJoin open the online screen and host (with a fixed code) or join right away;
// with auto set the room readies up / starts by itself and level-ups are picked like the bot (testing).
func (g *Game) AutoHost(code string, auto bool) {
	net.FixedCode = code
	g.on.autoGo, g.auto = auto, auto
	g.on.autoKind, g.on.autoCode = "host", code
	g.on.mode = "menu"
	g.scr = scOnline
}

func (g *Game) AutoJoin(code string, auto bool) {
	g.on.autoGo, g.auto = auto, auto
	g.on.autoKind, g.on.autoCode = "join", code
	g.on.mode = "menu"
	g.scr = scOnline
}
