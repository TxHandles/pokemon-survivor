package client

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// A tiny immediate-mode UI: each screen rebuilds its widget list every frame (in Update); Draw paints
// the list. Buttons work with the mouse, arrow keys / WASD / d-pad (focus moves to the nearest button
// in that direction) and Enter / A.

type widget struct {
	r        [4]float64 // x, y, w, h (screen space, after scrolling)
	draw     func(dst *ebiten.Image, r [4]float64, hot bool)
	act      func()
	disabled bool
	key      string // stable id, keeps focus across frames
	scroll   bool   // inside the scrolling area
}

type uiState struct {
	ws         []*widget
	focus      string
	scroll     float64 // current scroll offset of the scrolling area
	scrollMax  float64
	area       [4]float64 // scrolling area (screen space)
	usedMouse  bool
	lastMouseX int
	lastMouseY int
}

func (u *uiState) reset() { u.ws = u.ws[:0] }

// add registers a widget. In the scrolling area, y is in content space.
func (u *uiState) add(w *widget) *widget {
	if w.scroll {
		w.r[1] += u.area[1] - u.scroll
	}
	u.ws = append(u.ws, w)
	return w
}

func (u *uiState) visible(w *widget) bool {
	if !w.scroll {
		return true
	}
	return w.r[1]+w.r[3] > u.area[1] && w.r[1] < u.area[1]+u.area[3]
}

// label adds a non-interactive drawing.
func (u *uiState) label(r [4]float64, scroll bool, d func(dst *ebiten.Image, r [4]float64)) {
	u.add(&widget{r: r, scroll: scroll, draw: func(dst *ebiten.Image, r [4]float64, _ bool) { d(dst, r) }})
}

// button adds a clickable widget.
func (u *uiState) button(key string, r [4]float64, scroll, disabled bool, d func(dst *ebiten.Image, r [4]float64, hot bool), act func()) {
	u.add(&widget{key: key, r: r, scroll: scroll, disabled: disabled, draw: d, act: act})
}

func inR(r [4]float64, x, y float64) bool {
	return x >= r[0] && x <= r[0]+r[2] && y >= r[1] && y <= r[1]+r[3]
}

func (u *uiState) focused() *widget {
	for _, w := range u.ws {
		if w.key != "" && w.key == u.focus {
			return w
		}
	}
	return nil
}

// update handles input for this frame's widgets (call after building them).
func (u *uiState) update() {
	mx, my := ebiten.CursorPosition()
	if mx != u.lastMouseX || my != u.lastMouseY {
		u.usedMouse = true
		u.lastMouseX, u.lastMouseY = mx, my
	}
	fx, fy := float64(mx), float64(my)
	// wheel scroll
	if _, wy := ebiten.Wheel(); wy != 0 && inR(u.area, fx, fy) {
		u.scroll -= wy * 60
	}
	u.scroll = math.Max(0, math.Min(u.scrollMax, u.scroll))
	var clickable []*widget
	for _, w := range u.ws {
		if w.act != nil && w.key != "" {
			clickable = append(clickable, w)
		}
	}
	// mouse hover sets focus; click activates
	for _, w := range clickable {
		if u.usedMouse && inR(w.r, fx, fy) && u.visible(w) && (!w.scroll || inR(u.area, fx, fy)) {
			u.focus = w.key
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !w.disabled {
				w.act()
				return
			}
		}
	}
	if len(clickable) == 0 {
		return
	}
	cur := u.focused()
	if cur == nil {
		cur = clickable[0]
		u.focus = cur.key
	}
	dx, dy := nav()
	if dx != 0 || dy != 0 {
		u.usedMouse = false
		cx, cy := cur.r[0]+cur.r[2]/2, cur.r[1]+cur.r[3]/2
		var best *widget
		bd := math.Inf(1)
		for _, w := range clickable {
			if w == cur {
				continue
			}
			wx, wy := w.r[0]+w.r[2]/2, w.r[1]+w.r[3]/2
			ex, ey := wx-cx, wy-cy
			along := ex*float64(dx) + ey*float64(dy)
			if along <= 4 {
				continue
			}
			across := math.Abs(ex*float64(dy)) + math.Abs(ey*float64(dx))
			if d := along + across*2.2; d < bd {
				bd, best = d, w
			}
		}
		if best != nil {
			u.focus = best.key
			cur = best
			// keep the focused widget inside the scrolling area
			if best.scroll {
				if best.r[1] < u.area[1] {
					u.scroll -= u.area[1] - best.r[1] + 10
				} else if best.r[1]+best.r[3] > u.area[1]+u.area[3] {
					u.scroll += best.r[1] + best.r[3] - (u.area[1] + u.area[3]) + 10
				}
				u.scroll = math.Max(0, math.Min(u.scrollMax, u.scroll))
			}
		}
	}
	if confirm() && !cur.disabled {
		cur.act()
	}
}

func (u *uiState) draw(dst *ebiten.Image) {
	var clipped *ebiten.Image
	for _, w := range u.ws {
		if w.draw == nil || !u.visible(w) {
			continue
		}
		hot := w.key != "" && w.key == u.focus
		if w.scroll {
			if clipped == nil {
				clipped = dst.SubImage(rectI(u.area)).(*ebiten.Image)
			}
			w.draw(clipped, w.r, hot)
		} else {
			w.draw(dst, w.r, hot)
		}
	}
	if u.scrollMax > 0 { // scrollbar
		a := u.area
		h := a[3] * a[3] / (a[3] + u.scrollMax)
		y := a[1] + (a[3]-h)*u.scroll/u.scrollMax
		roundRect(dst, a[0]+a[2]-6, a[1], 4, a[3], 2, c(40, 46, 60, 200))
		roundRect(dst, a[0]+a[2]-6, y, 4, h, 2, c(140, 150, 175, 230))
	}
}

// ---- common widget looks ----

func btnDraw(label, sub string, col color.NRGBA, disabled bool) func(dst *ebiten.Image, r [4]float64, hot bool) {
	return func(dst *ebiten.Image, r [4]float64, hot bool) {
		bg := c(28, 34, 50, 240)
		if hot && !disabled {
			bg = mix(c(28, 34, 50, 240), col, .28)
			roundRect(dst, r[0]-2, r[1]-2, r[2]+4, r[3]+4, 10, alpha(col, .9))
		}
		roundRect(dst, r[0], r[1], r[2], r[3], 8, bg)
		tc := white
		if disabled {
			tc = c(120, 126, 140, 255)
		}
		if sub == "" {
			drawText(dst, label, r[0]+r[2]/2, r[1]+r[3]/2-9, 15, true, tc, text.AlignCenter, false)
			return
		}
		drawText(dst, label, r[0]+r[2]/2, r[1]+r[3]/2-17, 15, true, tc, text.AlignCenter, false)
		fs := 12
		for fs > 8 {
			if tw, _ := text.Measure(sub, font(fs, false), 0); tw <= r[2]-14 {
				break
			}
			fs--
		}
		drawText(dst, sub, r[0]+r[2]/2, r[1]+r[3]/2+3, fs, false, b2c(disabled, c(120, 126, 140, 255), dim), text.AlignCenter, false)
	}
}

func panelDraw(col color.NRGBA) func(dst *ebiten.Image, r [4]float64) {
	return func(dst *ebiten.Image, r [4]float64) { roundRect(dst, r[0], r[1], r[2], r[3], 12, col) }
}

func textDraw(s string, size int, bold bool, col color.NRGBA, align text.Align) func(dst *ebiten.Image, r [4]float64) {
	return func(dst *ebiten.Image, r [4]float64) {
		x := r[0]
		switch align {
		case text.AlignCenter:
			x += r[2] / 2
		case text.AlignEnd:
			x += r[2]
		}
		drawText(dst, s, x, r[1], size, bold, col, align, false)
	}
}
