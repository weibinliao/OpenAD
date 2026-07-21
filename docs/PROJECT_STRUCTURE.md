# 项目结构

[中文](#项目结构) | [English](#project-structure)

本项目围绕 Windows 优先的 OpenAD 桌面产品组织。PermissionProtector 只保留在部分
兼容名称中。

## 源码目录

| 路径 | 职责 |
| --- | --- |
| `apps/backend` | Go API、CLI、NTFS 扫描器、AD、数据库、导出和内置静态 Web 服务 |
| `apps/web` | 发布包使用的 Next.js 产品界面 |
| `apps/desktop-win` | 当前交付的 .NET WinForms/WebView2 Windows 桌面壳 |
| `apps/desktop-win.tests` | 桌面运行时、启动、品牌和窗口缩放测试 |
| `apps/desktop` | 历史 Tauri 探索，不属于当前 Windows 发布包 |
| `scripts` | Windows 构建、启动、备份、验证和网络辅助脚本 |
| `docs` | 产品、发布、部署和治理文档 |
| `windows`、`linux` | 历史或平台辅助资源；当前只有 Windows 扫描运行时已验证 |

## 生成目录和本地目录

| 路径 | 说明 |
| --- | --- |
| `dist` | 桌面发布包；使用 `scripts\build-desktop-windows.ps1` 重建 |
| `.gocache`、`.gomodcache`、`.gotelemetry` | 本地 Go 缓存 |
| `tools` | 安装脚本准备的便携 Go/Node 工具链 |
| `apps/web/.next-dev`、`apps/web/.next-static`、`apps/web/out` | Next.js 输出 |
| `apps/desktop-win/bin`、`apps/desktop-win/obj` | .NET 构建输出 |
| `.local` | 本地运行和测试数据 |
| `backups` | 已忽略的恢复备份，不是源码事实来源 |

## 当前事实来源

判断产品当前能力时优先读取：

- `README.md`
- `AGENTS.md`
- `.codex\memory.md`（维护者本地工作区存在时）
- `DEVELOPMENT.md`
- `docs\FEATURE_MATRIX.md`
- `docs\KNOWN_ISSUES.md`
- `docs\ENGINEERING_STANDARDS.md`
- `docs\PROJECT_HYGIENE.md`
- `docs\RELEASE_MANIFEST.md`

历史文档可能包含过时描述。只有与当前源码和验证结果一致时，才能作为对外发布依据。

## Project Structure

This project is organized around the Windows-first OpenAD desktop product. PermissionProtector remains only in selected compatibility names.

### Source Directories

| Path | Responsibility |
| --- | --- |
| `apps/backend` | Go API and CLI, NTFS scanner, AD integration, database, export, and embedded static Web server |
| `apps/web` | Next.js product interface used by release packages |
| `apps/desktop-win` | Shipping .NET WinForms/WebView2 Windows desktop host |
| `apps/desktop-win.tests` | Desktop runtime, startup, branding, and resize tests |
| `apps/desktop` | Historical Tauri exploration, not part of the current Windows release |
| `scripts` | Windows build, startup, backup, verification, and network helper scripts |
| `docs` | Product, release, deployment, and governance documentation |
| `windows`, `linux` | Historical or platform support resources; only the Windows scan runtime is currently verified |

### Generated and Local Directories

| Path | Purpose |
| --- | --- |
| `dist` | Desktop release packages, rebuilt with `scripts\build-desktop-windows.ps1` |
| `.gocache`, `.gomodcache`, `.gotelemetry` | Local Go caches |
| `tools` | Portable Go and Node toolchains prepared by setup scripts |
| `apps/web/.next-dev`, `apps/web/.next-static`, `apps/web/out` | Next.js output |
| `apps/desktop-win/bin`, `apps/desktop-win/obj` | .NET build output |
| `.local` | Local runtime and test data |
| `backups` | Ignored recovery backups, not a source of truth |

### Current Sources of Truth

Use `README.md`, `AGENTS.md`, `DEVELOPMENT.md`, and the current engineering, feature, known-issues, hygiene, and release-manifest documents under `docs/` when determining current behavior. Maintainers may also consult `.codex/memory.md` when it exists locally.

Historical documents may contain outdated descriptions. They are release evidence only when consistent with current source and verification results.
