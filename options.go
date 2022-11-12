package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/miekg/dns"
)

//
// Options - query options
//
type Options struct {
	qopts        QueryOptions
	useV6        bool
	useV4        bool
	sortresponse bool
	resolvconf   string
	resolvers    []net.IP
	masterIP     net.IP
	masterName   string
	additional   string
	noqueryns    bool
	masterSerial uint32
	delta        int
}

// Defaults
var (
	defaultTimeout     = 3
	defaultRetries     = 3
	defaultSerialDelta = 0
)

func doFlags() (string, Options) {

	var opts Options

	help := flag.Bool("h", false, "print help string")
	flag.BoolVar(&opts.useV6, "6", false, "use IPv6 only")
	flag.BoolVar(&opts.useV4, "4", false, "use IPv4 only")
	flag.BoolVar(&opts.sortresponse, "s", false, "sort responses")
	flag.BoolVar(&opts.qopts.tcp, "c", false, "use IPv4 only")
	flag.StringVar(&opts.resolvconf, "cf", "", "use alternate resolv.conf file")
	master := flag.String("m", "", "primary server address")
	flag.StringVar(&opts.additional, "a", "", "additional nameservers: n1,n2..")
	flag.BoolVar(&opts.noqueryns, "n", false, "don't query advertised nameservers")
	flag.IntVar(&opts.delta, "d", defaultSerialDelta, "allowed serial number drift")
	timeoutp := flag.Int("t", defaultTimeout, "query timeout in seconds")
	flag.IntVar(&opts.qopts.retries, "r", defaultRetries, "number of query retries")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `%s, version %s
Usage: %s [Options] <zone>

	Options:
	-h          Print this help string
	-4          Use IPv4 transport only
	-6          Use IPv6 transport only
	-cf file    Use alternate resolv.conf file
	-s          Print responses sorted by domain name and IP version
	-c          Use TCP for queries (default: UDP with TCP on truncation)
	-t N        Query timeout value in seconds (default %d)
	-r N        Maximum # SOA query retries for each server (default %d)
	-d N        Allowed SOA serial number drift (default %d)
	-m ns       Master server name/address to compare serial numbers with
	-a ns1,..   Specify additional nameserver names/addresses to query
	-n          Don't query advertised nameservers for the zone
`, progname, Version, progname, defaultTimeout, defaultRetries, defaultSerialDelta)
	}

	flag.Parse()
	opts.qopts.timeout = time.Second * time.Duration(*timeoutp)

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
