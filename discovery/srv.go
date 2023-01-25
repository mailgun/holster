package discovery

import (
	"fmt"
	"net"
	"time"

	"github.com/mailgun/holster/v5/errors"
	"github.com/miekg/dns"
)

// Given a DNS name return a list of addresses returned by the
// getting the SRV records from DNS for that name.
func GetSRVAddresses(dnsName, dnsServer string) ([]string, error) {
	if dnsServer != "" {
		return directLookupSRV(dnsName, dnsServer)
	}
	return builtinLookupSRV(dnsName)
}

// Queries the DNS server, by passing /etc/resolv.conf
func directLookupSRV(dnsName, dnsAddr string) ([]string, error) {
	if !dns.IsFqdn(dnsName) {
		dnsName += "."
	}

	c := new(dns.Client)
	c.Timeout = time.Second * 3

	m := new(dns.Msg)
	m.SetQuestion(dnsName, dns.TypeSRV)
	r, _, err := c.Exchange(m, dnsAddr)
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("no SRV records found for '%s'", dnsName)
	}

	var results []string
	for _, a := range r.Answer {
		srv := a.(*dns.SRV)
		if net.ParseIP(srv.Target) == nil {
			srv.Target = findARecord(srv.Target, r.Extra)
		}
		results = append(results, fmt.Sprintf("%s:%d", srv.Target, srv.Port))
	}
	return results, nil
}

// Attempts to find an 'A' record within the extra (additional answer section) of the DNS response
func findARecord(target string, extra []dns.RR) string {
	for _, item := range extra {
		if a, ok := item.(*dns.A); ok {
			if a.Hdr.Name == target {
				return a.A.String()
			}
		}
	}
	return target
}

// Uses the builtin golang net.LookupSRV on systems that have their
// /etc/resolv.conf configured correctly
func builtinLookupSRV(dnsName string) ([]string, error) {
	_, records, err := net.LookupSRV("", "", dnsName)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, srv := range records {
		if net.ParseIP(srv.Target) == nil {
			addresses, err := net.LookupHost(srv.Target)
			if err != nil {
				return results, errors.Wrapf(err, "while looking up A record for '%s'", srv.Target)
			}
			if len(addresses) == 0 {
				return results, errors.Wrapf(err, "no A records found for '%s'", srv.Target)
			}
			srv.Target = addresses[0]
		}
		results = append(results, fmt.Sprintf("%s:%d", srv.Target, srv.Port))
	}
	return results, nil
}
