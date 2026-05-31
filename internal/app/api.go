package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api2windsurf/internal/proxy"
)

type StatusDTO struct {
	CAInstalled     bool                `json:"ca_installed"`
	HostsMapped     bool                `json:"hosts_mapped"`
	ProxyRunning    bool                `json:"proxy_running"`
	ConfigValid     bool                `json:"config_valid"`
	ConfigError     string              `json:"config_error"`
	CACertPath      string              `json:"ca_cert_path"`
	ConfigPath      string              `json:"config_path"`
	Upstream        string              `json:"upstream"`
	Provider        string              `json:"provider"`
	Model           string              `json:"model"`
	ListenAddress   string              `json:"listen_address"`
	HostsHijacks    []proxy.HostsHijack `json:"hosts_hijacks"`
	ForeignHijack   bool                `json:"foreign_hijack"`
	HijackScanError string              `json:"hijack_scan_error"`
}

func (a *App) GetStatus() StatusDTO {
	a.mu.Lock()
	cfg := a.cfg
	running := a.server != nil
	a.mu.Unlock()

	path, _ := configPath()
	dto := StatusDTO{
		CAInstalled:   proxy.IsCAInstalled(),
		HostsMapped:   proxy.IsHostsMapped(),
		ProxyRunning:  running,
		CACertPath:    proxy.CACertPath(),
		ConfigPath:    path,
		Upstream:      cfg.BaseURL,
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		ListenAddress: fmt.Sprintf("127.0.0.1:%d", cfg.Port),
	}
	if err := cfg.validate(); err != nil {
		dto.ConfigError = err.Error()
	} else {
		dto.ConfigValid = true
	}
	if entries, err := proxy.ScanHostsHijacks(); err != nil {
		dto.HijackScanError = err.Error()
	} else {
		dto.HostsHijacks = entries
		for _, h := range entries {
			if !strings.EqualFold(h.Marker, "api2windsurf") {
				dto.ForeignHijack = true
				break
			}
		}
	}
	return dto
}

// PurgeAllHostsHijacks removes every hosts line that maps a known
// Windsurf/Codeium domain to a loopback IP, including ones added by other
// tools. Returns the removed entries. Requires the program to run elevated.
func (a *App) PurgeAllHostsHijacks() ([]proxy.HostsHijack, error) {
	a.mu.Lock()
	a.stopProxyLocked()
	a.mu.Unlock()
	return proxy.PurgeAllWindsurfHijacks()
}

func (a *App) GetConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) SaveConfig(cfg Config) error {
	cfg = cfg.normalized()
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg = cfg
	if a.server != nil {
		_ = a.server.Stop()
		a.server = nil
	}
	return a.startServer(cfg)
}

func (a *App) SetupSystem() error {
	return a.prepareSystem()
}

func (a *App) StartProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.cfg.validate(); err != nil {
		return err
	}
	return a.startServer(a.cfg)
}

func (a *App) StopProxy() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopProxyLocked()
	return proxy.TeardownRouting()
}

// RestoreOfficialEnvironment stops the proxy and removes hosts hijacking and
// proxy bypass entries so Windsurf can reach official backends with its own
// account. The local CA is intentionally kept; remove it manually via certmgr
// if you want a full uninstall.
func (a *App) RestoreOfficialEnvironment() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopProxyLocked()
	return proxy.TeardownRouting()
}

type ModelsResult struct {
	Models []string `json:"models"`
	Count  int      `json:"count"`
	Error  string   `json:"error"`
}

func (a *App) FetchModels(cfg Config) ModelsResult {
	cfg = cfg.normalized()
	if cfg.BaseURL == "" {
		return ModelsResult{Error: "Base URL 不能为空"}
	}
	if cfg.APIKey == "" {
		return ModelsResult{Error: "API Key 不能为空"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	models, err := proxy.FetchModels(ctx, &http.Client{Timeout: 20 * time.Second}, cfg.Provider, cfg.BaseURL, cfg.APIKey)
	if err != nil {
		return ModelsResult{Error: friendlyError(err)}
	}
	return ModelsResult{Models: models, Count: len(models)}
}

type TestResult struct {
	OK         bool   `json:"ok"`
	DurationMs int64  `json:"duration_ms"`
	ModelCount int    `json:"model_count"`
	Detail     string `json:"detail"`
	Error      string `json:"error"`
}

func (a *App) TestConnection(cfg Config) TestResult {
	cfg = cfg.normalized()
	if cfg.BaseURL == "" {
		return TestResult{Error: "Base URL 不能为空"}
	}
	if cfg.APIKey == "" {
		return TestResult{Error: "API Key 不能为空"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	models, err := proxy.FetchModels(ctx, &http.Client{Timeout: 20 * time.Second}, cfg.Provider, cfg.BaseURL, cfg.APIKey)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{OK: false, DurationMs: elapsed, Error: friendlyError(err)}
	}
	detail := fmt.Sprintf("连通成功，%s 返回 %d 个模型，耗时 %dms", providerLabel(cfg.Provider), len(models), elapsed)
	return TestResult{OK: true, DurationMs: elapsed, ModelCount: len(models), Detail: detail}
}

type UsageDTO struct {
	TotalRequests int                 `json:"total_requests"`
	TotalTokens   int                 `json:"total_tokens"`
	ErrorCount    int                 `json:"error_count"`
	Recent        []proxy.UsageRecord `json:"recent"`
}

func (a *App) GetUsage() UsageDTO {
	if a.tracker == nil {
		return UsageDTO{}
	}
	s := a.tracker.Summary()
	return UsageDTO{
		TotalRequests: s.TotalRequests,
		TotalTokens:   s.TotalTokens,
		ErrorCount:    s.ErrorCount,
		Recent:        a.tracker.Recent(10),
	}
}

func providerLabel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		return "Anthropic"
	case "google":
		return "Google Gemini"
	default:
		return "OpenAI 兼容端点"
	}
}

func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "connection refused") || strings.Contains(low, "actively refused"):
		return "连接被拒绝：目标端点未启动或端口不对（检查 Base URL）。原始: " + msg
	case strings.Contains(low, "no such host") || strings.Contains(low, "lookup"):
		return "域名解析失败：Base URL 主机名无法解析。原始: " + msg
	case strings.Contains(low, "timeout") || strings.Contains(low, "deadline exceeded"):
		return "请求超时：端点无响应或网络不通。原始: " + msg
	case strings.Contains(low, "401") || strings.Contains(low, "unauthorized"):
		return "鉴权失败 (401)：API Key 错误或缺失。原始: " + msg
	case strings.Contains(low, "403") || strings.Contains(low, "forbidden"):
		return "无权限 (403)：该 Key 无权访问此端点。原始: " + msg
	case strings.Contains(low, "404"):
		return "接口不存在 (404)：Base URL 路径不对。原始: " + msg
	case strings.Contains(low, "model list is empty"):
		return "端点返回模型列表为空：可手动填模型名。原始: " + msg
	default:
		return msg
	}
}
