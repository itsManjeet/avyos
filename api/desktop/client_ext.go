package desktop

// OnDisconnect is a no-op placeholder; disconnect is detected via Recv errors.
func (c *Client) OnDisconnect(_ func()) {}
