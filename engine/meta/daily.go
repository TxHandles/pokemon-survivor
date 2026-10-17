package meta

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// ---- daily run (web dailyDef): the same biome, modifiers and world for everyone on a given UTC day ----

type Daily struct {
	Key   string
	Biome string
	Mods  []string
	Tier  int
	Seed  int64
}

// the web game's type order (Object.keys(COL)); the daily picks by index into it
var dailyTypes = []string{"fire", "water", "grass", "electric", "ground", "psychic", "rock", "bug", "dark", "dragon", "steel", "fairy", "flying", "fighting", "poison", "ghost", "ice", "normal"}
var modKeys = []string{"swift", "tough", "brutal", "horde", "famine", "glass", "titans", "drought"}

func hash32(s string) uint32 {
	h := uint32(2166136261)
	for _, c := range []byte(s) {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

// mulberry32, as the web game uses it
func mulb(a uint32) func() float64 {
	return func() float64 {
		a += 0x6D2B79F5
		t := a
		t = (t ^ t>>15) * (1 | t)
		t = (t + (t^t>>7)*(61|t)) ^ t
		return float64(t^t>>14) / 4294967296
	}
}

func DayKey(t time.Time) string { return t.UTC().Format("2006-01-02") }
func WeekKey(t time.Time) string { // Monday of the UTC week
	u := t.UTC()
	d := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
	d = d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
	return d.Format("2006-01-02")
}

func UntilReset(t time.Time) string {
	u := t.UTC()
	next := time.Date(u.Year(), u.Month(), u.Day()+1, 0, 0, 0, 0, time.UTC)
	d := next.Sub(u)
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// DailyDef reproduces the web game's daily for a date, including its random-comparator sort of the
// modifier list (V8's TimSort for short arrays: a run count, then binary insertion).
func DailyDef(t time.Time) Daily {
	return dailyFor(DayKey(t))
}

func dailyFor(key string) Daily {
	r := mulb(hash32("daily-" + key))
	bi := dailyTypes[int(math.Floor(r()*18))]
	ms := append([]string{}, modKeys...)
	v8Sort(ms, func() int {
		if r()-.5 < 0 {
			return -1
		}
		return 1
	})
	ws := int64(math.Floor(r() * 1e9))
	return Daily{Key: key, Biome: bi, Mods: ms[:2], Tier: 2, Seed: ws}
}

// v8Sort sorts a short array the way V8 does with a comparator that ignores its arguments
// (CountAndMakeRun, then BinaryInsertionSort).
func v8Sort(a []string, cmp func() int) {
	n := len(a)
	if n < 2 {
		return
	}
	// CountAndMakeRun(0, n)
	run := 2
	desc := cmp() < 0 // Compare(a[1], a[0])
	for idx := 2; idx < n; idx++ {
		o := cmp()
		if desc {
			if o >= 0 {
				break
			}
		} else if o < 0 {
			break
		}
		run++
	}
	if desc {
		for i, j := 0, run-1; i < j; i, j = i+1, j-1 {
			a[i], a[j] = a[j], a[i]
		}
	}
	// BinaryInsertionSort(0, run, n)
	for start := run; start < n; start++ {
		pivot := a[start]
		left, right := 0, start
		for left < right {
			mid := left + (right-left)>>1
			if cmp() < 0 {
				right = mid
			} else {
				left = mid + 1
			}
		}
		copy(a[left+1:start+1], a[left:start])
		a[left] = pivot
	}
}

// ---- leaderboard document (web SV.lbl / lbSubmit) ----

// LB is the player's leaderboard record kept in the save ("lbl").
type LB struct{ M map[string]any }

func (p *Profile) LB() LB {
	m, ok := p.S.M["lbl"].(map[string]any)
	if !ok {
		m = map[string]any{}
		p.S.M["lbl"] = m
	}
	return LB{m}
}

func (l LB) Num(k string) float64 { return num(l.M[k]) }

// LBRun is one submitted daily run.
type LBRun struct {
	C   string `json:"c"`
	K   int    `json:"k"`
	Tm  int    `json:"tm"`
	W   int    `json:"w"`
	Key string `json:"key"`
	Bi  string `json:"bi"`
	MV  []struct {
		ID  string
		Lv  int
		Evo bool
	} `json:"-"`
	Ts int64 `json:"ts"`
}

// LBRow is a leaderboard entry.
type LBRow struct {
	UID, Name string
	V         int
	Run       LBRun
	Me        bool
}

// ParseLBRun reads a run from a save / Firestore value.
func ParseLBRun(v any) *LBRun {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	b, _ := json.Marshal(m)
	var r LBRun
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	if mv, ok := m["mv"].([]any); ok {
		for _, x := range mv {
			l, _ := x.([]any)
			if len(l) >= 3 {
				id, _ := l[0].(string)
				r.MV = append(r.MV, struct {
					ID  string
					Lv  int
					Evo bool
				}{id, int(num(l[1])), num(l[2]) > 0})
			}
		}
	}
	return &r
}

// lbRefresh resets day / week records when the key changed.
func (p *Profile) lbRefresh(now time.Time) LB {
	d := p.LB()
	if dk := DayKey(now); d.M["dayKey"] != dk {
		d.M["dayKey"], d.M["dayKills"], d.M["dayTime"] = dk, 0.0, 0.0
		delete(d.M, "dayKillsRun")
		delete(d.M, "dayTimeRun")
	}
	if wk := WeekKey(now); d.M["weekKey"] != wk {
		d.M["weekKey"], d.M["weekKills"], d.M["weekTime"] = wk, 0.0, 0.0
		delete(d.M, "weekKillsRun")
		delete(d.M, "weekTimeRun")
	}
	if _, ok := d.M["allKills"]; !ok {
		d.M["allKills"] = 0.0
	}
	if _, ok := d.M["allTime"]; !ok {
		d.M["allTime"] = 0.0
	}
	return d
}

// DailyResult records a finished daily run in the leaderboard record (web lbSubmit).
func (p *Profile) DailyResult(now time.Time, d Daily, cid string, kills int, t float64, win bool, moves [][3]any, items [][2]any, name string) {
	l := p.lbRefresh(now)
	mv := []any{}
	for _, m := range moves {
		mv = append(mv, []any{m[0], m[1], m[2]})
	}
	it := []any{}
	for _, m := range items {
		it = append(it, []any{m[0], m[1]})
	}
	run := map[string]any{"c": cid, "k": float64(kills), "tm": math.Round(t), "w": b2n(win), "key": d.Key, "bi": d.Biome, "mv": mv, "it": it, "ts": float64(now.UnixMilli())}
	for _, pf := range []string{"day", "week", "all"} {
		if float64(kills) > num(l.M[pf+"Kills"]) {
			l.M[pf+"Kills"], l.M[pf+"KillsRun"] = float64(kills), run
		}
		if math.Round(t) > num(l.M[pf+"Time"]) {
			l.M[pf+"Time"], l.M[pf+"TimeRun"] = math.Round(t), run
		}
	}
	l.M["name"] = name
}
