package main

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"sync"
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

// newSOAServer creates a mock DNS server returning the given serial for SOA queries
// and returns the server, its IP, and port.
func newSOAServer(t *testing.T, serial uint32) (*mockDNSServer, string, string) {
	server := newMockDNSServer(t, soaMockHandler(serial))
	<-server.ready
	host, port, _ := net.SplitHostPort(server.udpAddr)
	return server, host, port
}

// alternatingSOAHandler returns a handler where the first SOA query returns
// masterSerial and all subsequent queries return slaveSerial.
func alternatingSOAHandler(masterSerial, slaveSerial uint32) dns.Handler {
	var mu sync.Mutex
	first := true
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		mu.Lock()
		serial := slaveSerial
		if first {
			serial = masterSerial
			first = false
		}
		mu.Unlock()

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

func TestRun(t *testing.T) {
	t.Run("matching serials returns 0", func(t *testing.T) {
		server, host, port := newSOAServer(t, 2024010100)
		defer server.close()

		rn := NewRunner()
		opts := Options{
			noqueryns:  true,
			additional: host,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		status, message := rn.run("example.com.", opts)
		if status != 0 {
			t.Errorf("run() status = %d, want 0; message = %q", status, message)
		}
	})

	t.Run("matching serials with master returns 0", func(t *testing.T) {
		server, host, port := newSOAServer(t, 2024010100)
		defer server.close()

		rn := NewRunner()
		opts := Options{
			noqueryns:  true,
			additional: host,
			masterIP:   net.ParseIP(host),
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		status, message := rn.run("example.com.", opts)
		if status != 0 {
			t.Errorf("run() status = %d, want 0; message = %q", status, message)
		}
		if rn.output.Master == nil {
			t.Fatal("output.Master is nil")
		}
		if rn.output.Master.Serial != 2024010100 {
			t.Errorf("output.Master.Serial = %d, want 2024010100",
				rn.output.Master.Serial)
		}
	})

	t.Run("differing serials returns 1", func(t *testing.T) {
		// First query (master) gets 2024010100, subsequent (slave) gets 2024010105
		server := newMockDNSServer(t, alternatingSOAHandler(2024010100, 2024010105))
		defer server.close()
		<-server.ready
		host, port, _ := net.SplitHostPort(server.udpAddr)

		rn := NewRunner()
		opts := Options{
			noqueryns:  true,
			additional: host,
			masterIP:   net.ParseIP(host),
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		status, _ := rn.run("example.com.", opts)
		if status != 1 {
			t.Errorf("run() status = %d, want 1", status)
		}
	})

	t.Run("differing serials within drift returns 0", func(t *testing.T) {
		server := newMockDNSServer(t, alternatingSOAHandler(2024010100, 2024010103))
		defer server.close()
		<-server.ready
		host, port, _ := net.SplitHostPort(server.udpAddr)

		rn := NewRunner()
		opts := Options{
			noqueryns:  true,
			additional: host,
			masterIP:   net.ParseIP(host),
			delta:      5,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		status, message := rn.run("example.com.", opts)
		if status != 0 {
			t.Errorf("run() status = %d, want 0; message = %q", status, message)
		}
	})

	t.Run("master failure returns 3", func(t *testing.T) {
		rn := NewRunner()
		opts := Options{
			noqueryns:  true,
			additional: "127.0.0.1",
			masterIP:   net.ParseIP("127.0.0.1"),
			Qopts: QueryOptions{
				timeout: 100 * time.Millisecond,
				retries: 1,
				bufsize: defaultBufsize,
				port:    "1", // unlikely to have a DNS server
			},
		}

		status, _ := rn.run("example.com.", opts)
		if status != 3 {
			t.Errorf("run() status = %d, want 3", status)
		}
	})

	t.Run("no servers returns 2", func(t *testing.T) {
		rn := NewRunner()
		opts := Options{
			noqueryns: true,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
			},
		}

		status, message := rn.run("example.com.", opts)
		if status != 2 {
			t.Errorf("run() status = %d, want 2; message = %q", status, message)
		}
	})

	t.Run("differing serials error message reports allowed drift", func(t *testing.T) {
		// master=2024010100, slaves=2024010105 -> drift 5, exceeds allowed 2
		server := newMockDNSServer(t, alternatingSOAHandler(2024010100, 2024010105))
		defer server.close()
		<-server.ready
		host, port, _ := net.SplitHostPort(server.udpAddr)

		rn := NewRunner()
		opts := Options{
			noqueryns:  true,
			additional: host,
			masterIP:   net.ParseIP(host),
			delta:      2,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		status, message := rn.run("example.com.", opts)
		if status != 1 {
			t.Fatalf("run() status = %d, want 1; message = %q", status, message)
		}
		// The message must report the configured (allowed) drift threshold.
		if !contains(message, "exceeds allowed drift (2)") {
			t.Errorf("run() message = %q, want containing %q", message, "exceeds allowed drift (2)")
		}
	})
}

// nsMockHandler returns a dns.Handler that answers NS queries with the
// given nameserver names.
func nsMockHandler(nsNames ...string) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for _, ns := range nsNames {
			m.Answer = append(m.Answer, &dns.NS{
				Hdr: dns.RR_Header{
					Name:   r.Question[0].Name,
					Rrtype: dns.TypeNS,
					Class:  dns.ClassINET,
					Ttl:    3600,
				},
				Ns: ns,
			})
		}
		w.WriteMsg(m)
	})
}

func TestGetNSnames(t *testing.T) {
	tests := []struct {
		name        string
		handler     dns.Handler
		wantNames   []string
		wantErr     bool
		errContains string
	}{
		{
			name:      "successful NS response",
			handler:   nsMockHandler("ns1.example.com.", "ns2.example.com."),
			wantNames: []string{"ns1.example.com.", "ns2.example.com."},
		},
		{
			name:        "SERVFAIL response",
			handler:     rcodeMockHandler(dns.RcodeServerFailure),
			wantErr:     true,
			errContains: "response code",
		},
		{
			name:        "success but no NS records",
			handler:     emptyAnswerMockHandler(),
			wantErr:     true,
			errContains: "no nameserver records found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockDNSServer(t, tt.handler)
			defer server.close()
			<-server.ready

			host, port, _ := net.SplitHostPort(server.udpAddr)

			opts := &Options{
				resolvers: []net.IP{net.ParseIP(host)},
				Qopts: QueryOptions{
					timeout: 2 * time.Second,
					retries: 1,
					bufsize: defaultBufsize,
					port:    port,
				},
			}

			names, err := getNSnames("example.com.", opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("getNSnames() expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("getNSnames() error = %q, want containing %q",
						err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("getNSnames() unexpected error: %v", err)
			}
			if len(names) != len(tt.wantNames) {
				t.Fatalf("getNSnames() returned %d names, want %d: %v",
					len(names), len(tt.wantNames), names)
			}
			// NS answer order is not guaranteed; compare as a set.
			got := make(map[string]bool)
			for _, n := range names {
				got[n] = true
			}
			for _, want := range tt.wantNames {
				if !got[want] {
					t.Errorf("getNSnames() missing expected name %q; got %v",
						want, names)
				}
			}
		})
	}
}

// captureStdout redirects os.Stdout while fn runs and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestRunJSONOutput(t *testing.T) {
	t.Run("with master includes delta", func(t *testing.T) {
		server, host, port := newSOAServer(t, 2024010100)
		defer server.close()

		rn := NewRunner()
		opts := Options{
			noqueryns:    true,
			additional:   host,
			masterIP:     net.ParseIP(host),
			json:         true,
			sortresponse: true,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		var status int
		var message string
		out := captureStdout(t, func() {
			status, message = rn.run("example.com.", opts)
			rn.formatOutput(status, message, opts)
		})
		if status != 0 {
			t.Fatalf("run() status = %d, want 0; message = %q", status, message)
		}

		var parsed Output
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
		}
		if parsed.Status != 0 {
			t.Errorf("json status = %d, want 0", parsed.Status)
		}
		if parsed.Zone != "example.com." {
			t.Errorf("json zone = %q, want example.com.", parsed.Zone)
		}
		if parsed.Timestamp == "" {
			t.Error("json timestamp is empty")
		}
		if parsed.Master == nil {
			t.Fatal("json master is nil")
		}
		if parsed.Master.Serial != 2024010100 {
			t.Errorf("json master serial = %d, want 2024010100", parsed.Master.Serial)
		}
		if len(parsed.Responses) == 0 {
			t.Fatal("json responses is empty")
		}
		r0 := parsed.Responses[0]
		if r0.Serial != 2024010100 {
			t.Errorf("json response serial = %d, want 2024010100", r0.Serial)
		}
		if r0.Nsip == "" {
			t.Error("json response ip is empty")
		}
		if r0.Delta == nil {
			t.Error("json response delta is nil, want present when a master is set")
		}
	})

	t.Run("without master omits delta and error fields", func(t *testing.T) {
		server, host, port := newSOAServer(t, 2024010100)
		defer server.close()

		rn := NewRunner()
		opts := Options{
			noqueryns:    true,
			additional:   host,
			json:         true,
			sortresponse: true,
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}

		var status int
		var message string
		out := captureStdout(t, func() {
			status, message = rn.run("example.com.", opts)
			rn.formatOutput(status, message, opts)
		})
		if status != 0 {
			t.Fatalf("run() status = %d, want 0; message = %q", status, message)
		}

		// omitempty: no delta without a master, no error field on success.
		if contains(out, "\"delta\"") {
			t.Errorf("json unexpectedly contains delta without a master: %s", out)
		}
		if contains(out, "\"error\"") {
			t.Errorf("json unexpectedly contains error field on success: %s", out)
		}

		var parsed Output
		if err := json.Unmarshal([]byte(out), &parsed); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
		}
		if parsed.Master != nil {
			t.Errorf("json master should be nil without -m, got %+v", parsed.Master)
		}
	})
}

// addressMockHandler answers A queries with aRecords and AAAA queries with
// aaaaRecords, so a single mock resolver can serve both address families.
func addressMockHandler(aRecords, aaaaRecords []string) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		switch r.Question[0].Qtype {
		case dns.TypeA:
			for _, a := range aRecords {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					A: net.ParseIP(a),
				})
			}
		case dns.TypeAAAA:
			for _, aaaa := range aaaaRecords {
				m.Answer = append(m.Answer, &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   r.Question[0].Name,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    3600,
					},
					AAAA: net.ParseIP(aaaa),
				})
			}
		}
		w.WriteMsg(m)
	})
}

func TestGetIPAddresses(t *testing.T) {
	server := newMockDNSServer(t, addressMockHandler(
		[]string{"192.0.2.1", "192.0.2.2"}, []string{"2001:db8::1"}))
	defer server.close()
	<-server.ready

	host, port, _ := net.SplitHostPort(server.udpAddr)
	opts := Options{
		resolvers: []net.IP{net.ParseIP(host)},
		Qopts: QueryOptions{
			timeout: 2 * time.Second,
			retries: 1,
			bufsize: defaultBufsize,
			port:    port,
		},
	}

	t.Run("A records", func(t *testing.T) {
		ips, err := getIPAddresses("ns1.example.com.", dns.TypeA, opts)
		if err != nil {
			t.Fatalf("getIPAddresses() unexpected error: %v", err)
		}
		if len(ips) != 2 {
			t.Fatalf("getIPAddresses() returned %d addresses, want 2: %v", len(ips), ips)
		}
		for _, ip := range ips {
			if ip.To4() == nil {
				t.Errorf("getIPAddresses(A) returned non-IPv4 address %v", ip)
			}
		}
	})

	t.Run("AAAA records", func(t *testing.T) {
		ips, err := getIPAddresses("ns1.example.com.", dns.TypeAAAA, opts)
		if err != nil {
			t.Fatalf("getIPAddresses() unexpected error: %v", err)
		}
		if len(ips) != 1 {
			t.Fatalf("getIPAddresses() returned %d addresses, want 1: %v", len(ips), ips)
		}
		if ips[0].To4() != nil {
			t.Errorf("getIPAddresses(AAAA) returned non-IPv6 address %v", ips[0])
		}
	})

	t.Run("invalid rrtype", func(t *testing.T) {
		_, err := getIPAddresses("ns1.example.com.", dns.TypeMX, opts)
		if err == nil {
			t.Fatal("getIPAddresses() expected error for invalid rrtype, got nil")
		}
		if !contains(err.Error(), "invalid rrtype") {
			t.Errorf("getIPAddresses() error = %q, want containing 'invalid rrtype'", err.Error())
		}
	})
}

func TestGetRequestsHostnameResolution(t *testing.T) {
	server := newMockDNSServer(t, addressMockHandler(
		[]string{"192.0.2.1"}, []string{"2001:db8::1"}))
	defer server.close()
	<-server.ready

	host, port, _ := net.SplitHostPort(server.udpAddr)
	newOpts := func() *Options {
		return &Options{
			resolvers: []net.IP{net.ParseIP(host)},
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}
	}

	t.Run("both families resolved", func(t *testing.T) {
		reqs := getRequests([]string{"ns1.example.com."}, newOpts())
		if len(reqs) != 2 {
			t.Fatalf("getRequests() returned %d requests, want 2 (one A, one AAAA)", len(reqs))
		}
		var haveV4, haveV6 bool
		for _, r := range reqs {
			if r.nsname != "ns1.example.com." {
				t.Errorf("request nsname = %q, want ns1.example.com.", r.nsname)
			}
			if r.nsip == nil {
				t.Fatal("request has nil nsip")
			}
			if r.nsip.To4() != nil {
				haveV4 = true
			} else {
				haveV6 = true
			}
		}
		if !haveV4 || !haveV6 {
			t.Errorf("getRequests() families: haveV4=%v haveV6=%v, want both", haveV4, haveV6)
		}
	})

	t.Run("V4Only skips AAAA lookup", func(t *testing.T) {
		opts := newOpts()
		opts.V4Only = true
		reqs := getRequests([]string{"ns1.example.com."}, opts)
		if len(reqs) != 1 {
			t.Fatalf("getRequests() returned %d requests, want 1", len(reqs))
		}
		if reqs[0].nsip.To4() == nil {
			t.Errorf("V4Only request nsip = %v, want IPv4", reqs[0].nsip)
		}
	})

	t.Run("V6Only skips A lookup", func(t *testing.T) {
		opts := newOpts()
		opts.V6Only = true
		reqs := getRequests([]string{"ns1.example.com."}, opts)
		if len(reqs) != 1 {
			t.Fatalf("getRequests() returned %d requests, want 1", len(reqs))
		}
		if reqs[0].nsip.To4() != nil {
			t.Errorf("V6Only request nsip = %v, want IPv6", reqs[0].nsip)
		}
	})
}

// nsidSOAHandler answers SOA queries with the given serial and attaches an
// OPT record carrying an EDNS0_NSID option whose value is nsidHex (a hex
// string, as required on the wire).
func nsidSOAHandler(serial uint32, nsidHex string) dns.Handler {
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
		opt := new(dns.OPT)
		opt.Hdr.Name = "."
		opt.Hdr.Rrtype = dns.TypeOPT
		opt.SetUDPSize(defaultBufsize)
		opt.Option = append(opt.Option, &dns.EDNS0_NSID{
			Code: dns.EDNS0NSID,
			Nsid: nsidHex,
		})
		m.Extra = append(m.Extra, opt)
		w.WriteMsg(m)
	})
}

func TestGetSerialNSID(t *testing.T) {
	nsidHex := hex.EncodeToString([]byte("ns-east-1"))
	server := newMockDNSServer(t, nsidSOAHandler(2024010100, nsidHex))
	defer server.close()
	<-server.ready

	host, port, _ := net.SplitHostPort(server.udpAddr)
	ip := net.ParseIP(host)

	opts := Options{
		Qopts: QueryOptions{
			timeout: 2 * time.Second,
			retries: 1,
			bufsize: defaultBufsize,
			nsid:    true,
			port:    port,
		},
	}

	serial, _, nsid, err := getSerial("example.com.", ip, opts)
	if err != nil {
		t.Fatalf("getSerial() unexpected error: %v", err)
	}
	if serial != 2024010100 {
		t.Errorf("getSerial() serial = %d, want 2024010100", serial)
	}
	// The hex-encoded NSID must be decoded back to its human-readable form.
	if nsid != "ns-east-1" {
		t.Errorf("getSerial() nsid = %q, want %q", nsid, "ns-east-1")
	}
}

func TestGetMasterAddress(t *testing.T) {
	server := newMockDNSServer(t, addressMockHandler(
		[]string{"192.0.2.1"}, []string{"2001:db8::1"}))
	defer server.close()
	<-server.ready

	host, port, _ := net.SplitHostPort(server.udpAddr)
	newOpts := func() *Options {
		return &Options{
			resolvers: []net.IP{net.ParseIP(host)},
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    port,
			},
		}
	}

	t.Run("prefers IPv6 when both families allowed", func(t *testing.T) {
		ip := getMasterAddress("master.example.com.", newOpts())
		if ip == nil {
			t.Fatal("getMasterAddress() returned nil, want an address")
		}
		if ip.To4() != nil {
			t.Errorf("getMasterAddress() = %v, want IPv6 (preferred)", ip)
		}
	})

	t.Run("V4Only returns IPv4", func(t *testing.T) {
		opts := newOpts()
		opts.V4Only = true
		ip := getMasterAddress("master.example.com.", opts)
		if ip == nil {
			t.Fatal("getMasterAddress() returned nil, want an address")
		}
		if ip.To4() == nil {
			t.Errorf("getMasterAddress() = %v, want IPv4", ip)
		}
	})

	t.Run("nil when no address found", func(t *testing.T) {
		// A resolver that answers with no address records for any family.
		empty := newMockDNSServer(t, emptyAnswerMockHandler())
		defer empty.close()
		<-empty.ready
		ehost, eport, _ := net.SplitHostPort(empty.udpAddr)

		opts := &Options{
			resolvers: []net.IP{net.ParseIP(ehost)},
			Qopts: QueryOptions{
				timeout: 2 * time.Second,
				retries: 1,
				bufsize: defaultBufsize,
				port:    eport,
			},
		}
		if ip := getMasterAddress("master.example.com.", opts); ip != nil {
			t.Errorf("getMasterAddress() = %v, want nil when no address found", ip)
		}
	})
}

func TestPrintSerialLineNSID(t *testing.T) {
	opts := &Options{Qopts: QueryOptions{nsid: true}}
	out := captureStdout(t, func() {
		printSerialLine(false, 2024010100, "ns1.example.com.",
			net.ParseIP("192.0.2.1"), 5*time.Millisecond, "ns-east-1", opts)
	})
	if !contains(out, "ns-east-1") {
		t.Errorf("printSerialLine() output = %q, want it to include the NSID", out)
	}
}

