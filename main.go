package main

import (
	"encoding/json"
	"fmt"
	"log"
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
var Version = "1.1.0"
var progname = path.Base(os.Args[0])

// Status codes
var StatusCode = map[int]string{
	0: "",
	1: "serial mismatch or exceeds drift",
	2: "server issues",
	3: "master server error",
	4: "program invocation error",
}

// Request - request parameters
type Request struct {
	nsname string
	nsip   net.IP
}

// Response
type Response struct {
	Nsname   string `json:"name"`
	ip       net.IP
	Nsip     string `json:"ip"`
	Serial   uint32 `json:"serial"`
	Delta    *int   `json:"delta,omitempty"`
	resptime time.Duration
	Resptime float64 `json:"resptime"`
	err      error
	Err      string `json:"error,omitempty"`
}

// Master Server
type Master struct {
	Name     string  `json:"name"`
	IP       string  `json:"ip"`
	Serial   uint32  `json:"serial"`
	Resptime float64 `json:"resptime"`
	Err      string  `json:"error,omitempty"`
}

// Output
type Output struct {
	Status    int        `json:"status"`
	Error     string     `json:"error,omitempty"`
	Zone      string     `json:"zone"`
	Timestamp string     `json:"timestamp"`
	Master    *Master    `json:"master"`
	Responses []Response `json:"responses"`
}

// For goroutine communications and synchronization
var (
	wg          sync.WaitGroup
	numParallel uint16 = 20
	tokens             = make(chan struct{}, int(numParallel))
	results            = make(chan *Response)
)

// Other globals
var (
	output     Output
	serialList []uint32
	// map of Responses keyed by nameserver domain name
	ResponseByName = make(map[string][]Response)
)

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

	serial, resptime, err := getSerial(zone, ip, opts)
	<-tokens // Release token

	r := new(Response)
	r.ip = ip
	r.Nsip = ip.String()
	r.Nsname = nsName
	r.Serial = serial
	r.resptime = resptime
	r.Resptime = resptime.Seconds() * 1000.0
	if opts.masterIP != nil {
		delta := int(opts.masterSerial) - int(serial)
		r.Delta = &delta
	}
	r.err = err
	if err != nil {
		r.Err = err.Error()
	}
	results <- r
}

func getNSnames(zone string, opts Options) []string {

	var nsNameList []string

	opts.qopts.rdflag = true
	response, err := SendQuery(zone, dns.TypeNS, opts.resolvers, opts.qopts)
	if err != nil {
		bailout(1, err.Error(), opts)
	}
	if response.MsgHdr.Rcode == dns.RcodeNameError {
		bailout(1,
			fmt.Sprintf("%s doesn't exist", zone),
			opts)
	}
	if response.MsgHdr.Rcode != dns.RcodeSuccess {
		bailout(1,
			fmt.Sprintf("%s response code: %s", zone, dns.RcodeToString[response.MsgHdr.Rcode]),
			opts)
	}
	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeNS {
			nsNameList = append(nsNameList, rr.(*dns.NS).Ns)
		}
	}
	if nsNameList == nil {
		bailout(1,
			fmt.Sprintf("%s no nameserver records found", zone),
			opts)
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

func MilliSeconds(duration time.Duration) float32 {
	return float32(duration.Microseconds()) / 1000.0
}

func getMasterSerial(zone string, popts *Options) {

	var err error
	var took time.Duration
	var master = new(Master)

	output.Master = master

	if popts.masterIP == nil {
		master.Name = popts.masterName
		ipv4list := getIPAddresses(master.Name, dns.TypeA, *popts)
		if ipv4list == nil {
			bailout(3,
				fmt.Sprintf("Error: couldn't resolve master name: %s", master.Name),
				*popts)
		}
		popts.masterIP = ipv4list[0]
		master.IP = popts.masterIP.String()
	} else {
		popts.masterName = popts.masterIP.String()
		master.IP = popts.masterName
	}

	popts.masterSerial, took, err = getSerial(zone, popts.masterIP, *popts)

	if err == nil {
		master.Serial = popts.masterSerial
		master.Resptime = took.Seconds() * 1000.0
		serialList = append(serialList, popts.masterSerial)
		if !popts.json {
			fmt.Printf("%15d [%8s] %s %s %.2fms\n", popts.masterSerial, "MASTER",
				popts.masterName, popts.masterIP, MilliSeconds(took))
		}
	} else {
		master.Err = err.Error()
		bailout(3,
			fmt.Sprintf("Error: %s %s: couldn't obtain serial: %s\n",
				popts.masterName, popts.masterIP, err.Error()),
			*popts)
	}
}

func printResult(r *Response, opts Options) {

	if r.err != nil {
		fmt.Printf("Error: %s %s: couldn't obtain serial: %s\n", r.Nsname, r.ip, r.err.Error())
		return
	}
	if opts.masterIP == nil {
		fmt.Printf("%15d %s %s %.2fms\n", r.Serial, r.Nsname, r.ip, MilliSeconds(r.resptime))
		return
	}
	fmt.Printf("%15d [%8d] %s %s %.2fms\n", r.Serial, *r.Delta, r.Nsname, r.ip, MilliSeconds(r.resptime))
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

func bailout(status int, message string, opts Options) {

	output.Status = status
	if status != 0 && message == "" {
		message = StatusCode[status]
	}
	output.Error = message

	if opts.json {
		b, err := json.Marshal(output)
		if err != nil {
			log.Fatal("error:", err)
		}
		fmt.Printf("%s\n", b)
	} else {
		if message != "" {
			fmt.Printf("Error: %s\n", message)
		}
	}

	os.Exit(status)
}

func main() {

	var err error
	var rc int
	var nsNameList []string
	var requests []*Request

	zone, opts := doFlags()

	opts.resolvers, err = GetResolver(opts.resolvconf)
	if err != nil {
		bailout(2, fmt.Sprintf("Error getting resolver: %s", err.Error()), opts)
	}

	if opts.additional != "" {
		nsNameList = getAdditionalServers(opts)
	}
	if !opts.noqueryns {
		nsNameList = append(nsNameList, getNSnames(zone, opts)...)
	}
	requests = getRequests(nsNameList, opts)

	opts.qopts.rdflag = false

	timestamp := time.Now().Format("2006-01-02T15:04:05MST")
	if opts.json {
		output.Zone = zone
		output.Timestamp = timestamp
	} else {
		fmt.Printf("## %s %s\n", zone, timestamp)
	}

	if opts.masterIP != nil || opts.masterName != "" {
		getMasterSerial(zone, &opts)
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
		ResponseByName[r.Nsname] = append(ResponseByName[r.Nsname], *r)
		if !opts.sortresponse && !opts.json {
			printResult(r, opts)
		}
		if r.err != nil {
			rc = 2
		} else {
			serialList = append(serialList, r.Serial)
		}
	}

	if opts.sortresponse || opts.json {
		nsname_list := make([]string, 0, len(ResponseByName))

		output.Responses = make([]Response, 0, len(ResponseByName))

		for k := range ResponseByName {
			nsname_list = append(nsname_list, k)
		}
		sort.Sort(ByCanonicalOrder(nsname_list))
		for _, nsname := range nsname_list {
			responses := ResponseByName[nsname]
			sort.Sort(ByIPversion(responses))
			for _, r := range responses {
				if !opts.json {
					printResult(&r, opts)
				} else {
					output.Responses = append(output.Responses, r)
				}
			}
		}
	}

	if serialList == nil {
		bailout(2, "ERROR: no SOA serials obtained.\n", opts)
	}

	if rc != 2 {
		min, max := minmax(serialList)
		if (max - min) > uint32(opts.delta) {
			rc = 1
		}
	}
	bailout(rc, "", opts)
}
