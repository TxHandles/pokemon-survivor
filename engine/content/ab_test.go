package content_test

import (
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
)

func TestAbilities(t *testing.T) {
	db, _ := content.Load(data.FS)
	for _, p := range db.Pokemon {
		ab := db.AbilityPair(p.ID)
		if ab[0].Name == "" || ab[1].Name == "" {
			t.Errorf("%s: missing ability %v", p.ID, ab)
		}
	}
	a := db.AbilityPair("fire")
	t.Logf("Charmander: %s / %s", a[0].Name, a[1].Name)
}
