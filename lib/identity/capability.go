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
	"os"
)

// LookupCapabilityByID finds a capability by its numeric ID
func LookupCapabilityByID(gid int) (*Capability, error) {
	config, err := loadCapabilityConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Capabilities {
		if config.Capabilities[i].ID == gid {
			return &config.Capabilities[i], nil
		}
	}

	return nil, ErrGroupNotFound
}

// LookupCapabilityByName finds a capability by its name
func LookupCapabilityByName(name string) (*Capability, error) {
	config, err := loadCapabilityConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Capabilities {
		if config.Capabilities[i].Name == name {
			return &config.Capabilities[i], nil
		}
	}

	return nil, ErrGroupNotFound
}

// ListCapabilities returns all capabilities in the system
func ListCapabilities() ([]*Capability, error) {
	config, err := loadCapabilityConfig()
	if err != nil {
		return nil, err
	}

	capabilities := make([]*Capability, len(config.Capabilities))
	for i := range config.Capabilities {
		capabilities[i] = &config.Capabilities[i]
	}

	return capabilities, nil
}

// GetCapabilityMembers returns all identities who have a capability
func GetCapabilityMembers(capName string) ([]*Identity, error) {
	identities, err := ListIdentities()
	if err != nil {
		return nil, err
	}

	members := make([]*Identity, 0)
	for _, identity := range identities {
		if identity.HasCapability(capName) || identity.HasCapability("unix:"+capName) {
			members = append(members, identity)
		}
	}

	return members, nil
}

// loadCapabilityConfig reads and parses the capabilities config file
func loadCapabilityConfig() (*CapabilityConfig, error) {
	data, err := os.ReadFile("/etc/security/capabilities.conf")
	if err != nil {
		return nil, err
	}

	var config CapabilityConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
