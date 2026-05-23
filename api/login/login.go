package login

// Logout sends a logout request to the session manager.
func (c *Client) Logout() error {
	return c.Login.Logout()
}

// Lock sends a lock request to the session manager.
func (c *Client) Lock() error {
	return c.Login.Lock()
}

// GetSession returns the current session info.
func (c *Client) GetSession() (SessionInfo, error) {
	return c.Login.GetSession()
}

// OnSessionStarted registers a callback for SessionStarted events.
func (c *Client) OnSessionStarted(fn func(SessionInfo)) {
	c.Login.OnSessionStarted(fn)
}

// OnSessionEnded registers a callback for SessionEnded events.
func (c *Client) OnSessionEnded(fn func()) {
	c.Login.OnSessionEnded(fn)
}
