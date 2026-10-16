//go:build !js

// Package music provides the location and menu tracks. Desktop builds embed them; the browser build
// fetches them from music/ next to the page.
package music

import "embed"

//go:embed *.mp3
var files embed.FS

// Load returns the bytes of a track (its file name in music/), or nil.
func Load(name string, done func([]byte)) {
	b, err := files.ReadFile(name)
	if err != nil {
		done(nil)
		return
	}
	done(b)
}
