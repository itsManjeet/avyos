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

package supervisor

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"syscall"
	"time"

	"avyos.dev/sbin/init/service"
	"avyos.dev/lib/fs"
	"avyos.dev/lib/logger"
)

var log = logger.New("supervisor")

// ProcessExit contains information about an exited process
type ProcessExit struct {
	PID    int
	Status syscall.WaitStatus
}

type Supervisor struct {
	services []*service.Service
	mutex    sync.Mutex
	running  bool
	exitChan chan ProcessExit
}

// NewSupervisor creates a new supervisor instance
func NewSupervisor() *Supervisor {
	return &Supervisor{
		services: make([]*service.Service, 0),
		exitChan: make(chan ProcessExit, 10), // Buffered channel for process exits
	}
}

// ExitChannel returns the channel for receiving process exit information
func (sv *Supervisor) ExitChannel() chan<- ProcessExit {
	return sv.exitChan
}

func (sv *Supervisor) LoadService(name string) error {
	path := fmt.Sprintf("/etc/init.d/%s", name)
	if !fs.Exists(path) {
		return ErrNotFound
	}

	s, err := service.Parse(path)
	if err != nil {
		return err
	}

	sv.mutex.Lock()
	sv.services = append(sv.services, s)
	sv.mutex.Unlock()

	return nil
}

func (sv *Supervisor) LoadServices(names ...string) []error {
	var errs []error
	for _, name := range names {
		if err := sv.LoadService(name); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	return errs
}

func (sv *Supervisor) LoadInternalService(s *service.Service) {
	sv.mutex.Lock()
	sv.services = append(sv.services, s)
	sv.mutex.Unlock()
}

func (sv *Supervisor) Stop() {
	sv.mutex.Lock()
	if !sv.running {
		sv.mutex.Unlock()
		return
	}
	sv.running = false
	services := slices.Clone(sv.services)
	sv.mutex.Unlock()

	slices.Reverse(services)

	for _, s := range services {
		if s.Process != nil {
			log.Info("Terminating service: %s", s.Name)
			_ = s.Process.Signal(syscall.SIGTERM)
		}
	}

	time.Sleep(2 * time.Second)

	for _, s := range services {
		if s.Process != nil {
			log.Warn("Killing service: %s", s.Name)
			_ = s.Process.Signal(syscall.SIGKILL)
		}
	}
}

func (sv *Supervisor) StartAll() {
	sv.mutex.Lock()
	services := slices.Clone(sv.services)
	sv.mutex.Unlock()

	for _, s := range services {
		if s.Started || s.Failed {
			continue
		}

		if s.Description != "" {
			log.Info("Starting %s (%s)", s.Name, s.Description)
		} else {
			log.Info("Starting %s", s.Name)
		}

		if err := s.Start(); err != nil {
			log.Error("Failed to start %s: %v", s.Name, err)
			sv.mutex.Lock()
			s.Failed = true
			sv.mutex.Unlock()
			continue
		}

		sv.mutex.Lock()
		s.Started = true
		sv.mutex.Unlock()
	}
}

func (sv *Supervisor) Supervise() {
	sv.mutex.Lock()
	if sv.running {
		sv.mutex.Unlock()
		return
	}
	sv.running = true
	sv.mutex.Unlock()

	log.Debug("Entering supervisor loop")

	for {
		select {
		case exit := <-sv.exitChan:
			log.Debug("Received exit notification for PID %d", exit.PID)
			// Process was already reaped by reapZombies, just handle restart
			sv.handleExit(exit.PID, exit.Status)
		}

		// Check if we should stop
		sv.mutex.Lock()
		if !sv.running {
			sv.mutex.Unlock()
			log.Debug("Supervisor stopped")
			return
		}
		sv.mutex.Unlock()
	}
}

const maxRetries = 10

func (sv *Supervisor) handleExit(pid int, ws syscall.WaitStatus) {
	sv.mutex.Lock()

	var svc *service.Service
	for _, s := range sv.services {
		if s.Process != nil && s.Process.Pid == pid {
			svc = s
			break
		}
	}

	if svc == nil {
		sv.mutex.Unlock()
		return
	}

	svc.Process = nil
	svc.Started = false

	exitCode := 0
	if ws.Exited() {
		exitCode = ws.ExitStatus()
	} else {
		exitCode = -1
	}

	// Reset retries on clean exit
	if exitCode == 0 {
		svc.Retries = 0
	}

	shouldRestart :=
		svc.Restart == "always" ||
			(svc.Restart == "on-failure" && exitCode != 0)

	if !shouldRestart {
		svc.Failed = exitCode != 0
		sv.mutex.Unlock()
		return
	}

	// Check max retries for failure exits
	if exitCode != 0 {
		svc.Retries++
		if svc.Retries > maxRetries {
			log.Error("Service %s exceeded max retries (%d), marking as failed", svc.Name, maxRetries)
			svc.Failed = true
			sv.mutex.Unlock()
			return
		}
	}

	if svc.Description != "" {
		log.Info("Restarting %s (%s) after exit code %d (retry %d/%d)", svc.Name, svc.Description, exitCode, svc.Retries, maxRetries)
	} else {
		log.Info("Restarting %s after exit code %d (retry %d/%d)", svc.Name, exitCode, svc.Retries, maxRetries)
	}

	sv.mutex.Unlock()

	// Delay before restart
	time.Sleep(1 * time.Second)

	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	// Check if service was stopped while we were waiting
	if svc.Failed || svc.Process != nil {
		return
	}

	if err := svc.Start(); err != nil {
		log.Error("Failed to restart %s: %v", svc.Name, err)
		svc.Failed = true
	} else {
		svc.Started = true
	}
}

// ServiceStatus represents the current status of a service
type ServiceStatus struct {
	Name        string
	Description string
	Type        string
	Restart     string
	Running     bool
	Started     bool
	Failed      bool
	PID         int
}

// StartService starts a specific service by name
func (sv *Supervisor) StartService(name string) error {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	for _, s := range sv.services {
		if s.Name == name {
			if s.Started && s.Process != nil {
				return fmt.Errorf("service %s is already running", name)
			}

			log.Info("Starting service: %s", name)
			if err := s.Start(); err != nil {
				log.Error("Failed to start %s: %v", name, err)
				s.Failed = true
				return err
			}

			s.Started = true
			s.Failed = false
			return nil
		}
	}

	return ErrNotFound
}

// StopService stops a specific service by name
func (sv *Supervisor) StopService(name string) error {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	for _, s := range sv.services {
		if s.Name == name {
			if s.Process == nil {
				return fmt.Errorf("service %s is not running", name)
			}

			log.Info("Stopping service: %s", name)
			if err := s.Process.Signal(syscall.SIGTERM); err != nil {
				log.Error("Failed to stop %s: %v", name, err)
				return err
			}

			// Give it time to gracefully shutdown
			time.AfterFunc(2*time.Second, func() {
				sv.mutex.Lock()
				defer sv.mutex.Unlock()
				if s.Process != nil {
					log.Warn("Forcefully killing service: %s", name)
					_ = s.Process.Signal(syscall.SIGKILL)
				}
			})

			s.Started = false
			return nil
		}
	}

	return ErrNotFound
}

// RestartService restarts a specific service by name
func (sv *Supervisor) RestartService(name string) error {
	log.Info("Restarting service: %s", name)

	// Stop the service if running
	if err := sv.StopService(name); err != nil && err != ErrNotFound {
		// If service is not running, that's okay, just start it
		if err.Error() != fmt.Sprintf("service %s is not running", name) {
			return err
		}
	}

	// Wait a moment for the service to stop
	time.Sleep(500 * time.Millisecond)

	// Start the service
	return sv.StartService(name)
}

// ServiceStatus returns the status of a specific service
func (sv *Supervisor) GetServiceStatus(name string) (*ServiceStatus, error) {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	for _, s := range sv.services {
		if s.Name == name {
			status := &ServiceStatus{
				Name:        s.Name,
				Description: s.Description,
				Type:        s.Type,
				Restart:     s.Restart,
				Running:     s.Process != nil,
				Started:     s.Started,
				Failed:      s.Failed,
			}

			if s.Process != nil {
				status.PID = s.Process.Pid
			}

			return status, nil
		}
	}

	return nil, ErrNotFound
}

// ListServices returns the status of all services
func (sv *Supervisor) ListServices() []ServiceStatus {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	statuses := make([]ServiceStatus, 0, len(sv.services))
	for _, s := range sv.services {
		status := ServiceStatus{
			Name:        s.Name,
			Description: s.Description,
			Type:        s.Type,
			Restart:     s.Restart,
			Running:     s.Process != nil,
			Started:     s.Started,
			Failed:      s.Failed,
		}

		if s.Process != nil {
			status.PID = s.Process.Pid
		}

		statuses = append(statuses, status)
	}

	return statuses
}

var ErrNotFound = errors.New("service not found")
