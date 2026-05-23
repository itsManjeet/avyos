# Building UI with lib/graphics

`lib/graphics` is the Go UI toolkit used by every avyos application. It is a
retained widget framework that compiles without CGO and runs directly on the
DRM/KMS framebuffer, the avyos desktop protocol, or macOS (Metal). This guide
walks from the smallest possible window to a production-quality, responsive
app — covering every widget, every collection, and every lifecycle hook.

---

## Table of contents

1. [Mental model](#1-mental-model)
2. [Smallest possible app](#2-smallest-possible-app)
3. [Options and theme](#3-options-and-theme)
4. [State management](#4-state-management)
5. [Layout primitives](#5-layout-primitives)
6. [Container and surfaces](#6-container-and-surfaces)
7. [Text and forms](#7-text-and-forms)
8. [Buttons and interaction](#8-buttons-and-interaction)
9. [Toggles and range controls](#9-toggles-and-range-controls)
10. [Scroll views](#10-scroll-views)
11. [Images and icons](#11-images-and-icons)
12. [Animation](#12-animation)
13. [Collections: application shell](#13-collections-application-shell)
14. [Collections: overlays, dialogs, popups](#14-collections-overlays-dialogs-popups)
15. [Collections: toasts and panels](#15-collections-toasts-and-panels)
16. [Responsive layout](#16-responsive-layout)
17. [Custom widgets](#17-custom-widgets)
18. [Lifecycle hooks](#18-lifecycle-hooks)

---

## 1. Mental model

```
app.Run(root) → event loop
    ↓ each frame
  widget.Frame.Build(root)      ← calls Build / CreateState / etc.
    ↓
  layout pass                   ← RenderBox.Layout(constraints)
    ↓
  paint pass                    ← RenderBox.Paint(ctx, offset, size)
    ↓
  canvas.Flush → backend blit
```

Key rules:

- **Widget trees are rebuilt every frame.** Widgets are cheap structs; their
  methods are called on the render goroutine.
- **Stateless composition** is a plain struct with a `Build(ctx)` method
  (`Buildable`).
- **Stateful widgets** implement `StatefulWidget` (`CreateState() State`). The
  state persists across rebuilds; mutations must happen inside `SetState`.
- **Leaf rendering** uses `RenderBox` (`Layout`, `Paint`, `HitTest`).
- **Custom multi-child parents** use `MultiChild` / `ChildRenderer`.
- Widgets are keyed by their position in the widget tree. Moving a stateful
  widget to a different path resets its state.

---

## 2. Smallest possible app

```go
package main

import (
    "log"

    "avyos.dev/lib/graphics/app"
    "avyos.dev/lib/graphics/widget"
)

type HelloApp struct{}

func (HelloApp) Build(_ widget.BuildContext) widget.Widget {
    return widget.Text{Content: "Hello, avyos"}
}

func main() {
    app.Options.Title  = "Hello"
    app.Options.Width  = 480
    app.Options.Height = 320
    if err := app.Run(HelloApp{}); err != nil {
        log.Fatal(err)
    }
}
```

`app.Run` opens a window, enters the event loop, and returns only when the
window is closed (or `app.Stop()` is called). It reads configuration from the
package-level `app.Options` variable, which you set before calling `Run`.

---

## 3. Options and theme

All window options live in `app.Options`:

```go
app.Options.Title      = "My App"
app.Options.Width      = 800
app.Options.Height     = 600
app.Options.Fullscreen = false
app.Options.Resizable  = true
app.Options.Scale      = 1.0   // explicit HiDPI scale; 0 = auto
app.Options.Theme      = theme.Dark()   // default is theme.Light()
```

`theme.Dark()` and `theme.Light()` return a `*theme.ThemeData` that contains:

| Field         | Description                                          |
|---------------|------------------------------------------------------|
| `ColorScheme` | All semantic colors (Fg, BgPanel, Primary, …)        |
| `TextTheme`   | Named text styles (BodyMedium, TitleLarge, …)        |
| `Shape`       | Corner radii tokens (SmallRadius, LargeRadius, …)    |
| `Space`       | 4 px grid helper (`Space.Unit(4)` → 16 px)           |
| `Shadow`      | Elevation specs (XS through XXL)                     |
| `Motion`      | Animation durations (Fast = 120 ms, Moderate = 180 ms)|

Access the active theme inside `Build` via `ctx.Theme`:

```go
func (w MyWidget) Build(ctx widget.BuildContext) widget.Widget {
    headlineSt := ctx.Theme.TextTheme.HeadlineMedium
    headlineSt.Color = ctx.Theme.ColorScheme.Primary
    return widget.Text{Content: "Styled", Style: &headlineSt}
}
```

Fonts are loaded from `/etc/fonts.ini` at startup. If the file is absent,
the bundled TTF fallback is used.

---

## 4. State management

Any widget that needs to mutate its own data implements `StatefulWidget`:

```go
type CounterApp struct{}

func (CounterApp) CreateState() widget.State { return &counterState{} }

type counterState struct {
    widget.StateBase   // embed — provides SetState, BuildContext, etc.
    count int
}

func (s *counterState) InitState() {
    // called once when the state is first created
}

func (s *counterState) Build(ctx widget.BuildContext) widget.Widget {
    return widget.Text{Content: "Count: " + strconv.Itoa(s.count)}
}
```

**Mutate state only inside `SetState`:**

```go
widget.GestureDetector{
    OnTap: func() {
        s.SetState(func() { s.count++ })
    },
    Child: widget.Text{Content: "Tap me"},
}
```

`SetState` schedules a rebuild on the main goroutine. Calling it from a
background goroutine is safe and common (timers, network callbacks, etc.).

If your state type receives updated widget props between builds, implement
`UpdateWidget`:

```go
func (s *counterState) UpdateWidget(w widget.Widget) {
    if v, ok := w.(CounterApp); ok {
        s.widget = v   // store the latest widget value
    }
}
```

---

## 5. Layout primitives

### Row and Column

`Row` and `Column` implement flex layout:

```go
widget.Row{
    MainAxisAlignment:  layout.MainSpaceBetween,
    CrossAxisAlignment: layout.CrossCenter,
    MainAxisSize:       layout.MainMax,
    Children: []widget.Widget{
        widget.Text{Content: "Left"},
        widget.Spacer{},
        widget.Text{Content: "Right"},
    },
}

widget.Column{
    CrossAxisAlignment: layout.CrossStretch,
    Children: []widget.Widget{
        widget.Text{Content: "Top"},
        widget.SizedBox{Height: 8},
        widget.Text{Content: "Bottom"},
    },
}
```

`MainAxisAlignment` values: `MainStart`, `MainEnd`, `MainCenter`,
`MainSpaceBetween`, `MainSpaceAround`, `MainSpaceEvenly`.

`CrossAxisAlignment` values: `CrossStart`, `CrossEnd`, `CrossCenter`,
`CrossStretch`.

### Expanded and Spacer

`Expanded` fills remaining space along the main axis:

```go
widget.Row{
    Children: []widget.Widget{
        widget.SizedBox{Width: 80, Child: leftPanel},
        widget.Expanded{Child: mainContent},   // fills the rest
    },
}
```

`Spacer{}` is a zero-size `Expanded` used to push siblings apart:

```go
widget.Row{
    Children: []widget.Widget{
        widget.Text{Content: "Title"},
        widget.Spacer{},                       // push to sides
        widget.Text{Content: "Action"},
    },
}
```

### Flex and Grid

`Flex` is a direction-agnostic flex with a uniform gap:

```go
widget.Flex{
    Direction: layout.Vertical,
    Gap:       12,
    Children:  items,
}
```

`Grid` lays children in equal-width columns:

```go
widget.Grid{
    Columns:  3,   // or set MinChildWidth for auto count
    Gap:      8,
    Children: cards,
}
```

### Wrap

`Wrap` flows children onto new rows when the available width is exceeded —
ideal for tag chips:

```go
widget.Wrap{
    Spacing:    8,
    RunSpacing: 8,
    Children:   chips,
}
```

### Stack and Positioned

`Stack` layers children on top of each other. `Positioned` anchors a child
relative to the stack edges:

```go
widget.Stack{
    Children: []widget.Widget{
        backgroundWidget,
        widget.Positioned{
            Bottom: widget.Ptr(16.0),
            Right:  widget.Ptr(16.0),
            Child:  fabWidget,
        },
    },
}
```

`widget.Ptr(v)` is a shorthand that returns `*float64` — used because Go
doesn't allow taking the address of a literal.

### Padding, Align, SizedBox

```go
// Fixed inset on all four sides
widget.Padding{
    Insets: layout.All(16),
    Child:  content,
}

// Asymmetric inset: 24 left/right, 12 top/bottom
widget.Padding{
    Insets: layout.Symmetric(24, 12),
    Child:  content,
}

// Explicit per-side: left, top, right, bottom
widget.Padding{
    Insets: layout.LTRB(8, 4, 8, 4),
    Child:  content,
}

// Align child inside available space
widget.Align{
    Alignment: layout.AlignBottomRight,
    Child:     content,
}

// Center is shorthand for Align{AlignCenter, child}
widget.Center(content)

// Force exact size; Child is optional
widget.SizedBox{Width: 200, Height: 48, Child: content}
widget.SizedBox{Height: 16}  // spacer gap
```

### ScrollArea and Splitter

`ScrollArea` is a low-level clipping viewport. For most uses, prefer the
higher-level `Scroll` widget (covered in [section 10](#10-scroll-views)).

`Splitter` creates two panes separated by a draggable divider:

```go
widget.Splitter{
    Axis:   layout.Horizontal,
    Ratio:  0.3,       // initial split: 30% | 70%
    First:  sidePanel,
    Second: mainPanel,
}
```

### Separator and AspectRatio

```go
widget.Separator{Color: ctx.Theme.ColorScheme.Border}

widget.AspectRatio{
    Ratio: 16.0 / 9.0,
    Child: videoWidget,
}
```

---

## 6. Container and surfaces

`Container` is the main rectangular primitive. Every field is optional:

```go
widget.Container{
    Width:         240,
    Height:        80,
    Fill:          ctx.Theme.ColorScheme.BgPanel,
    Border:        ctx.Theme.ColorScheme.Border,
    BorderWidth:   1,
    Radius:        ctx.Theme.Shape.LargeRadius,
    Shadow:        ctx.Theme.ColorScheme.Shadow,
    ShadowBlur:    ctx.Theme.Shadow.MD.Blur,
    ShadowOffsetY: ctx.Theme.Shadow.MD.OffsetY,
    Padding:       layout.All(16),
    Child:         content,
}
```

`Opacity` fades a child by overlaying a transparent rect (approximation,
works well over opaque backgrounds):

```go
widget.Opacity{Value: 0.4, Child: content}
// Value 1.0 = fully opaque; 0.0 = invisible
```

---

## 7. Text and forms

### Text

`Text` renders a string. When `Style` is nil it uses `BodyMedium`:

```go
widget.Text{Content: "Plain body text"}

titleSt := ctx.Theme.TextTheme.TitleLarge
widget.Text{Content: "Page title", Style: &titleSt}
```

Available text styles in `TextTheme`:
`Size2XS`, `SizeXS`, `SizeSM`, `SizeMD`, `SizeLG`, `SizeXL` … `Size7XL`,
`DisplayLarge/Medium/Small`,
`HeadlineLarge/Medium/Small`,
`TitleLarge/Medium/Small`,
`BodyLarge/Medium/Small`,
`LabelLarge/Medium/Small`.

Text wider than its box is automatically truncated with `…`.

### TextInput

`TextInput` is a single-line editable field bound to a `*string`:

```go
var name string

// Outline variant (default)
widget.TextInput{
    Value: &name,
    Label: "Full name",   // optional label above the field
    Hint:  "John Smith",  // placeholder
}

// Filled variant
widget.TextInput{
    Value:   &email,
    Hint:    "Email address",
    Variant: widget.TextInputFilled,
}

// Flushed variant (underline only)
widget.TextInput{
    Value:   &search,
    Hint:    "Search…",
    Variant: widget.TextInputFlushed,
}

// Password field
widget.TextInput{
    Value:   &password,
    Hint:    "Password",
    Obscure: true,
}
```

`TextInput` gains keyboard focus when tapped. The `Frame` routes key events
to the focused input automatically.

---

## 8. Buttons and interaction

### Button

`Button` is the single action control configured by three orthogonal fields:

```go
// Solid primary (default)
widget.Button{
    Child:     widget.Text{Content: "Save"},
    OnPressed: func() { /* … */ },
}

// Outline danger button
widget.Button{
    Child:     widget.Text{Content: "Delete"},
    Variant:   widget.ButtonOutline,
    Tone:      widget.ButtonDanger,
    OnPressed: confirmDelete,
}

// Ghost neutral small button
widget.Button{
    Child:   widget.Text{Content: "Cancel"},
    Variant: widget.ButtonGhost,
    Tone:    widget.ButtonNeutral,
    Size:    widget.ButtonSmall,
}

// Large primary button with an icon
widget.Button{
    Child: widget.Row{
        CrossAxisAlignment: layout.CrossCenter,
        Children: []widget.Widget{
            widget.Icon{Name: "download", Size: 18},
            widget.SizedBox{Width: 8},
            widget.Text{Content: "Download"},
        },
    },
    Size: widget.ButtonLarge,
}
```

| `ButtonVariant` | Visual                                     |
|-----------------|--------------------------------------------|
| `ButtonSolid`   | Filled background, elevated                |
| `ButtonOutline` | Border only, transparent background        |
| `ButtonGhost`   | No border, subtle hover fill               |

| `ButtonTone`    | Color palette used                         |
|-----------------|--------------------------------------------|
| `ButtonPrimary` | Brand / teal                               |
| `ButtonDanger`  | Destructive / red                          |
| `ButtonNeutral` | No semantic meaning                        |

| `ButtonSize`    | Padding and min-height                     |
|-----------------|--------------------------------------------|
| `ButtonSmall`   | Compact                                    |
| `ButtonMedium`  | Default                                    |
| `ButtonLarge`   | Spacious                                   |

### GestureDetector

`GestureDetector` wraps any child and responds to pointer events. Use `Child`
for a simple tap target:

```go
widget.GestureDetector{
    OnTap: func() { s.SetState(func() { s.open = !s.open }) },
    Child: widget.Text{Content: "Tap me"},
}
```

Use `Builder` when you need hover and pressed state to drive visuals. The
`Builder` receives live `InteractionState{Hovered, Pressed bool}` every
frame, so you do not need your own pointer-tracking state:

```go
widget.GestureDetector{
    OnTap: handler,
    Builder: func(state widget.InteractionState) widget.Widget {
        fill := ctx.Theme.ColorScheme.BgPanel
        if state.Pressed {
            fill = ctx.Theme.ColorScheme.BgEmphasized
        } else if state.Hovered {
            fill = ctx.Theme.ColorScheme.BgSubtle
        }
        return widget.Container{
            Fill:    fill,
            Radius:  ctx.Theme.Shape.MediumRadius,
            Padding: layout.All(12),
            Child:   content,
        }
    },
}
```

Combine with `Animated` for smooth hover transitions (see [section 12](#12-animation)).

Available callbacks:

| Callback              | When it fires                                     |
|-----------------------|---------------------------------------------------|
| `OnTap`               | Button-up inside widget (click / tap)             |
| `OnTapDown`           | Button-down anywhere on widget                    |
| `OnPointerDown`       | Pointer-down in global coords                     |
| `OnPointerDownLocal`  | Pointer-down in local coords                      |
| `OnPointerUp`         | Pointer-up in global coords                       |
| `OnPointerUpLocal`    | Pointer-up in local coords                        |
| `OnPointerMove`       | Cursor moved (global)                             |
| `OnPointerMoveLocal`  | Cursor moved (local)                              |
| `OnDragMove`          | Drag-move in global coords                        |
| `OnDragEnd`           | Drag released                                     |
| `OnScroll`            | Scroll wheel delta (dx, dy)                       |

---

## 9. Toggles and range controls

### Checkbox

18×18 animated check mark:

```go
widget.Checkbox{
    Value: s.checked,
    OnChanged: func(v bool) {
        s.SetState(func() { s.checked = v })
    },
}
```

### Switch

Sliding track-and-thumb toggle:

```go
widget.Switch{
    Value: s.enabled,
    OnChanged: func(v bool) {
        s.SetState(func() { s.enabled = v })
    },
}
```

### Slider

Horizontal range input. Drag the thumb to call `OnChanged` with a value
in `[Min, Max]`:

```go
widget.Slider{
    Value: s.volume,
    Min:   0,
    Max:   100,
    OnChanged: func(v float64) {
        s.SetState(func() { s.volume = v })
    },
}
```

---

## 10. Scroll views

`Scroll` wraps any child in a scrollable viewport. It manages the scroll
offset internally and renders a `ScrollBar` automatically when content
overflows.

```go
// Vertical scroll (default)
widget.Scroll{
    Child: widget.Column{
        CrossAxisAlignment: layout.CrossStretch,
        Children:           longList,
    },
}

// Horizontal scroll
widget.Scroll{
    Axis:  layout.Horizontal,
    Child: wideContent,
}

// Bidirectional scroll
widget.Scroll{
    Both:  true,
    Child: largeCanvas,
}

// Constrained viewport
widget.Scroll{
    Height: 300,
    Child:  content,
}

// Observe scroll offset
widget.Scroll{
    Child:    content,
    OnScroll: func(pos geom.Point) { fmt.Println(pos.Y) },
}
```

Use a standalone `ScrollBar` when you manage the viewport yourself:

```go
widget.ScrollBar{
    Axis:        layout.Vertical,
    Offset:      s.scrollY,
    ContentSize: s.contentH,
    Viewport:    s.viewH,
    OnThumbDrag: func(v float64) { s.SetState(func() { s.scrollY = v }) },
}
```

---

## 11. Images and icons

### Image

`Image` renders a decoded `image.Image`:

```go
img, err := widget.NewImageFromFilePath("/usr/share/images/logo.jpg")
if err != nil { log.Fatal(err) }

// Contain: preserve aspect ratio, center in box
widget.Image{Source: img.Source}

// Stretch: fill box exactly, ignore aspect ratio
widget.Image{Source: img.Source, Fit: widget.ImageFitStretch}

// Fixed size box
widget.SizedBox{
    Width: 320, Height: 200,
    Child: widget.Image{Source: img.Source},
}
```

### Icon

`Icon` loads a themed PNG from `share/icons/<theme>/<name>.jpg`:

```go
widget.Icon{Name: "folder", Size: 24}

// Explicit theme
widget.Icon{Name: "network-wireless", Theme: "hicolor", Size: 16}

// Custom fallback if the icon is missing
widget.Icon{
    Name:     "unknown-app",
    Size:     32,
    Fallback: widget.Text{Content: "?"},
}
```

Icons are cached after first load. The provider searches `/usr/share/icons/` using
the avyos filesystem resolver.

---

## 12. Animation

`Animated` interpolates a single `float64` target over `Duration` and rebuilds
its `Builder` child each frame until the animation completes:

```go
widget.Animated{
    Value:    boolToFloat(s.open),   // 0.0 → 1.0
    Duration: 180 * time.Millisecond,
    Curve:    widget.EaseInOut,
    Builder: func(v float64) widget.Widget {
        return widget.Opacity{
            Value: v,
            Child: panel,
        }
    },
}
```

On the **first render** the value snaps to `Value` without animation. All
subsequent changes animate from the last rendered value to the new target.

Built-in curves:

| Curve             | Shape                               |
|-------------------|-------------------------------------|
| `widget.Linear`   | Constant speed                      |
| `widget.EaseIn`   | Slow start, fast end                |
| `widget.EaseOut`  | Fast start, slow end                |
| `widget.EaseInOut`| Slow at both ends (default)         |

Custom curve:

```go
Curve: func(t float64) float64 { return t * t * t } // cubic ease-in
```

### Animated hover effects

Combining `GestureDetector.Builder` with `Animated` gives smooth interactive
feedback without any explicit state:

```go
widget.GestureDetector{
    OnTap: handler,
    Builder: func(st widget.InteractionState) widget.Widget {
        target := 0.0
        if st.Hovered { target = 0.6 }
        if st.Pressed { target = 1.0 }
        return widget.Animated{
            Value:    target,
            Duration: ctx.Theme.Motion.Fast,
            Curve:    widget.EaseOut,
            Builder: func(v float64) widget.Widget {
                fill := ctx.Theme.ColorScheme.BgPanel.Lerp(
                    ctx.Theme.ColorScheme.BgEmphasized, v,
                )
                return widget.Container{
                    Fill:    fill,
                    Radius:  ctx.Theme.Shape.MediumRadius,
                    Padding: layout.All(12),
                    Child:   label,
                }
            },
        }
    },
}
```

---

## 13. Collections: application shell

The `collections` package provides higher-level components built from widget
primitives. Import it as:

```go
import "avyos.dev/lib/graphics/collections"
```

### Scaffold

`Scaffold` is the responsive application shell. It composes the major
structural regions and adapts their arrangement to screen width:

| Layout mode      | Width          | Structure                                    |
|------------------|----------------|----------------------------------------------|
| `LayoutCompact`  | < 600 px       | AppBar + Body + BottomNav (optional); Drawer |
| `LayoutMedium`   | 600–999 px     | AppBar + Body + BottomNav; NavBar hidden      |
| `LayoutExpanded` | ≥ 1000 px      | AppBar + NavBar (persistent) + Body          |

```go
collections.Scaffold{
    AppBar:    &collections.AppBar{Title: "Settings"},
    NavBar:    &collections.NavBar{
        Destinations: []collections.NavDestination{
            {Label: "General",  Icon: "settings"},
            {Label: "Network",  Icon: "network-wireless"},
            {Label: "Display",  Icon: "display"},
        },
        Selected:   s.page,
        OnSelected: func(i int) { s.SetState(func() { s.page = i }) },
    },
    BottomNav: &collections.BottomNav{
        Destinations: []collections.NavDestination{
            {Label: "Home",     Icon: "home"},
            {Label: "Search",   Icon: "search"},
            {Label: "Profile",  Icon: "person"},
        },
        Selected:   s.tab,
        OnSelected: func(i int) { s.SetState(func() { s.tab = i }) },
    },
    Body: bodyWidget,
}
```

### AppBar

`AppBar` renders a top bar with a leading widget, title, and action buttons:

```go
collections.AppBar{
    Title: "My App",
}

// With a hamburger button, custom title widget, and actions
collections.AppBar{
    Leading: widget.GestureDetector{
        OnTap:  s.drawer.Toggle,
        Child:  widget.Icon{Name: "menu", Size: 24},
    },
    TitleWidget: widget.Row{
        CrossAxisAlignment: layout.CrossCenter,
        Children: []widget.Widget{
            widget.Icon{Name: "logo", Size: 20},
            widget.SizedBox{Width: 8},
            widget.Text{Content: "avyos"},
        },
    },
    Actions: []widget.Widget{
        widget.GestureDetector{
            OnTap:  openSearch,
            Child:  widget.Icon{Name: "search", Size: 20},
        },
        widget.GestureDetector{
            OnTap:  openMenu,
            Child:  widget.Icon{Name: "more-vertical", Size: 20},
        },
    },
    // Bottom slot: search bar, tab row, etc.
    Bottom: searchBar,
}
```

### NavBar

Persistent vertical sidebar for desktop layouts:

```go
collections.NavBar{
    Destinations: []collections.NavDestination{
        {Label: "Dashboard", Icon: "home"},
        {Label: "Projects",  Icon: "folder"},
        {Label: "Settings",  Icon: "settings"},
    },
    Selected:   s.page,
    OnSelected: func(i int) { s.SetState(func() { s.page = i }) },
    Header: widget.Padding{
        Insets: layout.All(16),
        Child:  widget.Text{Content: "MyApp"},
    },
    Footer: widget.GestureDetector{
        OnTap: logout,
        Child: widget.Text{Content: "Sign out"},
    },
    Compact: false,   // true = icon-only mode
}
```

### Drawer

`DrawerController` manages open/close state and is safe to pass around:

```go
// In state:
drawerCtrl := collections.NewDrawerController()

// In AppBar leading:
widget.GestureDetector{
    OnTap: drawerCtrl.Toggle,
    Child: widget.Icon{Name: "menu", Size: 24},
}

// In Scaffold:
collections.Scaffold{
    AppBar: &collections.AppBar{
        Leading: toggleBtn,
        Title:   "Docs",
    },
    Drawer: &collections.DrawerConfig{
        Controller: drawerCtrl,
        Width:      280,
        Child:      navList,
    },
    Body: content,
}
```

Tapping the scrim outside the panel closes the drawer automatically.

### Card and Section

`Card` is a rounded elevated surface:

```go
collections.Card{
    Child: widget.Column{
        CrossAxisAlignment: layout.CrossStretch,
        Children: []widget.Widget{
            widget.Text{Content: "Card title"},
            widget.SizedBox{Height: 4},
            widget.Text{Content: "Subtitle"},
        },
    },
}

// Raised card: drop shadow instead of border
collections.Card{Raised: true, Child: content}
```

`Section` renders a labeled group with an optional trailing action:

```go
collections.Section{
    Title: "Notifications",
    Action: widget.Button{
        Child:   widget.Text{Content: "Clear all"},
        Variant: widget.ButtonGhost,
        Tone:    widget.ButtonNeutral,
        Size:    widget.ButtonSmall,
    },
    Child: notificationList,
}
```

### FAB

Floating action button. Scaffold places it at the bottom-right of `Body`:

```go
collections.Scaffold{
    FAB: &collections.FAB{
        Icon:      "add",
        OnPressed: createNewItem,
    },
    Body: content,
}

// Extended FAB with a label
collections.FAB{
    Icon:      "add",
    Label:     "New file",
    OnPressed: createFile,
}
```

---

## 14. Collections: overlays, dialogs, popups

All overlay-based components share a single `OverlayManager`. Create one in
your state and pass it to `Scaffold.Overlay`:

```go
type AppState struct {
    widget.StateBase
    overlay *collections.OverlayManager
    dialog  *collections.DialogController
    popup   *collections.PopupMenuController
}

func (s *AppState) InitState() {
    s.overlay = collections.NewOverlayManager()
    s.dialog  = collections.NewDialogController(s.overlay)
    s.popup   = collections.NewPopupMenuController(s.overlay)
}

func (s *AppState) Build(ctx widget.BuildContext) widget.Widget {
    return collections.Scaffold{
        Overlay: s.overlay,
        Body:    s.buildBody(ctx),
    }
}
```

### Dialog

```go
close := s.dialog.Show(collections.Dialog{
    Title: "Delete file?",
    Body:  widget.Text{Content: "This action cannot be undone."},
    Actions: []widget.Widget{
        widget.Button{
            Child:   widget.Text{Content: "Cancel"},
            Variant: widget.ButtonGhost,
            Tone:    widget.ButtonNeutral,
            OnPressed: func() { close() },
        },
        widget.Button{
            Child:     widget.Text{Content: "Delete"},
            Tone:      widget.ButtonDanger,
            OnPressed: func() { close(); s.deleteFile() },
        },
    },
})
```

Tapping the modal scrim does **not** auto-close the dialog — your action
buttons must call the returned `close` function.

### Popup menu

`PopupMenuController.Show` takes a slice of `MenuItem` and an anchor rectangle
(the button's screen position):

```go
var anchorRect geom.Rect  // captured from a GestureDetector.OnPointerDown

s.popup.Show([]collections.MenuItem{
    {Label: "Copy",   Icon: "copy",   OnTap: s.copy},
    {Label: "Paste",  Icon: "paste",  OnTap: s.paste},
    {Divider: true},
    {Label: "Delete", Icon: "delete", OnTap: s.delete, Disabled: !s.canDelete},
}, anchorRect)
```

Tapping outside the menu closes it automatically.

### Overlay entries (manual)

For custom overlay content (tooltips, drop-down panels, etc.) use
`OverlayManager` directly:

```go
entry := &collections.OverlayEntry{
    Modal: false,
    Z:     10,   // draw on top of other overlays
    Builder: func(ctx widget.BuildContext) widget.Widget {
        return widget.Positioned{
            Top:  widget.Ptr(100.0),
            Left: widget.Ptr(200.0),
            Child: widget.Container{
                Fill:    ctx.Theme.ColorScheme.BgPanel,
                Radius:  ctx.Theme.Shape.LargeRadius,
                Padding: layout.All(12),
                Child:   widget.Text{Content: "Tooltip"},
            },
        }
    },
}
s.overlay.Insert(entry)

// Remove when done
entry.Remove()
```

---

## 15. Collections: toasts and panels

### Toast notifications

`ToastController` manages a queue of non-blocking banners. Add it to
`Scaffold.Toast`:

```go
type AppState struct {
    widget.StateBase
    toast *collections.ToastController
}

func (s *AppState) InitState() {
    s.toast = collections.NewToastController()
}

func (s *AppState) Build(ctx widget.BuildContext) widget.Widget {
    return collections.Scaffold{
        Toast: s.toast,
        Body:  s.buildBody(ctx),
    }
}
```

Show toasts from anywhere:

```go
// Auto-dismiss after 3 seconds
s.toast.ShowFor("File saved", collections.ToastDefault, 3*time.Second)

// Manual dismiss
dismiss := s.toast.Show("Upload failed", collections.ToastError)
// later:
dismiss()
```

Available variants: `ToastDefault`, `ToastSuccess`, `ToastWarning`,
`ToastError`, `ToastInfo`.

### Panel controller

`PanelController` manages one exclusive overlay panel at a time — toggling a
panel closed when another is opened. This is the pattern used by the desktop
shelf for the launcher and quick-settings panel:

```go
panels := collections.NewPanelController(s.overlay)
panels.SetNotify(func() { s.SetState(nil) }) // rebuild on open/close

// Toggle a panel open/closed:
panels.Toggle("launcher", func() widget.Widget {
    return widget.Positioned{
        Bottom: widget.Ptr(48.0),   // above the shelf
        Left:   widget.Ptr(0.0),
        Right:  widget.Ptr(0.0),
        Child:  LauncherPanel{OnClose: panels.Close},
    }
})

// Check if open (e.g., to highlight a shelf button):
isOpen := panels.IsOpen("launcher")

// Close current panel:
panels.Close()
```

---

## 16. Responsive layout

### Breakpoint-based switching

`BreakpointLayout` selects one of three widgets based on screen width:

```go
collections.BreakpointLayout{
    Compact:  mobileContent,    // < 600 px
    Medium:   tabletContent,    // 600–999 px
    Expanded: desktopContent,   // ≥ 1000 px
}
```

### Master–detail split

`SplitLayout` renders a fixed-width primary panel and an expanded secondary
panel. On narrow screens only the secondary is shown:

```go
collections.SplitLayout{
    Primary:      listPanel,
    Secondary:    detailPanel,
    PrimaryWidth: 280,   // left column width
    BreakWidth:   600,   // hide primary below this
}
```

### Reading screen size

`ctx.ScreenSize` is available inside any `Build` method:

```go
func (w MyWidget) Build(ctx widget.BuildContext) widget.Widget {
    if ctx.ScreenSize.Width < 600 {
        return compactLayout
    }
    return fullLayout
}
```

---

## 17. Custom widgets

### Buildable (stateless composition)

```go
type Pill struct {
    Label  string
    Accent color.Color
}

func (p Pill) Build(ctx widget.BuildContext) widget.Widget {
    labelSt := ctx.Theme.TextTheme.LabelSmall
    labelSt.Color = ctx.Theme.ColorScheme.FgInverted
    return widget.Container{
        Fill:    p.Accent,
        Radius:  999,
        Padding: layout.Symmetric(12, 4),
        Child:   widget.Text{Content: p.Label, Style: &labelSt},
    }
}
```

### StatefulWidget (mutable state)

```go
type Expandable struct {
    Title   string
    Content widget.Widget
}

func (Expandable) CreateState() widget.State { return &expandableState{} }

type expandableState struct {
    widget.StateBase
    widget Expandable
    open   bool
}

func (s *expandableState) UpdateWidget(w widget.Widget) {
    if v, ok := w.(Expandable); ok {
        s.widget = v
    }
}

func (s *expandableState) Build(ctx widget.BuildContext) widget.Widget {
    th := ctx.Theme
    header := widget.GestureDetector{
        OnTap: func() { s.SetState(func() { s.open = !s.open }) },
        Builder: func(state widget.InteractionState) widget.Widget {
            icon := "chevron-right"
            if s.open { icon = "chevron-down" }
            fill := th.ColorScheme.BgPanel
            if state.Hovered { fill = th.ColorScheme.BgSubtle }
            return widget.Container{
                Fill:    fill,
                Padding: layout.Symmetric(16, 10),
                Child: widget.Row{
                    CrossAxisAlignment: layout.CrossCenter,
                    Children: []widget.Widget{
                        widget.Expanded{Child: widget.Text{Content: s.widget.Title}},
                        widget.Icon{Name: icon, Size: 16},
                    },
                },
            }
        },
    }

    if !s.open {
        return header
    }
    return widget.Column{
        CrossAxisAlignment: layout.CrossStretch,
        Children: []widget.Widget{
            header,
            widget.Padding{
                Insets: layout.LTRB(16, 0, 16, 12),
                Child:  s.widget.Content,
            },
        },
    }
}
```

### RenderBox (custom leaf with paint)

Use `RenderBox` for a leaf widget that needs full control over measurement and
painting. Implement three methods:

```go
type ProgressRing struct {
    Value float64   // 0–1
    Color color.Color
    Size  float64
}

func (r ProgressRing) Layout(c layout.BoxConstraints) geom.Size {
    side := r.Size
    if side <= 0 { side = 40 }
    return c.Constrain(geom.Sz(side, side))
}

func (r ProgressRing) Paint(ctx *paint.Context, offset geom.Point, size geom.Size) {
    cx := offset.X + size.Width/2
    cy := offset.Y + size.Height/2
    radius := size.Width/2 - 3

    // Track
    ctx.Canvas.SetStrokeColor(r.Color.WithAlpha(0.2))
    ctx.Canvas.SetLineWidth(3)
    ctx.Canvas.StrokeCircle(geom.Pt(cx, cy), radius)

    // Fill arc via path
    if r.Value > 0 {
        ctx.Canvas.SetStrokeColor(r.Color)
        ctx.Canvas.SetLineWidth(3)
        ctx.Canvas.BeginPath()
        // draw arc proportional to Value
        end := -math.Pi/2 + r.Value*2*math.Pi
        ctx.Canvas.ArcTo(geom.Pt(cx, cy), radius, radius,
            -math.Pi/2, end, false)
        ctx.Canvas.Stroke()
    }
}

func (r ProgressRing) HitTest(pos, offset geom.Point, size geom.Size) bool {
    return geom.NewRect(offset.X, offset.Y, size.Width, size.Height).Contains(pos)
}
```

### MultiChild (custom parent)

Implement `RenderChildren` when you need to control how children are measured
and placed:

```go
type Masonry struct {
    Columns int
    Gap     float64
    Children []widget.Widget
}

func (m Masonry) RenderChildren(
    c layout.BoxConstraints,
    pctx *paint.Context,
    offset geom.Point,
    cr widget.ChildRenderer,
) geom.Size {
    // measure and place children in a masonry grid
    // cr.Measure(child, constraints, path) — measure only
    // cr.Render(child, constraints, offset, path) — measure + paint
    // …
    return c.Constrain(totalSize)
}
```

---

## 18. Lifecycle hooks

These package-level variables in `app` let you intercept the render loop from
outside the widget tree:

```go
// Called at the start of each event iteration, before polling events.
// Use to drain background work queues (timers, channels, etc.)
app.BeforeEvents = func() {
    processIncomingMessages()
}

// Called after each frame is presented.
app.AfterFrame = func(f *widget.Frame) {
    frameCounter++
}

// Called before each render. Return a non-empty rectangle to restrict
// clearing and painting to only those pixels. Return empty for full repaint.
app.DamageProvider = func() image.Rectangle {
    return s.dirtyRect   // only blit the changed region
}

// Override all event handling (call app.DefaultHandler for built-ins).
app.EventHandler = func(e event.Event) {
    if ke, ok := e.(event.KeyEvent); ok && ke.Down {
        handleGlobalShortcut(ke)
    }
    app.DefaultHandler(e)
}
```

Stop the app from any goroutine:

```go
app.Stop()
```

Inject synthetic events (e.g., for testing):

```go
app.AddEvent(event.CloseEvent{})
```

---

## Quick-reference card

| Task                                 | What to use                                        |
|--------------------------------------|----------------------------------------------------|
| Open a window                        | `app.Options.*` then `app.Run(root)`               |
| Stateless composition                | Struct with `Build(ctx) Widget`                    |
| Mutable UI                           | `StatefulWidget` + `StateBase` + `SetState`        |
| Flex layout                          | `widget.Row` / `widget.Column`                     |
| Absolute positioning                 | `widget.Stack` + `widget.Positioned`               |
| Uniform spacing / gap                | `widget.Flex{Gap: 12}` or `widget.SizedBox`        |
| Scrollable list                      | `widget.Scroll{Child: column}`                     |
| Application shell                    | `collections.Scaffold`                             |
| Top bar                              | `collections.AppBar`                               |
| Sidebar navigation                   | `collections.NavBar`                               |
| Mobile navigation                    | `collections.BottomNav`                            |
| Slide-in panel                       | `collections.DrawerConfig` + `DrawerController`    |
| Elevated surface                     | `collections.Card`                                 |
| Labeled group                        | `collections.Section`                              |
| Primary action button                | `collections.FAB`                                  |
| Modal dialog                         | `collections.DialogController.Show`                |
| Contextual menu                      | `collections.PopupMenuController.Show`             |
| Non-blocking notifications           | `collections.ToastController.ShowFor`              |
| Exclusive overlay panel              | `collections.PanelController.Toggle`               |
| Responsive width switching           | `collections.BreakpointLayout`                     |
| Master–detail split                  | `collections.SplitLayout`                          |
| Animated value                       | `widget.Animated{Value, Duration, Curve, Builder}` |
| Fade in/out                          | `widget.Opacity{Value: 0–1}`                       |
| Hover + pressed visuals              | `GestureDetector{Builder: …}` + `Animated`         |
| Custom leaf paint                    | Implement `RenderBox`                              |
| Custom parent layout                 | Implement `RenderChildren`                         |
| Per-frame damage hint                | `app.DamageProvider`                               |
| Post-frame hook                      | `app.AfterFrame`                                   |
