package agentfw

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"aw/internal/domain"
)

// ErrNoPermittedInterface means no PERMIT rule opens the service on any
// currently-available interface, so it must not bind at all (deny-all). Callers
// treat it as "closed by firewall", not a startup failure.
var ErrNoPermittedInterface = errors.New("no firewall rule permits this service on any available interface")

// Lister implements ports.NetworkInterfaceLister so the composition root can
// enumerate interfaces through an application use case rather than calling this
// infrastructure I/O directly.
type Lister struct{}

// ListInterfaces implements ports.NetworkInterfaceLister.
func (Lister) ListInterfaces() ([]domain.NetworkInterface, error) { return ListInterfaces() }

// ResolveBindAddrs implements ports.FirewallBindResolver.
func (Lister) ResolveBindAddrs(rules []domain.FirewallRule, service string, port int) ([]string, error) {
	return BindAddrs(rules, service, port)
}

// ListInterfaces enumerates IPv4 addresses on every up interface, classified by
// reachability and returned safest-first (loopback, tailscale, lan, public). It
// uses only the standard library and performs read-only interface enumeration —
// no network I/O.
func ListInterfaces() ([]domain.NetworkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []domain.NetworkInterface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := addrIP(addr)
			if ip == nil {
				continue
			}
			v4 := ip.To4()
			if v4 == nil {
				continue // IPv4 only in v1
			}
			out = append(out, domain.NetworkInterface{Name: iface.Name, IP: v4.String(), Kind: ClassifyIP(v4)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := kindOrder(out[i].Kind), kindOrder(out[j].Kind)
		if oi != oj {
			return oi < oj
		}
		return out[i].IP < out[j].IP
	})
	return out, nil
}

func addrIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}

func kindOrder(kind string) int {
	switch kind {
	case domain.FirewallIfaceLoopback:
		return 0
	case domain.FirewallIfaceTailscale:
		return 1
	case domain.FirewallIfaceLAN:
		return 2
	case domain.FirewallIfacePublic:
		return 3
	default:
		return 4
	}
}

// ResolveBindIPs turns a set of interface kinds (from BindKinds) into the
// concrete local IPs a listener must bind under Option A. Loopback always
// resolves to 127.0.0.1. The "all" wildcard resolves to 0.0.0.0 (bind every
// interface). tailscale/lan/public resolve to the matching up-interface IPs;
// a kind with no matching interface contributes nothing (the service simply is
// not reachable that way right now). Results are de-duplicated, order-stable.
func ResolveBindIPs(kinds []string) ([]string, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	want := map[string]bool{}
	for _, k := range kinds {
		want[strings.ToLower(strings.TrimSpace(k))] = true
	}
	var out []string
	seen := map[string]bool{}
	add := func(ip string) {
		if ip != "" && !seen[ip] {
			seen[ip] = true
			out = append(out, ip)
		}
	}
	if want[domain.FirewallIfaceAll] {
		return []string{"0.0.0.0"}, nil // bind everything; nothing else matters
	}
	if want[domain.FirewallIfaceLoopback] {
		add("127.0.0.1")
	}
	if want[domain.FirewallIfaceTailscale] || want[domain.FirewallIfaceLAN] || want[domain.FirewallIfacePublic] {
		ifaces, err := ListInterfaces()
		if err != nil {
			return nil, err
		}
		for _, iface := range ifaces {
			if want[iface.Kind] {
				add(iface.IP)
			}
		}
	}
	return out, nil
}

// BindAddrs resolves the listen addresses (host:port) a service must bind given
// the firewall rules and its port. Empty means the service is fully closed —
// no rule permits it — and the caller should not start a listener.
func BindAddrs(rules []domain.FirewallRule, service string, port int) ([]string, error) {
	ips, err := ResolveBindIPs(BindKinds(rules, service))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, net.JoinHostPort(ip, fmt.Sprintf("%d", port)))
	}
	return out, nil
}

// PeerFilter returns a decision closure for one service: given the local address
// a connection arrived on and the remote peer IP, it consults the firewall. It
// is what each server's guard calls per request.
func PeerFilter(rules []domain.FirewallRule, service string) func(localIP, remoteIP string) bool {
	// Copy the slice so later config edits do not mutate a live server's policy.
	snapshot := append([]domain.FirewallRule(nil), rules...)
	return func(localIP, remoteIP string) bool {
		return Decide(snapshot, service, ClassifyHost(localIP), remoteIP)
	}
}
