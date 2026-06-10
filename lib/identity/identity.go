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

package identity

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// LookupByID finds an identity by their numeric ID
func LookupByID(uid int) (*Identity, error) {
	config, err := LoadIdentityConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Identities {
		if config.Identities[i].ID == uid {
			identity := &config.Identities[i]
			identity.setDefaults()
			return identity, nil
		}
	}

	return nil, ErrUserNotFound
}

// LookupByName finds an identity by their username
func LookupByName(name string) (*Identity, error) {
	config, err := LoadIdentityConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Identities {
		if config.Identities[i].Name == name {
			identity := &config.Identities[i]
			identity.setDefaults()
			return identity, nil
		}
	}

	return nil, ErrUserNotFound
}

// ListIdentities returns all identities in the system
func ListIdentities() ([]*Identity, error) {
	config, err := LoadIdentityConfig()
	if err != nil {
		return nil, err
	}

	identities := make([]*Identity, len(config.Identities))
	for i := range config.Identities {
		identity := &config.Identities[i]
		identity.setDefaults()
		identities[i] = identity
	}

	return identities, nil
}

// setDefaults sets default values for Home and Shell if not specified
func (i *Identity) setDefaults() {
	if i.Home == "" {
		if i.ID == 0 {
			i.Home = "/root"
		} else {
			i.Home = filepath.Join("/home/", i.Name)
		}
	}
	if i.Shell == "" {
		i.Shell = "/bin/sh"
	}
}

// HasCapability checks if an identity has a specific capability
func (i *Identity) HasCapability(cap string) bool {
	return slices.Contains(i.Capabilities, cap)
}

// InGroup checks if an identity is in a specific Unix group (via unix: capability)
func (i *Identity) InGroup(groupName string) bool {
	return i.HasCapability("unix:" + groupName)
}

// GetGroups returns all Unix groups for this identity
func (i *Identity) GetGroups() ([]*Capability, error) {
	groups := make([]*Capability, 0)

	for _, cap := range i.Capabilities {
		if after, ok := strings.CutPrefix(cap, "unix:"); ok {
			groupName := after
			group, err := LookupCapabilityByName(groupName)
			if err == nil {
				groups = append(groups, group)
			}
		}
	}

	return groups, nil
}

// GetGroupIDs returns all group IDs for the identity
func (i *Identity) GetGroupIDs() []int {
	groups, _ := i.GetGroups()
	ids := make([]int, len(groups))
	for idx, g := range groups {
		ids[idx] = g.ID
	}
	return ids
}

// LoadIdentityConfig reads and parses the identity config file
func LoadIdentityConfig() (*IdentityConfig, error) {
	data, err := os.ReadFile("/etc/security/identity.conf")
	if err != nil {
		return nil, err
	}

	var config IdentityConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// AddIdentity register new identity into system config
func AddIdentity(id Identity, kind string) error {
	idConfig, err := LoadIdentityConfig()
	if err != nil {
		return err
	}

	// TODO: verify identity
	id.ID, err = GetNextAvailableId(kind)
	if err != nil {
		return err
	}
	if id.Name == "" {
		return ErrInvalidCredentials
	}

	idConfig.Identities = append(idConfig.Identities, id)
	data, err := json.Marshal(idConfig)
	if err != nil {
		return err
	}
	return os.WriteFile("/config/security/identity.conf", data, 0644)
}

func GetNextAvailableId(kind string) (int, error) {
	config, err := LoadIdentityConfig()
	if err != nil {
		return 0, err
	}

	var minID, maxID int

	switch kind {
	case "system":
		minID, maxID = 0, 999
	case "service":
		minID, maxID = 1000, 9999
	case "user":
		minID, maxID = 10000, math.MaxInt
	default:
		return 0, ErrInvalidIdentityKind
	}

	used := make(map[int]struct{})
	for _, id := range config.Identities {
		used[id.ID] = struct{}{}
	}

	for id := minID; id <= maxID; id++ {
		if _, exists := used[id]; !exists {
			return id, nil
		}
	}

	return 0, ErrNoAvailableID
}
