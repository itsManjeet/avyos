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
	"net"
	"os"
	"path/filepath"
	"time"

	"avyos.dev/pkg/connect"
)

const (
	JournalPath  = "/cache/log/journal"
	ServicesPath = "/config/services.d"

	SocketDelay = 10
	SocketRetry = 10
)

var (
	stages = []string{"pre-init", "init", "post-init"}
	su     Supervisor
)

func startup() error {
	_ = os.MkdirAll(filepath.Dir(JournalPath), 0755)

	if _, err := os.Stat(JournalPath); err == nil {
		if err := os.Rename(JournalPath, JournalPath+".old"); err != nil {
			log.Printf("failed to replace older journal %v", err)
		}
	}

	var err error
	su.journal, err = os.OpenFile(JournalPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		su.journal = os.Stdout
	} else {
		log.SetOutput(su.journal)
	}

	su.loadServices(ServicesPath)

	connectService := su.get("connect")
	if connectService != nil {
		su.run(connectService)
	}

	waitForSocket()

	go func() {
		for {
			if err := startInitServer(&Init{su: &su}); err != nil {
				if su.isShuttingDown {
					return
				}
				fmt.Printf("[INIT]: failed to connect to system bus = %v\n", err)
				time.Sleep(time.Millisecond * SocketDelay)
			}
		}
	}()

	for _, stage := range stages {
		su.trigger(stage)
	}

	su.trigger("service")

	su.wg.Wait()
	return nil
}

func waitForSocket() {
	for i := 0; i < SocketRetry; i++ {
		c, err := net.Dial("unix", connect.SystemBusPath)
		if err == nil {
			c.Close()
			log.Println("[INIT]: bus socket is ready")
			return
		}
		time.Sleep(SocketDelay * time.Millisecond)
	}
}
