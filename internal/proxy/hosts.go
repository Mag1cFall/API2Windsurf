package proxy

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

const (
	TargetDomain = "server.self-serve.windsurf.com"
	UpstreamIP   = "34.49.14.144"
	hostsMarker  = "# api2windsurf"
)

var HijackDomains = []string{
	"server.self-serve.windsurf.com",
	"server.codeium.com",
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
