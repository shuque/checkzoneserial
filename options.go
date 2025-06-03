package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/miekg/dns"
)

// Options - main options
type Options struct {
	Qopts        QueryOptions
	useV6        bool
	useV4        bool
	sortresponse bool
	json         bool
	resolvconf   string
	resolvers    []net.IP
	masterIP     net.IP
	masterName   string
	additional   string
	noqueryns    bool
	masterSerial uint32
	delta        int
}

// QueryOptions - query options
type QueryOptions struct {
	rdflag  bool
	adflag  bool
	cdflag  bool
	timeout time.Duration
	retries int
	tcp     bool
	bufsize uint16
	nsid    bool
	port    string
}

// Defaults
var (
	defaultTimeout     = 3
	defaultRetries     = 3
	defaultSerialDelta = 0
	defaultBufsize     = uint16(1400)
)

func doFlags() (string, Options) {

	var opts Options

	help := flag.Bool("h", false, "print help string")
	flag.BoolVar(&opts.useV6, "6", false, "use IPv6 only")
	flag.BoolVar(&opts.useV4, "4", false, "use IPv4 only")
	flag.BoolVar(&opts.sortresponse, "s", false, "sort responses")
	flag.BoolVar(&opts.json, "j", false, "output json")
	flag.BoolVar(&opts.Qopts.tcp, "c", false, "use IPv4 only")
	flag.StringVar(&opts.resolvconf, "cf", "", "use alternate resolv.conf file")
	master := flag.String("m", "", "master server name or address")
	flag.StringVar(&opts.additional, "a", "", "additional nameservers: n1,n2..")
	flag.BoolVar(&opts.noqueryns, "n", false, "don't query advertised nameservers")
	flag.IntVar(&opts.delta, "d", defaultSerialDelta, "allowed serial number drift")
	timeoutp := flag.Int("t", defaultTimeout, "query timeout in seconds")
	flag.IntVar(&opts.Qopts.retries, "r", defaultRetries, "number of query retries")
	var bufsize uint
	flag.UintVar(&bufsize, "b", uint(defaultBufsize), "buffer size for DNS messages")
	flag.BoolVar(&opts.Qopts.nsid, "nsid", false, "request NSID option in DNS queries")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s, version %s
Usage: %s [Options] <zone>

	Options:
	-h          Print this help string
	-4          Use IPv4 transport only
	-6          Use IPv6 transport only
	-cf file    Use alternate resolv.conf file
	-s          Print responses sorted by domain name and IP version
	-j          Produce json formatted output (implies -s)
	-c          Use TCP for queries (default: UDP with TCP on truncation)
	-t N        Query timeout value in seconds (default %d)
	-r N        Maximum # SOA query retries for each server (default %d)
	-d N        Allowed SOA serial number drift (default %d)
	-b N        Buffer size for DNS messages (default %d)
	-nsid       Request NSID option in DNS queries
	-m ns       Master server name/address to compare serial numbers with
	-a ns1,..   Specify additional nameserver names/addresses to query
	-n          Don't query advertised nameservers for the zone
`, progname, Version, progname, defaultTimeout, defaultRetries, defaultSerialDelta, defaultBufsize)
	}

	flag.Parse()
	opts.Qopts.timeout = time.Second * time.Duration(*timeoutp)
	opts.Qopts.bufsize = uint16(bufsize)

	if *help {
		flag.Usage()
		os.Exit(4)
	}

	if *master != "" {
		opts.masterIP = net.ParseIP(*master)
		if opts.masterIP == nil { // assume hostname
			opts.masterName = dns.Fqdn(*master)
		}
	}

	if opts.useV4 && opts.useV6 {
		fmt.Fprintf(os.Stderr, "Cannot specify both -4 and -6.\n")
		flag.Usage()
		os.Exit(4)
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Incorrect number of arguments.\n")
		flag.Usage()
		os.Exit(4)
	}
	args := flag.Args()
	return dns.Fqdn(args[0]), opts
}
