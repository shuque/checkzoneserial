package main

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestMakeQuery(t *testing.T) {
	qopts := QueryOptions{
		rdflag:  true,
		adflag:  true,
		cdflag:  true,
		timeout: 5,
		retries: 3,
		tcp:     false,
		bufsize: 4096,
		nsid:    true,
	}

	msg := MakeQuery("example.com", dns.TypeA, qopts)

	// Test basic message properties
	if msg.Id == 0 {
		t.Error("Expected non-zero message ID")
	}
	if !msg.RecursionDesired {
		t.Error("Expected RecursionDesired to be true")
	}
	if !msg.AuthenticatedData {
		t.Error("Expected AuthenticatedData to be true")
	}
	if !msg.CheckingDisabled {
		t.Error("Expected CheckingDisabled to be true")
	}

	// Test question
	if len(msg.Question) != 1 {
		t.Errorf("Expected 1 question, got %d", len(msg.Question))
	}
	if msg.Question[0].Name != "example.com" {
		t.Errorf("Expected question name 'example.com', got '%s'", msg.Question[0].Name)
	}
	if msg.Question[0].Qtype != dns.TypeA {
		t.Errorf("Expected question type A, got %d", msg.Question[0].Qtype)
	}
	if msg.Question[0].Qclass != dns.ClassINET {
		t.Errorf("Expected question class INET, got %d", msg.Question[0].Qclass)
	}

	// Test OPT record
	if len(msg.Extra) != 1 {
		t.Errorf("Expected 1 extra record, got %d", len(msg.Extra))
	}
	opt, ok := msg.Extra[0].(*dns.OPT)
	if !ok {
		t.Error("Expected OPT record in extra")
	}
	if opt.UDPSize() != 4096 {
		t.Errorf("Expected UDP size 4096, got %d", opt.UDPSize())
	}
	if len(opt.Option) != 1 {
		t.Errorf("Expected 1 option, got %d", len(opt.Option))
	}
	nsid, ok := opt.Option[0].(*dns.EDNS0_NSID)
	if !ok {
		t.Error("Expected NSID option")
	}
	if nsid.Code != dns.EDNS0NSID {
		t.Errorf("Expected NSID code %d, got %d", dns.EDNS0NSID, nsid.Code)
	}
}

func TestAddressString(t *testing.T) {
	tests := []struct {
		addr     string
		port     int
		expected string
	}{
		{"192.168.1.1", 53, "192.168.1.1:53"},
		{"2001:db8::1", 53, "[2001:db8::1]:53"},
		{"example.com", 53, "example.com:53"},
	}

	for _, test := range tests {
		result := AddressString(test.addr, test.port)
		if result != test.expected {
			t.Errorf("AddressString(%s, %d) = %s, want %s",
				test.addr, test.port, result, test.expected)
		}
	}
}

func TestMakeOptRR(t *testing.T) {
	tests := []struct {
		name     string
		qopts    QueryOptions
		expected struct {
			udpsize uint16
			hasNSID bool
		}
	}{
		{
			name: "default options",
			qopts: QueryOptions{
				bufsize: defaultBufsize,
				nsid:    false,
			},
			expected: struct {
				udpsize uint16
				hasNSID bool
			}{
				udpsize: defaultBufsize,
				hasNSID: false,
			},
		},
		{
			name: "custom options with NSID",
			qopts: QueryOptions{
				bufsize: 4096,
				nsid:    true,
			},
			expected: struct {
				udpsize uint16
				hasNSID bool
			}{
				udpsize: 4096,
				hasNSID: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opt := makeOptRR(test.qopts)

			if opt.UDPSize() != test.expected.udpsize {
				t.Errorf("Expected UDP size %d, got %d",
					test.expected.udpsize, opt.UDPSize())
			}

			hasNSID := false
			for _, o := range opt.Option {
				if _, ok := o.(*dns.EDNS0_NSID); ok {
					hasNSID = true
					break
				}
			}
			if hasNSID != test.expected.hasNSID {
				t.Errorf("Expected NSID %v, got %v",
					test.expected.hasNSID, hasNSID)
			}
		})
	}
}

// mockDNSServer is a simple DNS server for testing
type mockDNSServer struct {
	udpServer *dns.Server
	tcpServer *dns.Server
	udpAddr   string
	tcpAddr   string
	ready     chan struct{}
}

func newMockDNSServer(t *testing.T, handler dns.Handler) *mockDNSServer {
	udpServer := &dns.Server{
		Addr:    ":0",
		Net:     "udp",
		Handler: handler,
	}
	tcpServer := &dns.Server{
		Addr:    ":0",
		Net:     "tcp",
		Handler: handler,
	}

	ready := make(chan struct{})

	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			t.Errorf("Failed to start mock UDP DNS server: %v", err)
		}
	}()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			t.Errorf("Failed to start mock TCP DNS server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	udpAddr := udpServer.PacketConn.LocalAddr().String()
	tcpAddr := tcpServer.Listener.Addr().String()
	close(ready)

	return &mockDNSServer{
		udpServer: udpServer,
		tcpServer: tcpServer,
		udpAddr:   udpAddr,
		tcpAddr:   tcpAddr,
		ready:     ready,
	}
}

func (m *mockDNSServer) close() {
	if m.udpServer != nil {
		m.udpServer.Shutdown()
	}
	if m.tcpServer != nil {
		m.tcpServer.Shutdown()
	}
}

// TestSendQuery tests the SendQuery function with various scenarios
func TestSendQuery(t *testing.T) {
	tests := []struct {
		name      string
		qopts     QueryOptions
		response  *dns.Msg
		err       error
		truncated bool
		wantErr   bool
	}{
		{
			name: "successful UDP query",
			qopts: QueryOptions{
				tcp:     false,
				timeout: 2 * time.Second,
				retries: 1,
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Rcode: dns.RcodeSuccess,
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
						},
						A: net.ParseIP("192.168.1.1"),
					},
				},
			},
			err:       nil,
			truncated: false,
			wantErr:   false,
		},
		{
			name: "UDP query with truncation falls back to TCP",
			qopts: QueryOptions{
				tcp:     false,
				timeout: 2 * time.Second,
				retries: 1,
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Rcode: dns.RcodeSuccess,
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
						},
						A: net.ParseIP("192.168.1.1"),
					},
				},
			},
			err:       nil,
			truncated: true,
			wantErr:   false,
		},
		{
			name: "TCP query success",
			qopts: QueryOptions{
				tcp:     true,
				timeout: 2 * time.Second,
				retries: 1,
			},
			response: &dns.Msg{
				MsgHdr: dns.MsgHdr{
					Rcode: dns.RcodeSuccess,
				},
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{
							Name:   "example.com.",
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
						},
						A: net.ParseIP("192.168.1.1"),
					},
				},
			},
			err:       nil,
			truncated: false,
			wantErr:   false,
		},
		{
			name: "query error",
			qopts: QueryOptions{
				tcp:     false,
				timeout: 2 * time.Second,
				retries: 1,
			},
			response:  nil,
			err:       &net.DNSError{Err: "no such host", Name: "example.com", IsNotFound: true},
			truncated: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
				if tt.err != nil {
					// Simulate error by returning a DNS error response
					m := new(dns.Msg)
					m.SetRcode(r, dns.RcodeServerFailure)
					w.WriteMsg(m)
					return
				}

				m := new(dns.Msg)
				m.SetReply(r)
				if tt.response != nil {
					m.Answer = tt.response.Answer
					m.MsgHdr.Rcode = tt.response.MsgHdr.Rcode
					m.MsgHdr.Truncated = tt.truncated
				}
				w.WriteMsg(m)
			})

			server := newMockDNSServer(t, handler)
			defer server.close()
			<-server.ready

			// Use correct port for each protocol
			var host, port string
			var ipaddrs []net.IP
			if tt.qopts.tcp {
				host, port, _ = net.SplitHostPort(server.tcpAddr)
				ipaddrs = []net.IP{net.ParseIP(host)}
				tt.qopts.port = port
			} else {
				host, port, _ = net.SplitHostPort(server.udpAddr)
				ipaddrs = []net.IP{net.ParseIP(host)}
				tt.qopts.port = port
			}

			query := MakeQuery("example.com.", dns.TypeA, tt.qopts)

			if !tt.qopts.tcp {
				response, err := SendQueryUDP(query, ipaddrs, tt.qopts)
				if tt.truncated {
					if err != nil {
						t.Errorf("SendQueryUDP() error = %v, want nil", err)
					}
					if response == nil {
						t.Error("SendQueryUDP() returned nil response")
					}
					if response != nil && !response.MsgHdr.Truncated {
						t.Error("SendQueryUDP() response not truncated")
					}
				} else {
					if tt.wantErr {
						if err == nil && response != nil && response.Rcode != dns.RcodeSuccess {
							// treat non-success Rcode as error (expected)
						} else if err == nil {
							t.Errorf("SendQueryUDP() error = %v, wantErr %v", err, tt.wantErr)
						}
					} else {
						if err != nil {
							t.Errorf("SendQueryUDP() error = %v, wantErr %v", err, tt.wantErr)
						}
						if response == nil {
							t.Error("SendQueryUDP() returned nil response when no error expected")
						}
					}
				}
			}

			if tt.qopts.tcp || tt.truncated {
				// For TCP fallback, use the TCP port
				host, port, _ = net.SplitHostPort(server.tcpAddr)
				ipaddrs = []net.IP{net.ParseIP(host)}
				tt.qopts.port = port
				response, err := SendQueryTCP(query, ipaddrs, tt.qopts)
				if (err != nil) != tt.wantErr {
					t.Errorf("SendQueryTCP() error = %v, wantErr %v", err, tt.wantErr)
				}
				if !tt.wantErr && response == nil {
					t.Error("SendQueryTCP() returned nil response when no error expected")
				}
				if !tt.wantErr && response != nil && tt.response != nil {
					if response.MsgHdr.Rcode != tt.response.MsgHdr.Rcode {
						t.Errorf("SendQueryTCP() response code = %v, want %v",
							response.MsgHdr.Rcode, tt.response.MsgHdr.Rcode)
					}
				}
			}
		})
	}
}
