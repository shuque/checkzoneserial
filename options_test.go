package main

import (
	"flag"
	"os"
	"testing"
	"time"
)

// resetFlags resets the flag package state between tests
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

func TestDefaultOptions(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Reset flags before test
	resetFlags()

	// Test with minimal arguments
	os.Args = []string{"cmd", "example.com"}
	zone, opts := doFlags()

	if zone != "example.com." {
		t.Errorf("Expected zone 'example.com.', got '%s'", zone)
	}

	// Check default values
	if opts.Qopts.timeout != time.Duration(defaultTimeout)*time.Second {
		t.Errorf("Expected timeout %d, got %v", defaultTimeout, opts.Qopts.timeout)
	}
	if opts.Qopts.retries != defaultRetries {
		t.Errorf("Expected retries %d, got %d", defaultRetries, opts.Qopts.retries)
	}
	if opts.Qopts.bufsize != defaultBufsize {
		t.Errorf("Expected bufsize %d, got %d", defaultBufsize, opts.Qopts.bufsize)
	}
	if opts.Qopts.nsid {
		t.Error("Expected nsid to be false by default")
	}
	if opts.Qopts.tcp {
		t.Error("Expected tcp to be false by default")
	}
}

func TestCustomOptions(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Reset flags before test
	resetFlags()

	// Test with custom options
	os.Args = []string{"cmd", "-t", "5", "-r", "2", "-b", "4096", "-nsid", "-c", "example.com"}
	zone, opts := doFlags()

	if zone != "example.com." {
		t.Errorf("Expected zone 'example.com.', got '%s'", zone)
	}

	// Check custom values
	if opts.Qopts.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", opts.Qopts.timeout)
	}
	if opts.Qopts.retries != 2 {
		t.Errorf("Expected retries 2, got %d", opts.Qopts.retries)
	}
	if opts.Qopts.bufsize != 4096 {
		t.Errorf("Expected bufsize 4096, got %d", opts.Qopts.bufsize)
	}
	if !opts.Qopts.nsid {
		t.Error("Expected nsid to be true")
	}
	if !opts.Qopts.tcp {
		t.Error("Expected tcp to be true")
	}
}

func TestDefaultValues(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Test with no timeout flag
	resetFlags()
	os.Args = []string{"cmd", "example.com"}
	zone, opts := doFlags()
	if zone != "example.com." {
		t.Errorf("Expected zone 'example.com.', got '%s'", zone)
	}
	if opts.Qopts.timeout != time.Duration(defaultTimeout)*time.Second {
		t.Errorf("Expected default timeout %d, got %v", defaultTimeout, opts.Qopts.timeout)
	}

	// Test with no buffer size flag
	resetFlags()
	os.Args = []string{"cmd", "example.com"}
	zone, opts = doFlags()
	if zone != "example.com." {
		t.Errorf("Expected zone 'example.com.', got '%s'", zone)
	}
	if opts.Qopts.bufsize != defaultBufsize {
		t.Errorf("Expected default bufsize %d, got %d", defaultBufsize, opts.Qopts.bufsize)
	}
}

func TestIPv4IPv6Options(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Reset flags before test
	resetFlags()

	// Test IPv4 only
	os.Args = []string{"cmd", "-4", "example.com"}
	_, opts := doFlags()
	if !opts.useV4 {
		t.Error("Expected useV4 to be true")
	}
	if opts.useV6 {
		t.Error("Expected useV6 to be false")
	}

	// Reset flags before next test
	resetFlags()

	// Test IPv6 only
	os.Args = []string{"cmd", "-6", "example.com"}
	_, opts = doFlags()
	if !opts.useV6 {
		t.Error("Expected useV6 to be true")
	}
	if opts.useV4 {
		t.Error("Expected useV4 to be false")
	}
}

func TestMasterServerOptions(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Reset flags before test
	resetFlags()

	// Test with IP address
	os.Args = []string{"cmd", "-m", "192.168.1.1", "example.com"}
	_, opts := doFlags()
	if opts.masterIP.String() != "192.168.1.1" {
		t.Errorf("Expected master IP 192.168.1.1, got %s", opts.masterIP)
	}

	// Reset flags before next test
	resetFlags()

	// Test with hostname
	os.Args = []string{"cmd", "-m", "master.example.com", "example.com"}
	_, opts = doFlags()
	if opts.masterName != "master.example.com." {
		t.Errorf("Expected master name master.example.com., got %s", opts.masterName)
	}
}

func TestAdditionalNameservers(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Reset flags before test
	resetFlags()

	// Test with additional nameservers
	os.Args = []string{"cmd", "-a", "ns1.example.com,ns2.example.com", "example.com"}
	_, opts := doFlags()
	if opts.additional != "ns1.example.com,ns2.example.com" {
		t.Errorf("Expected additional nameservers 'ns1.example.com,ns2.example.com', got '%s'", opts.additional)
	}
}
