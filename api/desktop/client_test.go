package desktop

import (
	"net"
	"testing"
	"time"

	"avyos.dev/lib/sutra"
)

func TestClientRoutesResponseWithServerAssignedObject(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := newClient(sutra.NewConn(clientConn))
	defer client.Close()

	server := sutra.NewConn(serverConn)
	defer server.Close()
	go func() {
		tx, err := server.Recv()
		if err != nil {
			return
		}
		payload, err := sutra.MarshalPayload(WindowCreateResponse{WindowId: 7})
		if err != nil {
			return
		}
		_ = server.Send(sutra.Transaction{Object: 42, Event: tx.Event, Payload: payload})
	}()

	type createResult struct {
		resp WindowCreateResponse
		err  error
	}
	result := make(chan createResult, 1)
	go func() {
		resp, err := client.Desktop.CreateWindow(WindowCreateRequest{})
		result <- createResult{resp: resp, err: err}
	}()

	select {
	case result := <-result:
		if result.err != nil {
			t.Fatalf("CreateWindow() error = %v", result.err)
		}
		if result.resp.WindowId != 7 {
			t.Fatalf("CreateWindow() window ID = %d, want 7", result.resp.WindowId)
		}
	case <-time.After(time.Second):
		t.Fatal("CreateWindow() did not route the desktop server response")
	}
}
