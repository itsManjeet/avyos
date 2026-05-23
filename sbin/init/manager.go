/*
 * Copyright (c) 2026 Manjeet Singh <itsmanjeet1998@gmail.com>.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package main

import (
	"time"

	"avyos.dev/api/service"
)

type serviceManager struct{}

func (m *serviceManager) StartService(_ uint32, req service.ServiceNameRequest) error {
	return sv.StartService(req.Name)
}

func (m *serviceManager) StopService(_ uint32, req service.ServiceNameRequest) error {
	return sv.StopService(req.Name)
}

func (m *serviceManager) RestartService(_ uint32, req service.ServiceNameRequest) error {
	return sv.RestartService(req.Name)
}

func (m *serviceManager) GetStatus(_ uint32, req service.ServiceNameRequest) (service.ServiceStatus, error) {
	status, err := sv.GetServiceStatus(req.Name)
	if err != nil {
		return service.ServiceStatus{}, err
	}
	return service.ServiceStatus{
		Name:        status.Name,
		Description: status.Description,
		Type:        status.Type,
		Restart:     status.Restart,
		Running:     boolToUint8(status.Running),
		Started:     boolToUint8(status.Started),
		Failed:      boolToUint8(status.Failed),
		PID:         int32(status.PID),
	}, nil
}

func (m *serviceManager) ListServices(_ uint32) (service.ServiceStatusList, error) {
	items := sv.ListServices()
	out := make([]service.ServiceStatus, 0, len(items))
	for _, item := range items {
		out = append(out, service.ServiceStatus{
			Name:        item.Name,
			Description: item.Description,
			Type:        item.Type,
			Restart:     item.Restart,
			Running:     boolToUint8(item.Running),
			Started:     boolToUint8(item.Started),
			Failed:      boolToUint8(item.Failed),
			PID:         int32(item.PID),
		})
	}
	return service.ServiceStatusList{Items: out}, nil
}

func (m *serviceManager) SystemPoweroff(_ uint32) error {
	go func() {
		time.Sleep(200 * time.Millisecond)
		shutdownPoweroff()
	}()
	return nil
}

func (m *serviceManager) SystemReboot(_ uint32) error {
	go func() {
		time.Sleep(200 * time.Millisecond)
		shutdownReboot()
	}()
	return nil
}

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

var managerServer *service.Server

func setupServiceManager() {
	srv, err := service.Listen()
	if err != nil {
		log.Error("Failed to start service manager API: %v", err)
		return
	}
	srv.Handlers = service.Handlers{Service: &serviceManager{}}
	managerServer = srv
	log.Info("Service manager API ready at %s", service.ServiceName)
	go func() {
		if err := srv.Serve(); err != nil {
			log.Error("service manager serve error: %v", err)
		}
	}()
}
