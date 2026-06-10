package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	desktopapi "avyos.dev/api/desktop"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/widget"
)

const notificationsPanelW = 360.0

type notificationEntry struct {
	ID        int
	AppID     string
	AppName   string
	Title     string
	Message   string
	Icon      string
	CreatedAt time.Time
}

type notificationCenter struct {
	mu      sync.Mutex
	nextID  int
	entries []notificationEntry
	notify  func()
}

func newNotificationCenter() *notificationCenter {
	return &notificationCenter{}
}

func (nc *notificationCenter) SetNotify(fn func()) {
	nc.mu.Lock()
	nc.notify = fn
	nc.mu.Unlock()
}

func (nc *notificationCenter) Add(req desktopapi.NotificationRequest) notificationEntry {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.nextID++
	entry := notificationEntry{
		ID:        nc.nextID,
		AppID:     strings.TrimSpace(req.AppId),
		AppName:   strings.TrimSpace(req.AppName),
		Title:     strings.TrimSpace(req.Title),
		Message:   strings.TrimSpace(req.Message),
		Icon:      strings.TrimSpace(req.Icon),
		CreatedAt: time.Now(),
	}
	if entry.Title == "" {
		entry.Title = "Notification"
	}
	if entry.AppName == "" {
		entry.AppName = entry.AppID
	}
	if entry.AppName == "" {
		entry.AppName = "Application"
	}

	nc.entries = append([]notificationEntry{entry}, nc.entries...)
	if notify := nc.notify; notify != nil {
		go notify()
	}
	return entry
}

func (nc *notificationCenter) Snapshot() []notificationEntry {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	out := make([]notificationEntry, len(nc.entries))
	copy(out, nc.entries)
	return out
}

func (nc *notificationCenter) UnreadCount() int {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return len(nc.entries)
}

func (nc *notificationCenter) Clear(id int) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	for i, entry := range nc.entries {
		if entry.ID != id {
			continue
		}
		nc.entries = append(nc.entries[:i], nc.entries[i+1:]...)
		if notify := nc.notify; notify != nil {
			go notify()
		}
		return
	}
}

func (nc *notificationCenter) ClearAll() {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	if len(nc.entries) == 0 {
		return
	}
	nc.entries = nil
	if notify := nc.notify; notify != nil {
		go notify()
	}
}

func notificationBanners(ctx widget.BuildContext, entries []notificationEntry) widget.Widget {
	if len(entries) == 0 {
		return widget.SizedBox{}
	}
	if len(entries) > 3 {
		entries = entries[:3]
	}
	cards := make([]widget.Widget, 0, len(entries)*2)
	for i, entry := range entries {
		if i > 0 {
			cards = append(cards, widget.SizedBox{Height: 10})
		}
		cards = append(cards, notificationCard(ctx, entry, nil))
	}
	return widget.Column{
		CrossAxisAlignment: layout.CrossEnd,
		Children:           cards,
	}
}

type NotificationsPanel struct {
	Entries    []notificationEntry
	OnClear    func(id int)
	OnClearAll func()
	OnClose    func()
}

func (p NotificationsPanel) Build(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	backdrop := widget.GestureDetector{
		OnTap: p.OnClose,
		Child: widget.Container{Fill: color.Color{A: 0}},
	}

	titleSt := th.TextTheme.TitleMedium
	titleSt.Color = th.ColorScheme.OnSurface
	metaSt := th.TextTheme.LabelSmall
	metaSt.Color = th.ColorScheme.OnSurfaceVariant

	clearAll := widget.GestureDetector{
		OnTap: p.OnClearAll,
		Builder: func(state widget.InteractionState) widget.Widget {
			st := metaSt
			if state.Hovered {
				st.Color = th.ColorScheme.OnSurface
			}
			return widget.Container{
				Padding: layout.Symmetric(8, 4),
				Child:   widget.Text{Content: "Clear all", Style: &st},
			}
		},
	}

	items := make([]widget.Widget, 0, len(p.Entries))
	for _, entry := range p.Entries {
		entry := entry
		items = append(items, notificationCard(ctx, entry, func() {
			if p.OnClear != nil {
				p.OnClear(entry.ID)
			}
		}))
	}
	if len(items) == 0 {
		items = append(items, widget.Container{
			Padding: layout.Symmetric(12, 24),
			Child:   widget.Text{Content: "No unread notifications", Style: &metaSt},
		})
	}

	panel := widget.Container{
		Width:         notificationsPanelW,
		Height:        420,
		Fill:          th.ColorScheme.Surface,
		Radius:        12,
		Shadow:        th.ColorScheme.Shadow,
		ShadowBlur:    th.Shadow.LG.Blur,
		ShadowOffsetY: -4,
		Padding:       layout.All(16),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children: []widget.Widget{
				widget.Container{
					Height: 32,
					Child: widget.Row{
						CrossAxisAlignment: layout.CrossCenter,
						Children: []widget.Widget{
							widget.Text{Content: "Notifications", Style: &titleSt},
							widget.Spacer{},
							clearAll,
						},
					},
				},
				widget.SizedBox{Height: 12},
				widget.Container{
					Height: 344,
					Child: widget.Scroll{
						Axis:   layout.Vertical,
						Height: 344,
						Child: widget.Column{
							CrossAxisAlignment: layout.CrossStretch,
							Children:           items,
						},
					},
				},
			},
		},
	}

	return widget.Stack{
		Children: []widget.Widget{
			widget.Positioned{
				Top: widget.Ptr(0.0), Right: widget.Ptr(0.0),
				Bottom: widget.Ptr(0.0), Left: widget.Ptr(0.0),
				Child: backdrop,
			},
			widget.Positioned{
				Right:  widget.Ptr(quickSettingsScreenInset),
				Bottom: widget.Ptr(shelfHeight + quickSettingsGap),
				Child:  panel,
			},
		},
	}
}

func notificationCard(ctx widget.BuildContext, entry notificationEntry, onClear func()) widget.Widget {
	th := ctx.Theme

	titleSt := th.TextTheme.LabelMedium
	titleSt.Color = th.ColorScheme.OnSurface
	appSt := th.TextTheme.LabelSmall
	appSt.Color = th.ColorScheme.OnSurfaceVariant
	bodySt := th.TextTheme.BodySmall
	bodySt.Color = th.ColorScheme.OnSurfaceVariant

	icon := widget.Container{
		Width:  28,
		Height: 28,
		Fill:   th.ColorScheme.PrimaryContainer,
		Radius: 8,
		Child: widget.Center(widget.Icon{Name: entry.Icon, Size: 16,
			Fallback: widget.Text{Content: "•", Style: &th.TextTheme.LabelMedium},
		}),
	}

	headerChildren := []widget.Widget{
		icon,
		widget.SizedBox{Width: 10},
		widget.Expanded{
			Child: widget.Column{
				CrossAxisAlignment: layout.CrossStart,
				Children: []widget.Widget{
					widget.Text{Content: entry.Title, Style: &titleSt},
					widget.Text{Content: fmt.Sprintf("%s • %s", entry.AppName, entry.CreatedAt.Format("15:04")), Style: &appSt},
				},
			},
		},
	}

	if onClear != nil {
		headerChildren = append(headerChildren, widget.GestureDetector{
			OnTap: onClear,
			Builder: func(state widget.InteractionState) widget.Widget {
				st := appSt
				if state.Hovered {
					st.Color = th.ColorScheme.OnSurface
				}
				return widget.Container{
					Padding: layout.Symmetric(6, 4),
					Child:   widget.Text{Content: "×", Style: &st},
				}
			},
		})
	}

	children := []widget.Widget{
		widget.Row{
			CrossAxisAlignment: layout.CrossStart,
			Children:           headerChildren,
		},
	}
	if entry.Message != "" {
		children = append(children,
			widget.SizedBox{Height: 8},
			widget.Text{Content: entry.Message, Style: &bodySt},
		)
	}

	return widget.Container{
		Fill:    th.ColorScheme.SurfaceContainer,
		Radius:  12,
		Padding: layout.All(12),
		Child: widget.Column{
			CrossAxisAlignment: layout.CrossStretch,
			Children:           children,
		},
	}
}
