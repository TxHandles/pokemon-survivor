// Package meta is the progression layer: the save file, the shop, unlocks, mastery, challenges and run
// rewards. The save uses the web game's format (the same JSON object, version wv=2), so progress moves
// between the web and Go versions with a save code, and unknown fields are kept untouched.
package meta

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"
)

// SaveVersion is the web game's save version (WIPE).
const SaveVersion = 2

// Save is the raw save object. Accessors below read and write the fields the game uses.
type Save struct {
	M map[string]any
}

func NewSave() *Save {
	s := &Save{M: map[string]any{}}
	s.normalize()
	return s
}

// ParseSave reads a save from JSON (returns an error if it is from another version).
func ParseSave(b []byte) (*Save, error) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("empty save")
	}
	if num(m["wv"]) != SaveVersion {
		return nil, errors.New("that save is from an incompatible version")
	}
	s := &Save{M: m}
	s.normalize()
	return s, nil
}

func (s *Save) JSON() []byte {
	s.M["ts"] = float64(time.Now().UnixMilli())
	b, _ := json.Marshal(s.M)
	return b
}

func (s *Save) normalize() {
	m := s.M
	m["wv"] = float64(SaveVersion)
	for _, k := range []string{"u", "cu", "gu", "cl", "col", "ci", "ms", "bu", "pu", "pp", "fl", "unl", "cs", "mx", "locb"} {
		if _, ok := m[k].(map[string]any); !ok {
			m[k] = map[string]any{}
		}
	}
	st, ok := m["st"].(map[string]any)
	if !ok {
		st = map[string]any{}
		m["st"] = st
	}
	for _, k := range []string{"k", "b", "t", "r", "w", "mv", "ev", "cp", "a", "el", "bi", "bs"} {
		if _, ok := st[k]; !ok {
			st[k] = 0.0
		}
	}
	if _, ok := m["hist"].([]any); !ok {
		m["hist"] = []any{}
	}
	if _, ok := m["rs"].(map[string]any); !ok {
		m["rs"] = map[string]any{"tier": 0.0, "mods": []any{}}
	}
	if _, ok := m["coins"]; !ok {
		m["coins"] = 0.0
	}
}

// ---- generic helpers ----

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case bool:
		if x {
			return 1
		}
	}
	return 0
}

func (s *Save) obj(k string) map[string]any {
	m, ok := s.M[k].(map[string]any)
	if !ok {
		m = map[string]any{}
		s.M[k] = m
	}
	return m
}

func sub(m map[string]any, k string) map[string]any {
	x, ok := m[k].(map[string]any)
	if !ok {
		x = map[string]any{}
		m[k] = x
	}
	return x
}

func (s *Save) Num(k string) float64       { return num(s.M[k]) }
func (s *Save) SetNum(k string, v float64) { s.M[k] = v }
func (s *Save) Add(k string, v float64)    { s.M[k] = num(s.M[k]) + v }

func (s *Save) Coins() int   { return int(s.Num("coins")) }
func (s *Save) Tickets() int { return int(s.Num("tickets")) }

// Stat returns a lifetime stat (SV.st).
func (s *Save) Stat(k string) float64 { return num(s.obj("st")[k]) }
func (s *Save) AddStat(k string, v float64) {
	st := s.obj("st")
	st[k] = num(st[k]) + v
}
func (s *Save) MaxStat(k string, v float64) {
	st := s.obj("st")
	st[k] = math.Max(num(st[k]), v)
}
func (s *Save) SetStat(k string, v float64) { s.obj("st")[k] = v }

// Flag / counter used by unlock rules (SV.fl).
func (s *Save) Flag(k string) float64       { return num(s.obj("fl")[k]) }
func (s *Save) SetFlag(k string, v float64) { s.obj("fl")[k] = v }

func (s *Save) Unlocked(id string) bool { return num(s.obj("unl")[id]) > 0 }
func (s *Save) SetUnlocked(id string)   { s.obj("unl")[id] = 1.0 }

// Sel is the selected Pokémon.
func (s *Save) Sel() string      { v, _ := s.M["sel"].(string); return v }
func (s *Save) SetSel(id string) { s.M["sel"] = id }

// RunSetup (SV.rs): chosen biome / location / tier / difficulty / modifiers.
func (s *Save) RunSetup() map[string]any { return s.obj("rs") }

// ---- transfer codes (web xferEnc / xferDec) ----

// EncodeCode makes a save code (PSZ1 = gzip+base64, like the web game's compressed codes).
func (s *Save) EncodeCode() string {
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	z.Write(s.JSON())
	z.Close()
	return "PSZ1." + base64.StdEncoding.EncodeToString(b.Bytes())
}

// DecodeCode reads a save code from the web or Go version.
func DecodeCode(c string) (*Save, error) {
	c = strings.Join(strings.Fields(c), "")
	switch {
	case strings.HasPrefix(c, "PSJ1."):
		b, err := base64.StdEncoding.DecodeString(c[5:])
		if err != nil {
			return nil, errors.New("that code is damaged")
		}
		return ParseSave(b)
	case strings.HasPrefix(c, "PSZ1."):
		b, err := base64.StdEncoding.DecodeString(c[5:])
		if err != nil {
			return nil, errors.New("that code is damaged")
		}
		z, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, errors.New("that code is damaged")
		}
		j, err := io.ReadAll(z)
		if err != nil {
			return nil, errors.New("that code is damaged")
		}
		return ParseSave(j)
	}
	return nil, errors.New("not a Pokémon Survivor save code")
}

func nowMs() int64 { return time.Now().UnixMilli() }
