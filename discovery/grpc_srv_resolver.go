package discovery

// Based on grpc-go/internal/resolver/dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/mailgun/holster/v4/cancel"
	"github.com/mailgun/holster/v4/retry"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/resolver"
)

func init() {
	GRPCSrvDefaultLogger = logrus.StandardLogger()
}

var (
	ErrMissingAddr         = errors.New("missing address")
	ErrEndsWithColon       = errors.New("missing port after port-separator colon")
	ErrIPAddressNotAllowed = errors.New("ip address is not allowed; must be a dns service name")
	GRPCSrvDefaultPort     = "443"
	GRPCSrvDefaultLogger   logrus.FieldLogger

	// GRPCSrvLogAddresses if true then GRPC will log the list of addresses received when making an SRV
	GRPCSrvLogAddresses = false
)

// NewGRPCSRVBuilder creates a srvResolverBuilder which is used to factory SRV-DNS resolvers.
func NewGRPCSRVBuilder() resolver.Builder {
	return &srvResolverBuilder{}
}

type srvResolverBuilder struct{}

func (*srvResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	host, port, err := parseTarget(target.Endpoint, GRPCSrvDefaultPort)
	if err != nil {
		return nil, err
	}

	// IP address.
	if _, ok := formatIP(host); ok {
		return nil, ErrIPAddressNotAllowed
	}

	d := &srvResolver{
		ctx:  cancel.New(context.Background()),
		rn:   make(chan struct{}, 1),
		host: host,
		port: port,
		cc:   cc,
	}

	d.wg.Add(1)
	go d.watcher()
	return d, nil
}

func (*srvResolverBuilder) Scheme() string { return "dns-srv" }

// srvResolver watches for the name resolution update for a non-IP target.
type srvResolver struct {
	host  string
	port  string
	ctx   cancel.Context
	cc    resolver.ClientConn
	state resolver.State
	// rn channel is used by ResolveNow() to force an immediate resolution of the target.
	rn chan struct{}
	// wg is used to enforce Close() to return after the watcher() goroutine has finished.
	// Otherwise, data race will be possible. [Race Example] in dns_resolver_test we
	// replace the real lookup functions with mocked ones to facilitate testing.
	// If Close() doesn't wait for watcher() goroutine finishes, race detector sometimes
	// will warns lookup (READ the lookup function pointers) inside watcher() goroutine
	// has data race with replaceNetFunc (WRITE the lookup function pointers).
	wg sync.WaitGroup
}

// ResolveNow invoke an immediate resolution of the target that this srvResolver watches.
func (d *srvResolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case d.rn <- struct{}{}:
	default:
	}
}

// Close closes the srvResolver.
func (d *srvResolver) Close() {
	d.ctx.Cancel()
	d.wg.Wait()
}

func (d *srvResolver) watcher() {
	defer d.wg.Done()

	ticker := time.NewTicker(time.Minute * 60)
	backOff := &retry.ExponentialBackOff{
		Min:    time.Second,
		Max:    120 * time.Second,
		Factor: 1.6,
	}
	var lastSuccess time.Time

	for {
		// Avoid constantly re-resolving if multiple connections make ResolveNow() calls
		if time.Since(lastSuccess) < time.Second*30 {
			goto wait
		}

		if err := d.lookupSRV(); err != nil {
			d.cc.ReportError(err)
			next := backOff.NextIteration()
			GRPCSrvDefaultLogger.WithError(err).WithField("retry-after", next).
				Error("dns lookup failed; retrying...")

			timer := time.NewTimer(next)
			select {
			case <-d.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		lastSuccess = time.Now()
	wait:
		backOff.Reset()

		select {
		case <-d.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		case <-d.rn:
		}
	}
}

func (d *srvResolver) lookupSRV() error {
	var result []resolver.Address

	// TODO(thrawn01): At some point in the future we might parse the Target and determine
	//  if the Target name is in the RFC 2782 form of `_<service>._tcp[.service][.<datacenter>].<domain>`
	//  then fill out the service and proto fields in LookupSRV()

	_, srvs, err := net.DefaultResolver.LookupSRV(d.ctx, "", "", d.host)
	if err != nil {
		return fmt.Errorf("SRV record lookup err: %w", err)
	}
	for _, s := range srvs {
		resolved, err := net.DefaultResolver.LookupHost(d.ctx, s.Target)
		if err != nil {
			GRPCSrvDefaultLogger.WithError(err).WithField("target", s.Target).
				Error("error resolving 'A' records for SRV entry")
			continue
		}

		var addresses []resolver.Address
		for _, a := range resolved {
			ip, ok := formatIP(a)
			if !ok {
				GRPCSrvDefaultLogger.WithField("ip", ip).
					Error("error parsing 'A' record for SRV entries; is not a valid ip address")
				continue
			}
			addresses = append(addresses, resolver.Address{Addr: ip + ":" + strconv.Itoa(int(s.Port)), ServerName: s.Target})
		}

		// If our current state is empty, then immediately update state before looking up the remaining SRV records.
		// Looking up a lot of hosts could take a lot of time, and we want to connect as quickly as possible.
		// During testing, a DNS lookup on all service nodes for `ratelimits` took 5+ seconds, which caused the
		// GRPC calls to timeout.
		if len(d.state.Addresses) == 0 {
			if err := d.cc.UpdateState(resolver.State{Addresses: addresses}); err != nil {
				GRPCSrvDefaultLogger.WithError(err).Error("UpdateState() call returned an error")
			}
			d.state.Addresses = addresses
		}
		result = append(result, addresses...)
	}

	if len(result) == 0 {
		return fmt.Errorf("SRV record for '%s' contained no valid domain names", d.host)
	}

	d.state.Addresses = result
	if GRPCSrvLogAddresses {
		var addresses []string
		for _, a := range result {
			addresses = append(addresses, a.Addr)
		}
		GRPCSrvDefaultLogger.WithField("addresses", addresses).Info("dns-srv: address list updated")
	}
	return d.cc.UpdateState(d.state)
}

// formatIP returns ok = false if addr is not a valid textual representation of an IP address.
// If addr is an IPv4 address, return the addr and ok = true.
// If addr is an IPv6 address, return the addr enclosed in square brackets and ok = true.
func formatIP(addr string) (addrIP string, ok bool) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", false
	}
	if ip.To4() != nil {
		return addr, true
	}
	return "[" + addr + "]", true
}

// parseTarget takes the user input target string and default port, returns formatted host and port info.
// If target doesn't specify a port, set the port to be the defaultPort.
// If target is in IPv6 format and host-name is enclosed in square brackets, brackets
// are stripped when setting the host.
// examples:
// target: "www.google.com" defaultPort: "443" returns host: "www.google.com", port: "443"
// target: "ipv4-host:80" defaultPort: "443" returns host: "ipv4-host", port: "80"
// target: "[ipv6-host]" defaultPort: "443" returns host: "ipv6-host", port: "443"
// target: ":80" defaultPort: "443" returns host: "localhost", port: "80"
func parseTarget(target, defaultPort string) (host, port string, err error) {
	if target == "" {
		return "", "", ErrMissingAddr
	}
	if ip := net.ParseIP(target); ip != nil {
		// target is an IPv4 or IPv6(without brackets) address
		return target, defaultPort, nil
	}
	if host, port, err = net.SplitHostPort(target); err == nil {
		if port == "" {
			// If the port field is empty (target ends with colon), e.g. "[::1]:", this is an error.
			return "", "", ErrEndsWithColon
		}
		// target has port, i.e ipv4-host:port, [ipv6-host]:port, host-name:port
		if host == "" {
			// Keep consistent with net.Dial(): If the host is empty, as in ":80", the local system is assumed.
			host = "localhost"
		}
		return host, port, nil
	}
	if host, port, err = net.SplitHostPort(target + ":" + defaultPort); err == nil {
		// target doesn't have port
		return host, port, nil
	}
	return "", "", fmt.Errorf("invalid target address %v, error info: %v", target, err)
}
