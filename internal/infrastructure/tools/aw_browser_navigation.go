package tools

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"

	"aw/internal/infrastructure/sandbox"
)

type navigationGuard struct {
	policy    sandbox.Policy
	lookupIP  func(context.Context, string) ([]net.IPAddr, error)
	allowFile bool
}

var guardLookupIP = net.DefaultResolver.LookupIPAddr

func newNavigationGuard(policy sandbox.Policy) navigationGuard {
	return navigationGuard{policy: policy, lookupIP: guardLookupIP, allowFile: true}
}

func (g navigationGuard) check(ctx context.Context, rawURL string) (map[string]any, bool) {
	target := sanitizeBrowserURL(rawURL)
	if target == "" {
		return blockedNavigation(rawURL, "navigation target is empty"), true
	}
	if path, isFile := fileURLPath(target); isFile {
		if !g.allowFile {
			return blockedNavigation(target, "file:// URLs are blocked by the Agent Browser navigation guard"), true
		}
		if path == "" {
			return blockedNavigation(target, "file:// URL with no resolvable local path"), true
		}
		if _, err := sandbox.ResolveAndCheck(path, g.policy); err != nil {
			result := blockedNavigation(target, err.Error())
			result["blockedPaths"] = []string{path}
			return result, true
		}
		return nil, false
	}
	parsed, err := parseGuardURL(target)
	if err != nil {
		return blockedNavigation(target, err.Error()), true
	}
	if !isNetworkScheme(parsed.Scheme) {
		return nil, false
	}
	host := parsed.Hostname()
	if host == "" {
		return blockedNavigation(target, "network URL has no host"), true
	}
	if isLocalhostName(host) {
		return blockedNavigation(target, fmt.Sprintf("destination host %q is blocked because it is loopback/internal", host)), true
	}
	if addr, ok := parseHostAddr(host); ok {
		if reason := blockedAddrReason(addr); reason != "" {
			return blockedNavigation(target, reason), true
		}
		return nil, false
	}
	lookup := g.lookupIP
	if lookup == nil {
		return blockedNavigation(target, "DNS resolver is unavailable"), true
	}
	addrs, err := lookup(contextOrBackground(ctx), host)
	if err != nil {
		return blockedNavigation(target, fmt.Sprintf("could not resolve %q safely: %v", host, err)), true
	}
	if len(addrs) == 0 {
		return blockedNavigation(target, fmt.Sprintf("could not resolve %q safely: no addresses", host)), true
	}
	for _, ip := range addrs {
		addr, ok := netip.AddrFromSlice(ip.IP)
		if !ok {
			return blockedNavigation(target, fmt.Sprintf("could not classify resolved address for %q", host)), true
		}
		if reason := blockedAddrReason(addr.Unmap()); reason != "" {
			return blockedNavigation(target, fmt.Sprintf("%s (resolved from %q)", reason, host)), true
		}
	}
	return nil, false
}

func sanitizeBrowserURL(rawURL string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, strings.TrimSpace(rawURL))
}

func parseGuardURL(rawURL string) (*url.URL, error) {
	cleaned := sanitizeBrowserURL(rawURL)
	for {
		rest, ok := cutPrefixFold(cleaned, "view-source:")
		if !ok {
			break
		}
		cleaned = strings.TrimSpace(rest)
	}
	parsed, err := url.Parse(cleaned)
	if err != nil {
		return nil, fmt.Errorf("could not parse navigation URL: %w", err)
	}
	if parsed.Scheme == "" {
		return nil, fmt.Errorf("navigation URL must include a scheme")
	}
	return parsed, nil
}

func isNetworkScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https", "ws", "wss", "ftp":
		return true
	default:
		return false
	}
}

func isLocalhostName(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

func parseHostAddr(host string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func blockedAddrReason(addr netip.Addr) string {
	switch {
	case addr.IsLoopback():
		return fmt.Sprintf("destination %s is blocked because it is loopback/internal", addr)
	case addr.IsLinkLocalUnicast():
		return fmt.Sprintf("destination %s is blocked because it is link-local/internal", addr)
	case addr.IsPrivate():
		return fmt.Sprintf("destination %s is blocked because it is private/internal", addr)
	case netip.MustParsePrefix("100.64.0.0/10").Contains(addr):
		return fmt.Sprintf("destination %s is blocked because it is CGNAT/Tailscale/internal", addr)
	default:
		return ""
	}
}

func blockedNavigation(target string, reason string) map[string]any {
	return map[string]any{
		"blocked": true,
		"target":  target,
		"reason":  reason,
	}
}

func fileURLPath(rawURL string) (string, bool) {
	cleaned := sanitizeBrowserURL(rawURL)
	for {
		rest, ok := cutPrefixFold(cleaned, "view-source:")
		if !ok {
			break
		}
		cleaned = strings.TrimSpace(rest)
	}
	if _, ok := cutPrefixFold(cleaned, "file:"); !ok {
		return "", false
	}
	parsed, err := url.Parse(cleaned)
	if err != nil {
		if path, ok := fallbackWindowsFileURLPath(cleaned); ok {
			return path, true
		}
		return "", true
	}
	if parsed.Opaque != "" {
		return "", true
	}
	rawPath := parsed.Path
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		hostPath, err := url.PathUnescape(parsed.Host)
		if err != nil {
			return "", true
		}
		hostPath = strings.ReplaceAll(hostPath, `\`, string(filepath.Separator))
		if filepath.VolumeName(hostPath) == "" {
			return "", true
		}
		rawPath = hostPath + rawPath
	}
	if rawPath == "" {
		return "", true
	}
	path, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", true
	}
	return cleanLocalFilePath(path), true
}

func fallbackWindowsFileURLPath(cleaned string) (string, bool) {
	rest, ok := cutPrefixFold(cleaned, "file://")
	if !ok {
		return "", false
	}
	path, err := url.PathUnescape(rest)
	if err != nil {
		return "", false
	}
	path = cleanLocalFilePath(path)
	if filepath.VolumeName(path) == "" {
		return "", false
	}
	return path, true
}

func cleanLocalFilePath(path string) string {
	path = strings.ReplaceAll(path, `\`, string(filepath.Separator))
	if len(path) >= 3 && (path[0] == '/' || path[0] == '\\') && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Clean(path)
}
