package client

import (
	"github.com/TxHandles/pokemon-survivor/engine/sim"
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Held-item icons: original little drawings (thick outline, flat two-tone shading, like the
// landmarks), painted once into 64px images with the CPU canvas.

const itemOutline = "#1e1c26"

var itemIconCache = map[string]*ebiten.Image{}

// itemIcon returns item id's icon (nil for unknown items).
func itemIcon(id string) *ebiten.Image {
	if im, ok := itemIconCache[id]; ok {
		return im
	}
	f := itemPainters[id]
	if f == nil {
		itemIconCache[id] = nil
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	x := newCnv(img)
	f(x)
	im := ebiten.NewImageFromImage(img)
	itemIconCache[id] = im
	return im
}

// drawItemIcon draws item id's icon centred on (cx, cy) at size s.
func drawItemIcon(dst *ebiten.Image, id string, cx, cy, s float64) {
	if sim.IsFused(id) { // a fusion: both parts overlapping inside a swirl
		a, b := sim.FusedParts(id)
		strokeEllipse(dst, cx, cy, s*.5, s*.5, 0, 2*math.Pi, 1.5, alpha(hexC("#c49bff"), .8))
		drawItemIcon(dst, a, cx-s*.2, cy-s*.12, s*.68)
		drawItemIcon(dst, b, cx+s*.2, cy+s*.14, s*.68)
		return
	}
	if im := itemIcon(id); im != nil {
		stamp(dst, im, cx, cy, s, 0, 1)
		return
	}
	drawText(dst, itemAbbr(id), cx, cy-s*.25, int(math.Max(8, s*.35)), true, webFg, 1, false)
}

// fl fills the current path and outlines it.
func fl(x *cnv, fill string, w float64) {
	x.setFillS(fill)
	x.fill()
	if w > 0 {
		x.setStrokeS(itemOutline)
		x.lineWidth(w)
		x.stroke()
	}
}

func circ(x *cnv, cx, cy, r float64) {
	x.beginPath()
	x.arc(cx, cy, r, 0, 2*math.Pi)
	x.closePath()
}

func rrect(x *cnv, rx, ry, w, h, r float64) {
	x.beginPath()
	x.moveTo(rx+r, ry)
	x.lineTo(rx+w-r, ry)
	x.ellipse(rx+w-r, ry+r, r, r, 0, -math.Pi/2, 0, false)
	x.lineTo(rx+w, ry+h-r)
	x.ellipse(rx+w-r, ry+h-r, r, r, 0, 0, math.Pi/2, false)
	x.lineTo(rx+r, ry+h)
	x.ellipse(rx+r, ry+h-r, r, r, 0, math.Pi/2, math.Pi, false)
	x.lineTo(rx, ry+r)
	x.ellipse(rx+r, ry+r, r, r, 0, math.Pi, 3*math.Pi/2, false)
	x.closePath()
}

func polyP(x *cnv, pts ...float64) {
	x.beginPath()
	x.moveTo(pts[0], pts[1])
	for i := 2; i < len(pts); i += 2 {
		x.lineTo(pts[i], pts[i+1])
	}
	x.closePath()
}

func strokeL(x *cnv, col string, w float64, pts ...float64) {
	x.beginPath()
	x.moveTo(pts[0], pts[1])
	for i := 2; i < len(pts); i += 2 {
		x.lineTo(pts[i], pts[i+1])
	}
	x.setStrokeS(col)
	x.lineWidth(w)
	x.stroke()
}

// a headband: a band across with two tails (band / choice / muscle variants)
func headband(x *cnv, c1, c2 string) {
	polyP(x, 30, 34, 44, 52, 38, 56, 26, 40)
	fl(x, c2, 3)
	polyP(x, 34, 34, 54, 46, 50, 52, 30, 40)
	fl(x, c1, 3)
	rrect(x, 6, 20, 52, 16, 7)
	fl(x, c1, 3)
	rrect(x, 9, 23, 46, 5, 2.5)
	x.setFillS("rgba(255,255,255,.35)")
	x.fill()
}

func lensShape(x *cnv, glass, rim string, cross bool) {
	strokeL(x, itemOutline, 11, 40, 40, 56, 56)
	strokeL(x, "#8a5a2c", 7, 40, 40, 56, 56)
	circ(x, 27, 27, 20)
	fl(x, rim, 3)
	circ(x, 27, 27, 14)
	fl(x, glass, 2)
	if cross {
		strokeL(x, "#e83a3a", 2.5, 27, 15, 27, 39)
		strokeL(x, "#e83a3a", 2.5, 15, 27, 39, 27)
	}
	x.beginPath()
	x.ellipse(21, 21, 5, 3, -.7, 0, 2*math.Pi, false)
	x.setFillS("rgba(255,255,255,.7)")
	x.fill()
}

var itemPainters = map[string]func(x *cnv){
	"band": func(x *cnv) { headband(x, "#e8443a", "#a8221a") },
	"choice": func(x *cnv) {
		headband(x, "#ffd04a", "#c89a10")
		x.beginPath()
		for i := 0; i < 10; i++ {
			r := 7.0
			if i%2 == 1 {
				r = 3
			}
			a := -math.Pi/2 + float64(i)*math.Pi/5
			x.lineTo(32+math.Cos(a)*r, 28+math.Sin(a)*r)
		}
		x.closePath()
		fl(x, "#e8443a", 1.5)
	},
	"muscle": func(x *cnv) {
		rrect(x, 14, 14, 36, 36, 10)
		fl(x, "#ffd04a", 3)
		rrect(x, 20, 20, 24, 24, 6)
		fl(x, "#1e1c26", 0)
		rrect(x, 14, 28, 36, 8, 2)
		fl(x, "#c89a10", 2)
	},
	"claw": func(x *cnv) {
		for i := 0; i < 3; i++ {
			ox := float64(i)*13 - 13
			x.beginPath()
			x.moveTo(26+ox, 56)
			x.quadTo(14+ox, 30, 32+ox, 8)
			x.quadTo(26+ox, 30, 36+ox, 56)
			x.closePath()
			fl(x, "#dfe3ea", 2.5)
		}
		rrect(x, 10, 48, 44, 10, 4)
		fl(x, "#5a6a8a", 2.5)
	},
	"razor": func(x *cnv) {
		x.beginPath()
		x.moveTo(18, 58)
		x.quadTo(6, 24, 44, 6)
		x.quadTo(28, 28, 34, 58)
		x.closePath()
		fl(x, "#f2f2f8", 3)
		x.beginPath()
		x.moveTo(24, 52)
		x.quadTo(18, 30, 38, 14)
		x.setStrokeS("#b8a8e8")
		x.lineWidth(3)
		x.stroke()
		rrect(x, 12, 50, 30, 10, 4)
		fl(x, "#6a4aa8", 2.5)
	},
	"left": func(x *cnv) { // a half-eaten apple
		x.beginPath()
		x.arc(32, 36, 21, 0, 2*math.Pi)
		x.closePath()
		fl(x, "#e8443a", 3)
		circ(x, 52, 30, 9)
		fl(x, "#f5e6c8", 0)
		circ(x, 48, 44, 7)
		fl(x, "#f5e6c8", 0)
		x.beginPath()
		x.ellipse(24, 28, 6, 4, -.6, 0, 2*math.Pi, false)
		x.setFillS("rgba(255,255,255,.45)")
		x.fill()
		strokeL(x, "#6a4424", 3.5, 32, 16, 34, 7)
		x.beginPath()
		x.ellipse(40, 10, 7, 3.5, -.4, 0, 2*math.Pi, false)
		fl(x, "#4aa04a", 2)
	},
	"lens":  func(x *cnv) { lensShape(x, "#ffb0b0", "#c83a3a", true) },
	"wide":  func(x *cnv) { lensShape(x, "#bfe6ff", "#2f6fd0", false) },
	"focus": func(x *cnv) { lensShape(x, "#d6f58a", "#3a8a3a", true) },
	"boots": func(x *cnv) {
		x.beginPath() // wing
		x.moveTo(22, 22)
		x.quadTo(6, 10, 4, 26)
		x.quadTo(10, 24, 12, 30)
		x.quadTo(16, 28, 20, 32)
		x.closePath()
		fl(x, "#ffffff", 2.5)
		polyP(x, 22, 8, 40, 8, 40, 40, 58, 46, 58, 56, 22, 56)
		fl(x, "#3a7ae8", 3)
		rrect(x, 20, 50, 40, 8, 3)
		fl(x, "#1e3a8a", 2.5)
		strokeL(x, "#bfe6ff", 2.5, 26, 14, 36, 14)
	},
	"metal": func(x *cnv) { // a steel plate
		x.beginPath()
		x.moveTo(32, 6)
		x.lineTo(54, 14)
		x.quadTo(54, 44, 32, 58)
		x.quadTo(10, 44, 10, 14)
		x.closePath()
		fl(x, "#b8c4d4", 3)
		x.beginPath()
		x.moveTo(32, 12)
		x.lineTo(32, 52)
		x.quadTo(16, 42, 16, 18)
		x.closePath()
		x.setFillS("rgba(255,255,255,.45)")
		x.fill()
		for _, p := range [][2]float64{{20, 18}, {44, 18}, {32, 46}} {
			circ(x, p[0], p[1], 2.5)
			fl(x, "#5a6a7a", 0)
		}
	},
	"magnet": func(x *cnv) {
		x.beginPath()
		x.arc(32, 30, 20, math.Pi, 2*math.Pi)
		x.lineTo(52, 50)
		x.lineTo(42, 50)
		x.lineTo(42, 30)
		x.ellipse(32, 30, 10, 10, 0, 0, math.Pi, true) // the inner arc, back over the top
		x.lineTo(22, 50)
		x.lineTo(12, 50)
		x.closePath()
		fl(x, "#e8443a", 3)
		rrect(x, 11, 44, 12, 10, 2)
		fl(x, "#dfe3ea", 2.5)
		rrect(x, 41, 44, 12, 10, 2)
		fl(x, "#dfe3ea", 2.5)
	},
	"exp": func(x *cnv) { // two circling arrows around a star
		x.beginPath()
		x.arc(32, 32, 22, -2.6, .3)
		x.setStrokeS(itemOutline)
		x.lineWidth(9)
		x.stroke()
		x.setStrokeS("#ff9a3a")
		x.lineWidth(5)
		x.stroke()
		x.beginPath()
		x.arc(32, 32, 22, .54, 3.44)
		x.setStrokeS(itemOutline)
		x.lineWidth(9)
		x.stroke()
		x.setStrokeS("#ffd04a")
		x.lineWidth(5)
		x.stroke()
		polyP(x, 54, 40, 58, 28, 46, 32)
		fl(x, "#ff9a3a", 2)
		polyP(x, 10, 24, 6, 36, 18, 32)
		fl(x, "#ffd04a", 2)
		x.beginPath()
		for i := 0; i < 10; i++ {
			r := 11.0
			if i%2 == 1 {
				r = 5
			}
			a := -math.Pi/2 + float64(i)*math.Pi/5
			x.lineTo(32+math.Cos(a)*r, 33+math.Sin(a)*r)
		}
		x.closePath()
		fl(x, "#ffe066", 2)
	},
	"soothe": func(x *cnv) { // a bell
		x.beginPath()
		x.moveTo(32, 8)
		x.quadTo(50, 10, 50, 34)
		x.lineTo(56, 46)
		x.lineTo(8, 46)
		x.lineTo(14, 34)
		x.quadTo(14, 10, 32, 8)
		x.closePath()
		fl(x, "#ffd04a", 3)
		circ(x, 32, 50, 6)
		fl(x, "#c89a10", 2.5)
		x.beginPath()
		x.ellipse(24, 22, 4, 8, .3, 0, 2*math.Pi, false)
		x.setFillS("rgba(255,255,255,.5)")
		x.fill()
		rrect(x, 8, 40, 48, 6, 3)
		fl(x, "#e8b830", 2)
	},
	"glove": func(x *cnv) { // boxing glove
		rrect(x, 12, 10, 38, 34, 14)
		fl(x, "#e8443a", 3)
		x.beginPath()
		x.ellipse(16, 30, 8, 10, -.3, 0, 2*math.Pi, false)
		fl(x, "#e8443a", 3)
		rrect(x, 16, 42, 30, 14, 4)
		fl(x, "#f2f2f2", 3)
		x.beginPath()
		x.ellipse(28, 20, 8, 4, -.2, 0, 2*math.Pi, false)
		x.setFillS("rgba(255,255,255,.4)")
		x.fill()
	},
	"metro": func(x *cnv) { // metronome
		polyP(x, 22, 8, 42, 8, 54, 58, 10, 58)
		fl(x, "#8a5a2c", 3)
		polyP(x, 26, 14, 38, 14, 46, 50, 18, 50)
		fl(x, "#f5e6c8", 2)
		strokeL(x, itemOutline, 4.5, 32, 50, 42, 18)
		strokeL(x, "#dfe3ea", 2.5, 32, 50, 42, 18)
		rrect(x, 36, 26, 10, 7, 2)
		fl(x, "#e8443a", 2)
	},
	"eject": func(x *cnv) { // a big red button with an up arrow
		x.beginPath()
		x.ellipse(32, 42, 24, 12, 0, 0, 2*math.Pi, false)
		fl(x, "#5a6070", 3)
		x.beginPath()
		x.ellipse(32, 34, 20, 13, 0, 0, 2*math.Pi, false)
		fl(x, "#e8443a", 3)
		polyP(x, 32, 22, 42, 34, 36, 34, 36, 40, 28, 40, 28, 34, 22, 34)
		fl(x, "#ffffff", 0)
	},
	"belt": func(x *cnv) { // black belt with a gold buckle
		rrect(x, 4, 24, 56, 16, 5)
		fl(x, "#2a2630", 3)
		rrect(x, 22, 18, 20, 28, 4)
		fl(x, "#ffd04a", 3)
		rrect(x, 27, 23, 10, 18, 2)
		fl(x, "#2a2630", 0)
		strokeL(x, "#c89a10", 3, 32, 24, 32, 40)
	},
	"heat": func(x *cnv) { // a glowing rock with a flame
		x.beginPath()
		x.moveTo(8, 54)
		x.lineTo(12, 34)
		x.lineTo(26, 26)
		x.lineTo(44, 28)
		x.lineTo(56, 40)
		x.lineTo(56, 54)
		x.closePath()
		fl(x, "#7a3a2a", 3)
		strokeL(x, "#ff8a3a", 3, 18, 48, 28, 38, 40, 44)
		x.beginPath()
		x.moveTo(34, 4)
		x.quadTo(48, 18, 40, 28)
		x.quadTo(32, 34, 26, 26)
		x.quadTo(22, 16, 34, 4)
		x.closePath()
		fl(x, "#ff7a1f", 2.5)
		x.beginPath()
		x.moveTo(34, 14)
		x.quadTo(40, 22, 35, 26)
		x.quadTo(30, 26, 31, 21)
		x.closePath()
		fl(x, "#ffd24a", 0)
	},
	"helmet": func(x *cnv) { // rocky helmet: a rock dome with spikes
		for i := 0; i < 5; i++ {
			a := math.Pi + float64(i)*math.Pi/4
			cx, cy := 32+math.Cos(a)*22, 40+math.Sin(a)*22
			polyP(x, cx+math.Cos(a)*12, cy+math.Sin(a)*12, cx+math.Cos(a+1.6)*6, cy+math.Sin(a+1.6)*6, cx+math.Cos(a-1.6)*6, cy+math.Sin(a-1.6)*6)
			fl(x, "#8a8f99", 2.5)
		}
		x.beginPath()
		x.arc(32, 42, 22, math.Pi, 2*math.Pi)
		x.closePath()
		fl(x, "#a8adb8", 3)
		rrect(x, 8, 40, 48, 10, 3)
		fl(x, "#6a6f79", 3)
	},
	"lifeorb": func(x *cnv) {
		x.st.fill = x.radial(26, 24, 2, 26, gstop{0, cssC("#f2c8ff")}, gstop{.5, cssC("#b04fd1")}, gstop{1, cssC("#4a1a6a")})
		circ(x, 32, 32, 24)
		x.fill()
		x.setStrokeS(itemOutline)
		x.lineWidth(3)
		x.stroke()
		x.beginPath()
		x.ellipse(24, 22, 7, 4, -.6, 0, 2*math.Pi, false)
		x.setFillS("rgba(255,255,255,.75)")
		x.fill()
	},
	"sash": func(x *cnv) { // a ribbon sash
		polyP(x, 10, 10, 22, 6, 56, 50, 44, 56)
		fl(x, "#ff8a3a", 3)
		polyP(x, 40, 52, 48, 46, 50, 62, 44, 60)
		fl(x, "#e8642a", 2.5)
		strokeL(x, "#ffd8b8", 2.5, 18, 10, 50, 50)
	},
	"wise": func(x *cnv) { // glasses
		strokeL(x, itemOutline, 5, 6, 26, 12, 30)
		strokeL(x, itemOutline, 5, 58, 26, 52, 30)
		for _, cx := range []float64{20, 44} {
			rrect(x, cx-12, 24, 24, 18, 7)
			fl(x, "#bfe6ff", 3.5)
			x.beginPath()
			x.ellipse(cx-4, 30, 4, 2.5, -.5, 0, 2*math.Pi, false)
			x.setFillS("rgba(255,255,255,.8)")
			x.fill()
		}
		strokeL(x, itemOutline, 3.5, 30, 30, 34, 30)
	},
}
