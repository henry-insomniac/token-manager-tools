Token Manager Tools

版本：
0.1.0-preview.5

用途：
本工具用于在本机管理 Codex/OpenAI 账号池。它不需要安装 OpenClaw，也不会上传 token。

推荐启动方式：

macOS / Linux:
  ./token-manager start

Windows PowerShell:
  .\token-manager.exe start

启动后访问：
  http://127.0.0.1:1455/

如果 1455 端口被占用，可以指定端口启动：

macOS / Linux:
  ./token-manager start 18080

Windows PowerShell:
  .\token-manager.exe start 18080

然后访问：
  http://127.0.0.1:18080/

后台服务命令：
  token-manager start     后台启动服务，命令行可以关闭
  token-manager start 18080  使用指定端口后台启动
  token-manager status    查看服务是否正在后台运行
  token-manager stop      停止后台服务

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
