# 实施路线

## Phase 0: 文档和边界

目标：

- 锁定项目边界
- 明确账号池核心和 OpenClaw Manager Native 的关系
- 明确跨平台路径和安全策略

交付：

- `README.md`
- `docs/ARCHITECTURE.md`
- `docs/TEST_PLAN.md`

## Phase 1: Go Core

目标：

- 建立可测试的账号池核心包
- 不做 UI
- 不依赖 OpenClaw CLI

核心能力：

- profile discovery
- profile scaffold
- auth store read/write
- quota cache
- remove/archive
- default mirror sync

验收：

- macOS/Linux/Windows 路径单测
- archive 不被 rediscovery 扫回
- default mirror 同步不覆盖未知字段

当前进度：

- 已完成 profile discovery / scaffold / archive。
- 已完成 default auth pool 基础同步。
- 已完成 Codex OAuth token exchange 和 token 持久化。
- 已完成 usage 探测解析。
- 已补核心单测。

## Phase 2: CLI

目标：

- 用 CLI 验证核心能力跨平台可用

命令：

```text
list
create
login
probe
activate
remove
export
import
serve
doctor
```

验收：

- 无 OpenClaw 时可以创建、登录、探测、移除
- 有 OpenClaw 时可以同步默认位
- 登录失败时给出可操作提示

当前进度：

- `list/create/login/probe/activate/remove` 已接入。
- `login` 已支持本地 callback server、`--manual` 手动模式，以及 callback 端口不可用时自动降级手动模式。
- `probe` 已支持 access token 401/403 后 refresh 并重试一次。
- `serve` 已接入本地 HTTP API 和嵌入式浏览器账号池页面。

## Phase 3: Local API

目标：

- 提供本地 HTTP API，供 Web UI 和未来桌面壳复用

接口：

- profile CRUD actions
- login flow polling
- settings
- import/export

验收：

- API 不返回完整 token
- 错误信息中文清晰
- 支持 manual login fallback

当前进度：

- 已提供 profile list/create/probe/activate/remove/login-start。
- 已提供 `/auth/callback` 完成浏览器 OAuth 登录。
- OAuth verifier/state 留在本地服务内存，不返回给浏览器 API。

## Phase 4: Local Web UI

目标：

- 非技术用户可以通过浏览器管理账号池

页面：

- 账号池
- 登录流程
- 导入导出
- 设置

验收：

- Windows/macOS/Linux 浏览器可打开
- 没有 OpenClaw 时不显示 OpenClaw 专属功能
- 移除槽位有二次确认

当前进度：

- 已提供首版嵌入式账号池页，覆盖创建、登录入口、检查、切换、移除和详情查看。
- Web UI 登录已补齐手动兜底：支持重开登录页、复制链接，以及粘贴最终回调地址或 code 完成本机写入。
- 已统一本地浏览器入口和 OAuth 回调的 loopback host，减少 `localhost/127.0.0.1` 不一致造成的回调空白页。

## Phase 5: Native 复用

目标：

- OpenClaw Manager Native 复用这个核心或 API
- 避免 macOS app 和跨平台工具维护两套账号池逻辑

方式：

- 短期：native daemon 迁移到相同 core package
- 中期：native app 启动 token-manager daemon
- 长期：账号池项目独立发布，native 作为集成方

当前进度：

- 已接入桌面客户端入口、共享服务层和前端 transport 适配层
- 已提供桌面客户端预览分发包和应用图标
- macOS / Windows release 已开始附带桌面入口，Linux 继续保持 CLI / 本地 Web 主路径

## Release 建议

```text
0.1.0: core + CLI preview
0.2.0: local API + serve
0.3.0: local Web UI
0.4.0: import/export + Windows polish
1.0.0: stable cross-platform account pool
```
