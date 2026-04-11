# token-manager-tools

This project is the cross-platform account/token pool tool extracted from OpenClaw Manager Native.

Project rules:

- Default audience is non-technical or semi-technical users who need account pool management without living in Terminal.
- Keep OpenClaw optional. Account pool management must work even when OpenClaw is not installed.
- Build core logic first, UI shells second.
- Prefer one portable Go core over platform-specific duplicated logic.
- Keep token handling explicit and conservative. Never add cloud sync or remote token upload by default.
- Use Chinese-first UX copy for user-facing docs and UI.
- Use short, action-oriented wording. Avoid marketing tone.
- For destructive actions, use "移除本机槽位" or "归档本地资料", not "删除账号".
- Preserve compatibility with existing `.openclaw-*` and `.codex-*` folder conventions unless there is a strong reason to change them.

Initial scope:

- CLI
- Local HTTP API
- Browser-based local UI
- Cross-platform path and browser adapters
- Codex/OpenAI OAuth account pool management

Out of initial scope:

- Full OpenClaw diagnostics
- skills marketplace
- gateway/watchdog repair
- cloud sync
- team account sharing
- native Windows/macOS/Linux desktop shells
