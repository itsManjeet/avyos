package desktop

import (
	"net"
	"strings"

	"avyos.dev/lib/sutra"
)

const ServiceName = "dev.avyos.desktop"

// NewClient connects to the desktop service at the given Unix socket path and
// starts a background goroutine to dispatch incoming events.
func NewClient(socketPath string) (*Client, error) {
	nc, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	conn := sutra.NewConn(nc)
	c := &Client{
		conn:    conn,
		Desktop: NewDesktopClient(conn, 0),
	}
	go func() {
		for {
			tx, err := conn.Recv()
			if err != nil {
				return
			}
			if !conn.Route(tx) {
				_ = c.HandleEvent(tx)
			}
		}
	}()
	return c, nil
}

// Notify is a convenience wrapper for SendNotification.
func (c *Client) Notify(req NotificationRequest) error {
	req.AppId = strings.TrimSpace(req.AppId)
	req.AppName = strings.TrimSpace(req.AppName)
	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	req.Icon = strings.TrimSpace(req.Icon)
	return c.Desktop.SendNotification(req)
}
