package proxy

import "testing"

func TestStripMarkedHostsLines(t *testing.T) {
	in := "127.0.0.1 localhost\n127.0.0.1 server.self-serve.windsurf.com # api2windsurf\n"
	want := "127.0.0.1 localhost\n"
	got := stripMarkedHostsLines(in, hostsMarker)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStripMarkedHostsLinesNoMarker(t *testing.T) {
	in := "127.0.0.1 localhost\n"
	got := stripMarkedHostsLines(in, hostsMarker)
	if got != in {
		t.Fatalf("got %q, want %q", got, in)
	}
}
