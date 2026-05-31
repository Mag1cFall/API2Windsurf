# API2Windsurf

在 Windsurf 里使用你自己的模型。

这是一个本地转接程序：它把 Windsurf 发往云端的对话请求拦下来，改成标准格式转发到你指定的 API 端点。Windsurf 界面照常使用，背后实际调用的是你配置的模型。整个过程对 IDE 透明，不需要改 Windsurf 的任何设置。

## 它做了什么

程序在本机启动一个服务，接收 Windsurf 发出的对话请求，把它翻译成标准 API 调用，支持 OpenAI 兼容接口、Anthropic、Google Gemini 三种协议。

对支持思考过程（reasoning）的模型，思考内容会写入 Windsurf 原生的思考字段，IDE 会像官方 thinking 模型一样显示可折叠的思考块。

## 使用方法

1. 运行 `api2windsurf.exe`，同意管理员权限请求。
2. 在弹出的配置界面填写上游 API 地址、密钥，选择协议和模型。
3. 点「测试连接」确认连通，然后点「保存并应用」。
4. 完全退出 Windsurf 再重新打开。
5. 在 Windsurf 里选任意模型开始对话，请求会被改写成你配置的模型。

> 在 Windsurf 里选哪个模型都行——代理会强制改写成你配置的那个。真正的「切换模型」在本程序里做。

**切回官方账号**：退出本程序，或在界面点「恢复官方环境」，然后完全退出并重开 Windsurf。程序退出时会自动还原 hosts 与本地证书，避免 Windsurf 仍指向已关闭的本机代理。

详细说明见 [docs/usage.md](docs/usage.md)。

## 从源码构建

需要 Go 1.25+、Node.js 18+、[Wails v2](https://wails.io)。

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails build
```

产物是 `build/bin/api2windsurf.exe`，单文件，不需要额外运行环境。

开发模式（前端热更新）：

```bash
wails dev
```

## 原理

```
Windsurf IDE  ──对话请求──>  server.self-serve.windsurf.com
                              │ (hosts 指向 127.0.0.1)
                              ▼
                       api2windsurf.exe
                       拦截 → 解码 → 翻译 → 转发
                              │
                              ▼
                        你的 API 端点
```

程序通过修改系统 hosts 文件，把 Windsurf 的后端域名指向本机，然后在 443 端口用自签证书终结 TLS。只有对话相关的请求会被改写转发；登录、账号状态、遥测等其它请求原样透传，不影响 IDE 其它功能。

协议细节见 [docs/cascade-protocol.md](docs/cascade-protocol.md) 和 [docs/architecture.md](docs/architecture.md)。

## 项目结构

```
api2windsurf/
├─ main.go              Wails 入口
├─ internal/app/        应用逻辑（配置、生命周期、绑定 API）
├─ internal/proxy/      代理核心（证书、hosts、MITM 服务、provider 转译）
├─ internal/protocol/   Cascade protobuf 编解码
├─ frontend/            配置界面（React + Vite）
├─ docs/                协议文档
└─ .github/workflows/   CI 和发布
```

## 测试

```bash
go test ./...
```
