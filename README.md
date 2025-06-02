# checkzoneserial
Check zone serial numbers across servers

This program queries a zone's SOA record at all its authoritative
servers simultaneously and reports the serial number seen and the response
times. If a master server is specified (with the -m option) it will also
compute the difference in the serial numbers seen with the master serial
number.

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
checkzoneserial, version 1.1.2
Usage: checkzoneserial [Options] <zone>

        Options:
        -h          Print this help string
        -4          Use IPv4 transport only
        -6          Use IPv6 transport only
        -cf file    Use alternate resolv.conf file
        -s          Print responses sorted by domain name and IP version
        -j          Produce json formatted output (implies -s)
        -c          Use TCP for queries (default: UDP with TCP on truncation)
        -t N        Query timeout value in seconds (default 3)
        -r N        Maximum # SOA query retries for each server (default 3)
        -d N        Allowed SOA serial number drift (default 0)
        -b N        Buffer size for DNS messages (default 1400)
        -nsid       Request NSID option in DNS queries
        -m ns       Master server name/address to compare serial numbers with
        -a ns1,..   Specify additional nameserver names/addresses to query
        -n          Don't query advertised nameservers for the zone
```

### Return codes

* 0 on success
* 1 if serials aren't identical or differ by more than allowed drift
* 2 on detection of server issues (timeout, bad response, etc)
* 3 if the master server (if specified) fails to respond
* 4 on program invocation error


### Example runs

Report zone serials for all authoritative servers for upenn.edu:

```
$ checkzoneserial upenn.edu
## upenn.edu. 2022-12-05T17:57:40EST
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
master server 10.11.12.13 (-m option) and report the deltas.

```
$ checkzoneserial -m 10.11.12.13 siteforce.com
## siteforce.com. 2022-12-05T17:57:40EST
     2019120538 [  MASTER] 10.11.12.13 10.11.12.13 0.41ms
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
## siteforce.com. 2022-12-05T17:57:40EST
     2019120538 [  MASTER] 10.11.12.13 10.11.12.13 0.54ms
     2019120538 [       0] pch1.salesforce-dns.com. 2620:171:809::1 7.12ms
     2019120538 [       0] udns1.salesforce.com. 2001:502:2eda::8 6.43ms
     2019120538 [       0] udns2.salesforce.com. 2001:502:ad09::8 7.77ms
     2019120538 [       0] udns3.salesforce.com. 2610:a1:1009::8 4.67ms
     2019120538 [       0] udns4.salesforce.com. 2610:a1:1010::8 8.88ms
$ echo $?
0
```

Report the serials of servers for zone appforce.com, compare them to
the master 10.11.12.13, and allow a serial number difference (-d) of
2. Since the serials of some servers were observed to differ by more
than this value (3 is greater than 2), the exit code is 1.

```
$ checkzoneserial -m 10.11.12.13 -d 2 appforce.com
## appforce.com. 2022-12-05T17:57:40EST
     2001771862 [  MASTER]  10.11.12.13 10.11.12.13 0.87ms
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

Display json formatted output (-j)
```
$ checkzoneserial -m 10.1.2.3 -j appforce.com  | jq .

{
  "status": 1,
  "error": "serial mismatch or exceeds drift",
  "zone": "appforce.com.",
  "timestamp": "2023-07-15T20:02:06EDT",
  "master": {
    "name": "",
    "ip": "10.1.2.3",
    "serial": 2025360499,
    "resptime": 7.909792
  },
  "responses": [
    {
      "name": "udns1.salesforce.com.",
      "ip": "2001:502:2eda::8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 33.851403000000005
    },
    {
      "name": "udns1.salesforce.com.",
      "ip": "156.154.100.8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 3.389384
    },
    {
      "name": "udns2.salesforce.com.",
      "ip": "2001:502:ad09::8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 22.976582
    },
    {
      "name": "udns2.salesforce.com.",
      "ip": "156.154.101.8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 1.929406
    },
    {
      "name": "udns3.salesforce.com.",
      "ip": "2610:a1:1009::8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 33.61233
    },
    {
      "name": "udns3.salesforce.com.",
      "ip": "156.154.102.8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 7.206194
    },
    {
      "name": "udns4.salesforce.com.",
      "ip": "2610:a1:1010::8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 21.751995
    },
    {
      "name": "udns4.salesforce.com.",
      "ip": "156.154.103.8",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 2.648234
    },
    {
      "name": "pch1.salesforce-dns.com.",
      "ip": "2620:171:809::1",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 3.994213
    },
    {
      "name": "pch1.salesforce-dns.com.",
      "ip": "206.223.122.1",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 4.748021
    },
    {
      "name": "pch2.salesforce-dns.com.",
      "ip": "2620:171:80a::1",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 4.633182
    },
    {
      "name": "pch2.salesforce-dns.com.",
      "ip": "199.184.183.1",
      "serial": 2025360495,
      "delta": 4,
      "resptime": 4.594670000000001
    }
  ]
}
```
