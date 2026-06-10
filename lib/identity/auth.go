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
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"os"
	"slices"
	"strings"
)

// Authenticate verifies credentials and returns the identity
func Authenticate(username, password string) (*Identity, error) {
	// First lookup the identity
	identity, err := LookupByName(username)
	if err != nil {
		return nil, err
	}

	// Get auth entry
	auth, err := getAuthByName(username)
	if err != nil {
		return nil, err
	}

	// Check auth type
	switch auth.Type {
	case "none":
		// No password required
		return identity, nil
	case "locked":
		return nil, ErrAccountLocked
	case "password":
		if verifyPassword(password, auth.Hash) {
			return identity, nil
		}
		return nil, ErrInvalidCredentials
	default:
		return nil, ErrInvalidCredentials
	}
}

// AuthenticateByID verifies credentials using user ID
func AuthenticateByID(uid int, password string) (*Identity, error) {
	identity, err := LookupByID(uid)
	if err != nil {
		return nil, err
	}

	auth, err := getAuthByID(uid)
	if err != nil {
		return nil, err
	}

	switch auth.Type {
	case "none":
		return identity, nil
	case "locked":
		return nil, ErrAccountLocked
	case "password":
		if verifyPassword(password, auth.Hash) {
			return identity, nil
		}
		return nil, ErrInvalidCredentials
	default:
		return nil, ErrInvalidCredentials
	}
}

// HashPassword creates a hash for a password
func HashPassword(password string) string {
	hash := sha512.Sum512([]byte(password))
	return "sha512:" + base64.StdEncoding.EncodeToString(hash[:])
}

// UpdatePassword update the password
func UpdatePassword(identity, oldpassword, newpassword string) error {
	id, err := LookupByName(identity)
	if err != nil {
		return err
	}

	auth, err := getAuthByName(identity)
	if err != nil && err != ErrAuthNotFound {
		return err
	}

	if err == ErrAuthNotFound {
		auth = &Auth{
			ID:   id.ID,
			Name: id.Name,
			Type: "password",
		}
	} else {
		if IsAccountLocked(identity) {
			return ErrAccountLocked
		}
		if auth.Hash != HashPassword(oldpassword) {
			return ErrInvalidCredentials
		}
	}

	auth.Hash = HashPassword(newpassword)

	data, err := loadAuthConfig()
	if err != nil {
		return err
	}

	idx := slices.IndexFunc(data.Entries, func(a Auth) bool {
		return a.ID == auth.ID
	})
	if idx != -1 {
		data.Entries[idx] = *auth
	} else {
		data.Entries = append(data.Entries, *auth)
	}

	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile("/config/security/auth.conf", out, 0600)
}

// verifyPassword checks if a password matches a hash
func verifyPassword(password, storedHash string) bool {
	if after, ok := strings.CutPrefix(storedHash, "sha512:"); ok {
		hashData := after
		decoded, err := base64.StdEncoding.DecodeString(hashData)
		if err != nil {
			return false
		}

		computed := sha512.Sum512([]byte(password))
		return subtle.ConstantTimeCompare(decoded, computed[:]) == 1
	}

	// Plain text comparison (not recommended, but supported for testing)
	if after, ok := strings.CutPrefix(storedHash, "plain:"); ok {
		plainPass := after
		return subtle.ConstantTimeCompare([]byte(password), []byte(plainPass)) == 1
	}

	return false
}

// GetAuthType returns the authentication type for a user
func GetAuthType(username string) (string, error) {
	auth, err := getAuthByName(username)
	if err != nil {
		return "", err
	}
	return auth.Type, nil
}

// IsAccountLocked checks if a user account is locked
func IsAccountLocked(username string) bool {
	auth, err := getAuthByName(username)
	if err != nil {
		return true // Treat missing auth as locked
	}
	return auth.Type == "locked"
}

// getAuthByID loads auth config and finds by ID
func getAuthByID(uid int) (*Auth, error) {
	config, err := loadAuthConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Entries {
		if config.Entries[i].ID == uid {
			return &config.Entries[i], nil
		}
	}

	return nil, ErrAuthNotFound
}

// getAuthByName loads auth config and finds by name
func getAuthByName(name string) (*Auth, error) {
	config, err := loadAuthConfig()
	if err != nil {
		return nil, err
	}

	for i := range config.Entries {
		if config.Entries[i].Name == name {
			return &config.Entries[i], nil
		}
	}

	return nil, ErrAuthNotFound
}

// loadAuthConfig reads and parses the auth config file
func loadAuthConfig() (*AuthConfig, error) {
	data, err := os.ReadFile("/etc/security/auth.conf")
	if err != nil {
		return nil, err
	}

	var config AuthConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
