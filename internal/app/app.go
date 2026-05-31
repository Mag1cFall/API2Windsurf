package app

import (
	"context"
	"fmt"
	"sync"

	"api2windsurf/internal/proxy"
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	cfg     Config
	server  *proxy.Server
	tracker *proxy.UsageTracker
}

func New() *App {
	return &App{tracker: proxy.NewUsageTracker()}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := loadConfig()
	if err != nil {
		cfg = defaultConfig()
	}
	a.mu.Lock()
	a.cfg = cfg
	a.mu.Unlock()

	go func() {
		if err := a.prepareSystem(); err != nil {
			fmt.Println("system setup:", err)
		}
		if cfg.validate() == nil {
			if err := a.startServer(cfg); err != nil {
				fmt.Println("start proxy:", err)
			}
		}
	}()
}

func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server != nil {
		_ = a.server.Stop()
		a.server = nil
	}
}

func (a *App) prepareSystem() error {
	if _, err := proxy.EnsureCertificates(); err != nil {
		return err
	}
	if !proxy.IsCAInstalled() {
		if err := proxy.InstallCA(); err != nil {
			return fmt.Errorf("install CA: %w", err)
		}
		proxy.InvalidateCACache()
	}
	if !proxy.IsHostsMapped() {
		if err := proxy.MapHosts(); err != nil {
			return fmt.Errorf("map hosts: %w", err)
		}
	}
	_ = proxy.AddProxyOverride()
	return nil
}

func (a *App) startServer(cfg Config) error {
	if a.server != nil {
		return nil
	}
	srv := proxy.NewServer(a.serverConfig(cfg), a.tracker, func(msg string) { fmt.Println(msg) })
	if err := srv.Start(); err != nil {
		return err
	}
	a.server = srv
	return nil
}

func (a *App) serverConfig(cfg Config) proxy.Config {
	return proxy.Config{
		Provider:      cfg.Provider,
		BaseURL:       cfg.BaseURL,
		APIKey:        cfg.APIKey,
		Model:         cfg.Model,
		Port:          cfg.Port,
		ShowReasoning: cfg.showReasoning(),
		MaxTokens:     cfg.MaxTokens,
	}
}
