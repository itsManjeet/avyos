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

import "errors"

// Identity represents a user account from identity.conf
type Identity struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Home         string   `json:"home,omitempty"`
	Shell        string   `json:"shell,omitempty"`
}

// Capability represents a Unix group mapping from capabilities.conf
type Capability struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Auth represents authentication info from auth.conf
type Auth struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "password", "none", "locked"
	Hash string `json:"hash,omitempty"`
}

// IdentityConfig holds all identities
type IdentityConfig struct {
	Identities []Identity `json:"identities"`
}

// CapabilityConfig holds all capabilities (group mappings)
type CapabilityConfig struct {
	Capabilities []Capability `json:"capabilities"`
}

// AuthConfig holds all authentication entries
type AuthConfig struct {
	Entries []Auth `json:"entries"`
}

// Common errors
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrGroupNotFound       = errors.New("group not found")
	ErrAuthNotFound        = errors.New("auth entry not found")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountLocked       = errors.New("account is locked")
	ErrInvalidIdentityKind = errors.New("invalid identity kind")
	ErrNoAvailableID       = errors.New("no available id in range")
)

// Config paths
