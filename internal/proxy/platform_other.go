//go:build !windows

package proxy

import "os/exec"

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func runHidden(name string, args ...string) error {
	return execCommand(name, args...).Run()
}

func AddProxyOverride() error    { return nil }
func RemoveProxyOverride() error { return nil }
