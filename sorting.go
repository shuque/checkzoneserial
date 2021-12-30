package main

import (
	"github.com/miekg/dns"
)

//
// CanonicalDomainOrder compares 2 domain name strings in DNS canonical order,
// and returns -1, 0, or 1, according to whether the first string sorts earlier
// than, equal to, or later than the second string.
//
func CanonicalDomainOrder(d1, d2 string) int {

	name1, name2 := dns.CanonicalName(dns.Fqdn(d1)), dns.CanonicalName(dns.Fqdn(d2))
	if name1 == name2 {
		return 0
	}

	labels1, labels2 := dns.SplitDomainName(name1), dns.SplitDomainName(name2)
	len1, len2 := len(labels1), len(labels2)

	var minlength int
	if len1 <= len2 {
		minlength = len1
	} else {
		minlength = len2
	}

	for i := 0; i < minlength; i++ {
		l1, l2 := labels1[len1-i-1], labels2[len2-i-1]
		if l1 > l2 {
			return 1
		} else if l2 > l1 {
			return -1
		}
	}
	if len1 > len2 {
		return 1
	} else if len2 > len1 {
		return -1
	}
	return 0
}

type ByCanonicalOrder []string

func (s ByCanonicalOrder) Len() int {
	return len(s)
}
func (s ByCanonicalOrder) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByCanonicalOrder) Less(i, j int) bool {
	return CanonicalDomainOrder(s[i], s[j]) == -1
}

//
// To sort Response lists by version (IPv6 first)
//
type ByIPversion []Response

func (s ByIPversion) Len() int {
	return len(s)
}
func (s ByIPversion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByIPversion) Less(i, j int) bool {
	return s[i].nsip.To4() == nil
}
