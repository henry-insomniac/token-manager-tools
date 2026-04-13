Token Manager Tools

版本：
0.1.0-preview.11

用途：
本工具用于在本机管理 Codex/OpenAI 账号池。它不需要安装 OpenClaw，也不会上传 token。

推荐启动方式：

macOS / Linux:
  ./token-manager start

Windows PowerShell:
  .\token-manager.exe start

启动后访问：
  http://localhost:1455/

如果 1455 端口被占用，可以指定端口启动：

macOS / Linux:
  ./token-manager start 18080

Windows PowerShell:
  .\token-manager.exe start 18080

然后访问：
  http://localhost:18080/

后台服务命令：
  token-manager start     后台启动服务，命令行可以关闭
  token-manager start 18080  使用指定端口后台启动
  token-manager status    查看服务是否正在后台运行
  token-manager stop      停止后台服务

更新说明：
  这版新增了桌面客户端预览分发包和应用图标。
  macOS 压缩包内提供 Token Manager Tools.app，Windows 压缩包内提供 token-manager-desktop.exe。
  桌面端登录仍然走系统浏览器，完成后会自动把结果带回客户端；回调端口也会自动避让。
  桌面客户端支持关闭后隐藏、再次打开唤回，以及开机后隐藏启动。

常用账号池命令：
  token-manager list
  token-manager create acct-a
  token-manager login acct-a
  token-manager login acct-a --manual
  token-manager probe acct-a
  token-manager activate acct-a
  token-manager remove acct-a

调试命令：
  token-manager serve

安全说明：
serve/start 默认只监听 127.0.0.1，只给本机浏览器使用。
账号池里包含本机认证资料，不要把数据目录发给不可信的人。

Windows 提示：
如果双击闪退，请打开 PowerShell，进入文件所在目录后执行：
  .\token-manager.exe start

macOS 提示：
如果系统拦截未签名程序，请在“系统设置 -> 隐私与安全性”里允许打开，或在终端中执行：
  chmod +x ./token-manager
  ./token-manager start

桌面客户端提示：
  macOS 预览包已经补了应用 bundle 签名。
  如果系统仍提示无法验证开发者，请在 Finder 中右键应用，再点一次“打开”。
