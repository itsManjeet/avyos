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
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	JournalPath  = "/cache/log/journal"
	ServicesPath = "/config/services.d"
)

var (
	stages = []string{"pre-init", "init", "post-init"}
)

func startup() error {
	_ = os.MkdirAll(filepath.Dir(JournalPath), 0755)

	if _, err := os.Stat(JournalPath); err == nil {
		if err := os.Rename(JournalPath, JournalPath+".old"); err != nil {
			log.Printf("failed to replace older journal %v", err)
		}
	}

	ensureRequiredDirs()

	var err error
	journal, err = os.OpenFile(JournalPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		journal = os.Stdout
	} else {
		log.SetOutput(journal)
	}

	loadServices(ServicesPath)

	for _, stage := range stages {
		triggerStage(stage)
	}

	triggerStage("service")

	waitGroup.Wait()
	return nil
}

func ensureRequiredDirs() {
	const (
		USERNAME_FIELD int = iota
		X_FIELD
		UID_FIELD
		GID_FIELD
		COMMENT_FIELD
		HOME_DIR_FIELD
		SHELL_FIELD
	)

	file, err := os.OpenFile("/etc/passwd", os.O_RDONLY, 0)
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			user := strings.Split(line, ":")
			if len(user) != 7 {
				continue
			}
			username := user[USERNAME_FIELD]
			homedir := user[HOME_DIR_FIELD]
			// shell := user[SHELL_FIELD]
			uid, err := strconv.Atoi(user[UID_FIELD])
			if err != nil {
				log.Printf("invalid user id for %s, %s", username, user[UID_FIELD])
				continue
			}

			gid, err := strconv.Atoi(user[GID_FIELD])
			if err != nil {
				log.Printf("invalid groupd id for %s, %s", username, user[GID_FIELD])
				continue
			}

			if _, err := os.Stat(homedir); err != nil {
				if err := os.MkdirAll(filepath.Dir(homedir), 0755); err != nil {
					log.Printf("failed to create homedir %s for %s %v", homedir, user, err)
					continue
				}

				if err := os.Mkdir(homedir, 0750); err != nil {
					log.Printf("failed to create homedir %s for %s %v", homedir, user, err)
					continue
				}

				if err := os.Chown(homedir, uid, gid); err != nil {
					log.Printf("failed to chown homedir %s for %s %v", homedir, user, err)
					continue
				}
			}
		}
	}
	defer file.Close()

	for _, dir := range []string{"services", "log"} {
		_ = os.MkdirAll(filepath.Join("/cache", dir), 0755)
	}
}
