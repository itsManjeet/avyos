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
	"strconv"
	"sync"
	"time"

	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/font/ttf"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/ini"
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
		fs.Resolve("data:fonts/%s/%s.ttf", name, name),
		fs.Resolve("data:fonts/%s/%s-regular.ttf", name, name),
		fs.Resolve("data:fonts/%s.ttf", name),
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

	conf, err := ini.ParseFile(fs.Resolve("config:fonts.ini"))
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

// Dark returns the default dark theme.
func Dark() *ThemeData { return buildTheme(buildColorScheme(true)) }

// Light returns the default light theme.
func Light() *ThemeData { return buildTheme(buildColorScheme(false)) }

func buildColorScheme(dark bool) ColorScheme {
	if dark {
		return ColorScheme{
			Primary:                 color.FromHex(0x319795),
			OnPrimary:               color.White,
			PrimaryContainer:        color.FromHex(0x1D4044),
			OnPrimaryContainer:      color.FromHex(0x81E6D9),
			Secondary:               color.FromHex(0x3182CE),
			OnSecondary:             color.White,
			SecondaryContainer:      color.FromHex(0x1A365D),
			OnSecondaryContainer:    color.FromHex(0x90CDF4),
			Tertiary:                color.FromHex(0x805AD5),
			OnTertiary:              color.White,
			TertiaryContainer:       color.FromHex(0x322659),
			OnTertiaryContainer:     color.FromHex(0xD6BCFA),
			Surface:                 color.FromHex(0x1A202C),
			OnSurface:               color.White.WithAlpha(0.92),
			SurfaceVariant:          color.FromHex(0x2D3748),
			OnSurfaceVariant:        color.FromHex(0xA0AEC0),
			Background:              color.FromHex(0x171923),
			OnBackground:            color.White.WithAlpha(0.92),
			Error:                   color.FromHex(0xE53E3E),
			OnError:                 color.White,
			ErrorContainer:          color.FromHex(0x63171B),
			OnErrorContainer:        color.FromHex(0xFEB2B2),
			Outline:                 color.White.WithAlpha(0.16),
			OutlineVariant:          color.White.WithAlpha(0.08),
			Shadow:                  color.Black.WithAlpha(0.48),
			SurfaceDim:              color.FromHex(0x2D3748),
			SurfaceBright:           color.FromHex(0x4A5568),
			SurfaceContainerLowest:  color.FromHex(0x171923),
			SurfaceContainerLow:     color.FromHex(0x1A202C),
			SurfaceContainer:        color.FromHex(0x2D3748),
			SurfaceContainerHigh:    color.FromHex(0x4A5568),
			SurfaceContainerHighest: color.FromHex(0x718096),
			Scrim:                   color.Black.WithAlpha(0.60),
			InverseSurface:          color.White,
			OnInverseSurface:        color.FromHex(0x1A202C),
			InversePrimary:          color.FromHex(0x2C7A7B),
			SurfaceTint:             color.FromHex(0x319795),
			FocusRing:               color.FromHex(0x319795),
			Success:                 color.FromHex(0x38A169),
			OnSuccess:               color.White,
			SuccessContainer:        color.FromHex(0x1C4532),
			OnSuccessContainer:      color.FromHex(0x9AE6B4),
			Warning:                 color.FromHex(0xDD6B20),
			OnWarning:               color.White,
			WarningContainer:        color.FromHex(0x652B19),
			OnWarningContainer:      color.FromHex(0xFBD38D),
			Info:                    color.FromHex(0x3182CE),
			OnInfo:                  color.White,
			InfoContainer:           color.FromHex(0x1A365D),
			OnInfoContainer:         color.FromHex(0x90CDF4),
		}
	}

	return ColorScheme{
		Primary:                 color.FromHex(0x319795),
		OnPrimary:               color.White,
		PrimaryContainer:        color.FromHex(0xB2F5EA),
		OnPrimaryContainer:      color.FromHex(0x285E61),
		Secondary:               color.FromHex(0x3182CE),
		OnSecondary:             color.White,
		SecondaryContainer:      color.FromHex(0xBEE3F8),
		OnSecondaryContainer:    color.FromHex(0x2C5282),
		Tertiary:                color.FromHex(0x805AD5),
		OnTertiary:              color.White,
		TertiaryContainer:       color.FromHex(0xE9D8FD),
		OnTertiaryContainer:     color.FromHex(0x553C9A),
		Surface:                 color.White,
		OnSurface:               color.FromHex(0x1A202C),
		SurfaceVariant:          color.FromHex(0xF7FAFC),
		OnSurfaceVariant:        color.FromHex(0x4A5568),
		Background:              color.White,
		OnBackground:            color.FromHex(0x1A202C),
		Error:                   color.FromHex(0xE53E3E),
		OnError:                 color.White,
		ErrorContainer:          color.FromHex(0xFED7D7),
		OnErrorContainer:        color.FromHex(0x9B2C2C),
		Outline:                 color.FromHex(0xCBD5E0),
		OutlineVariant:          color.FromHex(0xE2E8F0),
		Shadow:                  color.FromHex(0x1A202C).WithAlpha(0.10),
		SurfaceDim:              color.FromHex(0xEDF2F7),
		SurfaceBright:           color.White,
		SurfaceContainerLowest:  color.White,
		SurfaceContainerLow:     color.FromHex(0xF7FAFC),
		SurfaceContainer:        color.FromHex(0xEDF2F7),
		SurfaceContainerHigh:    color.FromHex(0xE2E8F0),
		SurfaceContainerHighest: color.FromHex(0xCBD5E0),
		Scrim:                   color.FromHex(0x1A202C).WithAlpha(0.24),
		InverseSurface:          color.FromHex(0x1A202C),
		OnInverseSurface:        color.White,
		InversePrimary:          color.FromHex(0x81E6D9),
		SurfaceTint:             color.FromHex(0x319795),
		FocusRing:               color.FromHex(0x319795),
		Success:                 color.FromHex(0x38A169),
		OnSuccess:               color.White,
		SuccessContainer:        color.FromHex(0xC6F6D5),
		OnSuccessContainer:      color.FromHex(0x276749),
		Warning:                 color.FromHex(0xDD6B20),
		OnWarning:               color.White,
		WarningContainer:        color.FromHex(0xFEEBC8),
		OnWarningContainer:      color.FromHex(0x9C4221),
		Info:                    color.FromHex(0x3182CE),
		OnInfo:                  color.White,
		InfoContainer:           color.FromHex(0xBEE3F8),
		OnInfoContainer:         color.FromHex(0x2C5282),
	}
}

func buildTheme(cs ColorScheme) *ThemeData {
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
			XS:  ShadowSpec{Blur: 4, OffsetY: 1, GlowSpread: 0},
			SM:  ShadowSpec{Blur: 6, OffsetY: 1, GlowSpread: 0},
			MD:  ShadowSpec{Blur: 12, OffsetY: 4, GlowSpread: 0},
			LG:  ShadowSpec{Blur: 15, OffsetY: 10, GlowSpread: 0},
			XL:  ShadowSpec{Blur: 25, OffsetY: 20, GlowSpread: 0},
			XXL: ShadowSpec{Blur: 50, OffsetY: 25, GlowSpread: 0},
		},
		Motion: MotionTheme{
			Fast:     100 * time.Millisecond,
			Moderate: 200 * time.Millisecond,
			Slow:     300 * time.Millisecond,
		},
	}
}
