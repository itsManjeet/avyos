package main

import (
	"path/filepath"

	"avyos.dev/pkg/graphics/collections"
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

func (s *FilesState) Build(ctx widget.BuildContext) widget.Widget {
	backEnabled := s.historyIndex > 0
	forwardEnabled := s.historyIndex < len(s.history)-1

	return collections.Application{
		AppBar: &collections.AppBar{
			TitleWidget: widget.TextInput{
				Value:   &s.pathInput,
				Hint:    "Enter path",
				Variant: widget.TextInputFilled,
			},
			Actions: []widget.Widget{
				toolbarActionButton("go-previous-symbolic", "Back", backEnabled, s.goBack, ctx),
				toolbarActionButton("go-next-symbolic", "Forward", forwardEnabled, s.goForward, ctx),
				toolbarActionButton("go-jump-symbolic", "Go", true, func() { s.goToPath(s.pathInput) }, ctx),
				toolbarActionButton("view-refresh-symbolic", "Reload", true, func() { s.loadDirectory(s.currentPath) }, ctx),
			},
		},
		NavBar: &collections.NavBar{
			Destinations: locationDestinations(),
			Selected:     s.currentLocationIndex(),
			OnSelected:   s.selectLocation,
			Header:       widget.Text{Content: "Locations", Style: sectionTitleStyle(ctx)},
		},
		StatusBar: s.buildStatusBar(ctx),
		Body: widget.Scroll{
			Axis: layout.Vertical,
			Child: widget.Padding{
				Insets: layout.All(ctx.Theme.Space.Unit(3)),
				Child:  s.buildContentArea(ctx),
			},
		},
	}
}

func (s *FilesState) buildFilesTitle(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisSize:       layout.MainMin,
		Children: []widget.Widget{
			widget.Text{Content: "Files", Style: sectionTitleStyle(ctx)},
			widget.SizedBox{Height: th.Space.Unit(0.5)},
			widget.Text{Content: displayPath(s.currentPath), Style: mutedStyle(ctx)},
		},
	}
}

func (s *FilesState) buildLocationRow(loc locationItem, active bool, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	fill := color.Transparent
	border := color.Transparent
	if active {
		fill = th.ColorScheme.SurfaceContainerHighest
		border = th.ColorScheme.Outline
	}
	return widget.GestureDetector{
		OnTap: func() { s.goToPath(loc.Path) },
		Builder: func(state widget.InteractionState) widget.Widget {
			currentFill := fill
			if !active && state.Hovered {
				currentFill = th.ColorScheme.SurfaceContainer
			}
			return widget.Container{
				Fill:        currentFill,
				Radius:      th.Shape.LargeRadius,
				Border:      border,
				BorderWidth: 1,
				Padding:     layout.Symmetric(th.Space.Unit(2), th.Space.Unit(1.5)),
				Child: widget.Row{
					CrossAxisAlignment: layout.CrossCenter,
					Children: []widget.Widget{
						sidebarIcon(loc.Icon, ctx),
						widget.SizedBox{Width: th.Space.Unit(2)},
						widget.Expanded{Child: widget.Text{Content: loc.Label, Style: bodyStyle(ctx)}},
					},
				},
			}
		},
	}
}

func (s *FilesState) buildContentArea(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	if s.loadErr != "" {
		return widget.Container{
			Fill:        th.ColorScheme.SurfaceContainerLow,
			Radius:      th.Shape.XLargeRadius,
			Border:      th.ColorScheme.OutlineVariant,
			BorderWidth: 1,
			Padding:     layout.All(th.Space.Unit(4)),
			Child:       widget.Text{Content: s.loadErr, Style: bodyStyle(ctx)},
		}
	}

	children := make([]widget.Widget, 0, len(s.entries))
	for _, entry := range s.entries {
		entry := entry
		children = append(children, s.buildEntryIcon(entry, ctx))
	}
	if len(children) == 0 {
		children = append(children, widget.Text{Content: "This folder is empty.", Style: mutedStyle(ctx)})
	}

	return widget.Container{
		Fill:        th.ColorScheme.SurfaceContainerLow,
		Radius:      th.Shape.XLargeRadius,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.All(th.Space.Unit(3)),
		Child: widget.Wrap{
			Spacing:    th.Space.Unit(3),
			RunSpacing: th.Space.Unit(3),
			Children:   children,
		},
	}
}

func (s *FilesState) buildEntryIcon(entry fileEntry, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	active := entry.Path == s.selected
	fill := color.Transparent
	border := color.Transparent
	if active {
		fill = th.ColorScheme.SurfaceContainerHighest
		border = th.ColorScheme.Outline
	}
	return widget.GestureDetector{
		OnTap: func() { s.openEntry(entry) },
		Builder: func(state widget.InteractionState) widget.Widget {
			currentFill := fill
			if !active && state.Hovered {
				currentFill = th.ColorScheme.SurfaceContainer
			}
			return widget.Container{
				Width:       112,
				Fill:        currentFill,
				Radius:      th.Shape.XLargeRadius,
				Border:      border,
				BorderWidth: 1,
				Padding:     layout.All(th.Space.Unit(2)),
				Child: widget.Column{
					CrossAxisAlignment: layout.CrossCenter,
					MainAxisSize:       layout.MainMin,
					Children: []widget.Widget{
						contentIcon(entry, ctx),
						widget.SizedBox{Height: th.Space.Unit(1.5)},
						widget.Text{Content: entry.Name, Style: centeredBodyStyle(ctx)},
						widget.SizedBox{Height: th.Space.Unit(0.5)},
						widget.Text{Content: entryMeta(entry), Style: centeredMutedStyle(ctx)},
					},
				},
			}
		},
	}
}

func (s *FilesState) buildStatusBar(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	selected := "No selection"
	if s.selected != "" {
		selected = filepath.Base(s.selected)
	}
	return widget.Row{
		MainAxisAlignment:  layout.MainSpaceBetween,
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Text{Content: s.status, Style: mutedStyle(ctx)},
			widget.Row{
				CrossAxisAlignment: layout.CrossCenter,
				MainAxisSize:       layout.MainMin,
				Children: []widget.Widget{
					widget.Text{Content: displayPath(s.currentPath), Style: mutedStyle(ctx)},
					widget.SizedBox{Width: th.Space.Unit(3)},
					widget.Text{Content: selected, Style: mutedStyle(ctx)},
				},
			},
		},
	}
}

func toolbarActionButton(iconName, label string, enabled bool, onPress func(), ctx widget.BuildContext) widget.Widget {
	labelStyle := ctx.Theme.TextTheme.LabelSmall
	labelStyle.Color = ctx.Theme.ColorScheme.OnSurface
	if !enabled {
		labelStyle.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
	}

	btn := widget.Button{
		Child: widget.Icon{
			Name:     iconName,
			Size:     16,
			Fallback: fallbackIcon(label[:1], ctx),
		},
		Variant: widget.ButtonOutline,
		Tone:    widget.ButtonNeutral,
		Size:    widget.ButtonSmall,
	}
	if enabled {
		btn.OnPressed = onPress
	}
	return btn
}

func sidebarIcon(name string, ctx widget.BuildContext) widget.Widget {
	return widget.Icon{
		Name:     name,
		Size:     22,
		Fallback: fallbackIcon(name[:1], ctx),
	}
}

func contentIcon(entry fileEntry, ctx widget.BuildContext) widget.Widget {
	name := "text-x-generic"
	fallback := "F"
	if entry.IsDir {
		name = "default-folder"
		fallback = "D"
	}
	return widget.Container{
		Width:       64,
		Height:      64,
		Fill:        ctx.Theme.ColorScheme.SurfaceContainer,
		Radius:      ctx.Theme.Shape.LargeRadius,
		Border:      ctx.Theme.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Child: widget.Center(widget.Icon{
			Name:     name,
			Size:     48,
			Fallback: fallbackIcon(fallback, ctx),
		}),
	}
}

func fallbackIcon(label string, ctx widget.BuildContext) widget.Widget {
	return widget.Container{
		Width:  24,
		Height: 24,
		Fill:   ctx.Theme.ColorScheme.SurfaceContainerHighest,
		Radius: ctx.Theme.Shape.MediumRadius,
		Child:  widget.Center(widget.Text{Content: label, Style: bodyStyle(ctx)}),
	}
}

func entryMeta(entry fileEntry) string {
	if entry.IsDir {
		return "Folder"
	}
	return humanSize(entry.Size)
}

func displayPath(path string) string {
	clean := filepath.Clean(path)
	if clean == "." || clean == "" {
		return "/"
	}
	return clean
}

func sectionTitleStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.TitleMedium
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

func centeredBodyStyle(ctx widget.BuildContext) *theme.TextStyle {
	return bodyStyle(ctx)
}

func centeredMutedStyle(ctx widget.BuildContext) *theme.TextStyle {
	return mutedStyle(ctx)
}
