# Token Manager Tools

Token Manager Tools 是一个跨平台账号池管理工具。

它从 OpenClaw Manager Native 的账号池能力中拆出来，目标是让 Windows、macOS、Linux 用户都能管理本机 Codex/OpenAI 账号池。用户不安装 OpenClaw 也能使用账号池；安装 OpenClaw 后，再启用兼容同步和运行时集成。

当前状态：核心实现阶段。

已具备第一批本地能力：

- 创建账号槽位
- 列出账号槽位
- Codex OAuth 登录流程
- 手动登录兜底模式
- 保存 OpenClaw/Codex 兼容认证文件
- 探测 Codex 额度，access token 失效时自动 refresh 后重试一次
- 切换默认运行槽位
- 移除槽位并归档本地资料
- 本地浏览器账号池页面

## 目标

- 管理本机账号槽位
- 登录 Codex/OpenAI OAuth
- 查看账号身份和额度状态
- 切换默认运行账号
- 移除废弃槽位并归档本地资料
- 导出和导入本机账号池
- 提供 CLI、本地 HTTP API 和本地浏览器 UI
- 兼容 OpenClaw 的 `.openclaw-*` 和 `.codex-*` 目录结构

## 非目标

- 不删除远端账号
- 不默认上传 token
- 不在第一版做云同步
- 不在第一版移植完整 OpenClaw Manager Native
- 不在第一版实现 skills、gateway、watchdog、机器监控

## 计划交付形态

```text
token-manager-tools
        |
        +-- CLI: token-manager
        |
        +-- Local API: 127.0.0.1
        |
        +-- Local Web UI: browser
        |
        +-- Shared Go Core
```

## 当前 CLI

```bash
go run ./cmd/token-manager list
go run ./cmd/token-manager create acct-a
go run ./cmd/token-manager login acct-a
go run ./cmd/token-manager login acct-a --manual
go run ./cmd/token-manager probe acct-a
go run ./cmd/token-manager activate acct-a
go run ./cmd/token-manager remove acct-a
go run ./cmd/token-manager start
go run ./cmd/token-manager status
go run ./cmd/token-manager stop
go run ./cmd/token-manager serve
```

环境变量：

```bash
OPENCLAW_HOME_DIR=/path/to/openclaw-root
OPENCLAW_CODEX_HOME_DIR=/path/to/codex-root
OPENCLAW_MANAGER_DIR=/path/to/manager-state
TOKEN_MANAGER_OAUTH_TOKEN_URL=https://auth.openai.com/oauth/token
TOKEN_MANAGER_USAGE_URL=https://chatgpt.com/backend-api/wham/usage
TOKEN_MANAGER_LOGIN_MODE=manual
TOKEN_MANAGER_SERVER_ADDR=127.0.0.1:1455
TOKEN_MANAGER_SERVER_NO_OPEN=1
TOKEN_MANAGER_ALLOW_REMOTE=1
```

## 文档

- [架构设计](./docs/ARCHITECTURE.md)
- [实施路线](./docs/ROADMAP.md)
- [测试计划](./docs/TEST_PLAN.md)

## 和 OpenClaw Manager Native 的关系

OpenClaw Manager Native 继续作为 macOS 上的完整维护工具。

Token Manager Tools 只承担账号池核心能力。后续 macOS Native 可以改为复用这个项目的核心包或本地 API，避免账号池逻辑在两个项目里重复维护。
