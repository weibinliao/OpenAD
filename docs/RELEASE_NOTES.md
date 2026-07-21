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
- `.github/workflows`：CI 和 Windows 打包定义；只有项目成为真实 Git 仓库根目录后才会被 GitHub 自动识别。

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

2026-07-14 的项目记忆记录：

- Web：99 个测试通过，typecheck 通过，成功导出 16 个静态页面。
- 桌面端：73 个测试通过，Release 构建和完整打包通过。
- 后端：Go 全量测试通过。
- 运行时：`18080` 和 `43110` 正常监听，`/health` 正常。
- 现有 AD 配置连接测试通过；文档不记录绑定密码。

这是历史验证基线，不替代每次发布前重新执行完整验证。

### 发布状态

Windows 是当前唯一验证的 NTFS 扫描平台。工作区可作为内部 OpenAD 产品基线，但在许可证、安全联系方式、Git 仓库边界和完整索引、有效 CI、全新检出构建与兼容迁移方案完成前，不满足公开开源发布条件。

发布前同时检查 `FEATURE_MATRIX.md`、`KNOWN_ISSUES.md`、`OPEN_SOURCE_RELEASE_CHECKLIST.md`、`../SECURITY.md` 和 `../CONTRIBUTING.md`。

## English

This document records only the state supported by the current repository and the latest verification evidence. It is not marketing material.

### Current Repository Scope

- `apps/backend`: Go API and CLI, NTFS scanning, AD integration, access analysis, export, history, and audit.
- `apps/web`: the current OpenAD Next.js product UI and static export.
- `apps/desktop-win`: the shipping .NET WinForms/WebView2 Windows desktop application.
- `apps/desktop-win.tests`: desktop runtime, branding, startup, and resize tests.
- `apps/desktop`: historical Tauri exploration, not the current deliverable.
- `.github/workflows`: CI and Windows packaging definitions; GitHub will detect them only after this project becomes the effective repository root.

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

The project memory records the following baseline from 2026-07-14:

- Web: 99 tests passed, typecheck passed, and 16 static pages exported.
- Desktop: 73 tests passed, with successful Release build and complete packaging.
- Backend: the full Go test suite passed.
- Runtime: ports `18080` and `43110` listened successfully and `/health` was healthy.
- The existing AD configuration passed its connection test; binding passwords are not recorded in documentation.

This is a historical verification baseline and does not replace rerunning complete verification before each release.

### Release Posture

Windows is the only currently verified NTFS scanning platform. The workspace can serve as an internal OpenAD product baseline, but it is not ready for public open-source release until license selection, a security contact, correct Git repository boundaries and complete indexing, active CI, clean-checkout builds, and compatibility migration planning are complete.

Before release, review `FEATURE_MATRIX.md`, `KNOWN_ISSUES.md`, `OPEN_SOURCE_RELEASE_CHECKLIST.md`, `../SECURITY.md`, and `../CONTRIBUTING.md` together.
