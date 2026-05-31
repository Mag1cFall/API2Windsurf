package proxy

import "testing"

func TestScanHostsHijacks_DetectsForeignMarkers(t *testing.T) {
	content := "127.0.0.1 localhost\n" +
		"127.0.0.1 server.self-serve.windsurf.com # windsurf-tools-mitm\n" +
		"127.0.0.1 server.codeium.com # api2windsurf\n" +
		"# 127.0.0.1 commented.example.com\n" +
		"34.49.14.144 server.self-serve.windsurf.com\n"
	got := scanHostsHijacks(content, KnownWindsurfDomains)
	if len(got) != 2 {
		t.Fatalf("want 2 hijacks, got %d: %+v", len(got), got)
	}
	if got[0].Domain != "server.self-serve.windsurf.com" || got[0].Marker != "windsurf-tools-mitm" {
		t.Errorf("unexpected first entry: %+v", got[0])
	}
	if got[1].Domain != "server.codeium.com" || got[1].Marker != "api2windsurf" {
		t.Errorf("unexpected second entry: %+v", got[1])
	}
}

func TestPurgeAllWindsurfHijacks_RemovesAndPreserves(t *testing.T) {
	content := "127.0.0.1 localhost\n" +
		"127.0.0.1 server.self-serve.windsurf.com # windsurf-tools-mitm\n" +
		"127.0.0.1 server.codeium.com other.example.com # mixed\n"
	out, removed := purgeAllWindsurfHijacks(content, KnownWindsurfDomains)
	if len(removed) != 2 {
		t.Fatalf("want 2 removed, got %d", len(removed))
	}
	wantOut := "127.0.0.1 localhost\n127.0.0.1 other.example.com # mixed\n"
	if out != wantOut {
		t.Fatalf("output mismatch:\n--- got ---\n%s\n--- want ---\n%s", out, wantOut)
	}
}

func TestPurgeAllWindsurfHijacks_NoOpWhenClean(t *testing.T) {
	content := "127.0.0.1 localhost\n::1 localhost\n"
	out, removed := purgeAllWindsurfHijacks(content, KnownWindsurfDomains)
	if len(removed) != 0 {
		t.Fatalf("want 0 removed, got %d", len(removed))
	}
	if out != content {
		t.Fatalf("content should be unchanged, got: %q", out)
	}
}
