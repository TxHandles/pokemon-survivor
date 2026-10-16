package client

import (
	"bytes"
	"image"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/TxHandles/pokemon-survivor/engine/content"
)

var (
	faceSrcR, faceSrcB *text.GoTextFaceSource
	fontCache          = map[[2]int]*text.GoTextFace{}
)

func init() {
	var err error
	if faceSrcR, err = text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF)); err != nil {
		panic(err)
	}
	if faceSrcB, err = text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF)); err != nil {
		panic(err)
	}
}

// font returns a cached face of the given pixel size (bold or regular).
func font(size int, bold bool) *text.GoTextFace {
	k := [2]int{size, 0}
	if bold {
		k[1] = 1
	}
	if f := fontCache[k]; f != nil {
		return f
	}
	src := faceSrcR
	if bold {
		src = faceSrcB
	}
	f := &text.GoTextFace{Source: src, Size: float64(size)}
	fontCache[k] = f
	return f
}

// Sheet is one view (front or back) of a Pokémon's animation strip.
type Sheet struct {
	V      *content.SpriteView
	Img    *ebiten.Image
	frames []*ebiten.Image
	tot    int
	atot   int
}

func (s *Sheet) Frame(i int) *ebiten.Image {
	if i < 0 || i >= len(s.frames) {
		i = 0
	}
	return s.frames[i]
}

func frameAt(list, durs []int, tot int, t int) int {
	if tot <= 0 || len(list) == 0 {
		return 0
	}
	t = ((t % tot) + tot) % tot
	for i := range list {
		t -= durs[i]
		if t < 0 {
			return list[i]
		}
	}
	return list[len(list)-1]
}

// IdleFrame / ActFrame pick the frame for time ms.
func (s *Sheet) IdleFrame(ms int) int { return frameAt(s.V.Idle, s.V.Idur, s.tot, ms) }
func (s *Sheet) ActFrame(ms int) int  { return frameAt(s.V.Act, s.V.Adur, s.atot, ms) }

type SpriteSet struct {
	Scale       float64
	Front, Back *Sheet
}

// Assets loads sprites on demand from the embedded data FS.
type Assets struct {
	fs  fs.FS
	db  *content.DB
	set map[string]*SpriteSet
}

func NewAssets(f fs.FS, db *content.DB) *Assets {
	return &Assets{fs: f, db: db, set: map[string]*SpriteSet{}}
}

func (a *Assets) loadSheet(v *content.SpriteView) *Sheet {
	if v == nil {
		return nil
	}
	b, err := fs.ReadFile(a.fs, "sprites/"+v.File)
	if err != nil {
		return nil
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	img := ebiten.NewImageFromImage(im)
	s := &Sheet{V: v, Img: img}
	n := img.Bounds().Dx() / v.CW
	for i := 0; i < n; i++ {
		s.frames = append(s.frames, img.SubImage(image.Rect(i*v.CW, 0, (i+1)*v.CW, v.CH)).(*ebiten.Image))
	}
	for _, d := range v.Idur {
		s.tot += d
	}
	for _, d := range v.Adur {
		s.atot += d
	}
	return s
}

// Sprite returns the sprite set for a Pokémon id (nil if it has none).
func (a *Assets) Sprite(id string) *SpriteSet {
	if s, ok := a.set[id]; ok {
		return s
	}
	var s *SpriteSet
	if d := a.db.Sprites[id]; d != nil {
		s = &SpriteSet{Scale: d.Scale, Front: a.loadSheet(d.Front), Back: a.loadSheet(d.Back)}
		if s.Front == nil && s.Back == nil {
			s = nil
		} else if s.Front == nil {
			s.Front = s.Back
		} else if s.Back == nil {
			s.Back = s.Front
		}
	}
	a.set[id] = s
	return s
}
