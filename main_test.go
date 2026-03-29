package main

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestSerialDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1, s2   uint32
		expected uint32
	}{
		{"same serial", 100, 100, 0},
		{"small forward difference", 100, 95, 5},
		{"small backward difference", 95, 100, 5},
		{"wraparound", 5, 4294967290, 11},
		{"wraparound reversed", 4294967290, 5, 11},
		{"max ambiguous distance", 0, 2147483648, 2147483648},
		{"adjacent at zero", 0, 1, 1},
		{"adjacent at max", 4294967295, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serialDistance(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("serialDistance(%d, %d) = %d, want %d",
					tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}

func TestSerialDelta(t *testing.T) {
	tests := []struct {
		name           string
		master, slave  uint32
		expected       int
	}{
		{"same serial", 100, 100, 0},
		{"master ahead", 100, 95, 5},
		{"slave ahead", 95, 100, -5},
		{"wraparound master ahead", 5, 4294967290, 11},
		{"wraparound slave ahead", 4294967290, 5, -11},
		{"one apart at zero boundary", 0, 4294967295, 1},
		{"one apart at zero boundary reversed", 4294967295, 0, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serialDelta(tt.master, tt.slave)
			if result != tt.expected {
				t.Errorf("serialDelta(%d, %d) = %d, want %d",
					tt.master, tt.slave, result, tt.expected)
			}
		})
	}
}

func TestMaxSerialDrift(t *testing.T) {
	tests := []struct {
		name     string
		serials  []uint32
		expected uint32
	}{
		{"all same", []uint32{100, 100, 100}, 0},
		{"simple spread", []uint32{100, 103, 105}, 5},
		{"wraparound spread", []uint32{4294967294, 0, 2}, 4},
		{"single element", []uint32{42}, 0},
		{"two elements", []uint32{200, 195}, 5},
		{"two elements wraparound", []uint32{3, 4294967293}, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxSerialDrift(tt.serials)
			if result != tt.expected {
				t.Errorf("maxSerialDrift(%v) = %d, want %d",
					tt.serials, result, tt.expected)
			}
		})
	}
}

func TestGetAdditionalServers(t *testing.T) {
	tests := []struct {
		name       string
		additional string
		expected   []string
	}{
		{
			"mix of IPs and hostnames",
			"192.168.1.1,ns1.example.com,2001:db8::1",
			[]string{"192.168.1.1", "ns1.example.com.", "2001:db8::1"},
		},
		{
			"single IP",
			"10.0.0.1",
			[]string{"10.0.0.1"},
		},
		{
			"single hostname",
			"ns1.example.com",
			[]string{"ns1.example.com."},
		},
		{
			"hostname already fqdn",
			"ns1.example.com.",
			[]string{"ns1.example.com."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &Options{additional: tt.additional}
			result := getAdditionalServers(opts)
			if len(result) != len(tt.expected) {
				t.Fatalf("getAdditionalServers() returned %d items, want %d",
					len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("getAdditionalServers()[%d] = %q, want %q",
						i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestGetRequests(t *testing.T) {
	tests := []struct {
		name       string
		nsNameList []string
		expected   int
	}{
		{
			"IPv4 addresses",
			[]string{"192.168.1.1", "10.0.0.1"},
			2,
		},
		{
			"IPv6 addresses",
			[]string{"2001:db8::1", "2001:db8::2"},
			2,
		},
		{
			"mixed IP addresses",
			[]string{"192.168.1.1", "2001:db8::1"},
			2,
		},
		{
			"empty list",
			[]string{},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &Options{}
			result := getRequests(tt.nsNameList, opts)
			if len(result) != tt.expected {
				t.Errorf("getRequests() returned %d requests, want %d",
					len(result), tt.expected)
			}
			// Verify each IP-based entry has correct nsip set
			for i, r := range result {
				if r.nsip == nil {
					t.Errorf("getRequests()[%d] has nil nsip", i)
				}
			}
		})
	}
}

func TestMilliSeconds(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected float64
	}{
		{"one second", time.Second, 1000.0},
		{"one millisecond", time.Millisecond, 1.0},
		{"500 microseconds", 500 * time.Microsecond, 0.5},
		{"zero", 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MilliSeconds(tt.duration)
			if result != tt.expected {
				t.Errorf("MilliSeconds(%v) = %f, want %f",
					tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatOutput(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		message        string
		expectError    string
	}{
		{"zero status no message", 0, "", ""},
		{"nonzero status with message", 1, "custom error", "custom error"},
		{"nonzero status empty message fills default", 2, "", StatusCode[2]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rn := NewRunner()
			opts := Options{}
			rn.formatOutput(tt.status, tt.message, opts)
			if rn.output.Status != tt.status {
				t.Errorf("output.Status = %d, want %d", rn.output.Status, tt.status)
			}
			if rn.output.Error != tt.expectError {
				t.Errorf("output.Error = %q, want %q", rn.output.Error, tt.expectError)
			}
		})
	}
}

// soaMockHandler returns a dns.Handler that responds to SOA queries
// with the given serial number
func soaMockHandler(serial uint32) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = []dns.RR{
			&dns.SOA{
				Hdr: dns.RR_Header{
					Name:   r.Question[0].Name,
					Rrtype: dns.TypeSOA,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns:     "ns1.example.com.",
				Mbox:   "admin.example.com.",
				Serial: serial,
			},
		}
		w.WriteMsg(m)
	})
}

// rcodeMockHandler returns a dns.Handler that responds with the given rcode
func rcodeMockHandler(rcode int) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, rcode)
		w.WriteMsg(m)
	})
}

// emptyAnswerMockHandler returns a dns.Handler that responds with
// success but no answer records
func emptyAnswerMockHandler() dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	})
}

func TestGetSerial(t *testing.T) {
	tests := []struct {
		name        string
		handler     dns.Handler
		wantSerial  uint32
		wantErr     bool
		errContains string
	}{
		{
			name:       "successful SOA response",
			handler:    soaMockHandler(2024010100),
			wantSerial: 2024010100,
			wantErr:    false,
		},
		{
			name:        "NXDOMAIN response",
			handler:     rcodeMockHandler(dns.RcodeNameError),
			wantErr:     true,
			errContains: "NXDOMAIN",
		},
		{
			name:        "SERVFAIL response",
			handler:     rcodeMockHandler(dns.RcodeServerFailure),
			wantErr:     true,
			errContains: "response code",
		},
		{
			name:        "no SOA in answer",
			handler:     emptyAnswerMockHandler(),
			wantErr:     true,
			errContains: "SOA record not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockDNSServer(t, tt.handler)
			defer server.close()
			<-server.ready

			host, port, _ := net.SplitHostPort(server.udpAddr)
			ip := net.ParseIP(host)

			opts := Options{
				Qopts: QueryOptions{
					timeout: 2 * time.Second,
					retries: 1,
					bufsize: defaultBufsize,
					port:    port,
				},
			}

			serial, _, _, err := getSerial("example.com.", ip, opts)
			if tt.wantErr {
				if err == nil {
					t.Error("getSerial() expected error, got nil")
				} else if tt.errContains != "" {
					if !contains(err.Error(), tt.errContains) {
						t.Errorf("getSerial() error = %q, want containing %q",
							err.Error(), tt.errContains)
					}
				}
			} else {
				if err != nil {
					t.Errorf("getSerial() unexpected error: %v", err)
				}
				if serial != tt.wantSerial {
					t.Errorf("getSerial() serial = %d, want %d",
						serial, tt.wantSerial)
				}
			}
		})
	}
}

func TestGetMasterSerial(t *testing.T) {
	t.Run("successful with IP address", func(t *testing.T) {
		server := newMockDNSServer(t, soaMockHandler(2024010100))
		defer server.close()
		<-server.ready

		host, port, _ := net.SplitHostPort(server.udpAddr)
		ip := net.ParseIP(host)

		rn := NewRunner()

		opts := Options{
			masterIP: ip,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		err := rn.getMasterSerial("example.com.", &opts)
		if err != nil {
			t.Fatalf("getMasterSerial() unexpected error: %v", err)
		}
		if opts.masterSerial != 2024010100 {
			t.Errorf("masterSerial = %d, want %d", opts.masterSerial, 2024010100)
		}
		if rn.output.Master == nil {
			t.Fatal("output.Master is nil")
		}
		if rn.output.Master.Serial != 2024010100 {
			t.Errorf("output.Master.Serial = %d, want %d",
				rn.output.Master.Serial, 2024010100)
		}
		if len(rn.serialList) != 1 || rn.serialList[0] != 2024010100 {
			t.Errorf("serialList = %v, want [2024010100]", rn.serialList)
		}
	})

	t.Run("error from unresponsive server", func(t *testing.T) {
		rn := NewRunner()

		opts := Options{
			masterIP: net.ParseIP("127.0.0.1"),
			Qopts: QueryOptions{
				timeout: 100 * time.Millisecond,
				retries: 1,
				bufsize: defaultBufsize,
				port:    "1", // unlikely to have a DNS server
			},
		}

		err := rn.getMasterSerial("example.com.", &opts)
		if err == nil {
			t.Error("getMasterSerial() expected error, got nil")
		}
	})

	t.Run("unresolvable master hostname", func(t *testing.T) {
		rn := NewRunner()

		opts := Options{
			masterName: "nonexistent.invalid.",
			Qopts: QueryOptions{
				timeout: 100 * time.Millisecond,
				retries: 1,
				bufsize: defaultBufsize,
			},
			resolvers: []net.IP{net.ParseIP("127.0.0.1")},
		}

		err := rn.getMasterSerial("example.com.", &opts)
		if err == nil {
			t.Error("getMasterSerial() expected error, got nil")
		}
		if err != nil && !contains(err.Error(), "couldn't resolve master name") {
			t.Errorf("getMasterSerial() error = %q, want containing 'couldn't resolve master name'",
				err.Error())
		}
	})
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
