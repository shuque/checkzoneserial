package main

import (
	"github.com/miekg/dns"
)

// CanonicalDomainOrder compares 2 domain name strings in DNS canonical order,
// and returns -1, 0, or 1, according to whether the first string sorts earlier
// than, equal to, or later than the second string.
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

// To sort Response lists by version (IPv6 first)
type ByIPversion []Response

func (s ByIPversion) Len() int {
	return len(s)
}
func (s ByIPversion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByIPversion) Less(i, j int) bool {
	// Handle nil IPs
	if s[i].ip == nil {
		return true
	}
	if s[j].ip == nil {
		return false
	}

	// Compare IPv4 vs IPv6
	iIsIPv4 := s[i].ip.To4() != nil
	jIsIPv4 := s[j].ip.To4() != nil

	if iIsIPv4 != jIsIPv4 {
		return !iIsIPv4 // IPv6 comes before IPv4
	}

	// If both are same type, compare the IPs numerically
	if iIsIPv4 {
		// For IPv4, compare as 32-bit integers
		i4 := s[i].ip.To4()
		j4 := s[j].ip.To4()
		for k := 0; k < 4; k++ {
			if i4[k] != j4[k] {
				return i4[k] < j4[k]
			}
		}
	} else {
		// For IPv6, compare as 128-bit integers
		for k := 0; k < 16; k++ {
			if s[i].ip[k] != s[j].ip[k] {
				return s[i].ip[k] < s[j].ip[k]
			}
		}
	}
	return false // IPs are equal
}
