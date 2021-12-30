package main

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

//
// QueryOptions - query options
//
type QueryOptions struct {
	rdflag  bool
	adflag  bool
	cdflag  bool
	timeout time.Duration
	retries int
	tcp     bool
}

//
// AddressString - compose address string for net functions
//
func AddressString(addr string, port int) string {
	if !strings.Contains(addr, ":") {
		return addr + ":" + strconv.Itoa(port)
	}
	return "[" + addr + "]" + ":" + strconv.Itoa(port)
}

//
// GetResolver - obtains system resolver addresses
//
func GetResolver(conffile string) (resolvers []net.IP, err error) {

	if conffile == "" {
		conffile = "/etc/resolv.conf"
	}

	config, err := dns.ClientConfigFromFile(conffile)
	if err != nil {
		return nil, err
	}
	for _, s := range config.Servers {
		ip := net.ParseIP(s)
		resolvers = append(resolvers, ip)
	}
	return resolvers, err
}

//
// MakeQuery - construct a DNS query MakeMessage
//
func MakeQuery(qname string, qtype uint16, qopts QueryOptions) *dns.Msg {
	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = qopts.rdflag
	m.AuthenticatedData = qopts.adflag
	m.CheckingDisabled = qopts.cdflag
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{Name: qname, Qtype: qtype, Qclass: dns.ClassINET}
	return m
}

//
// SendQueryUDP - send DNS query via UDP
//
func SendQueryUDP(qname string, qtype uint16, ipaddrs []net.IP, qopts QueryOptions) (response *dns.Msg, err error) {

	var retries = qopts.retries

	m := MakeQuery(qname, qtype, qopts)
	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = qopts.timeout

	for retries > 0 {
		for _, ipaddr := range ipaddrs {
			destination := AddressString(ipaddr.String(), 53)
			response, _, err = c.Exchange(m, destination)
			if err == nil {
				return response, err
			}
			if nerr, ok := err.(net.Error); ok && !nerr.Timeout() {
				break
			}
		}
		retries--
	}

	return response, err
}

//
// SendQueryTCP - send DNS query via TCP
//
func SendQueryTCP(qname string, qtype uint16, ipaddrs []net.IP, qopts QueryOptions) (response *dns.Msg, err error) {

	m := MakeQuery(qname, qtype, qopts)

	c := new(dns.Client)
	c.Net = "tcp"
	c.Timeout = qopts.timeout

	for _, ipaddr := range ipaddrs {
		destination := AddressString(ipaddr.String(), 53)
		response, _, err = c.Exchange(m, destination)
		if err == nil {
			return response, err
		}
	}
	return response, err
}

//
// SendQuery - send DNS query via UDP with fallback to TCP upon truncation
//
func SendQuery(qname string, qtype uint16, ipaddrs []net.IP, qopts QueryOptions) (*dns.Msg, error) {

	if qopts.tcp {
		return SendQueryTCP(qname, qtype, ipaddrs, qopts)
	}

	response, err := SendQueryUDP(qname, qtype, ipaddrs, qopts)
	if err == nil && response.MsgHdr.Truncated {
		return SendQueryTCP(qname, qtype, ipaddrs, qopts)
	}

	return response, err

}
