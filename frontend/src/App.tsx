import { useCallback, useEffect, useRef, useState } from 'react'
import { Runtime } from './runtime'

type Config = {
  base_url: string
  api_key: string
  provider: string
  model: string
  port: number
  show_reasoning?: boolean
  max_tokens?: number
}

type HostsHijack = {
  domain: string
  ip: string
  marker: string
  line: string
}

type Status = {
  ca_installed: boolean
  hosts_mapped: boolean
  proxy_running: boolean
  config_valid: boolean
  config_error: string
  listen_address: string
  upstream: string
  model: string
  config_path: string
  hosts_hijacks?: HostsHijack[]
  foreign_hijack?: boolean
  hijack_scan_error?: string
}

type ModelsResult = { models: string[]; count: number; error: string }
type TestResult = { ok: boolean; duration_ms: number; model_count: number; detail: string; error: string }
type UsageRecord = {
  at: string
  model: string
  request_model: string
  total_tokens: number
  duration_ms: number
  status: string
  error_detail?: string
}
type Usage = { total_requests: number; total_tokens: number; error_count: number; recent: UsageRecord[] }

const api = () => (window as unknown as { go: { app: { App: Record<string, (...args: unknown[]) => Promise<unknown>> } } }).go.app.App

type ToastKind = 'ok' | 'err' | 'info' | 'busy'
type Toast = { id: number; kind: ToastKind; text: string }

type Theme = 'dark' | 'light'

type DocVT = Document & {
  startViewTransition?: (cb: () => void) => { ready: Promise<void> }
}

function useTheme(): [Theme, (e?: { clientX: number; clientY: number }) => void] {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem('byok-theme')
    return saved === 'light' ? 'light' : 'dark'
  })
  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem('byok-theme', theme)
  }, [theme])

  const toggle = useCallback((e?: { clientX: number; clientY: number }) => {
    const doc = document as DocVT
    const flip = () => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))

    // 无 View Transitions 支持(老 WebView2):退回「一帧禁用过渡同步换色」。
    if (!doc.startViewTransition || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      const root = document.documentElement
      root.classList.add('no-anim')
      flip()
      requestAnimationFrame(() => requestAnimationFrame(() => root.classList.remove('no-anim')))
      return
    }

    // 从点击点为圆心,做整屏圆形揭开。半径取到屏幕最远角。
    const x = e?.clientX ?? window.innerWidth - 60
    const y = e?.clientY ?? 28
    const end = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))

    const transition = doc.startViewTransition(() => {
      flip()
    })
    void transition.ready.then(() => {
      document.documentElement.animate(
        {
          clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${end}px at ${x}px ${y}px)`],
        },
        {
          duration: 480,
          easing: 'cubic-bezier(0.4, 0, 0.2, 1)',
          pseudoElement: '::view-transition-new(root)',
        },
      )
    })
  }, [])

  return [theme, toggle]
}

export default function App() {
  const [cfg, setCfg] = useState<Config>({
    base_url: 'http://127.0.0.1:8317',
    api_key: '',
    provider: 'openai',
    model: 'claude-opus-4.7',
    port: 443,
    show_reasoning: true,
    max_tokens: 0,
  })
  const [st, setSt] = useState<Status | null>(null)
  const [usage, setUsage] = useState<Usage | null>(null)
  const [models, setModels] = useState<string[]>([])
  const [toasts, setToasts] = useState<Toast[]>([])
  const [busy, setBusy] = useState(false)
  const [theme, toggleTheme] = useTheme()
  const dirty = useRef(false)
  const toastSeq = useRef(0)

  const pushToast = useCallback((kind: ToastKind, text: string) => {
    const id = ++toastSeq.current
    setToasts((ts) => [...ts, { id, kind, text }])
    if (kind !== 'busy') {
      setTimeout(() => setToasts((ts) => ts.filter((t) => t.id !== id)), 3200)
    }
    return id
  }, [])

  const dropToast = useCallback((id: number) => {
    setToasts((ts) => ts.filter((t) => t.id !== id))
  }, [])

  const refresh = useCallback(async () => {
    try {
      const [status, usageData] = await Promise.all([api().GetStatus() as Promise<Status>, api().GetUsage() as Promise<Usage>])
      setSt(status)
      setUsage(usageData)
      if (!dirty.current) {
        const config = (await api().GetConfig()) as Config
        setCfg(config)
      }
    } catch (e) {
      pushToast('err', String(e))
    }
  }, [pushToast])

  useEffect(() => {
    void refresh()
    const t = setInterval(() => void refresh(), 3000)
    return () => clearInterval(t)
  }, [refresh])

  const run = async (label: string, fn: () => Promise<unknown>, okText: string) => {
    setBusy(true)
    const busyId = pushToast('busy', `${label}中…`)
    try {
      await fn()
      dropToast(busyId)
      pushToast('ok', okText)
      await refresh()
    } catch (e) {
      dropToast(busyId)
      pushToast('err', `${label}失败：${String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  const onField = (patch: Partial<Config>) => {
    dirty.current = true
    setCfg((c) => ({ ...c, ...patch }))
  }

  const fetchModels = async () => {
    setBusy(true)
    const busyId = pushToast('busy', '拉取模型列表中…')
    try {
      const r = (await api().FetchModels(cfg)) as ModelsResult
      dropToast(busyId)
      if (r.error) {
        pushToast('err', `拉取失败：${r.error}`)
      } else {
        setModels(r.models)
        const next = r.models.includes(cfg.model) ? cfg.model : (r.models[0] ?? cfg.model)
        onField({ model: next })
        pushToast('ok', `拉到 ${r.count} 个模型，可在下拉框选择`)
      }
    } catch (e) {
      dropToast(busyId)
      pushToast('err', `拉取失败：${String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  const testConn = async () => {
    setBusy(true)
    const busyId = pushToast('busy', '测试连接中…')
    try {
      const r = (await api().TestConnection(cfg)) as TestResult
      dropToast(busyId)
      if (r.ok) {
        pushToast('ok', r.detail)
      } else {
        pushToast('err', `连接失败：${r.error}`)
      }
    } catch (e) {
      dropToast(busyId)
      pushToast('err', `连接失败：${String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  const save = async () => {
    await run('保存配置', () => api().SaveConfig(cfg), '已保存并重启代理。请完全退出并重开 Windsurf 生效')
    dirty.current = false
  }

  const allReady = st?.ca_installed && st?.hosts_mapped && st?.proxy_running && st?.config_valid

  return (
    <div className="app">
      <div className="titlebar">
        <div className="tb-drag">
          <span className="tb-dot" />
          <span className="tb-title">API2Windsurf</span>
        </div>
        <div className="tb-actions">
          <button
            className={`tb-btn theme-btn ${theme}`}
            onClick={(e) => toggleTheme({ clientX: e.clientX, clientY: e.clientY })}
            title={theme === 'dark' ? '切到亮色' : '切到暗色'}
            aria-label="切换主题"
          >
            <span className="theme-ico" key={theme}>
              {theme === 'dark' ? (
                <svg
                  viewBox="0 0 24 24"
                  width="15"
                  height="15"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
                </svg>
              ) : (
                <svg
                  viewBox="0 0 24 24"
                  width="15"
                  height="15"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <circle cx="12" cy="12" r="4" />
                  <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
                </svg>
              )}
            </span>
          </button>
          <button className="tb-btn" onClick={() => Runtime.minimise()} title="最小化">
            ─
          </button>
          <button className="tb-btn close" onClick={() => Runtime.quit()} title="退出">
            ✕
          </button>
        </div>
      </div>

      <main className="content">
        <header className="head">
          <div>
            <h1>API2Windsurf</h1>
          </div>
          <span className={`badge ${allReady ? 'ok' : 'warn'}`}>
            <i className="blip" />
            {allReady ? '就绪' : '未就绪'}
          </span>
        </header>

        <section className="card">
          <h2>运行状态</h2>
          <div className="pills">
            <span className={`pill ${st?.ca_installed ? 'ok' : 'bad'}`} title="根证书是否已装入系统信任区">
              CA 证书 {st?.ca_installed ? '已信任' : '未就绪'}
            </span>
            <span className={`pill ${st?.hosts_mapped ? 'ok' : 'bad'}`} title="是否已把 Windsurf 域名劫持到本机">
              Hosts 劫持 {st?.hosts_mapped ? '已生效' : '未就绪'}
            </span>
            <span className={`pill ${st?.proxy_running ? 'ok' : 'bad'}`} title="本机 443 端口的 MITM 代理是否在跑">
              代理 {st?.proxy_running ? '运行中' : '已停止'}
            </span>
            <span className={`pill ${st?.config_valid ? 'ok' : 'bad'}`} title="当前配置是否完整有效">
              配置 {st?.config_valid ? '有效' : '不完整'}
            </span>
          </div>
          {st && (
            <div className="kv">
              <div>
                <span>监听地址</span>
                <code>{st.listen_address}</code>
              </div>
              <div>
                <span>当前上游</span>
                <code>{st.upstream || '—'}</code>
              </div>
              <div>
                <span>当前模型</span>
                <code>{st.model || '—'}</code>
              </div>
              <div>
                <span>配置文件</span>
                <code>{st.config_path}</code>
              </div>
            </div>
          )}
          {st && !st.config_valid && st.config_error && <p className="inline err">⚠ {st.config_error}</p>}
          <div className="row mt">
            <button
              className="btn ghost"
              disabled={busy}
              onClick={() => run('安装 CA / Hosts', () => api().SetupSystem(), '系统组件已就绪')}
            >
              安装 CA / Hosts
            </button>
            <button className="btn ghost" disabled={busy} onClick={() => run('启动代理', () => api().StartProxy(), '代理已启动')}>
              启动代理
            </button>
            <button
              className="btn ghost"
              disabled={busy}
              onClick={() =>
                run('停止代理', () => api().StopProxy(), '代理已停止，hosts 已还原。可正常使用 Windsurf 官方账号')
              }
            >
              停止代理
            </button>
            <button
              className="btn danger"
              disabled={busy}
              onClick={() => {
                if (
                  !confirm(
                    '将停止代理并移除 hosts 劫持与代理例外项，Windsurf 可恢复使用官方账号。\n\n本机 CA 证书会保留（无害；下次启动免去再次 UAC 安装）。\n\n请完全退出并重开 Windsurf。继续？',
                  )
                ) {
                  return
                }
                void run(
                  '恢复官方环境',
                  () => api().RestoreOfficialEnvironment(),
                  '已恢复官方环境。请完全退出并重开 Windsurf',
                )
              }}
            >
              恢复官方环境
            </button>
          </div>
          <p className="hint">退出本程序或点「停止代理」会自动还原 hosts，让 Windsurf 重新连官方后端。本机 CA 不会被自动卸载，需要时去 certmgr.msc 里手动删除。</p>

          {st?.hosts_hijacks && st.hosts_hijacks.length > 0 && (
            <div className={`hijack-panel ${st.foreign_hijack ? 'foreign' : ''}`}>
              <div className="hijack-head">
                <b>检测到 hosts 中的 Windsurf 域名劫持</b>
                {st.foreign_hijack && (
                  <span className="hijack-warn">⚠ 含非本程序添加的条目（其他工具残留），仅清自己的标记不能恢复 Windsurf</span>
                )}
              </div>
              <ul className="hijack-list">
                {st.hosts_hijacks.map((h, i) => (
                  <li key={i}>
                    <code>{h.ip}</code> <code>{h.domain}</code>
                    <span className={`tag ${h.marker.toLowerCase() === 'api2windsurf' ? 'self' : 'foreign'}`}>
                      {h.marker || '无标记'}
                    </span>
                  </li>
                ))}
              </ul>
              <div className="row mt">
                <button
                  className="btn danger"
                  disabled={busy}
                  onClick={() => {
                    if (
                      !confirm(
                        '将清除上面列出的所有 Windsurf/Codeium 域名劫持（无论是哪个工具加的）。\n\n仅删除指向 127.0.0.1 等回环地址的劫持条目，其它 hosts 内容不动。继续？',
                      )
                    ) {
                      return
                    }
                    void run(
                      '清理所有 Windsurf 劫持',
                      async () => {
                        const removed = (await api().PurgeAllHostsHijacks()) as HostsHijack[] | null
                        return removed
                      },
                      '已清除 hosts 中的 Windsurf 劫持。请完全退出并重开 Windsurf',
                    )
                  }}
                >
                  清理所有 Windsurf 劫持
                </button>
              </div>
            </div>
          )}
          {st?.hijack_scan_error && <p className="inline err">⚠ 扫描 hosts 失败：{st.hijack_scan_error}</p>}
        </section>

        <section className="card">
          <h2>上游 API 配置</h2>
          <div className="grid">
            <label>
              Base URL
              <input value={cfg.base_url} onChange={(e) => onField({ base_url: e.target.value })} placeholder="http://127.0.0.1:8317" />
            </label>
            <label>
              API Key
              <input type="password" value={cfg.api_key} onChange={(e) => onField({ api_key: e.target.value })} placeholder="你的密钥" />
            </label>
            <label>
              协议类型
              <select value={cfg.provider} onChange={(e) => onField({ provider: e.target.value })}>
                <option value="openai">OpenAI 兼容 (/v1/chat/completions)</option>
                <option value="anthropic">Anthropic (/v1/messages)</option>
                <option value="google">Google Gemini</option>
              </select>
            </label>
            <label>
              模型
              <div className="modelrow">
                {models.length > 0 ? (
                  <select value={cfg.model} onChange={(e) => onField({ model: e.target.value })}>
                    {!models.includes(cfg.model) && <option value={cfg.model}>{cfg.model}（手填）</option>}
                    {models.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input value={cfg.model} onChange={(e) => onField({ model: e.target.value })} placeholder="claude-opus-4.7" />
                )}
                <button className="btn mini" disabled={busy} onClick={fetchModels} title="向上游拉取可用模型列表填充下拉框">
                  拉取模型
                </button>
              </div>
            </label>
            <label>
              最大输出 tokens（留空 = 不限制，用 Windsurf 默认）
              <input
                type="number"
                min={0}
                value={cfg.max_tokens ? cfg.max_tokens : ''}
                onChange={(e) => onField({ max_tokens: e.target.value ? Number(e.target.value) : 0 })}
                placeholder="不限制"
              />
            </label>
          </div>
          <label className="check">
            <input type="checkbox" checked={cfg.show_reasoning !== false} onChange={(e) => onField({ show_reasoning: e.target.checked })} />
            <span>
              透传思考过程（把模型的 reasoning/thinking 写入 Cascade 原生 delta_thinking 字段，Windsurf
              用其原生可折叠思考块渲染；关闭则只显示最终答案）
            </span>
          </label>
          <div className="row mt">
            <button className="btn primary" disabled={busy} onClick={save}>
              保存并应用
            </button>
            <button className="btn ghost" disabled={busy} onClick={testConn}>
              测试连接
            </button>
          </div>
          <p className="hint">提示：在 Windsurf 里选哪个模型都行——代理会强制改写成上面这个模型。真正的「切换模型」在这里做。</p>
        </section>

        <section className="card">
          <h2>用量统计（外部模型是否真在被调用）</h2>
          {usage ? (
            <>
              <div className="stats">
                <div className="stat">
                  <b>{usage.total_requests}</b>
                  <span>总请求</span>
                </div>
                <div className="stat">
                  <b>{usage.total_tokens}</b>
                  <span>总 tokens</span>
                </div>
                <div className={`stat ${usage.error_count > 0 ? 'bad' : ''}`}>
                  <b>{usage.error_count}</b>
                  <span>失败次数</span>
                </div>
              </div>
              {usage.recent.length > 0 ? (
                <div className="log">
                  {usage.recent.map((r, i) => (
                    <div key={i} className={`logitem ${r.status === 'ok' ? '' : 'err'}`}>
                      <span className="dot" />
                      <code>{r.model || r.request_model}</code>
                      <span className="meta">
                        {r.total_tokens}tok · {r.duration_ms}ms
                      </span>
                      {r.status !== 'ok' && r.error_detail && <span className="err-detail">{r.error_detail}</span>}
                    </div>
                  ))}
                </div>
              ) : (
                <p className="hint">还没有请求记录。去 Windsurf 里发一条 Cascade 消息试试。</p>
              )}
            </>
          ) : (
            <p className="hint">加载中…</p>
          )}
        </section>
      </main>

      <div className="toasts">
        {toasts.map((t) => (
          <div key={t.id} className={`toast ${t.kind}`} onClick={() => dropToast(t.id)}>
            {t.kind === 'busy' && <span className="spinner" />}
            {t.text}
          </div>
        ))}
      </div>
    </div>
  )
}
