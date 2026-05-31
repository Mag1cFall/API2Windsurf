//go:build windows

package proxy

import (
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func execCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

func runHidden(name string, args ...string) error {
	return execCommand(name, args...).Run()
}

const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

func AddProxyOverride() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	existing, _, _ := k.GetStringValue("ProxyOverride")
	for _, domain := range HijackDomains {
		if !strings.Contains(existing, domain) {
			if existing != "" {
				existing += ";"
			}
			existing += domain
		}
	}
	return k.SetStringValue("ProxyOverride", existing)
}
