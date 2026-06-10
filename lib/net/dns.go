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
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// DNS record types
const (
	TypeA     uint16 = 1
	TypeAAAA  uint16 = 28
	TypeCNAME uint16 = 5
	TypeMX    uint16 = 15
	TypeTXT   uint16 = 16
	TypeNS    uint16 = 2

	ClassIN uint16 = 1
)

// DNSRecord represents a DNS record
type DNSRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	Data  string
}

// DNSResponse represents a DNS response
type DNSResponse struct {
	ID        uint16
	Questions []DNSQuestion
	Answers   []DNSRecord
	Authority []DNSRecord
	Extra     []DNSRecord
}

// DNSQuestion represents a DNS question
type DNSQuestion struct {
	Name  string
	Type  uint16
	Class uint16
}

// Resolver is a custom DNS resolver
type Resolver struct {
	servers []string
	timeout time.Duration
	cache   *dnsCache
	mu      sync.RWMutex
}

type dnsCache struct {
	entries map[string]cacheEntry
	mu      sync.RWMutex
}

type cacheEntry struct {
	records   []DNSRecord
	expiresAt time.Time
}

var defaultResolver *Resolver
var resolverOnce sync.Once

// DefaultResolver returns the default DNS resolver
func DefaultResolver() *Resolver {
	resolverOnce.Do(func() {
		cfg := GetConfig()
		defaultResolver = NewResolver(cfg.DNS.Servers, time.Duration(cfg.DNS.Timeout)*time.Second, cfg.DNS.Cache)
	})
	return defaultResolver
}

// NewResolver creates a new DNS resolver
func NewResolver(servers []string, timeout time.Duration, enableCache bool) *Resolver {
	r := &Resolver{
		servers: servers,
		timeout: timeout,
	}
	if enableCache {
		r.cache = &dnsCache{
			entries: make(map[string]cacheEntry),
		}
	}
	return r
}

// LookupHost resolves a hostname to IP addresses
func (r *Resolver) LookupHost(host string) ([]string, error) {
	// Check if already an IP
	if ip := net.ParseIP(host); ip != nil {
		return []string{host}, nil
	}

	// Check cache
	if r.cache != nil {
		if ips := r.checkCache(host, TypeA); len(ips) > 0 {
			return ips, nil
		}
	}

	// Query DNS
	records, err := r.Query(host, TypeA)
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, rec := range records {
		if (rec.Type == TypeA || rec.Type == TypeAAAA) && net.ParseIP(rec.Data) != nil {
			ips = append(ips, rec.Data)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no addresses found for %s", host)
	}

	// Cache result
	if r.cache != nil && len(records) > 0 {
		r.updateCache(host, TypeA, records)
	}

	return ips, nil
}

// Query performs a DNS query
func (r *Resolver) Query(name string, qtype uint16) ([]DNSRecord, error) {
	query := r.buildQuery(name, qtype)

	var lastErr error
	for _, addr := range r.serverAddrs() {

		conn, err := net.DialTimeout("udp", addr, r.timeout)
		if err != nil {
			lastErr = err
			continue
		}

		conn.SetDeadline(time.Now().Add(r.timeout))
		_, err = conn.Write(query)
		if err != nil {
			conn.Close()
			lastErr = err
			continue
		}

		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		conn.Close()
		if err != nil {
			lastErr = err
			continue
		}

		response, err := r.parseResponse(buf[:n])
		if err != nil {
			lastErr = err
			continue
		}

		return response.Answers, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no DNS servers available")
}

func (r *Resolver) serverAddrs() []string {
	out := make([]string, 0, len(r.servers)+3)
	seen := make(map[string]struct{}, len(r.servers)+3)
	hasNonLoopback := false

	add := func(server string) {
		addr := normalizeDNSServerAddr(server)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		out = append(out, addr)
		if !isLoopbackDNSAddr(addr) {
			hasNonLoopback = true
		}
	}

	for _, server := range r.servers {
		add(server)
	}

	// If config only points to local stubs (for example ::1/127.0.0.1) and they
	// are down, keep DNS functional by trying public resolvers as fallback.
	if !hasNonLoopback {
		for _, server := range []string{"1.1.1.1", "8.8.8.8", "8.8.4.4"} {
			add(server)
		}
	}

	return out
}

func normalizeDNSServerAddr(server string) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return ""
	}

	if host, port, err := net.SplitHostPort(server); err == nil {
		if strings.TrimSpace(port) == "" {
			port = "53"
		}
		return net.JoinHostPort(host, port)
	}

	trimmed := strings.Trim(server, "[]")
	if ip := net.ParseIP(trimmed); ip != nil {
		return net.JoinHostPort(ip.String(), "53")
	}

	return net.JoinHostPort(server, "53")
}

func isLoopbackDNSAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (r *Resolver) buildQuery(name string, qtype uint16) []byte {
	// DNS header
	buf := make([]byte, 12)
	id := uint16(time.Now().UnixNano() & 0xFFFF)
	binary.BigEndian.PutUint16(buf[0:2], id)     // ID
	binary.BigEndian.PutUint16(buf[2:4], 0x0100) // Flags: standard query, recursion desired
	binary.BigEndian.PutUint16(buf[4:6], 1)      // Questions
	binary.BigEndian.PutUint16(buf[6:8], 0)      // Answers
	binary.BigEndian.PutUint16(buf[8:10], 0)     // Authority
	binary.BigEndian.PutUint16(buf[10:12], 0)    // Additional

	// Encode name
	for label := range strings.SplitSeq(name, ".") {
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0) // Root label

	// Type and class
	typeClass := make([]byte, 4)
	binary.BigEndian.PutUint16(typeClass[0:2], qtype)
	binary.BigEndian.PutUint16(typeClass[2:4], ClassIN)
	buf = append(buf, typeClass...)

	return buf
}

func (r *Resolver) parseResponse(buf []byte) (*DNSResponse, error) {
	if len(buf) < 12 {
		return nil, errors.New("response too short")
	}

	resp := &DNSResponse{
		ID: binary.BigEndian.Uint16(buf[0:2]),
	}

	qdcount := binary.BigEndian.Uint16(buf[4:6])
	ancount := binary.BigEndian.Uint16(buf[6:8])
	nscount := binary.BigEndian.Uint16(buf[8:10])
	arcount := binary.BigEndian.Uint16(buf[10:12])

	offset := 12

	// Parse questions
	for range qdcount {
		name, newOffset := r.parseName(buf, offset)
		offset = newOffset
		if offset+4 > len(buf) {
			return nil, errors.New("truncated question")
		}
		qtype := binary.BigEndian.Uint16(buf[offset : offset+2])
		qclass := binary.BigEndian.Uint16(buf[offset+2 : offset+4])
		offset += 4
		resp.Questions = append(resp.Questions, DNSQuestion{
			Name:  name,
			Type:  qtype,
			Class: qclass,
		})
	}

	// Parse answers
	var err error
	resp.Answers, offset, err = r.parseRecords(buf, offset, ancount)
	if err != nil {
		return nil, err
	}

	resp.Authority, offset, err = r.parseRecords(buf, offset, nscount)
	if err != nil {
		return nil, err
	}

	resp.Extra, _, err = r.parseRecords(buf, offset, arcount)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (r *Resolver) parseRecords(buf []byte, offset int, count uint16) ([]DNSRecord, int, error) {
	var records []DNSRecord

	for range count {
		if offset >= len(buf) {
			break
		}

		name, newOffset := r.parseName(buf, offset)
		offset = newOffset

		if offset+10 > len(buf) {
			return records, offset, errors.New("truncated record")
		}

		rtype := binary.BigEndian.Uint16(buf[offset : offset+2])
		rclass := binary.BigEndian.Uint16(buf[offset+2 : offset+4])
		ttl := binary.BigEndian.Uint32(buf[offset+4 : offset+8])
		rdlength := binary.BigEndian.Uint16(buf[offset+8 : offset+10])
		offset += 10

		if offset+int(rdlength) > len(buf) {
			return records, offset, errors.New("truncated rdata")
		}

		rdata := buf[offset : offset+int(rdlength)]
		offset += int(rdlength)

		data := r.parseRData(rtype, rdata, buf)

		records = append(records, DNSRecord{
			Name:  name,
			Type:  rtype,
			Class: rclass,
			TTL:   ttl,
			Data:  data,
		})
	}

	return records, offset, nil
}

func (r *Resolver) parseName(buf []byte, offset int) (string, int) {
	var parts []string
	jumped := false
	jumpOffset := offset

	for {
		if offset >= len(buf) {
			break
		}

		length := int(buf[offset])
		if length == 0 {
			offset++
			break
		}

		// Compression pointer
		if length&0xC0 == 0xC0 {
			if offset+1 >= len(buf) {
				break
			}
			pointer := int(binary.BigEndian.Uint16(buf[offset:offset+2]) & 0x3FFF)
			if !jumped {
				jumpOffset = offset + 2
			}
			jumped = true
			offset = pointer
			continue
		}

		offset++
		if offset+length > len(buf) {
			break
		}
		parts = append(parts, string(buf[offset:offset+length]))
		offset += length
	}

	if jumped {
		offset = jumpOffset
	}

	return strings.Join(parts, "."), offset
}

func (r *Resolver) parseRData(rtype uint16, rdata []byte, buf []byte) string {
	switch rtype {
	case TypeA:
		if len(rdata) == 4 {
			return fmt.Sprintf("%d.%d.%d.%d", rdata[0], rdata[1], rdata[2], rdata[3])
		}
	case TypeAAAA:
		if len(rdata) == 16 {
			return net.IP(rdata).String()
		}
	case TypeCNAME, TypeNS:
		return r.parseCompressedName(rdata, buf)
	case TypeMX:
		if len(rdata) > 2 {
			// prio := binary.BigEndian.Uint16(rdata[0:2])
			return r.parseCompressedName(rdata[2:], buf)
		}
	case TypeTXT:
		var texts []string
		offset := 0
		for offset < len(rdata) {
			length := int(rdata[offset])
			offset++
			if offset+length <= len(rdata) {
				texts = append(texts, string(rdata[offset:offset+length]))
				offset += length
			}
		}
		return strings.Join(texts, "")
	}
	return ""
}

func (r *Resolver) parseCompressedName(rdata, msg []byte) string {
	if len(rdata) == 0 {
		return ""
	}

	// RFC 1035 compressed pointer: two-byte offset in original message.
	if rdata[0]&0xC0 == 0xC0 {
		if len(rdata) < 2 {
			return ""
		}
		offset := int(binary.BigEndian.Uint16(rdata[:2]) & 0x3FFF)
		name, _ := r.parseName(msg, offset)
		return name
	}

	name, _ := r.parseName(rdata, 0)
	return name
}

func (r *Resolver) checkCache(name string, qtype uint16) []string {
	r.cache.mu.RLock()
	defer r.cache.mu.RUnlock()

	key := fmt.Sprintf("%s:%d", name, qtype)
	entry, ok := r.cache.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	var result []string
	for _, rec := range entry.records {
		if (rec.Type != TypeA && rec.Type != TypeAAAA) || net.ParseIP(rec.Data) == nil {
			continue
		}
		result = append(result, rec.Data)
	}
	return result
}

func (r *Resolver) updateCache(name string, qtype uint16, records []DNSRecord) {
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()

	key := fmt.Sprintf("%s:%d", name, qtype)

	// Find minimum TTL
	minTTL := uint32(300) // Default 5 minutes
	for _, rec := range records {
		if rec.TTL > 0 && rec.TTL < minTTL {
			minTTL = rec.TTL
		}
	}

	r.cache.entries[key] = cacheEntry{
		records:   records,
		expiresAt: time.Now().Add(time.Duration(minTTL) * time.Second),
	}
}

// LookupHost is a convenience function using the default resolver
func LookupHost(host string) ([]string, error) {
	return DefaultResolver().LookupHost(host)
}
