# Cascade 协议笔记

Windsurf IDE 与其后端之间使用一种基于 protobuf 的流式协议进行聊天通信。本文档记录该协议的结构，用于实现透明的请求转发。

所有内容为协议结构知识，不含任何账号凭证或密钥。

---

## 数据流

```
Windsurf IDE
   │  gRPC/Connect over HTTPS
   ▼
server.self-serve.windsurf.com  ← hosts 指向 127.0.0.1:443
   │
   ▼
本代理（MITM，持自签 CA）
   │  解码 protobuf → 翻译为标准 API 请求
   ▼
用户的 API 端点
```

只有聊天相关路径（Chat / Cortex / Trajectory）会被改写转发。登录、状态查询、插件等请求原样透传。

---

## 请求结构：GetChatMessage

Endpoint：`/exa.api_server_pb.ApiServerService/GetChatMessage`
Content-Type：`application/connect+proto`（流式）

### 帧格式

每帧 5 字节头：

| 偏移 | 长度 | 含义 |
|------|------|------|
| 0    | 1    | flags：bit0=gzip，bit1=end-stream |
| 1-4  | 4    | payload 长度（big-endian） |
| 5..  | N    | payload（protobuf 或 gzip 压缩后的 protobuf） |

### 请求体字段

| 字段 | 类型 | 含义 |
|------|------|------|
| F1   | message | metadata（认证、客户端信息） |
| F2   | string  | system prompt |
| F3   | message (repeated) | 聊天消息列表 |
| F8   | message | generation config |
| F10  | message (repeated) | 工具定义 |
| F15  | message | conversation context |
| F21  | string  | 模型名（IDE 选定的，会被代理覆盖） |
| F22  | string  | message ID |

单条消息（F3 子消息）：

| 字段 | 类型 | 含义 |
|------|------|------|
| F2   | varint | role：1=user，2=assistant，4=tool |
| F3   | string | 文本内容 |
| F4   | varint | index |
| F6   | message | tool_use |
| F7   | string | tool_use_id |

---

## 响应结构：GetChatMessageResponse

| 字段 | 名称 | 类型 | 含义 |
|------|------|------|------|
| F1   | message_id | string | bot 消息 ID |
| F2   | timestamp | message | 时间戳 |
| **F3** | **delta_text** | string | **正文增量** |
| F4   | delta_tokens | uint32 | 本帧 token 数 |
| F5   | stop_reason | enum | 结束原因（非 0 表示结束） |
| F6   | delta_tool_calls | repeated | 工具调用增量 |
| F7   | usage | message | token 用量 |
| **F9** | **delta_thinking** | string | **思考增量** |
| F10  | delta_signature | string | 思考签名 |
| F11  | thinking_redacted | bool | 思考是否被隐藏 |
| F15  | output_id | string | 输出 ID |
| F16  | thinking_id | string | 思考块 ID |
| F17  | request_id | string | 请求 ID |

### 关于 F9（delta_thinking）

响应里有独立的思考字段。上游模型返回的思考内容（OpenAI 的 `reasoning_content`、Anthropic 的 `thinking`）写入 F9，正文写入 F3。Windsurf 会把 F9 的内容显示为可折叠的思考块，和官方 thinking 模型的表现一致。

### 流终止

- 正常数据帧：flags=0x00 或 0x01（gzip）。
- 结束标志：F5 非 0。
- EOS 帧：flags=0x02，payload 为 JSON（成功 `{}`，失败 `{"error":{...}}`）。

---

## 上游协议翻译

| Provider | 请求地址 | 思考字段来源 |
|----------|----------|-------------|
| OpenAI 兼容 | `POST {base}/v1/chat/completions` | `delta.reasoning_content` |
| Anthropic | `POST {base}/v1/messages` | `thinking_delta` |
| Google Gemini | `POST {base}/v1beta/models/{model}:streamGenerateContent` | thought parts |

三种来源的思考内容统一写入 Cascade 响应的 F9。

---

## 参考

- Connect 协议：https://connectrpc.com/docs/protocol
- Protobuf wire format：https://protobuf.dev/programming-guides/encoding/
