package webserver

import (
	"net"
	"sort"
)

// Bind-candidate kinds, ordered by how safe a plain-HTTP bind is. Tailscale and
// loopback are safe; a private LAN address is plaintext on the local network; a
// public address would expose the unencrypted UI to the internet.
const (
	BindKindTailscale = "tailscale"
	BindKindLoopback  = "loopback"
	BindKindPrivate   = "private"
	BindKindPublic    = "public"
)

// BindCandidate is one local IPv4 address the web server could bind to,
// classified by reachability.
type BindCandidate struct {
	Iface string
	IP    string
	Kind  string
}

// ListBindCandidates enumerates IPv4 addresses on every up interface and
// classifies each by reachability. It uses only the Go standard library (no
// CLI) and performs read-only local interface enumeration — no external I/O —
// so the composition root may call it directly (see DetectTailscaleIP).
// Candidates are returned safest-first.
func ListBindCandidates() ([]BindCandidate, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []BindCandidate
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
			out = append(out, BindCandidate{
				Iface: iface.Name,
				IP:    v4.String(),
				Kind:  classifyBindIP(v4),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := bindKindOrder(out[i].Kind), bindKindOrder(out[j].Kind)
		if oi != oj {
			return oi < oj
		}
		return out[i].IP < out[j].IP
	})
	return out, nil
}

// classifyBindIP buckets a v4 address by reachability. Order matters: the
// Tailscale CGNAT range (100.64.0.0/10) is not RFC1918, so it is checked before
// the private/link-local test.
func classifyBindIP(v4 net.IP) string {
	switch {
	case v4.IsLoopback():
		return BindKindLoopback
	case tailscaleNet.Contains(v4):
		return BindKindTailscale
	case v4.IsPrivate() || v4.IsLinkLocalUnicast():
		return BindKindPrivate
	default:
		return BindKindPublic
	}
}

func bindKindOrder(kind string) int {
	switch kind {
	case BindKindTailscale:
		return 0
	case BindKindLoopback:
		return 1
	case BindKindPrivate:
		return 2
	case BindKindPublic:
		return 3
	default:
		return 4
	}
}
