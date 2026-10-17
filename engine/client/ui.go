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
	top      bool   // popup: drawn last, gets clicks first
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
	clicked    bool // a button was activated this frame
	tapX, tapY float64
	tapped     bool
	dragging   bool
	dragY      float64
	dragStart  float64
	touchScrl  float64
	edit       string // key of the text field being typed into
	typing     bool   // a text field has the keyboard this frame (no key navigation)
	swallow    bool   // ignore confirm this frame (the key that ended typing)
	sbDrag     bool   // dragging the scrollbar thumb
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

// popup widgets: drawn over everything and clicked first.
func (u *uiState) topLabel(r [4]float64, d func(dst *ebiten.Image, r [4]float64)) {
	u.add(&widget{r: r, top: true, draw: func(dst *ebiten.Image, r [4]float64, _ bool) { d(dst, r) }})
}

func (u *uiState) topButton(key string, r [4]float64, d func(dst *ebiten.Image, r [4]float64, hot bool), act func()) {
	u.add(&widget{key: key, r: r, top: true, draw: d, act: act})
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
	u.clicked = false
	mx, my := ebiten.CursorPosition()
	if mx != u.lastMouseX || my != u.lastMouseY {
		u.usedMouse = true
		u.lastMouseX, u.lastMouseY = mx, my
	}
	fx, fy := float64(mx), float64(my)
	// touch: a short tap activates; a drag in the scrolling area scrolls it
	u.tapped = false
	if ids := ebiten.AppendTouchIDs(nil); len(ids) > 0 {
		px, py := ebiten.TouchPosition(ids[0])
		if !u.dragging {
			u.dragging, u.dragY, u.dragStart, u.touchScrl = true, float64(py), float64(py), u.scroll
		} else if inR(u.area, float64(px), float64(py)) {
			u.scroll = u.touchScrl + (u.dragStart - float64(py))
		}
		u.dragY = float64(py)
	} else if u.dragging {
		u.dragging = false
		if math.Abs(u.dragY-u.dragStart) < 12 {
			for _, id := range inpututil.AppendJustReleasedTouchIDs(nil) {
				px, py := inpututil.TouchPositionInPreviousTick(id)
				u.tapX, u.tapY, u.tapped = float64(px), float64(py), true
			}
		}
	}
	// wheel scroll
	if _, wy := ebiten.Wheel(); wy != 0 && inR(u.area, fx, fy) {
		u.scroll -= wy * 60
	}
	// scrollbar: click or drag anywhere in the bar on the right edge of the scrolling area
	if u.scrollMax > 0 {
		a := u.area
		bar := [4]float64{a[0] + a[2] - 16, a[1], 16, a[3]}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && inR(bar, fx, fy) {
			u.sbDrag = true
		}
		if u.sbDrag {
			if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
				u.sbDrag = false
			} else {
				h := a[3] * a[3] / (a[3] + u.scrollMax)
				u.scroll = (fy - a[1] - h/2) / math.Max(1, a[3]-h) * u.scrollMax
				u.scroll = math.Max(0, math.Min(u.scrollMax, u.scroll))
				return // the drag doesn't click anything
			}
		}
	}
	u.scroll = math.Max(0, math.Min(u.scrollMax, u.scroll))
	var clickable []*widget
	for _, top := range []bool{true, false} {
		for _, w := range u.ws {
			if w.act != nil && w.key != "" && w.top == top {
				clickable = append(clickable, w)
			}
		}
	}
	// mouse hover sets focus; click (or a touch tap) activates
	for _, w := range clickable {
		if u.tapped && inR(w.r, u.tapX, u.tapY) && u.visible(w) && (!w.scroll || inR(u.area, u.tapX, u.tapY)) {
			u.focus = w.key
			if !w.disabled {
				u.clicked = true
				w.act()
			}
			return
		}
		if u.usedMouse && inR(w.r, fx, fy) && u.visible(w) && (!w.scroll || inR(u.area, fx, fy)) {
			u.focus = w.key
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !w.disabled {
				u.clicked = true
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
	if u.typing || u.swallow {
		u.swallow = false
		return
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
		u.clicked = true
		cur.act()
	}
}

func (u *uiState) draw(dst *ebiten.Image) {
	var clipped *ebiten.Image
	ws := make([]*widget, 0, len(u.ws))
	for _, top := range []bool{false, true} {
		for _, w := range u.ws {
			if w.top == top {
				ws = append(ws, w)
			}
		}
	}
	for _, w := range ws {
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
		roundRect(dst, a[0]+a[2]-9, a[1], 6, a[3], 3, c(40, 46, 60, 200))
		roundRect(dst, a[0]+a[2]-9, y, 6, h, 3, b2c(u.sbDrag, c(200, 210, 230, 255), c(140, 150, 175, 230)))
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
