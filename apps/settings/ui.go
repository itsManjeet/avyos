package main

import (
	"fmt"
	"strings"

	"avyos.dev/lib/graphics/collections"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/theme"
	"avyos.dev/lib/graphics/widget"
	gsettings "avyos.dev/lib/settings"
)

func (s *SettingsState) Build(ctx widget.BuildContext) widget.Widget {
	page := s.currentPage()

	return collections.Application{
		Controller: s.appCtrl,
		AppBar: &collections.AppBar{
			TitleWidget: s.buildTitle(ctx, page),
			Actions: []widget.Widget{
				widget.Button{
					Child:     widget.Text{Content: "Reload"},
					Variant:   widget.ButtonOutline,
					Tone:      widget.ButtonNeutral,
					OnPressed: s.reload,
				},
				widget.Button{
					Child:     widget.Text{Content: "Clear Search"},
					Variant:   widget.ButtonGhost,
					Tone:      widget.ButtonNeutral,
					OnPressed: func() { s.SetState(func() { s.search = "" }) },
				},
			},
			Bottom: widget.TextInput{
				Value:   &s.search,
				Hint:    "Search settings",
				Variant: widget.TextInputFilled,
			},
		},
		NavBar: &collections.NavBar{
			Destinations: pageDestinations(),
			Selected:     s.page,
			OnSelected:   s.selectPage,
			Header:       widget.Text{Content: "Categories", Style: sectionTitleStyle(ctx)},
			Footer:       widget.Text{Content: "Run `settings list` or `settings get` for CLI access.", Style: mutedStyle(ctx)},
		},
		StatusBar: s.buildStatusBar(ctx),
		Body: widget.Scroll{
			Axis: layout.Vertical,
			Child: widget.Padding{
				Insets: layout.All(ctx.Theme.Space.Unit(4)),
				Child:  s.buildPage(ctx, page),
			},
		},
	}
}

func (s *SettingsState) buildTitle(ctx widget.BuildContext, page settingPage) widget.Widget {
	th := ctx.Theme
	subtitle := page.Summary
	if strings.TrimSpace(s.search) != "" {
		subtitle = fmt.Sprintf("Searching for %q across user and system settings", strings.TrimSpace(s.search))
	} else if page.Label == "Overview" {
		subtitle = "Personal and system preferences"
	}
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisSize:       layout.MainMin,
		Children: []widget.Widget{
			widget.Text{Content: "Settings", Style: sectionTitleStyle(ctx)},
			widget.SizedBox{Height: th.Space.Unit(0.5)},
			widget.Text{Content: subtitle, Style: mutedStyle(ctx)},
		},
	}
}

func (s *SettingsState) buildStatusBar(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	scopeState := "System read-only"
	if s.canWriteSystem {
		scopeState = "System writable"
	}
	return widget.Row{
		MainAxisAlignment:  layout.MainSpaceBetween,
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Text{Content: s.status, Style: mutedStyle(ctx)},
			widget.Row{
				MainAxisSize:       layout.MainMin,
				CrossAxisAlignment: layout.CrossCenter,
				Children: []widget.Widget{
					scopeBadge(scopeVisualUser, ctx),
					widget.SizedBox{Width: th.Space.Unit(1)},
					widget.Text{Content: s.userPath, Style: mutedStyle(ctx)},
					widget.SizedBox{Width: th.Space.Unit(3)},
					scopeBadge(scopeVisualSystem, ctx),
					widget.SizedBox{Width: th.Space.Unit(1)},
					widget.Text{Content: scopeState, Style: mutedStyle(ctx)},
				},
			},
		},
	}
}

func (s *SettingsState) buildPage(ctx widget.BuildContext, page settingPage) widget.Widget {
	if strings.TrimSpace(s.search) != "" {
		return s.buildSearchPage(ctx)
	}
	if page.Label == "Overview" {
		return s.buildOverviewPage(ctx)
	}

	children := []widget.Widget{
		s.pageHeader(page.Title, page.Summary, ctx),
	}

	if s.errMsg != "" {
		children = append(children,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(3)},
			s.bannerCard(s.errMsg, ctx.Theme.ColorScheme.Error.WithAlpha(0.12), ctx.Theme.ColorScheme.Error, ctx),
		)
	}
	if pageHasSystemItems(page) && !s.canWriteSystem {
		children = append(children,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(3)},
			s.bannerCard("System settings are visible, but editing them requires elevated permissions.", ctx.Theme.ColorScheme.FocusRing.WithAlpha(0.08), ctx.Theme.ColorScheme.Outline, ctx),
		)
	}

	matches := 0
	for _, section := range page.Sections {
		items := s.visibleItems(section.Items)
		if len(items) == 0 {
			continue
		}
		matches++
		children = append(children,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(4)},
			s.sectionCard(section, items, ctx),
		)
	}

	if matches == 0 {
		children = append(children,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(4)},
			s.emptySearchCard(ctx),
		)
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children:           children,
	}
}

func (s *SettingsState) buildOverviewPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	overviewCards := []widget.Widget{
		s.metricCard("Theme", s.stringValue(choiceSetting(gsettings.ScopeUser, "appearance.theme", "", "", "System", "System")), ctx),
		s.metricCard("Brightness", fmt.Sprintf("%.0f%%", s.floatValue(sliderSetting(gsettings.ScopeSystem, "display.brightness", "", "", 72, 0, 100, true))), ctx),
		s.metricCard("Output", fmt.Sprintf("%.0f%%", s.floatValue(sliderSetting(gsettings.ScopeUser, "sound.output_volume", "", "", 68, 0, 100, true))), ctx),
		s.metricCard("Profile", s.stringValue(textSetting(gsettings.ScopeUser, "profile.display_name", "", "", displayNameDefault(), "")), ctx),
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.pageHeader("Settings", "A control-center style surface for appearance, display, sound, privacy, and app defaults.", ctx),
			widget.SizedBox{Height: th.Space.Unit(4)},
			widget.Grid{
				MinChildWidth: 220,
				Gap:           th.Space.Unit(3),
				Children:      overviewCards,
			},
			widget.SizedBox{Height: th.Space.Unit(4)},
			s.bannerCard(fmt.Sprintf("User settings file: %s", s.userPath), th.ColorScheme.SurfaceContainer, th.ColorScheme.OutlineVariant, ctx),
			widget.SizedBox{Height: th.Space.Unit(3)},
			s.bannerCard(fmt.Sprintf("System settings file: %s", s.systemPath), th.ColorScheme.SurfaceContainer, th.ColorScheme.OutlineVariant, ctx),
			widget.SizedBox{Height: th.Space.Unit(4)},
			s.pageHeader("Categories", "Open a full control page, then use the sidebar for fast switching. The same binary also supports `settings list`, `get`, `set`, and `delete` from the terminal.", ctx),
			widget.SizedBox{Height: th.Space.Unit(3)},
			s.buildCategoryGrid(ctx),
		},
	}
}

func (s *SettingsState) buildSearchPage(ctx widget.BuildContext) widget.Widget {
	query := strings.TrimSpace(s.search)
	children := []widget.Widget{
		s.pageHeader("Search results", fmt.Sprintf("Matches for %q across all settings categories.", query), ctx),
	}

	matches := 0
	for i, page := range settingsPages {
		if page.Label == "Overview" {
			continue
		}

		var pageSections []widget.Widget
		for _, section := range page.Sections {
			items := make([]settingItem, 0, len(section.Items))
			for _, item := range section.Items {
				if s.searchMatches(query, page, section, item) {
					items = append(items, item)
				}
			}
			if len(items) == 0 {
				continue
			}
			matches++
			pageSections = append(pageSections,
				widget.SizedBox{Height: ctx.Theme.Space.Unit(3)},
				s.sectionCard(settingSection{
					Title:       page.Label + " · " + section.Title,
					Description: section.Description,
				}, items, ctx),
			)
		}
		if len(pageSections) == 0 {
			continue
		}

		children = append(children,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(4)},
			s.categoryCard(i, page, ctx),
		)
		children = append(children, pageSections...)
	}

	if matches == 0 {
		children = append(children,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(4)},
			s.emptySearchCard(ctx),
		)
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children:           children,
	}
}

func (s *SettingsState) pageHeader(title, summary string, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			widget.Text{Content: title, Style: pageTitleStyle(ctx)},
			widget.SizedBox{Height: th.Space.Unit(1)},
			widget.Text{Content: summary, Style: bodyStyle(ctx)},
		},
	}
}

func (s *SettingsState) sectionCard(section settingSection, items []settingItem, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	children := []widget.Widget{
		widget.Text{Content: section.Title, Style: sectionTitleStyle(ctx)},
	}
	if section.Description != "" {
		children = append(children,
			widget.SizedBox{Height: th.Space.Unit(1)},
			widget.Text{Content: section.Description, Style: mutedStyle(ctx)},
		)
	}
	for _, item := range items {
		children = append(children,
			widget.SizedBox{Height: th.Space.Unit(3)},
			s.buildItem(item, ctx),
		)
	}

	return widget.Container{
		Fill:        th.ColorScheme.SurfaceContainerLow,
		Radius:      th.Shape.XLargeRadius,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(3)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           children,
		},
	}
}

func (s *SettingsState) buildItem(item settingItem, ctx widget.BuildContext) widget.Widget {
	switch item.Kind {
	case kindToggle:
		return s.buildToggleItem(item, ctx)
	case kindSlider:
		return s.buildSliderItem(item, ctx)
	case kindText:
		return s.buildTextItem(item, ctx)
	case kindChoice:
		return s.buildChoiceItem(item, ctx)
	default:
		return widget.SizedBox{}
	}
}

func (s *SettingsState) buildToggleItem(item settingItem, ctx widget.BuildContext) widget.Widget {
	editable := s.isEditable(item)
	return widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Expanded{Child: s.itemHeader(item, ctx)},
			widget.Switch{
				Value: s.boolValue(item),
				OnChanged: func(v bool) {
					if editable {
						s.saveValue(item, v)
					}
				},
			},
		},
	}
}

func (s *SettingsState) buildSliderItem(item settingItem, ctx widget.BuildContext) widget.Widget {
	value := s.floatValue(item)
	display := fmt.Sprintf("%.0f", value)
	if !item.Whole {
		display = fmt.Sprintf("%.1f", value)
	}
	if item.Max <= 100 {
		display += "%"
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			widget.Row{
				CrossAxisAlignment: layout.CrossCenter,
				Children: []widget.Widget{
					widget.Expanded{Child: s.itemHeader(item, ctx)},
					widget.Text{Content: display, Style: sectionTitleStyle(ctx)},
				},
			},
			widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
			widget.Slider{
				Value: value,
				Min:   item.Min,
				Max:   item.Max,
				OnChanged: func(v float64) {
					if !s.isEditable(item) {
						return
					}
					if item.Whole {
						v = float64(int(v + 0.5))
					}
					s.saveValue(item, v)
				},
			},
		},
	}
}

func (s *SettingsState) buildTextItem(item settingItem, ctx widget.BuildContext) widget.Widget {
	key := draftKey(item)
	draft := s.drafts[key]
	if draft == nil {
		value := s.stringValue(item)
		draft = &value
		s.drafts[key] = draft
	}
	editable := s.isEditable(item)

	if !editable {
		return widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children: []widget.Widget{
				s.itemHeader(item, ctx),
				widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
				s.valueCard(*draft, ctx),
			},
		}
	}

	current := s.stringValue(item)
	apply := func() {
		s.saveValue(item, strings.TrimSpace(*draft))
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.itemHeader(item, ctx),
			widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
			widget.TextInput{
				Value:   draft,
				Hint:    item.Placeholder,
				Variant: widget.TextInputFilled,
			},
			widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
			widget.Row{
				CrossAxisAlignment: layout.CrossCenter,
				Children: []widget.Widget{
					widget.Button{
						Child:     widget.Text{Content: "Apply"},
						OnPressed: apply,
					},
					widget.SizedBox{Width: ctx.Theme.Space.Unit(1.5)},
					widget.Button{
						Child:     widget.Text{Content: "Reset"},
						Variant:   widget.ButtonOutline,
						Tone:      widget.ButtonNeutral,
						OnPressed: func() { s.SetState(func() { *draft = current }) },
					},
				},
			},
		},
	}
}

func (s *SettingsState) buildCategoryGrid(ctx widget.BuildContext) widget.Widget {
	cards := make([]widget.Widget, 0, len(settingsPages)-1)
	for i, page := range settingsPages {
		if page.Label == "Overview" {
			continue
		}
		cards = append(cards, s.categoryCard(i, page, ctx))
	}
	return widget.Grid{
		MinChildWidth: 240,
		Gap:           ctx.Theme.Space.Unit(3),
		Children:      cards,
	}
}

func (s *SettingsState) categoryCard(index int, page settingPage, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.GestureDetector{
		OnTap: func() { s.selectPage(index) },
		Builder: func(state widget.InteractionState) widget.Widget {
			fill := th.ColorScheme.SurfaceContainerLow
			border := th.ColorScheme.OutlineVariant
			if index == s.page {
				fill = th.ColorScheme.SurfaceContainerHigh
				border = th.ColorScheme.Outline
			} else if state.Hovered {
				fill = th.ColorScheme.SurfaceContainer
				border = th.ColorScheme.Outline
			}

			children := []widget.Widget{
				widget.Row{
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						widget.Icon{Name: page.Icon, Size: 22},
						widget.SizedBox{Width: th.Space.Unit(2)},
						widget.Expanded{Child: widget.Text{Content: page.Label, Style: sectionTitleStyle(ctx)}},
					},
				},
				widget.SizedBox{Height: th.Space.Unit(1.5)},
				widget.Text{Content: page.Summary, Style: mutedStyle(ctx)},
			}
			if page.SystemHeavy {
				children = append(children,
					widget.SizedBox{Height: th.Space.Unit(2)},
					scopeBadge(scopeVisualSystem, ctx),
				)
			}

			return widget.Container{
				Fill:        fill,
				Radius:      th.Shape.XLargeRadius,
				Border:      border,
				BorderWidth: 1,
				Padding:     layout.All(th.Space.Unit(3)),
				Child: widget.Column{
					CrossAxisAlignment: layout.CrossStretch,
					Children:           children,
				},
			}
		},
	}
}

func (s *SettingsState) buildChoiceItem(item settingItem, ctx widget.BuildContext) widget.Widget {
	selected := s.stringValue(item)
	buttons := make([]widget.Widget, 0, len(item.Options))
	for _, option := range orderedChoices(item, selected) {
		option := option
		variant := widget.ButtonOutline
		tone := widget.ButtonNeutral
		if option == selected {
			variant = widget.ButtonSolid
			tone = widget.ButtonPrimary
		}
		btn := widget.Button{
			Child:   widget.Text{Content: option},
			Variant: variant,
			Tone:    tone,
			Size:    widget.ButtonSmall,
		}
		if s.isEditable(item) {
			btn.OnPressed = func() { s.saveValue(item, option) }
		}
		buttons = append(buttons, btn)
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			s.itemHeader(item, ctx),
			widget.SizedBox{Height: ctx.Theme.Space.Unit(2)},
			widget.Wrap{
				Spacing:    ctx.Theme.Space.Unit(1.5),
				RunSpacing: ctx.Theme.Space.Unit(1.5),
				Children:   buttons,
			},
		},
	}
}

func (s *SettingsState) itemHeader(item settingItem, ctx widget.BuildContext) widget.Widget {
	scope := scopeVisualUser
	if item.Scope == gsettings.ScopeSystem {
		scope = scopeVisualSystem
	}

	rows := []widget.Widget{
		widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Expanded{Child: widget.Text{Content: item.Label, Style: sectionTitleStyle(ctx)}},
				scopeBadge(scope, ctx),
			},
		},
	}
	if item.Detail != "" {
		rows = append(rows,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(0.5)},
			widget.Text{Content: item.Detail, Style: mutedStyle(ctx)},
		)
	}
	if item.Scope == gsettings.ScopeSystem && !s.canWriteSystem {
		rows = append(rows,
			widget.SizedBox{Height: ctx.Theme.Space.Unit(0.5)},
			widget.Text{Content: "Read-only without elevated permissions.", Style: mutedStyle(ctx)},
		)
	}
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisSize:       layout.MainMin,
		Children:           rows,
	}
}

func (s *SettingsState) metricCard(label, value string, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Container{
		Fill:        th.ColorScheme.SurfaceContainerLow,
		Radius:      th.Shape.XLargeRadius,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(3)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children: []widget.Widget{
				widget.Text{Content: label, Style: mutedStyle(ctx)},
				widget.SizedBox{Height: th.Space.Unit(1)},
				widget.Text{Content: value, Style: pageTitleStyle(ctx)},
			},
		},
	}
}

func (s *SettingsState) bannerCard(message string, fill, border color.Color, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Container{
		Fill:        fill,
		Radius:      th.Shape.LargeRadius,
		Border:      border,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(2)),
		Child:       widget.Text{Content: message, Style: bodyStyle(ctx)},
	}
}

func (s *SettingsState) valueCard(value string, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Container{
		Fill:        th.ColorScheme.Surface,
		Radius:      th.Shape.LargeRadius,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(2)),
		Child:       widget.Text{Content: value, Style: bodyStyle(ctx)},
	}
}

func (s *SettingsState) emptySearchCard(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Container{
		Fill:        th.ColorScheme.SurfaceContainerLow,
		Radius:      th.Shape.XLargeRadius,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(4)),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Text{Content: "No matching settings", Style: pageTitleStyle(ctx)},
				widget.SizedBox{Height: th.Space.Unit(1)},
				widget.Text{Content: "Try a broader search term or open another category.", Style: mutedStyle(ctx)},
			},
		},
	}
}

func (s *SettingsState) isEditable(item settingItem) bool {
	return item.Scope != gsettings.ScopeSystem || s.canWriteSystem
}

func (s *SettingsState) searchMatches(query string, page settingPage, section settingSection, item settingItem) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		page.Label,
		page.Title,
		page.Summary,
		section.Title,
		section.Description,
		item.Label,
		item.Detail,
		item.Path,
		item.Scope.String(),
	}, " "))
	return strings.Contains(text, query)
}

type scopeVisual struct {
	Label string
	Fill  color.Color
	Text  color.Color
}

var (
	scopeVisualUser = scopeVisual{
		Label: "User",
		Fill:  color.FromRGBA8(31, 98, 179, 30),
		Text:  color.FromRGBA8(31, 98, 179, 255),
	}
	scopeVisualSystem = scopeVisual{
		Label: "System",
		Fill:  color.FromRGBA8(180, 109, 28, 30),
		Text:  color.FromRGBA8(180, 109, 28, 255),
	}
)

func scopeBadge(v scopeVisual, ctx widget.BuildContext) widget.Widget {
	style := ctx.Theme.TextTheme.LabelSmall
	style.Color = v.Text
	return widget.Container{
		Fill:    v.Fill,
		Radius:  ctx.Theme.Shape.FullRadius,
		Padding: layout.Symmetric(ctx.Theme.Space.Unit(1.5), ctx.Theme.Space.Unit(0.75)),
		Child:   widget.Text{Content: v.Label, Style: &style},
	}
}

func pageTitleStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.TitleLarge
	style.Color = ctx.Theme.ColorScheme.OnSurface
	return &style
}

func sectionTitleStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.TitleSmall
	style.Color = ctx.Theme.ColorScheme.OnSurface
	return &style
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
