//go:build js

package net

import (
	"math/rand"
	"sync"
	"syscall/js"
	"time"
)

// Browser networking: WebRTC data channels between players, signalled (offer / answer exchange)
// through either the web game's Firestore lobby documents (lobbies/{code}/join/{uid}) or, for testing
// two tabs in one browser, a BroadcastChannel.

var rtcConfig = map[string]any{"iceServers": []any{map[string]any{"urls": []any{"stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"}}}}

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// FixedCode, when set, is used as the room code instead of a random one (testing).
var FixedCode string

// SignalBase is unused in the browser (the HTTP test relay is the page's own origin).
var SignalBase = ""

func randCode() string {
	if FixedCode != "" {
		return FixedCode
	}
	b := make([]byte, 5)
	for i := range b {
		b[i] = codeAlphabet[rand.Intn(len(codeAlphabet))]
	}
	return string(b)
}

// ---- data channel connection ----

type jsConn struct {
	pc, ch js.Value
	mu     sync.Mutex
	q      []Msg
	open   bool
	closed bool
}

func newJSConn(pc js.Value) *jsConn {
	c := &jsConn{pc: pc}
	pc.Set("onconnectionstatechange", js.FuncOf(func(js.Value, []js.Value) any {
		st := pc.Get("connectionState").String()
		if st == "failed" || st == "closed" || st == "disconnected" {
			c.mu.Lock()
			c.closed = true
			c.mu.Unlock()
		}
		return nil
	}))
	return c
}

func (c *jsConn) attach(ch js.Value) {
	c.ch = ch
	ch.Set("binaryType", "arraybuffer")
	ch.Set("onopen", js.FuncOf(func(js.Value, []js.Value) any { c.mu.Lock(); c.open = true; c.mu.Unlock(); return nil }))
	ch.Set("onclose", js.FuncOf(func(js.Value, []js.Value) any { c.mu.Lock(); c.closed = true; c.mu.Unlock(); return nil }))
	ch.Set("onmessage", js.FuncOf(func(this js.Value, a []js.Value) any {
		d := a[0].Get("data")
		var m Msg
		if d.Type() == js.TypeString {
			m.Text = d.String()
		} else {
			u8 := js.Global().Get("Uint8Array").New(d)
			m.Bin = make([]byte, u8.Length())
			js.CopyBytesToGo(m.Bin, u8)
		}
		c.mu.Lock()
		c.q = append(c.q, m)
		c.mu.Unlock()
		return nil
	}))
	if ch.Get("readyState").String() == "open" {
		c.open = true
	}
}

func (c *jsConn) Send(b []byte) {
	if !c.Open() {
		return
	}
	u8 := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u8, b)
	c.ch.Call("send", u8)
}

func (c *jsConn) SendText(s string) {
	if c.Open() {
		c.ch.Call("send", s)
	}
}

func (c *jsConn) Recv() []Msg {
	c.mu.Lock()
	o := c.q
	c.q = nil
	c.mu.Unlock()
	return o
}

func (c *jsConn) Open() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open && !c.closed && !c.ch.IsUndefined() && c.ch.Get("readyState").String() == "open"
}

func (c *jsConn) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *jsConn) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	if !c.ch.IsUndefined() {
		c.ch.Call("close")
	}
	c.pc.Call("close")
}

// ---- promises / ICE ----

func then(p js.Value, ok func(js.Value), fail func(js.Value)) {
	p.Call("then", js.FuncOf(func(this js.Value, a []js.Value) any {
		if ok != nil {
			var v js.Value
			if len(a) > 0 {
				v = a[0]
			}
			ok(v)
		}
		return nil
	}), js.FuncOf(func(this js.Value, a []js.Value) any {
		if fail != nil {
			var v js.Value
			if len(a) > 0 {
				v = a[0]
			}
			fail(v)
		}
		return nil
	}))
}

func errText(v js.Value) string {
	if v.IsUndefined() || v.IsNull() {
		return "error"
	}
	if m := v.Get("message"); m.Type() == js.TypeString {
		return m.String()
	}
	if c := v.Get("code"); c.Type() == js.TypeString {
		return c.String()
	}
	return v.String()
}

// iceDone calls f once ICE gathering is complete (or after a timeout, with what we have).
func iceDone(pc js.Value, f func()) {
	if pc.Get("iceGatheringState").String() == "complete" {
		f()
		return
	}
	done := false
	fire := func() {
		if !done {
			done = true
			f()
		}
	}
	timer := js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any { fire(); return nil }), 3500)
	pc.Set("onicegatheringstatechange", js.FuncOf(func(js.Value, []js.Value) any {
		if pc.Get("iceGatheringState").String() == "complete" {
			js.Global().Call("clearTimeout", timer)
			fire()
		}
		return nil
	}))
}

// signal is how offers and answers travel between a guest and a host before WebRTC connects.
type signal interface {
	// host: allocate a code, then receive offers; reply sends the answer back to that guest
	listen(name string, onCode func(code string), onOffer func(id, name, sdp string, reply func(answer string)), onErr func(string)) func()
	// guest: send an offer to a room, get the answer
	offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func()
	available() (bool, string)
	login(cb func(uid, name, err string))
	setOpen(code string, open bool)
}

// rtcNet implements Net over WebRTC with a signal.
type rtcNet struct{ sig signal }

// NewNet returns the browser Net: Firestore signalling by default; for testing, "local"
// (BroadcastChannel between tabs of one browser) or "http" (the page's origin relays offers).
func NewNet(mode string) Net {
	switch mode {
	case "local":
		return &rtcNet{sig: &bcSignal{}}
	case "http":
		return &rtcNet{sig: &httpSignal{}}
	}
	return &rtcNet{sig: &fbSignal{}}
}

func (n *rtcNet) Available() (bool, string) { return n.sig.available() }

func (n *rtcNet) Login(cb func(uid, name, err string)) { n.sig.login(cb) }

func (n *rtcNet) SetRoomOpen(code string, open bool) { n.sig.setOpen(code, open) }

func (n *rtcNet) Host(name string, onCode func(string), onPeer func(Peer), onErr func(string)) func() {
	var conns []*jsConn
	stop := n.sig.listen(name, onCode, func(id, gname, sdp string, reply func(string)) {
		pc := js.Global().Get("RTCPeerConnection").New(js.ValueOf(rtcConfig))
		c := newJSConn(pc)
		conns = append(conns, c)
		pc.Set("ondatachannel", js.FuncOf(func(this js.Value, a []js.Value) any {
			c.attach(a[0].Get("channel"))
			onPeer(Peer{UID: id, Name: gname, Conn: c})
			return nil
		}))
		then(pc.Call("setRemoteDescription", js.ValueOf(map[string]any{"type": "offer", "sdp": sdp})), func(js.Value) {
			then(pc.Call("createAnswer"), func(ans js.Value) {
				then(pc.Call("setLocalDescription", ans), func(js.Value) {
					iceDone(pc, func() { reply(pc.Get("localDescription").Get("sdp").String()) })
				}, nil)
			}, nil)
		}, func(e js.Value) { onErr("Could not connect a player (" + errText(e) + ")") })
	}, onErr)
	return func() {
		stop()
		for _, c := range conns {
			c.Close()
		}
	}
}

func (n *rtcNet) Join(code, name string, onConn func(Conn), onErr func(string)) func() {
	pc := js.Global().Get("RTCPeerConnection").New(js.ValueOf(rtcConfig))
	c := newJSConn(pc)
	ch := pc.Call("createDataChannel", "c", js.ValueOf(map[string]any{"ordered": true}))
	c.attach(ch)
	ch.Set("onopen", js.FuncOf(func(js.Value, []js.Value) any {
		c.mu.Lock()
		c.open = true
		c.mu.Unlock()
		onConn(c)
		return nil
	}))
	var stop func()
	then(pc.Call("createOffer"), func(off js.Value) {
		then(pc.Call("setLocalDescription", off), func(js.Value) {
			iceDone(pc, func() {
				sdp := pc.Get("localDescription").Get("sdp").String()
				stop = n.sig.offer(code, name, sdp, func(answer string) {
					if !pc.Get("remoteDescription").IsNull() {
						return
					}
					then(pc.Call("setRemoteDescription", js.ValueOf(map[string]any{"type": "answer", "sdp": answer})), nil,
						func(e js.Value) { onErr("Connection error: " + errText(e)) })
				}, onErr)
			})
		}, nil)
	}, func(e js.Value) { onErr("WebRTC is not available: " + errText(e)) })
	return func() {
		if stop != nil {
			stop()
		}
		c.Close()
	}
}

// ---- Firestore signalling (the web game's lobby documents) ----

type fbSignal struct{}

func fb() js.Value { return js.Global().Get("firebase") }

func (fbSignal) available() (bool, string) {
	f := fb()
	if f.IsUndefined() || f.Get("apps").Length() == 0 {
		return false, "Online features could not load."
	}
	u := f.Call("auth").Get("currentUser")
	if u.IsNull() || u.IsUndefined() {
		return false, "Sign in with Google first (main menu)."
	}
	return true, ""
}

func (fbSignal) uid() string {
	u := fb().Call("auth").Get("currentUser")
	if u.IsNull() || u.IsUndefined() {
		return ""
	}
	return u.Get("uid").String()
}

func (s fbSignal) login(cb func(uid, name, err string)) {
	if ok, why := s.available(); !ok {
		cb("", "", why)
		return
	}
	u := fb().Call("auth").Get("currentUser")
	name := "Player"
	if dn := u.Get("displayName"); dn.Type() == js.TypeString && dn.String() != "" {
		name = dn.String()
		for i, r := range name {
			if r == ' ' {
				name = name[:i]
				break
			}
		}
	}
	cb(u.Get("uid").String(), name, "")
}

func (fbSignal) setOpen(code string, open bool) {
	if f := fb(); f.IsUndefined() || code == "" {
		return
	}
	lobbyDoc(code).Call("set", js.ValueOf(map[string]any{"open": open}), js.ValueOf(map[string]any{"merge": true})).Call("catch", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
}

func lobbyDoc(code string) js.Value {
	return fb().Call("firestore").Call("collection", "lobbies").Call("doc", code)
}

func (s *fbSignal) listen(name string, onCode func(string), onOffer func(id, name, sdp string, reply func(string)), onErr func(string)) func() {
	uid := s.uid()
	var unsub js.Value
	var code string
	var try func(n int)
	try = func(n int) {
		code = randCode()
		then(lobbyDoc(code).Call("get"), func(snap js.Value) {
			if snap.Get("exists").Bool() && n < 8 {
				try(n + 1)
				return
			}
			doc := map[string]any{"host": uid, "hostName": name, "open": true, "ts": time.Now().UnixMilli(), "eng": "go", "proto": Protocol}
			then(lobbyDoc(code).Call("set", js.ValueOf(doc)), func(js.Value) {
				onCode(code)
				unsub = lobbyDoc(code).Call("collection", "join").Call("onSnapshot", js.FuncOf(func(this js.Value, a []js.Value) any {
					a[0].Call("docChanges").Call("forEach", js.FuncOf(func(this js.Value, b []js.Value) any {
						ch := b[0]
						if ch.Get("type").String() != "added" {
							return nil
						}
						d := ch.Get("doc").Call("data")
						off, ans := d.Get("offer"), d.Get("answer")
						if off.Type() != js.TypeString || ans.Type() == js.TypeString {
							return nil
						}
						id := ch.Get("doc").Get("id").String()
						gname := "Trainer"
						if nm := d.Get("name"); nm.Type() == js.TypeString {
							gname = nm.String()
						}
						onOffer(id, gname, off.String(), func(answer string) {
							lobbyDoc(code).Call("collection", "join").Call("doc", id).Call("update", js.ValueOf(map[string]any{"answer": answer})).Call("catch", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
						})
						return nil
					}))
					return nil
				}), js.FuncOf(func(this js.Value, a []js.Value) any {
					onErr("Lobby error: " + errText(a[0]))
					return nil
				}))
			}, func(e js.Value) { onErr("Could not open a room: " + errText(e)) })
		}, func(e js.Value) { onErr("Could not open a room: " + errText(e)) })
	}
	try(0)
	return func() {
		if !unsub.IsUndefined() {
			unsub.Invoke()
		}
		if code != "" {
			lobbyDoc(code).Call("delete").Call("catch", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
		}
	}
}

func (s *fbSignal) offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func() {
	uid := s.uid()
	var unsub js.Value
	ref := lobbyDoc(code).Call("collection", "join").Call("doc", uid)
	then(lobbyDoc(code).Call("get"), func(snap js.Value) {
		if !snap.Get("exists").Bool() {
			onErr("No room with that code.")
			return
		}
		d := snap.Call("data")
		if eng := d.Get("eng"); eng.Type() != js.TypeString || eng.String() != "go" {
			onErr("That room was made by the web version of the game; this version can only join rooms hosted by the same version.")
			return
		}
		if op := d.Get("open"); op.Type() == js.TypeBoolean && !op.Bool() {
			onErr("That room is in a run. Try again when it finishes.")
			return
		}
		then(ref.Call("set", js.ValueOf(map[string]any{"name": name, "offer": sdp, "ts": time.Now().UnixMilli()})), func(js.Value) {
			unsub = ref.Call("onSnapshot", js.FuncOf(func(this js.Value, a []js.Value) any {
				d := a[0].Call("data")
				if d.IsUndefined() || d.IsNull() {
					return nil
				}
				if ans := d.Get("answer"); ans.Type() == js.TypeString {
					onAnswer(ans.String())
				}
				return nil
			}), js.FuncOf(func(this js.Value, a []js.Value) any { onErr("Lobby error: " + errText(a[0])); return nil }))
		}, func(e js.Value) { onErr("Could not join: " + errText(e)) })
	}, func(e js.Value) { onErr("Could not look up the room: " + errText(e)) })
	return func() {
		if !unsub.IsUndefined() {
			unsub.Invoke()
		}
		ref.Call("delete").Call("catch", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
	}
}

// ---- BroadcastChannel signalling: tabs of one browser (testing) ----

type bcSignal struct{}

func (bcSignal) available() (bool, string) { return true, "" }
func (bcSignal) login(cb func(string, string, string)) {
	cb("local-"+randCode()+randCode(), "Player", "")
}
func (bcSignal) setOpen(string, bool) {}

func bchan() js.Value { return js.Global().Get("BroadcastChannel").New("ps-signal") }

func (bcSignal) listen(name string, onCode func(string), onOffer func(id, name, sdp string, reply func(string)), onErr func(string)) func() {
	code := randCode()
	ch := bchan()
	ch.Set("onmessage", js.FuncOf(func(this js.Value, a []js.Value) any {
		d := a[0].Get("data")
		if d.Get("t").String() != "offer" || d.Get("code").String() != code {
			return nil
		}
		id := d.Get("id").String()
		onOffer(id, d.Get("name").String(), d.Get("sdp").String(), func(answer string) {
			ch.Call("postMessage", js.ValueOf(map[string]any{"t": "answer", "code": code, "id": id, "sdp": answer}))
		})
		return nil
	}))
	js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any { onCode(code); return nil }), 0)
	return func() { ch.Call("close") }
}

func (bcSignal) offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func() {
	id := randCode() + randCode()
	ch := bchan()
	ch.Set("onmessage", js.FuncOf(func(this js.Value, a []js.Value) any {
		d := a[0].Get("data")
		if d.Get("t").String() == "answer" && d.Get("id").String() == id {
			onAnswer(d.Get("sdp").String())
		}
		return nil
	}))
	ch.Call("postMessage", js.ValueOf(map[string]any{"t": "offer", "code": code, "id": id, "name": name, "sdp": sdp}))
	return func() { ch.Call("close") }
}

// ---- HTTP signalling against the page's own origin (testing with two browsers) ----

type httpSignal struct{}

func (httpSignal) available() (bool, string) { return true, "" }
func (httpSignal) login(cb func(string, string, string)) {
	cb("local-"+randCode()+randCode(), "Player", "")
}
func (httpSignal) setOpen(string, bool) {}

func fetchText(method, url, body string, ok func(status int, text string), fail func(string)) {
	opt := map[string]any{"method": method}
	if body != "" {
		opt["body"] = body
	}
	then(js.Global().Call("fetch", url, js.ValueOf(opt)), func(resp js.Value) {
		st := resp.Get("status").Int()
		then(resp.Call("text"), func(t js.Value) { ok(st, t.String()) }, func(e js.Value) { fail(errText(e)) })
	}, func(e js.Value) { fail(errText(e)) })
}

func (httpSignal) listen(name string, onCode func(string), onOffer func(id, name, sdp string, reply func(string)), onErr func(string)) func() {
	code := randCode()
	seen := map[string]bool{}
	stopped := false
	var poll func()
	poll = func() {
		if stopped {
			return
		}
		fetchText("GET", "/sig/offers?code="+code, "", func(st int, t string) {
			// lines: id \t name \t sdp (sdp newlines escaped as \x01)
			for _, ln := range splitLines(t) {
				parts := splitN(ln, "\t", 3)
				if len(parts) < 3 || seen[parts[0]] {
					continue
				}
				seen[parts[0]] = true
				id := parts[0]
				onOffer(id, parts[1], unesc(parts[2]), func(answer string) {
					fetchText("POST", "/sig/answer?code="+code+"&id="+id, answer, func(int, string) {}, onErr)
				})
			}
			js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any { poll(); return nil }), 500)
		}, onErr)
	}
	fetchText("POST", "/sig/host?code="+code, name, func(int, string) { onCode(code); poll() }, onErr)
	return func() { stopped = true }
}

func (httpSignal) offer(code, name, sdp string, onAnswer func(string), onErr func(string)) func() {
	id := randCode() + randCode()
	stopped := false
	var poll func()
	poll = func() {
		if stopped {
			return
		}
		fetchText("GET", "/sig/answer?code="+code+"&id="+id, "", func(st int, t string) {
			if st == 200 && t != "" {
				stopped = true
				onAnswer(t)
				return
			}
			js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any { poll(); return nil }), 500)
		}, onErr)
	}
	fetchText("POST", "/sig/offer?code="+code+"&id="+id+"&name="+name, sdp, func(st int, t string) {
		if st != 200 {
			onErr("No room with that code.")
			return
		}
		poll()
	}, onErr)
	return func() { stopped = true }
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func splitN(s, sep string, n int) []string {
	var out []string
	for n > 1 {
		i := indexOf(s, sep)
		if i < 0 {
			break
		}
		out = append(out, s[:i])
		s = s[i+len(sep):]
		n--
	}
	return append(out, s)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func unesc(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] == 1 {
			b[i] = '\n'
		}
	}
	return string(b)
}
