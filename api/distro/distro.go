package distro

import (
	"fmt"
	"strings"

	"avyos.dev/pkg/sutra"
)

const (
	ServiceName    = "dev.avyos.distro"
	defaultShell   = "/bin/sh"
	commandSep     = "\x00"
	defaultCommand = "/bin/sh"
)

// Raw returns the underlying sutra connection.
func (c *Client) Raw() *sutra.Conn {
	return c.conn
}

// GetStatus returns the distro installation status.
func (c *Client) GetStatus() (StatusResponse, error) {
	return c.Distro.Status()
}

// InstallDistro installs the distro rootfs from the given URL.
func (c *Client) InstallDistro(url string) error {
	return c.Distro.Install(InstallRequest{URL: strings.TrimSpace(url)})
}

// RunDistro runs a command inside the distro container.
func (c *Client) RunDistro(req RunRequest) (RunResult, error) {
	if strings.TrimSpace(req.Workdir) == "" {
		req.Workdir = "/"
	}
	if strings.TrimSpace(req.Command) == "" {
		req.Command = defaultCommand
	}
	return c.Distro.Run(req)
}

// Uninstall removes the installed distro rootfs.
func (c *Client) Uninstall() error {
	return c.Distro.Remove()
}

// OpenShell opens an interactive shell session and returns the session ID.
func (c *Client) OpenShell(req ShellOpenRequest) (uint32, error) {
	if strings.TrimSpace(req.Workdir) == "" {
		req.Workdir = "/"
	}
	if req.Rows <= 0 {
		req.Rows = 24
	}
	if req.Cols <= 0 {
		req.Cols = 80
	}
	resp, err := c.Distro.ShellOpen(req)
	if err != nil {
		return 0, err
	}
	return resp.SessionID, nil
}

// SendShellInput sends input data to an open shell session.
func (c *Client) SendShellInput(sessionID uint32, data []byte) error {
	if sessionID == 0 {
		return fmt.Errorf("invalid shell session")
	}
	return c.Distro.ShellInput(ShellInputRequest{SessionID: sessionID, Data: data})
}

// ResizeShell resizes the terminal of an open shell session.
func (c *Client) ResizeShell(sessionID uint32, rows, cols int) error {
	if sessionID == 0 {
		return fmt.Errorf("invalid shell session")
	}
	if rows <= 0 || cols <= 0 {
		return nil
	}
	return c.Distro.ShellResize(ShellResizeRequest{
		SessionID: sessionID,
		Rows:      int32(rows),
		Cols:      int32(cols),
	})
}

// CloseShell closes an open shell session.
func (c *Client) CloseShell(sessionID uint32) error {
	if sessionID == 0 {
		return nil
	}
	return c.Distro.ShellClose(ShellCloseRequest{SessionID: sessionID})
}

// OnShellOutputEvent registers a callback for shell output events.
func (c *Client) OnShellOutputEvent(fn func(ShellOutputEvent)) {
	c.Distro.OnShellOutput(fn)
}

// OnShellExitEvent registers a callback for shell exit events.
func (c *Client) OnShellExitEvent(fn func(ShellExitEvent)) {
	c.Distro.OnShellExit(fn)
}

// EncodeCommand encodes a command slice into the wire format.
func EncodeCommand(args []string) string {
	if len(args) == 0 {
		return defaultCommand
	}
	return strings.Join(args, commandSep)
}

// DecodeCommand decodes a command wire format back to a slice.
func DecodeCommand(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{defaultShell}
	}
	parts := strings.Split(value, commandSep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{defaultShell}
	}
	return out
}
