package client

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	_ "image/png"
	"math"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
	"github.com/hajimehoshi/ebiten/v2"
)

// PREVIEW: pixel-art sprites (a new art style being tried on Charmander). Each Pokémon in
// pxart/px.json has front, back, left and right views with idle, walk and attack frames, drawn
// with nearest-neighbour scaling so the pixels stay crisp.

//go:embed pxart
var pxFS embed.FS

type pxView struct {
	File       string `json:"file"`
	CW         int    `json:"cw"`
	CH         int    `json:"ch"`
	Idle, Idur []int
	Walk, Wdur []int
	Act, Adur  []int
	frames     []*ebiten.Image
}

type pxSprite struct {
	Scale float64            `json:"scale"`
	Views map[string]*pxView `json:"views"`
}

var pxSprites = func() map[string]*pxSprite {
	m := map[string]*pxSprite{}
	b, err := pxFS.ReadFile("pxart/px.json")
	if err != nil || json.Unmarshal(b, &m) != nil {
		return map[string]*pxSprite{}
	}
	for _, s := range m {
		for _, v := range s.Views {
			raw, err := pxFS.ReadFile("pxart/" + v.File)
			if err != nil {
				continue
			}
			im, _, err := image.Decode(bytes.NewReader(raw))
			if err != nil {
				continue
			}
			img := ebiten.NewImageFromImage(im)
			for i := 0; i < im.Bounds().Dx()/v.CW; i++ {
				v.frames = append(v.frames, img.SubImage(image.Rect(i*v.CW, 0, (i+1)*v.CW, v.CH)).(*ebiten.Image))
			}
		}
	}
	return m
}()

// pxPick: the frame of list (durations durs, ms) at time t (ms), looping.
func pxPick(list, durs []int, t int) int {
	tot := 0
	for _, d := range durs {
		tot += d
	}
	return frameAt(list, durs, tot, t)
}

func (v *pxView) actMs() int {
	t := 0
	for _, d := range v.Adur {
		t += d
	}
	return t
}

// pxDraw draws frame fr of view v with its feet at (x, y).
func pxDraw(dst *ebiten.Image, v *pxView, fr int, x, y, sc, a float64, flash bool) {
	if fr < 0 || fr >= len(v.frames) {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(v.CW)/2, -float64(v.CH))
	op.GeoM.Scale(sc, sc)
	op.GeoM.Translate(math.Round(x), math.Round(y))
	op.ColorScale.ScaleAlpha(float32(a))
	op.Filter = ebiten.FilterNearest
	dst.DrawImage(v.frames[fr], op)
	if flash {
		op.ColorScale.Reset()
		op.ColorScale.Scale(1, .25, .25, float32(.75*a))
		op.Blend = ebiten.BlendLighter
		dst.DrawImage(v.frames[fr], op)
	}
}

// drawPxPlayer: a player whose Pokémon has pixel-art sprites. True if it drew.
func (g *Game) drawPxPlayer(dst *ebiten.Image, a *playerAnim, P *sim.Player, x, y float64) bool {
	ps := pxSprites[P.ID]
	if ps == nil {
		return false
	}
	w := g.W
	if P.Moving { // face where we're heading: sideways, away or towards the camera
		switch {
		case math.Abs(P.FX) > math.Abs(P.FY)*.8:
			a.pxView = map[bool]string{true: "left", false: "right"}[P.FX < 0]
		case P.FY < 0:
			a.pxView = "back"
		default:
			a.pxView = "front"
		}
	}
	if a.pxView == "" {
		a.pxView = "front"
	}
	v := ps.Views[a.pxView]
	if v == nil {
		return false
	}
	ms := int(w.T * 1000)
	var fr int
	switch at := w.T - P.AtkT; {
	case at >= 0 && at*1000 < float64(v.actMs()):
		fr = pxPick(v.Act, v.Adur, int(at*1000))
	case P.Moving:
		fr = pxPick(v.Walk, v.Wdur, ms)
	default:
		fr = pxPick(v.Idle, v.Idur, ms)
	}
	alphaV := 1.0
	if P.Inv > 0 && int(w.T*20)%2 == 1 && P.Dash == nil {
		alphaV = .45
	}
	pxDraw(dst, v, fr, x, y+15, ps.Scale, alphaV, w.T-P.HurtT < .16)
	return true
}

// drawPxEnemy: an enemy with pixel-art sprites walks in profile towards you.
func (g *Game) drawPxEnemy(dst *ebiten.Image, e *sim.Enemy, x, y float64) bool {
	ps := pxSprites[e.PokeID]
	if ps == nil {
		return false
	}
	v := ps.Views[map[bool]string{true: "left", false: "right"}[e.FaceL]]
	if v == nil {
		return false
	}
	ms := int(g.W.T*1000 + e.Seed*5000)
	fr := pxPick(v.Walk, v.Wdur, ms)
	sc := ps.Scale * e.R / 12 * .9
	pxDraw(dst, v, fr, x, y+e.R*.95, sc, 1, e.Flash > 0 && g.opt("flash") > 0)
	return true
}
