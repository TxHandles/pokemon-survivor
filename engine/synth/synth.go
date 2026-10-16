// Package synth reproduces the web game's procedural sounds (tone / noise / arp) and its placeholder
// music. Everything renders at 44.1 kHz; one-shots render to a buffer, music streams.
package synth

import (
	"math"
	"math/rand"
	"sync"
)

// SampleRate of everything the synth renders.
const SampleRate = 44100

type voice struct {
	wave       string // sine square triangle sawtooth noise
	f0, f1     float64
	t0, a, dur float64 // start, attack, total length (seconds)
	v          float64
	// noise filter
	ftype string
	fq, q float64
	// state
	phase    float64
	bq       biquad
	seed     uint32
	started  bool
	lastFreq float64
}

type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

func (b *biquad) set(kind string, f, q float64) {
	if q <= 0 {
		q = 1
	}
	w0 := 2 * math.Pi * f / SampleRate
	cw, sw := math.Cos(w0), math.Sin(w0)
	al := sw / (2 * q)
	var b0, b1, b2, a0, a1, a2 float64
	switch kind {
	case "lowpass":
		b0, b1, b2 = (1-cw)/2, 1-cw, (1-cw)/2
	case "bandpass":
		b0, b1, b2 = al, 0, -al
	default: // highpass
		b0, b1, b2 = (1+cw)/2, -(1 + cw), (1+cw)/2
	}
	a0, a1, a2 = 1+al, -2*cw, 1-al
	b.b0, b.b1, b.b2, b.a1, b.a2 = b0/a0, b1/a0, b2/a0, a1/a0, a2/a0
}

func (b *biquad) run(x float64) float64 {
	y := b.b0*x + b.b1*b.x1 + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1 = b.x1, x
	b.y2, b.y1 = b.y1, y
	return y
}

func (v *voice) env(t float64) float64 {
	const floor = .0001
	vv := math.Max(.0002, v.v)
	if t < v.a {
		return floor * math.Pow(vv/floor, t/v.a)
	}
	if v.dur <= v.a {
		return floor
	}
	return vv * math.Pow(floor/vv, (t-v.a)/(v.dur-v.a))
}

// sample renders one mono sample at time t (seconds since the voice started).
func (v *voice) sample(t float64) float64 {
	if !v.started {
		v.started = true
		if v.wave == "noise" {
			v.bq.set(v.ftype, v.fq, v.q)
			if v.seed == 0 {
				v.seed = rand.Uint32() | 1
			}
		}
	}
	g := v.env(t)
	if v.wave == "noise" {
		v.seed ^= v.seed << 13
		v.seed ^= v.seed >> 17
		v.seed ^= v.seed << 5
		n := float64(v.seed)/2147483648 - 1
		return v.bq.run(n) * g
	}
	f := v.f0
	if v.f1 > 0 && v.f1 != v.f0 && v.dur > 0 {
		f = v.f0 * math.Pow(v.f1/v.f0, math.Min(1, t/v.dur))
	}
	v.phase += f / SampleRate
	if v.phase >= 1 {
		v.phase -= math.Floor(v.phase)
	}
	p := v.phase
	var s float64
	switch v.wave {
	case "square":
		if p < .5 {
			s = 1
		} else {
			s = -1
		}
		s *= .5 // squares are loud
	case "triangle":
		s = 4*math.Abs(p-.5) - 1
	case "sawtooth":
		s = (2*p - 1) * .6
	default:
		s = math.Sin(2 * math.Pi * p)
	}
	return s * g
}

// ---- one-shot sound effects (web SYNTH) ----

type sfxBuilder struct {
	vs []*voice
}

func (b *sfxBuilder) tone(wave string, f0, f1, dur, v, t, a float64) {
	if a == 0 {
		a = .005
	}
	b.vs = append(b.vs, &voice{wave: wave, f0: f0, f1: f1, dur: dur, v: v, t0: t, a: a})
}

func (b *sfxBuilder) noise(dur, v, t float64, ft string, fq, q float64) {
	if ft == "" {
		ft = "highpass"
	}
	if fq == 0 {
		fq = 1000
	}
	b.vs = append(b.vs, &voice{wave: "noise", dur: dur, v: v, t0: t, a: .003, ftype: ft, fq: fq, q: q})
}

func note(n float64) float64 { return 440 * math.Pow(2, (n-69)/12) }

func (b *sfxBuilder) arp(ns []float64, t, st float64, wave string, v, dur float64) {
	if wave == "" {
		wave = "triangle"
	}
	if dur == 0 {
		dur = .18
	}
	for i, n := range ns {
		b.tone(wave, note(n), 0, dur, v, t+float64(i)*st, 0)
	}
}

// render mixes the voices into 16-bit stereo PCM.
func (b *sfxBuilder) render() []byte {
	end := 0.0
	for _, v := range b.vs {
		end = math.Max(end, v.t0+v.dur+.03)
	}
	n := int(end * SampleRate)
	out := make([]byte, n*4)
	for i := 0; i < n; i++ {
		t := float64(i) / SampleRate
		s := 0.0
		for _, v := range b.vs {
			if t >= v.t0 && t < v.t0+v.dur+.02 {
				s += v.sample(t - v.t0)
			}
		}
		s = math.Tanh(s * 1.4)
		x := int16(s * 32000)
		out[i*4], out[i*4+1] = byte(x), byte(x>>8)
		out[i*4+2], out[i*4+3] = byte(x), byte(x>>8)
	}
	return out
}

var rnd = rand.Float64

// buildSFX renders the named effect (web SYNTH); xpStep is the rising pitch of XP pickup streaks.
func BuildSFX(name string, xpStep int) []byte {
	b := &sfxBuilder{}
	v := 1.0
	t := 0.0
	switch name {
	case "hit":
		b.noise(.04, v, t, "bandpass", 2200+rnd()*900, 1.2)
	case "kill":
		b.tone("sine", 420+rnd()*80, 110, .08, v, t, 0)
		b.noise(.05, v*.4, t, "highpass", 3000, 0)
	case "elitedown":
		b.tone("square", 300, 80, .25, v*.5, t, 0)
		b.noise(.3, v*.6, t, "lowpass", 900, 0)
		b.arp([]float64{72, 76, 79}, t+.08, .06, "triangle", v*.5, .15)
	case "bossdown":
		b.noise(1.2, v, t, "lowpass", 500, 0)
		b.tone("sawtooth", 90, 30, 1.1, v*.6, t, 0)
		b.arp([]float64{60, 64, 67, 72, 76, 79, 84}, t+.35, .08, "triangle", v*.45, .3)
	case "xp":
		b.tone("sine", note(76+float64(xpStep)*.5), 0, .06, v, t, 0)
	case "coin":
		b.tone("square", note(88), 0, .05, v*.5, t, 0)
		b.tone("square", note(93), 0, .09, v*.5, t+.05, 0)
	case "ticket":
		b.arp([]float64{81, 85, 88, 93}, t, .05, "square", v*.35, .1)
	case "levelup":
		b.arp([]float64{60, 64, 67, 72}, t, .07, "triangle", v, .22)
		b.arp([]float64{72, 76, 79, 84}, t+.28, .05, "sine", v*.6, .25)
	case "pickup":
		b.arp([]float64{67, 74}, t, .07, "triangle", v, .14)
	case "heal":
		b.tone("sine", note(64), note(76), .35, v, t, .05)
		b.tone("sine", note(71), note(83), .35, v*.5, t+.06, .05)
	case "chest":
		b.arp([]float64{72, 76, 79, 84, 88}, t, .06, "triangle", v, .2)
		b.noise(.4, v*.25, t+.1, "highpass", 6000, 0)
	case "hurt":
		b.tone("square", 190, 70, .14, v*.6, t, 0)
		b.noise(.1, v*.5, t, "lowpass", 1200, 0)
	case "elite":
		b.tone("square", note(69), 0, .12, v*.5, t, 0)
		b.tone("square", note(69), 0, .12, v*.5, t+.18, 0)
	case "boss":
		b.tone("sawtooth", 55, 52, 1.6, v*.6, t, .25)
		b.tone("sawtooth", 82, 78, 1.6, v*.4, t, .3)
		b.noise(.5, v*.8, t, "lowpass", 180, 0)
		b.noise(.5, v*.7, t+.8, "lowpass", 180, 0)
	case "phase":
		b.tone("sawtooth", 220, 55, .7, v*.6, t, 0)
		b.noise(.4, v*.6, t, "lowpass", 400, 0)
	case "evolve":
		b.arp([]float64{60, 67, 72, 76, 79, 84, 88, 91}, t, .05, "sine", v*.7, .35)
		b.noise(.8, v*.2, t, "highpass", 7000, 0)
	case "death":
		b.tone("triangle", note(67), note(43), 1.1, v, t, .02)
		b.tone("triangle", note(62), note(38), 1.1, v*.6, t+.1, .02)
	case "victory":
		b.arp([]float64{60, 64, 67, 72}, t, .12, "square", v*.45, .25)
		b.arp([]float64{67, 72, 76, 79, 84}, t+.55, .09, "triangle", v*.6, .4)
	case "click":
		b.tone("triangle", note(84), note(79), .04, v, t, 0)
	case "dash":
		b.noise(.22, v*.7, t, "bandpass", 900, .7)
		b.tone("sine", 320, 140, .18, v*.3, t, .01)
	default:
		return nil
	}
	return b.render()
}

// ---- streaming placeholder music (web musTick): per-biome key, 16-step loop, more drive during a boss ----

var musicKey = map[string][2]int{"normal": {57, 0}, "fire": {52, 1}, "water": {55, 0}, "grass": {60, 0}, "electric": {59, 1}, "ground": {50, 1},
	"ice": {62, 0}, "fighting": {53, 1}, "poison": {54, 1}, "flying": {64, 0}, "psychic": {58, 0}, "bug": {56, 1}, "rock": {48, 1},
	"ghost": {51, 1}, "dragon": {49, 1}, "dark": {47, 1}, "steel": {45, 1}, "fairy": {65, 0}}

var progMinor = [4][3]int{{0, 3, 7}, {-4, 0, 3}, {3, 7, 10}, {-2, 2, 5}}
var progMajor = [4][3]int{{0, 4, 7}, {-3, 0, 4}, {5, 9, 12}, {7, 11, 14}}

// procMusic is an io.Reader that synthesises the loop as it is read.
type ProcMusic struct {
	mu     sync.Mutex
	root   int
	minor  bool
	boss   bool
	vs     []*voice
	step   int
	nt     float64 // next step time
	now    float64
	rng    *rand.Rand
	nextSt float64
}

func NewProcMusic(biome string) *ProcMusic {
	k, ok := musicKey[biome]
	if !ok {
		k = [2]int{57, 1}
	}
	return &ProcMusic{root: k[0], minor: k[1] == 1, rng: rand.New(rand.NewSource(1)), nt: .05}
}

func (m *ProcMusic) SetBoss(b bool) {
	m.mu.Lock()
	m.boss = b
	m.mu.Unlock()
}

func (m *ProcMusic) tone(wave string, f0, f1, dur, v, t, a float64) {
	if a == 0 {
		a = .005
	}
	m.vs = append(m.vs, &voice{wave: wave, f0: f0, f1: f1, dur: dur, v: v, t0: t, a: a})
}

func (m *ProcMusic) noise(dur, v, t float64, ft string, fq float64) {
	m.vs = append(m.vs, &voice{wave: "noise", dur: dur, v: v, t0: t, a: .003, ftype: ft, fq: fq, seed: m.rng.Uint32() | 1})
}

func (m *ProcMusic) schedule() {
	bpm := 104.0
	if m.boss {
		bpm = 126
	}
	st := 60 / bpm / 4
	pr := progMajor
	if m.minor {
		pr = progMinor
	}
	for m.nt < m.now+.2 {
		s := m.step % 16
		bar := (m.step / 16) % 4
		ch := pr[bar]
		t := m.nt
		root := float64(m.root)
		if s == 0 {
			for _, n := range ch {
				m.tone("triangle", note(root+float64(n)), 0, st*15, .035, t, .25)
			}
		}
		if s%4 == 0 || s == 6 || s == 14 {
			i := 0
			if s == 6 || s == 14 {
				i = 2
			}
			m.tone("sine", note(root-12+float64(ch[i])), 0, st*1.6, .16, t, 0)
		}
		if s%2 == 1 {
			m.noise(.03, b2f(m.boss, .05, .025), t, "highpass", 8000)
		}
		if m.boss && s%4 == 0 {
			m.tone("sine", 120, 40, .12, .25, t, 0)
		}
		if (s == 2 || s == 10 || (m.boss && s%4 == 2)) && m.rng.Float64() < .8 {
			n := float64(ch[(m.step>>1)%3] + 12)
			m.tone("square", note(root+n), 0, st*1.2, .02, t, 0)
		}
		m.step++
		m.nt += st
	}
}

func (m *ProcMusic) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(p) / 4
	for i := 0; i < n; i++ {
		if i%256 == 0 {
			m.schedule()
		}
		s := 0.0
		keep := m.vs[:0]
		for _, v := range m.vs {
			if m.now >= v.t0+v.dur+.02 {
				continue
			}
			keep = append(keep, v)
			if m.now >= v.t0 {
				s += v.sample(m.now - v.t0)
			}
		}
		m.vs = keep
		s = math.Tanh(s * 2.5)
		x := int16(s * 32000)
		p[i*4], p[i*4+1] = byte(x), byte(x>>8)
		p[i*4+2], p[i*4+3] = byte(x), byte(x>>8)
		m.now += 1.0 / SampleRate
	}
	return n * 4, nil
}

func b2f(b bool, t, f float64) float64 {
	if b {
		return t
	}
	return f
}

// Voices reports how many voices are live (tests).
func (m *ProcMusic) Voices() int { return len(m.vs) }

// SFXNames lists every effect BuildSFX knows.
var SFXNames = map[string]bool{"hit": true, "kill": true, "elitedown": true, "bossdown": true, "xp": true, "coin": true, "ticket": true, "levelup": true,
	"pickup": true, "heal": true, "chest": true, "hurt": true, "elite": true, "boss": true, "phase": true, "evolve": true, "death": true, "victory": true, "click": true, "dash": true}
