package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Version and Program name strings
var Version = "1.0.2"
var progname = path.Base(os.Args[0])

// Globals
var (
	serialList []uint32
)

//
// Request - request parameters
//
type Request struct {
	nsname string
	nsip   net.IP
}

//
// Response - response information
//
type Response struct {
	nsname string
	nsip   net.IP
	serial uint32
	took   time.Duration
	err    error
}

//
// map of Responses keyed by nameserver domain name
//
var ResponseByName = make(map[string][]Response)

// For goroutine communications and synchronization
var wg sync.WaitGroup
var numParallel uint16 = 20
var tokens = make(chan struct{}, int(numParallel))
var results = make(chan *Response)

func minmax(a []uint32) (min uint32, max uint32) {
	min = a[0]
	max = a[0]
	for _, value := range a {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	return min, max
}

func getIPAddresses(hostname string, rrtype uint16, opts Options) []net.IP {

	var ipList []net.IP

	opts.qopts.rdflag = true

	switch rrtype {
	case dns.TypeAAAA, dns.TypeA:
		response, err := SendQuery(hostname, rrtype, opts.resolvers, opts.qopts)
		if err != nil || response == nil {
			break
		}
		for _, rr := range response.Answer {
			if rr.Header().Rrtype == rrtype {
				if rrtype == dns.TypeAAAA {
					ipList = append(ipList, rr.(*dns.AAAA).AAAA)
				} else if rrtype == dns.TypeA {
					ipList = append(ipList, rr.(*dns.A).A)
				}
			}
		}
	default:
		fmt.Printf("getIPAddresses: %d: invalid rrtype\n", rrtype)
	}

	return ipList

}

func getSerial(zone string, ip net.IP, opts Options) (serial uint32, took time.Duration, err error) {

	var response *dns.Msg

	opts.qopts.rdflag = false

	t0 := time.Now()
	response, err = SendQuery(zone, dns.TypeSOA, []net.IP{ip}, opts.qopts)
	took = time.Since(t0)

	if err != nil {
		return serial, took, err
	}
	switch response.MsgHdr.Rcode {
	case dns.RcodeSuccess:
		break
	case dns.RcodeNameError:
		return serial, took, fmt.Errorf("NXDOMAIN: %s: name doesn't exist", zone)
	default:
		return serial, took, fmt.Errorf("response code: %s",
			dns.RcodeToString[response.MsgHdr.Rcode])
	}

	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeSOA {
			return rr.(*dns.SOA).Serial, took, nil
		}
	}

	return serial, took, fmt.Errorf("SOA record not found at %s",
		ip.String())
}

func getSerialAsync(zone string, ip net.IP, nsName string, opts Options) {

	defer wg.Done()

	serial, took, err := getSerial(zone, ip, opts)
	<-tokens // Release token

	r := new(Response)
	r.nsip = ip
	r.nsname = nsName
	r.serial = serial
	r.took = took
	r.err = err
	results <- r
}

func getNSnames(zone string, opts Options) []string {

	var nsNameList []string

	opts.qopts.rdflag = true
	response, err := SendQuery(zone, dns.TypeNS, opts.resolvers, opts.qopts)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if response.MsgHdr.Rcode == dns.RcodeNameError {
		fmt.Printf("Error: %s doesn't exist\n", zone)
		os.Exit(1)
	}
	if response.MsgHdr.Rcode != dns.RcodeSuccess {
		fmt.Printf("Error: %s response code: %s\n", zone, dns.RcodeToString[response.MsgHdr.Rcode])
		os.Exit(1)
	}
	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeNS {
			nsNameList = append(nsNameList, rr.(*dns.NS).Ns)
		}
	}
	if nsNameList == nil {
		fmt.Printf("Error: %s no nameserver records found\n", zone)
		os.Exit(1)
	}

	return nsNameList
}

func getRequests(nsNameList []string, opts Options) []*Request {

	var ip net.IP
	var aList []net.IP
	var requests []*Request
	var r *Request

	sort.Strings(nsNameList)

	for _, nsName := range nsNameList {
		ip = net.ParseIP(nsName)
		if ip != nil {
			r = new(Request)
			r.nsname = nsName
			r.nsip = ip
			requests = append(requests, r)
			continue
		}
		aList = make([]net.IP, 0)
		if !opts.useV4 {
			aList = append(aList,
				getIPAddresses(nsName, dns.TypeAAAA, opts)...)
		}
		if !opts.useV6 {
			aList = append(aList,
				getIPAddresses(nsName, dns.TypeA, opts)...)
		}
		for _, ip := range aList {
			r = new(Request)
			r.nsname = nsName
			r.nsip = ip
			requests = append(requests, r)
		}
	}

	return requests

}

func tookMilliSeconds(took time.Duration) float32 {
	return float32(took.Microseconds()) / 1000.0
}

func printMasterSerial(zone string, popts *Options) {

	var err error
	var took time.Duration

	if popts.masterIP == nil {
		ipv4list := getIPAddresses(popts.masterName, dns.TypeA, *popts)
		if ipv4list == nil {
			fmt.Printf("Error: couldn't resolve master name: %s\n", popts.masterName)
			os.Exit(3)
		}
		popts.masterIP = ipv4list[0]
	} else {
		popts.masterName = popts.masterIP.String()
	}

	popts.masterSerial, took, err = getSerial(zone, popts.masterIP, *popts)
	if err == nil {
		fmt.Printf("%15d [%8s] %s %s %.2fms\n", popts.masterSerial, "MASTER",
			popts.masterName, popts.masterIP, tookMilliSeconds(took))
		serialList = append(serialList, popts.masterSerial)
	} else {
		fmt.Printf("Error: %s %s: couldn't obtain serial: %s\n",
			popts.masterName, popts.masterIP, err.Error())
		os.Exit(3)
	}

}

func printResult(r *Response, opts Options) {

	if r.err == nil {
		if opts.masterIP != nil {
			delta := int(opts.masterSerial) - int(r.serial)
			fmt.Printf("%15d [%8d] %s %s %.2fms\n", r.serial, delta, r.nsname, r.nsip, tookMilliSeconds(r.took))
		} else {
			fmt.Printf("%15d %s %s %.2fms\n", r.serial, r.nsname, r.nsip, tookMilliSeconds(r.took))
		}
	} else {
		fmt.Printf("Error: %s %s: couldn't obtain serial: %s\n",
			r.nsname, r.nsip, r.err.Error())
	}
}

func getAdditionalServers(opts Options) []string {

	var s []string
	var ip net.IP

	s0 := strings.Split(opts.additional, ",")

	for _, x := range s0 {
		ip = net.ParseIP(x)
		if ip != nil {
			s = append(s, x)
		} else {
			s = append(s, dns.Fqdn(x))
		}
	}

	return s
}

func main() {

	var err error
	var rc int
	var nsNameList []string
	var requests []*Request

	zone, opts := doFlags()

	opts.resolvers, err = GetResolver(opts.resolvconf)
	if err != nil {
		fmt.Printf("Error getting resolver: %s\n", err.Error())
		os.Exit(1)
	}

	if opts.additional != "" {
		nsNameList = getAdditionalServers(opts)
	}
	if !opts.noqueryns {
		nsNameList = append(nsNameList, getNSnames(zone, opts)...)
	}
	requests = getRequests(nsNameList, opts)

	opts.qopts.rdflag = false

	fmt.Printf("## Zone: %s\n", zone)
	fmt.Println("## Time:", time.Now())
	if opts.masterIP != nil || opts.masterName != "" {
		printMasterSerial(zone, &opts)
	}

	go func() {
		for _, x := range requests {
			wg.Add(1)
			tokens <- struct{}{}
			go getSerialAsync(zone, x.nsip, x.nsname, opts)
		}
		wg.Wait()
		close(results)
	}()

	for r := range results {
		ResponseByName[r.nsname] = append(ResponseByName[r.nsname], *r)
		if !opts.sortresponse {
			printResult(r, opts)
		}
		if r.err != nil {
			rc = 2
		} else {
			serialList = append(serialList, r.serial)
		}
	}

	if opts.sortresponse {
		nsname_list := make([]string, 0, len(ResponseByName))

		for k := range ResponseByName {
			nsname_list = append(nsname_list, k)
		}
		sort.Sort(ByCanonicalOrder(nsname_list))
		for _, nsname := range nsname_list {
			responses := ResponseByName[nsname]
			sort.Sort(ByIPversion(responses))
			for _, r := range responses {
				printResult(&r, opts)
			}
		}
	}

	if serialList == nil {
		fmt.Printf("ERROR: no SOA serials obtained.\n")
		os.Exit(2)
	}

	if rc != 2 {
		min, max := minmax(serialList)
		if (max - min) > uint32(opts.delta) {
			rc = 1
		}
	}
	os.Exit(rc)
}
