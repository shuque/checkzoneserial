package main

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

//
// Options - query options
//
type Options struct {
	rdflag       bool
	adflag       bool
	cdflag       bool
	useV6        bool
	useV4        bool
	timeout      time.Duration
	retries      int
	resolver     net.IP
	master       net.IP
	additional   string
	masterSerial uint32
	delta        int
}

//
// AddressString - compose address string for net functions
//
func AddressString(addr string, port int) string {
	if strings.Index(addr, ":") == -1 {
		return addr + ":" + strconv.Itoa(port)
	}
	return "[" + addr + "]" + ":" + strconv.Itoa(port)
}

//
// GetResolver - obtain (1st) system default resolver address
//
func GetResolver() (resolver net.IP, err error) {
	config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err == nil {
		resolver = net.ParseIP(config.Servers[0])
	}
	return resolver, err
}

//
// MakeQuery - construct a DNS query MakeMessage
//
func MakeQuery(qname string, qtype uint16, opts Options) *dns.Msg {
	m := new(dns.Msg)
	m.Id = dns.Id()
	if opts.rdflag {
		m.RecursionDesired = true
	} else {
		m.RecursionDesired = false
	}
	if opts.adflag {
		m.AuthenticatedData = true
	} else {
		m.AuthenticatedData = false
	}
	if opts.cdflag {
		m.CheckingDisabled = true
	} else {
		m.CheckingDisabled = false
	}
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{Name: qname, Qtype: qtype, Qclass: dns.ClassINET}
	return m
}

//
// SendQueryUDP - send DNS query via UDP
//
func SendQueryUDP(qname string, qtype uint16, ipaddr net.IP, opts Options) (response *dns.Msg, err error) {

	var retries = opts.retries
	var timeout = opts.timeout
	destination := AddressString(ipaddr.String(), 53)

	m := MakeQuery(qname, qtype, opts)

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = timeout

	for retries > 0 {
		response, _, err = c.Exchange(m, destination)
		if err == nil {
			break
		}
		if nerr, ok := err.(net.Error); ok && !nerr.Timeout() {
			break
		}
		retries--
	}

	return response, err
}

//
// SendQueryTCP - send DNS query via TCP
//
func SendQueryTCP(qname string, qtype uint16, ipaddr net.IP, opts Options) (response *dns.Msg, err error) {

	destination := AddressString(ipaddr.String(), 53)

	m := MakeQuery(qname, qtype, opts)

	c := new(dns.Client)
	c.Net = "tcp"
	c.Timeout = opts.timeout

	response, _, err = c.Exchange(m, destination)
	return response, err

}

//
// SendQuery - send DNS query via UDP with fallback to TCP upon truncation
//
func SendQuery(qname string, qtype uint16, ipaddr net.IP, opts Options) (response *dns.Msg, err error) {

	response, err = SendQueryUDP(qname, qtype, ipaddr, opts)

	if err == nil && response.MsgHdr.Truncated {
		return SendQueryTCP(qname, qtype, ipaddr, opts)
	}

	return response, err

}
