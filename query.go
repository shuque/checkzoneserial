package main

import (
	"net"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

// AddressString - compose address string for net functions
func AddressString(addr string, port int) string {
	if !strings.Contains(addr, ":") {
		return addr + ":" + strconv.Itoa(port)
	}
	return "[" + addr + "]" + ":" + strconv.Itoa(port)
}

// GetResolver - obtains system resolver addresses
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

// makeOptRR() - construct OPT Pseudo RR structure
func makeOptRR(qopts QueryOptions) *dns.OPT {

	opt := new(dns.OPT)
	opt.Hdr.Name = "."
	opt.Hdr.Rrtype = dns.TypeOPT
	opt.SetUDPSize(qopts.bufsize)

	if qopts.nsid {
		e := new(dns.EDNS0_NSID)
		e.Code = dns.EDNS0NSID
		e.Nsid = ""
		opt.Option = append(opt.Option, e)
	}

	opt.SetVersion(0)
	return opt
}

// MakeQuery - construct a DNS query message
func MakeQuery(qname string, qtype uint16, qopts QueryOptions) *dns.Msg {
	m := new(dns.Msg)
	m.Id = dns.Id()
	m.RecursionDesired = qopts.rdflag
	m.AuthenticatedData = qopts.adflag
	m.CheckingDisabled = qopts.cdflag
	m.Extra = append(m.Extra, makeOptRR(qopts))
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{Name: qname, Qtype: qtype, Qclass: dns.ClassINET}
	return m
}

// SendQueryUDP - send DNS query via UDP
func SendQueryUDP(query *dns.Msg, ipaddrs []net.IP, qopts QueryOptions) (response *dns.Msg, err error) {

	var retries = qopts.retries

	c := new(dns.Client)
	c.Net = "udp"
	c.Timeout = qopts.timeout

	for retries > 0 {
		for _, ipaddr := range ipaddrs {
			destination := AddressString(ipaddr.String(), 53)
			response, _, err = c.Exchange(query, destination)
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

// SendQueryTCP - send DNS query via TCP
func SendQueryTCP(query *dns.Msg, ipaddrs []net.IP, qopts QueryOptions) (response *dns.Msg, err error) {

	c := new(dns.Client)
	c.Net = "tcp"
	c.Timeout = qopts.timeout

	for _, ipaddr := range ipaddrs {
		destination := AddressString(ipaddr.String(), 53)
		response, _, err = c.Exchange(query, destination)
		if err == nil {
			return response, err
		}
	}
	return response, err
}

// SendQuery - send DNS query via UDP with fallback to TCP upon truncation
func SendQuery(qname string, qtype uint16, ipaddrs []net.IP, qopts QueryOptions) (*dns.Msg, error) {

	query := MakeQuery(qname, qtype, qopts)

	if qopts.tcp {
		return SendQueryTCP(query, ipaddrs, qopts)
	}

	response, err := SendQueryUDP(query, ipaddrs, qopts)
	if err == nil && response.MsgHdr.Truncated {
		return SendQueryTCP(query, ipaddrs, qopts)
	}

	return response, err

}
