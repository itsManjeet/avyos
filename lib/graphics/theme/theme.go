// Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

// Package theme provides theming primitives for the UI framework.
//
// ThemeData contains color schemes, text styles, and widget defaults.
// Widgets read the active theme via BuildContext.Theme().
package theme

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/font/ttf"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/ini"
	"golang.org/x/image/font"
)

// TextStyle describes the visual style of a piece of text.
type TextStyle struct {
	Face          canvas.Typeface
	Size          float64
	LineHeight    float64
	Color         color.Color
	LetterSpacing float64
}

// ColorScheme holds Flutter-style semantic colors for a theme.
type ColorScheme struct {
	Primary                 color.Color
	OnPrimary               color.Color
	PrimaryContainer        color.Color
	OnPrimaryContainer      color.Color
	Secondary               color.Color
	OnSecondary             color.Color
	SecondaryContainer      color.Color
	OnSecondaryContainer    color.Color
	Tertiary                color.Color
	OnTertiary              color.Color
	TertiaryContainer       color.Color
	OnTertiaryContainer     color.Color
	Surface                 color.Color
	OnSurface               color.Color
	SurfaceVariant          color.Color
	OnSurfaceVariant        color.Color
	Background              color.Color
	OnBackground            color.Color
	Error                   color.Color
	OnError                 color.Color
	ErrorContainer          color.Color
	OnErrorContainer        color.Color
	Outline                 color.Color
	OutlineVariant          color.Color
	Shadow                  color.Color
	SurfaceDim              color.Color
	SurfaceBright           color.Color
	SurfaceContainerLowest  color.Color
	SurfaceContainerLow     color.Color
	SurfaceContainer        color.Color
	SurfaceContainerHigh    color.Color
	SurfaceContainerHighest color.Color
	Scrim                   color.Color
	InverseSurface          color.Color
	OnInverseSurface        color.Color
	InversePrimary          color.Color
	SurfaceTint             color.Color
	FocusRing               color.Color

	Success            color.Color
	OnSuccess          color.Color
	SuccessContainer   color.Color
	OnSuccessContainer color.Color
	Warning            color.Color
	OnWarning          color.Color
	WarningContainer   color.Color
	OnWarningContainer color.Color
	Info               color.Color
	OnInfo             color.Color
	InfoContainer      color.Color
	OnInfoContainer    color.Color
}

// ThemeData is the complete theme configuration.
type ThemeData struct {
	ColorScheme ColorScheme
	TextTheme   TextTheme
	Shape       ShapeTheme
	Space       SpaceTheme
	Shadow      ShadowTheme
	Motion      MotionTheme
	Accent      AccentTheme
}

// AccentTheme contains the Avyos accent palette used to derive semantic colors.
type AccentTheme struct {
	Name           string
	Accent         color.Color
	AccentStrong   color.Color
	AccentSoft     color.Color
	AccentAlt      color.Color
	PageBackground color.Color
	GlowOne        color.Color
	GlowTwo        color.Color
	GlowThree      color.Color
}

// TextTheme contains named text styles.
type TextTheme struct {
	Size2XS        TextStyle
	SizeXS         TextStyle
	SizeSM         TextStyle
	SizeMD         TextStyle
	SizeLG         TextStyle
	SizeXL         TextStyle
	Size2XL        TextStyle
	Size3XL        TextStyle
	Size4XL        TextStyle
	Size5XL        TextStyle
	Size6XL        TextStyle
	Size7XL        TextStyle
	DisplayLarge   TextStyle
	DisplayMedium  TextStyle
	DisplaySmall   TextStyle
	HeadlineLarge  TextStyle
	HeadlineMedium TextStyle
	HeadlineSmall  TextStyle
	TitleLarge     TextStyle
	TitleMedium    TextStyle
	TitleSmall     TextStyle
	BodyLarge      TextStyle
	BodyMedium     TextStyle
	BodySmall      TextStyle
	LabelLarge     TextStyle
	LabelMedium    TextStyle
	LabelSmall     TextStyle
}

// ShapeTheme controls default corner radii.
type ShapeTheme struct {
	NoneRadius     float64
	XXSmallRadius  float64
	XSmallRadius   float64
	SmallRadius    float64
	MediumRadius   float64
	LargeRadius    float64
	XLargeRadius   float64
	XXLargeRadius  float64
	XXXLargeRadius float64
	FullRadius     float64
}

type SpaceTheme struct{ Base float64 }

func (s SpaceTheme) Unit(token float64) float64 {
	base := s.Base
	if base <= 0 {
		base = 4
	}
	return token * base
}

func (s SpaceTheme) All(token float64) layout.EdgeInsets {
	return layout.All(s.Unit(token))
}

func (s SpaceTheme) Symmetric(horizontal, vertical float64) layout.EdgeInsets {
	return layout.Symmetric(s.Unit(horizontal), s.Unit(vertical))
}

func (s SpaceTheme) LTRB(left, top, right, bottom float64) layout.EdgeInsets {
	return layout.LTRB(s.Unit(left), s.Unit(top), s.Unit(right), s.Unit(bottom))
}

type ShadowSpec struct {
	Blur       float64
	OffsetY    float64
	GlowSpread float64
}

type ShadowTheme struct {
	XS  ShadowSpec
	SM  ShadowSpec
	MD  ShadowSpec
	LG  ShadowSpec
	XL  ShadowSpec
	XXL ShadowSpec
}

type MotionTheme struct {
	Fast     time.Duration
	Moderate time.Duration
	Slow     time.Duration
}

type fontConfig struct {
	displayFace  canvas.Typeface
	bodyFace     canvas.Typeface
	labelFace    canvas.Typeface
	bodySize     float64
	leadSize     float64
	titleSize    float64
	headlineSize float64
	displaySize  float64
	labelSize    float64
}

var (
	fontOnce sync.Once
	loadedFC fontConfig
)

func getFont() fontConfig {
	fontOnce.Do(func() { loadedFC = loadFontConfig() })
	return loadedFC
}

func defaultBodyFace() canvas.Typeface {
	if face := loadNamedFace("inter"); face != nil {
		return face
	}
	return ttf.Default()
}

func defaultDisplayFace() canvas.Typeface {
	return defaultBodyFace()
}

func loadNamedFace(name string) canvas.Typeface {
	if name == "" {
		return nil
	}

	paths := []string{
		filepath.Join("/usr/share/fonts", name, name+".ttf"),
		filepath.Join("/usr/share/fonts", name, name+"-regular.ttf"),
		filepath.Join("/usr/share/fonts", name+".ttf"),
		name,
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		face, err := ttf.New(data)
		if err == nil {
			return face
		}
	}
	return nil
}

func configureFace(face canvas.Typeface, dpi float64, hinting font.Hinting) canvas.Typeface {
	tf, ok := face.(*ttf.Face)
	if !ok || tf == nil {
		return face
	}
	if dpi > 0 {
		tf = tf.WithDPI(dpi)
	}
	return tf.WithHinting(hinting)
}

func loadFontConfig() fontConfig {
	cfg := fontConfig{
		displayFace:  defaultDisplayFace(),
		bodyFace:     defaultBodyFace(),
		labelFace:    defaultBodyFace(),
		bodySize:     10,
		leadSize:     12,
		titleSize:    14,
		headlineSize: 14,
		displaySize:  14,
		labelSize:    10,
	}

	conf, err := ini.ParseFile("/etc/fonts.ini")
	if err != nil {
		return cfg
	}

	if enabled, ok := conf.Get("ui", "enabled"); !ok || enabled != "true" {
		return cfg
	}

	parseSize := func(key string, dst *float64) {
		if v, ok := conf.Get("ui", key); ok {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				*dst = f
			}
		}
	}
	parseSize("body_size", &cfg.bodySize)
	parseSize("paragraph_size", &cfg.bodySize)
	parseSize("lead_size", &cfg.leadSize)
	parseSize("subheading_size", &cfg.leadSize)
	parseSize("title_size", &cfg.titleSize)
	parseSize("headline_size", &cfg.headlineSize)
	parseSize("heading_size", &cfg.headlineSize)
	parseSize("display_size", &cfg.displaySize)
	parseSize("label_size", &cfg.labelSize)

	dpi := 96.0
	if dpiStr, ok := conf.Get("ui", "dpi"); ok {
		if parsed, err := strconv.ParseFloat(dpiStr, 64); err == nil && parsed > 0 {
			dpi = parsed
		}
	}

	hinting := font.HintingFull
	if hintStr, ok := conf.Get("ui", "hinting"); ok {
		switch hintStr {
		case "none":
			hinting = font.HintingNone
		case "vertical":
			hinting = font.HintingVertical
		}
	}

	legacyName, _ := conf.Get("ui", "name")
	bodyName, ok := conf.Get("ui", "body_name")
	if !ok || bodyName == "" {
		bodyName = legacyName
	}
	displayName, ok := conf.Get("ui", "display_name")
	if !ok || displayName == "" {
		displayName = ""
	}
	labelName, ok := conf.Get("ui", "label_name")
	if !ok || labelName == "" {
		labelName = bodyName
	}

	if face := loadNamedFace(bodyName); face != nil {
		cfg.bodyFace = face
	}
	if face := loadNamedFace(displayName); face != nil {
		cfg.displayFace = face
	}
	if face := loadNamedFace(labelName); face != nil {
		cfg.labelFace = face
	}

	cfg.bodyFace = configureFace(cfg.bodyFace, dpi, hinting)
	cfg.displayFace = configureFace(cfg.displayFace, dpi, hinting)
	cfg.labelFace = configureFace(cfg.labelFace, dpi, hinting)
	return cfg
}

const defaultAccentTheme = "varanasi"

var accentThemeOrder = []string{
	"varanasi",
	"jaipur",
	"jodhpur",
	"coorg",
	"kochi",
	"ladakh",
	"konark",
	"madurai",
}

var accentThemes = map[string]AccentTheme{
	"varanasi": {
		Name: "Varanasi", Accent: color.FromHex(0xE36A22), AccentStrong: color.FromHex(0xC94F13),
		AccentSoft: color.FromHex(0xFFF3EC), AccentAlt: color.FromHex(0xF29B57), PageBackground: color.FromHex(0xF8F3EC),
		GlowOne: color.FromRGBA8(245, 196, 168, 184), GlowTwo: color.FromRGBA8(255, 224, 191, 184), GlowThree: color.FromRGBA8(238, 227, 210, 184),
	},
	"jaipur": {
		Name: "Jaipur", Accent: color.FromHex(0xC95C7A), AccentStrong: color.FromHex(0x9F3E5B),
		AccentSoft: color.FromHex(0xFFF0F4), AccentAlt: color.FromHex(0xE58AA0), PageBackground: color.FromHex(0xFAF1F3),
		GlowOne: color.FromRGBA8(243, 184, 200, 184), GlowTwo: color.FromRGBA8(255, 220, 229, 184), GlowThree: color.FromRGBA8(240, 226, 229, 184),
	},
	"jodhpur": {
		Name: "Jodhpur", Accent: color.FromHex(0x4B63C7), AccentStrong: color.FromHex(0x34479E),
		AccentSoft: color.FromHex(0xEEF1FF), AccentAlt: color.FromHex(0x8092EA), PageBackground: color.FromHex(0xF2F3FA),
		GlowOne: color.FromRGBA8(190, 200, 242, 184), GlowTwo: color.FromRGBA8(220, 226, 255, 184), GlowThree: color.FromRGBA8(228, 230, 242, 184),
	},
	"coorg": {
		Name: "Coorg", Accent: color.FromHex(0x6F4A35), AccentStrong: color.FromHex(0x4E3324),
		AccentSoft: color.FromHex(0xF5EEE9), AccentAlt: color.FromHex(0xA06F52), PageBackground: color.FromHex(0xF5F0EA),
		GlowOne: color.FromRGBA8(190, 154, 130, 184), GlowTwo: color.FromRGBA8(236, 218, 204, 184), GlowThree: color.FromRGBA8(226, 217, 207, 184),
	},
	"kochi": {
		Name: "Kochi", Accent: color.FromHex(0x1F9F8E), AccentStrong: color.FromHex(0x167568),
		AccentSoft: color.FromHex(0xE9F8F5), AccentAlt: color.FromHex(0x58C7B7), PageBackground: color.FromHex(0xEFF8F5),
		GlowOne: color.FromRGBA8(176, 232, 222, 184), GlowTwo: color.FromRGBA8(209, 246, 239, 184), GlowThree: color.FromRGBA8(225, 237, 233, 184),
	},
	"ladakh": {
		Name: "Ladakh", Accent: color.FromHex(0x3E8ED0), AccentStrong: color.FromHex(0x2B67A0),
		AccentSoft: color.FromHex(0xEDF6FF), AccentAlt: color.FromHex(0x76B7EE), PageBackground: color.FromHex(0xF1F6FB),
		GlowOne: color.FromRGBA8(186, 217, 245, 184), GlowTwo: color.FromRGBA8(218, 237, 255, 184), GlowThree: color.FromRGBA8(226, 233, 240, 184),
	},
	"konark": {
		Name: "Konark", Accent: color.FromHex(0xD58A18), AccentStrong: color.FromHex(0xA8660F),
		AccentSoft: color.FromHex(0xFFF5E3), AccentAlt: color.FromHex(0xF0B24D), PageBackground: color.FromHex(0xFAF3E6),
		GlowOne: color.FromRGBA8(238, 195, 119, 184), GlowTwo: color.FromRGBA8(255, 232, 184, 184), GlowThree: color.FromRGBA8(238, 226, 207, 184),
	},
	"madurai": {
		Name: "Madurai", Accent: color.FromHex(0xA04AA0), AccentStrong: color.FromHex(0x783278),
		AccentSoft: color.FromHex(0xFAEFFA), AccentAlt: color.FromHex(0xC77AC7), PageBackground: color.FromHex(0xF8F1F8),
		GlowOne: color.FromRGBA8(218, 177, 218, 184), GlowTwo: color.FromRGBA8(246, 218, 246, 184), GlowThree: color.FromRGBA8(233, 224, 233, 184),
	},
}

// AccentThemeNames returns the available accent theme keys in display order.
func AccentThemeNames() []string {
	names := make([]string, len(accentThemeOrder))
	copy(names, accentThemeOrder)
	return names
}

// AccentThemeByName returns a named Avyos accent theme.
func AccentThemeByName(name string) (AccentTheme, bool) {
	accent, ok := accentThemes[name]
	return accent, ok
}

// Dark returns the default dark theme.
func Dark() *ThemeData { return DarkWithAccent(defaultAccentTheme) }

// DarkWithAccent returns a dark theme using one of the named Avyos accents.
func DarkWithAccent(name string) *ThemeData {
	accent := resolveAccentTheme(name)
	return buildTheme(buildColorScheme(true, accent), accent)
}

// Light returns the default light theme.
func Light() *ThemeData { return LightWithAccent(defaultAccentTheme) }

// LightWithAccent returns a light theme using one of the named Avyos accents.
func LightWithAccent(name string) *ThemeData {
	accent := resolveAccentTheme(name)
	return buildTheme(buildColorScheme(false, accent), accent)
}

func resolveAccentTheme(name string) AccentTheme {
	if accent, ok := AccentThemeByName(name); ok {
		return accent
	}
	accent, _ := AccentThemeByName(defaultAccentTheme)
	return accent
}

func buildColorScheme(dark bool, accent AccentTheme) ColorScheme {
	if dark {
		return ColorScheme{
			Primary:                 accent.Accent,
			OnPrimary:               color.White,
			PrimaryContainer:        accent.AccentSoft.Lerp(color.FromHex(0x241D16), 0.82),
			OnPrimaryContainer:      accent.AccentAlt.Lerp(color.White, 0.18),
			Secondary:               accent.AccentAlt,
			OnSecondary:             color.FromHex(0x20140D),
			SecondaryContainer:      accent.AccentAlt.Lerp(color.FromHex(0x2B231A), 0.78),
			OnSecondaryContainer:    color.FromHex(0xFFE0BF),
			Tertiary:                color.FromHex(0x58C7B7),
			OnTertiary:              color.White,
			TertiaryContainer:       color.FromHex(0x173C36),
			OnTertiaryContainer:     color.FromHex(0xB7F2E9),
			Surface:                 color.FromHex(0x241D16),
			OnSurface:               color.FromHex(0xFFF8EF),
			SurfaceVariant:          color.FromHex(0x2B231A),
			OnSurfaceVariant:        color.FromHex(0xD8CBB8),
			Background:              color.FromHex(0x14110D),
			OnBackground:            color.FromHex(0xFFF8EF),
			Error:                   color.FromHex(0xF87171),
			OnError:                 color.White,
			ErrorContainer:          color.FromHex(0x4B1717),
			OnErrorContainer:        color.FromHex(0xFECACA),
			Outline:                 color.White.WithAlpha(0.16),
			OutlineVariant:          color.White.WithAlpha(0.08),
			Shadow:                  color.Black.WithAlpha(0.38),
			SurfaceDim:              color.FromHex(0x18130F),
			SurfaceBright:           color.FromHex(0x3A3025),
			SurfaceContainerLowest:  color.FromHex(0x14110D),
			SurfaceContainerLow:     color.FromHex(0x18130F),
			SurfaceContainer:        color.FromHex(0x241D16),
			SurfaceContainerHigh:    color.FromHex(0x2B231A),
			SurfaceContainerHighest: color.FromHex(0x3A3025),
			Scrim:                   color.Black.WithAlpha(0.62),
			InverseSurface:          color.FromHex(0xFFF8EF),
			OnInverseSurface:        color.FromHex(0x18130F),
			InversePrimary:          accent.AccentStrong,
			SurfaceTint:             accent.Accent,
			FocusRing:               accent.AccentStrong.Lerp(color.White, 0.20),
			Success:                 color.FromHex(0x34D399),
			OnSuccess:               color.White,
			SuccessContainer:        color.FromHex(0x123A2A),
			OnSuccessContainer:      color.FromHex(0xBBF7D0),
			Warning:                 accent.AccentAlt,
			OnWarning:               color.FromHex(0x20140D),
			WarningContainer:        color.FromHex(0x4A2812),
			OnWarningContainer:      color.FromHex(0xFED7AA),
			Info:                    color.FromHex(0x76B7EE),
			OnInfo:                  color.White,
			InfoContainer:           color.FromHex(0x173554),
			OnInfoContainer:         color.FromHex(0xBFDBFE),
		}
	}

	return ColorScheme{
		Primary:                 accent.Accent,
		OnPrimary:               color.White,
		PrimaryContainer:        accent.AccentSoft,
		OnPrimaryContainer:      accent.AccentStrong,
		Secondary:               accent.AccentAlt,
		OnSecondary:             color.FromHex(0x3E2D1C),
		SecondaryContainer:      accent.AccentAlt.Lerp(color.White, 0.72),
		OnSecondaryContainer:    accent.AccentStrong,
		Tertiary:                color.FromHex(0x1F9F8E),
		OnTertiary:              color.White,
		TertiaryContainer:       color.FromHex(0xE9F8F5),
		OnTertiaryContainer:     color.FromHex(0x167568),
		Surface:                 color.FromHex(0xFFFCF8),
		OnSurface:               color.FromHex(0x1C1917),
		SurfaceVariant:          color.FromHex(0xFFF8F2),
		OnSurfaceVariant:        color.FromHex(0x57534E),
		Background:              accent.PageBackground,
		OnBackground:            color.FromHex(0x1C1917),
		Error:                   color.FromHex(0xDC2626),
		OnError:                 color.White,
		ErrorContainer:          color.FromHex(0xFEE2E2),
		OnErrorContainer:        color.FromHex(0x991B1B),
		Outline:                 color.FromHex(0xF0E3D6),
		OutlineVariant:          color.FromHex(0xF4EAE0),
		Shadow:                  color.FromHex(0x3E2D1C).WithAlpha(0.10),
		SurfaceDim:              color.FromHex(0xEFE2D5),
		SurfaceBright:           color.FromHex(0xFFFCF8),
		SurfaceContainerLowest:  color.FromHex(0xFFFCF8),
		SurfaceContainerLow:     color.FromHex(0xFFF8F2),
		SurfaceContainer:        accent.AccentSoft,
		SurfaceContainerHigh:    color.FromHex(0xF6E9DC),
		SurfaceContainerHighest: color.FromHex(0xECDCCB),
		Scrim:                   color.FromHex(0x3E2D1C).WithAlpha(0.24),
		InverseSurface:          color.FromHex(0x1C1917),
		OnInverseSurface:        color.White,
		InversePrimary:          accent.AccentAlt,
		SurfaceTint:             accent.Accent,
		FocusRing:               accent.AccentStrong.WithAlpha(0.35),
		Success:                 color.FromHex(0x16A34A),
		OnSuccess:               color.White,
		SuccessContainer:        color.FromHex(0xDCFCE7),
		OnSuccessContainer:      color.FromHex(0x166534),
		Warning:                 accent.Accent,
		OnWarning:               color.White,
		WarningContainer:        color.FromHex(0xFFEDD5),
		OnWarningContainer:      color.FromHex(0x9A3412),
		Info:                    color.FromHex(0x3E8ED0),
		OnInfo:                  color.White,
		InfoContainer:           color.FromHex(0xEDF6FF),
		OnInfoContainer:         color.FromHex(0x2B67A0),
	}
}

func buildTheme(cs ColorScheme, accent AccentTheme) *ThemeData {
	fc := getFont()
	displayFace := fc.displayFace
	bodyFace := fc.bodyFace
	labelFace := fc.labelFace
	if bodyFace == nil {
		bodyFace = ttf.Default()
	}
	if displayFace == nil {
		displayFace = bodyFace
	}
	if labelFace == nil {
		labelFace = bodyFace
	}

	styled := func(face canvas.Typeface, size, lineHeightRatio float64, col color.Color, letterSpacingEm float64) TextStyle {
		return TextStyle{
			Face:          face,
			Size:          size,
			LineHeight:    size * lineHeightRatio,
			Color:         col,
			LetterSpacing: size * letterSpacingEm,
		}
	}

	size2XS := fc.labelSize * 0.72
	sizeXS := fc.labelSize * 0.86
	sizeSM := fc.labelSize
	sizeMD := fc.bodySize
	sizeLG := fc.leadSize
	sizeXL := fc.titleSize
	size2XL := fc.headlineSize
	size3XL := fc.headlineSize * 1.25
	size4XL := fc.headlineSize * 1.5
	size5XL := fc.displaySize
	size6XL := fc.displaySize * 1.25
	size7XL := fc.displaySize * 1.5

	text2XS := styled(labelFace, size2XS, 1.25, cs.OnSurfaceVariant, 0.05)
	textXS := styled(labelFace, sizeXS, 1.25, cs.OnSurfaceVariant, 0.025)
	textSM := styled(bodyFace, sizeSM, 1.5, cs.OnSurfaceVariant, 0)
	textMD := styled(bodyFace, sizeMD, 1.5, cs.OnSurface, 0)
	textLG := styled(bodyFace, sizeLG, 1.5, cs.OnSurface, 0)
	textXL := styled(displayFace, sizeXL, 1.375, cs.OnSurface, -0.025)
	text2XL := styled(displayFace, size2XL, 1.375, cs.OnSurface, -0.025)
	text3XL := styled(displayFace, size3XL, 1.25, cs.OnSurface, -0.025)
	text4XL := styled(displayFace, size4XL, 1.25, cs.OnSurface, -0.05)
	text5XL := styled(displayFace, size5XL, 1.25, cs.OnSurface, -0.05)
	text6XL := styled(displayFace, size6XL, 1.25, cs.OnSurface, -0.05)
	text7XL := styled(displayFace, size7XL, 1.25, cs.OnSurface, -0.05)

	labelLarge := styled(labelFace, sizeMD, 1.25, cs.OnSurface, 0.025)
	labelMedium := styled(labelFace, sizeSM, 1.25, cs.OnSurface, 0.025)
	labelSmall := styled(labelFace, sizeXS, 1.25, cs.OnSurfaceVariant, 0.05)

	return &ThemeData{
		ColorScheme: cs,
		TextTheme: TextTheme{
			Size2XS:        text2XS,
			SizeXS:         textXS,
			SizeSM:         textSM,
			SizeMD:         textMD,
			SizeLG:         textLG,
			SizeXL:         textXL,
			Size2XL:        text2XL,
			Size3XL:        text3XL,
			Size4XL:        text4XL,
			Size5XL:        text5XL,
			Size6XL:        text6XL,
			Size7XL:        text7XL,
			DisplayLarge:   text6XL,
			DisplayMedium:  text5XL,
			DisplaySmall:   text4XL,
			HeadlineLarge:  text3XL,
			HeadlineMedium: text2XL,
			HeadlineSmall:  textXL,
			TitleLarge:     styled(displayFace, sizeXL, 1.375, cs.OnSurface, -0.025),
			TitleMedium:    styled(displayFace, sizeMD, 1.375, cs.OnSurface, -0.02),
			TitleSmall:     styled(displayFace, sizeSM, 1.375, cs.OnSurface, -0.02),
			BodyLarge:      textLG,
			BodyMedium:     textMD,
			BodySmall:      textSM,
			LabelLarge:     labelLarge,
			LabelMedium:    labelMedium,
			LabelSmall:     labelSmall,
		},
		Shape: ShapeTheme{
			NoneRadius:     0,
			XXSmallRadius:  2,  // sm
			XSmallRadius:   4,  // base
			SmallRadius:    4,  // base (alias)
			MediumRadius:   6,  // md
			LargeRadius:    8,  // lg
			XLargeRadius:   12, // xl
			XXLargeRadius:  16, // 2xl
			XXXLargeRadius: 24, // 3xl
			FullRadius:     9999,
		},
		Space: SpaceTheme{Base: 4},
		Shadow: ShadowTheme{
			XS:  ShadowSpec{Blur: 3, OffsetY: 2, GlowSpread: 0},
			SM:  ShadowSpec{Blur: 5, OffsetY: 3, GlowSpread: 0},
			MD:  ShadowSpec{Blur: 5, OffsetY: 3, GlowSpread: 0},
			LG:  ShadowSpec{Blur: 12, OffsetY: 6, GlowSpread: 0},
			XL:  ShadowSpec{Blur: 24, OffsetY: 8, GlowSpread: 0},
			XXL: ShadowSpec{Blur: 32, OffsetY: 12, GlowSpread: 0},
		},
		Motion: MotionTheme{
			Fast:     100 * time.Millisecond,
			Moderate: 200 * time.Millisecond,
			Slow:     300 * time.Millisecond,
		},
		Accent: accent,
	}
}
