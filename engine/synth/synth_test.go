package synth_test

import (
	"bytes"
	"math"
	"os"
	"testing"

	"github.com/hajimehoshi/go-mp3"

	"github.com/TxHandles/pokemon-survivor/data"
	"github.com/TxHandles/pokemon-survivor/engine/content"
	"github.com/TxHandles/pokemon-survivor/engine/synth"
)

func peak(b []byte) (float64, float64) {
	mx, sum := 0.0, 0.0
	n := len(b) / 4
	for i := 0; i < n; i++ {
		x := float64(int16(uint16(b[i*4])|uint16(b[i*4+1])<<8)) / 32768
		mx = math.Max(mx, math.Abs(x))
		sum += x * x
	}
	return mx, math.Sqrt(sum / float64(max(n, 1)))
}

func TestSFX(t *testing.T) {
	for name := range synth.SFXNames {
		b := synth.BuildSFX(name, 3)
		if len(b) < 4*400 {
			t.Errorf("%s: too short (%d bytes)", name, len(b))
			continue
		}
		mx, rms := peak(b)
		if mx == 0 || math.IsNaN(rms) || mx > 1 {
			t.Errorf("%s: peak %.2f rms %.3f", name, mx, rms)
		}
		t.Logf("%-10s %.2fs peak %.2f rms %.3f", name, float64(len(b))/4/synth.SampleRate, mx, rms)
	}
}

func TestProcMusic(t *testing.T) {
	for _, boss := range []bool{false, true} {
		m := synth.NewProcMusic("fire")
		m.SetBoss(boss)
		b := make([]byte, 4*synth.SampleRate*4) // 4 seconds
		n, _ := m.Read(b)
		mx, rms := peak(b[:n])
		if mx == 0 || mx > 1 || math.IsNaN(rms) {
			t.Errorf("boss=%v: peak %.2f rms %.3f", boss, mx, rms)
		}
		t.Logf("proc music boss=%v: peak %.2f rms %.3f, %d voices live", boss, mx, rms, m.Voices())
	}
}

// Every data track decodes and its loop points fit inside it.
func TestTracks(t *testing.T) {
	db, _ := content.Load(data.FS)
	for key, tr := range db.World.Music {
		b, err := os.ReadFile("../../music/" + tr.File)
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		st, err := mp3.NewDecoder(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%s: %v", key, err)
			continue
		}
		if st.SampleRate() != synth.SampleRate {
			t.Errorf("%s: %d Hz", key, st.SampleRate())
		}
		if tr.LoopEndSample*4 > st.Length() || tr.LoopStartSample >= tr.LoopEndSample {
			t.Errorf("%s: loop %d-%d outside %d samples", key, tr.LoopStartSample, tr.LoopEndSample, st.Length()/4)
		}
		t.Logf("%s %s: %.1fs, loop %.3f-%.3f", key, tr.File, float64(st.Length())/4/synth.SampleRate, float64(tr.LoopStartSample)/synth.SampleRate, float64(tr.LoopEndSample)/synth.SampleRate)
	}
	b, _ := os.ReadFile("../../music/menu.mp3")
	st, err := mp3.NewDecoder(bytes.NewReader(b))
	if err != nil || float64(st.Length())/4/synth.SampleRate < 176.492 {
		t.Errorf("menu track: %v, %d", err, st.Length())
	}
}
