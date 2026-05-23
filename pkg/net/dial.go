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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"
)

// Dialer provides custom connection dialing with avyos DNS resolution
type Dialer struct {
	Timeout  time.Duration
	Resolver *Resolver
	Config   *Config
	Pool     *x509.CertPool
}

// NewDialer creates a new Dialer with default configuration
func NewDialer() *Dialer {
	cfg := GetConfig()
	d := &Dialer{
		Timeout:  time.Duration(cfg.HTTP.Timeout) * time.Second,
		Resolver: DefaultResolver(),
		Config:   cfg,
	}
	d.Pool = newCAPool()
	loadCertificates(d.Pool, cfg.TLS.CertPath)
	loadCertificates(d.Pool, "/avyos/config/certificates")
	return d
}

// Dial connects to the address using custom DNS resolution
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		// Might be missing port
		host = address
		port = ""
	}

	// Check if host is already an IP
	if ip := net.ParseIP(host); ip != nil {
		dialer := &net.Dialer{Timeout: d.Timeout}
		return dialer.Dial(network, address)
	}

	// Resolve hostname
	ips, err := d.Resolver.LookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	// Try each IP
	var lastErr error
	dialer := &net.Dialer{Timeout: d.Timeout}

	for _, ip := range ips {
		addr := ip
		if port != "" {
			addr = net.JoinHostPort(ip, port)
		}

		conn, err := dialer.Dial(network, addr)
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}

	return nil, lastErr
}

// DialTLS connects to the address over TLS using custom DNS resolution
func (d *Dialer) DialTLS(network, address string) (*tls.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host = address
		if strings.Contains(network, "tcp") {
			port = "443"
		}
	}

	// Get base connection
	conn, err := d.Dial(network, net.JoinHostPort(host, port))
	if err != nil {
		return nil, err
	}

	// Check if host should skip verification
	skipVerify := !d.Config.TLS.Verify
	if slices.Contains(d.Config.TLS.InsecureHosts, host) {
		skipVerify = true
	}

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: skipVerify,
		MinVersion:         tls.VersionTLS12,
		RootCAs:            d.Pool,
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return tlsConn, nil
}

// Dial connects to the address using the default dialer
func Dial(network, address string) (net.Conn, error) {
	return NewDialer().Dial(network, address)
}

// DialTLS connects to the address over TLS using the default dialer
func DialTLS(network, address string) (*tls.Conn, error) {
	return NewDialer().DialTLS(network, address)
}

// DialTimeout connects to the address with a custom timeout
func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d := NewDialer()
	d.Timeout = timeout
	return d.Dial(network, address)
}
