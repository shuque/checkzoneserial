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
$ ./checkzoneserial
Usage: checkzoneserial [options] <zone>
  -4    use IPv4 only
  -6    use IPv6 only
  -m string
        master server address
```

### Example run

```
$ ./checkzoneserial -m 18.213.55.141 siteforce.com
     2019120538 [   MASTER] 18.213.55.141 18.213.55.141
     2019120538 [        0] udns1.salesforce.com. 2001:502:2eda::8
     2019120538 [        0] pch1.salesforce-dns.com. 206.223.122.1
     2019120538 [        0] pch1.salesforce-dns.com. 2620:171:809::1
     2019120538 [        0] udns2.salesforce.com. 2001:502:ad09::8
     2019120538 [        0] udns4.salesforce.com. 156.154.103.8
     2019120538 [        0] udns1.salesforce.com. 156.154.100.8
     2019120538 [        0] udns2.salesforce.com. 156.154.101.8
     2019120538 [        0] udns3.salesforce.com. 156.154.102.8
     2019120538 [        0] udns4.salesforce.com. 2610:a1:1010::8
     2019120538 [        0] udns3.salesforce.com. 2610:a1:1009::8
```
