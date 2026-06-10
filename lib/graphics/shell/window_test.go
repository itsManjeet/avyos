package shell

import (
	"net"
	"testing"

	desktopapi "avyos.dev/api/desktop"
	"avyos.dev/lib/graphics/event"
	"avyos.dev/lib/sutra"
)

type recvResult struct {
	tx  sutra.Transaction
	err error
}

func captureDesktopEvent(t *testing.T, send func(*Window) error) sutra.Transaction {
	t.Helper()

	local, remote := net.Pipe()
	t.Cleanup(func() {
		_ = local.Close()
		_ = remote.Close()
	})

	recvCh := make(chan recvResult, 1)
	go func() {
		tx, err := sutra.NewConn(remote).Recv()
		recvCh <- recvResult{tx: tx, err: err}
	}()

	win := &Window{
		ID:       11,
		ClientID: 7,
		ctrl: &Controller{
			conns: map[uint32]*sutra.Conn{
				7: sutra.NewConn(local),
			},
		},
	}

	if err := send(win); err != nil {
		t.Fatalf("send event: %v", err)
	}

	res := <-recvCh
	if res.err != nil {
		t.Fatalf("receive event: %v", res.err)
	}
	if res.tx.Object != 0 {
		t.Fatalf("expected Desktop object 0, got %d", res.tx.Object)
	}
	return res.tx
}

func TestWindowSendInputUsesDesktopObject(t *testing.T) {
	tx := captureDesktopEvent(t, func(w *Window) error {
		return w.SendInput([]byte{1, 2, 3})
	})

	if tx.Event != desktopapi.OpDesktopInput {
		t.Fatalf("expected input opcode %d, got %d", desktopapi.OpDesktopInput, tx.Event)
	}

	args, err := sutra.UnmarshalPayload[desktopapi.WindowInputEvent](tx.Payload)
	if err != nil {
		t.Fatalf("decode input payload: %v", err)
	}
	if args.WindowId != 11 {
		t.Fatalf("expected window id 11, got %d", args.WindowId)
	}
	if got := string(args.Payload); got != string([]byte{1, 2, 3}) {
		t.Fatalf("expected payload %v, got %v", []byte{1, 2, 3}, args.Payload)
	}
}

func TestWindowSendResizeUsesDesktopObject(t *testing.T) {
	tx := captureDesktopEvent(t, func(w *Window) error {
		return w.SendResize(640, 480)
	})

	if tx.Event != desktopapi.OpDesktopResize {
		t.Fatalf("expected resize opcode %d, got %d", desktopapi.OpDesktopResize, tx.Event)
	}

	args, err := sutra.UnmarshalPayload[desktopapi.WindowResizeEvent](tx.Payload)
	if err != nil {
		t.Fatalf("decode resize payload: %v", err)
	}
	if args.WindowId != 11 || args.Width != 640 || args.Height != 480 {
		t.Fatalf("unexpected resize args: %+v", args)
	}
}

func TestWindowSendCloseRequestedUsesDesktopObject(t *testing.T) {
	tx := captureDesktopEvent(t, func(w *Window) error {
		return w.SendCloseRequested()
	})

	if tx.Event != desktopapi.OpDesktopCloseRequested {
		t.Fatalf("expected close opcode %d, got %d", desktopapi.OpDesktopCloseRequested, tx.Event)
	}

	args, err := sutra.UnmarshalPayload[desktopapi.WindowRequest](tx.Payload)
	if err != nil {
		t.Fatalf("decode close payload: %v", err)
	}
	if args.WindowId != 11 {
		t.Fatalf("expected window id 11, got %d", args.WindowId)
	}
}

func TestControllerUpdateCursorUpdatesWindowAndInvokesCallback(t *testing.T) {
	win := &Window{ID: 11}
	var gotShape event.CursorShape
	ctrl := &Controller{
		wins: map[windowKey]*Window{
			{clientID: 7, windowID: 11}: win,
		},
		OnCursor: func(w *Window, shape event.CursorShape) {
			if w != win {
				t.Fatalf("callback window mismatch: got %p want %p", w, win)
			}
			gotShape = shape
		},
	}

	if err := ctrl.UpdateCursor(7, desktopapi.WindowCursorRequest{
		WindowId: 11,
		Shape:    uint32(event.CursorText),
	}); err != nil {
		t.Fatalf("UpdateCursor failed: %v", err)
	}

	if win.Cursor != event.CursorText {
		t.Fatalf("window cursor = %v, want %v", win.Cursor, event.CursorText)
	}
	if gotShape != event.CursorText {
		t.Fatalf("callback cursor = %v, want %v", gotShape, event.CursorText)
	}
}
