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
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Request represents an HTTP request
type Request struct {
	Method  string
	URL     string
	Host    string
	Path    string
	Headers map[string]string
	Body    io.Reader
}

// Response represents an HTTP response
type Response struct {
	StatusCode int
	Status     string
	Headers    map[string]string
	Body       io.ReadCloser
	Request    *Request
}

// Client is a custom HTTP client
type Client struct {
	config   *Config
	resolver *Resolver
	timeout  time.Duration
	pool     *x509.CertPool
}

// NewClient creates a new HTTP client
func NewClient() *Client {
	cfg := GetConfig()
	c := &Client{
		config:   cfg,
		resolver: DefaultResolver(),
		timeout:  time.Duration(cfg.HTTP.Timeout) * time.Second,
	}
	c.pool = newCAPool()
	loadCertificates(c.pool, c.config.TLS.CertPath)
	loadCertificates(c.pool, "/avyos/config/certificates")
	return c
}

// Get performs an HTTP GET request
func (c *Client) Get(url string) (*Response, error) {
	return c.Do(&Request{
		Method:  "GET",
		URL:     url,
		Headers: make(map[string]string),
	})
}

// Post performs an HTTP POST request
func (c *Client) Post(url string, contentType string, body io.Reader) (*Response, error) {
	return c.Do(&Request{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		Body: body,
	})
}

// Do performs an HTTP request
func (c *Client) Do(req *Request) (*Response, error) {
	return c.doWithRedirects(req, 0)
}

func (c *Client) doWithRedirects(req *Request, redirectCount int) (*Response, error) {
	if redirectCount > c.config.HTTP.MaxRedirects {
		return nil, fmt.Errorf("too many redirects (max %d)", c.config.HTTP.MaxRedirects)
	}

	// Parse URL
	scheme, host, port, path := parseURL(req.URL)
	if scheme == "" {
		scheme = "http"
	}
	if path == "" {
		path = "/"
	}

	req.Host = host
	req.Path = path

	// Resolve host
	ips, err := c.resolver.LookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	targetIP := ""
	for _, ip := range ips {
		if net.ParseIP(ip) != nil {
			targetIP = ip
			break
		}
	}
	if targetIP == "" {
		return nil, fmt.Errorf("no valid IP addresses found for %s", host)
	}

	// Determine port
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Connect
	addr := targetIP + ":" + port
	var conn net.Conn

	dialer := &net.Dialer{Timeout: c.timeout}
	conn, err = dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	// Wrap with TLS if HTTPS
	if scheme == "https" {
		tlsConn, err := c.wrapTLS(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS handshake failed: %w", err)
		}
		conn = tlsConn
	}

	// Set deadline
	conn.SetDeadline(time.Now().Add(c.timeout))

	// Send request
	if err := c.sendRequest(conn, req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send request failed: %w", err)
	}

	// Read response
	resp, err := c.readResponse(conn, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	// Handle redirects
	if c.config.HTTP.FollowRedirects && isRedirect(resp.StatusCode) {
		location := resp.Headers["location"]
		if location == "" {
			return resp, nil
		}

		// Close current connection
		resp.Body.Close()

		// Resolve relative URLs
		if !strings.HasPrefix(location, "http://") && !strings.HasPrefix(location, "https://") {
			if strings.HasPrefix(location, "/") {
				location = scheme + "://" + host + location
			} else {
				location = scheme + "://" + host + "/" + location
			}
		}

		newReq := &Request{
			Method:  "GET", // Redirects typically become GET
			URL:     location,
			Headers: make(map[string]string),
		}

		// Copy safe headers
		for k, v := range req.Headers {
			lk := strings.ToLower(k)
			if lk != "content-type" && lk != "content-length" {
				newReq.Headers[k] = v
			}
		}

		return c.doWithRedirects(newReq, redirectCount+1)
	}

	return resp, nil
}

func (c *Client) wrapTLS(conn net.Conn, host string) (*tls.Conn, error) {
	// Check if host should skip verification
	skipVerify := !c.config.TLS.Verify
	if slices.Contains(c.config.TLS.InsecureHosts, host) {
		skipVerify = true
	}

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: skipVerify,
		RootCAs:            c.pool,
		MinVersion:         tls.VersionTLS12,
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, err
	}

	return tlsConn, nil
}

func (c *Client) sendRequest(conn net.Conn, req *Request) error {
	var sb strings.Builder

	// Request line
	sb.WriteString(req.Method)
	sb.WriteString(" ")
	sb.WriteString(req.Path)
	sb.WriteString(" HTTP/1.1\r\n")

	// Headers
	sb.WriteString("Host: ")
	sb.WriteString(req.Host)
	sb.WriteString("\r\n")

	sb.WriteString("User-Agent: ")
	sb.WriteString(c.config.HTTP.UserAgent)
	sb.WriteString("\r\n")

	sb.WriteString("Connection: close\r\n")
	sb.WriteString("Accept: */*\r\n")

	for k, v := range req.Headers {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString("\r\n")
	}

	sb.WriteString("\r\n")

	_, err := conn.Write([]byte(sb.String()))
	if err != nil {
		return err
	}

	// Write body if present
	if req.Body != nil {
		_, err = io.Copy(conn, req.Body)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) readResponse(conn net.Conn, req *Request) (*Response, error) {
	reader := bufio.NewReader(conn)

	// Read status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(strings.TrimSpace(statusLine), " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid status line: %s", statusLine)
	}

	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid status code: %s", parts[1])
	}

	status := ""
	if len(parts) > 2 {
		status = parts[2]
	}

	// Read headers
	headers := make(map[string]string)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx > 0 {
			key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
			value := strings.TrimSpace(line[colonIdx+1:])
			headers[key] = value
		}
	}

	// Create response with body reader
	resp := &Response{
		StatusCode: statusCode,
		Status:     status,
		Headers:    headers,
		Body:       &responseBody{reader: reader, conn: conn},
		Request:    req,
	}

	return resp, nil
}

type responseBody struct {
	reader *bufio.Reader
	conn   net.Conn
}

func (rb *responseBody) Read(p []byte) (int, error) {
	return rb.reader.Read(p)
}

func (rb *responseBody) Close() error {
	return rb.conn.Close()
}

func parseURL(url string) (scheme, host, port, path string) {
	// Remove scheme
	if strings.HasPrefix(url, "https://") {
		scheme = "https"
		url = url[8:]
	} else if strings.HasPrefix(url, "http://") {
		scheme = "http"
		url = url[7:]
	}

	// Split host and path
	pathIdx := strings.Index(url, "/")
	if pathIdx == -1 {
		host = url
		path = "/"
	} else {
		host = url[:pathIdx]
		path = url[pathIdx:]
	}

	// Extract port from host
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Check it's not IPv6
		if !strings.Contains(host, "[") || strings.LastIndex(host, "]") < colonIdx {
			port = host[colonIdx+1:]
			host = host[:colonIdx]
		}
	}

	return
}

func isRedirect(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

// Convenience functions using default client

var defaultClient *Client
var clientOnce = &struct{}{}

func getDefaultClient() *Client {
	if defaultClient == nil {
		defaultClient = NewClient()
	}
	return defaultClient
}

// Get performs an HTTP GET request using the default client
func Get(url string) (*Response, error) {
	return getDefaultClient().Get(url)
}

// Post performs an HTTP POST request using the default client
func Post(url string, contentType string, body io.Reader) (*Response, error) {
	return getDefaultClient().Post(url, contentType, body)
}
