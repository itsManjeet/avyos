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
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"avyos.dev/lib/shadow"
)

type loginAccountSpec struct {
	Username string
	FullName string
	Groups   []string
	Home     string
	Shell    string
}

type passwdRecord struct {
	Name   string
	Passwd string
	UID    int
	GID    int
	GECOS  string
	Home   string
	Shell  string
}

type groupRecord struct {
	Name    string
	Passwd  string
	GID     int
	Members []string
}

type shadowRecord struct {
	Name   string
	Hash   string
	Fields []string
}

func addLoginAccount(spec loginAccountSpec) error {
	username := strings.TrimSpace(spec.Username)
	if username == "" {
		return fmt.Errorf("username is required")
	}

	if _, err := user.Lookup(username); err == nil {
		return fmt.Errorf("username %q already exists", username)
	}

	passwd, err := readPasswdRecords()
	if err != nil {
		passwd = nil
	}
	uid := nextUID(passwd)
	gid := groupIDByName("users")
	if gid < 0 {
		gid = 100
	}
	home := strings.TrimSpace(spec.Home)
	if home == "" {
		home = filepath.Join("/home", username)
	}
	shell := shellOrDefault(spec.Shell)

	passwd = append(passwd, passwdRecord{
		Name:   username,
		Passwd: "x",
		UID:    uid,
		GID:    gid,
		GECOS:  spec.FullName,
		Home:   home,
		Shell:  shell,
	})
	if err := writePasswdRecords(passwd); err != nil {
		return err
	}
	if err := ensureUserGroups(username, gid, spec.Groups); err != nil {
		return err
	}
	return ensureInitialShadowLock(username)
}

func updateLoginPassword(username, password string) error {
	records, err := readShadowRecords()
	if err != nil {
		records = nil
	}
	idx := -1
	for i := range records {
		if records[i].Name == username {
			idx = i
			break
		}
	}

	record := shadowRecord{Name: username, Hash: shadow.Hash("sha512", password), Fields: defaultShadowFields()}
	if idx >= 0 {
		hash := records[idx].Hash
		if hash != "" && hash != "!" && hash != "*" && (strings.HasPrefix(hash, "!") || strings.HasPrefix(hash, "*")) {
			return fmt.Errorf("account is locked")
		}
		if len(records[idx].Fields) > 0 {
			record.Fields = records[idx].Fields
		}
		records[idx] = record
	} else {
		records = append(records, record)
	}
	return writeShadowRecords(records)
}

func ensureInitialShadowLock(username string) error {
	shadow, err := readShadowRecords()
	if err != nil {
		shadow = nil
	}
	for _, record := range shadow {
		if record.Name == username {
			return nil
		}
	}
	shadow = append(shadow, shadowRecord{Name: username, Hash: "!", Fields: defaultShadowFields()})
	return writeShadowRecords(shadow)
}

func readPasswdRecords() ([]passwdRecord, error) {
	lines, err := readRecords("/etc/passwd")
	if err != nil {
		return nil, err
	}
	records := make([]passwdRecord, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}
		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		gid, err := strconv.Atoi(fields[3])
		if err != nil {
			continue
		}
		records = append(records, passwdRecord{
			Name:   fields[0],
			Passwd: fields[1],
			UID:    uid,
			GID:    gid,
			GECOS:  fields[4],
			Home:   fields[5],
			Shell:  fields[6],
		})
	}
	return records, nil
}

func writePasswdRecords(records []passwdRecord) error {
	var b strings.Builder
	for _, r := range records {
		passwd := r.Passwd
		if passwd == "" {
			passwd = "x"
		}
		fmt.Fprintf(&b, "%s:%s:%d:%d:%s:%s:%s\n", r.Name, passwd, r.UID, r.GID, r.GECOS, r.Home, shellOrDefault(r.Shell))
	}
	return writeEtcFile("passwd", []byte(b.String()), 0644)
}

func readGroupRecords() ([]groupRecord, error) {
	lines, err := readRecords("/etc/group")
	if err != nil {
		return nil, err
	}
	records := make([]groupRecord, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 4 {
			continue
		}
		gid, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		records = append(records, groupRecord{
			Name:    fields[0],
			Passwd:  fields[1],
			GID:     gid,
			Members: splitList(fields[3]),
		})
	}
	return records, nil
}

func writeGroupRecords(records []groupRecord) error {
	var b strings.Builder
	for _, r := range records {
		passwd := r.Passwd
		if passwd == "" {
			passwd = "x"
		}
		fmt.Fprintf(&b, "%s:%s:%d:%s\n", r.Name, passwd, r.GID, strings.Join(uniqueStrings(r.Members), ","))
	}
	return writeEtcFile("group", []byte(b.String()), 0644)
}

func readShadowRecords() ([]shadowRecord, error) {
	lines, err := readRecords("/etc/shadow")
	if err != nil {
		return nil, err
	}
	records := make([]shadowRecord, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		records = append(records, shadowRecord{Name: fields[0], Hash: fields[1], Fields: append([]string{}, fields[2:]...)})
	}
	return records, nil
}

func writeShadowRecords(records []shadowRecord) error {
	var b strings.Builder
	for _, r := range records {
		fields := r.Fields
		if len(fields) == 0 {
			fields = defaultShadowFields()
		}
		fmt.Fprintf(&b, "%s:%s:%s\n", r.Name, r.Hash, strings.Join(fields, ":"))
	}
	return writeEtcFile("shadow", []byte(b.String()), 0600)
}

func readRecords(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		records = append(records, line)
	}
	return records, scanner.Err()
}

func writeEtcFile(name string, data []byte, perm os.FileMode) error {
	path := filepath.Join("/etc", name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func ensureUserGroups(username string, primaryGID int, names []string) error {
	groups, err := readGroupRecords()
	if err != nil {
		groups = defaultGroupRecords()
	}
	if len(names) == 0 {
		names = []string{"users"}
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		idx := -1
		for i := range groups {
			if groups[i].Name == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			groups = append(groups, groupRecord{Name: name, Passwd: "x", GID: nextGroupID(groups)})
			idx = len(groups) - 1
		}
		if groups[idx].GID != primaryGID && !containsString(groups[idx].Members, username) {
			groups[idx].Members = append(groups[idx].Members, username)
		}
	}
	return writeGroupRecords(groups)
}

func groupIDByName(name string) int {
	group, err := user.LookupGroup(name)
	if err == nil {
		gid, err := strconv.Atoi(group.Gid)
		if err == nil {
			return gid
		}
	}
	groups, err := readGroupRecords()
	if err != nil {
		return -1
	}
	for _, record := range groups {
		if record.Name == name {
			return record.GID
		}
	}
	return -1
}

func nextUID(records []passwdRecord) int {
	used := map[int]struct{}{}
	for _, record := range records {
		used[record.UID] = struct{}{}
	}
	for uid := 10000; ; uid++ {
		if _, ok := used[uid]; !ok {
			return uid
		}
	}
}

func nextGroupID(records []groupRecord) int {
	used := map[int]struct{}{}
	for _, record := range records {
		used[record.GID] = struct{}{}
	}
	for gid := 100; ; gid++ {
		if _, ok := used[gid]; !ok {
			return gid
		}
	}
}

func defaultGroupRecords() []groupRecord {
	return []groupRecord{
		{Name: "root", Passwd: "x", GID: 0},
		{Name: "wheel", Passwd: "x", GID: 10},
		{Name: "users", Passwd: "x", GID: 100},
	}
}

func firstGECOSField(value string) string {
	first, _, _ := strings.Cut(value, ",")
	return strings.TrimSpace(first)
}

func shellOrDefault(shell string) string {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return defaultLoginShell()
	}
	return shell
}

func defaultLoginShell() string {
	return "/usr/bin/sh"
}

func defaultShadowFields() []string {
	return []string{"", "", "", "", "", "", ""}
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
