package main

import (
	"fmt"

	"avyos.dev/api/service"
)

type Handler struct{}

func (h *Handler) StartService(_ uint32, req service.ServiceNameRequest) error {
	return fmt.Errorf("StartService(%s) not implemented", req.Name)
}

func (h *Handler) StopService(_ uint32, req service.ServiceNameRequest) error {
	return fmt.Errorf("StopService(%s) not implemented", req.Name)
}

func (h *Handler) RestartService(_ uint32, req service.ServiceNameRequest) error {
	return fmt.Errorf("RestartService(%s) not implemented", req.Name)
}

func (h *Handler) GetStatus(_ uint32, req service.ServiceNameRequest) (service.ServiceStatus, error) {
	return service.ServiceStatus{}, fmt.Errorf("GetStatus(%s) not implemented", req.Name)
}

func (h *Handler) ListServices(_ uint32) (service.ServiceStatusList, error) {
	return service.ServiceStatusList{}, fmt.Errorf("ListServices not implemented")
}

func (h *Handler) SystemPoweroff(_ uint32) error {
	return fmt.Errorf("SystemPoweroff not implemented")
}

func (h *Handler) SystemReboot(_ uint32) error {
	return fmt.Errorf("SystemReboot not implemented")
}
