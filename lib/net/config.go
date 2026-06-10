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

package net

import (
	"strconv"
	"strings"
	"sync"

	"avyos.dev/lib/ini"
)

const (
	ConfigPath         = "/config/net.conf"
	DefaultDNSTimeout  = 5
	DefaultHTTPTimeout = 30
	DefaultUserAgent   = "avyos/1.0"
)

// Config holds network configuration
type Config struct {
	DNS  DNSConfig
	HTTP HTTPConfig
	TLS  TLSConfig
}

// DNSConfig holds DNS resolver configuration
type DNSConfig struct {
	Servers []string
	Timeout int
	Cache   bool
}

// HTTPConfig holds HTTP client configuration
type HTTPConfig struct {
	UserAgent       string
	Timeout         int
	FollowRedirects bool
	MaxRedirects    int
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Verify        bool
	CertPath      string
	InsecureHosts []string
}

var (
	globalConfig *Config
	configMu     sync.RWMutex
	configLoaded bool
)

// DefaultConfig returns the default network configuration
func DefaultConfig() *Config {
	return &Config{
		DNS: DNSConfig{
			Servers: []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"},
			Timeout: DefaultDNSTimeout,
			Cache:   true,
		},
		HTTP: HTTPConfig{
			UserAgent:       DefaultUserAgent,
			Timeout:         DefaultHTTPTimeout,
			FollowRedirects: true,
			MaxRedirects:    10,
		},
		TLS: TLSConfig{
			Verify:   true,
			CertPath: "/config/certificates",
		},
	}
}

// LoadConfig loads network configuration from the config file
func LoadConfig() (*Config, error) {
	configMu.Lock()
	defer configMu.Unlock()

	if configLoaded && globalConfig != nil {
		return globalConfig, nil
	}

	cfg := DefaultConfig()

	iniCfg, err := ini.ParseFile(ConfigPath)
	if err != nil {
		// Use defaults if config doesn't exist
		globalConfig = cfg
		configLoaded = true
		return cfg, nil
	}

	// Parse DNS section
	if servers, ok := iniCfg.Get("dns", "servers"); ok {
		cfg.DNS.Servers = parseList(servers)
	}
	if timeout, ok := iniCfg.Get("dns", "timeout"); ok {
		if t, err := strconv.Atoi(timeout); err == nil {
			cfg.DNS.Timeout = t
		}
	}
	if cache, ok := iniCfg.Get("dns", "cache"); ok {
		cfg.DNS.Cache = parseBool(cache)
	}

	// Parse HTTP section
	if ua, ok := iniCfg.Get("http", "user_agent"); ok {
		cfg.HTTP.UserAgent = ua
	}
	if timeout, ok := iniCfg.Get("http", "timeout"); ok {
		if t, err := strconv.Atoi(timeout); err == nil {
			cfg.HTTP.Timeout = t
		}
	}
	if follow, ok := iniCfg.Get("http", "follow_redirects"); ok {
		cfg.HTTP.FollowRedirects = parseBool(follow)
	}
	if maxRedir, ok := iniCfg.Get("http", "max_redirects"); ok {
		if m, err := strconv.Atoi(maxRedir); err == nil {
			cfg.HTTP.MaxRedirects = m
		}
	}

	// Parse TLS section
	if verify, ok := iniCfg.Get("tls", "verify"); ok {
		cfg.TLS.Verify = parseBool(verify)
	}
	if certPath, ok := iniCfg.Get("tls", "cert_path"); ok {
		cfg.TLS.CertPath = certPath
	}
	if insecure, ok := iniCfg.Get("tls", "insecure_hosts"); ok {
		cfg.TLS.InsecureHosts = parseList(insecure)
	}

	globalConfig = cfg
	configLoaded = true
	return cfg, nil
}

// GetConfig returns the current global configuration
func GetConfig() *Config {
	configMu.RLock()
	if globalConfig != nil {
		defer configMu.RUnlock()
		return globalConfig
	}
	configMu.RUnlock()

	cfg, _ := LoadConfig()
	return cfg
}

// ReloadConfig forces a reload of the configuration
func ReloadConfig() (*Config, error) {
	configMu.Lock()
	configLoaded = false
	globalConfig = nil
	configMu.Unlock()
	return LoadConfig()
}

func parseList(s string) []string {
	var result []string
	for item := range strings.SplitSeq(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "yes" || s == "1" || s == "on"
}
