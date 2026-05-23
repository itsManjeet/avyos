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
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
)

type loginUser struct {
	Username    string
	DisplayName string
	UID         int
	GID         int
	HomeDir     string
	Shell       string
	Groups      []int
}

func lookupLoginUser(username string) (*loginUser, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}
	return loginUserFromUser(u)
}

func listLoginUsers() ([]*loginUser, error) {
	candidates := map[string]struct{}{}

	entries, err := os.ReadDir("/home")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			if name != "" {
				candidates[name] = struct{}{}
			}
		}
	}

	if current, err := user.Current(); err == nil && strings.TrimSpace(current.Username) != "" {
		candidates[current.Username] = struct{}{}
	}

	names := make([]string, 0, len(candidates))
	for name := range candidates {
		names = append(names, name)
	}
	sort.Strings(names)

	users := make([]*loginUser, 0, len(names))
	for _, name := range names {
		u, err := user.Lookup(name)
		if err != nil {
			continue
		}
		lu, err := loginUserFromUser(u)
		if err != nil {
			continue
		}
		users = append(users, lu)
	}
	return users, nil
}

func loginUserFromUser(u *user.User) (*loginUser, error) {
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, err
	}

	groups := []int{gid}
	if raw, err := u.GroupIds(); err == nil {
		for _, value := range raw {
			groupID, err := strconv.Atoi(value)
			if err == nil && !containsInt(groups, groupID) {
				groups = append(groups, groupID)
			}
		}
	}

	return &loginUser{
		Username:    u.Username,
		DisplayName: strings.TrimSpace(u.Name),
		UID:         uid,
		GID:         gid,
		HomeDir:     u.HomeDir,
		Shell:       shellOrDefault(""),
		Groups:      groups,
	}, nil
}

func containsInt(values []int, value int) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
