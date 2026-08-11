# OpenAD Windows 安装说明 / Windows Install Notes

[中文](#中文) | [English](#english)

## 中文

当前正式交付物是 .NET WinForms/WebView2 桌面应用。发布包同时包含 Go API 服务和 Next.js 静态界面，安装包和包内可执行文件统一使用 OpenAD 名称。

### 环境要求

- Windows 10 或更高版本
- Microsoft Edge WebView2 Runtime
- 当前用户对 `%APPDATA%\OpenAD` 具有读写权限

### 快速启动

1. 将完整发布包解压到稳定的本地目录。
2. 双击 `OpenAD.exe`。
3. 等待启动界面进入 OpenAD 主界面。
4. 如需诊断，检查 `http://127.0.0.1:18080/health` 和 `http://127.0.0.1:43110/`。

同一 Windows 会话中重复双击程序不会启动第二套本地服务；OpenAD 会还原并激活已经运行的窗口。

不要只复制主程序。桌面启动程序、Go 服务、静态 Web、`web\`、WebView2 集成文件和 `runtimes\` 必须保持在同一发布目录结构中。完整清单见 `RELEASE_MANIFEST.md`。

### 数据位置与备份

默认应用数据目录：

```text
%APPDATA%\OpenAD
```

默认 SQLite 数据库位于该目录内。移动主机、升级或清理前应备份整个数据目录。发布包中的兼容脚本可使用 `backup-data.bat` 创建备份；禁止把真实数据库、凭据或扫描导出提交到 Git。

### 端口

```text
API: 18080
桌面包 Web: 43110
开发 Web: 3010
```

桌面包强制只监听本机回环地址。正式桌面包不支持 LAN 浏览器访问，也不会接受
`0.0.0.0`、`*` 或 `+` 绑定参数。若旧版本曾启用局域网模式，请以管理员身份运行：

```bat
scripts\disable-lan-access.bat
```

### 启动故障检查

- 确认发布目录完整，尤其是 `OpenAD.Server.exe`、`OpenAD.Web.exe` 和 `web\`。
- 确认 WebView2 Runtime 已安装。
- 检查 `18080` 和 `43110` 是否被其他第三方进程占用；重复启动 OpenAD 本身会切换到已有窗口，不会显示端口占用错误。
- 查看 `%APPDATA%\OpenAD` 下的运行日志，但分享前必须删除身份、路径和凭据信息。
- 浏览器模式脚本只用于兼容和诊断，不代表当前正式桌面体验。

## English

The shipping application is the .NET WinForms/WebView2 desktop host. The release package also contains the Go API service and the Next.js static interface. The installer and bundled executables use OpenAD filenames.

### Requirements

- Windows 10 or later
- Microsoft Edge WebView2 Runtime
- Read and write access to `%APPDATA%\OpenAD`

### Quick Start

1. Extract the complete package to a stable local directory.
2. Double-click `OpenAD.exe`.
3. Wait for startup to enter the OpenAD main interface.
4. For diagnostics, check `http://127.0.0.1:18080/health` and `http://127.0.0.1:43110/`.

Launching the program again in the same Windows session does not start another set of local services. OpenAD restores and activates the existing window.

Do not copy only the main executable. The desktop launcher, Go services, static Web assets, `web\`, WebView2 integration files, and `runtimes\` must remain in the packaged directory structure. See `RELEASE_MANIFEST.md` for the complete list.

### Data, Backup, and Ports

Application data defaults to `%APPDATA%\OpenAD`; first launch migrates an existing `%APPDATA%\PermissionProtector` directory. Back up the complete directory before moving hosts, upgrading, or cleaning local data. Never commit real databases, credentials, or scan exports.

```text
API: 18080
Packaged Web: 43110
Development Web: 3010
```

The desktop package is forced to loopback-only binding. LAN browser access is not supported by the
shipping desktop package, and its launchers reject `0.0.0.0`, `*`, and `+` host values. If an older
release enabled LAN mode, run the following as Administrator:

```bat
scripts\disable-lan-access.bat
```

### Startup Troubleshooting

- Confirm the package is complete, especially `OpenAD.Server.exe`, `OpenAD.Web.exe`, and `web\`.
- Confirm WebView2 Runtime is installed.
- Check whether an unrelated process owns ports `18080` or `43110`. Launching OpenAD again activates the existing window instead of reporting its own ports as busy.
- Review runtime logs under `%APPDATA%\OpenAD`, removing identities, paths, and credentials before sharing them.
- Browser-mode scripts are compatibility and diagnostic tools, not the current shipping desktop experience.
