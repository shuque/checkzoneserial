package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"sort"
	"sync"

	"github.com/miekg/dns"
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
	err    error
}

// For goroutine communications and synchronization
var wg sync.WaitGroup
var numParallel uint16 = 20
var tokens = make(chan struct{}, int(numParallel))
var results = make(chan *Response)

func getIPAddresses(hostname string, rrtype uint16, resolver net.IP, opts Options) []net.IP {

	var rrA *dns.A
	var rrAAAA *dns.AAAA
	var ipList []net.IP

	opts.rdflag = true

	switch rrtype {
	case dns.TypeAAAA:
		response, err := SendQuery(hostname, rrtype, resolver, opts)
		if err != nil || response == nil {
			break
		}
		for _, rr := range response.Answer {
			if rr.Header().Rrtype == dns.TypeAAAA {
				rrAAAA = rr.(*dns.AAAA)
				ipList = append(ipList, rrAAAA.AAAA)
			}
		}
	case dns.TypeA:
		response, err := SendQuery(hostname, rrtype, resolver, opts)
		if err != nil || response == nil {
			break
		}
		for _, rr := range response.Answer {
			if rr.Header().Rrtype == dns.TypeA {
				rrA = rr.(*dns.A)
				ipList = append(ipList, rrA.A)
			}
		}
	default:
		fmt.Printf("getIPAddresses: %d: invalid rrtype\n", rrtype)
	}

	return ipList

}

func getSerial(zone string, ip net.IP, opts Options) (serial uint32, err error) {

	var response *dns.Msg

	opts.rdflag = false

	response, err = SendQuery(zone, dns.TypeSOA, ip, opts)
	if err != nil {
		return serial, err
	}
	switch response.MsgHdr.Rcode {
	case dns.RcodeSuccess:
		break
	case dns.RcodeNameError:
		return serial, fmt.Errorf("NXDOMAIN: %s: name doesn't exist", zone)
	default:
		return serial, fmt.Errorf("Error: Response code: %s",
			dns.RcodeToString[response.MsgHdr.Rcode])
	}

	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeSOA {
			return rr.(*dns.SOA).Serial, nil
		}
	}

	return serial, fmt.Errorf("SOA record not found at %s",
		ip.String())
}

func getSerialAsync(zone string, ip net.IP, nsName string, opts Options) {

	defer wg.Done()

	serial, err := getSerial(zone, ip, opts)
	<-tokens // Release token

	r := new(Response)
	r.nsip = ip
	r.nsname = nsName
	r.serial = serial
	r.err = err
	results <- r
}

func getNSnames(zone string, opts Options) []string {

	var rrNS *dns.NS
	var nsNameList []string

	opts.rdflag = true
	response, err := SendQuery(zone, dns.TypeNS, opts.resolver, opts)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if response.MsgHdr.Rcode == dns.RcodeNameError {
		fmt.Printf("Error: %s doesn't exist\n", zone)
		os.Exit(1)
	}

	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeNS {
			rrNS = rr.(*dns.NS)
			nsNameList = append(nsNameList, rrNS.Ns)
		}
	}

	return nsNameList
}

func getRequests(nsNameList []string, opts Options) []*Request {

	var aList []net.IP
	var requests []*Request
	var r *Request

	sort.Strings(nsNameList)

	for _, nsName := range nsNameList {
		aList = make([]net.IP, 0)
		if !opts.useV4 {
			aList = append(aList,
				getIPAddresses(nsName, dns.TypeAAAA, opts.resolver, opts)...)
		}
		if !opts.useV6 {
			aList = append(aList,
				getIPAddresses(nsName, dns.TypeA, opts.resolver, opts)...)
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

func printMasterSerial(zone string, popts *Options) {

	var err error
	popts.masterSerial, err = getSerial(zone, popts.master, *popts)
	if err == nil {
		fmt.Printf("%15d [%9s] %s %s\n", popts.masterSerial, "MASTER",
			popts.master, popts.master)
	} else {
		fmt.Printf("Error: %s %s: couldn't obtain serial: %s\n",
			"MASTER", popts.master, err.Error())
		os.Exit(1)
	}

}

func printResult(r *Response, opts Options) {

	if r.err == nil {
		delta := opts.masterSerial - r.serial
		if opts.master != nil {
			fmt.Printf("%15d [%9d] %s %s\n", r.serial, delta, r.nsname, r.nsip)
		} else {
			fmt.Printf("%15d %s %s\n", r.serial, r.nsname, r.nsip)
		}
	} else {
		fmt.Printf("Error: %s %s: couldn't obtain serial: %s\n",
			r.nsname, r.nsip, r.err.Error())
	}

}

func doFlags(popts *Options) string {

	flag.BoolVar(&popts.useV6, "6", false, "use IPv6 only")
	flag.BoolVar(&popts.useV4, "4", false, "use IPv4 only")
	master := flag.String("m", "", "master server address")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <zone>\n", path.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if *master != "" {
		popts.master = net.ParseIP(*master)
		if popts.master == nil {
			fmt.Printf("Invalid master address: %s\n", *master)
			os.Exit(1)
		}
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Printf("Error: bad usage\n")
		flag.Usage()
		os.Exit(1)
	}

	return args[0]
}

func main() {

	var err error
	var opts Options
	var nsNameList []string
	var requests []*Request

	opts = GetQueryOpts()
	zone := dns.Fqdn(doFlags(&opts))

	opts.resolver, err = GetResolver()
	if err != nil {
		fmt.Printf("Error getting resolver: %s\n", err.Error())
		os.Exit(1)
	}

	nsNameList = getNSnames(zone, opts)
	requests = getRequests(nsNameList, opts)

	opts.rdflag = false

	if opts.master != nil {
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
		printResult(r, opts)
	}

}
