package net

// Store is the small slice of Firestore the friends system needs (the web game's users/{uid},
// codes/{code}, users/{uid}/friends/{id} and users/{uid}/invites/{from} documents). The browser
// build uses the Firebase SDK; the desktop build uses the REST API with its anonymous account.
// Callbacks may run on another goroutine.
type Store interface {
	UID() string
	Get(path string, cb func(d Doc, ok bool, err string))
	Set(path string, d Doc, merge bool, cb func(err string))
	Delete(path string, cb func(err string))
	List(path string, cb func(docs map[string]Doc, err string))
}

// Doc is a document as plain values: string, float64 or bool.
type Doc map[string]any

// Str returns field k as a string ("" if missing).
func (d Doc) Str(k string) string { s, _ := d[k].(string); return s }

// Num returns field k as a number (0 if missing).
func (d Doc) Num(k string) float64 {
	switch v := d[k].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	}
	return 0
}

// FriendAlphabet: friend-code characters (web FRAL, no look-alike letters).
const FriendAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// StoreOf returns n's Firestore store, or nil when n has none (test relays).
func StoreOf(n Net) Store {
	if s, ok := n.(interface{ Store() Store }); ok {
		return s.Store()
	}
	return nil
}

// IdentityFile, when set, keeps the desktop's anonymous account between launches (its refresh
// token), so the player keeps the same friend code. Unused in the browser.
var IdentityFile string
