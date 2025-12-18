/*
 * Copyright (c) 2025 Manjeet Singh <itsmanjeet1998@gmail.com>.
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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Supervisor struct {
	mutex          sync.Mutex
	services       []*Service
	wg             sync.WaitGroup
	isShuttingDown bool
	journal        *os.File
}

func (su *Supervisor) loadServices(path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		log.Printf("failed to read services path %s: %v", path, err)
		return err
	}
	for _, serviceFile := range files {
		if serviceFile.IsDir() || filepath.Ext(serviceFile.Name()) != ".service" {
			continue
		}

		var service *Service

		service, err = NewService(filepath.Join(path, serviceFile.Name()))
		if err != nil {
			log.Printf("failed to load service %s: %v", serviceFile.Name(), err)
			continue
		}

		if !service.isTemplate {
			su.services = append(su.services, service)
		}
	}
	return nil
}

func (su *Supervisor) get(id string) *Service {
	for _, service := range su.services {
		if service.Name == id {
			return service
		}
	}
	return nil
}

func (su *Supervisor) foreach(f func(s *Service)) {
	for _, service := range su.services {
		f(service)
	}
}
func (su *Supervisor) trigger(stage string) {
	var stageWaitGroup sync.WaitGroup

	su.foreach(func(s *Service) {
		if s.Stage != stage {
			return
		}

		stageWaitGroup.Add(1)
		go func(s *Service) {
			defer stageWaitGroup.Done()

			if err := su.waitForDepends(s); err != nil {
				log.Printf("dependencies not met for %s: %v", s.Name, err)
				s.State = Failed
				return
			}

			su.run(s)
		}(s) // Capture s properly
	})

	stageWaitGroup.Wait()
}

func (su *Supervisor) run(s *Service) {
	for {
		log.Printf("Starting service: %s", s.Name)

		if err := s.Start(su.journal); err != nil {
			log.Printf("failed to start %s: %v", s.Name, err)
			s.State = Failed
			if !s.Restart {
				return
			}
			time.Sleep(1 * time.Second)
			continue
		}

		s.State = Running

		if s.Kind == Oneshot {
			if s.Process == nil {
				log.Printf("oneshot service %s has no process", s.Name)
				s.State = Failed
				return
			}
			if _, err := s.Process.Wait(); err != nil {
				log.Printf("oneshot service %s exited with error: %v", s.Name, err)
				s.State = Failed
			} else {
				s.State = Finished
			}
			return
		}

		su.wg.Add(1)
		go su.monitor(s)

		break
	}
}

func (su *Supervisor) monitor(s *Service) {
	defer su.wg.Done()

	for {
		if s.Process == nil {
			log.Printf("daemon service %s has no process", s.Name)
			s.State = Failed
			break
		}

		// Wait for the process to finish
		if _, err := s.Process.Wait(); err != nil {
			log.Printf("daemon service %s exited with error: %v", s.Name, err)
			s.State = Failed
		} else {
			log.Printf("daemon service %s exited cleanly", s.Name)
			s.State = Finished
		}

		if su.isShuttingDown || !s.Restart {
			break
		}

		log.Printf("restarting daemon service %s", s.Name)
		time.Sleep(1 * time.Second)

		if err := s.Start(su.journal); err != nil {
			log.Printf("failed to restart %s: %v", s.Name, err)
			s.State = Failed
			break
		}

		s.State = Running
	}
}

func (su *Supervisor) waitForDepends(s *Service) error {
	if len(s.Depends) == 0 {
		return nil
	}

	var deps []*Service
	for _, name := range s.Depends {
		dep := su.get(name)
		if dep == nil {
			return fmt.Errorf("missing required dependency %s", name)
		}
		deps = append(deps, dep)
	}

	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)

	for {
		select {
		case <-timeout:
			var names []string
			for _, dep := range deps {
				names = append(names, dep.Name)
			}
			return fmt.Errorf("timeout waiting for dependencies: %v", names)

		case <-tick:
			var remaining []*Service
			for _, dep := range deps {
				switch dep.State {
				case Running, Finished:
					continue
				default:
					remaining = append(remaining, dep)
				}
			}

			if len(remaining) == 0 {
				return nil
			}

			deps = remaining
		}
	}
}
