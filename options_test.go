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
	zone, opts, err := doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	if opts.Qopts.nsid != false {
		t.Errorf("Expected nsid false, got %v", opts.Qopts.nsid)
	}
	if opts.Qopts.tcp != false {
		t.Errorf("Expected tcp false, got %v", opts.Qopts.tcp)
	}
	if opts.delta != defaultSerialDelta {
		t.Errorf("Expected delta %d, got %d", defaultSerialDelta, opts.delta)
	}
	if opts.Qopts.rdflag != false {
		t.Errorf("Expected rdflag false, got %v", opts.Qopts.rdflag)
	}
	if opts.Qopts.adflag != false {
		t.Errorf("Expected adflag false, got %v", opts.Qopts.adflag)
	}
	if opts.Qopts.cdflag != false {
		t.Errorf("Expected cdflag false, got %v", opts.Qopts.cdflag)
	}
	if opts.sortresponse != false {
		t.Errorf("Expected sortresponse false, got %v", opts.sortresponse)
	}
	if opts.json != false {
		t.Errorf("Expected json false, got %v", opts.json)
	}
	if opts.V6Only != false {
		t.Errorf("Expected V6Only false, got %v", opts.V6Only)
	}
	if opts.V4Only != false {
		t.Errorf("Expected V4Only false, got %v", opts.V4Only)
	}
	if opts.noqueryns != false {
		t.Errorf("Expected noqueryns false, got %v", opts.noqueryns)
	}
	if opts.masterIP != nil {
		t.Errorf("Expected masterIP nil, got %v", opts.masterIP)
	}
	if opts.masterName != "" {
		t.Errorf("Expected masterName '', got '%s'", opts.masterName)
	}
	if opts.additional != "" {
		t.Errorf("Expected additional '', got '%s'", opts.additional)
	}
}

func TestCustomOptions(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Reset flags before test
	resetFlags()

	// Test with custom options
	os.Args = []string{"cmd", "-t", "5", "-r", "2", "-d", "10", "-b", "4096", "-s", "-j", "-6", "-n", "example.com"}
	zone, opts, err := doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

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
	if opts.delta != 10 {
		t.Errorf("Expected delta 10, got %d", opts.delta)
	}
	if opts.Qopts.bufsize != 4096 {
		t.Errorf("Expected bufsize 4096, got %d", opts.Qopts.bufsize)
	}
	if opts.sortresponse != true {
		t.Errorf("Expected sortresponse true, got %v", opts.sortresponse)
	}
	if opts.json != true {
		t.Errorf("Expected json true, got %v", opts.json)
	}
	if opts.V6Only != true {
		t.Errorf("Expected V6Only true, got %v", opts.V6Only)
	}
	if opts.V4Only != false {
		t.Errorf("Expected V4Only false, got %v", opts.V4Only)
	}
	if opts.noqueryns != true {
		t.Errorf("Expected noqueryns true, got %v", opts.noqueryns)
	}
}

func TestDefaultValues(t *testing.T) {
	// Save original args and restore them after the test
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Test with no timeout flag
	resetFlags()
	os.Args = []string{"cmd", "example.com"}
	zone, opts, err := doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if zone != "example.com." {
		t.Errorf("Expected zone 'example.com.', got '%s'", zone)
	}
	if opts.Qopts.timeout != time.Duration(defaultTimeout)*time.Second {
		t.Errorf("Expected default timeout %d, got %v", defaultTimeout, opts.Qopts.timeout)
	}

	// Test with no buffer size flag
	resetFlags()
	os.Args = []string{"cmd", "example.com"}
	zone, opts, err = doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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

	// Test IPv4 only
	resetFlags()
	os.Args = []string{"cmd", "-4", "example.com"}
	_, opts, err := doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !opts.V4Only {
		t.Errorf("Expected V4Only true, got %v", opts.V4Only)
	}
	if opts.V6Only {
		t.Errorf("Expected V6Only false, got %v", opts.V6Only)
	}

	// Test IPv6 only
	resetFlags()
	os.Args = []string{"cmd", "-6", "example.com"}
	_, opts, err = doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if opts.V4Only {
		t.Errorf("Expected V4Only false, got %v", opts.V4Only)
	}
	if !opts.V6Only {
		t.Errorf("Expected V6Only true, got %v", opts.V6Only)
	}

	// Test both (should fail)
	resetFlags()
	os.Args = []string{"cmd", "-4", "-6", "example.com"}
	_, opts, err = doFlags()
	if err == nil {
		t.Error("Expected error when both -4 and -6 are specified")
	}
	if err.Error() != "cannot specify both -4 and -6" {
		t.Errorf("Expected error 'cannot specify both -4 and -6', got '%v'", err)
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
	_, opts, err := doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if opts.masterIP.String() != "192.168.1.1" {
		t.Errorf("Expected master IP 192.168.1.1, got %s", opts.masterIP)
	}

	// Reset flags before next test
	resetFlags()

	// Test with hostname
	os.Args = []string{"cmd", "-m", "master.example.com", "example.com"}
	_, opts, err = doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	_, opts, err := doFlags()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if opts.additional != "ns1.example.com,ns2.example.com" {
		t.Errorf("Expected additional nameservers 'ns1.example.com,ns2.example.com', got '%s'", opts.additional)
	}
}
