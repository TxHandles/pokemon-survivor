package meta

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// Compares unlock rules and mastery ranks with the web game on random save states (METAWEB=dump from tools).
func TestUnlockRulesMatchWeb(t *testing.T) {
	f := os.Getenv("METAWEB")
	if f == "" {
		t.Skip("set METAWEB")
	}
	b, _ := os.ReadFile(f)
	var d struct {
		States []map[string]any
		Res    []struct {
			R map[string][2]any
			M map[string]float64
		}
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	db, _ := content.Load(data.FS)
	bad := 0
	for i, st := range d.States {
		s := NewSave()
		for k, v := range st {
			s.M[k] = v
		}
		s.M["mxs"], s.M["csb"] = 1.0, 1.0
		p := &Profile{S: s, DB: db}
		for id, want := range d.Res[i].R {
			r := p.rule(id)
			got := r != nil && p.urOK(*r)
			if r != nil && r.Tickets != nil {
				got = false
			}
			gh, gn := p.Progress(id)
			wp := want[1].([]any)
			if got != want[0].(bool) || float64(gh) != wp[0].(float64) || float64(gn) != wp[1].(float64) {
				if bad < 8 {
					t.Errorf("state %d %s: go %v %d/%d web %v %v", i, id, got, gh, gn, want[0], wp)
				}
				bad++
			}
		}
		for root, want := range d.Res[i].M {
			if got := p.MasteryRank(root); float64(got) != want {
				t.Errorf("state %d mastery %s: go %d web %v", i, root, got, want)
			}
		}
	}
	t.Logf("%d states compared, %d mismatches", len(d.States), bad)
}

// Boss-kill sequences: flags, biome unlocks/stars, Pokémon unlocks and location bits match the web (BOSSWEB=dump).
func TestBossKillMatchesWeb(t *testing.T) {
	f := os.Getenv("BOSSWEB")
	if f == "" {
		t.Skip("set BOSSWEB")
	}
	b, _ := os.ReadFile(f)
	var d struct {
		Seqs []struct {
			Mx map[string]any
			Ev []struct {
				Bi, Et, Cid string
				Fin         bool
				Sg, Li      int
				Bv          []string
			}
		}
		Res [][]struct {
			N    int
			Fl   map[string]any
			Bu   map[string]any
			Unl  []string
			Locb map[string]any
		}
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	db, _ := content.Load(data.FS)
	bad := 0
	for i, q := range d.Seqs {
		s := NewSave()
		s.M["mx"], s.M["mxs"], s.M["csb"] = q.Mx, 1.0, 1.0
		p := &Profile{S: s, DB: db}
		for j, e := range q.Ev {
			m := p.BossKill(e.Bi, e.Et, e.Li, e.Fin, e.Sg, e.Cid, e.Bv)
			w := d.Res[i][j]
			unl := []string{}
			for k := range s.obj("unl") {
				unl = append(unl, k)
			}
			sortStrings(unl)
			eq := func(a, b any) bool { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return string(x) == string(y) }
			if len(m) != w.N || !eq(s.obj("fl"), w.Fl) || !eq(s.obj("bu"), w.Bu) || !eq(unl, w.Unl) || !eq(s.obj("locb"), w.Locb) {
				if bad < 5 {
					t.Errorf("seq %d step %d: fl %v/%v bu %v/%v unl %v/%v locb %v/%v", i, j, eq(s.obj("fl"), w.Fl), s.obj("fl"), eq(s.obj("bu"), w.Bu), w.Fl, eq(unl, w.Unl), unl, eq(s.obj("locb"), w.Locb), w.Unl)
				}
				bad++
			}
		}
	}
	t.Logf("%d sequences compared, %d mismatching steps", len(d.Seqs), bad)
}
