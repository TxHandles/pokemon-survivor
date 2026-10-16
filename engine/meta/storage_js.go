//go:build js

package meta

import "syscall/js"

// The browser build uses the same localStorage key as the web game (ms_save), so on the same site
// both versions share one save.
const storageKey = "ms_save"

func SavePath() string { return "localStorage:" + storageKey }

func LoadStored() *Save {
	ls := js.Global().Get("localStorage")
	if ls.IsUndefined() || ls.IsNull() {
		return NewSave()
	}
	v := ls.Call("getItem", storageKey)
	if v.IsNull() || v.IsUndefined() {
		return NewSave()
	}
	s, err := ParseSave([]byte(v.String()))
	if err != nil {
		ls.Call("setItem", "ms_save_old", v)
		return NewSave()
	}
	return s
}

func (s *Save) Store() error {
	ls := js.Global().Get("localStorage")
	if ls.IsUndefined() || ls.IsNull() {
		return nil
	}
	ls.Call("setItem", storageKey, string(s.JSON()))
	return nil
}
