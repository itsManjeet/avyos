package widget

import (
	"image"
	"testing"

	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/paint"
	"avyos.dev/lib/graphics/theme"
)

type countingBox struct {
	layouts int
	paints  int
	size    geom.Size
}

type countingBuildable struct {
	builds int
	child  Widget
}

func (b *countingBox) Layout(c layout.BoxConstraints) geom.Size {
	b.layouts++
	return c.Constrain(b.size)
}

func (b *countingBox) Paint(_ *paint.Context, _ geom.Point, _ geom.Size) {
	b.paints++
}

func (b *countingBox) HitTest(_, _ geom.Point, _ geom.Size) bool { return false }

func (b *countingBuildable) Build(BuildContext) Widget {
	b.builds++
	return b.child
}

func TestFrameRenderReusesMeasuredLayoutWithinFrame(t *testing.T) {
	box := &countingBox{size: geom.Sz(120, 24)}
	root := Container{Child: box}
	frame := NewFrame(theme.Light(), geom.Sz(320, 200))
	canvas := pixbuf.NewCanvas(320, 200)

	frame.Render(root, canvas)

	if box.layouts != 1 {
		t.Fatalf("expected 1 layout call, got %d", box.layouts)
	}
	if box.paints != 1 {
		t.Fatalf("expected 1 paint call, got %d", box.paints)
	}
}

func TestFrameRenderReusesBuiltWidgetWithinFrame(t *testing.T) {
	box := &countingBox{size: geom.Sz(120, 24)}
	buildable := &countingBuildable{child: Container{Child: box}}
	root := Padding{Insets: layout.All(8), Child: buildable}
	frame := NewFrame(theme.Light(), geom.Sz(320, 200))
	canvas := pixbuf.NewCanvas(320, 200)

	frame.Render(root, canvas)

	if buildable.builds != 1 {
		t.Fatalf("expected 1 build call, got %d", buildable.builds)
	}
	if box.layouts != 1 {
		t.Fatalf("expected 1 layout call, got %d", box.layouts)
	}
}

func TestFrameDamageHintTracksHoverRect(t *testing.T) {
	root := Stack{
		Children: []Widget{
			Positioned{
				Left:  Ptr(5),
				Top:   Ptr(5),
				Child: GestureDetector{Child: SizedBox{Width: 40, Height: 20}},
			},
		},
	}
	frame := NewFrame(theme.Light(), geom.Sz(100, 60))
	canvas := pixbuf.NewCanvas(100, 60)

	frame.Render(root, canvas)
	frame.HandlePointerMove(geom.Pt(10, 10))

	want := image.Rect(0, 0, 77, 57)
	if got := frame.DamageHint(); got != want {
		t.Fatalf("expected hover damage %v, got %v", want, got)
	}
}

func TestFrameDamageHintTracksFocusedInputEdits(t *testing.T) {
	value := ""
	root := Positioned{
		Left:  Ptr(12),
		Top:   Ptr(10),
		Width: Ptr(120),
		Child: TextInput{Value: &value, Hint: "Name"},
	}
	frame := NewFrame(theme.Light(), geom.Sz(160, 80))
	canvas := pixbuf.NewCanvas(160, 80)

	frame.Render(root, canvas)
	frame.FocusInputPath(&value, "root")
	frame.Render(root, canvas)
	rect, ok := frame.pathRects["root"]
	if !ok {
		t.Fatal("expected root rect to be recorded")
	}

	frame.HandleKey(event.TextInputEvent{Rune: 'a'})

	want := image.Rect(
		max(0, int(rect.Min.X)-int(damageEffectPad)),
		max(0, int(rect.Min.Y)-int(damageEffectPad)),
		min(int(frame.Screen.Width), int(rect.Max.X)+int(damageEffectPad)),
		min(int(frame.Screen.Height), int(rect.Max.Y)+int(damageEffectPad)),
	)
	if got := frame.DamageHint(); got != want {
		t.Fatalf("expected focused input damage %v, got %v", want, got)
	}
}

func TestFrameCursorShapeTracksHoveredGestureCursor(t *testing.T) {
	root := Positioned{
		Left:   Ptr(8),
		Top:    Ptr(6),
		Width:  Ptr(30),
		Height: Ptr(20),
		Child: GestureDetector{
			Cursor: event.CursorResizeEW,
			Child:  SizedBox{Width: 30, Height: 20},
		},
	}
	frame := NewFrame(theme.Light(), geom.Sz(80, 40))
	canvas := pixbuf.NewCanvas(80, 40)

	frame.Render(root, canvas)
	frame.HandlePointerMove(geom.Pt(10, 10))

	if got := frame.CursorShape(); got != event.CursorResizeEW {
		t.Fatalf("CursorShape = %v, want %v", got, event.CursorResizeEW)
	}
}

func TestTextInputUsesTextCursorShape(t *testing.T) {
	value := ""
	root := Positioned{
		Left:  Ptr(12),
		Top:   Ptr(8),
		Width: Ptr(120),
		Child: TextInput{Value: &value, Hint: "Name"},
	}
	frame := NewFrame(theme.Light(), geom.Sz(160, 80))
	canvas := pixbuf.NewCanvas(160, 80)

	frame.Render(root, canvas)
	frame.HandlePointerMove(geom.Pt(20, 20))

	if got := frame.CursorShape(); got != event.CursorText {
		t.Fatalf("CursorShape = %v, want %v", got, event.CursorText)
	}
}

func TestColumnRenderHandlesMoreThanSixteenChildren(t *testing.T) {
	children := make([]Widget, 0, 28)
	for range 28 {
		children = append(children, SizedBox{Height: 10})
	}

	frame := NewFrame(theme.Light(), geom.Sz(320, 640))
	canvas := pixbuf.NewCanvas(320, 640)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("column render panicked: %v", r)
		}
	}()

	frame.Render(Column{Children: children}, canvas)
}

func TestGridRenderHandlesLargeChildAndColumnCounts(t *testing.T) {
	children := make([]Widget, 0, 20)
	for range 20 {
		children = append(children, SizedBox{Height: 24})
	}

	frame := NewFrame(theme.Light(), geom.Sz(1200, 600))
	canvas := pixbuf.NewCanvas(1200, 600)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("grid render panicked: %v", r)
		}
	}()

	frame.Render(Grid{Columns: 10, Children: children}, canvas)
}

func TestWrapRenderHandlesMoreThanSixteenChildren(t *testing.T) {
	children := make([]Widget, 0, 24)
	for range 24 {
		children = append(children, SizedBox{Width: 40, Height: 16})
	}

	frame := NewFrame(theme.Light(), geom.Sz(320, 240))
	canvas := pixbuf.NewCanvas(320, 240)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("wrap render panicked: %v", r)
		}
	}()

	frame.Render(Wrap{Children: children, Spacing: 4, RunSpacing: 4}, canvas)
}
