# OpenAD Windows 安装说明 / Windows Install Notes

[中文](#中文) | [English](#english)

## 中文

当前正式交付物是 .NET WinForms/WebView2 桌面应用。发布包同时包含 Go API 服务和 Next.js 静态界面；`PermissionProtector.exe` 暂时保留为兼容文件名，用户可见产品名称为 OpenAD。

### 环境要求

- Windows 10 或更高版本
- Microsoft Edge WebView2 Runtime
- 当前用户对 `%APPDATA%\PermissionProtector` 具有读写权限

### 快速启动

1. 将完整发布包解压到稳定的本地目录。
2. 双击 `PermissionProtector.exe`。
3. 等待启动界面进入 OpenAD 主界面。
4. 如需诊断，检查 `http://127.0.0.1:18080/health` 和 `http://127.0.0.1:43110/`。

不要只复制主程序。桌面启动程序、Go 服务、静态 Web、`web\`、WebView2 集成文件和 `runtimes\` 必须保持在同一发布目录结构中。完整清单见 `RELEASE_MANIFEST.md`。

### 数据位置与备份

默认应用数据目录：

```text
%APPDATA%\PermissionProtector
```

默认 SQLite 数据库位于该目录内。移动主机、升级或清理前应备份整个数据目录。发布包中的兼容脚本可使用 `backup-data.bat` 创建备份；禁止把真实数据库、凭据或扫描导出提交到 Git。

### 端口

```text
API: 18080
桌面包 Web: 43110
开发 Web: 3010
```

桌面包默认只监听本机回环地址。不要把这些端口直接暴露到公网。LAN 访问属于显式运维行为，启用前必须评估产品尚未提供登录和 RBAC 的限制。

### 启动故障检查

- 确认发布目录完整，尤其是 `permission-protector-server.exe`、`permission-protector-web.exe` 和 `web\`。
- 确认 WebView2 Runtime 已安装。
- 检查 `18080` 和 `43110` 是否被其他进程占用。
- 查看 `%APPDATA%\PermissionProtector` 下的运行日志，但分享前必须删除身份、路径和凭据信息。
- 浏览器模式脚本只用于兼容和诊断，不代表当前正式桌面体验。

## English

The shipping application is the .NET WinForms/WebView2 desktop host. The release package also contains the Go API service and the Next.js static interface. `PermissionProtector.exe` remains as a compatibility filename, while the user-visible product name is OpenAD.

### Requirements

- Windows 10 or later
- Microsoft Edge WebView2 Runtime
- Read and write access to `%APPDATA%\PermissionProtector`

### Quick Start

1. Extract the complete package to a stable local directory.
2. Double-click `PermissionProtector.exe`.
3. Wait for startup to enter the OpenAD main interface.
4. For diagnostics, check `http://127.0.0.1:18080/health` and `http://127.0.0.1:43110/`.

Do not copy only the main executable. The desktop launcher, Go services, static Web assets, `web\`, WebView2 integration files, and `runtimes\` must remain in the packaged directory structure. See `RELEASE_MANIFEST.md` for the complete list.

### Data, Backup, and Ports

Application data defaults to `%APPDATA%\PermissionProtector`. Back up the complete directory before moving hosts, upgrading, or cleaning local data. Compatibility packages may include `backup-data.bat`. Never commit real databases, credentials, or scan exports.

```text
API: 18080
Packaged Web: 43110
Development Web: 3010
```

The desktop package listens on loopback by default. Do not expose these ports directly to the public internet. LAN access is an explicit operational decision and must account for the absence of product login and RBAC.

### Startup Troubleshooting

- Confirm the package is complete, especially `permission-protector-server.exe`, `permission-protector-web.exe`, and `web\`.
- Confirm WebView2 Runtime is installed.
- Check whether another process owns ports `18080` or `43110`.
- Review runtime logs under `%APPDATA%\PermissionProtector`, removing identities, paths, and credentials before sharing them.
- Browser-mode scripts are compatibility and diagnostic tools, not the current shipping desktop experience.
