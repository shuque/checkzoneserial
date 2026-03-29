package main

import (
	"encoding/hex"
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
var Version = "1.1.2"
var progname = path.Base(os.Args[0])

// Status codes
var StatusCode = map[int]string{
	0: "",
	1: "serial mismatch or exceeds drift",
	2: "server issues",
	3: "master server error",
	4: "program invocation error",
}

// Concurrency limit
const numParallel = 20

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
	Nsid     string  `json:"nsid,omitempty"`
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
	Master    *Master    `json:"master,omitempty"`
	Responses []Response `json:"responses"`
}

// Runner holds all mutable state for a single program execution
type Runner struct {
	wg             sync.WaitGroup
	tokens         chan struct{}
	results        chan *Response
	output         Output
	serialList     []uint32
	ResponseByName map[string][]Response
}

// NewRunner creates a Runner with initialized channels and maps
func NewRunner() *Runner {
	return &Runner{
		tokens:         make(chan struct{}, numParallel),
		results:        make(chan *Response),
		ResponseByName: make(map[string][]Response),
	}
}

// serialDistance returns the unsigned distance between two serial numbers
// accounting for RFC 1982 serial number arithmetic (wrap at 2^32).
func serialDistance(s1, s2 uint32) uint32 {
	d := s1 - s2
	if d > (1 << 31) {
		return s2 - s1
	}
	return d
}

// serialDelta returns the signed difference (master - slave) using
// RFC 1982 arithmetic. Positive means the slave is behind the master.
func serialDelta(master, slave uint32) int {
	diff := master - slave
	if diff == 0 {
		return 0
	}
	if diff < (1 << 31) {
		return int(diff)
	}
	return -int(slave - master)
}

// maxSerialDrift returns the maximum pairwise serial distance
// across a list of serial numbers, using RFC 1982 arithmetic.
func maxSerialDrift(serials []uint32) uint32 {
	var maxDist uint32
	for i := 0; i < len(serials); i++ {
		for j := i + 1; j < len(serials); j++ {
			d := serialDistance(serials[i], serials[j])
			if d > maxDist {
				maxDist = d
			}
		}
	}
	return maxDist
}

func getIPAddresses(hostname string, rrtype uint16, opts Options) ([]net.IP, error) {

	var ipList []net.IP

	opts.Qopts.rdflag = true

	switch rrtype {
	case dns.TypeAAAA, dns.TypeA:
		response, err := SendQuery(hostname, rrtype, opts.resolvers, opts.Qopts)
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, fmt.Errorf("no response for %s %s", hostname, dns.TypeToString[rrtype])
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
		return nil, fmt.Errorf("getIPAddresses: %d: invalid rrtype", rrtype)
	}

	return ipList, nil
}

func getSerial(zone string, ip net.IP, opts Options) (serial uint32, took time.Duration, nsid string, err error) {

	var response *dns.Msg

	opts.Qopts.rdflag = false

	t0 := time.Now()
	response, err = SendQuery(zone, dns.TypeSOA, []net.IP{ip}, opts.Qopts)
	took = time.Since(t0)

	if err != nil {
		return serial, took, nsid, err
	}
	if response == nil {
		return serial, took, nsid, fmt.Errorf("no response from %s", ip.String())
	}
	switch response.MsgHdr.Rcode {
	case dns.RcodeSuccess:
		break
	case dns.RcodeNameError:
		return serial, took, nsid, fmt.Errorf("NXDOMAIN: %s: name doesn't exist", zone)
	default:
		return serial, took, nsid, fmt.Errorf("response code: %s",
			dns.RcodeToString[response.MsgHdr.Rcode])
	}

	ednsopt := response.IsEdns0()
	if ednsopt != nil {
		for _, o := range ednsopt.Option {
			switch o.(type) {
			case *dns.EDNS0_NSID:
				h, err := hex.DecodeString(o.String())
				if err != nil {
					nsid = o.String()
				} else {
					nsid = string(h)
				}
			}
		}
	}

	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeSOA {
			return rr.(*dns.SOA).Serial, took, nsid, nil
		}
	}

	return serial, took, nsid, fmt.Errorf("SOA record not found at %s",
		ip.String())
}

func (rn *Runner) getSerialAsync(zone string, ip net.IP, nsName string, opts Options) {

	defer rn.wg.Done()

	serial, resptime, nsid, err := getSerial(zone, ip, opts)
	<-rn.tokens // Release token

	r := new(Response)
	r.ip = ip
	r.Nsip = ip.String()
	r.Nsname = nsName
	r.Serial = serial
	r.Nsid = nsid
	r.resptime = resptime
	r.Resptime = MilliSeconds(resptime)
	if opts.masterIP != nil {
		delta := serialDelta(opts.masterSerial, serial)
		r.Delta = &delta
	}
	r.err = err
	if err != nil {
		r.Err = err.Error()
	}
	rn.results <- r
}

func getNSnames(zone string, opts *Options) ([]string, error) {

	var nsNameList []string

	opts.Qopts.rdflag = true
	response, err := SendQuery(zone, dns.TypeNS, opts.resolvers, opts.Qopts)
	if err != nil {
		return nil, err
	}
	if response.MsgHdr.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("%s response code: %s", zone, dns.RcodeToString[response.MsgHdr.Rcode])
	}
	for _, rr := range response.Answer {
		if rr.Header().Rrtype == dns.TypeNS {
			nsNameList = append(nsNameList, rr.(*dns.NS).Ns)
		}
	}
	if nsNameList == nil {
		return nil, fmt.Errorf("%s no nameserver records found", zone)
	}

	return nsNameList, nil
}

func getRequests(nsNameList []string, opts *Options) []*Request {

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
		if !opts.V4Only {
			ips, err := getIPAddresses(nsName, dns.TypeAAAA, *opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s AAAA lookup failed: %s\n", nsName, err)
			}
			aList = append(aList, ips...)
		}
		if !opts.V6Only {
			ips, err := getIPAddresses(nsName, dns.TypeA, *opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s A lookup failed: %s\n", nsName, err)
			}
			aList = append(aList, ips...)
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

func MilliSeconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000.0
}

func printSerialLine(isMaster bool, serial uint32, nsname string, nsip net.IP, elapsed time.Duration, nsid string, opts *Options) {
	if opts.json {
		return
	}

	if isMaster {
		fmt.Printf("%15d [%8s] %s %s %.2fms", serial, "MASTER",
			nsname, nsip, MilliSeconds(elapsed))
	} else {
		if opts.masterIP == nil {
			fmt.Printf("%15d %s %s %.2fms", serial, nsname, nsip, MilliSeconds(elapsed))
		} else {
			delta := serialDelta(opts.masterSerial, serial)
			fmt.Printf("%15d [%8d] %s %s %.2fms", serial, delta, nsname, nsip, MilliSeconds(elapsed))
		}
	}

	if opts.Qopts.nsid && nsid != "" {
		fmt.Printf(" %s\n", nsid)
	} else {
		fmt.Printf("\n")
	}
}

func getMasterAddress(name string, opts *Options) net.IP {
	// Try IPv6 if IPv4-only is not specified
	if !opts.V4Only {
		ipv6list, _ := getIPAddresses(name, dns.TypeAAAA, *opts)
		if len(ipv6list) > 0 {
			return ipv6list[0]
		}
	}

	// Try IPv4 if IPv6-only is not specified
	if !opts.V6Only {
		ipv4list, _ := getIPAddresses(name, dns.TypeA, *opts)
		if len(ipv4list) > 0 {
			return ipv4list[0]
		}
	}

	return nil
}

func (rn *Runner) getMasterSerial(zone string, opts *Options) error {

	var err error
	var took time.Duration
	var nsid string
	var master = new(Master)

	rn.output.Master = master

	if opts.masterIP == nil {
		master.Name = opts.masterName
		opts.masterIP = getMasterAddress(master.Name, opts)
		if opts.masterIP == nil {
			return fmt.Errorf("couldn't resolve master name: %s", master.Name)
		}
		master.IP = opts.masterIP.String()
	} else {
		opts.masterName = opts.masterIP.String()
		master.IP = opts.masterName
	}

	opts.masterSerial, took, nsid, err = getSerial(zone, opts.masterIP, *opts)

	if err != nil {
		master.Err = err.Error()
		return fmt.Errorf("%s %s: couldn't obtain serial: %s",
			opts.masterName, opts.masterIP, err.Error())
	}

	master.Serial = opts.masterSerial
	master.Resptime = MilliSeconds(took)
	rn.serialList = append(rn.serialList, opts.masterSerial)
	printSerialLine(true, opts.masterSerial, opts.masterName, opts.masterIP, took, nsid, opts)
	return nil
}

func printResult(r *Response, opts *Options) {

	if r.err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s %s: couldn't obtain serial: %s\n", r.Nsname, r.ip, r.err.Error())
		return
	}
	printSerialLine(false, r.Serial, r.Nsname, r.ip, r.resptime, r.Nsid, opts)
}

func getAdditionalServers(opts *Options) []string {

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

func (rn *Runner) formatOutput(status int, message string, opts Options) {

	rn.output.Status = status
	if status != 0 && message == "" {
		message = StatusCode[status]
	}
	rn.output.Error = message

	if opts.json {
		b, err := json.Marshal(rn.output)
		if err != nil {
			log.Fatal("error:", err)
		}
		fmt.Printf("%s\n", b)
	} else {
		if message != "" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", message)
		}
	}
}

func (rn *Runner) run(zone string, opts Options) (int, string) {

	var err error
	var rc int
	var nsNameList []string
	var requests []*Request

	opts.resolvers, err = GetResolver(opts.resolvconf)
	if err != nil {
		return 2, fmt.Sprintf("Error getting resolver: %s", err.Error())
	}

	if opts.additional != "" {
		nsNameList = getAdditionalServers(&opts)
	}
	if !opts.noqueryns {
		nsNames, err := getNSnames(zone, &opts)
		if err != nil {
			return 1, err.Error()
		}
		nsNameList = append(nsNameList, nsNames...)
	}
	requests = getRequests(nsNameList, &opts)

	opts.Qopts.rdflag = false

	timestamp := time.Now().Format("2006-01-02T15:04:05MST")
	if opts.json {
		rn.output.Zone = zone
		rn.output.Timestamp = timestamp
	} else {
		fmt.Printf("## %s %s\n", zone, timestamp)
	}

	if opts.masterIP != nil || opts.masterName != "" {
		if err := rn.getMasterSerial(zone, &opts); err != nil {
			return 3, err.Error()
		}
	}

	go func() {
		for _, x := range requests {
			rn.wg.Add(1)
			rn.tokens <- struct{}{}
			go rn.getSerialAsync(zone, x.nsip, x.nsname, opts)
		}
		rn.wg.Wait()
		close(rn.results)
	}()

	for r := range rn.results {
		rn.ResponseByName[r.Nsname] = append(rn.ResponseByName[r.Nsname], *r)
		if !opts.sortresponse && !opts.json {
			printResult(r, &opts)
		}
		if r.err != nil {
			rc = 2
		} else {
			rn.serialList = append(rn.serialList, r.Serial)
		}
	}

	if opts.sortresponse || opts.json {
		nsnameList := make([]string, 0, len(rn.ResponseByName))

		rn.output.Responses = make([]Response, 0, len(rn.ResponseByName))

		for k := range rn.ResponseByName {
			nsnameList = append(nsnameList, k)
		}
		sort.Sort(ByCanonicalOrder(nsnameList))
		for _, nsname := range nsnameList {
			responses := rn.ResponseByName[nsname]
			sort.Sort(ByIPversion(responses))
			for _, r := range responses {
				if !opts.json {
					printResult(&r, &opts)
				} else {
					rn.output.Responses = append(rn.output.Responses, r)
				}
			}
		}
	}

	if rn.serialList == nil {
		return 2, "ERROR: no SOA serials obtained."
	}

	if rc != 2 {
		if maxSerialDrift(rn.serialList) > uint32(opts.delta) {
			rc = 1
		}
	}
	return rc, ""
}

func main() {
	zone, opts, err := doFlags()
	if err != nil {
		os.Exit(4)
	}
	rn := NewRunner()
	status, message := rn.run(zone, opts)
	rn.formatOutput(status, message, opts)
	os.Exit(status)
}
