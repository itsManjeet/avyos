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

package ini

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

type EntryType uint8

const (
	EntryBlank EntryType = iota
	EntryComment
	EntrySection
	EntryKeyValue
)

type Entry struct {
	Type      EntryType
	Section   string
	Key       string
	Value     string
	Raw       string
	Multiline bool
	Lines     []string
}

type Section struct {
	Name    string
	Entries []*Entry
}

type Config struct {
	Entries  []*Entry
	Sections map[string]*Section
}

func NewConfig() *Config {
	defaultSec := &Section{Name: ""}
	return &Config{
		Entries:  make([]*Entry, 0, 128),
		Sections: map[string]*Section{"": defaultSec},
	}
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return b
}

func hasContinuation(b []byte) bool {
	n := len(b)
	return n > 0 && b[n-1] == '\\'
}

func Parse(r io.Reader) (*Config, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	f := NewConfig()

	current := &Section{
		Name:    "",
		Entries: nil,
	}
	f.Sections[""] = current
	lineNo := 0

	for {
		line, err := br.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break
		}
		lineNo++

		// strip newline
		if n := len(line); n > 0 && line[n-1] == '\n' {
			line = line[:n-1]
			if n > 1 && line[n-2] == '\r' {
				line = line[:n-2]
			}
		}

		raw := string(line)
		s := trimSpace(line)

		// blank
		if len(s) == 0 {
			e := &Entry{Type: EntryBlank, Raw: raw}
			f.Entries = append(f.Entries, e)
			continue
		}

		// comment
		if s[0] == '#' || s[0] == ';' {
			e := &Entry{Type: EntryComment, Raw: raw}
			f.Entries = append(f.Entries, e)
			continue
		}

		// section
		if s[0] == '[' && s[len(s)-1] == ']' {
			name := string(trimSpace(s[1 : len(s)-1]))
			if name == "" {
				// [] is a legacy artefact of writing keys to the unnamed
				// default section; treat it as a reset to the default section.
				current = f.Sections[""]
				continue
			}

			sec := &Section{Name: name}
			f.Sections[name] = sec
			current = sec

			e := &Entry{
				Type:    EntrySection,
				Section: name,
				Raw:     raw,
			}
			f.Entries = append(f.Entries, e)
			continue
		}

		// key/value
		// key/value
		sep := bytes.IndexRune(s, '=')
		if sep < 0 {
			return nil, fmt.Errorf("invalid line %d", lineNo)
		}

		key := string(trimSpace(s[:sep]))
		valPart := trimSpace(s[sep+1:])

		lines := []string{string(valPart)}
		valueBuf := bytes.TrimSuffix(valPart, []byte{'\\'})

		multiline := hasContinuation(valPart)

		for multiline {
			next, err := br.ReadBytes('\n')
			if len(next) == 0 && err != nil {
				return nil, fmt.Errorf("unterminated multiline value at line %d", lineNo)
			}
			lineNo++

			// strip newline
			if n := len(next); n > 0 && next[n-1] == '\n' {
				next = next[:n-1]
				if n > 1 && next[n-2] == '\r' {
					next = next[:n-2]
				}
			}

			rawLine := trimSpace(next)
			lines = append(lines, string(rawLine))

			if hasContinuation(rawLine) {
				rawLine = rawLine[:len(rawLine)-1]
				valueBuf = append(valueBuf, '\n')
				valueBuf = append(valueBuf, rawLine...)
				continue
			}

			valueBuf = append(valueBuf, '\n')
			valueBuf = append(valueBuf, rawLine...)
			multiline = false
		}

		e := &Entry{
			Type:      EntryKeyValue,
			Section:   "",
			Key:       key,
			Value:     string(valueBuf),
			Raw:       raw,
			Multiline: len(lines) > 1,
			Lines:     lines,
		}

		if current != nil {
			e.Section = current.Name
			current.Entries = append(current.Entries, e)
		}

		f.Entries = append(f.Entries, e)

		if err != nil {
			break
		}
	}

	return f, nil
}

func ParseFile(filepath string) (*Config, error) {
	file, err := os.OpenFile(filepath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Parse(file)
}

func (c *Config) Get(section, key string) (string, bool) {
	s, ok := c.Sections[section]
	if !ok {
		return "", false
	}
	for _, e := range s.Entries {
		if e.Type == EntryKeyValue && e.Key == key {
			return e.Value, true
		}
	}
	return "", false
}

func (c *Config) Set(section, key, value string) {
	// update existing
	if e, _ := c.findKey(section, key); e != nil {
		e.Value = value
		return
	}

	// ensure section exists
	sec, ok := c.Sections[section]
	if !ok {
		sec = &Section{Name: section}
		c.Sections[section] = sec

		// append new section header
		se := &Entry{
			Type:    EntrySection,
			Section: section,
			Raw:     "[" + section + "]",
		}
		c.Entries = append(c.Entries, se)
	}

	// create key entry
	ke := &Entry{
		Type:    EntryKeyValue,
		Section: section,
		Key:     key,
		Value:   value,
	}

	sec.Entries = append(sec.Entries, ke)

	// insert after last entry of section
	insertAt := c.findSectionIndex(section) + 1
	for insertAt < len(c.Entries) &&
		c.Entries[insertAt].Type != EntrySection {
		insertAt++
	}

	c.Entries = append(
		c.Entries[:insertAt],
		append([]*Entry{ke}, c.Entries[insertAt:]...)...,
	)
}

func (c *Config) Delete(section, key string) bool {
	e, idx := c.findKey(section, key)
	if e == nil {
		return false
	}

	// remove from file entries
	c.Entries = append(c.Entries[:idx], c.Entries[idx+1:]...)

	// remove from section
	sec := c.Sections[section]
	for i, se := range sec.Entries {
		if se == e {
			sec.Entries = append(sec.Entries[:i], sec.Entries[i+1:]...)
			break
		}
	}

	return true
}

func (c *Config) Move(section, key string, newIndex int) bool {
	sec, ok := c.Sections[section]
	if !ok {
		return false
	}

	var e *Entry
	var old int

	for i, x := range sec.Entries {
		if x.Type == EntryKeyValue && x.Key == key {
			e = x
			old = i
			break
		}
	}
	if e == nil || newIndex < 0 || newIndex >= len(sec.Entries) {
		return false
	}

	// reorder section entries
	sec.Entries = append(sec.Entries[:old], sec.Entries[old+1:]...)
	sec.Entries = append(
		sec.Entries[:newIndex],
		append([]*Entry{e}, sec.Entries[newIndex:]...)...,
	)

	// rebuild file entry ordering for that section
	start := c.findSectionIndex(section) + 1
	end := start
	for end < len(c.Entries) &&
		c.Entries[end].Type != EntrySection {
		end++
	}

	var rebuilt []*Entry
	for _, ent := range c.Entries[start:end] {
		if ent.Type != EntryKeyValue {
			rebuilt = append(rebuilt, ent)
		}
	}

	// reinsert keys in new order
	for _, ke := range sec.Entries {
		rebuilt = append(rebuilt, ke)
	}

	copy(c.Entries[start:end], rebuilt)
	return true
}

func (c *Config) Write(w io.Writer) error {
	for _, e := range c.Entries {
		switch e.Type {

		case EntryBlank, EntryComment, EntrySection:
			_, _ = w.Write([]byte(e.Raw + "\n"))

		case EntryKeyValue:
			if !e.Multiline {
				_, _ = w.Write([]byte(e.Key + " = " + e.Value + "\n"))
				continue
			}

			// multiline rewrite
			for i, line := range e.Lines {
				if i == 0 {
					_, _ = w.Write([]byte(e.Key + " = " + line + " \\\n"))
				} else if i < len(e.Lines)-1 {
					_, _ = w.Write([]byte("    " + line + " \\\n"))
				} else {
					_, _ = w.Write([]byte("    " + line + "\n"))
				}
			}
		}
	}
	return nil
}

func (c *Config) ForEachKey(section string, fn func(key, value string)) {
	sec, ok := c.Sections[section]
	if !ok {
		return
	}

	for _, e := range sec.Entries {
		if e.Type == EntryKeyValue {
			fn(e.Key, e.Value)
		}
	}
}

func (c *Config) findSectionIndex(name string) int {
	for i, e := range c.Entries {
		if e.Type == EntrySection && e.Section == name {
			return i
		}
	}
	return -1
}

func (c *Config) findKey(section, key string) (*Entry, int) {
	for i, e := range c.Entries {
		if e.Type == EntryKeyValue &&
			e.Section == section &&
			e.Key == key {
			return e, i
		}
	}
	return nil, -1
}
