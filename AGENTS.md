# OpenAD 项目开发约束

本文件适用于 `<repository-root>` 整个子目录，仅为 OpenAD 项目级规则，不得复制到 Codex
或其他代理的全局配置中。

## 开始工作前必须阅读

每次修改代码前按以下顺序读取：

1. `AGENTS.md`
2. `.codex/memory.md`（仅在本地维护工作区存在时）
3. `DEVELOPMENT.md`
4. 与本次需求最接近的实现文件和测试文件

必须读取磁盘上的最新文件和最新 Git 状态。该工作区会被多个任务共同使用，不能只依赖
较早任务留下的上下文摘要。

公开克隆不包含 `.codex`；缺失本地记忆文件时，以 `AGENTS.md`、`DEVELOPMENT.md`、当前
源码和测试为准，不得把缺失视为错误。

## 仓库安全规则

- OpenAD 的规范发布 Git 根目录是 `<repository-root>`。旧工作区如仍受父目录 Git 管理，
  只能作为本地回退副本，不能从中推送或生成公开发布物。
- 首次公开基线必须包含正式源码、测试、文档、资源、脚本和配置；除非能证明是生成物，
  否则都视为用户已有工作。
- 禁止执行大范围 `git clean`、`git reset --hard` 或基于 checkout 的回退。
- 禁止按通配模式删除未跟踪文件；只能删除经过检查的明确路径。
- 保留无关的用户改动。若修改区域重叠，应在现有改动基础上继续处理，不能回退。
- 清理生成物时遵循 `docs/PROJECT_HYGIENE.md`，优先使用
  `scripts/clean-workspace.ps1`。
- `<repository-root>` 已完成物理迁移，是唯一规范开发和发布路径。旧工作区仅作为本地回退
  副本，不能作为常规开发或发布入口。
- 2026-07-15 用户体验升级前的可回退快照位于
  `backups/OpenAD-pre-upgrade-20260715-ux-baseline`。快照包含
  `BACKUP_MANIFEST.json` 和 `SHA256SUMS.csv`，不得当作普通构建缓存删除。

## 产品身份

- 用户看到的产品名称是 **OpenAD**。
- `PermissionProtector` 暂时保留在部分可执行文件名、命名空间、安装包目录和数据目录中，
  仅用于兼容。界面中不能再把它当作主产品名称。
- OpenAD 是面向 Windows 的开源化桌面产品，覆盖 Active Directory 身份、NTFS 权限、
  有效访问分析、风险检查和报告输出。
- 当前版本对 AD 保持只读。未来如需写操作，必须有明确需求、权限边界和安全评审。

## 当前有效架构

- `apps/backend`：Go 1.23 API、扫描器、AD 集成、SQLite、导出、审计和访问分析。
- `apps/web`：Next.js 14 + React 18 产品界面和静态导出。
- `apps/desktop-win`：当前正式 Windows 桌面壳，.NET 10 WinForms + WebView2。
- `apps/desktop-win.tests`：桌面运行时、品牌、启动和窗口缩放测试。
- `apps/desktop`：历史 Tauri 探索，不是当前交付桌面端。
- `scripts/build-desktop-windows.ps1`：Windows 桌面包权威构建入口。

运行端口：

- API：`127.0.0.1:18080`
- 桌面包内置 Web：`127.0.0.1:43110`
- Web 开发服务器：`localhost:3010`

## 产品模块边界

- 总览负责运行状态、摘要和常用操作入口。
- 目录浏览负责 AD 树、用户/组搜索、自动补全、对象详情、所属组和组成员。
- 扫描中心只负责目标目录、扫描参数、AD 状态、开始/取消/重试、实时进度和完成摘要。
- 报告中心是独立 `/reports` 模块，负责用户、文件夹、所有者报告、模板、字段与范围、
  预览和导出。
- 系统设置只负责应用、连接、外观、语言和本地运行时配置。扫描流程字段和报告元数据
  不属于系统设置。
- 禁止恢复旧网页后台壳，也不能把报告矩阵、ACL 证据大表重新放回扫描中心。

## 前端规则

- 沿用当前 OpenAD 桌面壳和现有样式体系，禁止再建立第二套视觉系统。
- 优先复用 `AppShellV2`、`DesktopWindowFrame`、共享导航和现有 UI 控件。
- 界面图标优先使用项目已启用的 Lucide 图标库。
- 运维页面应紧凑、克制、便于重复操作，避免营销式大标题、装饰卡片、卡片嵌套和功能
  说明文案。
- 报告中心保持为“输出”分组下的最后一个导航项。
- 侧边栏默认紧凑并从模块导航直接开始；窗口标题栏是唯一的 OpenAD 产品身份。鼠标悬停或
  键盘焦点进入时临时展开，离开后自动收起；固定按钮只在展开状态下，与首个“工作台”分组
  共用工具行，负责持久展开或收起。不得恢复独立品牌头部、重复图标、旧副标题或空白占位。
  临时展开不能遮挡内容、造成布局跳动或让导航项失去可点击性。
- 桌面窗口必须支持上、下、左、右四边和四个角缩放。Web resize hit areas 只用于底层
  命中判定，任何状态下都不得显示边框、色块、方向提示或高亮线。
- 桌面端 WebView2 缓存目录必须由打包 Web 资源内容指纹隔离；本地静态 Web 服务必须禁止
  陈旧缓存，确保升级后的桌面包不会加载旧版 `_next` 脚本。
- 顶部搜索同时承担 AD 用户/组、文件路径和模块命令搜索；模块命令必须从
  `openadNavigation` 派生，禁止维护第二套路由表。`/reports`、`/scan`、`/settings`
  等命令必须有稳定行为，未知 `/` 命令不能误当文件路径。
- 本地语言设置必须在 Next.js hydration 后读取，并在初始化完成前禁止覆盖已保存语言；
  任何改动都要同时检查整页跳转和 SSR hydration。
- 必须验证文字适配、滚动、空状态、加载状态、错误状态和桌面窗口缩放。

## 验证矩阵

开发过程中先运行最小相关检查，交付前再运行对应模块的完整检查。

```powershell
# 后端
& .\tools\go\bin\go.exe -C .\apps\backend test ./...

# Web
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static

# 桌面端
dotnet test .\apps\desktop-win.tests\PermissionProtector.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\PermissionProtector.Desktop.csproj -c Release

# 完整 Windows 桌面包
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

涉及可见桌面行为时，还要启动实际桌面包，检查窗口、运行端口、`/health` 和现有 AD
连接状态。涉及侧栏时，还要检查紧凑和展开状态均无品牌空头部，固定按钮只在展开的
“工作台”工具行出现，图标、文字与导航目标均可见且可用。

## 文档维护规则

- 当前项目治理、开发和维护文档采用中英双语，中文完整正文在前，英文对应内容在后；历史记录可以保留中文原文并附英文摘要。命令、路径、代码标识、协议字段和兼容文件名保留英文且不重复翻译。
- `.codex/memory.md` 只记录长期有效的项目事实、已确认决策、验证基线和未解决风险，
  禁止写入密码、令牌或个人目录数据。
- 用户可见行为变化要更新 `CHANGELOG.md`。
- 开发命令或前置条件变化要更新 `DEVELOPMENT.md`。
- 不可逆或跨模块架构决策要增加 ADR。
- 未经项目所有者明确选择，不得擅自添加开源许可证。

## English Reference

This file applies only to the OpenAD project under `<repository-root>`. It must not be copied into
global Codex or agent configuration.

Before changing code, read the latest on-disk `AGENTS.md`, `DEVELOPMENT.md`, Git status, and the
implementation and tests closest to the request. Read `.codex/memory.md` when it exists in a local
maintainer workspace.
The workspace is shared by multiple tasks, so older task context is not authoritative.

Public clones do not include `.codex`. When the local memory file is absent, treat `AGENTS.md`,
`DEVELOPMENT.md`, current source, and tests as authoritative rather than treating its absence as an error.

Repository safety rules:

- The canonical OpenAD publication Git root is `<repository-root>`. A legacy workspace still managed
  by its parent Git repository is a local recovery copy only and must never be used for publishing.
- The first public baseline must include source, tests, documentation, assets, scripts, and
  configuration unless the files are proven generated.
- Do not run broad `git clean`, `git reset --hard`, checkout-based rollback, or wildcard deletion of untracked files.
- Preserve unrelated changes and use `scripts/clean-workspace.ps1` only within the boundaries documented in `docs/PROJECT_HYGIENE.md`.
- `<repository-root>` is now a physical, independent Git repository and the only canonical development
  and publication path. The legacy workspace is a local recovery copy only and must not be used for
  normal development or publishing.
- Preserve the verified pre-upgrade snapshot at `backups/OpenAD-pre-upgrade-20260715-ux-baseline`, including its manifest and SHA-256 inventory.

Product and module rules:

- The user-visible product name is OpenAD. `PermissionProtector` remains only where compatibility currently requires it.
- Active Directory operations remain read-only unless a separately reviewed requirement changes that boundary.
- `apps/backend` owns the Go API and domain logic, `apps/web` owns the product UI, and `apps/desktop-win` owns the shipping WinForms/WebView2 desktop host.
- Directory Explorer owns AD browsing, search, autocomplete, object details, memberships, and snapshots.
- Scan Center owns scan setup, execution, progress, and completion summary only.
- Report Center is the independent `/reports` module and owns templates, fields, scope, preview, export, and historical sessions.
- System Settings owns application, connection, appearance, language, and local runtime settings only.
- Keep the current OpenAD desktop shell and shared design system. Do not restore the legacy web-admin shell or move report workspaces back into Scan Center.
- The sidebar is compact by default and begins directly with module navigation; the window title bar is the only OpenAD product identity. It expands temporarily on pointer hover or keyboard focus and collapses after leaving. The pin control appears only in the expanded state, sharing the first Workspace group tool row, and owns persistent expansion or collapse. Do not restore a separate branded header, duplicate icon, legacy tagline, or empty placeholder. Temporary expansion must not obscure content, cause disruptive layout shifts, or make navigation targets unavailable.
- Desktop resizing must work from all four edges and all four corners. Web resize hit areas are implementation-only and must never expose borders, fills, direction markers, or highlight lines in any state.
- The desktop WebView2 cache directory must be isolated by a fingerprint of the bundled Web assets, and the local static Web service must disallow stale caching so a package upgrade never executes an old `_next` script.
- Derive global module commands from `openadNavigation`, keep directory/path search distinct from slash commands, and reject unknown slash commands without navigation.
- Read persisted locale only after hydration and never overwrite it during initialization.

Verification and documentation:

- Run the smallest relevant tests during development and the complete affected-module checks before delivery, using `DEVELOPMENT.md` as the command source.
- Visible desktop changes also require launching the packaged app and checking the window, ports, `/health`, AD connectivity, and all eight resize directions when relevant. For sidebar changes, verify both compact and expanded states have no empty branded header, the pin action appears only in the expanded Workspace tool row, and icons, text, and navigation targets remain visible and usable.
- Current project governance, development, and maintenance documents are bilingual: complete Chinese content first, corresponding English content second. Historical records may preserve the Chinese original with an English summary. Commands, paths, identifiers, protocol fields, and compatibility filenames remain in English.
- Update `CHANGELOG.md` for user-visible behavior, `DEVELOPMENT.md` for command or prerequisite changes, and add an ADR for irreversible or cross-module architecture decisions.
- Do not add an open-source license until the project owner explicitly selects one.
