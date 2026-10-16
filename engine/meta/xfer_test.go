package meta

import (
	"os"
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// WEBCODE: a save code made by the web game; GOCODE_OUT: where to write a Go-made code for the web to read.
func TestSaveCodes(t *testing.T) {
	f := os.Getenv("WEBCODE")
	if f == "" {
		t.Skip("set WEBCODE")
	}
	b, _ := os.ReadFile(f)
	s, err := DecodeCode(string(b))
	if err != nil {
		t.Fatal(err)
	}
	db, _ := content.Load(data.FS)
	p := NewProfile(s, db)
	if s.Coins() != 1234 || s.Tickets() != 5 || !p.Unl("cater") || p.UL("dmg", "fire") != 3 || p.UL("rr", "") != 2 || p.MasteryRank("fire") < 5 {
		t.Fatalf("decoded wrong: coins %d tickets %d cater %v dmg %d rr %d mastery %d", s.Coins(), s.Tickets(), p.Unl("cater"), p.UL("dmg", "fire"), p.UL("rr", ""), p.MasteryRank("fire"))
	}
	// play a fake run and hand the save back
	p.Finish(RunResult{CID: "fire", T: 400, Kills: 900, Coins: 50, Tickets: 1, Bosses: 1, Visited: []string{"fire"}, Dmg: map[string]float64{"fire_ember": 5000}})
	if out := os.Getenv("GOCODE_OUT"); out != "" {
		os.WriteFile(out, []byte(s.EncodeCode()), 0o644)
	}
	t.Logf("web code OK; after a run: %d coins, %d tickets", s.Coins(), s.Tickets())
}
