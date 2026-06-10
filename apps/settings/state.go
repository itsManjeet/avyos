package main

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"avyos.dev/lib/graphics/collections"
	"avyos.dev/lib/graphics/widget"
	gsettings "avyos.dev/lib/settings"
)

type settingKind uint8

const (
	kindToggle settingKind = iota
	kindSlider
	kindText
	kindChoice
)

type settingItem struct {
	Scope       gsettings.Scope
	Path        string
	Label       string
	Detail      string
	Kind        settingKind
	Default     any
	Min         float64
	Max         float64
	Whole       bool
	Placeholder string
	Options     []string
}

type settingSection struct {
	Title       string
	Description string
	Items       []settingItem
}

type settingPage struct {
	Label       string
	Icon        string
	Title       string
	Summary     string
	Sections    []settingSection
	SystemHeavy bool
}

var settingsPages = []settingPage{
	{
		Label:   "Overview",
		Icon:    "preferences-system",
		Title:   "Settings",
		Summary: "Personalize the desktop, inspect system defaults, and tune app behavior from one surface.",
	},
	{
		Label:   "Appearance",
		Icon:    "preferences-desktop-theme",
		Title:   "Appearance",
		Summary: "Theme, accent, density, and motion preferences that shape the whole desktop feel.",
		Sections: []settingSection{
			{
				Title:       "Theme",
				Description: "Core appearance choices for the shell and apps.",
				Items: []settingItem{
					choiceSetting(gsettings.ScopeUser, "appearance.theme", "Theme", "Switch the primary surface mood.", "System", "System", "Light", "Dark"),
					choiceSetting(gsettings.ScopeUser, "appearance.accent", "Accent color", "Used for highlights, selected items, and primary actions.", "Ocean", "Ocean", "Amber", "Forest", "Rose"),
					choiceSetting(gsettings.ScopeUser, "appearance.density", "Spacing density", "Controls how roomy lists and cards feel.", "Comfortable", "Compact", "Comfortable", "Spacious"),
				},
			},
			{
				Title:       "Motion",
				Description: "Visual feedback and transitions.",
				Items: []settingItem{
					toggleSetting(gsettings.ScopeUser, "appearance.animations", "Animations", "Enable interface transitions and motion accents.", true),
					toggleSetting(gsettings.ScopeUser, "appearance.blur", "Surface blur", "Use translucent blurred panels where supported.", true),
				},
			},
		},
	},
	{
		Label:       "Display",
		Icon:        "video-display",
		Title:       "Display",
		Summary:     "Monitor scale, brightness, and comfort features.",
		SystemHeavy: true,
		Sections: []settingSection{
			{
				Title:       "Screen",
				Description: "Visual comfort and scaling.",
				Items: []settingItem{
					sliderSetting(gsettings.ScopeSystem, "display.brightness", "Brightness", "System-wide screen brightness target.", float64(72), 15, 100, true),
					sliderSetting(gsettings.ScopeUser, "display.scale", "Interface scale", "Relative UI scale for this account.", float64(100), 75, 200, true),
					toggleSetting(gsettings.ScopeUser, "display.night_light", "Night light", "Reduce blue light after sunset.", false),
					choiceSetting(gsettings.ScopeUser, "display.color_profile", "Color profile", "Quick tuning for panel warmth.", "Balanced", "Balanced", "Warm", "Cool"),
				},
			},
		},
	},
	{
		Label:   "Sound",
		Icon:    "audio-speakers",
		Title:   "Sound",
		Summary: "Output level, alerts, and microphone behavior.",
		Sections: []settingSection{
			{
				Title:       "Output",
				Description: "Speaker and alert preferences.",
				Items: []settingItem{
					sliderSetting(gsettings.ScopeUser, "sound.output_volume", "Output volume", "Default speaker level for media and UI sounds.", float64(68), 0, 100, true),
					toggleSetting(gsettings.ScopeUser, "sound.notification_sounds", "Notification sounds", "Play an audible alert for incoming notifications.", true),
					toggleSetting(gsettings.ScopeUser, "sound.do_not_disturb", "Do not disturb", "Silence alerts while keeping notifications visible.", false),
				},
			},
			{
				Title:       "Input",
				Description: "Microphone tuning for calls and recordings.",
				Items: []settingItem{
					sliderSetting(gsettings.ScopeUser, "sound.microphone_boost", "Microphone boost", "Additional gain above the base input level.", float64(20), 0, 100, true),
				},
			},
		},
	},
	{
		Label:       "Privacy",
		Icon:        "changes-prevent",
		Title:       "Privacy & Security",
		Summary:     "Permissions, telemetry, and access controls.",
		SystemHeavy: true,
		Sections: []settingSection{
			{
				Title:       "Permissions",
				Description: "Controls for device access and sharing.",
				Items: []settingItem{
					toggleSetting(gsettings.ScopeUser, "privacy.location_services", "Location services", "Allow apps to use approximate location.", false),
					toggleSetting(gsettings.ScopeUser, "privacy.camera_access", "Camera access", "Allow trusted apps to open cameras.", true),
					toggleSetting(gsettings.ScopeUser, "privacy.microphone_access", "Microphone access", "Allow trusted apps to capture audio.", true),
				},
			},
			{
				Title:       "System reporting",
				Description: "Behavior that affects the whole device.",
				Items: []settingItem{
					toggleSetting(gsettings.ScopeSystem, "system.telemetry", "Anonymous telemetry", "Share lightweight health metrics to improve defaults.", false),
					toggleSetting(gsettings.ScopeSystem, "system.auto_lock", "Auto-lock on suspend", "Require unlock after the system wakes.", true),
				},
			},
		},
	},
	{
		Label:       "Apps",
		Icon:        "applications-system",
		Title:       "Apps",
		Summary:     "Default tools and app-session behavior.",
		SystemHeavy: true,
		Sections: []settingSection{
			{
				Title:       "Defaults",
				Description: "Choose the apps opened by the shell by default.",
				Items: []settingItem{
					textSetting(gsettings.ScopeUser, "apps.default_terminal", "Default terminal", "Launcher label for the preferred terminal app.", "Terminal", "Terminal"),
					textSetting(gsettings.ScopeUser, "apps.default_editor", "Default editor", "Preferred text editor label.", "Notepad", "Notepad"),
					toggleSetting(gsettings.ScopeUser, "apps.restore_sessions", "Restore sessions", "Reopen last app set on next sign-in.", true),
				},
			},
			{
				Title:       "System behavior",
				Description: "App distribution and maintenance defaults.",
				Items: []settingItem{
					toggleSetting(gsettings.ScopeSystem, "apps.automatic_updates", "Automatic updates", "Allow background app updates when available.", true),
				},
			},
		},
	},
	{
		Label:       "About",
		Icon:        "help-about",
		Title:       "About",
		Summary:     "Device identity, account context, and storage locations.",
		SystemHeavy: true,
		Sections: []settingSection{
			{
				Title:       "Identity",
				Description: "Labels used by the shell and device services.",
				Items: []settingItem{
					textSetting(gsettings.ScopeSystem, "system.device_name", "Device name", "Friendly machine name shown in system surfaces.", hostnameDefault(), hostnameDefault()),
					textSetting(gsettings.ScopeUser, "profile.display_name", "Display name", "Personal label shown by apps and the session shell.", displayNameDefault(), displayNameDefault()),
				},
			},
		},
	},
}

type SettingsApp struct {
	Store gsettings.Store
}

func (a SettingsApp) CreateState() widget.State {
	return &SettingsState{
		store:  a.Store,
		drafts: make(map[string]*string),
	}
}

type SettingsState struct {
	widget.StateBase

	appCtrl *collections.ApplicationController
	store   gsettings.Store

	page int

	search string
	status string
	errMsg string

	userValues   map[string]any
	systemValues map[string]any
	drafts       map[string]*string

	canWriteSystem bool
	hostname       string
	userName       string
	userPath       string
	systemPath     string
	watchStop      func()
}

func (s *SettingsState) InitState() {
	s.appCtrl = collections.NewApplicationController()
	s.canWriteSystem = os.Geteuid() == 0
	s.hostname = hostnameDefault()
	s.userName = displayNameDefault()
	s.userPath = s.store.UserPath
	s.systemPath = s.store.SystemPath
	s.reload()
	s.startWatcher()
}

func (s *SettingsState) reload() {
	userCfg, userErr := s.store.Load(gsettings.ScopeUser)
	systemCfg, systemErr := s.store.Load(gsettings.ScopeSystem)

	userValues := map[string]any{}
	if userErr == nil && userCfg != nil {
		userValues = userCfg.Data()
	}
	systemValues := map[string]any{}
	if systemErr == nil && systemCfg != nil {
		systemValues = systemCfg.Data()
	}

	s.SetState(func() {
		s.userValues = userValues
		s.systemValues = systemValues
		s.syncDrafts()
		switch {
		case userErr != nil && systemErr != nil:
			s.errMsg = "Unable to load settings files."
			s.status = userErr.Error() + " | " + systemErr.Error()
		case userErr != nil:
			s.errMsg = "Unable to load user settings."
			s.status = userErr.Error()
		case systemErr != nil:
			s.errMsg = "Unable to load system settings."
			s.status = systemErr.Error()
		default:
			s.errMsg = ""
			s.status = "Settings loaded"
		}
	})
}

func (s *SettingsState) syncDrafts() {
	for _, item := range allTextItems() {
		key := draftKey(item)
		value := s.stringValue(item)
		if s.drafts[key] == nil {
			draft := value
			s.drafts[key] = &draft
			continue
		}
		*s.drafts[key] = value
	}
}

func (s *SettingsState) selectPage(i int) {
	if i < 0 || i >= len(settingsPages) {
		return
	}
	s.SetState(func() {
		s.page = i
		s.search = ""
	})
}

func (s *SettingsState) saveValue(item settingItem, value any) {
	if item.Scope == gsettings.ScopeSystem && !s.canWriteSystem {
		s.fail("System settings are read-only in this session.")
		return
	}
	if err := s.store.Set(item.Scope, item.Path, value); err != nil {
		s.fail(err.Error())
		return
	}
	s.SetState(func() {
		target := s.valuesForScope(item.Scope)
		setValueMap(target, item.Path, value)
		s.status = fmt.Sprintf("Saved %s %s", item.Scope.String(), item.Path)
		s.errMsg = ""
		if item.Kind == kindText {
			if draft := s.drafts[draftKey(item)]; draft != nil {
				*draft = s.stringValue(item)
			}
		}
	})
	s.toast(fmt.Sprintf("Saved %s", item.Label), collections.ToastInfo)
}

func (s *SettingsState) fail(message string) {
	s.SetState(func() {
		s.errMsg = message
		s.status = message
	})
	s.toast(message, collections.ToastError)
}

func (s *SettingsState) toast(message string, variant collections.ToastVariant) {
	if s.appCtrl == nil {
		return
	}
	s.appCtrl.ShowToastFor(message, variant, 3*time.Second)
}

func (s *SettingsState) valuesForScope(scope gsettings.Scope) map[string]any {
	switch scope {
	case gsettings.ScopeSystem:
		if s.systemValues == nil {
			s.systemValues = map[string]any{}
		}
		return s.systemValues
	default:
		if s.userValues == nil {
			s.userValues = map[string]any{}
		}
		return s.userValues
	}
}

func (s *SettingsState) rawValue(item settingItem) (any, bool) {
	value, ok := lookupValueMap(s.valuesForScope(item.Scope), item.Path)
	if ok {
		return value, true
	}
	return item.Default, false
}

func (s *SettingsState) boolValue(item settingItem) bool {
	value, _ := s.rawValue(item)
	switch v := value.(type) {
	case bool:
		return v
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
	default:
		return false
	}
}

func (s *SettingsState) floatValue(item settingItem) float64 {
	value, _ := s.rawValue(item)
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case string:
		parsed, err := gsettings.ParseValue(v)
		if err == nil {
			if num, ok := parsed.(float64); ok {
				return num
			}
			if num, ok := parsed.(int64); ok {
				return float64(num)
			}
		}
	}
	if def, ok := item.Default.(float64); ok {
		return def
	}
	if def, ok := item.Default.(int64); ok {
		return float64(def)
	}
	return 0
}

func (s *SettingsState) stringValue(item settingItem) string {
	value, _ := s.rawValue(item)
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (s *SettingsState) searchable(item settingItem) bool {
	query := strings.TrimSpace(strings.ToLower(s.search))
	if query == "" {
		return true
	}
	text := strings.ToLower(strings.Join([]string{item.Label, item.Detail, item.Path, item.Scope.String()}, " "))
	return strings.Contains(text, query)
}

func (s *SettingsState) visibleItems(items []settingItem) []settingItem {
	if strings.TrimSpace(s.search) == "" {
		return items
	}
	out := make([]settingItem, 0, len(items))
	for _, item := range items {
		if s.searchable(item) {
			out = append(out, item)
		}
	}
	return out
}

func (s *SettingsState) currentPage() settingPage {
	if s.page < 0 || s.page >= len(settingsPages) {
		return settingsPages[0]
	}
	return settingsPages[s.page]
}

func allTextItems() []settingItem {
	out := make([]settingItem, 0, 8)
	for _, page := range settingsPages {
		for _, section := range page.Sections {
			for _, item := range section.Items {
				if item.Kind == kindText {
					out = append(out, item)
				}
			}
		}
	}
	return out
}

func choiceSetting(scope gsettings.Scope, path, label, detail, def string, options ...string) settingItem {
	return settingItem{
		Scope:   scope,
		Path:    path,
		Label:   label,
		Detail:  detail,
		Kind:    kindChoice,
		Default: def,
		Options: options,
	}
}

func sliderSetting(scope gsettings.Scope, path, label, detail string, def, min, max float64, whole bool) settingItem {
	return settingItem{
		Scope:   scope,
		Path:    path,
		Label:   label,
		Detail:  detail,
		Kind:    kindSlider,
		Default: def,
		Min:     min,
		Max:     max,
		Whole:   whole,
	}
}

func toggleSetting(scope gsettings.Scope, path, label, detail string, def bool) settingItem {
	return settingItem{
		Scope:   scope,
		Path:    path,
		Label:   label,
		Detail:  detail,
		Kind:    kindToggle,
		Default: def,
	}
}

func textSetting(scope gsettings.Scope, path, label, detail, def, placeholder string) settingItem {
	return settingItem{
		Scope:       scope,
		Path:        path,
		Label:       label,
		Detail:      detail,
		Kind:        kindText,
		Default:     def,
		Placeholder: placeholder,
	}
}

func draftKey(item settingItem) string {
	return item.Scope.String() + ":" + item.Path
}

func hostnameDefault() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return "avyos Device"
	}
	return name
}

func displayNameDefault() string {
	for _, value := range []string{os.Getenv("USER"), os.Getenv("LOGNAME")} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return runtime.GOOS
}

func (s *SettingsState) startWatcher() {
	paths := make([]string, 0, 2)
	for _, path := range []string{s.store.UserPath, s.store.SystemPath} {
		path = strings.TrimSpace(path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return
	}

	stop, err := gsettings.OnChange(func(_ ...gsettings.Change) {
		s.reload()
	}, paths...)
	if err != nil {
		s.fail(err.Error())
		return
	}
	s.watchStop = stop
}

func lookupValueMap(root map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = root
	for _, part := range parts {
		next, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = next[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setValueMap(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func pageDestinations() []collections.NavDestination {
	out := make([]collections.NavDestination, 0, len(settingsPages))
	for _, page := range settingsPages {
		out = append(out, collections.NavDestination{
			Label: page.Label,
			Icon:  page.Icon,
		})
	}
	return out
}

func pageHasSystemItems(page settingPage) bool {
	return page.SystemHeavy
}

func orderedChoices(item settingItem, selected string) []string {
	if selected == "" {
		return item.Options
	}
	options := append([]string(nil), item.Options...)
	if idx := slices.Index(options, selected); idx > 0 {
		options[0], options[idx] = options[idx], options[0]
	}
	return options
}
