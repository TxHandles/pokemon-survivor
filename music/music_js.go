//go:build js

package music

import (
	"io"
	"net/http"
)

// Load fetches music/<name> relative to the page and calls done with the bytes (nil on failure).
func Load(name string, done func([]byte)) {
	go func() {
		r, err := http.Get("music/" + name)
		if err != nil || r.StatusCode != 200 {
			done(nil)
			return
		}
		defer r.Body.Close()
		b, err := io.ReadAll(r.Body)
		if err != nil {
			done(nil)
			return
		}
		done(b)
	}()
}
