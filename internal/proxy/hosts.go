package proxy

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
)

const (
	TargetDomain = "server.self-serve.windsurf.com"
	UpstreamIP   = "34.49.14.144"
	hostsMarker  = "# api2windsurf"
)

// HijackDomains are domains api2windsurf actively MITMs.
var HijackDomains = []string{
	"server.self-serve.windsurf.com",
	"server.codeium.com",
}

// KnownWindsurfDomains is a superset of Windsurf/Codeium production hostnames
// that any local MITM tool tends to override. Used by ScanHostsHijacks to detect
// leftover entries from other tools (e.g. windsurf-tools-mitm) that block
// official Windsurf even after api2windsurf has been removed.
var KnownWindsurfDomains = []string{
	"server.self-serve.windsurf.com",
	"server.codeium.com",
	"inference.codeium.com",
	"web-backend.codeium.com",
	"api.codeium.com",
}

// HostsHijack describes a single line in the hosts file mapping a Windsurf or
// Codeium domain to a loopback address, regardless of which tool added it.
type HostsHijack struct {
	Domain string `json:"domain"`
	IP     string `json:"ip"`
	Marker string `json:"marker"` // trailing comment, e.g. "api2windsurf"
	Line   string `json:"line"`
}

func hostsPath() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

func IsHostsMapped() bool {
	data, err := os.ReadFile(hostsPath())
	if err != nil {
		return false
	}
	return strings.Contains(string(data), hostsMarker)
}

func MapHosts() error {
	path := hostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}
	if strings.Contains(string(data), hostsMarker) {
		return nil
	}
	var b strings.Builder
	b.WriteString(string(data))
	if !strings.HasSuffix(string(data), "\n") {
		b.WriteString("\n")
	}
	for _, domain := range HijackDomains {
		fmt.Fprintf(&b, "127.0.0.1 %s %s\n", domain, hostsMarker)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write hosts (needs privilege): %w", err)
	}
	return flushDNS()
}

func UnmapHosts() error {
	path := hostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}
	out := stripMarkedHostsLines(string(data), hostsMarker)
	if string(data) == out {
		return nil
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write hosts (needs privilege): %w", err)
	}
	return flushDNS()
}

func stripMarkedHostsLines(content, marker string) string {
	var kept []string
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, marker) {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

// ScanHostsHijacks reports every hosts entry that points a known Windsurf or
// Codeium domain at a loopback address — including ones added by other tools.
func ScanHostsHijacks() ([]HostsHijack, error) {
	data, err := os.ReadFile(hostsPath())
	if err != nil {
		return nil, fmt.Errorf("read hosts: %w", err)
	}
	return scanHostsHijacks(string(data), KnownWindsurfDomains), nil
}

func scanHostsHijacks(content string, domains []string) []HostsHijack {
	wanted := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		wanted[strings.ToLower(d)] = struct{}{}
	}
	var found []HostsHijack
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		body, marker := splitHostsLineComment(trimmed)
		fields := strings.Fields(body)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		if !isLoopbackIP(ip) {
			continue
		}
		for _, host := range fields[1:] {
			if _, ok := wanted[strings.ToLower(host)]; ok {
				found = append(found, HostsHijack{
					Domain: host,
					IP:     ip,
					Marker: strings.TrimSpace(marker),
					Line:   line,
				})
			}
		}
	}
	return found
}

func splitHostsLineComment(line string) (body, marker string) {
	if i := strings.Index(line, "#"); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(strings.TrimPrefix(line[i:], "#"))
	}
	return line, ""
}

func isLoopbackIP(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.IsLoopback()
}

// PurgeAllWindsurfHijacks removes every hosts line whose first token is a
// loopback address and that lists at least one known Windsurf/Codeium domain.
// Returns the entries that were removed. Lines that mix a hijacked domain with
// other unrelated hostnames are rewritten in place to drop only the hijacked
// hostnames, preserving any unrelated mappings on the same line.
func PurgeAllWindsurfHijacks() ([]HostsHijack, error) {
	path := hostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hosts: %w", err)
	}
	out, removed := purgeAllWindsurfHijacks(string(data), KnownWindsurfDomains)
	if len(removed) == 0 {
		return nil, nil
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return removed, fmt.Errorf("write hosts (needs privilege): %w", err)
	}
	if err := flushDNS(); err != nil {
		return removed, err
	}
	return removed, nil
}

func purgeAllWindsurfHijacks(content string, domains []string) (string, []HostsHijack) {
	wanted := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		wanted[strings.ToLower(d)] = struct{}{}
	}
	var (
		kept    []string
		removed []HostsHijack
	)
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		body, marker := splitHostsLineComment(trimmed)
		fields := strings.Fields(body)
		if len(fields) < 2 || !isLoopbackIP(fields[0]) {
			kept = append(kept, line)
			continue
		}
		ip := fields[0]
		var keepHosts []string
		var dropped []string
		for _, host := range fields[1:] {
			if _, ok := wanted[strings.ToLower(host)]; ok {
				dropped = append(dropped, host)
			} else {
				keepHosts = append(keepHosts, host)
			}
		}
		if len(dropped) == 0 {
			kept = append(kept, line)
			continue
		}
		for _, host := range dropped {
			removed = append(removed, HostsHijack{
				Domain: host,
				IP:     ip,
				Marker: marker,
				Line:   line,
			})
		}
		if len(keepHosts) > 0 {
			rebuilt := ip + " " + strings.Join(keepHosts, " ")
			if marker != "" {
				rebuilt += " # " + marker
			}
			kept = append(kept, rebuilt)
		}
	}
	out := strings.Join(kept, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, removed
}

func flushDNS() error {
	switch runtime.GOOS {
	case "windows":
		return runHidden("ipconfig", "/flushdns")
	case "darwin":
		_ = runPlain("dscacheutil", "-flushcache")
		_ = runPlain("killall", "-HUP", "mDNSResponder")
		return nil
	default:
		_ = runPlain("systemd-resolve", "--flush-caches")
		return nil
	}
}
