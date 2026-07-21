package webserver

import (
	"errors"
	"net"
)

// tailscaleCGNAT is the 100.64.0.0/10 carrier-grade-NAT range Tailscale assigns
// to every node. Matching by range (not interface name) works across the
// per-OS interface names: tailscale0 (Linux), utun* (macOS), Tailscale (Win).
const tailscaleCGNAT = "100.64.0.0/10"

// ErrNoTailscaleIP is returned when bind mode is Tailscale but no tailnet IPv4
// is present on any interface — the server refuses to start in that case.
var ErrNoTailscaleIP = errors.New("no Tailscale (100.64.0.0/10) IPv4 found on any interface")

// tailscaleNet is the parsed CGNAT range, computed once.
var tailscaleNet = mustCIDR(tailscaleCGNAT)

func mustCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return network
}

// DetectTailscaleIP returns the first IPv4 address found within the Tailscale
// CGNAT range across all interfaces, using only the Go standard library (no
// CLI). It returns ErrNoTailscaleIP when none is present.
func DetectTailscaleIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		ip := addrIP(addr)
		if ip == nil {
			continue
		}
		v4 := ip.To4()
		if v4 == nil {
			continue
		}
		if tailscaleNet.Contains(v4) {
			return v4.String(), nil
		}
	}
	return "", ErrNoTailscaleIP
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

// isTailscalePeer reports whether host (an IP string, no port) is inside the
// Tailscale CGNAT range.
func isTailscalePeer(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		return tailscaleNet.Contains(v4)
	}
	return false
}

// parseCIDRs parses a list of CIDR strings, skipping any that are invalid.
func parseCIDRs(cidrs []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, c := range cidrs {
		if _, network, err := net.ParseCIDR(c); err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}

// hostInAnyCIDR reports whether host (an IP string) falls in any of nets.
func hostInAnyCIDR(host string, nets []*net.IPNet) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
