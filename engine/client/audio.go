package client

import (
	"bytes"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"

	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/synth"
	"github.com/TxHandles/pokemon-survivor/music"
)

// Audio plays sound effects (synthesised, see synth.go) and music: location tracks from music/ with
// sample-exact loops, the menu track, and the procedural placeholder loop for biomes without a track.
type Audio struct {
	ctx   *audio.Context
	vol   func(k string) float64 // settings: vol sfxv musv
	sfxB  map[string][]byte
	last  map[string]float64
	live  []*audio.Player
	xpN   int
	xpT   float64
	now   float64
	mu    sync.Mutex
	files map[string][]byte // raw mp3 by file name
	bad   map[string]bool
	cur   *musicPlayer
	old   *musicPlayer
	want  string // key of the music that should be playing
	proc  *synth.ProcMusic
}

type musicPlayer struct {
	key  string
	p    *audio.Player
	gain float64 // 0..1 fade
	in   bool
	proc *synth.ProcMusic
}

var sfxGap = map[string]float64{"hit": .05, "kill": .035, "elitedown": .2, "bossdown": 1, "xp": .03, "coin": .05, "ticket": .2, "levelup": .3,
	"pickup": .15, "heal": .2, "chest": .3, "hurt": .22, "elite": 1, "boss": 2, "phase": 1, "evolve": .5, "death": 2, "victory": 2, "click": .05, "dash": .15}
var sfxVol = map[string]float64{"hit": .18, "kill": .22, "elitedown": .6, "bossdown": .9, "xp": .22, "coin": .3, "ticket": .5, "levelup": .55,
	"pickup": .5, "heal": .5, "chest": .6, "hurt": .5, "elite": .5, "boss": .8, "phase": .7, "evolve": .6, "death": .8, "victory": .8, "click": .25, "dash": .35}

func NewAudio(vol func(k string) float64) *Audio {
	a := &Audio{vol: vol, sfxB: map[string][]byte{}, last: map[string]float64{}, files: map[string][]byte{}, bad: map[string]bool{}}
	a.ctx = audio.CurrentContext()
	if a.ctx == nil {
		a.ctx = audio.NewContext(synth.SampleRate)
	}
	return a
}

func (a *Audio) cv(k string) float64 { v := a.vol(k); return v * v }

// Play fires a sound effect.
func (a *Audio) Play(name string) {
	if a.ctx == nil || a.cv("vol") == 0 || a.cv("sfxv") == 0 {
		return
	}
	if g, ok := sfxGap[name]; ok && a.now-a.last[name] < g && a.last[name] > 0 {
		return
	}
	// prune finished players; cap the cheap ones
	live := a.live[:0]
	for _, p := range a.live {
		if p.IsPlaying() {
			live = append(live, p)
		} else {
			p.Close()
		}
	}
	a.live = live
	if len(a.live) > 40 && (name == "hit" || name == "kill" || name == "xp") {
		return
	}
	a.last[name] = a.now
	key := name
	if name == "xp" {
		if a.now-a.xpT > .6 {
			a.xpN = 0
		}
		a.xpT = a.now
		a.xpN = min(24, a.xpN+1)
		key = "xp" + itoa(a.xpN)
	}
	b := a.sfxB[key]
	if b == nil {
		b = synth.BuildSFX(name, a.xpN)
		if b == nil {
			return
		}
		if name != "hit" && name != "kill" { // these use random pitches: rebuild each time for variety
			a.sfxB[key] = b
		}
	}
	p := a.ctx.NewPlayerFromBytes(b)
	v, ok := sfxVol[name]
	if !ok {
		v = .5
	}
	p.SetVolume(math.Min(1, v*a.cv("sfxv")*a.cv("vol")*1.6))
	p.Play()
	a.live = append(a.live, p)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// ---- music ----

func (a *Audio) load(file string) []byte {
	a.mu.Lock()
	b, ok := a.files[file]
	bad := a.bad[file]
	a.mu.Unlock()
	if ok || bad {
		return b
	}
	a.mu.Lock()
	a.files[file] = nil // loading
	a.mu.Unlock()
	music.Load(file, func(b []byte) {
		a.mu.Lock()
		if b == nil {
			a.bad[file] = true
			delete(a.files, file)
		} else {
			a.files[file] = b
		}
		a.mu.Unlock()
	})
	return nil
}

// menuLoop: the main-menu track's loop points (seconds), from the web game.
var menuLoop = [2]float64{93.5, 176.492}

// SetMusic chooses what should play: "" (silence), "menu", a location key "biome:li" (uses the data
// track when one exists, else the procedural loop for that biome). boss adds drive to the procedural loop.
func (a *Audio) SetMusic(db *content.DB, key, biome string, boss bool) {
	if a.ctx == nil {
		return
	}
	if a.cv("vol") == 0 || a.cv("musv") == 0 {
		key = ""
	}
	if key != a.want {
		a.want = key
		if a.cur != nil {
			a.fadeOut(a.cur)
			a.cur = nil
		}
	}
	if a.cur == nil && key != "" {
		a.cur = a.start(db, key, biome)
	}
	if a.cur != nil && a.cur.proc != nil {
		a.cur.proc.SetBoss(boss)
	}
}

func (a *Audio) fadeOut(m *musicPlayer) {
	if a.old != nil {
		a.old.p.Close()
	}
	m.in = false
	a.old = m
}

func (a *Audio) start(db *content.DB, key, biome string) *musicPlayer {
	var file string
	var loop [2]int64
	switch {
	case key == "menu":
		file = "menu.mp3"
		loop = [2]int64{int64(menuLoop[0] * synth.SampleRate), int64(menuLoop[1] * synth.SampleRate)}
	default:
		if t := db.World.Music[key]; t != nil {
			file, loop = t.File, [2]int64{t.LoopStartSample, t.LoopEndSample}
		}
	}
	if file != "" {
		b := a.load(file)
		if b == nil {
			if a.bad[file] {
				file = ""
			} else {
				return nil // still loading; try again next frame
			}
		} else {
			st, err := mp3.DecodeWithSampleRate(synth.SampleRate, bytes.NewReader(b))
			if err != nil {
				a.bad[file] = true
				file = ""
			} else {
				const bps = 4
				ls, le := loop[0]*bps, loop[1]*bps
				if le > st.Length() || le <= ls {
					ls, le = 0, st.Length()
				}
				p, err := a.ctx.NewPlayer(audio.NewInfiniteLoopWithIntro(st, ls, le-ls))
				if err != nil {
					file = ""
				} else {
					p.SetBufferSize(200 * time.Millisecond)
					p.SetVolume(0)
					p.Play()
					return &musicPlayer{key: key, p: p, in: true}
				}
			}
		}
	}
	if key == "menu" {
		return nil
	}
	pm := synth.NewProcMusic(biome)
	p, err := a.ctx.NewPlayer(pm)
	if err != nil {
		return nil
	}
	p.SetBufferSize(200 * time.Millisecond)
	p.SetVolume(0)
	p.Play()
	return &musicPlayer{key: key, p: p, in: true, proc: pm}
}

// Update advances fades; call once per frame.
func (a *Audio) Update(dt float64) {
	a.now += dt
	mv := a.cv("musv") * a.cv("vol") * 1.2
	if a.cur != nil {
		a.cur.gain = math.Min(1, a.cur.gain+dt/.8)
		a.cur.p.SetVolume(math.Min(1, a.cur.gain*mv))
	}
	if a.old != nil {
		a.old.gain -= dt / .8
		if a.old.gain <= 0 {
			a.old.p.Close()
			a.old = nil
		} else {
			a.old.p.SetVolume(math.Min(1, a.old.gain*mv))
		}
	}
}
