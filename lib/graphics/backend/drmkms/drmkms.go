//go:build linux

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

// Package drmkms implements a DRM/KMS backend for direct display access on Linux.
//
// It uses the Linux DRM subsystem via ioctl syscalls — no CGO required.
//
// Architecture:
//
//	Backend  → opens /dev/dri/card0 (configurable)
//	Window   → one CRTC + connector combination
//	Surface  → pair of dumb framebuffers for double-buffering
//
// Frame lifecycle:
//
//	Surface.Begin()   → returns canvas backed by the "back" buffer
//	Surface.Present() → blits RGBA→XRGB8888, issues PAGE_FLIP (or SETCRTC for first
//	                     frame), then waits for the vblank event

package drmkms

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"avyos.dev/lib/graphics/backend"
	"avyos.dev/lib/graphics/canvas"
	"avyos.dev/lib/graphics/canvas/pixbuf"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/graphics/geom"
	"avyos.dev/lib/logger"
)

var log = logger.New("drmkms")

//  ioctl helpers

func ioc(dir, typ, nr, size uintptr) uintptr {
	return dir<<30 | typ<<8 | nr | size<<16
}

func iow(typ, nr, size uintptr) uintptr  { return ioc(1, typ, nr, size) }
func iowr(typ, nr, size uintptr) uintptr { return ioc(3, typ, nr, size) }
func iocNone(typ, nr uintptr) uintptr    { return ioc(0, typ, nr, 0) }

func ioctl(fd, req uintptr, arg unsafe.Pointer) error {
	for {
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
		if errno == 0 {
			return nil
		}
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		}
		return errno
	}
}

func setClientCap(fd uintptr, capability, value uint64) error {
	cap := drmSetClientCap{Capability: capability, Value: value}
	return ioctl(fd, drmIoctlSetClientCap, unsafe.Pointer(&cap))
}

//  DRM ioctl request codes

var (
	drmIoctlGetResources = iowr('d', 0xA0, unsafe.Sizeof(drmModeCardRes{}))
	drmIoctlGetConnector = iowr('d', 0xA7, unsafe.Sizeof(drmModeGetConnector{}))
	drmIoctlGetEncoder   = iowr('d', 0xA6, unsafe.Sizeof(drmModeGetEncoder{}))
	drmIoctlSetCRTC      = iowr('d', 0xA2, unsafe.Sizeof(drmModeCRTC{}))
	drmIoctlCreateDumb   = iowr('d', 0xB2, unsafe.Sizeof(drmModeCreateDumb{}))
	drmIoctlMapDumb      = iowr('d', 0xB3, unsafe.Sizeof(drmModeMapDumb{}))
	drmIoctlDestroyDumb  = iowr('d', 0xB4, unsafe.Sizeof(drmModeDestroyDumb{}))
	drmIoctlAddFB        = iowr('d', 0xAE, unsafe.Sizeof(drmModeFBCmd{}))
	drmIoctlRmFB         = iowr('d', 0xAF, unsafe.Sizeof(uint32(0)))
	drmIoctlPageFlip     = iowr('d', 0xB0, unsafe.Sizeof(drmModePageFlip{}))
	drmIoctlSetClientCap = iow('d', 0x0D, unsafe.Sizeof(drmSetClientCap{}))
	drmIoctlSetMaster    = iocNone('d', 0x1E)
	drmIoctlDropMaster   = iocNone('d', 0x1F)
)

const (
	drmModeConnected     = 1
	drmModePageFlipEvent = 0x01
	drmModeEventPageFlip = 0x02
	drmModeTypePref      = 1 << 3
	defaultScale         = 1.0
	drmClientCapAtomic   = 3
	drmClientCapCursorHS = 6
)

//  DRM structures (64-bit Linux ABI)

type drmModeCardRes struct {
	FbIDPtr              uint64
	CrtcIDPtr            uint64
	ConnectorIDPtr       uint64
	EncoderIDPtr         uint64
	CountFbs             uint32
	CountCrtcs           uint32
	CountConnectors      uint32
	CountEncoders        uint32
	MinWidth, MaxWidth   uint32
	MinHeight, MaxHeight uint32
}

type drmModeModeInfo struct {
	Clock                                         uint32
	Hdisplay, HsyncStart, HsyncEnd, Htotal, Hskew uint16
	Vdisplay, VsyncStart, VsyncEnd, Vtotal, Vscan uint16
	Vrefresh                                      uint32
	Flags, Type                                   uint32
	Name                                          [32]byte
}

type drmModeGetConnector struct {
	EncodersPtr, ModesPtr, PropsPtr, PropValuesPtr uint64
	CountModes, CountProps, CountEncoders          uint32
	EncoderID, ConnectorID                         uint32
	ConnectorType, ConnectorTypeID                 uint32
	Connection                                     uint32
	MmWidth, MmHeight                              uint32
	Subpixel, Pad                                  uint32
}

type drmModeGetEncoder struct {
	EncoderID, EncoderType, CrtcID uint32
	PossibleCrtcs, PossibleClones  uint32
}

type drmModeCRTC struct {
	SetConnectorsPtr uint64
	CountConnectors  uint32
	CrtcID           uint32
	FbID             uint32
	X, Y             uint32
	GammaSize        uint32
	ModeValid        uint32
	Mode             drmModeModeInfo
}

type drmModeCreateDumb struct {
	Height, Width, Bpp, Flags uint32
	Handle, Pitch             uint32
	Size                      uint64
}

type drmModeMapDumb struct {
	Handle uint32
	Pad    uint32
	Offset uint64
}

type drmModeDestroyDumb struct{ Handle uint32 }

type drmModeFBCmd struct {
	FbID, Width, Height, Pitch, Bpp, Depth, Handle uint32
}

type drmModePageFlip struct {
	CrtcID, FbID, Flags, Reserved uint32
	UserData                      uint64
}

type drmSetClientCap struct {
	Capability uint64
	Value      uint64
}

type drmEventHeader struct{ Type, Length uint32 }

//  Backend

// Backend is the DRM/KMS platform backend.
type Backend struct {
	cardPath     string
	selectedMode string
	fd           *os.File
	events       chan event.Event
	done         chan struct{}
	evdev        *evdevManager
	cursor       *hwCursor
	cursorHS     bool
}

// Mode describes a DRM mode exposed by the selected connector.
type Mode struct {
	ConnectorID uint32
	Name        string
	Width       int
	Height      int
	Refresh     uint32
	Preferred   bool
}

// Spec returns a stable mode token accepted by Backend.SetMode.
func (m Mode) Spec() string {
	spec := fmt.Sprintf("%dx%d", m.Width, m.Height)
	if m.Refresh > 0 {
		spec = fmt.Sprintf("%s@%d", spec, m.Refresh)
	}
	return spec
}

const (
	modeConfigPath         = "/etc/desktop/resolution"
	modeConfigFallbackPath = "/usr/etc/desktop/resolution"
)

func (m Mode) String() string {
	spec := m.Spec()
	if m.Name != "" && m.Name != spec {
		spec = fmt.Sprintf("%s (%s)", m.Name, spec)
	}
	if m.Preferred {
		spec += " preferred"
	}
	return spec
}

// New creates a DRM/KMS backend. cardPath defaults to probing /dev/dri/card0–7.
func New(cardPath string) *Backend {
	return &Backend{cardPath: cardPath}
}

func (b *Backend) Name() string { return "drmkms" }

func (b *Backend) Scale() float64 { return defaultScale }

// LoadConfiguredMode returns the persisted system display mode, if any.
func LoadConfiguredMode() (string, error) {
	for _, path := range []string{modeConfigPath, modeConfigFallbackPath} {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return "", err
	}
	return "", nil
}

// SaveConfiguredMode persists the system display mode in WxH or WxH@Hz form.
func SaveConfiguredMode(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return errors.New("resolution is required")
	}
	if err := os.MkdirAll(filepath.Dir(modeConfigPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(modeConfigPath, []byte(spec+"\n"), 0o644)
}

// ClearConfiguredMode removes any persisted system display mode override.
func ClearConfiguredMode() error {
	if err := os.Remove(modeConfigPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SetMode requests a specific connector mode in WxH or WxH@Hz form.
func (b *Backend) SetMode(spec string) {
	b.selectedMode = strings.TrimSpace(spec)
}

func (b *Backend) Init() error {
	b.events = make(chan event.Event, 256)
	b.done = make(chan struct{})

	if b.selectedMode == "" {
		configured, err := LoadConfiguredMode()
		if err != nil {
			return fmt.Errorf("drmkms: load configured mode: %w", err)
		}
		if configured != "" {
			b.selectedMode = configured
			log.Info("using configured DRM mode %s", configured)
		}
	}

	// If an explicit path was given, try only that.
	if b.cardPath != "" {
		log.Info("initializing DRM/KMS backend on %s", b.cardPath)
		if err := b.openCard(b.cardPath); err != nil {
			log.Error("failed to open DRM device %s: %v", b.cardPath, err)
			return err
		}
		return nil
	}
	log.Info("initializing DRM/KMS backend by probing card nodes")
	// Probe card0–7, pick the first that has KMS resources.
	var lastErr error
	for i := range 8 {
		path := filepath.Join("/dev/dri", fmt.Sprintf("card%d", i))
		if path == "" {
			continue
		}
		if err := b.openCard(path); err != nil {
			log.Error("skipping DRM device %s: %v", path, err)
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("no card nodes found")
	}
	log.Error("failed to find usable DRM device: %v", lastErr)
	return fmt.Errorf("drmkms: no usable device: %w", lastErr)
}

func (b *Backend) openCard(path string) error {
	cardLog := log.With("card", path)
	cursorHS := false
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := ioctl(uintptr(f.Fd()), drmIoctlSetMaster, nil); err != nil {
		cardLog.Warn("failed to become DRM master: %v", err)
	}
	if err := setClientCap(uintptr(f.Fd()), drmClientCapAtomic, 1); err != nil {
		if !errors.Is(err, syscall.EOPNOTSUPP) && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTTY) {
			cardLog.Debug("failed to enable atomic client capability: %v", err)
		}
	} else if err := setClientCap(uintptr(f.Fd()), drmClientCapCursorHS, 1); err != nil {
		if !errors.Is(err, syscall.EOPNOTSUPP) && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTTY) {
			cardLog.Debug("failed to enable cursor hotspot capability: %v", err)
		}
	} else {
		cursorHS = true
		cardLog.Debug("enabled DRM cursor hotspot capability")
	}
	// Verify the node has connectors and CRTCs.
	var probe drmModeCardRes
	if err := ioctl(uintptr(f.Fd()), drmIoctlGetResources, unsafe.Pointer(&probe)); err != nil || probe.CountConnectors == 0 || probe.CountCrtcs == 0 {
		f.Close()
		if err != nil {
			return err
		}
		return errors.New("no resources")
	}
	b.fd = f
	b.cardPath = path
	b.cursorHS = cursorHS
	cardLog.Info("opened DRM device with %d connectors and %d CRTCs", probe.CountConnectors, probe.CountCrtcs)
	return nil
}

func (b *Backend) Shutdown() {
	if b.fd != nil {
		log.Info("shutting down DRM/KMS backend on %s", b.cardPath)
	}
	if b.evdev != nil {
		b.evdev.stop()
		b.evdev = nil
	}
	if b.cursor != nil {
		b.cursor.destroy()
		b.cursor = nil
	}
	select {
	case <-b.done:
	default:
		close(b.done)
	}
	if b.fd != nil {
		_ = ioctl(uintptr(b.fd.Fd()), drmIoctlDropMaster, nil)
		b.fd.Close()
		b.fd = nil
	}
}

func (b *Backend) NewWindow(opts backend.WindowOptions) (backend.Window, error) {
	if b.fd == nil {
		return nil, errors.New("drmkms: not initialised")
	}
	fd := uintptr(b.fd.Fd())

	res, err := b.getResources(fd)
	if err != nil {
		return nil, fmt.Errorf("drmkms: getResources: %w", err)
	}
	conn, mode, err := b.findConnector(fd, res)
	if err != nil {
		return nil, fmt.Errorf("drmkms: findConnector: %w", err)
	}
	crtcID, err := b.findCRTC(fd, conn, res.crtcIDs)
	if err != nil {
		return nil, fmt.Errorf("drmkms: findCRTC: %w", err)
	}

	width, height := int(mode.Hdisplay), int(mode.Vdisplay)
	log.Info(
		"creating scanout surface on %s connector=%d crtc=%d mode=%s size=%dx%d",
		b.cardPath,
		conn.ConnectorID,
		crtcID,
		formatMode(mode),
		width,
		height,
	)

	fb0, err := b.createFramebuffer(fd, width, height)
	if err != nil {
		return nil, fmt.Errorf("drmkms: fb0: %w", err)
	}
	fb1, err := b.createFramebuffer(fd, width, height)
	if err != nil {
		fb0.destroy(fd)
		return nil, fmt.Errorf("drmkms: fb1: %w", err)
	}

	surf := &surface{
		fd:          fd,
		crtcID:      crtcID,
		connectorID: conn.ConnectorID,
		mode:        mode,
		fbs:         [2]*framebuffer{fb0, fb1},
		width:       width,
		height:      height,
	}

	// Start evdev input and hardware cursor on first window.
	if b.evdev == nil {
		b.cursor = newHWCursor(fd, crtcID, b.cursorHS) // nil if hw cursor unsupported
		if b.cursor == nil {
			log.Debug("hardware cursor is unavailable on connector=%d crtc=%d", conn.ConnectorID, crtcID)
		} else {
			b.cursor.move(int32(width/2), int32(height/2))
			log.Debug("hardware cursor is enabled on connector=%d crtc=%d", conn.ConnectorID, crtcID)
		}
		surf.cursor = b.cursor
		b.evdev = newEvdevManager(b, width, height)
		b.evdev.start()
		log.Info("started evdev input manager for %dx%d output", width, height)
	}
	if surf.cursor == nil {
		surf.cursor = b.cursor
	}

	return &window{fd: fd, backend: b, surf: surf, width: width, height: height}, nil
}

// Modes returns the modes advertised by the first connected connector.
func (b *Backend) Modes() ([]Mode, error) {
	if b.fd == nil {
		return nil, errors.New("drmkms: not initialised")
	}
	fd := uintptr(b.fd.Fd())
	res, err := b.getResources(fd)
	if err != nil {
		return nil, fmt.Errorf("drmkms: getResources: %w", err)
	}
	conn, err := b.firstConnectedConnector(fd, res)
	if err != nil {
		return nil, fmt.Errorf("drmkms: findConnector: %w", err)
	}
	out := make([]Mode, 0, len(conn.modes))
	for _, mode := range conn.modes {
		out = append(out, exportMode(conn.ConnectorID, mode))
	}
	return out, nil
}

func (b *Backend) pushEvent(e event.Event) {
	select {
	case b.events <- e:
	default:
	}
}

func (b *Backend) PollEvents() ([]event.Event, error) {
	var evs []event.Event
	for {
		select {
		case e := <-b.events:
			evs = append(evs, e)
		default:
			return evs, nil
		}
	}
}

func (b *Backend) WaitEvent() ([]event.Event, error) {
	e := <-b.events
	more, _ := b.PollEvents()
	return append([]event.Event{e}, more...), nil
}

type resourcesResult struct {
	drmModeCardRes
	crtcIDs []uint32
	connIDs []uint32
}

func (b *Backend) getResources(fd uintptr) (*resourcesResult, error) {
	res := &drmModeCardRes{}
	if err := ioctl(fd, drmIoctlGetResources, unsafe.Pointer(res)); err != nil {
		return nil, err
	}
	if res.CountConnectors == 0 || res.CountCrtcs == 0 {
		return nil, errors.New("no connectors or CRTCs")
	}
	crtcIDs := make([]uint32, res.CountCrtcs)
	connIDs := make([]uint32, res.CountConnectors)
	res.CrtcIDPtr = uint64(uintptr(unsafe.Pointer(&crtcIDs[0])))
	res.ConnectorIDPtr = uint64(uintptr(unsafe.Pointer(&connIDs[0])))
	if res.CountEncoders > 0 {
		encIDs := make([]uint32, res.CountEncoders)
		res.EncoderIDPtr = uint64(uintptr(unsafe.Pointer(&encIDs[0])))
	}
	if res.CountFbs > 0 {
		fbIDs := make([]uint32, res.CountFbs)
		res.FbIDPtr = uint64(uintptr(unsafe.Pointer(&fbIDs[0])))
	}
	if err := ioctl(fd, drmIoctlGetResources, unsafe.Pointer(res)); err != nil {
		return nil, err
	}
	return &resourcesResult{*res, crtcIDs, connIDs}, nil
}

type connectorResult struct {
	drmModeGetConnector
	modes    []drmModeModeInfo
	encoders []uint32
}

func (b *Backend) findConnector(fd uintptr, res *resourcesResult) (*connectorResult, drmModeModeInfo, error) {
	conn, err := b.firstConnectedConnector(fd, res)
	if err != nil {
		return nil, drmModeModeInfo{}, err
	}
	mode, err := selectMode(conn.modes, b.selectedMode)
	if err != nil {
		return nil, drmModeModeInfo{}, err
	}
	if b.selectedMode != "" {
		log.Info("selected explicit connector mode %s", formatMode(mode))
	}
	log.Info("selected connector=%d mode=%s from %d modes", conn.ConnectorID, formatMode(mode), len(conn.modes))
	return conn, mode, nil
}

func (b *Backend) firstConnectedConnector(fd uintptr, res *resourcesResult) (*connectorResult, error) {
	if len(res.connIDs) == 0 {
		return nil, errors.New("no connectors")
	}
	for _, id := range res.connIDs {
		conn := &drmModeGetConnector{ConnectorID: id}
		// First pass: get counts.
		if err := ioctl(fd, drmIoctlGetConnector, unsafe.Pointer(conn)); err != nil {
			log.Debug("failed to inspect connector %d: %v", id, err)
			continue
		}
		if conn.Connection != drmModeConnected || conn.CountModes == 0 {
			log.Debug("skipping connector %d connection=%d modes=%d", id, conn.Connection, conn.CountModes)
			continue
		}
		// Second pass: fill all arrays.
		modes := make([]drmModeModeInfo, conn.CountModes)
		conn.ModesPtr = uint64(uintptr(unsafe.Pointer(&modes[0])))
		var encs []uint32
		if conn.CountEncoders > 0 {
			encs = make([]uint32, conn.CountEncoders)
			conn.EncodersPtr = uint64(uintptr(unsafe.Pointer(&encs[0])))
		}
		if conn.CountProps > 0 {
			props := make([]uint32, conn.CountProps)
			propVals := make([]uint64, conn.CountProps)
			conn.PropsPtr = uint64(uintptr(unsafe.Pointer(&props[0])))
			conn.PropValuesPtr = uint64(uintptr(unsafe.Pointer(&propVals[0])))
		}
		if err := ioctl(fd, drmIoctlGetConnector, unsafe.Pointer(conn)); err != nil {
			log.Debug("failed to load connector %d details: %v", id, err)
			continue
		}
		return &connectorResult{*conn, modes, encs}, nil
	}
	return nil, errors.New("no connected connector with modes")
}

func bestMode(modes []drmModeModeInfo) (drmModeModeInfo, bool) {
	if len(modes) == 0 {
		return drmModeModeInfo{}, false
	}
	best := modes[0]
	for _, mode := range modes[1:] {
		log.Info("checking connector mode %s vs best %s", formatMode(mode), formatMode(best))
		if modeBetterThan(mode, best) {
			best = mode
		}
	}
	return best, true
}

type modeRequest struct {
	Width      int
	Height     int
	Refresh    uint32
	HasRefresh bool
}

func selectMode(modes []drmModeModeInfo, requested string) (drmModeModeInfo, error) {
	if requested == "" {
		mode, ok := bestMode(modes)
		if !ok {
			return drmModeModeInfo{}, errors.New("no modes available")
		}
		return mode, nil
	}
	req, err := parseModeRequest(requested)
	if err != nil {
		return drmModeModeInfo{}, fmt.Errorf("invalid resolution %q: %w", requested, err)
	}
	var matches []drmModeModeInfo
	for _, mode := range modes {
		if modeMatchesRequest(mode, req) {
			matches = append(matches, mode)
		}
	}
	if len(matches) == 0 {
		return drmModeModeInfo{}, fmt.Errorf("resolution %q is not available; available modes: %s", requested, summarizeModes(modes))
	}
	best := matches[0]
	if !req.HasRefresh {
		for _, mode := range matches[1:] {
			if mode.Vrefresh > best.Vrefresh {
				best = mode
			}
		}
	}
	return best, nil
}

func parseModeRequest(spec string) (modeRequest, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return modeRequest{}, errors.New("empty resolution")
	}
	sizePart, refreshPart, hasRefresh := strings.Cut(spec, "@")
	sep := strings.IndexAny(sizePart, "xX")
	if sep <= 0 || sep >= len(sizePart)-1 {
		return modeRequest{}, errors.New("expected WxH or WxH@Hz")
	}
	var req modeRequest
	if _, err := fmt.Sscanf(sizePart, "%dx%d", &req.Width, &req.Height); err != nil {
		if _, errAlt := fmt.Sscanf(sizePart, "%dX%d", &req.Width, &req.Height); errAlt != nil {
			return modeRequest{}, errors.New("expected numeric width and height")
		}
	}
	if req.Width <= 0 || req.Height <= 0 {
		return modeRequest{}, errors.New("width and height must be positive")
	}
	if hasRefresh {
		var refresh uint32
		if _, err := fmt.Sscanf(refreshPart, "%d", &refresh); err != nil || refresh == 0 {
			return modeRequest{}, errors.New("refresh must be a positive integer")
		}
		req.Refresh = refresh
		req.HasRefresh = true
	}
	return req, nil
}

func modeMatchesRequest(mode drmModeModeInfo, req modeRequest) bool {
	if int(mode.Hdisplay) != req.Width || int(mode.Vdisplay) != req.Height {
		return false
	}
	if req.HasRefresh && mode.Vrefresh != req.Refresh {
		return false
	}
	return true
}

func summarizeModes(modes []drmModeModeInfo) string {
	parts := make([]string, 0, len(modes))
	for _, mode := range modes {
		parts = append(parts, modeSpec(mode))
	}
	return strings.Join(parts, ", ")
}

func modeBetterThan(a, b drmModeModeInfo) bool {
	aPreferred := a.Type&drmModeTypePref != 0
	bPreferred := b.Type&drmModeTypePref != 0
	if aPreferred != bPreferred {
		return aPreferred
	}
	aArea := uint32(a.Hdisplay) * uint32(a.Vdisplay)
	bArea := uint32(b.Hdisplay) * uint32(b.Vdisplay)
	if aArea != bArea {
		return aArea > bArea
	}
	if a.Hdisplay != b.Hdisplay {
		return a.Hdisplay > b.Hdisplay
	}
	if a.Vdisplay != b.Vdisplay {
		return a.Vdisplay > b.Vdisplay
	}
	return false
}

func (b *Backend) findCRTC(fd uintptr, conn *connectorResult, crtcIDs []uint32) (uint32, error) {
	// Build candidate encoder list: current encoder first, then others.
	candidates := make([]uint32, 0, 1+len(conn.encoders))
	if conn.EncoderID != 0 {
		candidates = append(candidates, conn.EncoderID)
	}
	for _, eid := range conn.encoders {
		if eid == 0 || eid == conn.EncoderID {
			continue
		}
		candidates = append(candidates, eid)
	}
	for _, eid := range candidates {
		enc := &drmModeGetEncoder{EncoderID: eid}
		if err := ioctl(fd, drmIoctlGetEncoder, unsafe.Pointer(enc)); err != nil {
			continue
		}
		// Prefer the encoder's currently attached CRTC.
		if enc.CrtcID != 0 {
			if slices.Contains(crtcIDs, enc.CrtcID) {
				log.Info("selected CRTC %d for connector=%d via encoder=%d", enc.CrtcID, conn.ConnectorID, eid)
				return enc.CrtcID, nil
			}
		}
		// Fall back to any possible CRTC.
		for i, id := range crtcIDs {
			if enc.PossibleCrtcs&(1<<uint(i)) != 0 {
				log.Info("selected fallback CRTC %d for connector=%d via encoder=%d", id, conn.ConnectorID, eid)
				return id, nil
			}
		}
	}
	return 0, errors.New("no usable encoder/CRTC for connector")
}

type framebuffer struct {
	handle uint32
	fbID   uint32
	pitch  uint32
	size   uint64
	mmap   []byte
	canvas canvas.Canvas
}

func (b *Backend) createFramebuffer(fd uintptr, width, height int) (*framebuffer, error) {
	create := drmModeCreateDumb{Width: uint32(width), Height: uint32(height), Bpp: 32}
	if err := ioctl(fd, drmIoctlCreateDumb, unsafe.Pointer(&create)); err != nil {
		return nil, fmt.Errorf("CREATE_DUMB: %w", err)
	}
	addFB := drmModeFBCmd{
		Width:  uint32(width),
		Height: uint32(height),
		Bpp:    32,
		Depth:  24,
		Pitch:  create.Pitch,
		Handle: create.Handle,
	}
	if err := ioctl(fd, drmIoctlAddFB, unsafe.Pointer(&addFB)); err != nil {
		return nil, fmt.Errorf("ADD_FB: %w", err)
	}
	mapDumb := drmModeMapDumb{Handle: create.Handle}
	if err := ioctl(fd, drmIoctlMapDumb, unsafe.Pointer(&mapDumb)); err != nil {
		return nil, fmt.Errorf("MAP_DUMB: %w", err)
	}
	pix, err := syscall.Mmap(int(fd), int64(mapDumb.Offset), int(create.Size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	return &framebuffer{
		handle: create.Handle,
		fbID:   addFB.FbID,
		pitch:  create.Pitch,
		size:   create.Size,
		mmap:   pix,
		canvas: pixbuf.NewCanvas(width, height),
	}, nil
}

// blitRegionToMmap copies the specified rect of the canvas RGBA pixels into
// the mmap buffer as XRGB8888. Pre-slicing rows eliminates per-pixel bounds
// checks and allows the compiler to auto-vectorize the inner loop.
func (fb *framebuffer) blitRegionToMmap(width, height int, r image.Rectangle) {
	src := fb.canvas.Pixels()
	pitch := int(fb.pitch)
	srcStride := width * 4
	for y := r.Min.Y; y < r.Max.Y; y++ {
		srcRow := src[y*srcStride+r.Min.X*4 : y*srcStride+r.Max.X*4]
		dstRow := fb.mmap[y*pitch+r.Min.X*4 : y*pitch+r.Max.X*4]
		blitRGBAToXRGBRow(dstRow, srcRow)
	}
}

func blitRGBAToXRGBRow(dst, src []byte) {
	if len(src) == 0 {
		return
	}
	pixels := len(src) / 4
	src32 := unsafe.Slice((*uint32)(unsafe.Pointer(&src[0])), pixels)
	dst32 := unsafe.Slice((*uint32)(unsafe.Pointer(&dst[0])), pixels)
	for i := range pixels {
		p := src32[i]
		// src RGBA bytes as LE uint32: [R G B A]
		// dst XRGB8888 LE bytes:       [B G R FF]
		dst32[i] = 0xFF000000 | (p & 0x0000FF00) | ((p & 0x00FF0000) >> 16) | ((p & 0x000000FF) << 16)
	}
}

func (fb *framebuffer) destroy(fd uintptr) {
	if fb.mmap != nil {
		_ = syscall.Munmap(fb.mmap)
	}
	if fb.fbID != 0 {
		id := fb.fbID
		_ = ioctl(fd, drmIoctlRmFB, unsafe.Pointer(&id))
	}
	if fb.handle != 0 {
		d := drmModeDestroyDumb{Handle: fb.handle}
		_ = ioctl(fd, drmIoctlDestroyDumb, unsafe.Pointer(&d))
	}
}

type window struct {
	fd      uintptr
	backend *Backend
	surf    *surface
	width   int
	height  int
}

func (w *window) Surface() backend.Surface { return w.surf }
func (w *window) SetTitle(_ string)        {}
func (w *window) Size() (int, int)         { return w.width, w.height }
func (w *window) Close() {
	w.surf.fbs[0].destroy(w.fd)
	w.surf.fbs[1].destroy(w.fd)
}

var _ backend.Window = (*window)(nil)

type surface struct {
	fd          uintptr
	crtcID      uint32
	connectorID uint32
	mode        drmModeModeInfo
	fbs         [2]*framebuffer
	cursor      *hwCursor
	front       int
	firstFrame  bool
	width       int
	height      int
	mu          sync.Mutex

	// accumDirty[i] accumulates the union of canvas dirty rects for all frames
	// rendered to framebuffer i since it was last presented to the display.
	// When we blit fb[i], we must blit at least accumDirty[i] to make it
	// current (correct double-buffer damage tracking).
	accumDirty [2]image.Rectangle

	// flipBuf is reused across waitFlip calls to avoid per-frame heap allocs.
	flipBuf [64]byte

	lastPresent backend.PresentStats
}

func (s *surface) Size() geom.Size { return geom.Sz(float64(s.width), float64(s.height)) }

func (s *surface) Begin() (canvas.Canvas, error) {
	return s.fbs[1-s.front].canvas, nil
}

func (s *surface) PresentStats() backend.PresentStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPresent
}

func (s *surface) Present(c canvas.Canvas) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	backIdx := 1 - s.front
	back := s.fbs[backIdx]
	fullBounds := image.Rect(0, 0, s.width, s.height)
	stats := backend.PresentStats{}

	// Determine which pixels the canvas actually wrote this frame.
	canvasDirty := fullBounds
	if dt, ok := c.(canvas.DirtyTracker); ok {
		if d := dt.Dirty(); !d.Empty() {
			canvasDirty = d.Intersect(fullBounds)
		}
	}

	// blitRegion = canvas dirty ∪ accumulated dirty for this buffer.
	// The accumulated dirty covers pixels changed in earlier frames while
	// this buffer was showing (i.e., what it missed during the previous flip).
	blitRegion := canvasDirty.Union(s.accumDirty[backIdx]).Intersect(fullBounds)
	if blitRegion.Empty() {
		blitRegion = fullBounds
	}

	// This buffer will be current after the flip; the other buffer now needs
	// to catch up to what we changed this frame.
	s.accumDirty[s.front] = s.accumDirty[s.front].Union(canvasDirty)
	s.accumDirty[backIdx] = image.Rectangle{}

	blitStart := time.Now()
	back.blitRegionToMmap(s.width, s.height, blitRegion)
	stats.Blit = time.Since(blitStart)

	if !s.firstFrame {
		connID := s.connectorID
		crtc := drmModeCRTC{
			CrtcID:           s.crtcID,
			FbID:             back.fbID,
			SetConnectorsPtr: uint64(uintptr(unsafe.Pointer(&connID))),
			CountConnectors:  1,
			ModeValid:        1,
			Mode:             s.mode,
		}
		submitStart := time.Now()
		if err := ioctl(s.fd, drmIoctlSetCRTC, unsafe.Pointer(&crtc)); err != nil {
			log.Error("failed to program CRTC %d on connector=%d mode=%s: %v", s.crtcID, s.connectorID, formatMode(s.mode), err)
			return fmt.Errorf("drmkms: SETCRTC: %w", err)
		}
		stats.Submit = time.Since(submitStart)
		log.Info("programmed CRTC %d on connector=%d mode=%s", s.crtcID, s.connectorID, formatMode(s.mode))
		if s.cursor != nil {
			if err := s.cursor.reapply(); err != nil {
				log.Debug("failed to reapply hardware cursor on CRTC %d: %v", s.crtcID, err)
			}
		}
		s.lastPresent = stats
		s.firstFrame = true
		s.front = 1 - s.front
		return nil
	}

	flip := drmModePageFlip{
		CrtcID: s.crtcID,
		FbID:   back.fbID,
		Flags:  drmModePageFlipEvent,
	}
	submitStart := time.Now()
	if err := ioctl(s.fd, drmIoctlPageFlip, unsafe.Pointer(&flip)); err != nil {
		log.Error("page flip failed on CRTC %d connector=%d: %v", s.crtcID, s.connectorID, err)
		return fmt.Errorf("drmkms: PAGE_FLIP: %w", err)
	}
	stats.Submit = time.Since(submitStart)
	waitStart := time.Now()
	if err := s.waitFlip(); err != nil {
		return err
	}
	stats.Wait = time.Since(waitStart)
	s.lastPresent = stats
	s.front = 1 - s.front
	return nil
}

func (s *surface) waitFlip() error {
	for {
		n, err := syscall.Read(int(s.fd), s.flipBuf[:])
		if err != nil {
			return err
		}
		if n < 8 {
			continue
		}
		hdr := (*drmEventHeader)(unsafe.Pointer(&s.flipBuf[0]))
		if hdr.Type == drmModeEventPageFlip {
			return nil
		}
	}
}

func (s *surface) Resize(_, _ int) error {
	return errors.New("drmkms: runtime resize not supported; restart the application")
}

func (s *surface) SetCursorShape(shape event.CursorShape) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cursor == nil {
		return nil
	}
	return s.cursor.setShape(shape)
}

func formatMode(mode drmModeModeInfo) string {
	name := strings.TrimRight(string(mode.Name[:]), "\x00")
	size := modeSpec(mode)
	if name == "" || name == size {
		return size
	}
	return fmt.Sprintf("%s (%s)", name, size)
}

func modeSpec(mode drmModeModeInfo) string {
	spec := fmt.Sprintf("%dx%d", mode.Hdisplay, mode.Vdisplay)
	if mode.Vrefresh > 0 {
		spec = fmt.Sprintf("%s@%d", spec, mode.Vrefresh)
	}
	return spec
}

func exportMode(connectorID uint32, mode drmModeModeInfo) Mode {
	return Mode{
		ConnectorID: connectorID,
		Name:        strings.TrimRight(string(mode.Name[:]), "\x00"),
		Width:       int(mode.Hdisplay),
		Height:      int(mode.Vdisplay),
		Refresh:     mode.Vrefresh,
		Preferred:   mode.Type&drmModeTypePref != 0,
	}
}

var _ backend.Surface = (*surface)(nil)
var _ backend.CursorSetter = (*surface)(nil)
var _ backend.PresentStatsProvider = (*surface)(nil)
var _ backend.Backend = (*Backend)(nil)
