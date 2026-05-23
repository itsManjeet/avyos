package main

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"time"

	"avyos.dev/pkg/graphics/collections"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/geom"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

var demoSections = []collections.NavDestination{
	{Label: "Overview"},
	{Label: "Typography"},
	{Label: "Inputs"},
	{Label: "Layout"},
	{Label: "Motion"},
	{Label: "Scroll"},
	{Label: "Collections"},
}

type ShowcaseApp struct{}

func (ShowcaseApp) CreateState() widget.State { return &ShowcaseState{} }

type ShowcaseState struct {
	widget.StateBase

	section int
	screen  geom.Size

	app *collections.ApplicationController

	name     string
	query    string
	password string
	notes    string

	checkAlpha bool
	checkBeta  bool
	switchLive bool
	switchSync bool

	sliderPrimary   float64
	sliderSecondary float64
	scrollX         float64
	scrollY         float64
	scrollBarOffset float64

	gestureLog string
	hoverTile  bool
	pressTile  bool
	pulseOn    bool

	scrollContent  geom.Size
	scrollViewport geom.Size
}

func (s *ShowcaseState) InitState() {
	s.app = collections.NewApplicationController()

	s.name = "Ava Systems"
	s.query = "command palette"
	s.password = "demo-pass"
	s.notes = "Drag, scroll, resize, and open overlays to exercise the full surface."
	s.checkAlpha = true
	s.switchLive = true
	s.switchSync = true
	s.sliderPrimary = 64
	s.sliderSecondary = 28
	s.scrollX = 60
	s.scrollY = 48
	s.scrollBarOffset = 140
	s.gestureLog = "Hover or drag inside the interaction tile."
}

func (s *ShowcaseState) Build(ctx widget.BuildContext) widget.Widget {
	s.screen = ctx.ScreenSize

	var leading widget.Widget
	if s.app.CanPop() {
		leading = s.appBarButton("Back", func() { s.app.Pop() })
	}

	appBar := collections.AppBar{
		TitleWidget: s.brandLockup(ctx),
		Leading:     leading,
		Actions: []widget.Widget{
			s.appBarButton("Toast", func() {
				s.app.ShowToastFor("Application controller toast host is active.", collections.ToastInfo, 4*time.Second)
			}),
			s.appBarButton("Panel", s.toggleInspectorPanel),
			s.appBarButton("Menu", s.showActionMenu),
		},
		Bottom: widget.TextInput{
			Value:   &s.query,
			Hint:    "Search the demo surface",
			Variant: widget.TextInputFilled,
		},
	}

	nav := collections.NavBar{
		Destinations: demoSections,
		Selected:     s.section,
		OnSelected:   s.selectSection,
		Header:       s.navHeader(ctx),
		Footer: widget.Text{
			Content: "Application switches between sidebar and bottom nav by breakpoint.",
			Style:   mutedStyle(ctx),
		},
	}

	fab := collections.FAB{
		Label: "Open dialog",
		OnPressed: func() {
			s.openSampleDialog()
		},
	}

	return collections.Application{
		Controller: s.app,
		AppBar:     &appBar,
		NavBar:     &nav,
		StatusBar:  s.buildStatusBar(ctx),
		FAB:        &fab,
		Body: widget.Scroll{
			Axis: layout.Vertical,
			Child: widget.Padding{
				Insets: layout.All(ctx.Theme.Space.Unit(5)),
				Child:  s.buildPage(ctx),
			},
		},
	}
}

func (s *ShowcaseState) selectSection(i int) {
	s.SetState(func() { s.section = i })
}

func (s *ShowcaseState) buildPage(ctx widget.BuildContext) widget.Widget {
	switch s.section {
	case 1:
		return s.buildTypographyPage(ctx)
	case 2:
		return s.buildInputsPage(ctx)
	case 3:
		return s.buildLayoutPage(ctx)
	case 4:
		return s.buildMotionPage(ctx)
	case 5:
		return s.buildScrollPage(ctx)
	case 6:
		return s.buildCollectionsPage(ctx)
	default:
		return s.buildOverviewPage(ctx)
	}
}

func (s *ShowcaseState) buildOverviewPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHero(
				"Graphics demo rebuilt as a full surface map",
				"The app shell now uses the collection layer, and each page drills into the lower-level widget primitives instead of hiding them behind a single showcase wall.",
				ctx,
			),
			widget.SizedBox{Height: th.Space.Unit(5)},
			widget.Grid{
				MinChildWidth: 260,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.metricCard("Widgets", "Text, input, layout, effects, animation, scrolling.", ctx),
					s.metricCard("Collections", "Application, nav, drawer, dialog, popup, toast, panel.", ctx),
					s.metricCard("Responsive", "Desktop sidebar, mobile bottom nav, drawer and split layouts.", ctx),
					s.metricCard("Interactive", "Gesture logging, animated states, drag targets, overlay actions.", ctx),
				},
			},
			widget.SizedBox{Height: th.Space.Unit(5)},
			s.surfaceCard("What to try", widget.Wrap{
				Spacing:    th.Space.Unit(2),
				RunSpacing: th.Space.Unit(2),
				Children: []widget.Widget{
					s.tag("Open the drawer", ctx),
					s.tag("Use the app bar search field", ctx),
					s.tag("Trigger popup, panel, dialog, toast", ctx),
					s.tag("Drag the manual scroll bars", ctx),
					s.tag("Resize the window for breakpoint helpers", ctx),
					s.tag("Inspect each widget page", ctx),
				},
			}, ctx),
			widget.SizedBox{Height: th.Space.Unit(5)},
			s.surfaceCard("Media", widget.Row{
				CrossAxisAlignment: layout.CrossCenter,
				Children: []widget.Widget{
					widget.AspectRatio{
						Ratio: 1,
						Child: widget.Container{
							Radius:      th.Shape.XLargeRadius,
							Border:      th.ColorScheme.Outline,
							BorderWidth: 1,
							Child: widget.Image{
								Source: demoImage(),
								Fit:    widget.ImageFitContain,
							},
						},
					},
					widget.SizedBox{Width: th.Space.Unit(4)},
					widget.Expanded{
						Child: widget.Column{
							CrossAxisAlignment: layout.CrossStart,
							Children: []widget.Widget{
								widget.Text{Content: "Image, AspectRatio, Container, and Text in one card.", Style: bodyStyle(ctx)},
								widget.SizedBox{Height: th.Space.Unit(3)},
								widget.Text{Content: "The image is generated locally at runtime so the demo stays self-contained.", Style: mutedStyle(ctx)},
							},
						},
					},
				},
			}, ctx),
		},
	}
}

func (s *ShowcaseState) buildTypographyPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	titleLarge := th.TextTheme.TitleLarge
	titleMedium := th.TextTheme.TitleMedium
	titleSmall := th.TextTheme.TitleSmall
	bodyLarge := th.TextTheme.BodyLarge
	bodyMedium := th.TextTheme.BodyMedium
	bodySmall := th.TextTheme.BodySmall
	labelLarge := th.TextTheme.LabelLarge
	labelMedium := th.TextTheme.LabelMedium
	labelSmall := th.TextTheme.LabelSmall

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Typography", "Text, label hierarchy, icon fallback, images, separators, and opacity.", ctx),
			widget.Grid{
				MinChildWidth: 320,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.surfaceCard("Type scale", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: "Title Large", Style: &titleLarge},
							widget.Text{Content: "Title Medium", Style: &titleMedium},
							widget.Text{Content: "Title Small", Style: &titleSmall},
							widget.SizedBox{Height: th.Space.Unit(2)},
							widget.Text{Content: "Body Large for long-form explanation blocks.", Style: &bodyLarge},
							widget.Text{Content: "Body Medium is the default text style.", Style: &bodyMedium},
							widget.Text{Content: "Body Small works for metadata and supporting notes.", Style: &bodySmall},
							widget.SizedBox{Height: th.Space.Unit(2)},
							widget.Text{Content: "Label Large", Style: &labelLarge},
							widget.Text{Content: "Label Medium", Style: &labelMedium},
							widget.Text{Content: "Label Small", Style: &labelSmall},
						},
					}, ctx),
					s.surfaceCard("Icons and image fits", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Row{
								CrossAxisAlignment: layout.CrossCenter,
								Children: []widget.Widget{
									widget.Icon{Name: "folder", Size: 24, Fallback: fallbackIcon("A", ctx)},
									widget.SizedBox{Width: th.Space.Unit(2)},
									widget.Icon{Name: "settings", Size: 24, Fallback: fallbackIcon("V", ctx)},
									widget.SizedBox{Width: th.Space.Unit(2)},
									widget.Icon{Name: "search", Size: 24, Fallback: fallbackIcon("Y", ctx)},
								},
							},
							widget.SizedBox{Height: th.Space.Unit(4)},
							widget.Row{
								CrossAxisAlignment: layout.CrossCenter,
								Children: []widget.Widget{
									widget.Container{
										Width:       96,
										Height:      72,
										Fill:        th.ColorScheme.SurfaceVariant,
										Radius:      th.Shape.MediumRadius,
										Border:      th.ColorScheme.Outline,
										BorderWidth: 1,
										Child:       widget.Image{Source: demoImage(), Fit: widget.ImageFitContain},
									},
									widget.SizedBox{Width: th.Space.Unit(3)},
									widget.Container{
										Width:       96,
										Height:      72,
										Fill:        th.ColorScheme.SurfaceVariant,
										Radius:      th.Shape.MediumRadius,
										Border:      th.ColorScheme.Outline,
										BorderWidth: 1,
										Child:       widget.Image{Source: demoImage(), Fit: widget.ImageFitStretch},
									},
								},
							},
						},
					}, ctx),
					s.surfaceCard("Separator and opacity", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: "Opacity layers are approximated with a translucent overlay.", Style: bodyStyle(ctx)},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Row{
								CrossAxisAlignment: layout.CrossCenter,
								Children: []widget.Widget{
									opacitySwatch(1.0, ctx),
									widget.SizedBox{Width: th.Space.Unit(2)},
									opacitySwatch(0.66, ctx),
									widget.SizedBox{Width: th.Space.Unit(2)},
									opacitySwatch(0.33, ctx),
								},
							},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Separator{},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Row{
								CrossAxisAlignment: layout.CrossCenter,
								Children: []widget.Widget{
									widget.Text{Content: "left", Style: bodyStyle(ctx)},
									widget.SizedBox{Width: th.Space.Unit(3)},
									widget.Separator{Axis: widget.SeparatorVertical, Length: 24},
									widget.SizedBox{Width: th.Space.Unit(3)},
									widget.Text{Content: "right", Style: bodyStyle(ctx)},
								},
							},
						},
					}, ctx),
				},
			},
		},
	}
}

func (s *ShowcaseState) buildInputsPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Inputs", "Buttons, toggles, sliders, text fields, and direct gesture instrumentation.", ctx),
			widget.Grid{
				MinChildWidth: 360,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.surfaceCard("Buttons", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Wrap{
								Spacing:    th.Space.Unit(2),
								RunSpacing: th.Space.Unit(2),
								Children: []widget.Widget{
									actionButton("Primary", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonMedium, func() {
										s.app.ShowToastFor("Primary action fired.", collections.ToastSuccess, 3*time.Second)
									}),
									actionButton("Outline", widget.ButtonOutline, widget.ButtonPrimary, widget.ButtonMedium, nil),
									actionButton("Ghost", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, nil),
									actionButton("Danger", widget.ButtonOutline, widget.ButtonDanger, widget.ButtonMedium, nil),
									actionButton("Small", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonSmall, nil),
									actionButton("Large", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonLarge, nil),
								},
							},
						},
					}, ctx),
					s.surfaceCard("Checkbox and switch", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							s.controlRow("Enable alpha branch", widget.Checkbox{
								Value:     s.checkAlpha,
								OnChanged: func(v bool) { s.SetState(func() { s.checkAlpha = v }) },
							}, ctx),
							widget.SizedBox{Height: th.Space.Unit(3)},
							s.controlRow("Pin beta release", widget.Checkbox{
								Value:     s.checkBeta,
								OnChanged: func(v bool) { s.SetState(func() { s.checkBeta = v }) },
							}, ctx),
							widget.SizedBox{Height: th.Space.Unit(3)},
							s.controlRow("Live updates", widget.Switch{
								Value:     s.switchLive,
								OnChanged: func(v bool) { s.SetState(func() { s.switchLive = v }) },
							}, ctx),
							widget.SizedBox{Height: th.Space.Unit(3)},
							s.controlRow("Background sync", widget.Switch{
								Value:     s.switchSync,
								OnChanged: func(v bool) { s.SetState(func() { s.switchSync = v }) },
							}, ctx),
						},
					}, ctx),
					s.surfaceCard("Sliders", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: fmt.Sprintf("Primary %.0f%%", s.sliderPrimary), Style: bodyStyle(ctx)},
							widget.Slider{
								Value:     s.sliderPrimary,
								Min:       0,
								Max:       100,
								OnChanged: func(v float64) { s.SetState(func() { s.sliderPrimary = v }) },
							},
							widget.SizedBox{Height: th.Space.Unit(4)},
							widget.Text{Content: fmt.Sprintf("Secondary %.0f%%", s.sliderSecondary), Style: bodyStyle(ctx)},
							widget.Slider{
								Value:     s.sliderSecondary,
								Min:       0,
								Max:       100,
								OnChanged: func(v float64) { s.SetState(func() { s.sliderSecondary = v }) },
							},
						},
					}, ctx),
					s.surfaceCard("TextInput variants", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.TextInput{Value: &s.name, Label: "Name", Hint: "Project name"},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.TextInput{Value: &s.query, Label: "Filled", Hint: "Filter", Variant: widget.TextInputFilled},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.TextInput{Value: &s.password, Label: "Password", Hint: "Secret", Obscure: true, Variant: widget.TextInputFlushed},
						},
					}, ctx),
					s.surfaceCard("GestureDetector", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.GestureDetector{
								OnTap: func() {
									s.SetState(func() { s.gestureLog = "Tap detected." })
								},
								OnHoverChanged: func(v bool) {
									s.SetState(func() {
										s.hoverTile = v
										s.gestureLog = fmt.Sprintf("Hover changed: %t", v)
									})
								},
								OnPressChanged: func(v bool) {
									s.SetState(func() {
										s.pressTile = v
										s.gestureLog = fmt.Sprintf("Pressed: %t", v)
									})
								},
								OnPointerMoveLocal: func(p geom.Point) {
									s.SetState(func() {
										s.gestureLog = fmt.Sprintf("Local pointer %.0f, %.0f", p.X, p.Y)
									})
								},
								Builder: func(state widget.InteractionState) widget.Widget {
									fill := ctx.Theme.ColorScheme.SurfaceContainer
									if state.Hovered {
										fill = ctx.Theme.ColorScheme.PrimaryContainer
									}
									if state.Pressed {
										fill = ctx.Theme.ColorScheme.PrimaryContainer
									}
									return widget.Container{
										Height:      140,
										Fill:        fill,
										Radius:      ctx.Theme.Shape.LargeRadius,
										Border:      ctx.Theme.ColorScheme.Outline,
										BorderWidth: 1,
										Child:       widget.Center(widget.Text{Content: "Interaction tile", Style: bodyStyle(ctx)}),
									}
								},
							},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Text{Content: s.gestureLog, Style: mutedStyle(ctx)},
						},
					}, ctx),
				},
			},
		},
	}
}

func (s *ShowcaseState) buildLayoutPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Layout", "Row, Column, Flex, Grid, Wrap, Stack, Align, Splitter, AspectRatio, Bleed, and box primitives.", ctx),
			widget.Grid{
				MinChildWidth: 340,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.surfaceCard("Row, Column, Expanded, Spacer", widget.Row{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							colorBox(ctx.Theme.ColorScheme.Primary, 56, 56),
							widget.SizedBox{Width: th.Space.Unit(2)},
							widget.Expanded{
								Child: widget.Column{
									CrossAxisAlignment: layout.CrossStretch,
									Children: []widget.Widget{
										colorBox(ctx.Theme.ColorScheme.Secondary, 0, 24),
										widget.SizedBox{Height: th.Space.Unit(2)},
										colorBox(ctx.Theme.ColorScheme.Warning, 0, 36),
									},
								},
							},
							widget.Spacer{},
							colorBox(ctx.Theme.ColorScheme.Error, 44, 80),
						},
					}, ctx),
					s.surfaceCard("Flex and Wrap", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Flex{
								Direction:          layout.Horizontal,
								Gap:                th.Space.Unit(2),
								CrossAxisAlignment: layout.CrossCenter,
								Children: []widget.Widget{
									colorBox(ctx.Theme.ColorScheme.PrimaryContainer, 48, 32),
									colorBox(ctx.Theme.ColorScheme.SecondaryContainer, 72, 32),
									colorBox(ctx.Theme.ColorScheme.InfoContainer, 54, 32),
								},
							},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Wrap{
								Spacing:    th.Space.Unit(2),
								RunSpacing: th.Space.Unit(2),
								Children: []widget.Widget{
									s.tag("Wrap", ctx),
									s.tag("keeps", ctx),
									s.tag("flowing", ctx),
									s.tag("when", ctx),
									s.tag("width", ctx),
									s.tag("changes", ctx),
								},
							},
						},
					}, ctx),
					s.surfaceCard("Grid", widget.Grid{
						Columns: 3,
						Gap:     th.Space.Unit(2),
						Children: []widget.Widget{
							colorBox(ctx.Theme.ColorScheme.PrimaryContainer, 0, 54),
							colorBox(ctx.Theme.ColorScheme.SecondaryContainer, 0, 42),
							colorBox(ctx.Theme.ColorScheme.InfoContainer, 0, 64),
							colorBox(ctx.Theme.ColorScheme.WarningContainer, 0, 48),
							colorBox(ctx.Theme.ColorScheme.ErrorContainer, 0, 70),
							colorBox(ctx.Theme.ColorScheme.SuccessContainer, 0, 40),
						},
					}, ctx),
					s.surfaceCard("Stack, Positioned, Align, Center", widget.Stack{
						Children: []widget.Widget{
							widget.Container{
								Height:      170,
								Fill:        ctx.Theme.ColorScheme.SurfaceContainer,
								Radius:      ctx.Theme.Shape.XLargeRadius,
								Border:      ctx.Theme.ColorScheme.Outline,
								BorderWidth: 1,
							},
							widget.Align{
								Alignment: layout.AlignTopLeft,
								Child:     badgePill("Top Left", ctx),
							},
							widget.Align{
								Alignment: layout.AlignCenter,
								Child:     badgePill("Center", ctx),
							},
							widget.Positioned{
								Right:  widget.Ptr(12),
								Bottom: widget.Ptr(12),
								Child:  badgePill("Positioned", ctx),
							},
						},
					}, ctx),
					s.surfaceCard("AspectRatio and Bleed", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.AspectRatio{
								Ratio: 16.0 / 9.0,
								Child: widget.Bleed{
									Insets: layout.Symmetric(12, 12),
									Child: widget.Container{
										Fill:        ctx.Theme.ColorScheme.PrimaryContainer,
										Border:      ctx.Theme.ColorScheme.Primary,
										BorderWidth: 1,
										Radius:      ctx.Theme.Shape.XLargeRadius,
										Child:       widget.Center(widget.Text{Content: "Bleed beyond box", Style: bodyStyle(ctx)}),
									},
								},
							},
						},
					}, ctx),
					s.surfaceCard("Padding, SizedBox, Container", widget.Container{
						Fill:          ctx.Theme.ColorScheme.SurfaceVariant,
						Radius:        ctx.Theme.Shape.LargeRadius,
						Border:        ctx.Theme.ColorScheme.Outline,
						BorderWidth:   1,
						Shadow:        ctx.Theme.ColorScheme.Shadow,
						ShadowBlur:    ctx.Theme.Shadow.SM.Blur,
						ShadowOffsetY: ctx.Theme.Shadow.SM.OffsetY,
						Padding:       layout.All(th.Space.Unit(4)),
						Child: widget.Row{
							CrossAxisAlignment: layout.CrossCenter,
							Children: []widget.Widget{
								widget.SizedBox{Width: 28, Height: 28, Child: colorBox(ctx.Theme.ColorScheme.Primary, 0, 0)},
								widget.SizedBox{Width: th.Space.Unit(3)},
								widget.Text{Content: "Container handles fill, border, shadow, glow, size, and padding.", Style: bodyStyle(ctx)},
							},
						},
					}, ctx),
					s.surfaceCard("Splitter", widget.SizedBox{
						Height: 180,
						Child: widget.Splitter{
							Axis:  layout.Horizontal,
							Ratio: 0.38,
							Gap:   8,
							First: widget.Container{
								Fill:   ctx.Theme.ColorScheme.SecondaryContainer,
								Radius: ctx.Theme.Shape.LargeRadius,
								Child:  widget.Center(widget.Text{Content: "Primary pane", Style: bodyStyle(ctx)}),
							},
							Second: widget.Container{
								Fill:   ctx.Theme.ColorScheme.InfoContainer,
								Radius: ctx.Theme.Shape.LargeRadius,
								Child:  widget.Center(widget.Text{Content: "Secondary pane", Style: bodyStyle(ctx)}),
							},
						},
					}, ctx),
				},
			},
		},
	}
}

func (s *ShowcaseState) buildMotionPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	target := 0.2
	if s.pulseOn {
		target = 1
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Motion", "Animated values, opacity, hover state, and state-driven transitions.", ctx),
			widget.Grid{
				MinChildWidth: 340,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.surfaceCard("Animated", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							actionButton("Toggle pulse", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonMedium, func() {
								s.SetState(func() { s.pulseOn = !s.pulseOn })
							}),
							widget.SizedBox{Height: th.Space.Unit(4)},
							widget.Animated{
								Value:    target,
								Duration: 380 * time.Millisecond,
								Curve:    widget.EaseInOut,
								Builder: func(v float64) widget.Widget {
									return widget.Container{
										Height:      140 + 24*v,
										Fill:        ctx.Theme.ColorScheme.PrimaryContainer.Lerp(ctx.Theme.ColorScheme.Primary, v*0.45),
										Border:      ctx.Theme.ColorScheme.Primary,
										BorderWidth: 1,
										Radius:      ctx.Theme.Shape.XLargeRadius + 4*v,
										Glow:        ctx.Theme.ColorScheme.FocusRing.WithAlpha(0.18 * v),
										GlowSpread:  6 * v,
										Child:       widget.Center(widget.Text{Content: fmt.Sprintf("animated %.0f%%", v*100), Style: bodyStyle(ctx)}),
									}
								},
							},
						},
					}, ctx),
					s.surfaceCard("Opacity", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Opacity{
								Value: 0.45,
								Child: widget.Container{
									Height: 96,
									Fill:   ctx.Theme.ColorScheme.Warning,
									Radius: ctx.Theme.Shape.LargeRadius,
									Child:  widget.Center(widget.Text{Content: "45%", Style: bodyStyle(ctx)}),
								},
							},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Text{Content: "Opacity is implemented as an approximation over opaque surfaces.", Style: mutedStyle(ctx)},
						},
					}, ctx),
					s.surfaceCard("State transitions", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							s.controlRow("Live mode", widget.Switch{
								Value:     s.switchLive,
								OnChanged: func(v bool) { s.SetState(func() { s.switchLive = v }) },
							}, ctx),
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Animated{
								Value:    boolToFloat64(s.switchLive),
								Duration: 220 * time.Millisecond,
								Curve:    widget.EaseOut,
								Builder: func(v float64) widget.Widget {
									return widget.Container{
										Height:      84,
										Fill:        ctx.Theme.ColorScheme.SuccessContainer.Lerp(ctx.Theme.ColorScheme.Success, v*0.35),
										Radius:      ctx.Theme.Shape.LargeRadius,
										Border:      ctx.Theme.ColorScheme.Success,
										BorderWidth: 1,
										Child:       widget.Center(widget.Text{Content: fmt.Sprintf("live factor %.2f", v), Style: bodyStyle(ctx)}),
									}
								},
							},
						},
					}, ctx),
				},
			},
		},
	}
}

func (s *ShowcaseState) buildScrollPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Scroll", "Stateful scrolling, manual ScrollArea offsets, and standalone ScrollBar widgets.", ctx),
			widget.Grid{
				MinChildWidth: 360,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.surfaceCard("Scroll", widget.SizedBox{
						Height: 280,
						Child: widget.Scroll{
							Child: widget.Column{
								CrossAxisAlignment: layout.CrossStretch,
								Children:           scrollRows(ctx),
							},
						},
					}, ctx),
					s.surfaceCard("ScrollArea", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: fmt.Sprintf("offset %.0f, %.0f", s.scrollX, s.scrollY), Style: bodyStyle(ctx)},
							widget.SizedBox{Height: th.Space.Unit(2)},
							widget.Slider{
								Value: s.scrollX, Min: 0, Max: 240,
								OnChanged: func(v float64) { s.SetState(func() { s.scrollX = v }) },
							},
							widget.SizedBox{Height: th.Space.Unit(2)},
							widget.Slider{
								Value: s.scrollY, Min: 0, Max: 240,
								OnChanged: func(v float64) { s.SetState(func() { s.scrollY = v }) },
							},
							widget.SizedBox{Height: th.Space.Unit(4)},
							widget.ScrollArea{
								Width:  320,
								Height: 200,
								Offset: geom.Pt(s.scrollX, s.scrollY),
								OnMeasure: func(sz geom.Size) {
									if s.scrollContent != sz {
										s.SetState(func() { s.scrollContent = sz })
									}
								},
								OnViewport: func(sz geom.Size) {
									if s.scrollViewport != sz {
										s.SetState(func() { s.scrollViewport = sz })
									}
								},
								Child: scrollCanvas(ctx),
							},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Text{
								Content: fmt.Sprintf("content %.0fx%.0f viewport %.0fx%.0f", s.scrollContent.Width, s.scrollContent.Height, s.scrollViewport.Width, s.scrollViewport.Height),
								Style:   mutedStyle(ctx),
							},
						},
					}, ctx),
					s.surfaceCard("ScrollBar", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: fmt.Sprintf("dragged offset %.0f", s.scrollBarOffset), Style: bodyStyle(ctx)},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Row{
								CrossAxisAlignment: layout.CrossCenter,
								Children: []widget.Widget{
									widget.SizedBox{
										Height: 180,
										Width:  7,
										Child: widget.ScrollBar{
											Axis:        layout.Vertical,
											Offset:      s.scrollBarOffset,
											ContentSize: 620,
											Viewport:    180,
											OnThumbDrag: func(v float64) { s.SetState(func() { s.scrollBarOffset = v }) },
										},
									},
									widget.SizedBox{Width: th.Space.Unit(4)},
									widget.Expanded{
										Child: widget.Column{
											CrossAxisAlignment: layout.CrossStretch,
											Children: []widget.Widget{
												widget.SizedBox{
													Height: 7,
													Child: widget.ScrollBar{
														Axis:        layout.Horizontal,
														Offset:      s.scrollBarOffset,
														ContentSize: 620,
														Viewport:    240,
														OnThumbDrag: func(v float64) { s.SetState(func() { s.scrollBarOffset = v }) },
													},
												},
												widget.SizedBox{Height: th.Space.Unit(3)},
												widget.Text{Content: "This standalone widget is the same scrollbar used by Scroll.", Style: mutedStyle(ctx)},
											},
										},
									},
								},
							},
						},
					}, ctx),
				},
			},
		},
	}
}

func (s *ShowcaseState) buildCollectionsPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Collections", "Application helpers, cards, sections, breakpoints, split layouts, drawers, dialogs, panels, popups, toasts, and overlays.", ctx),
			widget.Grid{
				MinChildWidth: 360,
				Gap:           th.Space.Unit(4),
				Children: []widget.Widget{
					s.surfaceCard("Card and Section", collections.Card{
						Raised: true,
						Child: collections.Section{
							Title: "Deployment lane",
							Action: actionButton("Ship", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonSmall, func() {
								s.app.ShowToastFor("Section action executed.", collections.ToastSuccess, 3*time.Second)
							}),
							Child: widget.Column{
								CrossAxisAlignment: layout.CrossStretch,
								Children: []widget.Widget{
									widget.Text{Content: "Collection components stay small and composable.", Style: bodyStyle(ctx)},
									widget.SizedBox{Height: th.Space.Unit(2)},
									widget.Text{Content: "Card provides surface treatment, Section provides heading structure.", Style: mutedStyle(ctx)},
								},
							},
						},
					}, ctx),
					s.surfaceCard("BreakpointLayout", collections.BreakpointLayout{
						Compact:  breakpointBlock("Compact layout", ctx.Theme.ColorScheme.WarningContainer, ctx),
						Medium:   breakpointBlock("Medium layout", ctx.Theme.ColorScheme.InfoContainer, ctx),
						Expanded: breakpointBlock("Expanded layout", ctx.Theme.ColorScheme.SuccessContainer, ctx),
					}, ctx),
					s.surfaceCard("SplitLayout", widget.SizedBox{
						Height: 220,
						Child: collections.SplitLayout{
							PrimaryWidth: 120,
							Primary: widget.Container{
								Fill:   ctx.Theme.ColorScheme.SurfaceContainer,
								Radius: ctx.Theme.Shape.LargeRadius,
								Child:  widget.Center(widget.Text{Content: "Primary", Style: bodyStyle(ctx)}),
							},
							Secondary: widget.Container{
								Fill:   ctx.Theme.ColorScheme.SurfaceVariant,
								Radius: ctx.Theme.Shape.LargeRadius,
								Child:  widget.Center(widget.Text{Content: "Secondary", Style: bodyStyle(ctx)}),
							},
						},
					}, ctx),
					s.surfaceCard("Application navigation", widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: fmt.Sprintf("page depth %d", s.app.PageDepth()), Style: bodyStyle(ctx)},
							widget.SizedBox{Height: th.Space.Unit(3)},
							widget.Wrap{
								Spacing:    th.Space.Unit(2),
								RunSpacing: th.Space.Unit(2),
								Children: []widget.Widget{
									actionButton("Navigate to page", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonMedium, s.openRouteDemo),
									actionButton("Replace page", widget.ButtonOutline, widget.ButtonNeutral, widget.ButtonMedium, s.replaceWithRouteDemo),
									actionButton("Pop", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, func() { s.app.Pop() }),
									actionButton("Go home", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, s.app.GoHome),
								},
							},
						},
					}, ctx),
					s.surfaceCard("Overlay actions", widget.Wrap{
						Spacing:    th.Space.Unit(2),
						RunSpacing: th.Space.Unit(2),
						Children: []widget.Widget{
							actionButton("Dialog", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonMedium, s.openSampleDialog),
							actionButton("Popup", widget.ButtonOutline, widget.ButtonPrimary, widget.ButtonMedium, s.showActionMenu),
							actionButton("Toast", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, func() {
								s.app.ShowToastFor("Toast queued from the collections page.", collections.ToastInfo, 4*time.Second)
							}),
							actionButton("Panel", widget.ButtonOutline, widget.ButtonNeutral, widget.ButtonMedium, s.toggleInspectorPanel),
							actionButton("Overlay entry", widget.ButtonGhost, widget.ButtonPrimary, widget.ButtonMedium, s.showCustomOverlayEntry),
						},
					}, ctx),
				},
			},
		},
	}
}

func (s *ShowcaseState) pageHeader(title, body string, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	titleStyle := th.TextTheme.TitleLarge
	titleStyle.Color = th.ColorScheme.OnSurface
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			widget.Text{Content: title, Style: &titleStyle},
			widget.SizedBox{Height: th.Space.Unit(2)},
			widget.Text{Content: body, Style: mutedStyle(ctx)},
			widget.SizedBox{Height: th.Space.Unit(4)},
		},
	}
}

func (s *ShowcaseState) pageHero(title, body string, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	titleStyle := th.TextTheme.TitleLarge
	titleStyle.Color = th.ColorScheme.OnSurface
	return widget.Container{
		Fill:          th.ColorScheme.Surface,
		Radius:        th.Shape.XLargeRadius,
		Border:        th.ColorScheme.Outline,
		BorderWidth:   1,
		Shadow:        th.ColorScheme.Shadow,
		ShadowBlur:    th.Shadow.MD.Blur,
		ShadowOffsetY: th.Shadow.MD.OffsetY,
		Padding:       layout.All(th.Space.Unit(5)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children: []widget.Widget{
				widget.Text{Content: title, Style: &titleStyle},
				widget.SizedBox{Height: th.Space.Unit(3)},
				widget.Text{Content: body, Style: bodyStyle(ctx)},
			},
		},
	}
}

func (s *ShowcaseState) surfaceCard(title string, child widget.Widget, ctx widget.BuildContext) widget.Widget {
	return collections.Card{
		Child: collections.Section{
			Title: title,
			Child: child,
		},
	}
}

func (s *ShowcaseState) metricCard(title, body string, ctx widget.BuildContext) widget.Widget {
	return s.surfaceCard(title, widget.Text{Content: body, Style: bodyStyle(ctx)}, ctx)
}

func (s *ShowcaseState) controlRow(label string, trailing widget.Widget, ctx widget.BuildContext) widget.Widget {
	return widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Expanded{Child: widget.Text{Content: label, Style: bodyStyle(ctx)}},
			trailing,
		},
	}
}

func (s *ShowcaseState) navHeader(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	titleStyle := th.TextTheme.TitleSmall
	titleStyle.Color = th.ColorScheme.OnSurface
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			widget.Text{Content: "Graphics Demo", Style: &titleStyle},
			widget.SizedBox{Height: th.Space.Unit(1)},
			widget.Text{Content: "Every public widget and collection is surfaced here.", Style: mutedStyle(ctx)},
		},
	}
}

func (s *ShowcaseState) drawerContent(ctx widget.BuildContext) widget.Widget {
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.navHeader(ctx),
			widget.SizedBox{Height: ctx.Theme.Space.Unit(4)},
			collections.NavBar{
				Destinations: demoSections,
				Selected:     s.section,
				OnSelected:   s.selectSection,
				Compact:      false,
			},
		},
	}
}

func (s *ShowcaseState) brandLockup(ctx widget.BuildContext) widget.Widget {
	return widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			fallbackIcon("A", ctx),
			widget.SizedBox{Width: ctx.Theme.Space.Unit(2)},
			widget.Column{
				CrossAxisAlignment: layout.CrossStart,
				Children: []widget.Widget{
					widget.Text{Content: "graphics", Style: bodyStyle(ctx)},
					widget.Text{Content: "full widget surface", Style: mutedStyle(ctx)},
				},
			},
		},
	}
}

func (s *ShowcaseState) buildStatusBar(ctx widget.BuildContext) widget.Widget {
	return widget.Row{
		MainAxisAlignment:  layout.MainSpaceBetween,
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Text{Content: fmt.Sprintf("Section: %s", demoSections[s.section].Label), Style: mutedStyle(ctx)},
			widget.Text{Content: fmt.Sprintf("Page depth: %d", s.app.PageDepth()), Style: mutedStyle(ctx)},
		},
	}
}

func (s *ShowcaseState) appBarButton(label string, onPressed func()) widget.Widget {
	return actionButton(label, widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonSmall, onPressed)
}

func (s *ShowcaseState) showActionMenu() {
	s.app.ShowPopup([]collections.MenuItem{
		{Label: "Info toast", OnTap: func() { s.app.ShowToastFor("Popup action fired.", collections.ToastInfo, 3*time.Second) }},
		{Label: "Open dialog", OnTap: s.openSampleDialog},
		{Divider: true},
		{Label: "Toggle inspector", OnTap: s.toggleInspectorPanel},
	}, s.popupAnchor())
}

func (s *ShowcaseState) popupAnchor() geom.Rect {
	x := s.screen.Width - 180
	if x < 24 {
		x = 24
	}
	return geom.NewRect(x, 88, 120, 32)
}

func (s *ShowcaseState) openSampleDialog() {
	var close func()
	close = s.app.ShowDialog(collections.Dialog{
		Title: "Overlay-backed dialog",
		Body: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children: []widget.Widget{
				widget.Text{Content: "Dialogs are hosted by OverlayView through DialogController."},
				widget.SizedBox{Height: 12},
				widget.Text{Content: "This demonstrates modal scrims, stacked overlays, and collection-level composition."},
			},
		},
		Actions: []widget.Widget{
			actionButton("Close", widget.ButtonOutline, widget.ButtonNeutral, widget.ButtonSmall, func() {
				if close != nil {
					close()
				}
			}),
			actionButton("Toast", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonSmall, func() {
				s.app.ShowToastFor("Dialog action completed.", collections.ToastSuccess, 3*time.Second)
				if close != nil {
					close()
				}
			}),
		},
	})
}

func (s *ShowcaseState) openRouteDemo() {
	depth := s.app.PageDepth() + 1
	s.app.NavigateTo(collections.ApplicationPage{
		Name: fmt.Sprintf("route-%d", depth),
		Builder: func(ctx widget.BuildContext) widget.Widget {
			return widget.Padding{
				Insets: layout.All(ctx.Theme.Space.Unit(5)),
				Child: widget.Column{
					CrossAxisAlignment: layout.CrossStretch,
					Children: []widget.Widget{
						s.pageHeader("Route Page", "This page is rendered from ApplicationController's navigation stack.", ctx),
						s.surfaceCard("Navigation actions", widget.Wrap{
							Spacing:    ctx.Theme.Space.Unit(2),
							RunSpacing: ctx.Theme.Space.Unit(2),
							Children: []widget.Widget{
								actionButton("Push again", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonMedium, s.openRouteDemo),
								actionButton("Replace", widget.ButtonOutline, widget.ButtonNeutral, widget.ButtonMedium, s.replaceWithRouteDemo),
								actionButton("Pop", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, func() { s.app.Pop() }),
								actionButton("Go home", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, s.app.GoHome),
							},
						}, ctx),
						widget.SizedBox{Height: ctx.Theme.Space.Unit(4)},
						s.surfaceCard("State", widget.Column{
							CrossAxisAlignment: layout.CrossStretch,
							Children: []widget.Widget{
								widget.Text{Content: fmt.Sprintf("Current depth: %d", s.app.PageDepth()), Style: bodyStyle(ctx)},
								widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
								widget.Text{Content: "Use the app bar Back button or the actions above to walk the stack.", Style: mutedStyle(ctx)},
							},
						}, ctx),
					},
				},
			}
		},
	})
}

func (s *ShowcaseState) replaceWithRouteDemo() {
	s.app.Replace(collections.ApplicationPage{
		Name: "replaced-page",
		Builder: func(ctx widget.BuildContext) widget.Widget {
			return widget.Padding{
				Insets: layout.All(ctx.Theme.Space.Unit(5)),
				Child: widget.Column{
					CrossAxisAlignment: layout.CrossStretch,
					Children: []widget.Widget{
						s.pageHeader("Replaced Page", "Replace swaps the current top route without clearing the whole stack.", ctx),
						s.surfaceCard("Actions", widget.Wrap{
							Spacing:    ctx.Theme.Space.Unit(2),
							RunSpacing: ctx.Theme.Space.Unit(2),
							Children: []widget.Widget{
								actionButton("Push next", widget.ButtonSolid, widget.ButtonPrimary, widget.ButtonMedium, s.openRouteDemo),
								actionButton("Pop", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, func() { s.app.Pop() }),
								actionButton("Go home", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonMedium, s.app.GoHome),
							},
						}, ctx),
					},
				},
			}
		},
	})
}

func (s *ShowcaseState) toggleInspectorPanel() {
	s.app.TogglePanel("inspector", func() widget.Widget {
		return widget.Positioned{
			Top:   widget.Ptr(76),
			Right: widget.Ptr(20),
			Child: widget.SizedBox{
				Width: 320,
				Child: widget.Container{
					Fill:          color.White,
					Radius:        20,
					Border:        color.Color{R: 0.82, G: 0.84, B: 0.9, A: 1},
					BorderWidth:   1,
					Shadow:        color.Black.WithAlpha(0.18),
					ShadowBlur:    18,
					ShadowOffsetY: 8,
					Padding:       layout.All(20),
					Child: widget.Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []widget.Widget{
							widget.Text{Content: "Inspector Panel"},
							widget.SizedBox{Height: 10},
							widget.Text{Content: "Application owns one exclusive panel controller for this shell."},
							widget.SizedBox{Height: 14},
							actionButton("Close panel", widget.ButtonOutline, widget.ButtonNeutral, widget.ButtonSmall, s.app.ClosePanel),
						},
					},
				},
			},
		}
	})
}

func (s *ShowcaseState) showCustomOverlayEntry() {
	var entry *collections.OverlayEntry
	entry = &collections.OverlayEntry{
		Z: 10,
		Builder: func(_ widget.BuildContext) widget.Widget {
			return widget.Positioned{
				Top:   widget.Ptr(20),
				Right: widget.Ptr(20),
				Child: widget.Container{
					Fill:          color.Color{R: 0.14, G: 0.18, B: 0.24, A: 0.96},
					Radius:        18,
					Padding:       layout.Symmetric(16, 12),
					Shadow:        color.Black.WithAlpha(0.2),
					ShadowBlur:    14,
					ShadowOffsetY: 6,
					Child: widget.Row{
						CrossAxisAlignment: layout.CrossCenter,
						Children: []widget.Widget{
							widget.Text{Content: "Custom OverlayEntry"},
							widget.SizedBox{Width: 12},
							actionButton("Dismiss", widget.ButtonGhost, widget.ButtonNeutral, widget.ButtonSmall, entry.Remove),
						},
					},
				},
			}
		},
	}
	s.app.InsertOverlay(entry)
}

func scrollRows(ctx widget.BuildContext) []widget.Widget {
	rows := make([]widget.Widget, 0, 14)
	for i := 0; i < 14; i++ {
		rows = append(rows,
			widget.Container{
				Fill:        ctx.Theme.ColorScheme.Surface,
				Radius:      ctx.Theme.Shape.MediumRadius,
				Border:      ctx.Theme.ColorScheme.Outline,
				BorderWidth: 1,
				Padding:     layout.Symmetric(ctx.Theme.Space.Unit(3), ctx.Theme.Space.Unit(2)),
				Child:       widget.Text{Content: fmt.Sprintf("scroll row %02d", i+1), Style: bodyStyle(ctx)},
			},
			widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
		)
	}
	return rows
}

func scrollCanvas(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	cells := make([]widget.Widget, 0, 20)
	palette := []color.Color{
		th.ColorScheme.PrimaryContainer,
		th.ColorScheme.SecondaryContainer,
		th.ColorScheme.InfoContainer,
		th.ColorScheme.WarningContainer,
		th.ColorScheme.SuccessContainer,
	}
	for i := 0; i < 20; i++ {
		cells = append(cells, widget.Container{
			Height:      72,
			Fill:        palette[i%len(palette)],
			Radius:      th.Shape.MediumRadius,
			Border:      th.ColorScheme.Outline,
			BorderWidth: 1,
			Child:       widget.Center(widget.Text{Content: fmt.Sprintf("cell %d", i+1), Style: bodyStyle(ctx)}),
		})
	}
	return widget.SizedBox{
		Width:  560,
		Height: 460,
		Child: widget.Padding{
			Insets: layout.All(th.Space.Unit(3)),
			Child: widget.Grid{
				Columns:  4,
				Gap:      th.Space.Unit(2),
				Children: cells,
			},
		},
	}
}

func colorBox(fill color.Color, width, height float64) widget.Widget {
	return widget.Container{Width: width, Height: height, Fill: fill, Radius: 12}
}

func actionButton(label string, variant widget.ButtonVariant, tone widget.ButtonTone, size widget.ButtonSize, onPressed func()) widget.Widget {
	return widget.Button{
		Child:     widget.Text{Content: label},
		Variant:   variant,
		Tone:      tone,
		Size:      size,
		OnPressed: onPressed,
	}
}

func (s *ShowcaseState) tag(text string, ctx widget.BuildContext) widget.Widget {
	return widget.Container{
		Fill:        ctx.Theme.ColorScheme.SurfaceContainer,
		Radius:      ctx.Theme.Shape.LargeRadius,
		Border:      ctx.Theme.ColorScheme.Outline,
		BorderWidth: 1,
		Padding:     layout.Symmetric(ctx.Theme.Space.Unit(3), ctx.Theme.Space.Unit(1.5)),
		Child:       widget.Text{Content: text, Style: mutedStyle(ctx)},
	}
}

func badgePill(text string, ctx widget.BuildContext) widget.Widget {
	return widget.Container{
		Fill:        ctx.Theme.ColorScheme.Surface,
		Border:      ctx.Theme.ColorScheme.Outline,
		BorderWidth: 1,
		Radius:      ctx.Theme.Shape.LargeRadius,
		Padding:     layout.Symmetric(ctx.Theme.Space.Unit(3), ctx.Theme.Space.Unit(1.5)),
		Child:       widget.Text{Content: text, Style: bodyStyle(ctx)},
	}
}

func opacitySwatch(value float64, ctx widget.BuildContext) widget.Widget {
	return widget.Opacity{
		Value: value,
		Child: widget.Container{
			Width:  72,
			Height: 72,
			Fill:   ctx.Theme.ColorScheme.Primary,
			Radius: ctx.Theme.Shape.LargeRadius,
			Child:  widget.Center(widget.Text{Content: fmt.Sprintf("%.0f%%", value*100), Style: bodyStyle(ctx)}),
		},
	}
}

func breakpointBlock(label string, fill color.Color, ctx widget.BuildContext) widget.Widget {
	return widget.Container{
		Height:      140,
		Fill:        fill,
		Radius:      ctx.Theme.Shape.XLargeRadius,
		Border:      ctx.Theme.ColorScheme.Outline,
		BorderWidth: 1,
		Child:       widget.Center(widget.Text{Content: label, Style: bodyStyle(ctx)}),
	}
}

func fallbackIcon(label string, ctx widget.BuildContext) widget.Widget {
	return widget.Container{
		Width:       36,
		Height:      36,
		Fill:        ctx.Theme.ColorScheme.PrimaryContainer,
		Radius:      ctx.Theme.Shape.LargeRadius,
		Border:      ctx.Theme.ColorScheme.Primary,
		BorderWidth: 1,
		Child:       widget.Center(widget.Text{Content: label, Style: bodyStyle(ctx)}),
	}
}

func bodyStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.BodyMedium
	style.Color = ctx.Theme.ColorScheme.OnSurface
	return &style
}

func mutedStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.BodySmall
	style.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
	return &style
}

func demoImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 96, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			switch {
			case x < 32:
				img.Set(x, y, stdcolor.RGBA{R: 52, G: 120, B: 246, A: 255})
			case x < 64:
				img.Set(x, y, stdcolor.RGBA{R: 34, G: 197, B: 94, A: 255})
			default:
				img.Set(x, y, stdcolor.RGBA{R: 249, G: 115, B: 22, A: 255})
			}
		}
	}
	for i := 16; i < 80; i++ {
		img.Set(i, i, stdcolor.White)
		img.Set(95-i, i, stdcolor.White)
	}
	return img
}

func boolToFloat64(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
