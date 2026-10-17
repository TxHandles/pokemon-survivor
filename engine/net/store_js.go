//go:build js

package net

import "syscall/js"

// Store through the Firebase SDK the page loads (signed in with Google).

type jsStore struct{}

// Store returns the Firestore store (nil with the test relays).
func (n *rtcNet) Store() Store {
	switch n.sig.(type) {
	case *fbSignal:
		return jsStore{}
	case *bcSignal:
		return lsStore{}
	}
	return nil
}

func (jsStore) UID() string { return fbSignal{}.uid() }

func docRef(path string) js.Value { return fb().Call("firestore").Call("doc", path) }

func jsDoc(v js.Value) Doc {
	d := Doc{}
	if v.IsUndefined() || v.IsNull() {
		return d
	}
	keys := js.Global().Get("Object").Call("keys", v)
	for i := 0; i < keys.Length(); i++ {
		k := keys.Index(i).String()
		x := v.Get(k)
		switch x.Type() {
		case js.TypeString:
			d[k] = x.String()
		case js.TypeNumber:
			d[k] = x.Float()
		case js.TypeBoolean:
			d[k] = x.Bool()
		}
	}
	return d
}

func jsObj(d Doc) map[string]any {
	m := map[string]any{}
	for k, v := range d {
		m[k] = v
	}
	return m
}

func (jsStore) Get(path string, cb func(Doc, bool, string)) {
	defer func() {
		if r := recover(); r != nil {
			cb(nil, false, "firestore unavailable")
		}
	}()
	then(docRef(path).Call("get"), func(s js.Value) {
		if !s.Get("exists").Bool() {
			cb(nil, false, "")
			return
		}
		cb(jsDoc(s.Call("data")), true, "")
	}, func(e js.Value) { cb(nil, false, errText(e)) })
}

func (jsStore) Set(path string, d Doc, merge bool, cb func(string)) {
	defer func() {
		if r := recover(); r != nil && cb != nil {
			cb("firestore unavailable")
		}
	}()
	var p js.Value
	if merge {
		p = docRef(path).Call("set", js.ValueOf(jsObj(d)), js.ValueOf(map[string]any{"merge": true}))
	} else {
		p = docRef(path).Call("set", js.ValueOf(jsObj(d)))
	}
	then(p, func(js.Value) {
		if cb != nil {
			cb("")
		}
	}, func(e js.Value) {
		if cb != nil {
			cb(errText(e))
		}
	})
}

func (jsStore) Delete(path string, cb func(string)) {
	defer func() {
		if r := recover(); r != nil && cb != nil {
			cb("firestore unavailable")
		}
	}()
	then(docRef(path).Call("delete"), func(js.Value) {
		if cb != nil {
			cb("")
		}
	}, func(e js.Value) {
		if cb != nil {
			cb(errText(e))
		}
	})
}

func (jsStore) List(path string, cb func(map[string]Doc, string)) {
	defer func() {
		if r := recover(); r != nil {
			cb(nil, "firestore unavailable")
		}
	}()
	then(fb().Call("firestore").Call("collection", path).Call("get"), func(s js.Value) {
		out := map[string]Doc{}
		docs := s.Get("docs")
		for i := 0; i < docs.Length(); i++ {
			d := docs.Index(i)
			out[d.Get("id").String()] = jsDoc(d.Call("data"))
		}
		cb(out, "")
	}, func(e js.Value) { cb(nil, errText(e)) })
}

// lsStore keeps documents in localStorage (testing with the "local" signal: tabs of one browser
// share it, like they share the BroadcastChannel).
type lsStore struct{}

func (lsStore) UID() string { return "" }

func lsKey(path string) string { return "fs:" + path }

func lsGet(path string) (Doc, bool) {
	v := js.Global().Get("localStorage").Call("getItem", lsKey(path))
	if v.IsNull() || v.IsUndefined() {
		return nil, false
	}
	o := js.Global().Get("JSON").Call("parse", v)
	return jsDoc(o), true
}

func lsPut(path string, d Doc) {
	js.Global().Get("localStorage").Call("setItem", lsKey(path), js.Global().Get("JSON").Call("stringify", js.ValueOf(jsObj(d))))
}

func (lsStore) Get(path string, cb func(Doc, bool, string)) {
	d, ok := lsGet(path)
	go cb(d, ok, "")
}

func (lsStore) Set(path string, d Doc, merge bool, cb func(string)) {
	if merge {
		if old, ok := lsGet(path); ok {
			for k, v := range d {
				old[k] = v
			}
			d = old
		}
	}
	lsPut(path, d)
	if cb != nil {
		go cb("")
	}
}

func (lsStore) Delete(path string, cb func(string)) {
	js.Global().Get("localStorage").Call("removeItem", lsKey(path))
	if cb != nil {
		go cb("")
	}
}

func (lsStore) List(path string, cb func(map[string]Doc, string)) {
	ls := js.Global().Get("localStorage")
	pre := lsKey(path) + "/"
	out := map[string]Doc{}
	for i := 0; i < ls.Get("length").Int(); i++ {
		k := ls.Call("key", i).String()
		if len(k) > len(pre) && k[:len(pre)] == pre {
			rest := k[len(pre):]
			if !containsSlash(rest) {
				d, _ := lsGet(k[3:])
				out[rest] = d
			}
		}
	}
	go cb(out, "")
}

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}
