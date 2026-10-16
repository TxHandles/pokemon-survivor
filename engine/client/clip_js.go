//go:build js

package client

import "syscall/js"

func clipRead(done func(string)) {
	nav := js.Global().Get("navigator")
	cb := nav.Get("clipboard")
	fallback := func() {
		v := js.Global().Call("prompt", "Paste your save code:")
		if v.IsNull() || v.IsUndefined() {
			done("")
			return
		}
		done(v.String())
	}
	if cb.IsUndefined() || cb.Get("readText").IsUndefined() {
		fallback()
		return
	}
	var ok, fail js.Func
	ok = js.FuncOf(func(this js.Value, a []js.Value) any { done(a[0].String()); ok.Release(); fail.Release(); return nil })
	fail = js.FuncOf(func(this js.Value, a []js.Value) any { fallback(); ok.Release(); fail.Release(); return nil })
	cb.Call("readText").Call("then", ok, fail)
}

func clipWrite(s string) bool {
	cb := js.Global().Get("navigator").Get("clipboard")
	if cb.IsUndefined() {
		js.Global().Call("prompt", "Copy your save code:", s)
		return true
	}
	cb.Call("writeText", s)
	return true
}
