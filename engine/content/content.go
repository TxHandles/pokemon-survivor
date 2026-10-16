// Package content loads the game data (data/*.json) shared with the web version.
package content

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
)

type Stats struct {
	HP  float64 `json:"hp"`
	Atk float64 `json:"atk"`
	Def float64 `json:"def"`
	Spd float64 `json:"spd"`
}

type Pokemon struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Dex         int             `json:"dex"`
	Types       []string        `json:"types"`
	EvolvesFrom string          `json:"evolvesFrom"`
	RegionalOf  string          `json:"regionalOf"`
	Form        string          `json:"form"`
	StatsFrom   string          `json:"statsFrom"`
	StatMult    float64         `json:"statMult"`
	BaseStats   *Stats          `json:"baseStats"`
	Tickets     int             `json:"tickets"`
	Unlock      json.RawMessage `json:"unlock"`
	UnlockText  string          `json:"unlockText"`
	EnemyPool   []string        `json:"enemyPool"`
	Tag         string          `json:"tag"`
	DexNote     string          `json:"dexNote"`
	SpriteScale float64         `json:"spriteScale"`
	Legendary   bool            `json:"legendary"`
	Abilities   *struct {
		Starting int `json:"starting"`
		Hidden   int `json:"hidden"`
	} `json:"abilities"`
}

type Move struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Shape    string         `json:"shape"`
	Tier     string         `json:"tier"`
	Desc     string         `json:"desc"`
	HeldItem string         `json:"heldItem"`
	Icon     string         `json:"icon"`
	Params   map[string]any `json:"params"`
}

type Learnset struct {
	Start      string              `json:"start"`
	Moves      []string            `json:"moves"`
	SecondType map[string][]string `json:"secondType"`
}

type Balance struct {
	TypeStats         map[string][3]float64 `json:"typeStats"`
	MoveDamage        map[string]float64    `json:"moveDamage"`
	MoveDamageEvolved map[string]float64    `json:"moveDamageEvolved"`
}

type SpriteView struct {
	File  string  `json:"file"`
	CW    int     `json:"cw"`
	CH    int     `json:"ch"`
	OX    float64 `json:"ox"`
	OY    float64 `json:"oy"`
	Idle  []int   `json:"idle"`
	Idur  []int   `json:"idur"`
	Act   []int   `json:"act"`
	Adur  []int   `json:"adur"`
	Faces string  `json:"faces"`
}

type SpriteSet struct {
	Scale    float64     `json:"scale"`
	Front    *SpriteView `json:"front"`
	Back     *SpriteView `json:"back"`
	Portrait string      `json:"portrait"`
}

// Ability is one Pokémon ability; P holds its effect parameters (read by engine/sim).
type Ability struct {
	ID   string         `json:"-"`
	Name string         `json:"n"`
	Desc string         `json:"d"`
	P    map[string]any `json:"p"`
	Tier int            `json:"-"` // 1 starting, 2 hidden, 3 legendary
}

type AbilitySet struct {
	T1  []Ability `json:"t1"`
	T2  []Ability `json:"t2"`
	Leg Ability   `json:"leg"`
}

// Track is a location's music with sample-exact loop points (44.1 kHz).
type Track struct {
	File            string `json:"file"`
	LoopStartSample int64  `json:"loopStartSample"`
	LoopEndSample   int64  `json:"loopEndSample"`
}

type World struct {
	Locations map[string][]string `json:"locations"`
	Music     map[string]*Track   `json:"music"` // "biome:locationIndex"
}

// DB is all loaded content.
type DB struct {
	Pokemon   []*Pokemon
	ByID      map[string]*Pokemon
	Moves     map[string]*Move
	MoveList  []*Move
	Learnsets map[string]*Learnset
	Balance   Balance
	Sprites   map[string]*SpriteSet
	World     World
	Stats     map[string]Stats // final stats per Pokemon (type base x type balance x evolution multiplier)
	Abilities map[string]*AbilitySet
	children  map[string][]string
}

var Types = []string{"normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison", "ground", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"}

func readJSON(f fs.FS, name string, v any) error {
	b, err := fs.ReadFile(f, name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// Load reads every content file from f (the data/ directory).
func Load(f fs.FS) (*DB, error) {
	db := &DB{ByID: map[string]*Pokemon{}, Moves: map[string]*Move{}, Stats: map[string]Stats{}, children: map[string][]string{}}
	var pk struct{ Pokemon []*Pokemon }
	var mv struct{ Moves []*Move }
	var ls struct{ Learnsets map[string]*Learnset }
	var sp struct{ Sprites map[string]*SpriteSet }
	var ab struct{ Abilities map[string]*AbilitySet }
	for _, x := range []struct {
		n string
		v any
	}{{"pokemon.json", &pk}, {"moves.json", &mv}, {"learnsets.json", &ls}, {"balance.json", &db.Balance}, {"sprites/sprites.json", &sp}, {"world.json", &db.World}, {"abilities.json", &ab}} {
		if err := readJSON(f, x.n, x.v); err != nil {
			return nil, err
		}
	}
	db.Pokemon, db.MoveList, db.Learnsets, db.Sprites, db.Abilities = pk.Pokemon, mv.Moves, ls.Learnsets, sp.Sprites, ab.Abilities
	for _, p := range db.Pokemon {
		db.ByID[p.ID] = p
		if p.EvolvesFrom != "" {
			db.children[p.EvolvesFrom] = append(db.children[p.EvolvesFrom], p.ID)
		}
	}
	for _, m := range db.MoveList {
		db.Moves[m.ID] = m
	}
	// stats: type base forms get baseStats x type balance; others copy their statsFrom base x statMult
	base := map[string]Stats{}
	for _, p := range db.Pokemon {
		if p.BaseStats != nil {
			s := *p.BaseStats
			if m, ok := db.Balance.TypeStats[p.ID]; ok {
				s.HP *= m[0]
				s.Atk *= m[1]
				s.Def *= m[2]
			}
			base[p.ID] = s
			db.Stats[p.ID] = s
		}
	}
	for _, p := range db.Pokemon {
		if p.BaseStats == nil {
			b, ok := base[p.StatsFrom]
			if !ok {
				return nil, fmt.Errorf("%s: statsFrom %q has no base stats", p.ID, p.StatsFrom)
			}
			m := p.StatMult
			db.Stats[p.ID] = Stats{b.HP * m, b.Atk * m, b.Def * m, b.Spd * m}
		}
	}
	return db, nil
}

// LineRoot walks evolutions back to the first form.
func (db *DB) LineRoot(id string) string {
	for {
		p := db.ByID[id]
		if p == nil || p.EvolvesFrom == "" {
			return id
		}
		id = p.EvolvesFrom
	}
}

func (db *DB) depth(id string) int {
	n := 1
	for p := db.ByID[id]; p != nil && p.EvolvesFrom != ""; p = db.ByID[p.EvolvesFrom] {
		n++
	}
	return n
}

func (db *DB) lineMembers(root string) []string {
	out := []string{root}
	for i := 0; i < len(out); i++ {
		out = append(out, db.children[out[i]]...)
	}
	return out
}

// learnRoot: the nearest form (itself or an ancestor) that has a learnset.
func (db *DB) learnRoot(id string) string {
	for id != "" {
		if _, ok := db.Learnsets[id]; ok {
			return id
		}
		p := db.ByID[id]
		if p == nil {
			return ""
		}
		id = p.EvolvesFrom
	}
	return ""
}

// TierOK mirrors the web rules: basic always; evolved from stage 2; final on fully evolved forms;
// lines with no evolutions get every tier.
func (db *DB) TierOK(id, tier string) bool {
	if tier == "basic" {
		return true
	}
	root := db.learnRoot(id)
	if root == "" {
		root = id
	}
	mx := 0
	for _, m := range db.lineMembers(db.LineRoot(root)) {
		if d := db.depth(m); d > mx {
			mx = d
		}
	}
	if mx <= 1 {
		return true
	}
	me := db.depth(id)
	if tier == "evolved" {
		return me >= 2
	}
	return me >= 2 && len(db.children[id]) == 0
}

// LearnList returns the moves this Pokemon can learn in a run, in learnset order.
func (db *DB) LearnList(id string) []string {
	p := db.ByID[id]
	r := db.learnRoot(id)
	var ids []string
	if ls := db.Learnsets[r]; ls != nil {
		ids = append(ids, ls.Moves...)
		types := []string{}
		for t := range ls.SecondType {
			types = append(types, t)
		}
		sort.Strings(types)
		for _, t := range types {
			for _, pt := range p.Types {
				if pt == t {
					ids = append(ids, ls.SecondType[t]...)
				}
			}
		}
	} else {
		for _, m := range db.MoveList {
			for _, t := range p.Types {
				if m.Type == t {
					ids = append(ids, m.ID)
				}
			}
		}
	}
	out := ids[:0:0]
	for _, x := range ids {
		if m := db.Moves[x]; m != nil && db.TierOK(id, m.Tier) {
			out = append(out, x)
		}
	}
	return out
}

// StartMove is the line's starting move.
func (db *DB) StartMove(id string) string {
	if ls := db.Learnsets[db.learnRoot(id)]; ls != nil {
		return ls.Start
	}
	for _, m := range db.MoveList {
		if m.Type == db.ByID[id].Types[0] && m.Tier == "basic" {
			return m.ID
		}
	}
	return db.MoveList[0].ID
}

// EnemyPool returns Pokemon that appear as regular enemies of type t (first forms of that type + explicit reg pools).
func (db *DB) EnemyPool(t string) []string {
	var out []string
	for _, p := range db.Pokemon {
		if p.Legendary {
			continue
		}
		has := false
		for _, x := range p.Types {
			if x == t {
				has = true
			}
		}
		if !has {
			continue
		}
		reg := p.EvolvesFrom == ""
		if p.EnemyPool != nil {
			reg = false
			for _, k := range p.EnemyPool {
				if k == "reg" {
					reg = true
				}
			}
		}
		if reg {
			out = append(out, p.ID)
		}
	}
	return out
}

func abHash(s string) uint32 {
	h := uint32(2166136261)
	for _, c := range utf16(s) {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

func utf16(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
}

// AbilityPair returns the starting and hidden ability of a Pokémon (web abList): the line root's assignment
// (pokemon.json "abilities") picks one tier-1 and one tier-2 ability of the Pokémon's primary type;
// legendaries get their type's legendary ability as the second one.
func (db *DB) AbilityPair(id string) [2]Ability {
	p := db.ByID[id]
	ty := p.Types[0]
	set := db.Abilities[ty]
	if set == nil {
		set = db.Abilities["normal"]
	}
	root := db.LineRoot(id)
	i1, i2 := 0, 0
	if r := db.ByID[root]; r != nil && r.Abilities != nil {
		i1, i2 = r.Abilities.Starting, r.Abilities.Hidden
	} else {
		h := abHash(root)
		i1, i2 = int(h%3), int((h>>5)%3)
	}
	a := set.T1[i1%len(set.T1)]
	a.ID, a.Tier = fmt.Sprintf("%s:t1:%d", ty, i1), 1
	var b Ability
	if p.Legendary {
		b = set.Leg
		b.ID, b.Tier = ty+":leg", 3
	} else {
		b = set.T2[i2%len(set.T2)]
		b.ID, b.Tier = fmt.Sprintf("%s:t2:%d", ty, i2), 2
	}
	return [2]Ability{a, b}
}
