package service

import (
	"fmt"
	"strings"

	"avyos.dev/pkg/sutra"
)

const ServiceName = "dev.avyos.service"

// Raw returns the underlying sutra connection.
func (c *Client) Raw() *sutra.Conn {
	return c.conn
}

// Start starts the named service.
func (c *Client) Start(name string) error {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return err
	}
	return c.Service.StartService(req)
}

// Stop stops the named service.
func (c *Client) Stop(name string) error {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return err
	}
	return c.Service.StopService(req)
}

// Restart restarts the named service.
func (c *Client) Restart(name string) error {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return err
	}
	return c.Service.RestartService(req)
}

// Status returns the status of the named service.
func (c *Client) Status(name string) (ServiceStatus, error) {
	req, err := newServiceNameRequest(name)
	if err != nil {
		return ServiceStatus{}, err
	}
	return c.Service.GetStatus(req)
}

// List returns the status of all services.
func (c *Client) List() ([]ServiceStatus, error) {
	resp, err := c.Service.ListServices()
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// Poweroff requests a system power off.
func (c *Client) Poweroff() error {
	return c.Service.SystemPoweroff()
}

// Reboot requests a system reboot.
func (c *Client) Reboot() error {
	return c.Service.SystemReboot()
}

func newServiceNameRequest(name string) (ServiceNameRequest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ServiceNameRequest{}, fmt.Errorf("service name is required")
	}
	return ServiceNameRequest{Name: name}, nil
}
