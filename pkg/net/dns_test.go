package net

import (
	"net"
	"testing"
	"time"
)

func TestCheckCacheFiltersInvalidIPs(t *testing.T) {
	r := NewResolver(nil, time.Second, true)
	if r.cache == nil {
		t.Fatalf("expected resolver cache to be enabled")
	}

	r.cache.entries["example.com:1"] = cacheEntry{
		records: []DNSRecord{
			{Type: TypeCNAME, Data: ""},
			{Type: TypeCNAME, Data: "cdn.example.com"},
			{Type: TypeA, Data: "93.184.216.34"},
			{Type: TypeAAAA, Data: "2001:db8::1"},
			{Type: TypeA, Data: "not-an-ip"},
		},
		expiresAt: time.Now().Add(time.Minute),
	}

	ips := r.checkCache("example.com", TypeA)
	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d: %#v", len(ips), ips)
	}
	if ips[0] != "93.184.216.34" {
		t.Fatalf("unexpected first IP: %q", ips[0])
	}
	if ips[1] != "2001:db8::1" {
		t.Fatalf("unexpected second IP: %q", ips[1])
	}
}

func TestParseRDataCNAMECompressedPointer(t *testing.T) {
	r := NewResolver(nil, time.Second, false)

	// DNS message with "example.com" at offset 12.
	msg := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,
	}

	got := r.parseRData(TypeCNAME, []byte{0xC0, 0x0C}, msg)
	if got != "example.com" {
		t.Fatalf("expected example.com, got %q", got)
	}
}

func TestNormalizeDNSServerAddr(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "8.8.8.8", want: "8.8.8.8:53"},
		{in: "8.8.8.8:1053", want: "8.8.8.8:1053"},
		{in: "::1", want: "[::1]:53"},
		{in: "[::1]:53", want: "[::1]:53"},
		{in: "2001:4860:4860::8888", want: "[2001:4860:4860::8888]:53"},
		{in: "localhost", want: "localhost:53"},
		{in: "", want: ""},
	}

	for _, tt := range tests {
		if got := normalizeDNSServerAddr(tt.in); got != tt.want {
			t.Fatalf("normalizeDNSServerAddr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestServerAddrsFallbackForLoopbackOnlyConfig(t *testing.T) {
	r := NewResolver([]string{"::1", "127.0.0.1"}, time.Second, false)
	addrs := r.serverAddrs()
	if len(addrs) < 3 {
		t.Fatalf("expected loopback and fallback resolvers, got %#v", addrs)
	}

	hasFallback := false
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		ip := net.ParseIP(host)
		if ip != nil && !ip.IsLoopback() {
			hasFallback = true
			break
		}
	}
	if !hasFallback {
		t.Fatalf("expected at least one non-loopback fallback resolver, got %#v", addrs)
	}
}
