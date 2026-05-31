# 架构

## 整体结构

这是一个 Wails 桌面应用（Go 后端 + React 前端），编译后是单个 Windows exe。它在本机运行一个代理，拦截 Windsurf 的聊天请求并转发到用户指定的 API。

```
Windsurf IDE ──▶ 本机代理（:443）──▶ 用户的 API 端点
```

## 启动过程

1. 程序启动时请求管理员权限（exe 内嵌了 UAC 清单）。
2. 生成自签 CA 证书并安装到系统信任区。
3. 修改 hosts 文件，把 Windsurf 的后端域名指向 127.0.0.1。
4. 在 443 端口启动代理，用自签证书接收 HTTPS 请求。
5. 弹出配置界面。

## 退出与还原

退出或调用「恢复官方环境」时会执行 `TeardownSystem`：删除 hosts 中的 `# api2windsurf` 行、从系统信任区卸载本地 CA、移除为劫持添加的代理例外项，并刷新 DNS。仅「停止代理」时会还原 hosts 与代理例外项，但保留 CA 以便下次快速启动。

## 请求处理

代理收到请求后，根据路径判断是否需要处理：

- 聊天请求（GetChatMessage 等）：解码 protobuf，提取消息内容，翻译成目标 API 格式，转发并把响应翻译回来。
- 其他请求（登录、状态查询、插件等）：直接转发到 Windsurf 真实后端，不做修改。

## 模型覆盖

Windsurf 在请求里会带上用户选择的模型名。代理会忽略这个值，替换成用户在配置页面设置的模型。所以在 Windsurf 里选哪个模型不影响实际调用。

## 思考过程

Cascade 协议的响应帧里有一个 `delta_thinking` 字段（字段号 9）。当上游模型返回思考内容时，代理把它写入这个字段。Windsurf 会用原生的可折叠思考块来显示。

## 目录说明

| 路径 | 作用 |
|------|------|
| `main.go` | 程序入口，内嵌前端资源 |
| `internal/app/` | 应用逻辑：配置管理、生命周期、前端 API |
| `internal/proxy/` | 代理核心：证书、hosts、MITM 服务、provider 协议翻译 |
| `internal/protocol/` | Cascade protobuf 编解码 |
| `frontend/` | React 配置界面 |
