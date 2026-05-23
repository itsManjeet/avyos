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
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"avyos.dev/cmd/init/service"
	"avyos.dev/cmd/init/supervisor"
	"avyos.dev/pkg/fs"
	"avyos.dev/pkg/ini"
)

var (
	config *ini.Config
	sv     *supervisor.Supervisor
)

func startup() {
	// Create supervisor
	sv = supervisor.NewSupervisor()

	// Setup signal handlers with supervisor's exit channel
	setupSignalHandlers(sv.ExitChannel())

	var err error
	config, err = ini.ParseFile(fs.Resolve("config:init.conf"))
	if err != nil {
		config = defaultConfig()
	}

	_ = syscall.Umask(022)

	// Setup environment variables
	config.ForEachKey("environment", func(key, value string) {
		if err := os.Setenv(key, value); err != nil {
			log.Error("Failed to set environment variable: %s=%v: %v", key, value, err)
		}
	})

	if value, ok := config.Get("rescue", "enable"); ok && value == "true" {
		tty, ok := config.Get("rescue", "tty")
		if !ok || tty == "" {
			switch runtime.GOARCH {
			case "amd64":
				tty = "ttyS0"
			case "arm64":
				tty = "ttyAMA0"
			}
		}
		sv.LoadInternalService(&service.Service{
			Name:        "Rescue Shell",
			Description: "Rescue shell to debug avyos",
			Command:     "/avyos/cmd/shell",
			Type:        "daemon",
			Restart:     "always",
			TTY:         fs.Resolve("device:%s", tty),
		})
	}

	// Load enabled services
	services, ok := config.Get("", "services")
	if ok {
		errs := sv.LoadServices(strings.Fields(services)...)
		for _, err := range errs {
			log.Error("Failed to load service: %v", err)
		}
	}

	// Setup hostname
	hostname, ok := config.Get("", "hostname")
	if !ok {
		hostname = "avyos"
	}
	if err := os.WriteFile(fs.Resolve("process:sys/kernel/hostname"), []byte(hostname), 0); err != nil {
		log.Error("Failed to set hostname: %v", err)
	}

	setupCrashReporter()

	// Setup network devices
	if devices, ok := config.Get("network", "devices"); ok {
		for dev := range strings.FieldsSeq(devices) {
			if err := run("net", "start", dev); err != nil {
				log.Error("Failed to activate network device %s: %v", dev, err)
				continue
			}
			if ip, ok := config.Get("network."+dev, "ip"); ok {
				if err := run("net", "assign", dev, ip); err != nil {
					log.Error("Failed to assign ip network device %s: %v", dev, err)
					continue
				}
			}
			if route, ok := config.Get("network."+dev, "route"); ok {
				if err := run("net", "route", dev, route); err != nil {
					log.Error("Failed to set route network device %s: %v", dev, err)
					continue
				}
			}
		}
	}

	setupServiceManager()

	log.Info("Starting all services")
	sv.StartAll()

	log.Info("Starting supervisor for services: %s", services)
	sv.Supervise()

	if isShutdownRequested() {
		log.Info("Shutdown in progress; keeping PID 1 alive until reboot completes")
		for {
			syscall.Pause()
		}
	}
}

func run(bin string, args ...string) error {
	null, err := os.Open(fs.Resolve("device:null"))
	if err != nil {
		return err
	}
	defer null.Close()

	cmd := exec.Command(bin, args...)
	cmd.Stdin = null
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func defaultConfig() *ini.Config {
	config = ini.NewConfig()
	config.Set("", "shell", "/avyos/cmd/shell")
	config.Set("", "services", "shell")
	config.Set("", "hostname", "avyos")
	for key, value := range map[string]string{
		"PATH": "/cmd:/avyos/cmd",
	} {
		config.Set("environment", key, value)
	}
	return config
}

func setupCrashReporter() {
	helper := fs.Resolve("cmd:coredump")
	pattern := fmt.Sprintf("|%s -pid %%p -signal %%s -time %%t -exec %%e -uid %%u -gid %%g -hostname %%h", helper)

	if err := os.WriteFile(fs.Resolve("process:sys/kernel/core_pattern"), []byte(pattern), 0); err != nil {
		log.Error("Failed to set kernel core_pattern: %v", err)
	}
	if err := os.WriteFile(fs.Resolve("process:sys/kernel/core_pipe_limit"), []byte("4"), 0); err != nil {
		log.Error("Failed to set kernel core_pipe_limit: %v", err)
	}
}
