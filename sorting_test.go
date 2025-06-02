package main

import (
	"net"
	"reflect"
	"sort"
	"testing"
)

func TestCanonicalDomainOrder(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{
			name:     "different TLDs",
			a:        "foo.example.com",
			b:        "foo.example.net",
			expected: -1,
		},
		{
			name:     "same domain",
			a:        "salesforce.com",
			b:        "salesforce.com",
			expected: 0,
		},
		{
			name:     "different subdomains same TLD",
			a:        "foo.example.com",
			b:        "bar.example.com",
			expected: 1,
		},
		{
			name:     "different TLDs with subdomains",
			a:        "z.x.y.example.com",
			b:        "a.example.net",
			expected: -1,
		},
		{
			name:     "empty strings",
			a:        "",
			b:        "",
			expected: 0,
		},
		{
			name:     "one empty string",
			a:        "",
			b:        "example.com",
			expected: -1,
		},
		{
			name:     "different length domains",
			a:        "example.com",
			b:        "sub.example.com",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanonicalDomainOrder(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("CanonicalDomainOrder(%q, %q) = %d, want %d",
					tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestByCanonicalOrder(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "sort domains",
			input: []string{
				"z.example.com",
				"a.example.net",
				"b.example.com",
				"a.example.com",
			},
			expected: []string{
				"a.example.com",
				"b.example.com",
				"z.example.com",
				"a.example.net",
			},
		},
		{
			name: "sort with empty strings",
			input: []string{
				"example.com",
				"",
				"a.example.com",
			},
			expected: []string{
				"",
				"example.com",
				"a.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Sort(ByCanonicalOrder(tt.input))
			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("ByCanonicalOrder.Sort() = %v, want %v",
					tt.input, tt.expected)
			}
		})
	}
}

func TestByIPversion(t *testing.T) {
	tests := []struct {
		name     string
		input    []Response
		expected []Response
	}{
		{
			name: "sort by IP version",
			input: []Response{
				{ip: net.ParseIP("2001:db8::1")},
				{ip: net.ParseIP("192.168.1.1")},
				{ip: net.ParseIP("2001:db8::2")},
				{ip: net.ParseIP("10.0.0.1")},
			},
			expected: []Response{
				{ip: net.ParseIP("2001:db8::1")},
				{ip: net.ParseIP("2001:db8::2")},
				{ip: net.ParseIP("10.0.0.1")},
				{ip: net.ParseIP("192.168.1.1")},
			},
		},
		{
			name: "sort with nil IPs",
			input: []Response{
				{ip: net.ParseIP("192.168.1.1")},
				{ip: nil},
				{ip: net.ParseIP("2001:db8::1")},
			},
			expected: []Response{
				{ip: nil},
				{ip: net.ParseIP("2001:db8::1")},
				{ip: net.ParseIP("192.168.1.1")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Sort(ByIPversion(tt.input))
			if len(tt.input) != len(tt.expected) {
				t.Errorf("ByIPversion.Sort() length = %d, want %d", len(tt.input), len(tt.expected))
				return
			}
			for i := range tt.input {
				if tt.input[i].ip == nil && tt.expected[i].ip == nil {
					continue
				}
				if tt.input[i].ip == nil || tt.expected[i].ip == nil {
					t.Errorf("ByIPversion.Sort()[%d] = %v, want %v", i, tt.input[i].ip, tt.expected[i].ip)
					continue
				}
				if !tt.input[i].ip.Equal(tt.expected[i].ip) {
					t.Errorf("ByIPversion.Sort()[%d] = %v, want %v", i, tt.input[i].ip, tt.expected[i].ip)
				}
			}
		})
	}
}
