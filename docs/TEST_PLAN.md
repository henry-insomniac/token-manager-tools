# 测试计划

## 测试目标

账号池必须在没有 OpenClaw 的情况下独立可用，并在安装 OpenClaw 后保持兼容。

## 核心路径

```text
create
  |
  v
login
  |
  v
probe
  |
  v
activate
  |
  v
sync default
  |
  v
remove/archive
```

## 单测矩阵

### Profile Discovery

- 发现默认 `.openclaw`
- 发现 `.openclaw-*`
- 忽略 manager state
- 忽略 archive
- 忽略 backup
- Windows 路径可正常解析

### Profile Scaffold

- 创建新槽位
- 已存在槽位不覆盖用户文件
- 默认配置缺失时写最小配置
- Codex companion 目录可选创建

### Auth Store

- 读取空 auth store
- 写入 Codex OAuth credential
- 保留未知字段
- 不在 API response 暴露完整 token

### OAuth

- 生成 authorization URL
- callback state 校验
- token exchange 成功
- token exchange 失败
- manual login fallback

当前覆盖：

- 已用 `httptest` 覆盖 authorization URL、token exchange、token 持久化。
- 已覆盖手动登录输入解析、state 校验和 raw code 兜底。
- 已覆盖 loopback 回调地址和浏览器入口统一为 `localhost` 的生成逻辑。
- 尚未覆盖 CLI callback server 端到端。

### Quota

- 额度接口成功
- 401/403 触发重新登录状态
- API 失败时可用缓存
- unknown 不误算成风险账号

当前覆盖：

- 已用 `httptest` 覆盖 usage 请求头和额度解析。
- 已覆盖 401 后 refresh token、持久化新 access token、重试 usage。
- 尚未实现 usage 缓存。

### Activate

- 切换 managed profile
- 同步 default mirror
- 保留 default 非 Codex 配置
- active-first 排序

### Remove

- active profile 不能移除
- pending login profile 不能移除
- state dir 被归档
- codex dir 被归档
- default auth pool 清掉已移除 profile
- archive 不会被重新发现

### Import / Export

- 导出包含必要账号资料
- 导出提示 token 风险
- 导入重复账号时给出策略
- 导入失败不破坏现有池

### Local API / Web

- `GET /api/profiles` 不返回 token
- `POST /api/profiles` 可创建槽位
- `POST /api/profiles/{name}/login/start` 不返回 verifier
- `/auth/callback` 对未知 state 给出可操作页面
- Web UI 可执行创建、登录入口、检查、切换、移除

当前覆盖：

- 已覆盖 API 创建/列表。
- 已覆盖 login-start 不泄露 verifier。
- 已覆盖 login-complete 手动写入流程。
- 已覆盖未知 callback state 页面。
- 尚未做浏览器端端到端自动化。

## 集成测试矩阵

```text
macOS:
  - CLI
  - serve
  - browser open
  - OpenClaw compatible sync

Windows:
  - CLI
  - serve
  - default browser open
  - manual login fallback
  - %APPDATA% state dir

Linux:
  - CLI
  - serve
  - xdg-open fallback
  - no desktop manual mode
  - XDG state dir
```

## UX 验收

- 无 OpenClaw 时显示 standalone 状态，不报错。
- 移除动作始终写“移除本机槽位”，不写“删除账号”。
- 登录失败要告诉用户下一步。
- 导出账号池要提示包含本机认证资料。
- 列表只展示关键摘要，详情页展示完整路径和错误原文。

## 性能验收

- `list` 不应每次都触发远端 quota 请求。
- profile discovery 应有短缓存或显式 fresh 参数。
- quota 请求并发受控，避免多个账号同时打爆远端。
- local Web 首屏不依赖 OpenClaw CLI。
