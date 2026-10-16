package content_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
)

// Compares ability assignments with a dump from the web build (ABWEB=path to JSON {id:[name1,name2]}).
func TestAbilitiesMatchWeb(t *testing.T) {
	f := os.Getenv("ABWEB")
	if f == "" {
		t.Skip("set ABWEB")
	}
	b, _ := os.ReadFile(f)
	var web map[string][2]string
	json.Unmarshal(b, &web)
	db, _ := content.Load(data.FS)
	for id, w := range web {
		a := db.AbilityPair(id)
		if a[0].Name != w[0] || a[1].Name != w[1] {
			t.Errorf("%s: go %s/%s web %s/%s", id, a[0].Name, a[1].Name, w[0], w[1])
		}
	}
	t.Logf("compared %d", len(web))
}
