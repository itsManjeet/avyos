package desktop

import (
	"net"
	"strings"

	"avyos.dev/lib/sutra"
)

const ServiceName = "dev.avyos.desktop"

// NewClient connects to the desktop service at the given Unix socket path.
func NewClient(socketPath string) (*Client, error) {
	nc, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return newClient(sutra.NewConn(nc)), nil
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
