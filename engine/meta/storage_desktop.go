//go:build !js

package meta

import (
	"os"
	"path/filepath"
)

// SavePath is where the desktop build keeps its save (%APPDATA%\PokemonSurvivor\save.json on Windows).
func SavePath() string {
	d, err := os.UserConfigDir()
	if err != nil || d == "" {
		d = "."
	}
	return filepath.Join(d, "PokemonSurvivor", "save.json")
}

// LoadStored reads the saved progress (a fresh save if there is none).
func LoadStored() *Save {
	b, err := os.ReadFile(SavePath())
	if err != nil {
		return NewSave()
	}
	s, err := ParseSave(b)
	if err != nil {
		os.WriteFile(SavePath()+".old", b, 0o644) // keep an unreadable save rather than losing it
		return NewSave()
	}
	return s
}

// Store writes the save (atomically, via a temp file).
func (s *Save) Store() error {
	p := SavePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, s.JSON(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
