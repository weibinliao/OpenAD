# OpenAD 发布说明与验证状态 / Release Notes and Verification Status

[中文](#中文) | [English](#english)

## 中文

本文只记录当前仓库和最近验证能够证明的状态，不作为营销材料。

### 当前仓库范围

- `apps/backend`：Go API、CLI、NTFS 扫描、AD 集成、权限分析、导出、历史和审计。
- `apps/web`：当前 OpenAD Next.js 产品界面和静态导出。
- `apps/desktop-win`：当前正式交付的 .NET WinForms/WebView2 Windows 桌面端。
- `apps/desktop-win.tests`：桌面运行时、品牌、启动和窗口缩放测试。
- `apps/desktop`：历史 Tauri 探索，不是当前交付物。
- `.github/workflows`：当前规范仓库中启用的 CI 和 Windows 打包定义。

### 已确认产品能力

- Windows NTFS 权限扫描、会话历史和会话对比。
- CSV、Excel、HTML 和管理摘要导出。
- AD 连接测试、树浏览、用户/组搜索、所属组和组成员。
- 用户到资源和资源到用户的有效权限分析。
- 风险发现、审计请求历史和 Windows 文件访问活动诊断。
- WebSocket 实时扫描进度。
- 独立 `/reports` 报告中心，包含用户、文件夹、所有者报告、模板、字段、范围、预览、历史会话和导出。
- OpenAD 桌面壳、启动体验，以及四条边和四个角缩放。

### 最近完整验证记录

2026-08-06 的 `v1.0.0` 发布验证：

- Web：41 个测试套件、152 个测试通过，typecheck 通过，成功导出 16 个静态页面。
- 桌面端：111 个测试通过，Release 构建、完整打包和安装程序构建通过。
- 后端：Go 全量测试通过。
- 依赖：Next.js 16.3.0、Node.js 22.23.2，npm 安全审计为 0 个漏洞。
- 运行时：正式包窗口标题为 OpenAD，`18080` 和 `43110` 正常监听，`/health` 返回
  `healthy/openad`，静态 Web 返回 HTTP 200。
- 现有 AD 配置连接测试通过；文档和发布物不记录绑定密码。
- 公开源码审计和安装包隐私审计通过。

### 发布状态

`v1.0.0` 是首个面向公司同事内部试用的 OpenAD 正式打包版本。Windows 安装程序、便携 ZIP
和 SHA-256 校验文件均来自已验证的 `main` 源码，不包含本地数据库、凭据、扫描导出或构建者
专属路径。Windows 是当前唯一验证的 NTFS 扫描平台，`KNOWN_ISSUES.md` 中的限制仍然适用。

分发前同时检查 `FEATURE_MATRIX.md`、`KNOWN_ISSUES.md`、`../SECURITY.md` 和
`../CONTRIBUTING.md`。公开社区支持渠道和漏洞响应策略作为独立事项继续完善，不阻塞本次内部
`v1.0.0` 试用。

## English

This document records only the state supported by the current repository and the latest verification evidence. It is not marketing material.

### Current Repository Scope

- `apps/backend`: Go API and CLI, NTFS scanning, AD integration, access analysis, export, history, and audit.
- `apps/web`: the current OpenAD Next.js product UI and static export.
- `apps/desktop-win`: the shipping .NET WinForms/WebView2 Windows desktop application.
- `apps/desktop-win.tests`: desktop runtime, branding, startup, and resize tests.
- `apps/desktop`: historical Tauri exploration, not the current deliverable.
- `.github/workflows`: active CI and Windows packaging definitions in the canonical repository.

### Confirmed Product Areas

- Windows NTFS permission scanning, session history, and session comparison.
- CSV, Excel, HTML, and management-summary export.
- AD connection testing, tree browsing, user/group search, memberships, and group members.
- User-to-resource and resource-to-user effective access analysis.
- Risk findings, audit request history, and Windows file-activity diagnostics.
- WebSocket live scan progress.
- Independent `/reports` Report Center with user, folder, and owner reports, templates, fields, scope, preview, historical sessions, and export.
- OpenAD desktop shell and startup experience, including resize behavior from all four edges and four corners.

### Latest Recorded Complete Verification

The `v1.0.0` release verification completed on 2026-08-06:

- Web: 41 suites and 152 tests passed, typecheck passed, and 16 static pages exported.
- Desktop: 111 tests passed, including the Release build, full package, and installer build.
- Backend: the full Go test suite passed.
- Dependencies: Next.js 16.3.0 and Node.js 22.23.2, with zero npm audit findings.
- Runtime: the packaged window title was OpenAD, ports `18080` and `43110` listened successfully,
  `/health` returned `healthy/openad`, and the packaged Web service returned HTTP 200.
- The existing AD configuration passed its connection test; binding passwords are not recorded in
  documentation or release artifacts.
- Public-source and installer privacy audits passed.

### Release Posture

`v1.0.0` is the first packaged OpenAD release for an internal colleague pilot. The Windows installer,
portable ZIP, and SHA-256 files are built from the verified `main` tree and do not contain local
databases, credentials, scan exports, or builder-specific paths. Windows is the only currently
verified NTFS scanning platform; the limitations in `KNOWN_ISSUES.md` still apply.

Before distributing the package, review `FEATURE_MATRIX.md`, `KNOWN_ISSUES.md`, `../SECURITY.md`, and
`../CONTRIBUTING.md` together. Public support-channel and vulnerability-response policy work remains a
separate follow-up from this internal v1.0 package.
