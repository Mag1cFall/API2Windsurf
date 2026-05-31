package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"api2windsurf/internal/protocol"
	"golang.org/x/net/http2"
)

type Config struct {
	Provider      string
	BaseURL       string
	APIKey        string
	Model         string
	Port          int
	ShowReasoning bool
	MaxTokens     int
}

func (c Config) upstream() Upstream {
	return Upstream{Provider: c.Provider, BaseURL: c.BaseURL, APIKey: c.APIKey, Model: c.Model}
}

type Server struct {
	mu       sync.RWMutex
	cfg      Config
	listener net.Listener
	httpSrv  *http.Server
	tracker  *UsageTracker
	log      func(string)

	resolvedIP string
	resolvedAt time.Time
}

func NewServer(cfg Config, tracker *UsageTracker, logFn func(string)) *Server {
	if logFn == nil {
		logFn = func(string) {}
	}
	return &Server{cfg: cfg, tracker: tracker, log: logFn}
}

func (s *Server) Start() error {
	cert, err := EnsureCertificates()
	if err != nil {
		return fmt.Errorf("certificates: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		NextProtos:   []string{"h2", "http/1.1"},
	}
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	ln, err := tls.Listen("tcp4", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	srv := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 30 * time.Second,
	}
	_ = http2.ConfigureServer(srv, &http2.Server{
		IdleTimeout:          120 * time.Second,
		MaxConcurrentStreams: 250,
	})
	s.mu.Lock()
	s.listener = ln
	s.httpSrv = srv
	s.mu.Unlock()

	s.log("proxy listening on " + addr)
	go func() {
		if err := srv.Serve(ln); err != nil && !errIsClosed(err) {
			s.log("serve stopped: " + err.Error())
		}
	}()
	return nil
}

// Stop closes the HTTP server and underlying listener, forcibly terminating
// any in-flight or keep-alive connections so Windsurf re-resolves DNS on retry.
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.httpSrv
	ln := s.listener
	s.httpSrv = nil
	s.listener = nil
	s.mu.Unlock()

	if srv == nil && ln == nil {
		return nil
	}
	if srv != nil {
		// Close (not Shutdown) so existing TLS / HTTP2 connections are
		// reset immediately rather than waiting for client-side timeout.
		_ = srv.Close()
	} else if ln != nil {
		_ = ln.Close()
	}
	return nil
}

func (s *Server) handler() http.Handler {
	reverse := s.newPassthrough()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isChatPath(r.URL.Path) && s.tryRelay(w, r) {
			return
		}
		reverse.ServeHTTP(w, r)
	})
}

func isChatPath(path string) bool {
	return strings.Contains(path, "GetChatMessage") ||
		strings.Contains(path, "GetChatMessageBurst") ||
		strings.Contains(path, "GetCompletions")
}

func (s *Server) tryRelay(w http.ResponseWriter, r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.Contains(ct, "proto") && !strings.Contains(ct, "grpc") {
		return false
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	body, err := stripEnvelope(raw)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(raw))
		return false
	}

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(r.Context(), RelayTimeout)
	defer cancel()

	start := time.Now()
	result := Relay(ctx, w, http.DefaultClient, cfg.upstream(), body, RelayOptions{
		ShowReasoning:     cfg.ShowReasoning,
		MaxTokensOverride: cfg.MaxTokens,
	})
	if !result.Served {
		r.Body = io.NopCloser(bytes.NewReader(raw))
		return false
	}
	if s.tracker != nil {
		status := "ok"
		detail := ""
		if result.Err != nil {
			status = "error"
			detail = result.Err.Error()
		}
		s.tracker.Record(UsageRecord{
			Model:        result.Model,
			Provider:     result.Provider,
			PromptTokens: result.Prompt,
			OutputTokens: result.Completion,
			DurationMs:   time.Since(start).Milliseconds(),
			Status:       status,
			ErrorDetail:  detail,
		})
	}
	return true
}

func (s *Server) newPassthrough() *httputil.ReverseProxy {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         TargetDomain,
			NextProtos:         []string{"h2"},
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxConnsPerHost:       20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 180 * time.Second,
	}
	if h2t, err := http2.ConfigureTransports(transport); err == nil {
		h2t.ReadIdleTimeout = 30 * time.Second
		h2t.PingTimeout = 15 * time.Second
	}
	return &httputil.ReverseProxy{
		FlushInterval: -1,
		Transport:     transport,
		Director: func(req *http.Request) {
			origHost := req.Host
			if origHost == "" || strings.HasPrefix(origHost, "127.0.0.1") {
				origHost = TargetDomain
			}
			if h, _, err := net.SplitHostPort(origHost); err == nil {
				origHost = h
			}
			req.URL.Scheme = "https"
			req.URL.Host = s.upstreamIP()
			req.Host = origHost
			req.Header.Set("Host", origHost)
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			s.log(fmt.Sprintf("passthrough error %s %s: %v", req.Method, req.URL.Path, err))
			w.WriteHeader(http.StatusBadGateway)
		},
	}
}

func (s *Server) upstreamIP() string {
	s.mu.RLock()
	if s.resolvedIP != "" && time.Since(s.resolvedAt) < 5*time.Minute {
		ip := s.resolvedIP
		s.mu.RUnlock()
		return ip
	}
	s.mu.RUnlock()

	ip := resolveRealIP()
	s.mu.Lock()
	s.resolvedIP = ip
	s.resolvedAt = time.Now()
	s.mu.Unlock()
	return ip
}

func resolveRealIP() string {
	ips, err := net.LookupHost(TargetDomain)
	if err == nil {
		for _, ip := range ips {
			if !strings.HasPrefix(ip, "127.") && !strings.Contains(ip, ":") {
				return ip
			}
		}
	}
	return UpstreamIP
}

func stripEnvelope(raw []byte) ([]byte, error) {
	return protocol.StripEnvelope(raw)
}

func errIsClosed(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection") ||
		err == http.ErrServerClosed
}
