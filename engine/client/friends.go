package client

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"github.com/TxHandles/pokemon-survivor/engine/net"
)

// Friends (web friends.js): a 6-character friend code per player, requests by code, the friends
// list with online status, and co-op invites from a room's lobby. Everything lives in the same
// Firestore documents the web game uses, so browser and desktop players can be friends.

const scFriends screen = 190

type invite struct {
	from, name, code string
	ts               float64
}

type friendsState struct {
	mu       sync.Mutex
	events   []func()
	started  bool
	starting bool
	uid      string
	code     string
	frs      map[string]net.Doc // my friends subcollection: status in / out / friends
	prof     map[string]net.Doc // users/{id}
	listAt   time.Time
	profAt   time.Time
	seenAt   time.Time
	invAt    time.Time
	input    string
	msg      string
	busy     bool
	rmArm    string
	inv      *invite
	invSeen  map[string]bool
	sent     map[string]string // lobby invites: friend id → state
	tAdd     string            // testing: send a request to this code once signed in
	tAccept  bool              // testing: accept incoming requests
}

// TestFriends (testing): send a friend request to code once signed in, accept incoming requests,
// and use name as the trainer name.
func (g *Game) TestFriends(code string, accept bool, name string) {
	g.fr.tAdd, g.fr.tAccept = code, accept
	if name != "" {
		g.setOptStr("name", name)
	}
}

func nowMS() float64 { return float64(time.Now().UnixMilli()) }

func (g *Game) frPost(f func()) {
	g.fr.mu.Lock()
	g.fr.events = append(g.fr.events, f)
	g.fr.mu.Unlock()
}

func (g *Game) frStore() net.Store {
	if g.on.sigMode != "" && g.on.sigMode != "local" { // the http test relay has no Firestore
		return nil
	}
	n := g.onlineNet()
	if ok, _ := n.Available(); !ok {
		return nil
	}
	return net.StoreOf(n)
}

func (g *Game) frName() string {
	n := strings.TrimSpace(g.optStr("name"))
	if n == "" {
		n = g.onlineName()
	}
	if n == "" || n == "Player" {
		n = "Trainer"
	}
	if r := []rune(n); len(r) > 16 {
		n = string(r[:16])
	}
	return n
}

func frOnline(p net.Doc) bool { return p != nil && p.Num("seen") > 0 && nowMS()-p.Num("seen") < 150000 }

func frAgo(t float64) string {
	m := int((nowMS() - t) / 60000)
	switch {
	case m < 60:
		return fmt.Sprintf("%dm ago", max(0, m))
	case m < 1440:
		return fmt.Sprintf("%dh ago", m/60)
	}
	return fmt.Sprintf("%dd ago", m/1440)
}

// friendsTick runs every frame: sign-in, the friend code, presence, the friends list and invites.
func (g *Game) friendsTick() {
	f := &g.fr
	f.mu.Lock()
	ev := f.events
	f.events = nil
	f.mu.Unlock()
	for _, e := range ev {
		e()
	}
	st := g.frStore()
	if st == nil {
		return
	}
	g.onlineLogin()
	g.onlinePoll()
	if g.on.uid == "" {
		return
	}
	if f.uid != g.on.uid { // signed in as someone else: start over
		ta, tc := f.tAdd, f.tAccept
		*f = friendsState{uid: g.on.uid, tAdd: ta, tAccept: tc}
	}
	now := time.Now()
	if !f.started {
		if !f.starting {
			f.starting = true
			g.frStart(st)
		}
		return
	}
	if f.tAdd != "" {
		f.input, f.tAdd = f.tAdd, ""
		g.frAdd()
	}
	if f.tAccept {
		for _, id := range g.frIDs("in") {
			g.frAccept(id)
			f.frs[id]["status"] = "friends"
		}
	}
	if now.Sub(f.seenAt) > 60*time.Second && ebiten.IsFocused() {
		f.seenAt = now
		st.Set("users/"+f.uid, net.Doc{"seen": nowMS(), "name": g.frName()}, true, nil)
	}
	listEvery := 20 * time.Second
	if g.scr == scFriends || (g.scr == scOnline && g.on.room != nil && g.on.room.Host) {
		listEvery = 5 * time.Second
	}
	if now.Sub(f.listAt) > listEvery {
		f.listAt = now
		g.frList(st)
	}
	inRun := g.scr == scPlay || g.scr == scPause
	if !inRun && now.Sub(f.invAt) > 8*time.Second {
		f.invAt = now
		uid := f.uid
		st.List("users/"+uid+"/invites", func(docs map[string]net.Doc, err string) {
			if err != "" {
				return
			}
			g.frPost(func() {
				if f.invSeen == nil {
					f.invSeen = map[string]bool{}
				}
				for from, d := range docs {
					k := from + fmt.Sprint(d.Num("ts"))
					if nowMS()-d.Num("ts") > 180000 || f.invSeen[k] {
						continue
					}
					f.invSeen[k] = true
					f.inv = &invite{from: from, name: d.Str("name"), code: d.Str("code"), ts: d.Num("ts")}
				}
			})
		})
	}
}

// frStart: read / create my user document and friend code (web frStart).
func (g *Game) frStart(st net.Store) {
	f := &g.fr
	uid, name := f.uid, g.frName()
	st.Get("users/"+uid, func(d net.Doc, _ bool, err string) {
		code := d.Str("code")
		finish := func(code string) {
			st.Set("users/"+uid, net.Doc{"name": name, "code": code, "seen": nowMS()}, true, func(string) {})
			g.frPost(func() {
				f.code, f.started, f.starting = code, true, false
				f.seenAt = time.Now()
				g.frList(st)
			})
		}
		if code != "" || err != "" {
			finish(code)
			return
		}
		var try func(n int)
		try = func(n int) {
			if n >= 6 {
				finish("")
				return
			}
			c := make([]byte, 6)
			for i := range c {
				c[i] = net.FriendAlphabet[rand.Intn(len(net.FriendAlphabet))]
			}
			cs := string(c)
			st.Get("codes/"+cs, func(_ net.Doc, taken bool, err string) {
				if taken || err != "" {
					try(n + 1)
					return
				}
				st.Set("codes/"+cs, net.Doc{"uid": uid}, false, func(err string) {
					if err != "" {
						try(n + 1)
						return
					}
					finish(cs)
				})
			})
		}
		try(0)
	})
}

// frList refreshes my friends subcollection and (when shown) their profiles.
func (g *Game) frList(st net.Store) {
	f := &g.fr
	uid := f.uid
	wantProf := g.scr == scFriends || g.scr == scOnline
	st.List("users/"+uid+"/friends", func(docs map[string]net.Doc, err string) {
		if err != "" {
			return
		}
		g.frPost(func() { f.frs = docs })
		if !wantProf {
			return
		}
		for id := range docs {
			id := id
			st.Get("users/"+id, func(d net.Doc, ok bool, err string) {
				if ok {
					g.frPost(func() {
						if f.prof == nil {
							f.prof = map[string]net.Doc{}
						}
						f.prof[id] = d
					})
				}
			})
		}
	})
}

func (g *Game) frMsg(s string) { g.frPost(func() { g.fr.msg = s }) }

// frAdd sends a friend request by code (web frAdd).
func (g *Game) frAdd() {
	f := &g.fr
	st := g.frStore()
	c := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			r -= 32
		}
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, f.input)
	f.msg = ""
	switch {
	case st == nil || !f.started:
		f.msg = "Not signed in yet."
		return
	case len(c) != 6:
		f.msg = "Friend codes are 6 characters."
		return
	case c == f.code:
		f.msg = "That's your own code!"
		return
	case f.busy:
		return
	}
	f.busy = true
	uid, name := f.uid, g.frName()
	done := func(msg string, clear bool) {
		g.frPost(func() {
			f.busy, f.msg = false, msg
			if clear {
				f.input = ""
			}
			g.frList(st)
		})
	}
	st.Get("codes/"+c, func(d net.Doc, ok bool, err string) {
		if err != "" {
			done("Could not send the request ("+err+")", false)
			return
		}
		if !ok {
			done("No trainer has that code.", false)
			return
		}
		fid := d.Str("uid")
		cur := f.frs[fid]
		switch cur.Str("status") {
		case "friends":
			done("You're already friends.", false)
			return
		case "in":
			g.frAccept(fid)
			done("They had already asked, you're now friends!", true)
			return
		}
		st.Get("users/"+fid, func(fp net.Doc, _ bool, _ string) {
			fname := fp.Str("name")
			if fname == "" {
				fname = "Trainer"
			}
			now := nowMS()
			st.Set("users/"+fid+"/friends/"+uid, net.Doc{"status": "in", "name": name, "ts": now}, false, func(err string) {
				if err != "" {
					done("Could not send the request ("+err+")", false)
					return
				}
				st.Set("users/"+uid+"/friends/"+fid, net.Doc{"status": "out", "name": fname, "ts": now}, false, func(string) {
					done("Request sent to "+fname+"!", true)
				})
			})
		})
	})
}

func (g *Game) frAccept(fid string) {
	st := g.frStore()
	if st == nil {
		return
	}
	uid, name := g.fr.uid, g.frName()
	fname := g.fr.frs[fid].Str("name")
	if fname == "" {
		fname = "Trainer"
	}
	now := nowMS()
	st.Set("users/"+fid+"/friends/"+uid, net.Doc{"status": "friends", "name": name, "ts": now}, true, func(err string) {
		if err != "" {
			g.frMsg("Could not accept (" + err + ")")
			return
		}
		st.Set("users/"+uid+"/friends/"+fid, net.Doc{"status": "friends", "name": fname, "ts": now}, false, func(string) {
			g.frPost(func() { g.frList(st) })
		})
	})
}

func (g *Game) frRemove(fid string) {
	st := g.frStore()
	if st == nil {
		return
	}
	uid := g.fr.uid
	st.Delete("users/"+uid+"/friends/"+fid, func(string) {
		st.Delete("users/"+fid+"/friends/"+uid, func(string) { g.frPost(func() { g.frList(st) }) })
	})
}

// frInvite asks friend fid to join my room (web lobInvite).
func (g *Game) frInvite(fid string) {
	st := g.frStore()
	if st == nil || g.on.room == nil || g.on.room.Code == "" {
		return
	}
	if g.fr.sent == nil {
		g.fr.sent = map[string]string{}
	}
	g.fr.sent[fid] = "…"
	st.Set("users/"+fid+"/invites/"+g.fr.uid, net.Doc{"code": g.on.room.Code, "name": g.frName(), "ts": nowMS()}, false, func(err string) {
		g.frPost(func() {
			if err != "" {
				g.fr.sent[fid] = "Failed"
			} else {
				g.fr.sent[fid] = "Sent"
			}
		})
	})
}

// frIDs: friend ids with a status, friends online first then by name.
func (g *Game) frIDs(status string) []string {
	f := &g.fr
	var ids []string
	for id, d := range f.frs {
		if d.Str("status") == status {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		oi, oj := frOnline(f.prof[ids[i]]), frOnline(f.prof[ids[j]])
		if oi != oj {
			return oi
		}
		return g.frNameOf(ids[i]) < g.frNameOf(ids[j])
	})
	return ids
}

func (g *Game) frNameOf(id string) string {
	if p := g.fr.prof[id]; p.Str("name") != "" {
		return p.Str("name")
	}
	if n := g.fr.frs[id].Str("name"); n != "" {
		return n
	}
	return "Trainer"
}

// frRequests: incoming requests (the menu badge).
func (g *Game) frRequests() int { return len(g.frIDs("in")) }

// ---- screen ----

func (g *Game) menuFriends() {
	sw, sh := float64(g.sw), float64(g.sh)
	u := &g.ui
	f := &g.fr
	g.webHeaderParts("Friends", "", scMenu, nil)
	u.area = [4]float64{0, 60, sw, sh - 60}
	cw := 520.0
	x := sw/2 - cw/2
	y := 14.0
	st := g.frStore()
	if st == nil || g.on.uid == "" || !f.started {
		msg := "Signing in…"
		if st == nil {
			_, why := g.onlineNet().Available()
			msg = nonEmptyStr(why, "Online features are not available in this build.")
		} else if g.on.loginErr != "" {
			msg = g.on.loginErr
		}
		y += g.para(msg, y, 480, 14, false, alpha(webFg, .85)) + 10
		if g.cloud.Available() && !g.cloud.Online() {
			u.button("signin", [4]float64{sw/2 - 130, y, 260, 40}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
				webBtnDraw(dst, r, hot, false, false)
				drawText(dst, "Sign in with Google", r[0]+r[2]/2, r[1]+11, 15, false, webFg, text.AlignCenter, false)
			}, func() { g.cloud.SignIn() })
		}
		return
	}
	// my code
	u.label([4]float64{x + 80, y, cw - 160, 104}, true, func(dst *ebiten.Image, r [4]float64) {
		webBox(dst, r[0], r[1], r[2], r[3], 12, webBorder, webPanel)
		drawText(dst, "Your friend code", r[0]+r[2]/2, r[1]+10, 12, false, alpha(webFg, .75), text.AlignCenter, false)
		code := nonEmptyStr(f.code, "—")
		drawText(dst, spaced(code), r[0]+r[2]/2, r[1]+28, 26, true, webAcc, text.AlignCenter, false)
	})
	bw := smallBtnW("Copy code", gUpload, " ")
	u.button("frcopy", [4]float64{sw/2 - bw/2, y + 66, bw, 30}, true, false, func(dst *ebiten.Image, r [4]float64, hot bool) {
		smallBtn(dst, r, "Copy code", hot, false, gUpload, " ")
	}, func() {
		if clipWrite(f.code) {
			f.msg = "Code copied!"
		} else {
			f.msg = "Your code: " + f.code
		}
	})
	y += 116
	// add by code
	fw := 190.0
	ab := smallBtnW("+ Add friend")
	tx := sw/2 - (fw+8+ab)/2
	was := u.edit == "frcode"
	g.textField("frcode", [4]float64{tx, y + u.area[1] - u.scroll, fw, 34}, &f.input, "FRIEND CODE", false)
	if len(f.input) > 6 {
		f.input = f.input[:6]
	}
	f.input = strings.ToUpper(f.input)
	if was && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.frAdd()
	}
	u.button("fradd", [4]float64{tx + fw + 8, y + 2, ab, 30}, true, f.busy, func(dst *ebiten.Image, r [4]float64, hot bool) {
		smallBtn(dst, r, "+ Add friend", hot, f.busy)
	}, func() { g.frAdd() })
	y += 44
	if f.msg != "" {
		u.label([4]float64{0, y, sw, 18}, true, textDraw(f.msg, 13, true, webFg, text.AlignCenter))
	}
	y += 24
	// lists
	any := false
	for _, grp := range [][2]string{{"in", "Friend requests"}, {"friends", "Friends"}, {"out", "Sent requests"}} {
		ids := g.frIDs(grp[0])
		if len(ids) == 0 {
			continue
		}
		any = true
		y += g.h3(fmt.Sprintf("%s (%d)", grp[1], len(ids)), y)
		for _, id := range ids {
			id, status := id, grp[0]
			p := f.prof[id]
			on := status == "friends" && frOnline(p)
			sub := " pending…"
			switch {
			case status == "in":
				sub = " wants to be friends"
			case status == "friends" && on:
				sub = " Online"
			case status == "friends" && p.Num("seen") > 0:
				sub = " Last seen " + frAgo(p.Num("seen"))
			case status == "friends":
				sub = " Offline"
			}
			name := g.frNameOf(id)
			ry := y
			// buttons first (they take clicks before the row)
			type btn struct {
				key, label string
				act        func()
			}
			var bs []btn
			switch status {
			case "in":
				bs = []btn{{"ok" + id, "Accept", func() { g.frAccept(id) }}, {"no" + id, "Decline", func() { g.frRemove(id) }}}
			case "out":
				bs = []btn{{"cx" + id, "Cancel", func() { g.frRemove(id) }}}
			default:
				lbl := "Remove"
				if f.rmArm == id {
					lbl = "Sure?"
				}
				bs = []btn{{"rm" + id, lbl, func() {
					if f.rmArm == id {
						f.rmArm = ""
						g.frRemove(id)
						return
					}
					f.rmArm = id
				}}}
			}
			bx := x + cw - 10
			type placed struct {
				key, label string
				r          [4]float64
			}
			var pl []placed
			for i := len(bs) - 1; i >= 0; i-- {
				b := bs[i]
				w := smallBtnW(b.label)
				bx -= w
				r := [4]float64{bx, ry + 7, w, 28}
				// the row draws the buttons; the widgets only take the clicks (registered first)
				u.button(b.key, r, true, false, nil, b.act)
				pl = append(pl, placed{b.key, b.label, r})
				bx -= 6
			}
			u.label([4]float64{x, ry, cw, 42}, true, func(dst *ebiten.Image, r [4]float64) {
				webBox(dst, r[0], r[1], r[2], r[3], 10, webBorder, webPanel)
				dc := c(85, 85, 85, 255)
				if on {
					soft(dst, r[0]+16, r[1]+21, 9, 9, alpha(hexC("#4cdf6a"), .6))
					dc = hexC("#4cdf6a")
				}
				disc(dst, r[0]+16, r[1]+21, 5, dc)
				nw := richLine(dst, r[0]+28, r[1]+12, 14, true, webFg, name)
				drawText(dst, sub, r[0]+28+nw, r[1]+14, 12, false, alpha(webFg, .7), text.AlignStart, false)
				dy := r[1] - ry
				for _, p := range pl {
					br := p.r
					br[1] += dy
					smallBtn(dst, br, p.label, u.focus == p.key, false)
				}
			})
			y += 48
		}
		y += 6
	}
	if !any {
		y += g.para("No friends yet. Share your code, or enter a friend's code above.", y, 480, 13, false, alpha(webFg, .7))
	}
	y += 20
	u.scrollMax = maxf(0, y-u.area[3])
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func spaced(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// invitePopup: a co-op invite from a friend, over any menu screen (web invShow).
func (g *Game) invitePopup() {
	f := &g.fr
	iv := f.inv
	if iv == nil {
		return
	}
	if nowMS()-iv.ts > 60000+180000 {
		f.inv = nil
		return
	}
	u := &g.ui
	sw, sh := float64(g.sw), float64(g.sh)
	r := [4]float64{sw/2 - 230, sh - 96, 460, 76}
	u.topLabel(r, func(dst *ebiten.Image, r [4]float64) {
		webBox(dst, r[0], r[1], r[2], r[3], 12, webAcc, c(16, 20, 30, 245))
		name := nonEmptyStr(iv.name, "A friend")
		w := richLine(dst, r[0]+16, r[1]+14, 15, true, webAcc, name)
		drawText(dst, " invited you to co-op!", r[0]+16+w, r[1]+15, 14, false, webFg, text.AlignStart, false)
		drawText(dst, "Room "+iv.code, r[0]+16, r[1]+42, 12, false, alpha(webFg, .7), text.AlignStart, false)
	})
	done := func() {
		f.inv = nil
		if st := g.frStore(); st != nil {
			st.Delete("users/"+f.uid+"/invites/"+iv.from, nil)
		}
	}
	u.topButton("invno", [4]float64{r[0] + r[2] - 100, r[1] + 38, 86, 28}, func(dst *ebiten.Image, rr [4]float64, hot bool) {
		smallBtn(dst, rr, "Decline", hot, false)
	}, done)
	u.topButton("invyes", [4]float64{r[0] + r[2] - 170, r[1] + 38, 62, 28}, func(dst *ebiten.Image, rr [4]float64, hot bool) {
		smallBtn(dst, rr, "Join", hot, false)
	}, func() {
		done()
		g.scr = scOnline
		g.on.mode = "menu"
		g.joinRoom(iv.code)
		g.ui.focus = ""
	})
}

// lobbyInvites: the host's "Invite friends" list inside an open seat of the online lobby. The lobby
// handles its own keys, so the buttons use the lobby's click test instead of focusable widgets.
func (g *Game) lobbyInvites(x, y, w float64, click func([4]float64) bool) {
	u := &g.ui
	f := &g.fr
	if g.frStore() == nil || !f.started {
		return
	}
	r := g.on.room
	inRoom := map[string]bool{}
	for _, p := range r.Players {
		inRoom[p.UID] = true
	}
	var ids []string
	for _, id := range g.frIDs("friends") {
		if !inRoom[id] {
			ids = append(ids, id)
		}
	}
	u.label([4]float64{x, y, w, 22}, false, textDraw("Invite friends", 14, true, webFg, text.AlignCenter))
	y += 26
	if len(ids) == 0 {
		u.label([4]float64{x + 10, y, w - 20, 40}, false, func(dst *ebiten.Image, rr [4]float64) {
			for i, l := range wrap("Add friends in Friends on the main menu to invite them here.", font(11, false), rr[2]) {
				drawText(dst, l, rr[0]+rr[2]/2, rr[1]+float64(i)*14, 11, false, alpha(webFg, .65), text.AlignCenter, false)
			}
		})
		return
	}
	mx, my := ebiten.CursorPosition()
	for i, id := range ids {
		if i >= 5 {
			break
		}
		id := id
		on := frOnline(f.prof[id])
		lbl := "Invite"
		if s := f.sent[id]; s != "" {
			lbl = s
		}
		bw := smallBtnW(lbl)
		br := [4]float64{x + w - bw - 8, y + 5, bw, 26}
		if click(br) && lbl == "Invite" {
			g.frInvite(id)
		}
		name := g.frNameOf(id)
		u.label([4]float64{x + 6, y, w - 12, 36}, false, func(dst *ebiten.Image, rr [4]float64) {
			webBox(dst, rr[0], rr[1], rr[2], rr[3], 9, webBorder, webPanel)
			dc := c(85, 85, 85, 255)
			if on {
				dc = hexC("#4cdf6a")
			}
			disc(dst, rr[0]+12, rr[1]+18, 5, dc)
			drawText(dst, fitText(name, 13, true, rr[2]-bw-36), rr[0]+22, rr[1]+10, 13, true, webFg, text.AlignStart, false)
			smallBtn(dst, br, lbl, inR(br, float64(mx), float64(my)), lbl != "Invite")
		})
		y += 40
	}
}
