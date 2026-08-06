# OpenAD 开发指南

[中文](#openad-开发指南) | [English](#openad-development-guide)

本文件是 OpenAD 项目构建、测试和打包的统一入口。使用代理开发时，先阅读
`AGENTS.md`；维护者本地工作区存在 `.codex/memory.md` 时再一并阅读。

## 工作目录与回退

- 规范物理目录和公开发布根目录是 `<repository-root>`。
- 旧工作区如因当前任务占用而暂时存在，只作为本地回退副本，不得用于公开发布。
- 2026-07-15 用户体验升级前快照位于
  `backups/OpenAD-pre-upgrade-20260715-ux-baseline`，恢复前必须先核对其中的
  `BACKUP_MANIFEST.json` 和 `SHA256SUMS.csv`。
- 当前公开 Git 根目录是 `<repository-root>`。恢复快照或调整目录前，必须先关闭桌面应用、
  开发服务器和占用目录的任务，并且不能覆盖其他项目或用户改动。

## 技术栈

| 模块 | 技术 | 路径 |
| --- | --- | --- |
| API 与扫描器 | Go 1.23、Gin、GORM、SQLite | `apps/backend` |
| 产品界面 | Next.js 14、React 18、TypeScript | `apps/web` |
| Windows 桌面端 | .NET 10 WinForms、WebView2 | `apps/desktop-win` |
| 桌面端测试 | xUnit | `apps/desktop-win.tests` |

正式桌面端会启动 `18080` 端口的 API 和 `43110` 端口的静态 Web 服务；Next 开发
服务器使用 `3010`。

## 开发环境要求

- Windows 10 或更高版本
- PowerShell 5.1 或更高版本
- .NET 10 SDK
- WebView2 Runtime
- Node.js 22 LTS、Go，或项目 `tools/` 下的便携工具链

项目提供便携工具链准备脚本：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-portable-go.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-portable-node.ps1
```

缺少 `apps/web/node_modules` 时安装前端依赖：

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web ci
```

## 后端开发

```powershell
$env:GOTELEMETRY = 'off'
$env:GOCACHE = Join-Path $PWD '.gocache'
$env:GOMODCACHE = Join-Path $PWD '.gomodcache'

& .\tools\go\bin\go.exe -C .\apps\backend test ./...
& .\tools\go\bin\go.exe -C .\apps\backend run ./cmd/api
```

API 默认地址为 `http://127.0.0.1:18080`。健康检查：

```powershell
Invoke-RestMethod http://127.0.0.1:18080/health
```

扫描服务默认最多同时运行 1 个扫描，以避免 NTFS 磁盘 IO 和 SQLite 写锁互相放大。可用
`PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS` 设置大于等于 1 的整数；旧部署可继续使用兼容变量
`FSA_MAX_CONCURRENT_SCANS`，主变量存在时优先。达到上限的扫描请求会立即返回 HTTP 409，不会在
后台无限排队。调整该值会增加磁盘和数据库争用，需在重启 API 后生效并结合实际目录负载验证。

本地应用数据保存在 `%APPDATA%\OpenAD`；首次升级会迁移旧 `%APPDATA%\PermissionProtector`
目录。禁止把真实凭据或目录导出数据提交到 Git。

## Web 界面开发

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run dev
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

开发界面地址为 `http://127.0.0.1:3010`。API、Next.js 开发服务器和兼容静态 Web 服务默认都只监听 `127.0.0.1`；空 `ALLOW_ORIGINS` 只允许本机 `3010` 与 `43110` Web 来源。静态导出写入 `apps/web/out`，该目录已忽略。

需要在可信管理网络上显式启用浏览器模式时，以管理员身份运行 `scripts\enable-lan-access.bat`，把输出命令中的 `your-host-ip` 替换为实际 LAN 地址后执行。该命令只对本次启动设置全网卡绑定与精确 CORS/WebSocket Origin；普通启动不会持久化 LAN 暴露。OpenAD 当前没有产品级登录或 RBAC，禁止用于公网或不可信网络。

## Windows 桌面端

快速检查：

```powershell
dotnet test .\apps\desktop-win.tests\OpenAD.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\OpenAD.Desktop.csproj -c Release
```

完整桌面包：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

输出目录为 `dist/OpenAD-Windows-Desktop-v<version>`。安装包和包内可执行文件统一使用
OpenAD 产品名；旧数据路径仅作为升级兼容入口保留。

构建供最终用户安装的 `win-x64` 安装程序：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-windows-installer.ps1 -Version 1.0.0
```

输出为 `dist/OpenAD.exe` 及 `dist/OpenAD.exe.sha256`。版本号保存在安装器元数据中，构建会发布
自包含 .NET 运行时的桌面包、校验并准备 Inno Setup、审计解压目录和最终安装器中的数据库、
日志、环境文件、密钥、本机路径、邮箱、环境域名及内网地址，然后才生成校验和。安装程序按
当前用户安装到 `%LOCALAPPDATA%\Programs\OpenAD`，不需要管理员权限；它不打包也不删除
`%APPDATA%\OpenAD` 或兼容数据目录，因此新电脑首次启动为空白，升级和卸载则保留已有数据。
目标电脑仍需安装 Microsoft Edge WebView2 Runtime。当前安装程序未做代码签名，Windows
SmartScreen 可能显示“未知发布者”。

## 验证等级

### 局部修改

- 运行距离修改代码最近的单元测试或组件测试。
- 运行对应模块的编译或 typecheck。
- 检查受影响页面或桌面状态。

### 前端交付

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

### 桌面版交付

运行 `scripts/build-desktop-windows.ps1`，启动桌面包，并验证：

- 启动过程能进入 OpenAD 主界面。
- `18080` 和 `43110` 正常监听。
- `/health` 返回数据库健康状态。
- 现有 AD 配置仍能通过连接测试。
- 窗口可以从四条边和四个角缩放。

## 工作区清理

先预览普通清理：

```powershell
.\scripts\clean-workspace.ps1 -WhatIf
```

删除可重建输出和缓存，同时保留依赖、桌面包、备份、设计参考和本地数据库：

```powershell
.\scripts\clean-workspace.ps1
```

扩展清理必须显式传入开关，使用前阅读 `docs/PROJECT_HYGIENE.md`。

## Git 注意事项

OpenAD 的公开 Git 根目录是 `<repository-root>`。禁止执行大范围 Git 清理或回退；首次
提交前必须核对源码边界，确保不包含本地代理状态、缓存、备份、凭据或其他项目文件。

公开提交前还应运行 `.\scripts\audit-public-source.ps1`，确认示例不含私有地址、本地用户路径或
环境专属目录身份。

## OpenAD Development Guide

This file is the canonical entry point for building, testing, packaging, and cleaning OpenAD. Agent-based development must begin by reading `AGENTS.md` and, when it exists in a maintainer workspace, `.codex/memory.md`.

### Workspace And Rollback

- The canonical physical directory and public publication root is `<repository-root>`.
- A legacy workspace that remains temporarily because it is held by an active task is a local recovery copy only and must not be published.
- The verified pre-upgrade snapshot is `backups/OpenAD-pre-upgrade-20260715-ux-baseline`; inspect `BACKUP_MANIFEST.json` and `SHA256SUMS.csv` before any restore.
- The public Git root is `<repository-root>`. Close desktop processes, development servers, and workspace handles before a restore or physical rename, and never overwrite unrelated projects or user changes.

### Technology Stack

| Area | Technology | Path |
| --- | --- | --- |
| API and scanner | Go 1.23, Gin, GORM, SQLite | `apps/backend` |
| Product UI | Next.js 14, React 18, TypeScript | `apps/web` |
| Windows desktop | .NET 10 WinForms, WebView2 | `apps/desktop-win` |
| Desktop tests | xUnit | `apps/desktop-win.tests` |

The shipping desktop app starts the API on port `18080` and packaged Web content on port `43110`. The Next.js development server uses port `3010`.

### Prerequisites

- Windows 10 or later
- PowerShell 5.1 or later
- .NET 10 SDK
- WebView2 Runtime
- Node.js 22 LTS and Go, or the portable toolchains under `tools/`

Prepare the portable toolchains with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-portable-go.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-portable-node.ps1
```

Install Web dependencies when `apps/web/node_modules` is missing:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web ci
```

### Backend Development

```powershell
$env:GOTELEMETRY = 'off'
$env:GOCACHE = Join-Path $PWD '.gocache'
$env:GOMODCACHE = Join-Path $PWD '.gomodcache'

& .\tools\go\bin\go.exe -C .\apps\backend test ./...
& .\tools\go\bin\go.exe -C .\apps\backend run ./cmd/api
```

The API defaults to `http://127.0.0.1:18080`. Health check:

```powershell
Invoke-RestMethod http://127.0.0.1:18080/health
```

The scan service runs at most one scan concurrently by default to avoid compounding NTFS disk I/O and
SQLite write-lock contention. Set `PERMISSION_PROTECTOR_MAX_CONCURRENT_SCANS` to an integer of 1 or
greater; existing deployments may keep using the compatibility variable `FSA_MAX_CONCURRENT_SCANS`,
while the primary variable takes precedence when both are present. Requests above the limit return
HTTP 409 immediately instead of waiting in an unbounded queue. Higher values increase disk and database
contention and take effect after the API restarts, so validate them against representative directories.

Local application data is stored under `%APPDATA%\OpenAD`; first launch migrates an existing `%APPDATA%\PermissionProtector` directory. Never commit real credentials or directory exports.

### Web Development

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run dev
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

The development UI is available at `http://127.0.0.1:3010`. The API, Next.js development server, and compatibility static Web server all bind only to `127.0.0.1` by default. With an empty `ALLOW_ORIGINS`, only local Web origins on ports `3010` and `43110` are allowed. Static export writes to the ignored `apps/web/out` directory.

To enable browser mode explicitly on a trusted administration network, run `scripts\enable-lan-access.bat` as Administrator, replace `your-host-ip` in the printed command with the actual LAN address, and execute it. The command enables all-interface binding and exact CORS/WebSocket Origin only for that launch; normal launches do not persist LAN exposure. OpenAD has no product-level login or RBAC and must not be used on the public internet or an untrusted network.

### Windows Desktop

Fast checks:

```powershell
dotnet test .\apps\desktop-win.tests\OpenAD.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\OpenAD.Desktop.csproj -c Release
```

Build the complete package with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

Output is written to `dist/OpenAD-Windows-Desktop-v<version>`. The installer and bundled executables use OpenAD filenames; legacy data paths remain only for migration compatibility.

Build the installable `win-x64` setup program with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-windows-installer.ps1 -Version 1.0.0
```

This creates `dist/OpenAD.exe` and `dist/OpenAD.exe.sha256`. The version is retained in the installer
metadata. The build
publishes a self-contained .NET desktop payload, prepares a checksum-verified Inno Setup toolchain,
and audits both the unpacked payload and final installer for databases, logs, environment files, keys,
local paths, email addresses, environment identities, and private IP addresses before writing the
checksum. Setup installs per user under `%LOCALAPPDATA%\Programs\OpenAD` without elevation. It neither
packages nor deletes `%APPDATA%\OpenAD` or the compatibility data directory, so a first install on a
new machine starts blank while upgrades and uninstall preserve existing data. Microsoft Edge WebView2
Runtime remains required on the target computer. The installer is currently unsigned, so Windows
SmartScreen may show an unknown-publisher warning.

### Verification Levels

For a local change, run the nearest unit or component tests, the corresponding compiler or typecheck, and inspect the affected UI or desktop state.

For a Web delivery, run:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

For a desktop delivery, build with `scripts/build-desktop-windows.ps1`, launch the package, and verify startup into OpenAD, listeners on `18080` and `43110`, `/health`, the existing AD connection, and resize behavior from all four edges and four corners.

### Workspace Cleanup

Preview normal cleanup:

```powershell
.\scripts\clean-workspace.ps1 -WhatIf
```

Remove rebuildable output and caches while preserving dependencies, desktop packages, backups, design references, and local databases:

```powershell
.\scripts\clean-workspace.ps1
```

Extended cleanup requires an explicit switch. Read `docs/PROJECT_HYGIENE.md` first.

### Git Safety

The OpenAD public Git root is `<repository-root>`. Do not perform broad Git cleanup or rollback. Verify
the source boundary before the first public commit so local agent state, caches, backups, credentials,
and unrelated projects are excluded.

Before a public commit, also run `.\scripts\audit-public-source.ps1` to confirm examples contain no
private addresses, local user paths, or environment-specific directory identities.
