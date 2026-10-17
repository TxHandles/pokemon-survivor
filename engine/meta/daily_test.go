package meta

import (
	"encoding/json"
	"os"
	"testing"
)

func TestDailyMatchesWeb(t *testing.T) {
	f := os.Getenv("DAILYWEB")
	if f == "" {
		t.Skip("set DAILYWEB")
	}
	b, _ := os.ReadFile(f)
	var rows [][]any
	json.Unmarshal(b, &rows)
	bad := 0
	for _, r := range rows {
		key := r[0].(string)
		d := dailyFor(key)
		mods, _ := r[2].([]any)
		if d.Biome != r[1].(string) || d.Mods[0] != mods[0].(string) || d.Mods[1] != mods[1].(string) || float64(d.Seed) != r[3].(float64) {
			bad++
			if bad < 5 {
				t.Errorf("%s: go %v web %v", key, d, r)
			}
		}
	}
	t.Logf("%d days compared, %d mismatches", len(rows), bad)
}
