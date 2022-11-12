# checkzoneserial
Check zone serial numbers across servers

This program queries a zone's SOA record at all its authoritative
servers simultaneously and reports the serial number seen and the response
times. If a primary server is specified (with the -m option) it will also
compute the difference in the serial numbers seen with the primary serial
number.

The primary, if provided, is queried first. Then all the authoritative
servers are queried in parallel.

### Pre-requisites

* Go
* Miek Gieben's Go dns package: https://github.com/miekg/dns

### Building

Just run 'go build'. This will generate the executable 'checkzoneserial'.

### Usage

```
$ checkzoneserial -h
checkzoneserial, version 1.0.2
Usage: checkzoneserial [Options] <zone>

        Options:
        -h          Print this help string
        -4          Use IPv4 transport only
        -6          Use IPv6 transport only
        -cf file    Use alternate resolv.conf file
        -s          Print responses sorted by domain name and IP version
        -c          Use TCP for queries (default: UDP with TCP on truncation)
        -t N        Query timeout value in seconds (default 3)
        -r N        Maximum # SOA query retries for each server (default 3)
        -d N        Allowed SOA serial number drift (default 0)
        -m ns       Primary server name/address to compare serial numbers with
        -a ns1,..   Specify additional nameserver names/addresses to query
        -n          Don't query advertised nameservers for the zone
```

### Return codes

* 0 on success
* 1 if serials aren't identical or differ by more than allowed drift
* 2 on detection of server issues (timeout, bad response, etc)
* 3 if the primary server (if specified) fails to respond
* 4 on program invocation error


### Example runs

Report zone serials for all authoritative servers for upenn.edu:

```
$ checkzoneserial upenn.edu
## Zone: upenn.edu.
## Time: 2021-10-30 19:00:39.503772028 -0500 EST m=+0.003235641
     1007401858 adns3.upenn.edu. 128.91.251.33 7.12ms
     1007401858 adns1.upenn.edu. 128.91.3.128 5.31ms
     1007401858 dns1.udel.edu. 128.175.13.16 12.64ms
     1007401858 dns2.udel.edu. 128.175.13.17 17.41ms
     1007401858 adns2.upenn.edu. 128.91.254.22 6.42ms
     1007401858 adns3.upenn.edu. 2607:f470:1003::3:c 5.21ms
     1007401858 adns1.upenn.edu. 2607:f470:1001::1:a 5.67ms
     1007401858 adns2.upenn.edu. 2607:f470:1002::2:3 5.12ms
     1007401858 sns-pb.isc.org. 192.5.4.1 9.99ms
     1007401858 sns-pb.isc.org. 2001:500:2e::1 8.87ms
$ echo $?
0
```

Report zone serials for siteforce.com servers, compare them to the
primary server 10.11.12.13 (-m option) and report the deltas.

```
$ checkzoneserial -m 10.11.12.13 siteforce.com
## Zone: siteforce.com
## Time: 2021-12-30 19:00:39.503772028 -0500 EST m=+0.003235641
     2019120538 [ PRIMARY] 10.11.12.13 10.11.12.13 0.41ms
     2019120538 [       0] udns1.salesforce.com. 2001:502:2eda::8 5.43ms
     2019120537 [       1] pch1.salesforce-dns.com. 206.223.122.1 6.71ms
     2019120538 [       0] pch1.salesforce-dns.com. 2620:171:809::1 7.12ms
     2019120536 [       2] udns2.salesforce.com. 2001:502:ad09::8 8.88ms
     2019120538 [       0] udns4.salesforce.com. 156.154.103.8 5.89ms
     2019120538 [       0] udns1.salesforce.com. 156.154.100.8 6.74ms
     2019120538 [       0] udns2.salesforce.com. 156.154.101.8 3.39ms
     2019120536 [       2] udns3.salesforce.com. 156.154.102.8 12.12ms
     2019120538 [       0] udns4.salesforce.com. 2610:a1:1010::8 9.74ms
     2019120536 [       2] udns3.salesforce.com. 2610:a1:1009::8 8.61ms
$ echo $?
1
```

The same as the last run, but only check the IPv6 addresses of the
servers. Since all the serials are the same, the exit code is 0.

```
$ checkzoneserial -m 10.11.12.13 -6 siteforce.com
## Zone: siteforce.com
## Time: 2021-12-30 19:00:39.503772028 -0500 EST m=+0.003235641
     2019120538 [ PRIMARY] 10.11.12.13 10.11.12.13 0.54ms
     2019120538 [       0] pch1.salesforce-dns.com. 2620:171:809::1 7.12ms
     2019120538 [       0] udns1.salesforce.com. 2001:502:2eda::8 6.43ms
     2019120538 [       0] udns2.salesforce.com. 2001:502:ad09::8 7.77ms
     2019120538 [       0] udns3.salesforce.com. 2610:a1:1009::8 4.67ms
     2019120538 [       0] udns4.salesforce.com. 2610:a1:1010::8 8.88ms
$ echo $?
0
```

Report the serials of servers for zone appforce.com, compare them to
the primary 10.11.12.13, and allow a serial number difference (-d) of
2. Since the serials of some servers were observed to differ by more
than this value (3 is greater than 2), the exit code is 1.

```
$ checkzoneserial -m 10.11.12.13 -d 2 appforce.com
## Zone: appforce.com.
## Time: 2021-12-30 19:00:39.503772028 -0500 EST m=+0.003235641
     2001771862 [ PRIMARY]  10.11.12.13 10.11.12.13 0.87ms
     2001771861 [       1] pch1.salesforce-dns.com. 2620:171:809::1 4.23ms
     2001771861 [       1] pch1.salesforce-dns.com. 206.223.122.1 4.54ms
     2001771859 [       3] udns2.salesforce.com. 2001:502:ad09::8 5.56ms
     2001771859 [       3] udns1.salesforce.com. 2001:502:2eda::8 7.98ms
     2001771861 [       1] udns4.salesforce.com. 156.154.103.8 8.81ms
     2001771861 [       1] udns1.salesforce.com. 156.154.100.8 5.43ms
     2001771861 [       1] udns3.salesforce.com. 156.154.102.8 4.55ms
     2001771861 [       1] udns2.salesforce.com. 156.154.101.8 7.77ms
     2001771861 [       1] udns4.salesforce.com. 2610:a1:1010::8 9.43ms
     2001771861 [       1] udns3.salesforce.com. 2610:a1:1009::8 11.32ms
$ echo $?
1
```
