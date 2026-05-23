//go:build linux

package desktop

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	desktopapi "avyos.dev/api/desktop"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/graphics/backend"
	"avyos.dev/pkg/graphics/canvas"
	"avyos.dev/pkg/graphics/canvas/pixbuf"
	"avyos.dev/pkg/graphics/event"
	"avyos.dev/pkg/graphics/geom"
)

const (
	defaultWidth     = 800
	defaultHeight    = 600
	defaultMinWidth  = 160
	defaultMinHeight = 120
	defaultScale     = 1.0
)

// Backend renders app windows into the desktop shell service over shared memory.
type Backend struct {
	socketPath string
	scale      float64

	mu          sync.RWMutex
	client      *desktopapi.Client
	initialized bool
	windows     map[uint32]*window
}

func New(socketPath string) *Backend {
	return &Backend{
		socketPath: resolveSocketPath(socketPath),
		scale:      normalizeScale(0),
		windows:    make(map[uint32]*window),
	}
}

func Available(socketPath string) bool {
	client, err := desktopapi.NewClient(resolveSocketPath(socketPath))
	if err != nil {
		return false
	}
	_ = client.Close()
	return true
}

func (b *Backend) Name() string { return "desktop" }

func (b *Backend) Scale() float64 { return normalizeScale(b.scale) }

func (b *Backend) Init() error {
	b.mu.Lock()
	if b.initialized {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	client, err := desktopapi.NewClient(b.socketPath)
	if err != nil {
		return err
	}

	client.Desktop.OnInput(func(ev desktopapi.WindowInputEvent) {
		appEvent, err := event.Decode(ev.Payload)
		if err != nil {
			return
		}
		if win := b.windowFor(ev.WindowId); win != nil {
			win.surface.pushEvent(scaleInputEvent(appEvent, win.surface.eventScale()))
		}
	})
	client.Desktop.OnResize(func(ev desktopapi.WindowResizeEvent) {
		if win := b.windowFor(ev.WindowId); win != nil {
			if err := win.surface.handleRemoteResize(int(ev.Width), int(ev.Height)); err != nil {
				win.surface.pushEvent(event.CloseEvent{})
			}
		}
	})
	client.Desktop.OnCloseRequested(func(ev desktopapi.WindowRequest) {
		if win := b.windowFor(ev.WindowId); win != nil {
			win.surface.pushEvent(event.CloseEvent{})
		}
	})
	client.OnDisconnect(func() {
		b.handleDisconnect()
	})

	b.mu.Lock()
	b.client = client
	b.initialized = true
	b.mu.Unlock()
	return nil
}

func (b *Backend) Shutdown() {
	b.mu.Lock()
	windows := make([]*window, 0, len(b.windows))
	for _, win := range b.windows {
		windows = append(windows, win)
	}
	client := b.client
	b.client = nil
	b.initialized = false
	b.windows = make(map[uint32]*window)
	b.mu.Unlock()

	for _, win := range windows {
		win.close(false)
	}
	if client != nil {
		_ = client.Close()
	}
}

func (b *Backend) NewWindow(opts backend.WindowOptions) (backend.Window, error) {
	if opts.Layer != nil {
		return nil, fmt.Errorf("desktop backend does not support layer surfaces")
	}

	client := b.clientConn()
	if client == nil {
		return nil, fmt.Errorf("desktop backend is not initialized")
	}

	width := opts.Width
	if width <= 0 {
		width = defaultWidth
	}
	height := opts.Height
	if height <= 0 {
		height = defaultHeight
	}
	scale := normalizeScale(opts.Scale)
	if opts.Scale <= 0 {
		scale = b.Scale()
	}

	surface, err := newSurface(b, width, height, scale)
	if err != nil {
		return nil, err
	}

	appID, appName, icon := inferDesktopIdentity(opts.Title)
	minWidth, minHeight := inferMinimumSize(opts, width, height)
	resp, err := client.Desktop.CreateWindow(desktopapi.WindowCreateRequest{
		AppId:      appID,
		AppName:    appName,
		Title:      nonEmpty(strings.TrimSpace(opts.Title), appName),
		Icon:       icon,
		Width:      uint32(width),
		Height:     uint32(height),
		MinWidth:   uint32(minWidth),
		MinHeight:  uint32(minHeight),
		ScaleMilli: scaleMilli(scale),
	})
	if err != nil {
		surface.closeLocal()
		return nil, err
	}

	win := &window{
		backend: b,
		surface: surface,
		title:   nonEmpty(strings.TrimSpace(opts.Title), appName),
	}
	surface.windowID = resp.WindowId
	win.windowID = resp.WindowId

	b.mu.Lock()
	b.windows[resp.WindowId] = win
	b.mu.Unlock()
	return win, nil
}

func (b *Backend) PollEvents() ([]event.Event, error) {
	b.mu.RLock()
	windows := make([]*window, 0, len(b.windows))
	for _, win := range b.windows {
		windows = append(windows, win)
	}
	b.mu.RUnlock()

	var evs []event.Event
	for _, win := range windows {
		if win == nil || win.surface == nil {
			continue
		}
		evs = append(evs, win.surface.PollEvents()...)
	}
	return evs, nil
}

func (b *Backend) WaitEvent() ([]event.Event, error) {
	return b.PollEvents()
}

func (b *Backend) windowFor(windowID uint32) *window {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.windows[windowID]
}

func (b *Backend) clientConn() *desktopapi.Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.client
}

func (b *Backend) unregister(windowID uint32) {
	b.mu.Lock()
	delete(b.windows, windowID)
	b.mu.Unlock()
}

func (b *Backend) handleDisconnect() {
	b.mu.Lock()
	windows := make([]*window, 0, len(b.windows))
	for _, win := range b.windows {
		windows = append(windows, win)
	}
	b.client = nil
	b.initialized = false
	b.mu.Unlock()

	for _, win := range windows {
		if win != nil && win.surface != nil {
			win.surface.pushEvent(event.CloseEvent{})
		}
	}
}

var _ backend.Backend = (*Backend)(nil)

type window struct {
	backend  *Backend
	surface  *surface
	windowID uint32
	title    string
	once     sync.Once
}

func (w *window) Surface() backend.Surface { return w.surface }

func (w *window) SetTitle(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	w.title = title
	client := w.backend.clientConn()
	if client == nil || w.windowID == 0 {
		return
	}
	_ = client.Desktop.UpdateWindowState(desktopapi.WindowStateRequest{
		WindowId: w.windowID,
		Title:    title,
	})
}

func (w *window) Size() (int, int) {
	if w.surface == nil {
		return 0, 0
	}
	return w.surface.dimensions()
}

func (w *window) Close() { w.close(true) }

func (w *window) close(destroyRemote bool) {
	w.once.Do(func() {
		if w.backend != nil && w.windowID != 0 {
			w.backend.unregister(w.windowID)
			if destroyRemote {
				if client := w.backend.clientConn(); client != nil {
					_ = client.Desktop.DestroyWindow(desktopapi.WindowRequest{WindowId: w.windowID})
				}
			}
		}
		if w.surface != nil {
			w.surface.closeLocal()
		}
	})
}

var _ backend.Window = (*window)(nil)

type surface struct {
	backend  *Backend
	windowID uint32

	frameMu sync.Mutex
	mu      sync.RWMutex
	buffers [2]*mappedRGBA
	server  int
	render  int
	canvas  canvas.Canvas
	width   int
	height  int
	pixelW  int
	pixelH  int
	scale   float64
	closed  bool

	// sentBufPaths tracks the last buffer path acknowledged by the compositor
	// for each buffer slot. UpdateWindowBuffer is skipped when the path is
	// unchanged (steady-state double-buffering), reducing per-frame IPC.
	sentBufPaths [2]string

	eventsMu sync.Mutex
	events   []event.Event

	cursorShape event.CursorShape
}

func newSurface(b *Backend, width, height int, scale float64) (*surface, error) {
	scale = normalizeScale(scale)
	pixelW := scalePixels(width, scale)
	pixelH := scalePixels(height, scale)
	buffers, err := allocMappedPair(pixelW, pixelH)
	if err != nil {
		return nil, err
	}
	return &surface{
		backend: b,
		buffers: buffers,
		server:  0,
		render:  1,
		canvas:  pixbuf.NewCanvasFromImage(buffers[1].img),
		width:   width,
		height:  height,
		pixelW:  pixelW,
		pixelH:  pixelH,
		scale:   scale,
	}, nil
}

func (s *surface) Begin() (canvas.Canvas, error) {
	s.frameMu.Lock()
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		s.frameMu.Unlock()
		return nil, fmt.Errorf("desktop surface is closed")
	}
	c := s.canvas
	s.mu.RUnlock()
	if c == nil {
		s.frameMu.Unlock()
		return nil, fmt.Errorf("desktop surface has no canvas")
	}
	return c, nil
}

func (s *surface) Present(c canvas.Canvas) error {
	defer s.frameMu.Unlock()
	s.mu.RLock()
	closed := s.closed
	windowID := s.windowID
	width := s.width
	height := s.height
	scale := s.scale
	renderIndex := s.render
	renderBuffer := s.buffers[renderIndex]
	lastSentPath := s.sentBufPaths[renderIndex]
	s.mu.RUnlock()
	if closed {
		return fmt.Errorf("desktop surface is closed")
	}
	if renderBuffer == nil {
		return fmt.Errorf("desktop surface has no render buffer")
	}
	client := s.backend.clientConn()
	if client == nil {
		return fmt.Errorf("desktop backend disconnected")
	}

	// Only tell the compositor about this buffer slot when its path is new.
	// After the first two frames each path is cached; no further IPC needed
	// for steady-state rendering.
	if renderBuffer.path != lastSentPath {
		err := client.Desktop.UpdateWindowBuffer(desktopapi.WindowBufferRequest{
			WindowId:   windowID,
			Width:      uint32(width),
			Height:     uint32(height),
			ScaleMilli: scaleMilli(scale),
			BufferPath: renderBuffer.path,
		})
		if err != nil {
			return err
		}
		s.mu.Lock()
		s.sentBufPaths[renderIndex] = renderBuffer.path
		s.mu.Unlock()
	}

	// Collect the precise damaged region from the canvas so the compositor
	// can restrict its own repaint and the DRM blit to the changed pixels.
	req := desktopapi.WindowPresentRequest{
		WindowId:   windowID,
		BufferPath: renderBuffer.path,
	}
	if dt, ok := c.(interface {
		Dirty() image.Rectangle
	}); ok {
		if d := dt.Dirty(); !d.Empty() {
			req.DamageRects = []desktopapi.DamageRect{{
				X: uint32(d.Min.X),
				Y: uint32(d.Min.Y),
				W: uint32(d.Dx()),
				H: uint32(d.Dy()),
			}}
		}
	}

	if err := client.Desktop.PresentWindow(req); err != nil {
		return err
	}

	s.mu.Lock()
	if !s.closed && s.render == renderIndex {
		s.server = renderIndex
		s.render = 1 - renderIndex
		next := s.buffers[s.render]
		if next != nil {
			s.canvas = pixbuf.NewCanvasFromImage(next.img)
		} else {
			s.canvas = nil
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *surface) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid size %dx%d", width, height)
	}
	s.frameMu.Lock()
	defer s.frameMu.Unlock()

	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return fmt.Errorf("desktop surface is closed")
	}
	if s.pixelW == width && s.pixelH == height {
		s.mu.RUnlock()
		return nil
	}
	scale := s.scale
	s.mu.RUnlock()

	buffers, err := allocMappedPair(width, height)
	if err != nil {
		return err
	}

	s.mu.Lock()
	old := s.buffers
	s.buffers = buffers
	s.server = 0
	s.render = 1
	s.canvas = pixbuf.NewCanvasFromImage(buffers[1].img)
	s.width = logicalPixels(width, scale)
	s.height = logicalPixels(height, scale)
	s.pixelW = width
	s.pixelH = height
	s.sentBufPaths = [2]string{} // new paths — force UpdateWindowBuffer on next present
	closed := s.closed
	s.mu.Unlock()

	for _, buffer := range old {
		if buffer != nil {
			buffer.Close()
		}
	}
	if closed {
		for _, buffer := range buffers {
			if buffer != nil {
				buffer.Close()
			}
		}
		return fmt.Errorf("desktop surface is closed")
	}
	return nil
}

func (s *surface) Size() geom.Size {
	w, h := s.dimensions()
	return geom.Sz(float64(w), float64(h))
}

func (s *surface) PollEvents() []event.Event {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	if len(s.events) == 0 {
		return nil
	}
	out := s.events
	s.events = nil
	return out
}

func (s *surface) dimensions() (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pixelW, s.pixelH
}

func (s *surface) pushEvent(ev event.Event) {
	s.eventsMu.Lock()
	s.events = append(s.events, ev)
	s.eventsMu.Unlock()
}

func (s *surface) handleRemoteResize(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid size %dx%d", width, height)
	}
	s.mu.RLock()
	scale := s.scale
	s.mu.RUnlock()
	s.pushEvent(event.ResizeEvent{
		Width:  scalePixels(width, scale),
		Height: scalePixels(height, scale),
	})
	return nil
}

func (s *surface) eventScale() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scale
}

func (s *surface) SetCursorShape(shape event.CursorShape) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("desktop surface is closed")
	}
	if s.cursorShape == shape {
		s.mu.Unlock()
		return nil
	}
	s.cursorShape = shape
	windowID := s.windowID
	s.mu.Unlock()

	if windowID == 0 {
		return nil
	}
	client := s.backend.clientConn()
	if client == nil {
		return fmt.Errorf("desktop backend disconnected")
	}
	return client.Desktop.UpdateCursor(desktopapi.WindowCursorRequest{
		WindowId: windowID,
		Shape:    uint32(shape),
	})
}

func (s *surface) closeLocal() {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	buffers := s.buffers
	s.buffers = [2]*mappedRGBA{}
	s.canvas = nil
	s.closed = true
	s.mu.Unlock()
	for _, buffer := range buffers {
		if buffer != nil {
			buffer.Close()
		}
	}
}

var _ backend.Surface = (*surface)(nil)
var _ backend.EventPoller = (*surface)(nil)
var _ backend.CursorSetter = (*surface)(nil)

type mappedRGBA struct {
	path   string
	data   []byte
	img    *image.RGBA
	width  int
	height int
}

func allocMappedPair(width, height int) ([2]*mappedRGBA, error) {
	var out [2]*mappedRGBA
	for i := range out {
		buffer, err := allocMappedRGBA(width, height)
		if err != nil {
			for _, existing := range out {
				if existing != nil {
					existing.Close()
				}
			}
			return [2]*mappedRGBA{}, err
		}
		out[i] = buffer
	}
	return out, nil
}

func allocMappedRGBA(width, height int) (*mappedRGBA, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid size %dx%d", width, height)
	}

	dir := fs.Resolve("shared:desktop")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0o777)

	file, err := os.CreateTemp(dir, "window-*.rgba")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	size := width * height * 4
	if err := file.Truncate(int64(size)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	_ = file.Chmod(0o666)

	data, err := syscall.Mmap(int(file.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = syscall.Munmap(data)
		_ = os.Remove(path)
		return nil, err
	}

	return &mappedRGBA{
		path:   path,
		data:   data,
		img:    &image.RGBA{Pix: data, Stride: width * 4, Rect: image.Rect(0, 0, width, height)},
		width:  width,
		height: height,
	}, nil
}

func (m *mappedRGBA) Close() {
	if m == nil {
		return
	}
	if len(m.data) > 0 {
		_ = syscall.Munmap(m.data)
	}
	if m.path != "" {
		_ = os.Remove(m.path)
	}
	m.data = nil
	m.img = nil
	m.path = ""
}

func resolveSocketPath(path string) string {
	path = strings.TrimSpace(path)
	if path != "" {
		return path
	}
	return fs.Resolve("system:%s", desktopapi.ServiceName)
}

func normalizeScale(scale float64) float64 {
	if scale <= 0 {
		return defaultScale
	}
	return scale
}

func scaleMilli(scale float64) uint32 {
	milli := int(math.Round(normalizeScale(scale) * 1000))
	if milli <= 0 {
		return 1000
	}
	return uint32(milli)
}

func scalePixels(value int, scale float64) int {
	if value <= 0 {
		return 1
	}
	px := int(math.Round(float64(value) * normalizeScale(scale)))
	if px <= 0 {
		return 1
	}
	return px
}

func logicalPixels(value int, scale float64) int {
	if value <= 0 {
		return 1
	}
	logical := int(math.Round(float64(value) / normalizeScale(scale)))
	if logical <= 0 {
		return 1
	}
	return logical
}

func scaleInputEvent(ev event.Event, scale float64) event.Event {
	scale = normalizeScale(scale)
	switch e := ev.(type) {
	case event.ButtonEvent:
		e.X *= scale
		e.Y *= scale
		return e
	case event.CursorEvent:
		e.X *= scale
		e.Y *= scale
		return e
	case event.ScrollEvent:
		e.X *= scale
		e.Y *= scale
		return e
	default:
		return ev
	}
}

func inferDesktopIdentity(title string) (appID, appName, icon string) {
	base := filepath.Base(strings.TrimSpace(os.Args[0]))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSpace(base)

	if mf, ok := loadDesktopManifest(os.Args[0]); ok {
		if id := normalizeAppID(mf.ID); id != "" {
			appID = id
		}
		if name := strings.TrimSpace(mf.Name); name != "" {
			appName = name
		}
		if appID != "" {
			icon = filepath.Base(filepath.Dir(strings.TrimSpace(os.Args[0])))
		}
	}

	if base == "" || base == "." || base == string(filepath.Separator) {
		base = normalizeAppID(title)
	}
	if base == "" {
		base = "app"
	}
	if appID == "" {
		appID = normalizeAppID(base)
		if appID == "" {
			appID = "app"
		}
	}
	if appName == "" {
		appName = strings.TrimSpace(title)
		if appName == "" {
			appName = humanizeAppName(base)
		}
	}
	if icon == "" {
		icon = appID
	}
	return appID, appName, icon
}

type desktopManifest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func loadDesktopManifest(exe string) (desktopManifest, bool) {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return desktopManifest{}, false
	}

	manifestPath := filepath.Join(filepath.Dir(exe), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return desktopManifest{}, false
	}

	var mf desktopManifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return desktopManifest{}, false
	}
	return mf, true
}

func inferMinimumSize(opts backend.WindowOptions, width, height int) (int, int) {
	if !opts.Resizable {
		return width, height
	}
	minWidth := width / 2
	minHeight := height / 2
	if minWidth < defaultMinWidth {
		minWidth = defaultMinWidth
	}
	if minHeight < defaultMinHeight {
		minHeight = defaultMinHeight
	}
	return minWidth, minHeight
}

func normalizeAppID(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', r == ' ', r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func humanizeAppName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "App"
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == ' '
	})
	if len(parts) == 0 {
		return "App"
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
