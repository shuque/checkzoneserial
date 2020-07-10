# checkzoneserial
Check zone serial numbers across servers

This program queries a zone's SOA record at all its authoritative
servers and reports the serial number seen. If a master server is
specified (with the -m option) it will also compute the difference
in the serial numbers seen with the master serial number.

The master, if provided, is queried first. Then all the authoritative
servers are queried in parallel.

### Pre-requisites

* Go
* Miek Gieben's Go dns package: https://github.com/miekg/dns

### Building

Just run 'go build'. This will generate the executable 'checkzoneserial'.

### Usage

```
$ checkzoneserial -h
checkzoneserial, version 1.0.1
Usage: checkzoneserial [Options] <zone>

        Options:
        -h          Print this help string
        -4          Use IPv4 transport only
        -6          Use IPv6 transport only
        -cf file    Use alternate resolv.conf file
        -c          Use TCP for queries (default: UDP with TCP on truncation)
        -t N        Query timeout value in seconds (default 3)
        -r N        Maximum # SOA query retries for each server (default 3)
        -d N        Allowed SOA serial number drift (default 0)
        -m ns       Master server name/address to compare serial numbers with
        -a ns1,..   Specify additional nameserver names/addresses to query
        -n          Don't query advertised nameservers for the zone
```

### Return codes

* 0 on success
* 1 if serials are identical or do not differ by more than allowed drift
* 2 on detection of server issues (timeout, bad response, etc)
* 3 if the master server (if specified) fails to respond
* 4 on program invocation error


### Example runs

Report zone serials for all authoritative servers for upenn.edu:

```
$ checkzoneserial upenn.edu
Zone: upenn.edu.
     1007401858 adns3.upenn.edu. 128.91.251.33
     1007401858 adns1.upenn.edu. 128.91.3.128
     1007401858 dns1.udel.edu. 128.175.13.16
     1007401858 dns2.udel.edu. 128.175.13.17
     1007401858 adns2.upenn.edu. 128.91.254.22
     1007401858 adns3.upenn.edu. 2607:f470:1003::3:c
     1007401858 adns1.upenn.edu. 2607:f470:1001::1:a
     1007401858 adns2.upenn.edu. 2607:f470:1002::2:3
     1007401858 sns-pb.isc.org. 192.5.4.1
     1007401858 sns-pb.isc.org. 2001:500:2e::1
$ echo $?
0
```

Report zone serials for siteforce.com servers, compare them to the
master server 10.11.12.13 (-m option) and report the deltas.

```
$ checkzoneserial -m 10.11.12.13 siteforce.com
Zone: siteforce.com
     2019120538 [   MASTER] 10.11.12.13 10.11.12.13
     2019120538 [        0] udns1.salesforce.com. 2001:502:2eda::8
     2019120537 [        1] pch1.salesforce-dns.com. 206.223.122.1
     2019120538 [        0] pch1.salesforce-dns.com. 2620:171:809::1
     2019120536 [        2] udns2.salesforce.com. 2001:502:ad09::8
     2019120538 [        0] udns4.salesforce.com. 156.154.103.8
     2019120538 [        0] udns1.salesforce.com. 156.154.100.8
     2019120538 [        0] udns2.salesforce.com. 156.154.101.8
     2019120536 [        2] udns3.salesforce.com. 156.154.102.8
     2019120538 [        0] udns4.salesforce.com. 2610:a1:1010::8
     2019120536 [        2] udns3.salesforce.com. 2610:a1:1009::8
$ echo $?
1
```

The same as the last run, but only check the IPv6 addresses of the
servers. Since all the serials are the same, the exit code is 0.

```
$ checkzoneserial -m 10.11.12.13 -6 siteforce.com
Zone: siteforce.com
     2019120538 [   MASTER] 10.11.12.13 10.11.12.13
     2019120538 [        0] pch1.salesforce-dns.com. 2620:171:809::1
     2019120538 [        0] udns1.salesforce.com. 2001:502:2eda::8
     2019120538 [        0] udns2.salesforce.com. 2001:502:ad09::8
     2019120538 [        0] udns3.salesforce.com. 2610:a1:1009::8
     2019120538 [        0] udns4.salesforce.com. 2610:a1:1010::8
$ echo $?
0
```

Report the serials of servers for zone appforce.com, compare them to
the master 10.11.12.13, and allow a serial number difference (-d) of
2. Since the serials of some servers were observed to differ by more
than this value (3 is greater than 2), the exit code is 1.

```
$ checkzoneserial -m 10.11.12.13 -d 2 appforce.com
Zone: appforce.com.
     2001771862 [   MASTER]  10.11.12.13 10.11.12.13
     2001771861 [        1] pch1.salesforce-dns.com. 2620:171:809::1
     2001771861 [        1] pch1.salesforce-dns.com. 206.223.122.1
     2001771859 [        3] udns2.salesforce.com. 2001:502:ad09::8
     2001771859 [        3] udns1.salesforce.com. 2001:502:2eda::8
     2001771861 [        1] udns4.salesforce.com. 156.154.103.8
     2001771861 [        1] udns1.salesforce.com. 156.154.100.8
     2001771861 [        1] udns3.salesforce.com. 156.154.102.8
     2001771861 [        1] udns2.salesforce.com. 156.154.101.8
     2001771861 [        1] udns4.salesforce.com. 2610:a1:1010::8
     2001771861 [        1] udns3.salesforce.com. 2610:a1:1009::8
$ echo $?
1
```
