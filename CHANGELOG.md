# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [SemVer](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [1.0.3] - 2026-06-02

### Added
- 仓库基线：`LICENSE`（MIT）、`CHANGELOG.md`、`.editorconfig`。
- 英文 `README.en.md`，首页 README 顶部加 badges、关键词、许可声明。
- CI 增加 `gofmt -l`、`staticcheck`、`prettier --check`、`eslint`、并发取消、依赖缓存。
- Release 增加 SHA256 校验（写入 release notes）、自动 release notes、固定 wails CLI 版本。

### Fixed
- `internal/proxy/ca.go`：`IsCAInstalled` 缓存改为加锁，消除 `GetStatus` 并发轮询时的数据竞争。
- `internal/app/config.go`：`config.json` 写入权限改为 `0o600`（含 API Key）。
- `internal/protocol/export_test_helper.go`：修复 staticcheck S1016（直接结构体转换）。
- `internal/proxy/usage.go`：`for i := 0; i < n; i++` 改为 Go 1.22+ `for i := range n`。
- 前端 `App.tsx` 应用 `prettier --write`。

## [1.0.2]

### Added
- 检测并清理其他工具（如 windsurf-tools-mitm）残留的 Windsurf/Codeium hosts 劫持。

## [1.0.1]

### Added
- 「恢复官方环境」按钮，一键停代理还原 hosts。

### Changed
- 退出时仅还原 hosts 与代理例外项，保留本机 CA（避免下次启动重新 UAC 安装）。

## [1.0.0]

### Added
- 首个公开版本：拦截 Windsurf 对话请求，转发到 OpenAI 兼容 / Anthropic / Google Gemini 上游。
- 思考过程透传（写入 Cascade `delta_thinking` 字段）。
- 配置界面：Base URL / API Key / 协议 / 模型，测试连接、拉取模型、用量统计。

[Unreleased]: https://github.com/Mag1cFall/API2Windsurf/compare/v1.0.3...HEAD
[1.0.3]: https://github.com/Mag1cFall/API2Windsurf/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/Mag1cFall/API2Windsurf/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/Mag1cFall/API2Windsurf/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/Mag1cFall/API2Windsurf/releases/tag/v1.0.0
