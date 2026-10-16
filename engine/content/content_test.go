package content_test

import (
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
)

func TestLoad(t *testing.T) {
	db, err := content.Load(data.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Pokemon) < 80 || len(db.MoveList) < 300 {
		t.Fatalf("loaded %d Pokémon, %d moves", len(db.Pokemon), len(db.MoveList))
	}
	for _, p := range db.Pokemon {
		if _, ok := db.Stats[p.ID]; !ok {
			t.Errorf("%s has no stats", p.ID)
		}
		if db.Moves[db.StartMove(p.ID)] == nil {
			t.Errorf("%s: bad start move", p.ID)
		}
		if len(db.LearnList(p.ID)) == 0 {
			t.Errorf("%s learns nothing", p.ID)
		}
	}
	// a basic form only gets basic moves; its final evolution gets all tiers
	for _, id := range db.LearnList("fire") {
		if db.Moves[id].Tier != "basic" {
			t.Errorf("Charmander can learn %s (%s)", id, db.Moves[id].Tier)
		}
	}
}
