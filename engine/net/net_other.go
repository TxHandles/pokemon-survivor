//go:build !js

package net

// FixedCode is unused without a transport.
var FixedCode string

type noNet struct{}

// NewNet returns the platform Net. The desktop build has no transport yet.
func NewNet(mode string) Net { return noNet{} }

func (noNet) Available() (bool, string) {
	return false, "Online co-op is only in the browser version for now."
}
func (noNet) Host(string, func(string), func(Peer), func(string)) func() { return func() {} }
func (noNet) Join(string, string, func(Conn), func(string)) func()       { return func() {} }

// SetRoomOpen is a no-op without a transport.
func SetRoomOpen(code string, open bool) {}
