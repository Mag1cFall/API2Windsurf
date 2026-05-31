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

使用自定义 API 时，本工具会修改系统 hosts 并安装本地 CA。**只有 hosts 劫持会让 Windsurf 连不上官方后端**——CA 证书在没有代理的情况下不会改变任何流量，只是个躺在系统信任区里的占位条目。

**自动还原（推荐）**

- 直接退出 api2windsurf（点标题栏 ✕）：自动移除 hosts 劫持与代理例外项。
- 点「停止代理」：停代理并还原 hosts，便于在不退出程序时切回官方 Windsurf。
- 点「恢复官方环境」：与退出等效，立即还原。

完成后请**完全退出** Windsurf 进程再重新打开（任务管理器里所有 Windsurf/Codeium 进程都要结束）。

**关于本机 CA**

CA 证书默认**不会**被自动卸载，避免每次启动都触发 UAC 弹窗重装。它只在 api2windsurf 运行并劫持指定域名时才生效，平时无害。若想彻底清掉：

- `certmgr.msc` → 受信任的根证书颁发机构 → 删除「API2Windsurf Local CA」
- 或管理员 PowerShell：`certutil -delstore Root "API2Windsurf Local CA"`

**程序异常退出后的兜底**

若 hosts 未自动还原，用管理员 PowerShell：

```powershell
$hosts = "C:\Windows\System32\drivers\etc\hosts"
(Get-Content $hosts) | Where-Object { $_ -notmatch 'api2windsurf' } | Set-Content $hosts
ipconfig /flushdns
```

## 常见问题

**关闭 api2windsurf 后 Windsurf 仍无法使用官方账号**

1. 打开本程序，配置页中部会列出 hosts 里所有把 Windsurf/Codeium 域名指向回环地址的劫持条目，并标注是哪个工具加的。常见的「外部劫持」标记包括 `windsurf-tools-mitm`、`api2windsurf` 等。点「清理所有 Windsurf 劫持」即可一次性删掉（无论是哪个工具加的）。
2. 任务管理器里**完全结束** Windsurf / Codeium 所有进程，再重新打开——已建立的 TCP 连接会一直指向 127.0.0.1，仅关窗口不够。
3. 若 Windsurf 设置里手动配过 BYOK / 自定义 Provider，请删掉。
4. 如果你装了 Clash / V2Ray 等系统代理，并启用了 fake-ip 模式，本程序停止后域名解析会指向 `198.18.x.x` 之类的伪 IP，这是正常现象，请求会经由你的代理出口走出去。

**为什么有时退出了还连不上**

最常见的原因不是 api2windsurf 本身——而是**另一个 MITM 工具**在过去把同样的域名劫持到了本机却忘了还原，标记不是 `# api2windsurf` 因此不会被本程序识别。1.0.2 起本程序会扫描并显示**所有**对 Windsurf 域名的回环劫持条目，可以一键清理。

**保存后 Windsurf 没变化**
需要完全退出 Windsurf 进程再重新打开，不是只关窗口。

**测试连接报连接被拒绝**
检查 Base URL 的地址和端口是否正确，确认 API 服务在运行。

**测试连接报 401**
API Key 不对。

**拉取模型列表为空**
有些 API 不支持模型列表查询，直接在模型框里手动输入模型名即可。
