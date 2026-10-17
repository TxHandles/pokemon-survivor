package client

import "github.com/TxHandles/pokemon-survivor/engine/meta"

// Cloud is the online layer: account, cloud save and leaderboards. The browser build talks to the
// same Firebase project and documents as the web game (saves/{uid}, lb/{uid}), so both versions share
// one account and one leaderboard. The desktop build has no sign-in; it uses save codes instead.
type Cloud interface {
	Online() bool
	UID() string
	Name() string
	Status() string // one line for the menus
	SignIn()
	SignOut()
	// Poll handles events since the last frame; onSave gets a newer save from another device.
	Poll(onSave func(*meta.Save))
	PushSave(s *meta.Save)
	PushLB(lb map[string]any)
	FetchBoard(period, field, dayKey string, cb func([]meta.LBRow, error))
	Available() bool // sign-in exists on this platform
}

type noCloud struct{}

func (noCloud) Online() bool { return false }
func (noCloud) UID() string  { return "" }
func (noCloud) Name() string { return "" }
func (noCloud) Status() string {
	return "Saved on this device · online boards and sync: sign in on the web version"
}
func (noCloud) SignIn()                                                      {}
func (noCloud) SignOut()                                                     {}
func (noCloud) Poll(func(*meta.Save))                                        {}
func (noCloud) PushSave(*meta.Save)                                          {}
func (noCloud) PushLB(map[string]any)                                        {}
func (noCloud) FetchBoard(string, string, string, func([]meta.LBRow, error)) {}
func (noCloud) Available() bool                                              { return false }
