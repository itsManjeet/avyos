/* macos.m – Cocoa backend implementation (Objective-C, compiled via CGO) */
#include "macos.h"
#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>

/* ─────────────────────────────────────────────────────────────
 *  Event queue (per window, ring buffer)
 * ───────────────────────────────────────────────────────────── */

#define EVQ_CAP 256

typedef struct {
  GEvent buf[EVQ_CAP];
  int head;
  int tail;
} EventQueue;

static void evq_push(EventQueue *q, GEvent e) {
  int next = (q->tail + 1) % EVQ_CAP;
  if (next == q->head)
    return; /* drop on full */
  q->buf[q->tail] = e;
  q->tail = next;
}

static int evq_pop(EventQueue *q, GEvent *out) {
  if (q->head == q->tail)
    return 0;
  *out = q->buf[q->head];
  q->head = (q->head + 1) % EVQ_CAP;
  return 1;
}

/* ─────────────────────────────────────────────────────────────
 *  GraphicsView – NSView subclass that blits an RGBA pixel buffer
 * ───────────────────────────────────────────────────────────── */

@interface GraphicsView : NSView {
  unsigned char *_pixels;
  int _pixW;
  int _pixH;
  NSLock *_lock;
  EventQueue _evq;
}
- (void)setPixels:(const unsigned char *)rgba width:(int)w height:(int)h;
- (int)pollEvent:(GEvent *)out;
- (void)pushEvent:(GEvent)e;
@end

@implementation GraphicsView

- (instancetype)initWithFrame:(NSRect)frame {
  self = [super initWithFrame:frame];
  if (self) {
    _lock = [[NSLock alloc] init];
    _pixels = NULL;
    _pixW = 0;
    _pixH = 0;
  }
  return self;
}

- (void)dealloc {
  free(_pixels);
  [_lock release];
  [super dealloc];
}

- (BOOL)acceptsFirstResponder {
  return YES;
}
- (BOOL)isOpaque {
  return YES;
}

- (void)setPixels:(const unsigned char *)rgba width:(int)w height:(int)h {
  [_lock lock];
  int sz = w * h * 4;
  if (_pixW != w || _pixH != h) {
    free(_pixels);
    _pixels = (unsigned char *)malloc(sz);
    _pixW = w;
    _pixH = h;
  }
  if (_pixels && rgba)
    memcpy(_pixels, rgba, sz);
  [_lock unlock];
  /* Schedule redraw on main thread. */
  dispatch_async(dispatch_get_main_queue(), ^{
    [self setNeedsDisplay:YES];
  });
}

- (void)drawRect:(NSRect)dirtyRect {
  [_lock lock];
  unsigned char *pix = _pixels;
  int w = _pixW, h = _pixH;
  if (!pix || w == 0 || h == 0) {
    [_lock unlock];
    return;
  }

  /* NSBitmapImageRep handles NSView coordinate-system flipping automatically.
   * Row 0 of the pixel data (top of the Metal texture) is drawn at the top
   * of the view regardless of whether the context uses Y-up or Y-down coords.
   */
  NSBitmapImageRep *rep =
      [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:&pix
                                              pixelsWide:(NSInteger)w
                                              pixelsHigh:(NSInteger)h
                                           bitsPerSample:8
                                         samplesPerPixel:4
                                                hasAlpha:YES
                                                isPlanar:NO
                                          colorSpaceName:NSDeviceRGBColorSpace
                                             bytesPerRow:(NSInteger)(w * 4)
                                            bitsPerPixel:32];
  [_lock unlock];
  if (rep) {
    [rep drawInRect:self.bounds];
    [rep release];
  }
}

/* ─── Key translation ────────────────────────────────────────── */

static int translateKeyCode(unsigned short vc) {
  /* Maps macOS virtual key codes (Carbon kVK_*) → GKEY_* constants.
   * GKEY_* are kept in sync with event.KeyCode iota values in event/event.go.
   * See /System/Library/Frameworks/Carbon.framework/Headers/Events.h */
  switch (vc) {
  /* ── Letters ── */
  case 0x00: return GKEY_A;
  case 0x0B: return GKEY_B;
  case 0x08: return GKEY_C;
  case 0x02: return GKEY_D;
  case 0x0E: return GKEY_E;
  case 0x03: return GKEY_F;
  case 0x05: return GKEY_G;
  case 0x04: return GKEY_H;
  case 0x22: return GKEY_I;
  case 0x26: return GKEY_J;
  case 0x28: return GKEY_K;
  case 0x25: return GKEY_L;
  case 0x2E: return GKEY_M;
  case 0x2D: return GKEY_N;
  case 0x1F: return GKEY_O;
  case 0x23: return GKEY_P;
  case 0x0C: return GKEY_Q;
  case 0x0F: return GKEY_R;
  case 0x01: return GKEY_S;
  case 0x11: return GKEY_T;
  case 0x20: return GKEY_U;
  case 0x09: return GKEY_V;
  case 0x0D: return GKEY_W;
  case 0x07: return GKEY_X;
  case 0x10: return GKEY_Y;
  case 0x06: return GKEY_Z;
  /* ── Digits ── */
  case 0x1D: return GKEY_0;
  case 0x12: return GKEY_1;
  case 0x13: return GKEY_2;
  case 0x14: return GKEY_3;
  case 0x15: return GKEY_4;
  case 0x17: return GKEY_5;
  case 0x16: return GKEY_6;
  case 0x1A: return GKEY_7;
  case 0x1C: return GKEY_8;
  case 0x19: return GKEY_9;
  /* ── Navigation ── */
  case 0x7E: return GKEY_ARROW_UP;
  case 0x7D: return GKEY_ARROW_DOWN;
  case 0x7B: return GKEY_ARROW_LEFT;
  case 0x7C: return GKEY_ARROW_RIGHT;
  case 0x73: return GKEY_HOME;
  case 0x77: return GKEY_END;
  case 0x74: return GKEY_PAGE_UP;
  case 0x79: return GKEY_PAGE_DOWN;
  /* ── Editing ── */
  case 0x33: return GKEY_BACKSPACE;   /* kVK_Delete (backspace) */
  case 0x75: return GKEY_DELETE;      /* kVK_ForwardDelete */
  case 0x72: return GKEY_INSERT;      /* kVK_Help (Insert on ext. kb) */
  case 0x30: return GKEY_TAB;
  case 0x24: return GKEY_ENTER;
  case 0x4C: return GKEY_ENTER;       /* numpad enter */
  case 0x35: return GKEY_ESCAPE;
  case 0x31: return GKEY_SPACE;
  /* ── Function keys ── */
  case 0x7A: return GKEY_F1;
  case 0x78: return GKEY_F2;
  case 0x63: return GKEY_F3;
  case 0x76: return GKEY_F4;
  case 0x60: return GKEY_F5;
  case 0x61: return GKEY_F6;
  case 0x62: return GKEY_F7;
  case 0x64: return GKEY_F8;
  case 0x65: return GKEY_F9;
  case 0x6D: return GKEY_F10;
  case 0x67: return GKEY_F11;
  case 0x6F: return GKEY_F12;
  /* ── Modifiers ── */
  case 0x38: return GKEY_SHIFT;  /* Left Shift */
  case 0x3C: return GKEY_SHIFT;  /* Right Shift */
  case 0x3B: return GKEY_CTRL;   /* Left Control */
  case 0x3E: return GKEY_CTRL;   /* Right Control */
  case 0x3A: return GKEY_ALT;    /* Left Option */
  case 0x3D: return GKEY_ALT;    /* Right Option */
  case 0x37: return GKEY_SUPER;  /* Left Command */
  case 0x36: return GKEY_SUPER;  /* Right Command */
  /* ── Punctuation ── */
  case 0x1B: return GKEY_MINUS;
  case 0x18: return GKEY_EQUAL;
  case 0x21: return GKEY_LEFT_BRACKET;
  case 0x1E: return GKEY_RIGHT_BRACKET;
  case 0x29: return GKEY_SEMICOLON;
  case 0x27: return GKEY_APOSTROPHE;
  case 0x32: return GKEY_GRAVE;
  case 0x2A: return GKEY_BACKSLASH;
  case 0x2B: return GKEY_COMMA;
  case 0x2F: return GKEY_PERIOD;
  case 0x2C: return GKEY_SLASH;
  default:   return GKEY_UNKNOWN;
  }
}

static int translateMods(NSEventModifierFlags f) {
  int m = 0;
  if (f & NSEventModifierFlagShift)
    m |= GMOD_SHIFT;
  if (f & NSEventModifierFlagControl)
    m |= GMOD_CTRL;
  if (f & NSEventModifierFlagOption)
    m |= GMOD_ALT;
  if (f & NSEventModifierFlagCommand)
    m |= GMOD_SUPER;
  return m;
}

/* ─── Coordinate helpers ────────────────────────────────────── */

/* eventPhysPoint: converts an NSEvent's window-space location to view-space
 * PHYSICAL pixels with Y-down orientation (origin at top-left).
 *
 * Why physical pixels?  DefaultHandler in app.go divides all mouse coords by
 * the backing scale factor before passing them to widget.Frame (which operates
 * in logical/point coordinates).  Emitting physical pixels here ensures that
 * division produces correct logical coordinates on both Retina and non-Retina
 * displays.  Previously logical-pixel coords were emitted, causing them to be
 * halved on Retina. */
- (NSPoint)eventPhysPoint:(NSEvent *)e {
  /* Convert window coords → view logical coords → view physical coords. */
  NSPoint logical = [self convertPoint:e.locationInWindow fromView:nil];
  NSPoint phys = [self convertPointToBacking:logical];
  /* Flip Y: NSView has origin at bottom-left; we want top-left. */
  float physH = (float)[self convertSizeToBacking:self.bounds.size].height;
  return NSMakePoint(phys.x, physH - phys.y);
}

/* ─── Keyboard events ───────────────────────────────────────── */

- (void)keyDown:(NSEvent *)e {
  GEvent ge = {0};
  ge.type = GEVT_KEYDOWN;
  ge.keyCode = translateKeyCode(e.keyCode);
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);

  /* Emit rune events for printable input, but NOT when a command modifier
   * (Ctrl, Alt, Cmd) is held — those are keyboard shortcuts, not text input.
   * Shift is allowed because it changes the character (e.g. 'A' vs 'a'). */
  if (!(ge.mods & (GMOD_CTRL | GMOD_ALT | GMOD_SUPER))) {
    NSString *chars = e.characters;
    if (chars != nil) {
      for (NSUInteger i = 0; i < chars.length; i++) {
        unichar c = [chars characterAtIndex:i];
        /* Exclude control characters (< 0x20) and DEL (0x7F). */
        if (c >= 0x20 && c != 0x7F) {
          GEvent re = {0};
          re.type = GEVT_RUNE;
          re.rune = (unsigned int)c;
          evq_push(&_evq, re);
        }
      }
    }
  }
}

- (void)keyUp:(NSEvent *)e {
  GEvent ge = {0};
  ge.type = GEVT_KEYUP;
  ge.keyCode = translateKeyCode(e.keyCode);
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

/* flagsChanged: fires when a modifier key (Shift/Ctrl/Option/Command) is
 * pressed or released.  We derive the direction from whether the relevant
 * modifier flag is now set or cleared. */
- (void)flagsChanged:(NSEvent *)e {
  unsigned short vc = e.keyCode;
  NSEventModifierFlags flags = e.modifierFlags;
  int keyCode = translateKeyCode(vc);
  if (keyCode == GKEY_UNKNOWN)
    return;

  /* Determine up/down from the flag that this key controls. */
  BOOL isDown = NO;
  switch (vc) {
  case 0x38: case 0x3C: isDown = (flags & NSEventModifierFlagShift)   != 0; break;
  case 0x3B: case 0x3E: isDown = (flags & NSEventModifierFlagControl) != 0; break;
  case 0x3A: case 0x3D: isDown = (flags & NSEventModifierFlagOption)  != 0; break;
  case 0x37: case 0x36: isDown = (flags & NSEventModifierFlagCommand) != 0; break;
  default: return;
  }

  GEvent ge = {0};
  ge.type = isDown ? GEVT_KEYDOWN : GEVT_KEYUP;
  ge.keyCode = keyCode;
  ge.mods = translateMods(flags);
  evq_push(&_evq, ge);
}

/* ─── Mouse events ───────────────────────────────────────────── */

- (void)mouseDown:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEDOWN;
  ge.button = 1;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

- (void)mouseUp:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEUP;
  ge.button = 1;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

- (void)rightMouseDown:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEDOWN;
  ge.button = 3;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

- (void)rightMouseUp:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEUP;
  ge.button = 3;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

- (void)otherMouseDown:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEDOWN;
  ge.button = 2; /* middle */
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

- (void)otherMouseUp:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEUP;
  ge.button = 2;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  ge.mods = translateMods(e.modifierFlags);
  evq_push(&_evq, ge);
}

/* mouseMoved: fires for hover (no button held). */
- (void)mouseMoved:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_MOUSEMOVE;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  evq_push(&_evq, ge);
}

/* Dragged variants fire while the corresponding button is held.
 * All delegate to mouseMoved: which emits a GEVT_MOUSEMOVE — the framework
 * detects that pressedPath is set and fires onDragMove. */
- (void)mouseDragged:(NSEvent *)e      { [self mouseMoved:e]; }
- (void)rightMouseDragged:(NSEvent *)e { [self mouseMoved:e]; }
- (void)otherMouseDragged:(NSEvent *)e { [self mouseMoved:e]; }

- (void)scrollWheel:(NSEvent *)e {
  NSPoint p = [self eventPhysPoint:e];
  GEvent ge = {0};
  ge.type = GEVT_SCROLL;
  ge.x = (float)p.x;
  ge.y = (float)p.y;
  /* Scroll deltas are in logical (point) units — do not scale. */
  ge.dx = (float)e.scrollingDeltaX;
  ge.dy = (float)e.scrollingDeltaY;
  evq_push(&_evq, ge);
}

- (int)pollEvent:(GEvent *)out {
  return evq_pop(&_evq, out);
}
- (void)pushEvent:(GEvent)e {
  evq_push(&_evq, e);
}

@end

/* ─────────────────────────────────────────────────────────────
 *  GWindowHandle – pairs NSWindow + GraphicsView
 * ───────────────────────────────────────────────────────────── */

@interface GWindowHandle : NSObject <NSWindowDelegate> {
@public
  NSWindow *window;
  GraphicsView *view;
}
@end

@implementation GWindowHandle

- (void)windowWillClose:(NSNotification *)n {
  GEvent ge = {0};
  ge.type = GEVT_CLOSE;
  [view pushEvent:ge];
}

- (void)windowDidResize:(NSNotification *)n {
  /* Report physical backing pixels so the canvas is sized correctly on Retina.
   */
  NSSize logical = view.bounds.size;
  NSSize physical = [view convertSizeToBacking:logical];
  GEvent ge = {0};
  ge.type = GEVT_RESIZE;
  ge.width = (int)physical.width;
  ge.height = (int)physical.height;
  [view pushEvent:ge];
}

- (void)windowDidBecomeKey:(NSNotification *)n {
  GEvent ge = {0};
  ge.type = GEVT_FOCUS;
  [view pushEvent:ge];
}

- (void)windowDidResignKey:(NSNotification *)n {
  GEvent ge = {0};
  ge.type = GEVT_BLUR;
  [view pushEvent:ge];
}

@end

/* ─────────────────────────────────────────────────────────────
 *  C API implementation
 * ───────────────────────────────────────────────────────────── */

void gfx_init(void) {
  [NSApplication sharedApplication];
  [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

  /* Create a minimal menu bar so the app behaves properly. */
  NSMenu *menuBar = [[NSMenu alloc] init];
  NSMenuItem *appItem = [[NSMenuItem alloc] init];
  [menuBar addItem:appItem];
  NSMenu *appMenu = [[NSMenu alloc] init];
  NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit"
                                                    action:@selector(terminate:)
                                             keyEquivalent:@"q"];
  [appMenu addItem:quitItem];
  [appItem setSubmenu:appMenu];
  [NSApp setMainMenu:menuBar];
  [menuBar release];
  [appItem release];
  [appMenu release];
  [quitItem release];

  [NSApp finishLaunching];
}

uintptr_t gfx_create_window(int width, int height, const char *title) {
  NSRect frame = NSMakeRect(0, 0, width, height);
  NSWindowStyleMask style =
      NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
      NSWindowStyleMaskResizable | NSWindowStyleMaskMiniaturizable;

  NSWindow *window =
      [[NSWindow alloc] initWithContentRect:frame
                                  styleMask:style
                                    backing:NSBackingStoreBuffered
                                      defer:NO];
  [window setTitle:[NSString stringWithUTF8String:title]];

  GraphicsView *view = [[GraphicsView alloc] initWithFrame:frame];
  [window setContentView:view];
  /* Track mouse moves without button press. */
  [window setAcceptsMouseMovedEvents:YES];

  GWindowHandle *handle = [[GWindowHandle alloc] init];
  handle->window = window;
  handle->view = view;
  [window setDelegate:handle];

  [window center];
  [window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];

  return (uintptr_t)handle;
}

void gfx_destroy_window(uintptr_t h) {
  GWindowHandle *handle = (GWindowHandle *)h;
  [handle->window close];
  [handle->view release];
  [handle->window release];
  [handle release];
}

void gfx_present(uintptr_t h, const unsigned char *rgba, int width,
                 int height) {
  GWindowHandle *handle = (GWindowHandle *)h;
  [handle->view setPixels:rgba width:width height:height];
}

float gfx_scale(uintptr_t h) {
  GWindowHandle *handle = (GWindowHandle *)h;
  return (float)handle->window.backingScaleFactor;
}

float gfx_main_screen_scale(void) {
  return (float)[NSScreen mainScreen].backingScaleFactor;
}

int gfx_poll_event(uintptr_t h, GEvent *out) {
  /* Pump the NSApplication run loop (non-blocking). */
  NSEvent *event;
  while ((event = [NSApp nextEventMatchingMask:NSEventMaskAny
                                     untilDate:[NSDate distantPast]
                                        inMode:NSDefaultRunLoopMode
                                       dequeue:YES]) != nil) {
    [NSApp sendEvent:event];
    [NSApp updateWindows];
  }
  /* Dequeue one translated event. */
  GWindowHandle *handle = (GWindowHandle *)h;
  return [handle->view pollEvent:out];
}
