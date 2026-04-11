# 架构设计

## Scope Challenge

### 已有能力

OpenClaw Manager Native 已经有一套 Go daemon 账号池能力：

- 发现 `.openclaw` 和 `.openclaw-*` profile
- 创建账号槽位
- Codex OAuth 登录
- 额度探测和短缓存
- 切换账号并同步默认位
- 移除槽位并归档本地目录
- 运行 auth selection 可视化

这些逻辑可以复用思路和部分代码，但不应直接复制整份 daemon。当前 daemon 同时包含 skills、诊断、watchdog、机器监控和 macOS 服务维护，边界太宽。

### 最小目标

第一阶段只抽账号池核心：

- profile 文件系统
- auth store 读写
- OAuth 登录
- quota 探测
- activate/sync default
- remove/archive
- CLI 和 local HTTP API

### 复杂度控制

第一阶段最多引入两个核心抽象：

- `AccountPool`: 账号池业务入口
- `Platform`: 跨平台路径、浏览器、权限适配

如果一开始拆出过多 service，会让项目还没跑起来就过度工程化。

## 总体架构

```text
┌──────────────────────────────────────────────┐
│ User Interfaces                              │
│ CLI / Local Web UI / future desktop shells   │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│ Local API                                    │
│ /api/profiles /api/login /api/settings       │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│ AccountPool Core                            │
│ create/login/probe/activate/remove/import    │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│ Storage + Platform Adapters                  │
│ profile dirs / auth store / browser / paths  │
└──────────────────────────────────────────────┘
```

## 模块边界

```text
cmd/token-manager
  CLI entrypoint

internal/server
  local API + embedded browser UI

internal/accountpool
  profile discovery, scaffold, archive, OAuth, quota, default mirror sync

internal/platform
  OS paths, browser open, file permissions
```

## 运行模式

### Standalone 模式

无 OpenClaw 也能用。

```text
token-manager list
token-manager login acct-a
token-manager probe acct-a
token-manager remove acct-a
```

可用能力：

- 创建账号槽位
- OAuth 登录
- 保存认证文件
- 查看账号和额度
- 移除本地槽位
- 导入导出

隐藏或禁用能力：

- OpenClaw config validate
- skills
- gateway
- watchdog
- doctor/fix

### OpenClaw Compatible 模式

检测到 OpenClaw 或用户启用兼容同步后，把账号池同步成 OpenClaw/Codex 运行时能读的默认位。

```text
managed profile dirs
        |
        v
default .openclaw + default .codex
        |
        v
OpenClaw / Codex runtime
```

## 核心数据流

```text
create profile
      |
      v
write .openclaw-name + .codex-name
      |
      v
login oauth
      |
      v
write auth store
      |
      v
probe quota
      |
      v
activate
      |
      v
sync default mirror
      |
      v
remove/archive
      |
      v
profile disappears, archive not rediscovered
```

## 目录策略

### 账号槽位目录

继续兼容现有约定：

```text
macOS/Linux:
~/.openclaw
~/.openclaw-acct-a
~/.codex
~/.codex-acct-a

Windows:
%USERPROFILE%\.openclaw
%USERPROFILE%\.openclaw-acct-a
%USERPROFILE%\.codex
%USERPROFILE%\.codex-acct-a
```

### Manager 状态目录

不要放在账号扫描根目录下面，避免归档被重新发现。

```text
macOS:
~/Library/Application Support/Token Manager Tools

Windows:
%APPDATA%\Token Manager Tools

Linux:
$XDG_STATE_HOME/token-manager-tools
fallback: ~/.local/state/token-manager-tools
```

### 归档目录

```text
{manager-state}/removed-profiles/{timestamp}-{profile}
```

归档目录必须被 discovery 忽略。

## API 草案

```text
GET  /api/health
GET  /api/profiles
POST /api/profiles
POST /api/profiles/{name}/login/start
GET  /auth/callback
POST /api/profiles/{name}/probe
POST /api/profiles/{name}/activate
POST /api/profiles/{name}/remove
POST /api/export
POST /api/import
GET  /api/settings
PATCH /api/settings
```

返回对象要避免暴露完整 token。UI 只能看到：

- profile name
- account email
- account id short form
- quota windows
- status
- status reason
- local path
- last error summary

OAuth verifier/state 留在本地服务内存，不返回给浏览器 API。
`serve` 默认只允许 loopback 监听；远程访问必须显式设置 `TOKEN_MANAGER_ALLOW_REMOTE=1`。

## CLI 草案

```text
token-manager list
token-manager create acct-a
token-manager login acct-a
token-manager probe acct-a
token-manager activate acct-a
token-manager remove acct-a
token-manager export ./pool.zip
token-manager import ./pool.zip
token-manager serve
token-manager doctor
```

`doctor` 第一版只检查账号池自身，不检查 OpenClaw gateway。

## Token 安全

V1 采用文件兼容模式：

- 写 OpenClaw/Codex 兼容 auth 文件
- 文件权限尽量收紧
- 导出时强制确认
- 默认不上传、不同步、不共享

V2 再考虑系统凭据：

- macOS Keychain
- Windows Credential Manager
- Linux Secret Service

不要第一版就强制使用系统钥匙串，因为 OpenClaw/Codex 运行时仍可能需要文件兼容。

## OpenClaw 可选集成

OpenClaw 不存在时：

```text
状态: 未安装 OpenClaw
可用: 账号池管理
不可用: 运行诊断、skills、gateway、watchdog
```

OpenClaw 存在时：

```text
状态: 已检测到 OpenClaw
可用: 默认位同步、兼容运行、可选配置检查
```

## NOT in Scope

- 完整 OpenClaw Manager Native 移植：会把账号池和诊断维护混在一起。
- 云同步：token 风险高，第一版不碰。
- 团队共享：涉及权限、审计和加密，后置。
- Windows 服务常驻：先用 CLI/local server 验证。
- 原生 Windows/macOS/Linux 桌面壳：先用本地 Web UI 降低交付成本。
- skills marketplace：属于 OpenClaw Manager，不属于账号池核心。
- gateway/watchdog：属于本机维护，不属于账号池核心。

## 主要风险

### token 存储风险

本地 token 文件必须明确权限和导出提示。任何导入导出都要让用户知道里面包含认证资料。

### Windows OAuth callback

Windows 防火墙或浏览器策略可能拦截 callback。本地 Web UI 必须提供手动复制 callback code 的备选流程。

### 目录发现误扫

归档目录、备份目录、manager 状态目录必须从 profile discovery 中排除。

### 兼容默认位

同步默认 `.openclaw` / `.codex` 不能覆盖用户的非账号池配置。需要保留未知字段。

### OpenClaw 不存在

不能把无 OpenClaw 判成错误。它只是 standalone 模式。
