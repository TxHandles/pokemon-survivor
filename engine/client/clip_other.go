//go:build !windows && !js

package client

func clipRead(done func(string)) { done("") }
func clipWrite(string) bool      { return false }
