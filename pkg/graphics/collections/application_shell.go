package collections

import (
	"avyos.dev/pkg/graphics/color"
	"avyos.dev/pkg/graphics/layout"
	"avyos.dev/pkg/graphics/theme"
	"avyos.dev/pkg/graphics/widget"
)

func buildResponsiveApplicationShell(app Application, ctrl *ApplicationController, autoDrawer *DrawerController, ctx widget.BuildContext, mode LayoutMode, body widget.Widget, sidebarVisible bool, toggleSidebar func()) widget.Widget {
	if body == nil {
		body = widget.SizedBox{}
	}

	if app.FAB != nil {
		th := ctx.Theme
		body = widget.Stack{
			Children: []widget.Widget{
				body,
				widget.Positioned{
					Right:  widget.Ptr(th.Space.Unit(4)),
					Bottom: widget.Ptr(th.Space.Unit(4)),
					Child:  app.FAB.Build(ctx),
				},
			},
		}
	}

	if app.NavBar != nil && mode == LayoutExpanded && sidebarVisible {
		body = expandedRow(
			app.NavBar.Build(ctx),
			widget.Expanded{Child: body},
		)
	}

	if app.BottomNav != nil && mode != LayoutExpanded && app.NavBar == nil {
		body = expandedCol(
			widget.Expanded{Child: body},
			app.BottomNav.Build(ctx),
		)
	}

	if cfg, ok := resolveApplicationDrawer(app, autoDrawer, ctx, mode); ok {
		body = Drawer{Config: cfg, Body: body}
	}

	content := widget.Widget(widget.Expanded{Child: body})
	if status := buildApplicationStatusBar(app, ctx, mode); status != nil {
		content = expandedCol(
			widget.Expanded{Child: body},
			status,
		)
	}

	if top := buildApplicationChrome(app, ctrl, autoDrawer, ctx, mode, sidebarVisible, toggleSidebar); top != nil {
		content = expandedCol(
			top,
			widget.Expanded{Child: content},
		)
	}

	return widget.Container{
		Fill:  ctx.Theme.ColorScheme.Background,
		Child: content,
	}
}

func resolveApplicationDrawer(app Application, autoDrawer *DrawerController, ctx widget.BuildContext, mode LayoutMode) (DrawerConfig, bool) {
	if mode == LayoutExpanded {
		return DrawerConfig{}, false
	}

	if app.NavBar != nil {
		cfg := DrawerConfig{
			Controller: autoDrawer,
			Child:      app.NavBar.Build(ctx),
		}
		if cfg.Width <= 0 {
			cfg.Width = resolveNavWidth(app.NavBar, ctx)
		}
		return cfg, true
	}

	if app.Drawer != nil {
		cfg := *app.Drawer
		if cfg.Controller == nil {
			cfg.Controller = autoDrawer
		}
		return cfg, true
	}

	return DrawerConfig{}, false
}

func resolveNavWidth(nav *NavBar, ctx widget.BuildContext) float64 {
	if nav == nil {
		return 0
	}
	if nav.Width > 0 {
		return nav.Width
	}
	if nav.Compact {
		return ctx.Theme.Space.Unit(14)
	}
	return ctx.Theme.Space.Unit(60)
}

func buildApplicationChrome(app Application, ctrl *ApplicationController, autoDrawer *DrawerController, ctx widget.BuildContext, mode LayoutMode, sidebarVisible bool, toggleSidebar func()) widget.Widget {
	if app.AppBar == nil && app.NavBar == nil && app.Drawer == nil {
		return nil
	}

	th := ctx.Theme
	bar := app.AppBar

	rowChildren := make([]widget.Widget, 0, 8)

	if app.NavBar != nil && mode == LayoutExpanded && toggleSidebar != nil {
		iconName := "sidebar-collapse-left"
		if !sidebarVisible {
			iconName = "sidebar-collapse-right"
		}
		rowChildren = append(rowChildren, applicationChromeIconButton(iconName, toggleSidebar, ctx))
	} else if mode != LayoutExpanded && (app.NavBar != nil || app.Drawer != nil) {
		rowChildren = append(rowChildren, applicationChromeIconButton("sidebar-collapse-left", autoDrawer.Toggle, ctx))
	}

	if bar != nil && bar.Leading != nil {
		if len(rowChildren) > 0 {
			rowChildren = append(rowChildren, widget.SizedBox{Width: th.Space.Unit(1.5)})
		}
		rowChildren = append(rowChildren, bar.Leading)
	}

	if len(rowChildren) > 0 {
		rowChildren = append(rowChildren, widget.SizedBox{Width: th.Space.Unit(2)})
	}
	rowChildren = append(rowChildren, widget.Expanded{Child: applicationBarTitle(bar, ctx)})

	if bar != nil {
		if mode == LayoutExpanded {
			for _, action := range bar.Actions {
				rowChildren = append(rowChildren, widget.SizedBox{Width: th.Space.Unit(1)}, action)
			}
		} else if len(bar.Actions) > 0 && ctrl != nil {
			rowChildren = append(rowChildren,
				widget.SizedBox{Width: th.Space.Unit(1)},
				applicationChromeIconButton("overflow-menu", func() {
					ctrl.TogglePanel("application-overflow", func() widget.Widget {
						return buildApplicationOverflowPanel(bar.Actions, ctrl, ctx)
					})
				}, ctx),
			)
		}
	}

	children := []widget.Widget{
		widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children:           rowChildren,
		},
	}

	if bar != nil && bar.Bottom != nil {
		children = append(children,
			widget.SizedBox{Height: th.Space.Unit(2)},
			bar.Bottom,
		)
	}

	vpad := th.Space.Unit(2.5)
	if bar != nil && bar.Compact {
		vpad = th.Space.Unit(2)
	}

	return widget.Container{
		Fill:        th.ColorScheme.SurfaceContainerLow,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.Symmetric(th.Space.Unit(3), vpad),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           children,
		},
	}
}

func buildApplicationOverflowPanel(actions []widget.Widget, ctrl *ApplicationController, ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	items := make([]widget.Widget, 0, len(actions)*2)
	for i, action := range actions {
		if i > 0 {
			items = append(items, widget.SizedBox{Height: th.Space.Unit(1)})
		}
		items = append(items, action)
	}

	backdrop := widget.GestureDetector{
		OnTap: func() {
			if ctrl != nil {
				ctrl.ClosePanel()
			}
		},
		Child: widget.Positioned{
			Top:    widget.Ptr(0),
			Right:  widget.Ptr(0),
			Bottom: widget.Ptr(0),
			Left:   widget.Ptr(0),
			Child:  widget.Container{Fill: color.Transparent},
		},
	}

	return widget.Stack{
		Children: []widget.Widget{
			backdrop,
			widget.Positioned{
				Top:   widget.Ptr(th.Space.Unit(12)),
				Right: widget.Ptr(th.Space.Unit(4)),
				Child: widget.SizedBox{
					Width: 220,
					Child: widget.Container{
						Fill:          th.ColorScheme.Surface,
						Radius:        th.Shape.LargeRadius,
						Border:        th.ColorScheme.Outline,
						BorderWidth:   1,
						Shadow:        th.ColorScheme.Shadow.WithAlpha(0.12),
						ShadowBlur:    th.Shadow.MD.Blur,
						ShadowOffsetY: th.Shadow.MD.OffsetY,
						Padding:       layout.All(th.Space.Unit(2)),
						Child: widget.Column{
							CrossAxisAlignment: layout.CrossStretch,
							Children:           items,
						},
					},
				},
			},
		},
	}
}

func buildApplicationStatusBar(app Application, ctx widget.BuildContext, mode LayoutMode) widget.Widget {
	th := ctx.Theme
	content := app.StatusBar
	if content == nil {
		label := "Desktop"
		if mode != LayoutExpanded {
			label = "Mobile"
		}
		content = widget.Row{
			MainAxisAlignment:  layout.MainSpaceBetween,
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Text{Content: "Ready", Style: applicationStatusStyle(ctx)},
				widget.Text{Content: label, Style: applicationStatusStyle(ctx)},
			},
		}
	}

	return widget.Container{
		Fill:        th.ColorScheme.SurfaceContainerLow,
		Border:      th.ColorScheme.OutlineVariant,
		BorderWidth: 1,
		Padding:     layout.Symmetric(th.Space.Unit(3), th.Space.Unit(1.5)),
		Child:       content,
	}
}

func applicationBarTitle(bar *AppBar, ctx widget.BuildContext) widget.Widget {
	if bar == nil {
		style := ctx.Theme.TextTheme.TitleMedium
		style.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
		return widget.Text{Content: "", Style: &style}
	}
	if bar.TitleWidget != nil {
		return bar.TitleWidget
	}
	if bar.Title == "" {
		return widget.SizedBox{}
	}
	style := ctx.Theme.TextTheme.TitleMedium
	style.Color = ctx.Theme.ColorScheme.OnSurface
	return widget.Text{Content: bar.Title, Style: &style}
}

func applicationStatusStyle(ctx widget.BuildContext) *theme.TextStyle {
	style := ctx.Theme.TextTheme.LabelSmall
	style.Color = ctx.Theme.ColorScheme.OnSurfaceVariant
	return &style
}

func applicationChromeIconButton(iconName string, onTap func(), ctx widget.BuildContext) widget.Widget {
	return widget.Button{
		Child: widget.SizedBox{
			Width:  18,
			Height: 18,
			Child: widget.Center(widget.Icon{
				Name: iconName,
				Size: 18,
			}),
		},
		Variant:   widget.ButtonGhost,
		Tone:      widget.ButtonNeutral,
		Size:      widget.ButtonSmall,
		OnPressed: onTap,
	}
}
