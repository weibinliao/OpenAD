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
- 系统安装的 Node.js、Go，或项目 `tools/` 下的便携工具链

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

本地应用数据为了兼容保存在 `%APPDATA%\PermissionProtector`。禁止把真实凭据或目录
导出数据提交到 Git。

## Web 界面开发

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run dev
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

开发界面地址为 `http://localhost:3010`。静态导出写入 `apps/web/out`，该目录已忽略。

## Windows 桌面端

快速检查：

```powershell
dotnet test .\apps\desktop-win.tests\PermissionProtector.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\PermissionProtector.Desktop.csproj -c Release
```

完整桌面包：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

输出目录为 `dist/OpenAD-Windows-Desktop-v<version>`。可执行文件暂时保留
`PermissionProtector.exe` 兼容名称，但用户看到的产品必须是 OpenAD。

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
- System Node.js and Go, or the portable toolchains under `tools/`

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

For compatibility, local application data remains under `%APPDATA%\PermissionProtector`. Never commit real credentials or directory exports.

### Web Development

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run dev
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

The development UI is available at `http://localhost:3010`. Static export writes to the ignored `apps/web/out` directory.

### Windows Desktop

Fast checks:

```powershell
dotnet test .\apps\desktop-win.tests\PermissionProtector.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\PermissionProtector.Desktop.csproj -c Release
```

Build the complete package with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

Output is written to `dist/OpenAD-Windows-Desktop-v<version>`. The executable temporarily keeps the compatibility name `PermissionProtector.exe`, while all user-visible branding must use OpenAD.

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
