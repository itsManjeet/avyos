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
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"avyos.dev/lib/fs"
	"avyos.dev/lib/graphics/app"
	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/collections"
	"avyos.dev/lib/graphics/color"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/graphics/layout"
	"avyos.dev/lib/graphics/widget"
	"avyos.dev/lib/identity"
	"avyos.dev/lib/logger"
)

var (
	log      *logger.Logger
	OOBE_TAG = "/var/lib/.oobe-done"
)

// LoginApp is the root StatefulWidget for the login screen.
type LoginApp struct{}

func (LoginApp) CreateState() widget.State { return &LoginState{} }

// LoginState holds mutable login form data.
type LoginState struct {
	widget.StateBase
	username        string
	password        string
	errMsg          string
	showOOBE        bool
	oobePage        int
	fullName        string
	setupPassword   string
	confirmPassword string
	deviceName      string
	oobeErr         string
	background      widget.Image
	backgroundSrc   image.Image
	backgroundW     int
	backgroundH     int
	logo            image.Image
}

func (s *LoginState) InitState() {
	s.showOOBE = !fs.Exists(OOBE_TAG)
	s.deviceName = "My avyos Device"
	s.backgroundSrc = loadImage("data:backgrounds/default_blur.png", "data:backgrounds/default_blur.jpg")
	s.logo = loadImage("data:icons/logo/logo.png")
}

func (s *LoginState) Build(ctx widget.BuildContext) widget.Widget {
	s.ensureBackground(ctx.ScreenSize)

	var background widget.Widget = widget.SizedBox{}
	if s.background.Source != nil {
		background = s.background
	}

	return widget.Stack{
		Children: []widget.Widget{
			background,
			widget.Container{Fill: color.Black.WithAlpha(0.28)},
			widget.Center(s.buildSurface(ctx)),
		},
	}
}

func (s *LoginState) ensureBackground(screen geom.Size) {
	if s.backgroundSrc == nil {
		s.background = widget.Image{}
		s.backgroundW = 0
		s.backgroundH = 0
		return
	}

	w := int(screen.Width + 0.5)
	h := int(screen.Height + 0.5)
	if w <= 0 || h <= 0 {
		return
	}
	if s.backgroundW == w && s.backgroundH == h && s.background.Source != nil {
		return
	}

	if b := s.backgroundSrc.Bounds(); b.Dx() == w && b.Dy() == h {
		s.background = widget.Image{Source: s.backgroundSrc, Fit: widget.ImageFitStretch}
	} else {
		cv := pixbuf.NewCanvas(w, h)
		cv.DrawImage(s.backgroundSrc, geom.NewRect(0, 0, float64(w), float64(h)))
		s.background = widget.Image{Source: cv.Image(), Fit: widget.ImageFitStretch}
	}

	s.backgroundW = w
	s.backgroundH = h
}

func (s *LoginState) buildSurface(ctx widget.BuildContext) widget.Widget {
	if s.showOOBE {
		return s.buildOOBE(ctx)
	}
	return s.buildLogin(ctx)
}

func (s *LoginState) buildLogin(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme

	var errWidget widget.Widget = widget.SizedBox{}
	if s.errMsg != "" {
		errWidget = widget.Container{
			Fill:    th.ColorScheme.Error,
			Radius:  th.Shape.SmallRadius,
			Padding: layout.All(10),
			Child:   widget.Text{Content: s.errMsg},
		}
	}

	return collections.Card{
		Child: widget.Padding{
			Insets: layout.All(40),
			Child: widget.Container{
				Width: 380,
				Child: widget.Column{
					CrossAxisAlignment: layout.CrossStretch,
					Children: []widget.Widget{
						widget.Text{Content: "Sign In", Style: &th.TextTheme.DisplaySmall},
						widget.SizedBox{Height: 6},
						widget.Text{Content: "Enter your credentials to continue", Style: &th.TextTheme.BodyMedium},
						widget.SizedBox{Height: 28},
						widget.TextInput{Value: &s.username, Hint: "Username"},
						widget.SizedBox{Height: 14},
						widget.TextInput{Value: &s.password, Hint: "Password", Obscure: true},
						widget.SizedBox{Height: 16},
						errWidget,
						widget.SizedBox{Height: 8},
						widget.Button{
							OnPressed: s.doLogin,
							Child:     widget.Text{Content: "Sign In"},
						},
					},
				},
			},
		},
	}
}

func (s *LoginState) buildOOBE(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	return widget.Padding{
		Insets: layout.All(th.Space.Unit(5)),
		Child: widget.Container{
			Width:  920,
			Height: 560,
			Child: collections.Card{
				Raised:  true,
				Padding: layout.All(th.Space.Unit(5)),
				Child: widget.Row{
					CrossAxisAlignment: layout.CrossStretch,
					Children: []widget.Widget{
						widget.Container{
							Width:   300,
							Fill:    th.ColorScheme.PrimaryContainer.WithAlpha(0.68),
							Radius:  th.Shape.XXLargeRadius,
							Padding: layout.All(th.Space.Unit(5)),
							Child:   s.buildOOBESidebar(ctx),
						},
						widget.SizedBox{Width: th.Space.Unit(5)},
						widget.Expanded{
							Child: widget.Column{
								CrossAxisAlignment: layout.CrossStretch,
								Children: []widget.Widget{
									widget.Container{
										Height: 40,
										Child:  s.buildOOBEHeader(ctx),
									},
									widget.SizedBox{Height: th.Space.Unit(4)},
									widget.Container{Height: 1, Fill: th.ColorScheme.OutlineVariant},
									widget.SizedBox{Height: th.Space.Unit(4)},
									widget.Container{
										Height: 340,
										Child:  s.buildOOBEPage(ctx),
									},
									widget.Spacer{},
									widget.Container{Height: 1, Fill: th.ColorScheme.OutlineVariant},
									widget.SizedBox{Height: th.Space.Unit(4)},
									widget.Container{
										Height: 56,
										Child:  s.buildOOBEActions(ctx),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (s *LoginState) buildOOBESidebar(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	bodyStyle := th.TextTheme.BodyMedium
	bodyStyle.Color = th.ColorScheme.OnSurfaceVariant
	labelStyle := th.TextTheme.LabelLarge

	steps := []string{"Welcome", "Create account", "Finish"}
	stepWidgets := make([]widget.Widget, 0, len(steps)*2)
	for i, step := range steps {
		accent := th.ColorScheme.Outline
		if i == s.oobePage {
			accent = th.ColorScheme.Primary
		}
		stepWidgets = append(stepWidgets, widget.Row{
			CrossAxisAlignment: layout.CrossCenter,
			Children: []widget.Widget{
				widget.Container{
					Width:  12,
					Height: 12,
					Radius: th.Shape.FullRadius,
					Fill:   accent,
				},
				widget.SizedBox{Width: th.Space.Unit(2)},
				widget.Text{Content: step, Style: &labelStyle},
			},
		})
		if i < len(steps)-1 {
			stepWidgets = append(stepWidgets, widget.SizedBox{Height: th.Space.Unit(3)})
		}
	}

	var logo widget.Widget = widget.SizedBox{}
	if s.logo != nil {
		logo = widget.Container{
			Width:   88,
			Height:  88,
			Radius:  th.Shape.XXLargeRadius,
			Fill:    color.White.WithAlpha(0.16),
			Padding: layout.All(th.Space.Unit(2)),
			Child:   widget.Image{Source: s.logo, Fit: widget.ImageFitContain},
		}
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		Children: []widget.Widget{
			logo,
			widget.SizedBox{Height: th.Space.Unit(4)},
			widget.Text{Content: "Set up avyos", Style: &th.TextTheme.DisplaySmall},
			widget.SizedBox{Height: th.Space.Unit(2)},
			widget.Text{Content: "Create your first account and prepare this device for daily use.", Style: &bodyStyle},
			widget.SizedBox{Height: th.Space.Unit(6)},
			widget.Column{
				CrossAxisAlignment: layout.CrossStretch,
				Children:           stepWidgets,
			},
			widget.Spacer{},
			widget.Text{Content: "Device", Style: &th.TextTheme.LabelMedium},
			widget.SizedBox{Height: th.Space.Unit(1)},
			widget.Text{Content: s.deviceLabel(), Style: &bodyStyle},
		},
	}
}

func (s *LoginState) buildOOBEHeader(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	labelStyle := th.TextTheme.LabelMedium
	labelStyle.Color = th.ColorScheme.OnSurfaceVariant

	return widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Text{Content: "Out-of-box setup", Style: &th.TextTheme.TitleLarge},
			widget.Spacer{},
			widget.Text{Content: s.pageTitle(), Style: &th.TextTheme.LabelLarge},
			widget.SizedBox{Width: th.Space.Unit(3)},
			widget.Text{Content: s.oobeProgressLabel(), Style: &labelStyle},
			widget.SizedBox{Width: th.Space.Unit(3)},
			s.buildOOBEDots(ctx),
		},
	}
}

func (s *LoginState) buildOOBEDots(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	children := make([]widget.Widget, 0, 5)
	for i := 0; i < 3; i++ {
		fill := th.ColorScheme.Outline
		width := th.Space.Unit(2)
		if i == s.oobePage {
			fill = th.ColorScheme.Primary
			width = th.Space.Unit(5)
		}
		children = append(children, widget.Container{
			Width:  width,
			Height: th.Space.Unit(1.5),
			Radius: th.Shape.FullRadius,
			Fill:   fill,
		})
		if i < 2 {
			children = append(children, widget.SizedBox{Width: th.Space.Unit(1)})
		}
	}
	return widget.Row{Children: children}
}

func (s *LoginState) buildOOBEPage(ctx widget.BuildContext) widget.Widget {
	switch s.oobePage {
	case 0:
		return s.buildWelcomePage(ctx)
	case 1:
		return s.buildUserSetupPage(ctx)
	default:
		return s.buildThanksPage(ctx)
	}
}

func (s *LoginState) buildWelcomePage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	bodyStyle := th.TextTheme.BodyLarge
	bodyStyle.Color = th.ColorScheme.OnSurfaceVariant

	logo := widget.SizedBox{}
	if s.logo != nil {
		logo = widget.SizedBox{
			Width:  96,
			Height: 96,
			Child: widget.Container{
				Radius:      th.Shape.XXLargeRadius,
				Fill:        color.White.WithAlpha(0.14),
				Border:      color.White.WithAlpha(0.22),
				BorderWidth: 1,
				Padding:     layout.All(th.Space.Unit(2)),
				Child:       widget.Image{Source: s.logo, Fit: widget.ImageFitContain},
			},
		}
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisAlignment:  layout.MainCenter,
		Children: []widget.Widget{
			widget.Center(logo),
			widget.SizedBox{Height: th.Space.Unit(4)},
			widget.Center(widget.Text{Content: "Welcome to avyos", Style: &th.TextTheme.DisplaySmall}),
			widget.SizedBox{Height: th.Space.Unit(2)},
			widget.Center(widget.Text{
				Content: "This guided setup will get your device ready before sign in.",
				Style:   &bodyStyle,
			}),
			widget.SizedBox{Height: th.Space.Unit(4)},
			widget.Container{
				Fill:        th.ColorScheme.PrimaryContainer.WithAlpha(0.55),
				Border:      th.ColorScheme.OutlineVariant,
				BorderWidth: 1,
				Radius:      th.Shape.XLargeRadius,
				Padding:     layout.All(th.Space.Unit(3)),
				Child:       widget.Center(widget.Text{Content: "Three quick steps. Then sign in.", Style: &th.TextTheme.LabelLarge}),
			},
			widget.SizedBox{Height: th.Space.Unit(4)},
			widget.Center(widget.Text{Content: "Continue to create your first local account.", Style: &bodyStyle}),
		},
	}
}

func (s *LoginState) buildUserSetupPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	bodyStyle := th.TextTheme.BodyMedium
	bodyStyle.Color = th.ColorScheme.OnSurfaceVariant

	var errWidget widget.Widget = widget.SizedBox{}
	if s.oobeErr != "" {
		errWidget = widget.Container{
			Fill:        th.ColorScheme.ErrorContainer.WithAlpha(0.95),
			Border:      th.ColorScheme.Error,
			BorderWidth: 1,
			Radius:      th.Shape.XLargeRadius,
			Padding:     layout.All(th.Space.Unit(3)),
			Child:       widget.Text{Content: s.oobeErr},
		}
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisSize:       layout.MainMin,
		Children: []widget.Widget{
			widget.Text{Content: "User setup", Style: &th.TextTheme.DisplaySmall},
			widget.SizedBox{Height: th.Space.Unit(2)},
			widget.Text{
				Content: "Create the first local user account for this system.",
				Style:   &bodyStyle,
			},
			widget.SizedBox{Height: th.Space.Unit(4)},
			errWidget,
			widget.SizedBox{Height: th.Space.Unit(3)},
			widget.Row{
				CrossAxisAlignment: layout.CrossStart,
				Children: []widget.Widget{
					widget.Expanded{
						Child: widget.Column{
							CrossAxisAlignment: layout.CrossStretch,
							Children: []widget.Widget{
								widget.TextInput{Value: &s.fullName, Label: "Full name", Hint: "Jane Doe"},
								widget.SizedBox{Height: th.Space.Unit(3)},
								widget.TextInput{Value: &s.username, Label: "Username", Hint: "jane"},
								widget.SizedBox{Height: th.Space.Unit(3)},
								widget.TextInput{Value: &s.deviceName, Label: "Device name", Hint: "Living Room PC"},
							},
						},
					},
					widget.SizedBox{Width: th.Space.Unit(4)},
					widget.Expanded{
						Child: widget.Column{
							CrossAxisAlignment: layout.CrossStretch,
							Children: []widget.Widget{
								widget.TextInput{Value: &s.setupPassword, Label: "Password", Hint: "Choose a password", Obscure: true},
								widget.SizedBox{Height: th.Space.Unit(3)},
								widget.TextInput{Value: &s.confirmPassword, Label: "Confirm password", Hint: "Repeat password", Obscure: true},
								widget.SizedBox{Height: th.Space.Unit(3)},
								widget.Container{
									Fill:        th.ColorScheme.SurfaceContainer.WithAlpha(0.8),
									Radius:      th.Shape.XLargeRadius,
									Border:      th.ColorScheme.OutlineVariant,
									BorderWidth: 1,
									Padding:     layout.All(th.Space.Unit(3)),
									Child:       widget.Text{Content: "The account will be created in /users/<username> with standard user capabilities.", Style: &th.TextTheme.LabelMedium},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (s *LoginState) buildThanksPage(ctx widget.BuildContext) widget.Widget {
	th := ctx.Theme
	bodyStyle := th.TextTheme.BodyLarge
	bodyStyle.Color = th.ColorScheme.OnSurfaceVariant

	var errWidget widget.Widget = widget.SizedBox{}
	if s.oobeErr != "" {
		errWidget = widget.Container{
			Fill:        th.ColorScheme.ErrorContainer.WithAlpha(0.95),
			Border:      th.ColorScheme.Error,
			BorderWidth: 1,
			Radius:      th.Shape.XLargeRadius,
			Padding:     layout.All(th.Space.Unit(3)),
			Child:       widget.Text{Content: s.oobeErr},
		}
	}

	name := s.fullName
	if name == "" {
		name = "there"
	}
	device := s.deviceName
	if device == "" {
		device = "your device"
	}

	return widget.Column{
		CrossAxisAlignment: layout.CrossStretch,
		MainAxisAlignment:  layout.MainCenter,
		Children: []widget.Widget{
			errWidget,
			widget.SizedBox{Height: th.Space.Unit(3)},
			widget.Text{Content: "Thanks, " + name, Style: &th.TextTheme.DisplaySmall},
			widget.SizedBox{Height: th.Space.Unit(2)},
			widget.Text{Content: device + " is ready. Finish setup to create the account and continue to sign in.", Style: &bodyStyle},
			widget.SizedBox{Height: th.Space.Unit(5)},
			widget.Container{
				Fill:        th.ColorScheme.SuccessContainer.WithAlpha(0.95),
				Border:      th.ColorScheme.Success,
				BorderWidth: 1,
				Radius:      th.Shape.XLargeRadius,
				Padding:     layout.All(th.Space.Unit(4)),
				Child:       widget.Text{Content: "After Finish, the regular login screen will appear.", Style: &th.TextTheme.TitleMedium},
			},
		},
	}
}

func (s *LoginState) buildOOBEActions(ctx widget.BuildContext) widget.Widget {
	backLabel := "Back"
	nextLabel := "Continue"
	if s.oobePage == 0 {
		backLabel = "Skip"
	}
	if s.oobePage == 2 {
		nextLabel = "Finish"
	}

	return widget.Row{
		CrossAxisAlignment: layout.CrossCenter,
		Children: []widget.Widget{
			widget.Button{
				Child:     widget.Text{Content: backLabel},
				Variant:   widget.ButtonOutline,
				Tone:      widget.ButtonNeutral,
				OnPressed: s.oobeBack,
			},
			widget.Spacer{},
			widget.Text{Content: s.oobeProgressLabel()},
			widget.SizedBox{Width: 16},
			widget.Button{
				Child:     widget.Text{Content: nextLabel},
				Size:      widget.ButtonLarge,
				OnPressed: s.oobeNext,
			},
		},
	}
}

func (s *LoginState) doLogin() {
	id, err := identity.Authenticate(s.username, s.password)
	if err != nil {
		s.SetState(func() { s.errMsg = err.Error() })
		return
	}
	if err := launchDesktop(id); err != nil {
		s.SetState(func() { s.errMsg = "Failed to launch desktop: " + err.Error() })
	}
}

func (s *LoginState) oobeBack() {
	s.SetState(func() {
		s.oobeErr = ""
		if s.oobePage > 0 {
			s.oobePage--
			return
		}
		s.showOOBE = false
	})
}

func (s *LoginState) oobeNext() {
	if s.oobePage < 2 {
		if s.oobePage == 1 {
			if err := s.validateOOBEAccount(); err != nil {
				s.SetState(func() { s.oobeErr = err.Error() })
				return
			}
		}
		s.SetState(func() {
			s.oobeErr = ""
			s.oobePage++
		})
		return
	}

	if err := s.finishOOBE(); err != nil {
		s.SetState(func() { s.oobeErr = "Failed to finish setup: " + err.Error() })
		return
	}

	s.SetState(func() {
		s.showOOBE = false
		s.oobePage = 0
		s.errMsg = ""
		s.oobeErr = ""
		s.password = ""
	})
}

func (s *LoginState) oobeProgressLabel() string {
	switch s.oobePage {
	case 0:
		return "Step 1 of 3"
	case 1:
		return "Step 2 of 3"
	default:
		return "Step 3 of 3"
	}
}

func (s *LoginState) pageTitle() string {
	switch s.oobePage {
	case 0:
		return "Welcome"
	case 1:
		return "Account"
	default:
		return "Finish"
	}
}

func (s *LoginState) deviceLabel() string {
	if strings.TrimSpace(s.deviceName) == "" {
		return "My avyos Device"
	}
	return strings.TrimSpace(s.deviceName)
}

func (s *LoginState) validateOOBEAccount() error {
	username := strings.TrimSpace(strings.ToLower(s.username))
	fullName := strings.TrimSpace(s.fullName)
	password := s.setupPassword
	confirm := s.confirmPassword

	if fullName == "" {
		return fmt.Errorf("full name is required")
	}
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`).MatchString(username) {
		return fmt.Errorf("username must start with a letter and use only lowercase letters, digits, _ or -")
	}
	if _, err := identity.LookupByName(username); err == nil {
		return fmt.Errorf("username %q already exists", username)
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if len(password) < 4 {
		return fmt.Errorf("password must be at least 4 characters")
	}
	if password != confirm {
		return fmt.Errorf("password confirmation does not match")
	}
	return nil
}

func (s *LoginState) finishOOBE() error {
	if err := s.validateOOBEAccount(); err != nil {
		return err
	}

	username := strings.TrimSpace(strings.ToLower(s.username))
	id := identity.Identity{
		Name: username,
		Capabilities: []string{
			"unix:users",
			"unix:audio",
			"unix:video",
			"unix:input",
			"unix:network",
			"unix:storage",
		},
		Home:  filepath.Join("/home", username),
		Shell: "/bin/sh",
	}
	if err := identity.AddIdentity(id, "user"); err != nil {
		return err
	}
	if err := identity.UpdatePassword(username, "", s.setupPassword); err != nil {
		return err
	}

	created, err := identity.LookupByName(username)
	if err != nil {
		return err
	}
	if err := ensureHomeDir(created); err != nil {
		return err
	}
	if err := completeOOBE(); err != nil {
		return err
	}

	s.username = username
	return nil
}

func completeOOBE() error {
	if err := os.MkdirAll(filepath.Dir(OOBE_TAG), 0755); err != nil {
		return err
	}
	return os.WriteFile(OOBE_TAG, []byte("done\n"), 0644)
}

func ensureHomeDir(id *identity.Identity) error {
	if id == nil || id.Home == "" {
		return nil
	}
	if err := os.MkdirAll(id.Home, 0755); err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(id.Home, id.ID, id.ID); err != nil {
			return err
		}
	}
	return nil
}

func runtimeDirForUID(uid int) string {
	return filepath.Join("/run/user", strconv.Itoa(uid))
}

func launchDesktop(id *identity.Identity) error {
	desktopPath := "/usr/libexec/desktop"
	xdgRuntime := runtimeDirForUID(id.ID)
	if err := os.MkdirAll("/run/user", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(xdgRuntime, 0700); err != nil {
		return err
	}
	if err := os.Chmod(xdgRuntime, 0700); err != nil {
		return err
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(xdgRuntime, id.ID, id.ID); err != nil {
			return err
		}
	}

	os.Setenv("HOME", id.Home)
	os.Setenv("USER", id.Name)
	os.Setenv("LOGNAME", id.Name)
	os.Setenv("PATH", fmt.Sprintf("%s/.local/bin:/opt/bin:/bin:/usr/bin", id.Home))

	if err := syscall.Setgid(int(id.ID)); err != nil {
		return err
	}

	if err := syscall.Setgroups(id.GetGroupIDs()); err != nil {
		return err
	}

	if err := syscall.Setuid(id.ID); err != nil {
		return err
	}

	return syscall.Exec(desktopPath, []string{desktopPath}, os.Environ())
}

func loadImage(uris ...string) image.Image {
	for _, uri := range uris {
		file, err := os.Open(fmt.Sprintf("%s", uri))
		if err != nil {
			continue
		}
		img, _, err := image.Decode(file)
		file.Close()
		if err == nil {
			return img
		}
	}
	return nil
}

func init() {
	log = logger.New("dev.avyos.login")
	_ = logger.SetupLog()
}

func main() {
	if err := app.Run(LoginApp{}); err != nil {
		log.Error("failed to start LoginApp %v", err)
		os.Exit(1)
	}
}
