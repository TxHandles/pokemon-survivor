package client

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/TxHandles/pokemon-survivor/engine/sim"
)

// Shared menu pieces matching the web version: type icons (data/types, the same images the web game
// uses), small vector glyphs in place of the web's emoji (the game font has none), portraits,
// type chips, matchup rows and a search box.

var (
	webPanel  = hexC("#1b2030")
	webBorder = hexC("#3a4260")
	webBg     = hexC("#10131a")
	webFg     = hexC("#eef1f7")
	webAcc    = hexC("#ffd04a")
	webInk    = hexC("#0b0e16")
	webGreen  = hexC("#5be37d")
	webCyan   = hexC("#6fe3ff")
)

// ---- type icons ----

var typeIcons = map[string]*ebiten.Image{}

func (g *Game) typeIcon(t string) *ebiten.Image {
	if im, ok := typeIcons[t]; ok {
		return im
	}
	var im *ebiten.Image
	if b, err := fs.ReadFile(g.assets.fs, "types/"+t+".png"); err == nil {
		if src, _, err := image.Decode(bytes.NewReader(b)); err == nil {
			im = ebiten.NewImageFromImage(src)
		}
	}
	typeIcons[t] = im
	return im
}

// drawTypeIcon draws type t's icon centred at (x, y) with the given size; dim greys it out.
func (g *Game) drawTypeIcon(dst *ebiten.Image, t string, x, y, size float64, dim bool) {
	im := g.typeIcon(t)
	if im == nil {
		disc(dst, x, y, size/2, tc(t))
		return
	}
	w := float64(im.Bounds().Dx())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(size/w, size/w)
	op.GeoM.Translate(x-size/2, y-size/2)
	op.Filter = ebiten.FilterLinear
	if dim {
		op.ColorScale.Scale(.45, .45, .45, 1)
	}
	dst.DrawImage(im, op)
}

// ---- text helpers ----

func textW(s string, size int, bold bool) float64 {
	w, _ := text.Measure(s, font(size, bold), 0)
	return w
}

// fitText shortens s with an ellipsis to fit width w.
func fitText(s string, size int, bold bool, w float64) string {
	if textW(s, size, bold) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && textW(string(r)+"…", size, bold) > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func cap1s(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---- glyphs (vector stand-ins for the web version's emoji) ----

type glyph int

const (
	gStar glyph = iota
	gCheck
	gLock
	gTicket
	gSwords
	gShield
	gHand
	gArrow
	gUp
	gSearch
	gTrophy
	gSparkle
	gSwap
	gCoin
	gPlay
	gGear
	gCart
	gBook
	gBars
	gChart
	gCalendar
	gMedal
	gPad
	gGlobe
	gGift
	gPin
	gDice
	gUpload
	gDownload
	gTriUp
	gTriDown
)

// drawGlyph draws a glyph centred at (x, y) fitting a box of the given size.
func drawGlyph(dst *ebiten.Image, gl glyph, x, y, s float64, col color.NRGBA) {
	h := s / 2
	switch gl {
	case gStar, gSparkle:
		n, inner := 5, .45
		if gl == gSparkle {
			n, inner = 4, .3
		}
		var pts [][2]float64
		for i := 0; i < n*2; i++ {
			a := -math.Pi/2 + float64(i)*math.Pi/float64(n)
			r := h
			if i%2 == 1 {
				r = h * inner
			}
			pts = append(pts, [2]float64{x + math.Cos(a)*r, y + math.Sin(a)*r})
		}
		poly(dst, pts, col)
	case gCheck:
		w := math.Max(1.6, s*.16)
		line(dst, x-h*.8, y, x-h*.2, y+h*.6, w, col)
		line(dst, x-h*.2, y+h*.6, x+h*.85, y-h*.65, w, col)
	case gLock:
		bw, bh := s*.72, s*.5
		roundRect(dst, x-bw/2, y-bh/2+s*.18, bw, bh, s*.08, col)
		var p vector.Path
		p.Arc(float32(x), float32(y-s*.08), float32(s*.24), math.Pi, 0, vector.Clockwise)
		strokePath(dst, &p, math.Max(1.4, s*.12), col)
		line(dst, x-s*.24, y-s*.08, x-s*.24, y+s*.1, math.Max(1.4, s*.12), col)
		line(dst, x+s*.24, y-s*.08, x+s*.24, y+s*.1, math.Max(1.4, s*.12), col)
	case gTicket:
		red := c(232, 72, 85, 255)
		if col.A < 200 { // dimmed (disabled)
			red = alpha(red, .5)
		}
		w, hh := s, s*.62
		roundRect(dst, x-w/2, y-hh/2, w, hh, s*.1, red)
		disc(dst, x-w/2, y, s*.14, webBg)
		disc(dst, x+w/2, y, s*.14, webBg)
		for i := -1; i <= 1; i++ {
			rect(dst, x+w*.18, y+float64(i)*hh*.28-0.6, 1.2, 1.4, alpha(white, .8))
		}
	case gSwords:
		w := math.Max(1.4, s*.12)
		line(dst, x-h*.8, y-h*.8, x+h*.8, y+h*.8, w, col)
		line(dst, x+h*.8, y-h*.8, x-h*.8, y+h*.8, w, col)
		line(dst, x-h*.85, y+h*.35, x-h*.35, y+h*.85, w, col)
		line(dst, x+h*.85, y+h*.35, x+h*.35, y+h*.85, w, col)
	case gShield:
		var p vector.Path
		p.MoveTo(float32(x), float32(y-h))
		p.LineTo(float32(x+h*.85), float32(y-h*.62))
		p.QuadTo(float32(x+h*.8), float32(y+h*.5), float32(x), float32(y+h))
		p.QuadTo(float32(x-h*.8), float32(y+h*.5), float32(x-h*.85), float32(y-h*.62))
		p.Close()
		fillPath(dst, &p, col)
	case gHand:
		roundRect(dst, x-h*.62, y-h*.05, h*1.24, h*1.05, h*.3, col)
		for i := 0; i < 4; i++ {
			fx := x - h*.62 + float64(i)*h*.31 + h*.02
			roundRect(dst, fx, y-h*.8+math.Abs(float64(i)-1.5)*h*.12, h*.26, h*.9, h*.12, col)
		}
		roundRect(dst, x-h*.95, y-h*.05, h*.4, h*.26, h*.12, col)
	case gArrow:
		w := math.Max(1.4, s*.12)
		line(dst, x-h, y, x+h*.7, y, w, col)
		poly(dst, [][2]float64{{x + h, y}, {x + h*.35, y - h*.45}, {x + h*.35, y + h*.45}}, col)
	case gUp:
		poly(dst, [][2]float64{{x, y - h}, {x + h*.85, y - h*.05}, {x + h*.35, y - h*.05}, {x + h*.35, y + h*.8}, {x - h*.35, y + h*.8}, {x - h*.35, y - h*.05}, {x - h*.85, y - h*.05}}, col)
	case gSearch:
		ring(dst, x-h*.18, y-h*.18, h*.52, math.Max(1.4, s*.11), col)
		line(dst, x+h*.22, y+h*.22, x+h*.85, y+h*.85, math.Max(1.6, s*.14), col)
	case gTrophy:
		var p vector.Path
		p.MoveTo(float32(x-h*.6), float32(y-h*.8))
		p.LineTo(float32(x+h*.6), float32(y-h*.8))
		p.QuadTo(float32(x+h*.6), float32(y+h*.2), float32(x), float32(y+h*.25))
		p.QuadTo(float32(x-h*.6), float32(y+h*.2), float32(x-h*.6), float32(y-h*.8))
		p.Close()
		fillPath(dst, &p, col)
		rect(dst, x-h*.12, y+h*.2, h*.24, h*.4, col)
		roundRect(dst, x-h*.45, y+h*.55, h*.9, h*.3, h*.08, col)
		ring(dst, x-h*.62, y-h*.45, h*.25, math.Max(1.2, s*.08), col)
		ring(dst, x+h*.62, y-h*.45, h*.25, math.Max(1.2, s*.08), col)
	case gCoin:
		disc(dst, x, y, h, hexC("#c9952b"))
		disc(dst, x, y, h*.82, hexC("#f4c542"))
		ring(dst, x, y, h*.55, math.Max(1, s*.07), hexC("#c9952b"))
	case gPlay:
		poly(dst, [][2]float64{{x - h*.6, y - h*.8}, {x + h*.85, y}, {x - h*.6, y + h*.8}}, col)
	case gGear:
		for i := 0; i < 8; i++ {
			a := float64(i) * math.Pi / 4
			orect(dst, x, y, a, h*.95, h*.17, col)
		}
		disc(dst, x, y, h*.68, col)
		disc(dst, x, y, h*.28, webPanel)
	case gCart:
		w := math.Max(1.3, s*.1)
		line(dst, x-h, y-h*.75, x-h*.65, y-h*.75, w, col)
		poly(dst, [][2]float64{{x - h*.65, y - h*.55}, {x + h*.95, y - h*.55}, {x + h*.7, y + h*.3}, {x - h*.45, y + h*.3}}, col)
		disc(dst, x-h*.3, y+h*.72, h*.2, col)
		disc(dst, x+h*.5, y+h*.72, h*.2, col)
	case gBook:
		poly(dst, [][2]float64{{x - h, y - h*.7}, {x - h*.05, y - h*.5}, {x - h*.05, y + h*.85}, {x - h, y + h*.65}}, col)
		poly(dst, [][2]float64{{x + h, y - h*.7}, {x + h*.05, y - h*.5}, {x + h*.05, y + h*.85}, {x + h, y + h*.65}}, col)
	case gBars:
		for i, v := range []float64{.55, 1, .75} {
			bw := h * .5
			bx := x - h + float64(i)*h*.75
			rect(dst, bx, y+h-v*h*1.8, bw, v*h*1.8, col)
		}
	case gChart:
		w := math.Max(1.4, s*.12)
		line(dst, x-h, y-h, x-h, y+h, w*.8, col)
		line(dst, x-h, y+h, x+h, y+h, w*.8, col)
		line(dst, x-h*.7, y+h*.4, x-h*.2, y-h*.1, w, col)
		line(dst, x-h*.2, y-h*.1, x+h*.2, y+h*.2, w, col)
		line(dst, x+h*.2, y+h*.2, x+h*.85, y-h*.65, w, col)
	case gCalendar:
		roundRect(dst, x-h*.9, y-h*.7, h*1.8, h*1.6, h*.2, col)
		rect(dst, x-h*.9, y-h*.7, h*1.8, h*.45, c(220, 70, 70, 255))
		for i := 0; i < 3; i++ {
			for j := 0; j < 2; j++ {
				rect(dst, x-h*.6+float64(i)*h*.45, y+float64(j)*h*.42, h*.3, h*.26, webPanel)
			}
		}
	case gMedal:
		poly(dst, [][2]float64{{x - h*.6, y - h}, {x - h*.15, y - h}, {x + h*.2, y - h*.15}, {x - h*.25, y - h*.15}}, c(80, 140, 230, 255))
		poly(dst, [][2]float64{{x + h*.6, y - h}, {x + h*.15, y - h}, {x - h*.2, y - h*.15}, {x + h*.25, y - h*.15}}, c(220, 70, 70, 255))
		disc(dst, x, y+h*.35, h*.62, hexC("#c9952b"))
		disc(dst, x, y+h*.35, h*.48, hexC("#f4c542"))
	case gPad:
		roundRect(dst, x-h, y-h*.55, h*2, h*1.15, h*.5, col)
		rect(dst, x-h*.62, y-h*.1, h*.5, h*.14, webPanel)
		rect(dst, x-h*.44, y-h*.28, h*.14, h*.5, webPanel)
		disc(dst, x+h*.45, y-h*.12, h*.12, webPanel)
		disc(dst, x+h*.65, y+h*.08, h*.12, webPanel)
	case gGlobe:
		w := math.Max(1.2, s*.08)
		ring(dst, x, y, h*.9, w, col)
		line(dst, x-h*.9, y, x+h*.9, y, w, col)
		var p vector.Path
		p.MoveTo(float32(x), float32(y-h*.9))
		p.QuadTo(float32(x+h*.75), float32(y), float32(x), float32(y+h*.9))
		p.QuadTo(float32(x-h*.75), float32(y), float32(x), float32(y-h*.9))
		strokePath(dst, &p, w, col)
	case gGift:
		rect(dst, x-h*.85, y-h*.25, h*1.7, h*1.1, c(220, 70, 70, 255))
		rect(dst, x-h, y-h*.55, h*2, h*.38, c(240, 90, 90, 255))
		rect(dst, x-h*.14, y-h*.55, h*.28, h*1.4, webAcc)
	case gPin:
		disc(dst, x, y-h*.25, h*.6, c(220, 70, 70, 255))
		poly(dst, [][2]float64{{x - h*.45, y}, {x + h*.45, y}, {x, y + h}}, c(220, 70, 70, 255))
		disc(dst, x, y-h*.25, h*.22, white)
	case gDice:
		roundRect(dst, x-h*.85, y-h*.85, h*1.7, h*1.7, h*.3, white)
		for _, d := range [][2]float64{{-.42, -.42}, {.42, .42}, {0, 0}, {.42, -.42}, {-.42, .42}} {
			disc(dst, x+d[0]*h, y+d[1]*h, h*.15, c(200, 40, 40, 255))
		}
	case gUpload, gDownload:
		w := math.Max(1.4, s*.12)
		rect(dst, x-h*.8, y+h*.6, h*1.6, w, col)
		line(dst, x, y-h*.8, x, y+h*.35, w, col)
		if gl == gUpload {
			poly(dst, [][2]float64{{x, y - h}, {x - h*.5, y - h*.35}, {x + h*.5, y - h*.35}}, col)
		} else {
			poly(dst, [][2]float64{{x, y + h*.5}, {x - h*.5, y - h*.1}, {x + h*.5, y - h*.1}}, col)
		}
	case gTriUp:
		poly(dst, [][2]float64{{x, y - h*.7}, {x + h*.8, y + h*.6}, {x - h*.8, y + h*.6}}, col)
	case gTriDown:
		poly(dst, [][2]float64{{x, y + h*.7}, {x + h*.8, y - h*.6}, {x - h*.8, y - h*.6}}, col)
	case gSwap:
		w := math.Max(1.3, s*.1)
		line(dst, x-h, y-h*.35, x+h*.6, y-h*.35, w, col)
		poly(dst, [][2]float64{{x + h, y - h*.35}, {x + h*.5, y - h*.75}, {x + h*.5, y + h*.05}}, col)
		line(dst, x+h, y+h*.35, x-h*.6, y+h*.35, w, col)
		poly(dst, [][2]float64{{x - h, y + h*.35}, {x - h*.5, y - h*.05}, {x - h*.5, y + h*.75}}, col)
	}
}

// uiImg loads an image from data/ui (cached).
var (
	uiImgs = map[string]*ebiten.Image{}
	uiFS   fs.FS // the data FS (set by New)
)

// uiIcon is an inline data/ui image in richLine.
type uiIcon string

func (g *Game) uiImg(name string) *ebiten.Image { return uiImage(name) }

func uiImage(name string) *ebiten.Image {
	if im, ok := uiImgs[name]; ok {
		return im
	}
	var im *ebiten.Image
	if uiFS == nil {
		return nil
	}
	if b, err := fs.ReadFile(uiFS, "ui/"+name); err == nil {
		if src, _, err := image.Decode(bytes.NewReader(b)); err == nil {
			im = ebiten.NewImageFromImage(src)
		}
	}
	uiImgs[name] = im
	return im
}

// drawUIImg draws a data/ui image fitted to height h with its left edge at x, centred on y.
func (g *Game) drawUIImg(dst *ebiten.Image, name string, x, y, h float64) float64 {
	im := g.uiImg(name)
	if im == nil {
		return 0
	}
	k := h / float64(im.Bounds().Dy())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(x, y-h/2)
	if k < 1 {
		op.Filter = ebiten.FilterLinear
	}
	dst.DrawImage(im, op)
	return float64(im.Bounds().Dx()) * k
}

// richLine draws a run of text with inline glyphs: parts are strings or glyphs (each followed by a
// colour change via color.NRGBA). Returns the width.
func richLine(dst *ebiten.Image, x, y float64, size int, bold bool, col color.NRGBA, parts ...any) float64 {
	x0 := x
	cur := col
	b := bold
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			if dst != nil {
				drawText(dst, v, x, y, size, b, cur, text.AlignStart, false)
			}
			x += textW(v, size, b)
		case glyph:
			s := float64(size) * 1.05
			if dst != nil {
				drawGlyph(dst, v, x+s/2, y+float64(size)*.62, s, cur)
			}
			x += s + float64(size)*.3
		case uiIcon:
			im := uiImage(string(v))
			if im == nil {
				continue
			}
			h := float64(size) * 1.2
			w := h * float64(im.Bounds().Dx()) / float64(im.Bounds().Dy())
			if dst != nil {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(w/float64(im.Bounds().Dx()), h/float64(im.Bounds().Dy()))
				op.GeoM.Translate(x, y+float64(size)*.62-h/2)
				if h < float64(im.Bounds().Dy()) {
					op.Filter = ebiten.FilterLinear
				}
				dst.DrawImage(im, op)
			}
			x += w + float64(size)*.35
		case color.NRGBA:
			cur = v
		case bool:
			b = v
		}
	}
	return x - x0
}

// ---- portraits (web .cimg) ----

// drawPortrait draws a Pokémon in a rounded box with its type colour, like the web version's
// portraits: locked ones are dark silhouettes and hide the type badges.
func (g *Game) drawPortrait(dst *ebiten.Image, id string, x, y, size float64, locked, anim, badges bool) {
	p := g.db.ByID[id]
	col := webBorder
	if p != nil {
		col = tc(p.Types[0])
	}
	bw := math.Max(1.5, size*.026)
	roundRect(dst, x, y, size, size, size*.15, col)
	roundRect(dst, x+bw, y+bw, size-2*bw, size-2*bw, size*.13, webInk)
	soft(dst, x+size/2, y+size*.38, size*.5, size*.42, alpha(col, .38))
	if s := g.assets.Sprite(id); s != nil && s.Front != nil {
		fr := s.Front.IdleFrame(0)
		if anim {
			fr = s.Front.IdleFrame(g.tick * 1000 / 60)
		}
		im := s.Front.Frame(fr)
		f0 := s.Front.IdleFrame(0)
		bx := s.Front.Box(f0).Sub(s.Front.Frame(f0).Bounds().Min).Add(im.Bounds().Min) // frame 0's crop, on the current frame
		sub := im.SubImage(bx).(*ebiten.Image)
		in := size - 2*bw - size*.08
		k := math.Min(in/float64(bx.Dx()), in/float64(bx.Dy()))
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(x+size/2-float64(bx.Dx())*k/2, y+size/2-float64(bx.Dy())*k/2)
		if k < 1.45 {
			op.Filter = ebiten.FilterLinear
		}
		if locked {
			op.ColorScale.Scale(0, 0, 0, .55)
		}
		dst.DrawImage(sub, op)
	}
	if badges && !locked && p != nil {
		is := math.Max(13, size*.22)
		for i := len(p.Types) - 1; i >= 0; i-- {
			bx := x + size - is/2 - 4 - float64(len(p.Types)-1-i)*(is+1)
			disc(dst, bx, y+size-is/2-3, is/2+1, alpha(webInk, .7))
			g.drawTypeIcon(dst, p.Types[i], bx, y+size-is/2-3, is, false)
		}
	}
}

// ---- type chips and matchups (web .tch / muHTML) ----

func chipText(t string, mult float64) string {
	s := cap1s(t)
	if mult != 0 && mult != 1.25 && mult != .5 {
		s += " ×" + trimF(mult)
	}
	return s
}

func trimF(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s
}

func chipW(t string, mult float64) float64 {
	return 5 + 12 + 3 + textW(chipText(t, mult), 10, true) + 6
}

// drawChip draws a type chip with its top-left at (x, y); 15px tall.
func (g *Game) drawChip(dst *ebiten.Image, t string, mult float64, x, y float64) float64 {
	w := chipW(t, mult)
	col := tc(t)
	roundRect(dst, x, y, w, 15, 7, col)
	roundRect(dst, x+1, y+1, w-2, 13, 6, mix(webPanel, color.NRGBA{0, 0, 0, 255}, .25))
	g.drawTypeIcon(dst, t, x+5+6, y+7.5, 11, false)
	drawText(dst, chipText(t, mult), x+5+12+3, y+1, 10, true, col, text.AlignStart, false)
	return w
}

type tmult struct {
	t string
	m float64
}

var typeOrder = []string{"fire", "water", "grass", "electric", "ground", "psychic", "rock", "bug", "dark", "dragon", "steel", "fairy", "flying", "fighting", "poison", "ghost", "ice", "normal"}

func matchups(ts []string) (strong []string, weak, res []tmult) {
	seen := map[string]bool{}
	for _, t := range ts {
		for _, d := range sim.SE[t] {
			if !seen[d] {
				seen[d] = true
				strong = append(strong, d)
			}
		}
	}
	// web iterates Object.keys(SE): the SE literal's order
	for _, a := range seOrder {
		m := sim.TypeMul(a, ts)
		if m > 1 {
			weak = append(weak, tmult{a, m})
		} else if m < 1 {
			res = append(res, tmult{a, m})
		}
	}
	sortTM(weak, func(a, b tmult) bool { return a.m > b.m })
	sortTM(res, func(a, b tmult) bool { return a.m < b.m })
	return
}

var seOrder = []string{"fire", "water", "grass", "electric", "ground", "ice", "fighting", "poison", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy", "normal"}

func sortTM(l []tmult, less func(a, b tmult) bool) { // stable insertion sort (JS sort is stable)
	for i := 1; i < len(l); i++ {
		for j := i; j > 0 && less(l[j], l[j-1]); j-- {
			l[j], l[j-1] = l[j-1], l[j]
		}
	}
}

// matchupRows lays out (and, with dst, draws) the Strong vs / Weak to / Resists rows in width w.
// Returns the height used.
func (g *Game) matchupRows(dst *ebiten.Image, ts []string, x, y, w float64, short bool) float64 {
	strong, weak, res := matchups(ts)
	type row struct {
		gl    glyph
		label string
		ts    []tmult
	}
	var rows []row
	var st []tmult
	for _, t := range strong {
		st = append(st, tmult{t, 0})
	}
	rows = append(rows, row{gSwords, "Strong vs", st})
	if !short {
		rows = append(rows, row{gShield, "Weak to", weak}, row{gHand, "Resists", res})
	}
	y0 := y
	for _, r := range rows {
		lw := richLine(dst, x, y+1, 11, true, webFg, r.gl, r.label+" ")
		cx := x + lw
		if len(r.ts) == 0 {
			if dst != nil {
				drawText(dst, "nothing", cx, y+1, 11, false, alpha(webFg, .55), text.AlignStart, false)
			}
			y += 18
			continue
		}
		for _, m := range r.ts {
			cw := chipW(m.t, m.m)
			if cx+cw > x+w {
				cx = x
				y += 17
			}
			if dst != nil {
				g.drawChip(dst, m.t, m.m, cx, y)
			}
			cx += cw + 3
		}
		y += 18
	}
	return y - y0
}

// ---- small bits ----

// pbar is the web's thin progress bar.
func pbar(dst *ebiten.Image, x, y, w, h, f float64, col color.NRGBA) {
	roundRect(dst, x, y, w, h, h/2, color.NRGBA{0, 0, 0, 120})
	if f > 0 {
		roundRect(dst, x, y, math.Max(h, w*math.Min(1, f)), h, h/2, col)
	}
}

// webBox draws a web-style panel: background, 2px border.
func webBox(dst *ebiten.Image, x, y, w, h, r float64, border, bg color.NRGBA) {
	roundRect(dst, x, y, w, h, r, border)
	roundRect(dst, x+2, y+2, w-4, h-4, math.Max(1, r-2), bg)
}

// smallBtn draws a web "button.sm".
func smallBtn(dst *ebiten.Image, r [4]float64, label string, hot, disabled bool, parts ...any) {
	border := webBorder
	if hot && !disabled {
		border = webAcc
	}
	webBox(dst, r[0], r[1], r[2], r[3], 8, border, webPanel)
	fg := webFg
	if disabled {
		fg = alpha(webFg, .5)
	}
	all := append(parts, label)
	w := richLine(nil, 0, 0, 12, false, fg, all...)
	richLine(dst, r[0]+r[2]/2-w/2, r[1]+r[3]/2-8, 12, false, fg, all...)
}

func smallBtnW(label string, parts ...any) float64 {
	return richLine(nil, 0, 0, 12, false, webFg, append(parts, label)...) + 18
}

// ---- search box ----

// searchBox is a search field: click (or press /) to type; Enter or Esc stops typing.
func (g *Game) searchBox(key string, r [4]float64, q *string, placeholder string) {
	g.textField(key, r, q, placeholder, true)
}

// textField is a single-line text input (search fields reset the list scroll as you type).
func (g *Game) textField(key string, r [4]float64, q *string, placeholder string, search bool) {
	u := &g.ui
	editing := u.edit == key
	if editing {
		for _, ch := range ebiten.AppendInputChars(nil) {
			if len(*q) < 30 && ch >= ' ' {
				*q += string(ch)
				if search {
					u.scroll = 0
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(*q) > 0 {
			rs := []rune(*q)
			*q = string(rs[:len(rs)-1])
			if search {
				u.scroll = 0
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			u.edit = ""
			u.swallow = true
		}
		mx, my := ebiten.CursorPosition()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !inR(r, float64(mx), float64(my)) {
			u.edit = ""
		}
	} else if search && (inpututil.IsKeyJustPressed(ebiten.KeySlash) || (ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyF))) {
		u.edit = key
	}
	u.typing = u.edit != ""
	val := *q
	u.button(key, r, false, false, func(dst *ebiten.Image, rr [4]float64, hot bool) {
		border := webBorder
		if u.edit == key || hot {
			border = b2c(u.edit == key, webAcc, mix(webBorder, webAcc, .4))
		}
		webBox(dst, rr[0], rr[1], rr[2], rr[3], 10, border, webInk)
		tx := rr[0] + 12
		if search {
			drawGlyph(dst, gSearch, rr[0]+22, rr[1]+rr[3]/2, 14, alpha(webFg, .7))
			tx = rr[0] + 38
		}
		if val == "" && u.edit != key {
			drawText(dst, placeholder, tx, rr[1]+rr[3]/2-9, 15, false, alpha(webFg, .45), text.AlignStart, false)
			return
		}
		drawText(dst, val, tx, rr[1]+rr[3]/2-9, 15, false, webFg, text.AlignStart, false)
		if u.edit == key && g.tick/30%2 == 0 {
			cx := tx + textW(val, 15, false) + 1
			rect(dst, cx, rr[1]+rr[3]/2-9, 1.5, 18, webFg)
		}
	}, func() { u.edit = key })
}

// chipBtn draws a web ".chips button" (filter chip).
func chipBtn(dst *ebiten.Image, r [4]float64, on, hot bool, draw func(x, y float64)) {
	border := webBorder
	bg := webPanel
	if on {
		border, bg = webAcc, hexC("#2a2a14")
	} else if hot {
		border = mix(webBorder, webAcc, .5)
	}
	webBox(dst, r[0], r[1], r[2], r[3], 10, border, bg)
	draw(r[0]+12, r[1]+r[3]/2)
}

// ---- cached renders ----

// cacheImg returns an image of size w×h drawn by f, cached under key. Entries unused for a couple of
// seconds are freed.
func (g *Game) cacheImg(key string, w, h int, f func(*ebiten.Image)) *ebiten.Image {
	if g.icache == nil {
		g.icache, g.icacheUsed = map[string]*ebiten.Image{}, map[string]int{}
	}
	if g.tick%60 == 0 {
		for k, t := range g.icacheUsed {
			if g.tick-t > 120 {
				g.icache[k].Deallocate()
				delete(g.icache, k)
				delete(g.icacheUsed, k)
			}
		}
	}
	g.icacheUsed[key] = g.tick
	if im := g.icache[key]; im != nil {
		return im
	}
	im := ebiten.NewImage(max(1, w), max(1, h))
	f(im)
	g.icache[key] = im
	return im
}
