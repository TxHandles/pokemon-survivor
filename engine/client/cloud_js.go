//go:build js

package client

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"syscall/js"
	"time"

	"github.com/TxHandles/pokemon-survivor/engine/meta"
)

// Firebase (compat SDK) through syscall/js: the same project, auth and documents as the web game.
const fbVersion = "10.12.2"

var fbConfig = map[string]any{"apiKey": "AIzaSyArpClEYGkFOq552fSgMYEwRIQlcJs5NJU", "authDomain": "pokemon-survivors-e8213.firebaseapp.com",
	"projectId": "pokemon-survivors-e8213", "storageBucket": "pokemon-survivors-e8213.firebasestorage.app", "messagingSenderId": "718817782433", "appId": "1:718817782433:web:1803f4b323b3e78cc71ee0"}

type fbCloud struct {
	mu       sync.Mutex
	ready    bool
	failed   bool
	uid      string
	name     string
	status   string
	events   []func()
	fs       js.Value
	unsub    js.Value
	lbBusy   bool
	signedIn bool // auth state changed since the last Poll
	needPush bool
}

func newCloud() Cloud {
	c := &fbCloud{status: "Loading online features…"}
	c.loadScripts()
	return c
}

func (c *fbCloud) post(f func()) {
	c.mu.Lock()
	c.events = append(c.events, f)
	c.mu.Unlock()
}

func (c *fbCloud) loadScripts() {
	doc := js.Global().Get("document")
	mods := []string{"app", "auth", "firestore"}
	n := 0
	var next func()
	next = func() {
		if n >= len(mods) {
			c.init()
			return
		}
		s := doc.Call("createElement", "script")
		s.Set("src", "https://www.gstatic.com/firebasejs/"+fbVersion+"/firebase-"+mods[n]+"-compat.js")
		n++
		s.Set("onload", js.FuncOf(func(js.Value, []js.Value) any { next(); return nil }))
		s.Set("onerror", js.FuncOf(func(js.Value, []js.Value) any {
			c.post(func() { c.failed = true; c.status = "Saved on this device only (online features could not load)" })
			return nil
		}))
		doc.Get("head").Call("appendChild", s)
	}
	next()
}

func (c *fbCloud) init() {
	fb := js.Global().Get("firebase")
	if fb.IsUndefined() {
		c.post(func() { c.failed = true; c.status = "Saved on this device only (online features could not load)" })
		return
	}
	if fb.Get("apps").Length() == 0 {
		fb.Call("initializeApp", js.ValueOf(fbConfig))
	}
	c.fs = fb.Call("firestore")
	au := fb.Call("auth")
	au.Call("getRedirectResult").Call("catch", js.FuncOf(func(js.Value, []js.Value) any { return nil }))
	au.Call("onAuthStateChanged", js.FuncOf(func(this js.Value, a []js.Value) any {
		u := a[0]
		uid, name := "", ""
		if !u.IsNull() && !u.IsUndefined() {
			uid = u.Get("uid").String()
			if dn := u.Get("displayName"); !dn.IsNull() && !dn.IsUndefined() {
				name = dn.String()
			}
		}
		c.post(func() {
			c.ready = true
			c.uid, c.name = uid, name
			if uid == "" {
				c.status = "Saved on this device · sign in with Google to sync across devices and join leaderboards"
			} else {
				c.status = "Signed in as " + firstWord(name)
			}
			c.signedIn = true
		})
		return nil
	}))
	c.post(func() {
		c.ready = true
		c.status = "Saved on this device · sign in with Google to sync across devices and join leaderboards"
	})
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	if s == "" {
		return "player"
	}
	return s
}

func (c *fbCloud) Available() bool { return !c.failed }

// NeedPush reports (once) that the cloud had no save: ours should be pushed.
func (c *fbCloud) NeedPush() bool { v := c.needPush; c.needPush = false; return v }
func (c *fbCloud) Online() bool   { return c.uid != "" && !c.fs.IsUndefined() }
func (c *fbCloud) UID() string    { return c.uid }
func (c *fbCloud) Name() string   { return c.name }
func (c *fbCloud) Status() string { return c.status }

func (c *fbCloud) SignIn() {
	fb := js.Global().Get("firebase")
	if fb.IsUndefined() {
		return
	}
	au := fb.Call("auth")
	pr := fb.Get("auth").Get("GoogleAuthProvider").New()
	pr.Call("setCustomParameters", js.ValueOf(map[string]any{"prompt": "select_account"}))
	au.Call("signInWithPopup", pr).Call("catch", js.FuncOf(func(this js.Value, a []js.Value) any {
		code := a[0].Get("code").String()
		if code == "auth/popup-blocked" || code == "auth/operation-not-supported-in-this-environment" {
			au.Call("signInWithRedirect", pr)
		} else if code != "auth/popup-closed-by-user" && code != "auth/cancelled-popup-request" {
			c.post(func() { c.status = "Sign-in failed: " + code })
		}
		return nil
	}))
}

func (c *fbCloud) SignOut() {
	if fb := js.Global().Get("firebase"); !fb.IsUndefined() {
		fb.Call("auth").Call("signOut")
	}
	if !c.unsub.IsUndefined() && !c.unsub.IsNull() {
		c.unsub.Invoke()
		c.unsub = js.Undefined()
	}
}

// Poll runs queued events; when newly signed in it starts the save sync and watches the save doc.
func (c *fbCloud) Poll(onSave func(*meta.Save)) {
	c.mu.Lock()
	ev := c.events
	c.events = nil
	c.mu.Unlock()
	for _, f := range ev {
		f()
	}
	if c.signedIn {
		c.signedIn = false
		if c.uid != "" {
			c.watch(onSave)
		}
	}
}

// fbEnc mirrors the web game's document shape: j = the JSON, plus top-level scalars for queries.
func fbEnc(d map[string]any) js.Value {
	b, _ := json.Marshal(d)
	o := map[string]any{"j": string(b)}
	for k, v := range d {
		switch v.(type) {
		case string, float64, bool:
			o[k] = v
		}
	}
	return js.ValueOf(o)
}

func fbDecode(snap js.Value) map[string]any {
	if !snap.Get("exists").Bool() {
		return nil
	}
	d := snap.Call("data")
	if j := d.Get("j"); j.Type() == js.TypeString {
		var m map[string]any
		if json.Unmarshal([]byte(j.String()), &m) == nil {
			return m
		}
	}
	return nil
}

func (c *fbCloud) saveDoc() js.Value { return c.fs.Call("doc", "saves/"+c.uid) }

func (c *fbCloud) watch(onSave func(*meta.Save)) {
	ref := c.saveDoc()
	// first sync: adopt a newer cloud save, otherwise push ours (the caller decides via onSave / PushSave)
	ref.Call("get").Call("then", js.FuncOf(func(this js.Value, a []js.Value) any {
		m := fbDecode(a[0])
		c.post(func() {
			if m == nil {
				c.status = "Signed in as " + firstWord(c.name) + " · synced"
				c.needPush = true
				return
			}
			s, err := meta.ParseSave(mustJSON(m))
			if err != nil {
				c.needPush = true
				return
			}
			onSave(s)
			c.status = "Signed in as " + firstWord(c.name) + " · synced"
		})
		return nil
	}), js.FuncOf(func(this js.Value, a []js.Value) any {
		c.post(func() { c.status = "Sync unavailable – saved on this device" })
		return nil
	}))
	c.unsub = ref.Call("onSnapshot", js.FuncOf(func(this js.Value, a []js.Value) any {
		m := fbDecode(a[0])
		if m == nil {
			return nil
		}
		if s, err := meta.ParseSave(mustJSON(m)); err == nil {
			c.post(func() { onSave(s) })
		}
		return nil
	}), js.FuncOf(func(js.Value, []js.Value) any { return nil }))
}

func mustJSON(m map[string]any) []byte { b, _ := json.Marshal(m); return b }

func (c *fbCloud) PushSave(s *meta.Save) {
	if !c.Online() {
		return
	}
	c.saveDoc().Call("set", fbEnc(s.M)).Call("then", js.FuncOf(func(js.Value, []js.Value) any {
		c.post(func() { c.status = "Signed in as " + firstWord(c.name) + " · synced" })
		return nil
	}), js.FuncOf(func(js.Value, []js.Value) any {
		c.post(func() { c.status = "Sync failed – saved on this device" })
		return nil
	}))
}

func (c *fbCloud) PushLB(lb map[string]any) {
	if !c.Online() || c.lbBusy {
		return
	}
	c.lbBusy = true
	done := js.FuncOf(func(js.Value, []js.Value) any { c.post(func() { c.lbBusy = false }); return nil })
	c.fs.Call("doc", "lb/"+c.uid).Call("set", fbEnc(lb)).Call("then", done, done)
}

func (c *fbCloud) FetchBoard(period, field, dayKey string, cb func([]meta.LBRow, error)) {
	if !c.Online() {
		cb(nil, errors.New("offline"))
		return
	}
	q := c.fs.Call("collection", "lb")
	filtered := false
	switch period {
	case "day":
		q = q.Call("where", "dayKey", "==", dayKey)
		filtered = true
	case "week":
		q = q.Call("where", "weekKey", "==", meta.WeekKey(nowUTC()))
		filtered = true
	default:
		q = q.Call("orderBy", field, "desc").Call("limit", 25)
	}
	q.Call("get").Call("then", js.FuncOf(func(this js.Value, a []js.Value) any {
		docs := a[0].Get("docs")
		var rows []meta.LBRow
		for i := 0; i < docs.Length(); i++ {
			d := docs.Index(i)
			m := fbDecode(d)
			if m == nil {
				continue
			}
			v := int(numAny(m[field]))
			run := meta.ParseLBRun(m[field+"Run"])
			if v <= 0 || run == nil {
				continue
			}
			name, _ := m["name"].(string)
			if name == "" {
				name = "Player"
			}
			rows = append(rows, meta.LBRow{UID: d.Get("id").String(), Name: name, V: v, Run: *run})
		}
		if filtered {
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].V > rows[j].V })
			if len(rows) > 25 {
				rows = rows[:25]
			}
		}
		c.post(func() { cb(rows, nil) })
		return nil
	}), js.FuncOf(func(this js.Value, a []js.Value) any {
		c.post(func() { cb(nil, errors.New("could not load the board")) })
		return nil
	}))
}

func numAny(v any) float64 {
	f, _ := v.(float64)
	return f
}

func nowUTC() time.Time { return time.Now() }
