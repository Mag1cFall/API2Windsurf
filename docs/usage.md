# 使用说明

## 启动

运行 `api2windsurf.exe`，系统会弹出管理员权限请求，点「是」。

程序需要管理员权限来安装证书、修改 hosts 文件和监听 443 端口。启动后这些会自动完成，同时弹出配置页面。

## 配置

在配置页面填写以下信息：

| 字段 | 说明 | 例子 |
|------|------|------|
| Base URL | API 地址 | `http://127.0.0.1:8317` |
| API Key | 密钥 | 你的密钥 |
| 协议类型 | API 使用的协议 | OpenAI 兼容 / Anthropic / Gemini |
| 模型 | 要使用的模型名 | `claude-opus-4.7` |

填好后：
1. 点「拉取模型」可以从 API 获取可用模型列表。
2. 点「测试连接」确认配置正确。
3. 点「保存并应用」。

## 在 Windsurf 里使用

保存配置后，关闭 Windsurf 再重新打开。

打开 Cascade 聊天，选任意一个自带模型，正常聊天即可。请求会自动转发到你配置的模型。

注意：不要在 Windsurf 的设置里配置 BYOK 或自定义 provider。那样做会让请求绕过本工具。只需要选自带模型就行。

## 思考过程

如果你的模型支持思考（比如 Claude 的 thinking 或 DeepSeek R1），勾选配置页面的「透传思考过程」选项。思考内容会显示在 Windsurf 原生的可折叠思考块里。不需要可以取消勾选。

## 确认是否生效

配置页面底部有用量统计，显示通过本工具转发的请求数和 token 数。在 Windsurf 里发一条消息后，这里出现记录就说明配置成功。

## 恢复官方账号

使用自定义 API 时，本工具会修改系统 hosts 并安装本地 CA。若未还原，关闭程序后 Windsurf 仍会把后端域名解析到本机，导致无法登录或使用官方模型。

**自动还原（推荐）**

- 直接退出 api2windsurf（点标题栏 ✕）：会自动移除 hosts 劫持、代理例外项，并卸载本地 CA。
- 点「停止代理」：会停止本机代理并还原 hosts，便于在不退出程序时切回官方 Windsurf。

**手动还原**

在配置页点「恢复官方环境」，效果与退出时相同。完成后请**完全退出** Windsurf 进程再重新打开。

若程序异常退出导致 hosts 未还原，可用管理员 PowerShell 执行：

```powershell
$hosts = "C:\Windows\System32\drivers\etc\hosts"
(Get-Content $hosts) | Where-Object { $_ -notmatch 'api2windsurf' } | Set-Content $hosts
ipconfig /flushdns
certutil -delstore Root "API2Windsurf Local CA"
```

## 常见问题

**关闭 api2windsurf 后 Windsurf 仍无法使用官方账号**

确认 hosts 中已无 `# api2windsurf` 行（见上文手动还原），并完全重启 Windsurf。

**保存后 Windsurf 没变化**
需要完全退出 Windsurf 进程再重新打开，不是只关窗口。

**测试连接报连接被拒绝**
检查 Base URL 的地址和端口是否正确，确认 API 服务在运行。

**测试连接报 401**
API Key 不对。

**拉取模型列表为空**
有些 API 不支持模型列表查询，直接在模型框里手动输入模型名即可。
