# OpenAD 变更日志 / Changelog

[中文](#中文) | [English](#english)

## 中文

### 未发布

#### 新增

- 新增顶层 `README.md`，如实说明产品能力、仓库结构、当前状态和治理文档入口。
- 新增 `SECURITY.md`，记录安全问题报告方式和当前安全支持范围。
- 新增 `CONTRIBUTING.md`，说明产品、文档和验证证据的贡献要求。
- 新增 `docs/FEATURE_MATRIX.md`，记录已支持、部分支持和暂不支持的能力。
- 新增 `docs/KNOWN_ISSUES.md`，记录当前发布限制。
- 新增项目级 `AGENTS.md`、`.codex/memory.md`、工程规范、目录清理规范、发布检查表、ADR 和 GitHub 协作模板。
- 新增可校验的升级前快照和 `<repository-root>` 开发入口，支持本轮体验升级回退。
- 顶部工作区搜索新增 `/reports`、`/scan`、`/settings` 等模块命令，并复用共享导航定义。

#### 变更

- 公开仓库、规范开发路径、报告默认标题和所有用户可见产品文案统一使用 OpenAD；旧名称仅保留在经过说明的内部兼容层。
- 将 `/health`、桌面 Web 启动标记、报告服务端默认值、启动动画和 Windows 防火墙/登录任务名称统一为 OpenAD；旧数据目录、可执行文件名和升级清理键仅保留为兼容层。
- 首次公开基线排除本地代理状态、设计过程材料、审计截图和未复核历史文档，避免公开本机路径、身份或过时说明。
- 将 `docs/README.md` 调整为文档索引，并指向顶层 `README.md`。
- 用基于仓库实际状态的发布与验证说明替换 `docs/RELEASE_NOTES.md` 中不准确的表述。
- 调整 Windows 发布包内容，使生成的桌面包包含顶层 `README.md`，不再使用文档索引占位文件。
- 桌面包内说明改用 OpenAD 作为主产品名，并明确 `PermissionProtector.exe` 与旧数据目录仅为兼容名称。
- 在 `apps/web` 中持久化报告标题、组织名称、操作人、报告周期、权限映射和共享备注等默认值。
- 将权限翻译映射同时接入导出载荷和页面分组报告行，保持预览与导出结果一致。
- 项目治理、开发和维护文档改为中文在前、英文在后的双语格式。
- 报告中心压缩了重复标题、会话说明和嵌套容器；报告模式、配置操作、用户列表与结果表在常用桌面宽度下更紧凑。
- 侧边栏恢复桌面软件常用的悬停/焦点临时展开交互，离开后自动收起；固定按钮继续控制
  持久展开状态。
- 扫描中心继续保持单一职责，默认完整目录树深度，报告模板、预览和导出仍归独立报告中心所有。
- 统一桌面壳的产品层级：窗口标题栏只显示 OpenAD，侧栏从模块导航直接开始；固定按钮仅在展开时作为“工作台”工具行操作出现，避免重复品牌和无用途空白区。

#### 修复

- 修复 AD 目录树遇到 LDAP size/admin limit 时降级为用户或结构节点过滤器导致组/用户被吞掉的问题；目录树现在坚持使用 LDAP paged control 翻页，不再用缩窄过滤器代替完整结果。
- 修复 AD 用户查询只返回直接 `memberOf` DN 的问题；用户结果现在保留直接组 DN，并在 `groups` 中返回包含嵌套组的有效组显示名。
- 修复 AD 同步快照落库缺少 `first_name`、`last_name`、`division` 和 `domain`，导致目录浏览与报告字段不一致的问题。
- 修复顶部搜索把未知 `/` 命令当作文件路径、以及报告中心无法通过模块命令稳定进入的问题。
- 修复已保存中文在整页跳转时被默认英文覆盖的问题，并避免 Next.js hydration 不一致。
- 修复无边框桌面窗口丢失原生缩放样式和 Web 缩放热区未贴合边缘的问题。
- 隐藏桌面窗口四边和四角的 resize hit areas 视觉提示，保留完整缩放命中能力和方向光标。
- 修复中文报告列表仍显示 `high`、`medium`、`low` 英文风险值的问题。
- 修复桌面包升级后 WebView2 可能继续加载旧版 `_next` 脚本的问题；本地静态资源禁止陈旧缓存，且 WebView2 缓存目录会随打包内容指纹自动隔离。

## English

### Unreleased

#### Added

- Added a top-level `README.md` with a repository-grounded product overview, layout, current status, and governance links.
- Added `SECURITY.md` with vulnerability-reporting guidance and the current security posture.
- Added `CONTRIBUTING.md` with contribution rules for product changes, documentation, and verification evidence.
- Added `docs/FEATURE_MATRIX.md` for supported, conditional, and unsupported capabilities.
- Added `docs/KNOWN_ISSUES.md` for current release limitations.
- Added project-local `AGENTS.md`, `.codex/memory.md`, engineering and workspace hygiene standards, a release checklist, ADRs, and GitHub collaboration templates.
- Added a verified pre-upgrade snapshot and the `<repository-root>` development entry point for rollback during this UX upgrade.
- Added `/reports`, `/scan`, `/settings`, and other module commands to workspace search, derived from shared navigation definitions.

#### Changed

- Unified the public repository, canonical development path, report default title, and all user-visible product copy under OpenAD; legacy names remain only in documented internal compatibility layers.
- Unified `/health`, the desktop Web startup marker, server-side report defaults, startup animation, and Windows firewall/logon-task names under OpenAD; legacy data paths, executable names, and upgrade cleanup keys remain compatibility-only.
- Excluded local agent state, design-process material, audit screenshots, and unverified historical documents from the first public baseline to avoid publishing machine paths, identities, or stale guidance.
- Reworked `docs/README.md` into a documentation index that points to the top-level `README.md`.
- Replaced unsupported claims in `docs/RELEASE_NOTES.md` with repository-grounded release and verification notes.
- Updated Windows packaging so generated desktop packages include the top-level `README.md` instead of a documentation-index placeholder.
- Updated the packaged desktop notes to use OpenAD as the primary product name and identify `PermissionProtector.exe` and the legacy data directory as compatibility names only.
- Persisted report defaults for title, organization, operator, report period, permission mappings, and sharing notes in `apps/web`.
- Wired permission translation mappings into both export payloads and grouped on-page report rows so preview and export remain consistent.
- Changed project governance, development, and maintenance documentation to a bilingual format with Chinese first and English second.
- Reduced duplicate headings, session copy, and nested containers in Report Center, with denser report controls and side-by-side list/results layouts at common desktop widths.
- Restored desktop-style temporary sidebar expansion on hover and focus, with automatic collapse after leaving; the pin control continues to own persistent expansion.
- Kept Scan Center focused on scan execution with complete-tree depth as the default; templates, preview, and export remain owned by Report Center.
- Unified desktop-shell product hierarchy: the window title bar now shows only OpenAD and the sidebar starts directly with module navigation. Its pin control appears only as an action in the expanded Workspace row, eliminating redundant branding and an empty header region.

#### Fixed

- Fixed AD tree queries degrading to users-only or structural-only filters after LDAP size/admin limits, which could hide groups and users; tree loading now relies on LDAP paged controls instead of narrowing the content filter.
- Fixed AD user results only exposing direct `memberOf` DNs; user responses now preserve direct group DNs and return effective nested group display names in `groups`.
- Fixed AD sync snapshots missing `first_name`, `last_name`, `division`, and `domain`, keeping directory browsing and report data consistent.
- Fixed unknown slash commands being treated as paths and restored stable module-command access to Report Center.
- Fixed persisted Chinese being overwritten during full-page navigation while preserving hydration-safe rendering.
- Restored native borderless resize styles and aligned Web resize hit areas to the true window edges.
- Hid visual indicators for all edge and corner resize hit areas while preserving full resize targeting and directional cursors.
- Localized raw `high`, `medium`, and `low` risk values in Chinese report lists.
- Fixed packaged desktop upgrades potentially reusing stale `_next` scripts in WebView2. Local static assets now disallow stale caching, and the WebView2 profile is isolated by a fingerprint of the bundled Web assets.
- Fixed packaged desktop upgrades potentially reusing stale `_next` scripts in WebView2. Local static assets now disallow stale caching, and the WebView2 profile is isolated by a fingerprint of the bundled Web assets.
