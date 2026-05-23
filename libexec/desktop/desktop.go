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

package main

import (
	"fmt"
	"image"
	"math"
	"strings"
	"time"

	"avyos.dev/api/desktop"
	"avyos.dev/api/login"
	"avyos.dev/api/service"
	"avyos.dev/lib/graphics/app"
	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/collections"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/shell"
	"avyos.dev/lib/graphics/widget"
)

// DesktopApp is the root StatefulWidget for the desktop compositor.
type DesktopApp struct{}

func (DesktopApp) CreateState() widget.State { return &DesktopState{} }

// DesktopState holds all mutable desktop compositor state.
type DesktopState struct {
	widget.StateBase

	background       widget.Image
	backgroundSource image.Image
	backgroundW      int
	backgroundH      int
	ctrl             *shell.Controller
	wins             []*ManagedWindow // z-ordered: last element is topmost
	focusedWinID     uint32           // 0 = none focused

	overlay *collections.OverlayManager
	panels  *collections.PanelController // exclusive panel manager (launcher, QS…)
	dialogs *collections.DialogController
	toast   *collections.ToastController
	notify  *notificationCenter
	apps    []launcherApp

	clock string // formatted current time

	// Logical screen dimensions used for maximize calculations.
	screenW, screenH float64

	// Damage tracking for the current frame.
	// damageAll means the full screen must be repainted (layout change).
	// damage is the union of all content-only dirty regions.
	// Both are reset by app.AfterFrame after every presented frame.
	damageAll bool
	damage    image.Rectangle
}

const (
	quickSettingsApproxH = 420.0
	toastDamageW         = 600.0
	toastDamageH         = 220.0
	damagePad            = 40.0
)

// addFullDamage signals that the entire screen needs repainting this frame.
func (s *DesktopState) addFullDamage() { s.damageAll = true }

func (s *DesktopState) reportDesktopError(title string, err error) {
	if s == nil || err == nil || s.toast == nil {
		return
	}
	s.toast.ShowFor(title+": "+err.Error(), collections.ToastError, 6*time.Second)
	s.addToastDamage()
}

func (s *DesktopState) focusedWindow() *ManagedWindow {
	for i := len(s.wins) - 1; i >= 0; i-- {
		if s.wins[i].Win.ID == s.focusedWinID {
			return s.wins[i]
		}
	}
	return nil
}

func (s *DesktopState) ensureBackground(screen geom.Size) {
	if s.backgroundSource == nil {
		return
	}
	w := int(math.Round(screen.Width))
	h := int(math.Round(screen.Height))
	if w <= 0 || h <= 0 {
		return
	}
	if s.backgroundW == w && s.backgroundH == h && s.background.Source != nil {
		return
	}
	if b := s.backgroundSource.Bounds(); b.Dx() == w && b.Dy() == h {
		s.background = widget.Image{Source: s.backgroundSource, Fit: widget.ImageFitStretch}
	} else {
		cv := pixbuf.NewCanvas(w, h)
		cv.DrawImage(s.backgroundSource, geom.NewRect(0, 0, float64(w), float64(h)))
		s.background = widget.Image{Source: cv.Image(), Fit: widget.ImageFitStretch}
	}
	s.backgroundW = w
	s.backgroundH = h
}

func snapWindowMetrics(mw *ManagedWindow) {
	if mw == nil {
		return
	}
	mw.X = math.Round(mw.X)
	mw.Y = math.Round(mw.Y)
	mw.W = math.Round(mw.W)
	mw.H = math.Round(mw.H)
	if mw.W < 1 {
		mw.W = 1
	}
	if mw.H < 1 {
		mw.H = 1
	}
}

// addContentDamage adds the full chrome rect of mw to the per-frame damage union.
// Using ChromeRect (not ContentRect) ensures the titlebar and borders are included
// in the DRM blit region so the back buffer never shows stale chrome.
func (s *DesktopState) addContentDamage(mw *ManagedWindow) {
	if s.damageAll {
		return
	}
	cx, cy, cw, ch := mw.ChromeRect()
	r := image.Rect(int(cx), int(cy), int(cx+cw), int(cy+ch))
	s.addRectDamage(r)
}

func (s *DesktopState) addRectDamage(r image.Rectangle) {
	if s.damageAll || r.Empty() {
		return
	}
	screen := image.Rect(0, 0, int(math.Round(s.screenW)), int(math.Round(s.screenH)))
	if screen.Dx() <= 0 || screen.Dy() <= 0 {
		return
	}
	r = r.Intersect(screen)
	if s.damage.Empty() {
		s.damage = r
	} else {
		s.damage = s.damage.Union(r)
	}
}

func (s *DesktopState) rectDamage(x, y, w, h float64) image.Rectangle {
	return image.Rect(
		int(math.Floor(x)),
		int(math.Floor(y)),
		int(math.Ceil(x+w)),
		int(math.Ceil(y+h)),
	)
}

func inflateRect(r image.Rectangle, pad float64) image.Rectangle {
	p := int(math.Ceil(pad))
	return image.Rect(r.Min.X-p, r.Min.Y-p, r.Max.X+p, r.Max.Y+p)
}

func (s *DesktopState) addShelfDamage() {
	if s.screenW <= 0 || s.screenH <= 0 {
		s.addFullDamage()
		return
	}
	s.addRectDamage(s.rectDamage(desktopShelfInset, s.screenH-desktopShelfReserve(), s.screenW-desktopShelfInset*2, shelfHeight))
}

func (s *DesktopState) addPanelDamage() {
	if s.screenW <= 0 || s.screenH <= 0 {
		s.addFullDamage()
		return
	}
	panelBottom := desktopPanelBottomInset()
	s.addRectDamage(inflateRect(s.rectDamage(desktopShelfInset, s.screenH-desktopShelfReserve(), s.screenW-desktopShelfInset*2, shelfHeight), damagePad))
	s.addRectDamage(inflateRect(s.rectDamage(quickSettingsScreenInset, s.screenH-panelBottom-launcherH, launcherW, launcherH), damagePad))
	s.addRectDamage(inflateRect(s.rectDamage(s.screenW-quickSettingsScreenInset-quickSettingsW, s.screenH-panelBottom-quickSettingsApproxH, quickSettingsW, quickSettingsApproxH), damagePad))
	s.addRectDamage(inflateRect(s.rectDamage(s.screenW-quickSettingsScreenInset-notificationsPanelW, s.screenH-panelBottom-420, notificationsPanelW, 420), damagePad))
}

func (s *DesktopState) addNotificationsDamage() {
	if s.screenW <= 0 || s.screenH <= 0 {
		s.addFullDamage()
		return
	}
	panelBottom := desktopPanelBottomInset()
	s.addRectDamage(inflateRect(s.rectDamage(desktopShelfInset, s.screenH-desktopShelfReserve(), s.screenW-desktopShelfInset*2, shelfHeight), damagePad))
	s.addRectDamage(inflateRect(s.rectDamage(s.screenW-quickSettingsScreenInset-notificationsPanelW, s.screenH-panelBottom-420, notificationsPanelW, 420), damagePad))
}

func (s *DesktopState) addToastDamage() {
	if s.screenW <= 0 || s.screenH <= 0 {
		s.addFullDamage()
		return
	}
	rightInset := quickSettingsScreenInset
	bottomInset := desktopPanelBottomInset()
	s.addRectDamage(inflateRect(s.rectDamage(s.screenW-toastDamageW-rightInset, s.screenH-toastDamageH-bottomInset, toastDamageW, toastDamageH), damagePad))
}

func (s *DesktopState) InitState() {
	s.screenW = float64(app.Options.Width)
	s.screenH = float64(app.Options.Height)

	s.overlay = collections.NewOverlayManager()
	s.panels = collections.NewPanelController(s.overlay)
	s.dialogs = collections.NewDialogController(s.overlay)
	s.panels.SetNotify(func() { s.SetState(func() { s.addPanelDamage() }) })
	s.toast = collections.NewToastController()
	s.toast.Watch(func() { s.SetState(func() { s.addToastDamage() }) })
	s.notify = newNotificationCenter()
	s.notify.SetNotify(func() { s.SetState(func() { s.addNotificationsDamage() }) })
	s.apps = discoverLauncherApps()

	s.ctrl = shell.NewController()
	app.EventHandler = func(e event.Event) {
		switch ev := e.(type) {
		case event.KeyEvent:
			if mw := s.focusedWindow(); mw != nil && !mw.Minimized {
				_ = mw.Win.SendInput(event.Encode(ev))
				return
			}
		case event.TextInputEvent:
			if mw := s.focusedWindow(); mw != nil && !mw.Minimized {
				_ = mw.Win.SendInput(event.Encode(ev))
				return
			}
		}
		app.DefaultHandler(e)
	}
	s.ctrl.Notify = func() { s.SetState(func() { s.addFullDamage() }) }
	s.ctrl.OnCreate = func(w *shell.Window) error {
		s.SetState(func() { s.addWindow(w); s.addFullDamage() })
		return nil
	}
	s.ctrl.OnBuffer = func(w *shell.Window) {
		// Buffer change may mean a resize — treat as layout change.
		s.SetState(func() { s.addFullDamage() })
	}
	s.ctrl.OnState = func(w *shell.Window) {
		// Title/subtitle change affects the titlebar — full damage is safe.
		s.SetState(func() { s.addFullDamage() })
	}
	s.ctrl.OnPresent = func(w *shell.Window) {
		s.SetState(func() {
			for _, mw := range s.wins {
				if mw.Win == w {
					s.addContentDamage(mw)
					break
				}
			}
		})
	}
	s.ctrl.OnCursor = func(w *shell.Window, shape event.CursorShape) {
		widget.EnqueueWork(func() {
			for _, mw := range s.wins {
				if mw.Win == w {
					mw.CursorShape = shape
					break
				}
			}
		})
	}
	s.ctrl.OnDestroy = func(w *shell.Window) {
		s.SetState(func() {
			s.removeWindow(w)
			// CloseBuffer runs here on the main goroutine, after removeWindow
			// has taken the window out of s.wins. Any in-flight Paint for this
			// window has already completed, so the mmap is safe to release.
			w.CloseBuffer()
			s.addFullDamage()
		})
	}

	var err error
	s.background, err = widget.NewImageFromFilePath("/usr/share/backgrounds/default.png")
	if err != nil {
		s.reportDesktopError("Wallpaper load failed", err)
	} else {
		s.backgroundSource = s.background.Source
		s.background.Fit = widget.ImageFitStretch
	}

	s.ctrl.OnNotification = func(_ uint32, req desktop.NotificationRequest) error {
		s.SetState(func() {
			s.notify.Add(req)
			s.toast.ShowDetailedFor(formatNotificationTitle(req), formatNotificationBody(req), collections.ToastInfo, 5*time.Second)
			s.addNotificationsDamage()
			s.addToastDamage()
		})
		return nil
	}
	if err := s.ctrl.Start(""); err != nil {
		s.reportDesktopError("Desktop service start failed", err)
	}

	// DamageProvider tells the app loop which pixels semantically changed.
	// When damageAll is set (layout change), return empty to use the full
	// canvas dirty. Otherwise return compositor-level semantic damage only.
	app.DamageProvider = func() image.Rectangle {
		if s.damageAll {
			return image.Rectangle{}
		}
		return s.damage
	}

	// Reset per-frame damage state after each presented frame.
	app.AfterFrame = func(_ *widget.Frame) {
		s.damageAll = false
		s.damage = image.Rectangle{}
	}

	go s.runClock()
}

func (s *DesktopState) runClock() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var last string
	for t := range ticker.C {
		formatted := t.Format("15:04")
		if formatted == last {
			continue // display unchanged (sub-minute tick) — skip redraw
		}
		last = formatted
		s.SetState(func() { s.clock = formatted; s.addFullDamage() })
	}
}

// addWindow appends a new ManagedWindow and focuses it.
// Must be called inside a SetState closure.
func (s *DesktopState) addWindow(w *shell.Window) {
	mw := &ManagedWindow{
		Win:         w,
		CursorShape: w.Cursor,
		X:           100,
		Y:           60,
		W:           float64(w.Width),
		H:           float64(w.Height),
	}
	if app := s.lookupLauncherApp(w); app != nil {
		mw.Icon = app.Icon
		if strings.TrimSpace(w.AppName) == "" {
			w.AppName = app.Name
		}
	}
	if mw.W <= 0 {
		mw.W = 640
	}
	if mw.H <= 0 {
		mw.H = 480
	}
	snapWindowMetrics(mw)
	s.wins = append(s.wins, mw)
	s.focusedWinID = w.ID
}

func (s *DesktopState) lookupLauncherApp(w *shell.Window) *launcherApp {
	if s == nil || w == nil {
		return nil
	}
	for i := range s.apps {
		app := &s.apps[i]
		if app.ID != "" && app.ID == w.AppID {
			return app
		}
	}
	for i := range s.apps {
		app := &s.apps[i]
		if app.Name != "" && strings.EqualFold(app.Name, w.AppName) {
			return app
		}
	}
	return nil
}

// removeWindow removes the ManagedWindow from the z-order list.
// Must be called inside a SetState closure.
func (s *DesktopState) removeWindow(w *shell.Window) {
	for i, mw := range s.wins {
		if mw.Win.ID == w.ID {
			s.wins = append(s.wins[:i], s.wins[i+1:]...)
			break
		}
	}
	if s.focusedWinID == w.ID {
		s.focusedWinID = 0
		for i := len(s.wins) - 1; i >= 0; i-- {
			if !s.wins[i].Minimized {
				s.focusedWinID = s.wins[i].Win.ID
				break
			}
		}
	}
}

// focusWindow moves a window to the top of the z-order, unminimizes it if
// needed, and sets focus. Must be called inside a SetState closure.
func (s *DesktopState) focusWindow(id uint32) {
	for i, mw := range s.wins {
		if mw.Win.ID == id {
			mw.Minimized = false
			s.wins = append(s.wins[:i], s.wins[i+1:]...)
			s.wins = append(s.wins, mw)
			break
		}
	}
	s.focusedWinID = id
	s.addFullDamage()
}

// minimizeWindow hides a window and shifts focus to the next visible window.
// Must be called inside a SetState closure.
func (s *DesktopState) minimizeWindow(id uint32) {
	for _, mw := range s.wins {
		if mw.Win.ID == id {
			mw.Minimized = true
			break
		}
	}
	if s.focusedWinID == id {
		s.focusedWinID = 0
		for i := len(s.wins) - 1; i >= 0; i-- {
			if !s.wins[i].Minimized {
				s.focusedWinID = s.wins[i].Win.ID
				break
			}
		}
	}
	s.addFullDamage()
}

// Build constructs the full desktop widget tree each frame.
func (s *DesktopState) Build(ctx widget.BuildContext) widget.Widget {
	s.screenW = ctx.ScreenSize.Width
	s.screenH = ctx.ScreenSize.Height
	s.ensureBackground(ctx.ScreenSize)

	children := make([]widget.Widget, 0, len(s.wins)+2)

	// Layer 0: wallpaper / background.
	children = append(children, s.background)

	// Layers 1..N: managed windows (minimized windows are hidden).
	for _, mw := range s.wins {
		if mw.Minimized {
			continue
		}
		mw := mw
		focused := mw.Win.ID == s.focusedWinID
		x := math.Round(mw.X)
		y := math.Round(mw.Y)
		w := math.Round(mw.W)
		h := math.Round(mw.H)
		chromeTotalW := w + borderWidth*2
		chromeTotalH := h + titlebarHeight + borderWidth*2

		children = append(children, widget.Positioned{
			Left:   widget.Ptr(x),
			Top:    widget.Ptr(y),
			Width:  widget.Ptr(chromeTotalW),
			Height: widget.Ptr(chromeTotalH),
			Child: WindowFrame{
				MW:      mw,
				Focused: focused,
				OnFocus: func() {
					s.SetState(func() { s.focusWindow(mw.Win.ID) })
				},
				OnClose: func() {
					_ = mw.Win.SendCloseRequested()
				},
				OnMinimize: func() {
					s.SetState(func() { s.minimizeWindow(mw.Win.ID) })
				},
				OnMaximize: func() {
					s.SetState(func() {
						if mw.Maximized {
							mw.X, mw.Y = mw.restoreX, mw.restoreY
							mw.W, mw.H = mw.restoreW, mw.restoreH
							mw.Maximized = false
						} else {
							mw.restoreX, mw.restoreY = mw.X, mw.Y
							mw.restoreW, mw.restoreH = mw.W, mw.H
							mw.X, mw.Y = 0, 0
							mw.W = s.screenW - borderWidth*2
							mw.H = s.screenH - desktopShelfReserve() - titlebarHeight - borderWidth*2
							mw.Maximized = true
						}
						snapWindowMetrics(mw)
						_ = mw.Win.SendResize(uint32(mw.W), uint32(mw.H))
						s.focusWindow(mw.Win.ID)
					})
				},
				OnMove: func(dx, dy float64) {
					s.SetState(func() {
						mw.X += dx
						mw.Y += dy
						snapWindowMetrics(mw)
						s.addFullDamage()
					})
				},
				OnResize: func(edge ResizeEdge, dx, dy float64) {
					s.SetState(func() {
						minW := float64(mw.Win.MinWidth)
						if minW < 100 {
							minW = 100
						}
						minH := float64(mw.Win.MinHeight)
						if minH < 100 {
							minH = 100
						}
						switch edge {
						case ResizeE:
							mw.W = max(mw.W+dx, minW)
						case ResizeW:
							newW := max(mw.W-dx, minW)
							mw.X += mw.W - newW
							mw.W = newW
						case ResizeS:
							mw.H = max(mw.H+dy, minH)
						case ResizeN:
							newH := max(mw.H-dy, minH)
							mw.Y += mw.H - newH
							mw.H = newH
						case ResizeSE:
							mw.W = max(mw.W+dx, minW)
							mw.H = max(mw.H+dy, minH)
						case ResizeSW:
							newW := max(mw.W-dx, minW)
							mw.X += mw.W - newW
							mw.W = newW
							mw.H = max(mw.H+dy, minH)
						case ResizeNE:
							mw.W = max(mw.W+dx, minW)
							newH := max(mw.H-dy, minH)
							mw.Y += mw.H - newH
							mw.H = newH
						case ResizeNW:
							newW := max(mw.W-dx, minW)
							mw.X += mw.W - newW
							mw.W = newW
							newH := max(mw.H-dy, minH)
							mw.Y += mw.H - newH
							mw.H = newH
						}
						snapWindowMetrics(mw)
						_ = mw.Win.SendResize(uint32(mw.W), uint32(mw.H))
						s.addFullDamage()
					})
				},
			},
		})
	}

	// Shelf (taskbar) — floated above the bottom edge.
	children = append(children, widget.Positioned{
		Left:   widget.Ptr(desktopShelfInset),
		Right:  widget.Ptr(desktopShelfInset),
		Bottom: widget.Ptr(desktopShelfInset),
		Height: widget.Ptr(shelfHeight),
		Child: Shelf{
			Wins:              s.wins,
			FocusedWinID:      s.focusedWinID,
			UnreadCount:       s.notify.UnreadCount(),
			NotificationsOpen: s.panels.IsOpen("notifications"),
			OnFocusWin: func(id uint32) {
				s.SetState(func() { s.focusWindow(id) })
			},
			OnMinimizeWin: func(id uint32) {
				s.SetState(func() { s.minimizeWindow(id) })
			},
			OnCloseWin: func(id uint32) {
				for _, mw := range s.wins {
					if mw.Win.ID == id {
						_ = mw.Win.SendCloseRequested()
						break
					}
				}
			},
			OnToggleLauncher: func() {
				s.panels.Toggle("launcher", func() widget.Widget {
					return widget.Positioned{
						Top: widget.Ptr(0.0), Right: widget.Ptr(0.0),
						Bottom: widget.Ptr(0.0), Left: widget.Ptr(0.0),
						Child: LauncherPanel{
							Apps: s.apps,
							OnLaunch: func(app launcherApp) {
								if err := launchApp(app, s.handleLaunchedAppExit); err != nil {
									s.SetState(func() {
										s.reportDesktopError(fmt.Sprintf("Failed to launch %s", app.Name), err)
									})
									return
								}
								s.panels.Close()
							},
							OnLogout:   s.confirmLogout,
							OnReboot:   s.confirmReboot,
							OnPoweroff: s.confirmPoweroff,
							OnClose:    s.panels.Close,
						},
					}
				})
			},
			OnToggleQuickSettings: func() {
				s.panels.Toggle("quick-settings", func() widget.Widget {
					return widget.Positioned{
						Top: widget.Ptr(0.0), Right: widget.Ptr(0.0),
						Bottom: widget.Ptr(0.0), Left: widget.Ptr(0.0),
						Child: QuickSettingsPanel{OnClose: s.panels.Close},
					}
				})
			},
			OnToggleNotifications: func() {
				s.panels.Toggle("notifications", func() widget.Widget {
					return widget.Positioned{
						Top: widget.Ptr(0.0), Right: widget.Ptr(0.0),
						Bottom: widget.Ptr(0.0), Left: widget.Ptr(0.0),
						Child: NotificationsPanel{
							Entries: s.notify.Snapshot(),
							OnClear: func(id int) {
								s.SetState(func() {
									s.notify.Clear(id)
									s.addNotificationsDamage()
								})
							},
							OnClearAll: func() {
								s.SetState(func() {
									s.notify.ClearAll()
									s.addNotificationsDamage()
								})
							},
							OnClose: s.panels.Close,
						},
					}
				})
			},
			LauncherOpen:      s.panels.IsOpen("launcher"),
			QuickSettingsOpen: s.panels.IsOpen("quick-settings"),
			Clock:             s.clock,
		},
	})

	desktop := widget.Stack{Children: children}

	return collections.ToastHost{
		Controller:  s.toast,
		RightInset:  quickSettingsScreenInset,
		BottomInset: desktopPanelBottomInset(),
		Child: collections.OverlayView{
			Manager: s.overlay,
			Child:   desktop,
		},
	}
}

func (s *DesktopState) showDesktopNotification(req desktop.NotificationRequest, variant collections.ToastVariant, duration time.Duration) {
	s.notify.Add(req)
	s.toast.ShowDetailedFor(formatNotificationTitle(req), formatNotificationBody(req), variant, duration)
	s.addNotificationsDamage()
	s.addToastDamage()
}

func formatNotificationTitle(req desktop.NotificationRequest) string {
	title := strings.TrimSpace(req.Title)
	if title != "" {
		return title
	}
	if appName := strings.TrimSpace(req.AppName); appName != "" {
		return appName
	}
	return "Notification"
}

func formatNotificationBody(req desktop.NotificationRequest) string {
	message := strings.TrimSpace(req.Message)
	if message != "" {
		return message
	}
	if appName := strings.TrimSpace(req.AppName); appName != "" && strings.TrimSpace(req.Title) != "" {
		return appName
	}
	return ""
}

func (s *DesktopState) handleLaunchedAppExit(report appExitReport) {
	if !report.crashed() {
		return
	}

	req := desktop.NotificationRequest{
		AppId:   "dev.avyos.desktop",
		AppName: "Desktop",
		Title:   fmt.Sprintf("%s crashed", report.App.Name),
		Message: formatCrashMessage(report),
		Icon:    "dialog-error",
	}

	s.SetState(func() {
		s.showDesktopNotification(req, collections.ToastError, 8*time.Second)
	})
}

func (s *DesktopState) confirmLogout() {
	s.confirmSystemAction("Log out?", "Your current desktop session will end and return to the login screen.", "Log out", func() error {
		client, err := login.Connect()
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Logout()
	})
}

func (s *DesktopState) confirmReboot() {
	s.confirmSystemAction("Restart device?", "All apps will close and the system will reboot.", "Restart", func() error {
		client, err := service.Connect()
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Reboot()
	})
}

func (s *DesktopState) confirmPoweroff() {
	s.confirmSystemAction("Shut down device?", "All apps will close and the device will power off.", "Shut down", func() error {
		client, err := service.Connect()
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Poweroff()
	})
}

func (s *DesktopState) confirmSystemAction(title, body, confirm string, action func() error) {
	s.panels.Close()
	if s.dialogs == nil {
		return
	}

	var closeDialog func()
	closeDialog = s.dialogs.Show(collections.Dialog{
		Title: title,
		Body:  widget.Text{Content: body},
		Actions: []widget.Widget{
			widget.Button{
				Child:   widget.Text{Content: "Cancel"},
				Variant: widget.ButtonOutline,
				Tone:    widget.ButtonNeutral,
				OnPressed: func() {
					if closeDialog != nil {
						closeDialog()
					}
				},
			},
			widget.Button{
				Child: widget.Text{Content: confirm},
				Tone:  widget.ButtonDanger,
				OnPressed: func() {
					if closeDialog != nil {
						closeDialog()
					}
					if err := action(); err != nil {
						s.SetState(func() {
							s.reportDesktopError(confirm+" failed", err)
						})
					}
				},
			},
		},
	})
}

func formatCrashMessage(report appExitReport) string {
	parts := []string{}
	if report.PID > 0 {
		parts = append(parts, fmt.Sprintf("PID %d", report.PID))
	}
	switch {
	case report.CoreDump && report.Signaled:
		parts = append(parts, fmt.Sprintf("terminated by %s", report.Signal))
		parts = append(parts, "core dumped")
	case report.Signaled:
		parts = append(parts, fmt.Sprintf("terminated by %s", report.Signal))
	case report.ExitCode != 0:
		parts = append(parts, fmt.Sprintf("exited with code %d", report.ExitCode))
	}
	if len(parts) == 0 && report.WaitErr != nil {
		parts = append(parts, report.WaitErr.Error())
	}
	return strings.Join(parts, " • ")
}
