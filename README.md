# OpenAD

[中文](#openad) | [English](#english)

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)

OpenAD 是一款面向 Windows 的 IT 运维桌面应用，用于统一查看 Active Directory 身份
与 NTFS/网络共享权限。它帮助运维人员回答：哪个用户或组可以访问某个资源、权限来自
直接授权还是嵌套组关系，以及具体原因是什么。

当前版本支持简体中文和英文，对 Active Directory 保持只读，并以 .NET WebView2
桌面应用形式交付，后端使用 Go API，产品界面使用 Next.js。`PermissionProtector`
暂时保留在部分二进制文件、命名空间和本地数据路径中，仅用于兼容。

## 功能模块

| 模块 | 路由 | 职责 |
| --- | --- | --- |
| 总览 | `/` | 运行状态、AD 状态、扫描/风险摘要、最近活动和主要操作入口 |
| 目录浏览 | `/identity` | AD 树、用户/组自动补全、对象详情、所属组、组成员和目录快照同步 |
| 扫描中心 | `/scan-workspace` | 目录选择、扫描参数、AD 状态、运行控制、实时进度和完成后跳转 |
| 扫描历史 | `/history` | 历史扫描会话和对比入口 |
| 权限分析 | `/access` | 用户到资源、资源到用户的有效权限分析及组关系路径 |
| 风险 | `/findings` | 权限暴露风险的优先级检查 |
| 审计 | `/audit`、`/file-activity` | API 请求记录和 Windows 文件访问事件 |
| 报告中心 | `/reports` | 用户、文件夹、所有者报告，以及模板、范围、字段、预览和导出 |
| 系统设置 | `/settings` | AD 连接、工作区身份、外观、语言和本地运行时配置 |

## 权限分析原理

1. 在系统设置中保存 AD 连接。密码会加密保存，API 不会返回原始密码。
2. 在目录浏览中执行目录同步，将用户、组和展开后的嵌套成员关系写入 SQLite 快照。
3. 扫描本地目录或共享目录，将 NTFS ACL 证据按扫描会话保存。
4. 按用户或资源查询有效权限，系统关联目录快照与扫描结果，并保留直接授权或组关系
   形成的解释路径。

## 项目目录

```text
apps/backend           Go API、扫描器、AD、权限分析、导出和审计
apps/web               Next.js 产品界面和静态导出
apps/desktop-win       当前交付的 .NET WebView2 Windows 桌面壳
apps/desktop-win.tests 桌面运行时、品牌、启动和缩放测试
apps/desktop           历史 Tauri 探索，不是当前桌面交付物
ee                     商业功能保留目录，适用独立商业许可
docs                   产品、工程、发行和运维文档
scripts                构建、启动、验证和清理脚本
tools                  已忽略的便携 Go/Node 工具链
```

后端主要包：

- `internal/ad`：LDAP 客户端、嵌套组解析、OU 树和分页
- `internal/adsync`：目录快照和成员关系持久化
- `internal/access`：用户/资源交叉权限分析
- `internal/connections`、`internal/secrets`：连接配置和加密
- `internal/scanner`、`internal/scanservice`：NTFS 扫描和进度

## 开发入口

修改项目前按顺序阅读：

1. `AGENTS.md`：项目级代理约束和模块边界
2. `.codex/memory.md`：维护者本地工作区存在该文件时，再读取长期有效状态和未解决风险
3. `DEVELOPMENT.md`：环境、测试、打包和清理命令

快速启动后端：

```powershell
$env:GOCACHE = Join-Path $PWD '.gocache'
$env:GOMODCACHE = Join-Path $PWD '.gomodcache'
$env:PERMISSION_PROTECTOR_DATA_DIR = "$env:APPDATA\PermissionProtector"
& .\tools\go\bin\go.exe -C .\apps\backend run ./cmd/api
```

快速启动 Web：

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run dev
```

开发界面使用 `http://localhost:3010`。桌面包使用 API 端口 `18080` 和 Web 端口
`43110`。

## 验证

- 后端：在 `apps/backend` 下运行 `go test ./...`
- Web：运行 `npm run typecheck`、`npm test`、`npm run build:static`
- 桌面端：运行 `apps/desktop-win*` 下的 xUnit 测试和 Release 构建
- 完整桌面包：运行 `scripts/build-desktop-windows.ps1`

## 安全边界

- 当前版本不提供 AD 写操作。
- 连接密码使用本地生成的密钥加密，但本机管理员仍可能同时访问密文和密钥，因此必须
  限制主机访问权限。
- 网络访问受本地准入配置和接口限流约束。当前没有产品登录功能，只能部署在可信管理
  网络中。
- 报告、审计日志、路径、身份和扫描数据都可能包含敏感信息。

安全问题报告方式见 `SECURITY.md`；对外开源发布阻塞项见
`docs/OPEN_SOURCE_RELEASE_CHECKLIST.md`。

## 许可证状态

OpenAD 采用 open core 双许可结构。除 `ee/` 和另有自身许可声明的第三方材料外，仓库主体
按 [GNU AGPL-3.0](LICENSE) 提供；`ee/` 被明确排除在该授权之外，并适用
[商业许可条款](ee/LICENSE)。许可边界、AGPL 实际义务和商业授权场景见
[许可说明](LICENSING.md)。

## English

OpenAD is a Windows-first IT operations desktop application for reviewing Active Directory identities together with NTFS and network-share permissions. It helps operators determine which users or groups can access a resource, whether that access is direct or inherited through nested groups, and why the permission is effective.

The current release supports Simplified Chinese and English, keeps Active Directory operations read-only, and ships as a .NET WebView2 desktop application backed by a Go API and a Next.js interface. `PermissionProtector` remains in selected binary names, namespaces, and local data paths only for compatibility.

### Product Modules

| Module | Route | Responsibility |
| --- | --- | --- |
| Overview | `/` | Runtime and AD status, scan and risk summary, recent activity, and primary actions |
| Directory Explorer | `/identity` | AD tree, user/group autocomplete, object details, memberships, group members, and directory snapshot sync |
| Scan Center | `/scan-workspace` | Directory selection, scan parameters, AD status, runtime controls, live progress, and completion navigation |
| Scan History | `/history` | Historical scan sessions and comparison entry points |
| Access Analysis | `/access` | User-to-resource and resource-to-user effective access analysis with group relationship paths |
| Findings | `/findings` | Prioritized permission exposure checks |
| Audit | `/audit`, `/file-activity` | API request history and Windows file-access events |
| Report Center | `/reports` | User, folder, and owner reports with templates, scope, fields, preview, and export |
| System Settings | `/settings` | AD connections, workspace identity, appearance, language, and local runtime configuration |

### How Access Analysis Works

1. Save an AD connection in System Settings. Passwords are encrypted at rest and are never returned by the API.
2. Synchronize the directory in Directory Explorer to store users, groups, and expanded nested membership in a SQLite snapshot.
3. Scan a local or shared directory to store NTFS ACL evidence under a scan session.
4. Query effective access by identity or resource. OpenAD combines the directory snapshot and scan evidence while preserving the direct grant or group-membership explanation path.

### Repository Layout

```text
apps/backend           Go API, scanner, AD, access analysis, export, and audit
apps/web               Next.js product interface and static export
apps/desktop-win       Shipping .NET WebView2 Windows desktop host
apps/desktop-win.tests Desktop runtime, branding, startup, and resize tests
apps/desktop           Historical Tauri exploration, not the shipping desktop app
ee                     Reserved commercial-feature directory under a separate commercial license
docs                   Product, engineering, release, and operations documentation
scripts                Build, startup, verification, and cleanup scripts
tools                  Ignored portable Go and Node toolchains
```

Key backend packages include `internal/ad` for LDAP and group resolution, `internal/adsync` for directory snapshots, `internal/access` for effective access analysis, `internal/connections` and `internal/secrets` for encrypted connections, and `internal/scanner` plus `internal/scanservice` for NTFS scanning and progress.

### Development

Read these files before changing the project:

1. `AGENTS.md` for project-specific agent constraints and module boundaries.
2. `.codex/memory.md` for durable project state and unresolved risks, when it exists in a maintainer's local workspace.
3. `DEVELOPMENT.md` for environment, testing, packaging, and cleanup commands.

Quick backend start:

```powershell
$env:GOCACHE = Join-Path $PWD '.gocache'
$env:GOMODCACHE = Join-Path $PWD '.gomodcache'
$env:PERMISSION_PROTECTOR_DATA_DIR = "$env:APPDATA\PermissionProtector"
& .\tools\go\bin\go.exe -C .\apps\backend run ./cmd/api
```

Quick Web start:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web run dev
```

The development UI runs at `http://localhost:3010`. The packaged desktop app uses API port `18080` and Web port `43110`.

### Verification

- Backend: run `go test ./...` from `apps/backend`.
- Web: run `npm run typecheck`, `npm test`, and `npm run build:static`.
- Desktop: run the xUnit tests and Release build under `apps/desktop-win*`.
- Full package: run `scripts/build-desktop-windows.ps1`.

### Security Boundary

- The current release does not provide AD write operations.
- Connection passwords are encrypted with a locally generated key, but a local administrator may be able to access both ciphertext and key material. Restrict host access accordingly.
- Network access is constrained by local admission settings and endpoint rate limits. Product authentication is not yet implemented, so deploy only on a trusted administration network.
- Reports, audit logs, paths, identities, and scan data may contain sensitive information.

See `SECURITY.md` for vulnerability reporting guidance and `docs/OPEN_SOURCE_RELEASE_CHECKLIST.md` for public-release blockers.

### License Status

OpenAD uses an open-core dual-license structure. Except for `ee/` and third-party materials carrying
their own notices, the repository is available under the [GNU AGPL-3.0](LICENSE). The `ee/` directory
is expressly excluded from that grant and is governed by the [commercial license terms](ee/LICENSE).
See the [licensing guide](LICENSING.md) for the license boundary, practical AGPL obligations, and
commercial authorization scenarios.
