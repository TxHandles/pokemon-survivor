//go:build js

package main

import "syscall/js"

func options() map[string]string {
	return parseKV(js.Global().Get("location").Get("search").String())
}
