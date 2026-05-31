//go:build windows

package proxy

import "testing"

func TestRemoveProxyOverrideDomains(t *testing.T) {
	in := "localhost;server.self-serve.windsurf.com;192.168.0.0/16"
	want := "localhost;192.168.0.0/16"
	got := removeProxyOverrideDomains(in, HijackDomains)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
