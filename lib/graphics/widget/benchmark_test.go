package widget

import (
	"testing"

	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/theme"
)

func BenchmarkFrameRenderSettingsPanel(b *testing.B) {
	th := theme.Light()
	frame := NewFrame(th, geom.Sz(1280, 720))
	canvas := pixbuf.NewCanvas(1280, 720)

	titleStyle := th.TextTheme.TitleLarge
	bodyStyle := th.TextTheme.BodyMedium

	root := Scroll{
		Child: Padding{
			Insets: layout.Symmetric(40, 24),
			Child: Column{
				CrossAxisAlignment: layout.CrossStretch,
				Children: []Widget{
					Text{Content: "Graphics Benchmark Dashboard", Style: &titleStyle},
					SizedBox{Height: 24},
					benchmarkSection(th, "System", bodyStyle),
					SizedBox{Height: 16},
					benchmarkSection(th, "Display", bodyStyle),
					SizedBox{Height: 16},
					benchmarkSection(th, "Audio", bodyStyle),
					SizedBox{Height: 16},
					benchmarkSection(th, "Network", bodyStyle),
				},
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame.Render(root, canvas)
	}
}

func BenchmarkFrameRenderDesktopChrome(b *testing.B) {
	th := theme.Light()
	frame := NewFrame(th, geom.Sz(1536, 863))
	canvas := pixbuf.NewCanvas(1536, 863)

	effectImageCache.mu.Lock()
	effectImageCache.entries = nil
	effectImageCache.usedBytes = 0
	effectImageCache.stamp = 0
	effectImageCache.mu.Unlock()

	root := benchmarkDesktopChrome(th)
	frame.Render(root, canvas)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame.Render(root, canvas)
	}
}

func benchmarkSection(th *theme.ThemeData, title string, bodyStyle theme.TextStyle) Widget {
	labelStyle := th.TextTheme.LabelLarge
	labelStyle.Color = th.ColorScheme.OnSurfaceVariant

	rows := make([]Widget, 0, 9)
	rows = append(rows,
		Text{Content: title, Style: &labelStyle},
		SizedBox{Height: 12},
	)
	for i := 0; i < 3; i++ {
		rows = append(rows, benchmarkRow(th, bodyStyle, i))
		if i < 2 {
			rows = append(rows, SizedBox{Height: 10})
		}
	}

	return Container{
		Fill:        th.ColorScheme.Surface,
		Border:      th.ColorScheme.Outline,
		BorderWidth: 1,
		Radius:      th.Shape.LargeRadius,
		Padding:     layout.All(16),
		Child: Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           rows,
		},
	}
}

func benchmarkRow(th *theme.ThemeData, bodyStyle theme.TextStyle, i int) Widget {
	left := Column{
		CrossAxisAlignment: layout.CrossStart,
		Children: []Widget{
			Text{Content: "A fairly long label that commonly truncates in constrained layouts", Style: &bodyStyle},
			SizedBox{Height: 4},
			Text{Content: "Secondary description text that should exercise repeated text measurement paths", Style: &bodyStyle},
		},
	}

	value := "Enabled"
	if i%2 == 1 {
		value = "Disabled"
	}

	return Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []Widget{
			Expanded{Child: left},
			SizedBox{Width: 12},
			Button{
				Variant: ButtonOutline,
				Child:   Text{Content: "Open advanced settings"},
			},
			SizedBox{Width: 12},
			TextInput{
				Value: ptrString(value),
				Label: "State",
				Hint:  "Value",
			},
		},
	}
}

func ptrString(s string) *string { return &s }

func benchmarkDesktopChrome(th *theme.ThemeData) Widget {
	return Stack{
		Children: []Widget{
			Container{Fill: th.ColorScheme.Background},
			Positioned{
				Left: Ptr(24), Right: Ptr(24), Bottom: Ptr(18), Height: Ptr(60),
				Child: Container{
					Fill:          th.ColorScheme.Surface,
					Border:        th.ColorScheme.Outline,
					BorderWidth:   1,
					Radius:        th.Shape.XLargeRadius,
					Shadow:        th.ColorScheme.Shadow,
					ShadowBlur:    th.Shadow.LG.Blur,
					ShadowOffsetY: -4,
					Padding:       layout.Symmetric(18, 12),
					Child: Row{
						CrossAxisAlignment: layout.CrossCenter,
						Children: []Widget{
							Button{Variant: ButtonSolid, Child: Text{Content: "Launcher"}},
							SizedBox{Width: 12},
							Button{Variant: ButtonOutline, Child: Text{Content: "Search"}},
							SizedBox{Width: 12},
							Switch{Value: true},
							Expanded{Child: SizedBox{}},
							TextInput{Label: "Search", Hint: "Type to filter"},
						},
					},
				},
			},
			Positioned{
				Left: Ptr(64), Top: Ptr(56), Width: Ptr(520), Height: Ptr(360),
				Child: Container{
					Fill:          th.ColorScheme.Surface,
					Border:        th.ColorScheme.Outline,
					BorderWidth:   1,
					Radius:        th.Shape.XLargeRadius,
					Shadow:        th.ColorScheme.Shadow,
					ShadowBlur:    th.Shadow.MD.Blur,
					ShadowOffsetY: th.Shadow.MD.OffsetY,
					Padding:       layout.All(16),
					Child:         benchmarkWindowContent(th, "Files"),
				},
			},
			Positioned{
				Left: Ptr(628), Top: Ptr(84), Width: Ptr(460), Height: Ptr(308),
				Child: Container{
					Fill:          th.ColorScheme.Surface,
					Border:        th.ColorScheme.Outline,
					BorderWidth:   1,
					Radius:        th.Shape.LargeRadius,
					Shadow:        th.ColorScheme.Shadow,
					ShadowBlur:    th.Shadow.MD.Blur,
					ShadowOffsetY: th.Shadow.MD.OffsetY,
					Padding:       layout.All(14),
					Child:         benchmarkWindowContent(th, "Monitor"),
				},
			},
			Positioned{
				Right: Ptr(36), Top: Ptr(64), Width: Ptr(312), Height: Ptr(424),
				Child: Container{
					Fill:          th.ColorScheme.Surface,
					Border:        th.ColorScheme.Outline,
					BorderWidth:   1,
					Radius:        th.Shape.XLargeRadius,
					Shadow:        th.ColorScheme.Shadow,
					ShadowBlur:    th.Shadow.LG.Blur,
					ShadowOffsetY: -4,
					Padding:       layout.All(16),
					Child: Column{
						CrossAxisAlignment: layout.CrossStretch,
						Children: []Widget{
							Text{Content: "Quick Settings", Style: &th.TextTheme.TitleMedium},
							SizedBox{Height: 12},
							benchmarkSettingRow(th, "Wi-Fi", true),
							SizedBox{Height: 10},
							benchmarkSettingRow(th, "Bluetooth", false),
							SizedBox{Height: 10},
							benchmarkSettingRow(th, "Night light", true),
							SizedBox{Height: 16},
							Button{Variant: ButtonSolid, Child: Text{Content: "Open display settings"}},
						},
					},
				},
			},
		},
	}
}

func benchmarkWindowContent(th *theme.ThemeData, title string) Widget {
	titleStyle := th.TextTheme.TitleMedium
	bodyStyle := th.TextTheme.BodyMedium
	bodyStyle.Color = th.ColorScheme.OnSurfaceVariant
	return Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []Widget{
			Text{Content: title, Style: &titleStyle},
			SizedBox{Height: 12},
			Container{
				Fill:       th.ColorScheme.SurfaceVariant,
				Radius:     th.Shape.MediumRadius,
				Glow:       th.ColorScheme.FocusRing.WithAlpha(0.08),
				GlowSpread: 3,
				Padding:    layout.All(12),
				Child:      Text{Content: "A composited panel with rounded corners, shadowed chrome, and a few interactive controls.", Style: &bodyStyle},
			},
			SizedBox{Height: 12},
			Button{Variant: ButtonSolid, Child: Text{Content: "Primary action"}},
			SizedBox{Height: 10},
			Button{Variant: ButtonOutline, Child: Text{Content: "Secondary action"}},
		},
	}
}

func benchmarkSettingRow(th *theme.ThemeData, label string, value bool) Widget {
	style := th.TextTheme.BodyMedium
	return Container{
		Fill:    th.ColorScheme.SurfaceVariant,
		Radius:  th.Shape.MediumRadius,
		Padding: layout.Symmetric(12, 10),
		Child: Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []Widget{
				Expanded{Child: Text{Content: label, Style: &style}},
				Switch{Value: value},
			},
		},
	}
}
