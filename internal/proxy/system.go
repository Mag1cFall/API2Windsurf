package proxy

import "errors"

// TeardownSystem removes hosts hijacking, proxy bypass entries, and the local CA
// so Windsurf can reach official backends again without api2windsurf running.
func TeardownSystem() error {
	var errs []error
	if err := UnmapHosts(); err != nil {
		errs = append(errs, err)
	}
	if err := RemoveProxyOverride(); err != nil {
		errs = append(errs, err)
	}
	if err := UninstallCA(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// TeardownRouting removes hosts hijacking and proxy bypass entries only.
// Use when stopping the proxy but keeping the CA installed for a faster restart.
func TeardownRouting() error {
	var errs []error
	if err := UnmapHosts(); err != nil {
		errs = append(errs, err)
	}
	if err := RemoveProxyOverride(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
