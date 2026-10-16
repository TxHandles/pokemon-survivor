//go:build js

package music

import "syscall/js"

// Load fetches music/<name> relative to the page (browser fetch, no net/http: keeps the wasm small)
// and calls done with the bytes, or nil on failure.
func Load(name string, done func([]byte)) {
	fail := func() { done(nil) }
	var onResp, onBuf, onErr js.Func
	release := func() { onResp.Release(); onBuf.Release(); onErr.Release() }
	onErr = js.FuncOf(func(this js.Value, a []js.Value) any { fail(); release(); return nil })
	onBuf = js.FuncOf(func(this js.Value, a []js.Value) any {
		u8 := js.Global().Get("Uint8Array").New(a[0])
		b := make([]byte, u8.Get("length").Int())
		js.CopyBytesToGo(b, u8)
		done(b)
		release()
		return nil
	})
	onResp = js.FuncOf(func(this js.Value, a []js.Value) any {
		r := a[0]
		if !r.Get("ok").Bool() {
			fail()
			release()
			return nil
		}
		r.Call("arrayBuffer").Call("then", onBuf, onErr)
		return nil
	})
	js.Global().Call("fetch", "music/"+name).Call("then", onResp, onErr)
}
