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
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"avyos.dev/lib/ini"
	"avyos.dev/sbin/init/service"
	"avyos.dev/sbin/init/supervisor"
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
	config, err = ini.ParseFile("/etc/init.conf")
	if err != nil {
		config, err = ini.ParseFile("/usr/etc/init.conf")
		if err != nil {
			log.Error("Failed to parse init.conf: %v", err)
			config = defaultConfig()
		}
	}

	_ = syscall.Umask(022)

	// Setup environment variables
	config.ForEachKey("environment", func(key, value string) {
		if err := os.Setenv(key, value); err != nil {
			log.Error("Failed to set environment variable: %s=%v: %v", key, value, err)
		}
	})

	restore()

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
			Command:     "/usr/bin/sh",
			Type:        "daemon",
			Restart:     "always",
			TTY:         filepath.Join("/dev", tty),
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
	if err := os.WriteFile("/proc/sys/kernel/hostname", []byte(hostname), 0); err != nil {
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
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func defaultConfig() *ini.Config {
	config = ini.NewConfig()
	config.Set("", "shell", "/usr/bin/sh")
	config.Set("", "services", "shell")
	config.Set("", "hostname", "avyos")
	for key, value := range map[string]string{
		"PATH": "/bin:/sbin:/usr/bin:/usr/sbin",
	} {
		config.Set("environment", key, value)
	}
	return config
}

func setupCrashReporter() {
	helper := "/usr/sbin/coredump"
	pattern := fmt.Sprintf("|%s -pid %%p -signal %%s -time %%t -exec %%e -uid %%u -gid %%g -hostname %%h", helper)

	if err := os.WriteFile("/proc/sys/kernel/core_pattern", []byte(pattern), 0); err != nil {
		log.Error("Failed to set kernel core_pattern: %v", err)
	}
	if err := os.WriteFile("/proc/sys/kernel/core_pipe_limit", []byte("4"), 0); err != nil {
		log.Error("Failed to set kernel core_pipe_limit: %v", err)
	}
}

func restore() {
	files, ok := config.Get("restore", "files")
	if !ok {
		return
	}

	for _, file := range strings.Fields(files) {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			output, err := exec.Command("copy", "--recursive", filepath.Join("/usr", file), file).CombinedOutput()
			if err != nil {
				log.Error("failed to restore %s: %s %v", file, string(output), err)
			}
		}
	}
}
