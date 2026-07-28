# OpenAD 变更日志 / Changelog

[中文](#中文) | [English](#english)

## 中文

### 未发布

#### 新增

- 确立 OpenAD open core 双许可结构：除 `ee/` 外的仓库主体采用 AGPL-3.0，新增
  `LICENSING.md`、`NOTICE` 及 `ee/` 商业许可边界。
- 在 `CONTRIBUTING.md` 中加入保留贡献者版权且允许项目所有者在商业许可下再授权贡献的 CLA；
  在 `SECURITY.md` 中加入待所有者替换的私密安全邮箱占位符和 GitHub 私密漏洞报告指引。
- 新增顶层 `README.md`，如实说明产品能力、仓库结构、当前状态和治理文档入口。
- 新增 `SECURITY.md`，记录安全问题报告方式和当前安全支持范围。
- 新增 `CONTRIBUTING.md`，说明产品、文档和验证证据的贡献要求。
- 新增 `docs/FEATURE_MATRIX.md`，记录已支持、部分支持和暂不支持的能力。
- 新增 `docs/KNOWN_ISSUES.md`，记录当前发布限制。
- 新增项目级 `AGENTS.md`、`.codex/memory.md`、工程规范、目录清理规范、发布检查表、ADR 和 GitHub 协作模板。
- 新增可校验的升级前快照和 `<repository-root>` 开发入口，支持本轮体验升级回退。
- 顶部工作区搜索新增 `/reports`、`/scan`、`/settings` 等模块命令，并复用共享导航定义。

#### 变更

- 首页总览现在直接提供未连接 AD 时的快速连接、四级风险分布、最近权限条目数和最后扫描时间；遗留 `/dashboard` 地址在静态导出中保留页面并无闪烁地跳转到首页。
- **破坏性变更（网络默认值）**：Go API、静态 Web、开发服务器和 Windows 浏览器模式启动器现在默认只绑定 `127.0.0.1`，空 `ALLOW_ORIGINS` 只允许本机 Web 来源；依赖默认 LAN 访问的部署必须改为显式开启。恢复方法：以管理员身份运行 `scripts\enable-lan-access.bat`，替换脚本输出命令中的 `your-host-ip` 后执行该一次性 LAN 启动命令。
- Windows 桌面启动画面现在跟随已保存的 `zh-CN` 或 `en` 应用语言；Web 界面通过受限消息桥接把 locale 写入兼容数据目录，首次启动则按 Windows UI 文化选择中文或英文。
- 桌面启动画面的最短可见时长由 1.8 秒调整为 750 毫秒，并在成功启动后使用 240 毫秒淡出；底部同时显示程序集版本，便于问题定位。
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

- 修复 SQLite 在扫描写入与历史查询并发时可能立即返回 `database is locked` 的问题；SQLite 连接现在使用 WAL、5 秒 `busy_timeout` 和单连接池串行写入，PostgreSQL 配置保持不变。
- 修复同一 `scan_id` 的重复扫描会覆盖取消函数、以及旧扫描清理可能误删新注册项的问题；重复 ID 和超出扫描并发上限的请求现在明确返回 HTTP 409，默认同时只运行一个扫描。
- 修复大结果导出请求无大小限制且下载前把完整临时文件再次读入内存的问题；导出 JSON 请求现在限制为 64 MiB，下载改为流式响应并在发送完成后清理临时文件。
- 修复首页“需要关注”直接截取前三条开放风险的问题；现在按 `priorityScore` 或风险级别回退分值降序选择最高优先级的三条风险。
- 修复桌面窗口右侧 resize 热区遮住内容滚动条的问题；可滚动工作区现在按与热区一致的 `12px` 逻辑宽度向内避让，同时保留四边四角缩放和完全不可见的命中区。
- 修复 AD 组成员与主体查询丢弃扫描 context、导致主动取消后仍继续展开大组的问题；LDAP 查询和成员循环现在传播取消，并继续由现有扫描状态显示为“已取消”而不是失败。
- 修复下载导出文件名可通过引号、分号或控制字符破坏 `Content-Disposition` 的问题；服务端落盘与下载共用同一清洗逻辑，并通过 RFC 6266 `filename*` 正确传递中文文件名。
- 修复重复启动 OpenAD 时第二个进程因端口占用显示启动错误的问题；同一 Windows 会话现在只运行一个桌面实例，重复启动会还原并激活已有窗口。
- 修复 `/api/scan/ws` 无条件接受任意浏览器 Origin 的问题；现在只放行同源、本机/回环来源、无 `Origin` 的非浏览器客户端和通过 `WEBSOCKET_ALLOWED_ORIGINS`（兼容 `FSA_WEBSOCKET_ALLOWED_ORIGINS`）显式配置的来源，并记录被拒绝的 Origin。
- 修复桌面启动画面的布局、标题栏拖拽区、窗口按钮和点阵背景未随 DPI 一致缩放的问题，并为主标题字体增加 `Segoe UI` 回退。
- 修复 AD 目录树遇到 LDAP size/admin limit 时降级为用户或结构节点过滤器导致组/用户被吞掉的问题；目录树现在坚持使用 LDAP paged control 翻页，不再用缩窄过滤器代替完整结果。
- 修复 AD 用户查询只返回直接 `memberOf` DN 的问题；用户结果现在保留直接组 DN，并在 `groups` 中返回包含嵌套组的有效组显示名。
- 修复 AD 同步快照落库缺少 `first_name`、`last_name`、`division` 和 `domain`，导致目录浏览与报告字段不一致的问题。
- 修复顶部搜索把未知 `/` 命令当作文件路径、以及报告中心无法通过模块命令稳定进入的问题。
- 修复已保存中文在整页跳转时被默认英文覆盖的问题，并避免 Next.js hydration 不一致。
- 修复无边框桌面窗口丢失原生缩放样式和 Web 缩放热区未贴合边缘的问题。
- 隐藏桌面窗口四边和四角的 resize hit areas 视觉提示，保留完整缩放命中能力和方向光标。
- 修复中文报告列表仍显示 `high`、`medium`、`low` 英文风险值的问题。
- 修复桌面包升级后 WebView2 可能继续加载旧版 `_next` 脚本的问题；本地静态资源禁止陈旧缓存，且 WebView2 缓存目录会随打包内容指纹自动隔离。
- 补齐总览、目录同步、资源清单、文件活动、操作审计和报告会话的首次加载、空结果、可重试错误与 AD/Windows 审计前置条件状态；相关 AD 引导统一进入系统设置。
- 让“紧凑表格”设置作用于所有工作区表格，并修复窄窗口下宽表滚动、表头吸顶、长路径/DN/SID 完整值提示和后端风险/扫描状态枚举未本地化的问题。

## English

### Unreleased

#### Added

- Established OpenAD's open-core dual-license structure: the repository body excluding `ee/` is under
  AGPL-3.0, with new `LICENSING.md`, `NOTICE`, and a commercial license boundary under `ee/`.
- Added CLA terms to `CONTRIBUTING.md` that preserve contributor copyright while allowing commercial
  sublicensing by the project owner, plus a security-contact placeholder and GitHub private-reporting
  guidance in `SECURITY.md`.
- Added a top-level `README.md` with a repository-grounded product overview, layout, current status, and governance links.
- Added `SECURITY.md` with vulnerability-reporting guidance and the current security posture.
- Added `CONTRIBUTING.md` with contribution rules for product changes, documentation, and verification evidence.
- Added `docs/FEATURE_MATRIX.md` for supported, conditional, and unsupported capabilities.
- Added `docs/KNOWN_ISSUES.md` for current release limitations.
- Added project-local `AGENTS.md`, `.codex/memory.md`, engineering and workspace hygiene standards, a release checklist, ADRs, and GitHub collaboration templates.
- Added a verified pre-upgrade snapshot and the `<repository-root>` development entry point for rollback during this UX upgrade.
- Added `/reports`, `/scan`, `/settings`, and other module commands to workspace search, derived from shared navigation definitions.

#### Changed

- The Overview now provides inline quick AD connection when disconnected, a four-level risk distribution, the latest permission count, and the last scan time. The legacy `/dashboard` static-export page now redirects to home without rendering stale content.
- **Breaking change (network defaults):** The Go API, static Web server, development server, and Windows browser-mode launchers now bind only to `127.0.0.1` by default, while an empty `ALLOW_ORIGINS` allows only local Web origins. Deployments that relied on implicit LAN access must opt in explicitly. To restore LAN access, run `scripts\enable-lan-access.bat` as Administrator, replace `your-host-ip` in the printed command, and run that one-time LAN launch command.
- The Windows startup experience now follows the saved `zh-CN` or `en` application locale. The Web UI sends the active locale through a constrained host bridge into the compatibility data directory, while first launch falls back to the Windows UI culture.
- Reduced the startup minimum visibility from 1.8 seconds to 750 milliseconds, added a 240-millisecond success fade, and exposed the assembly version in the footer for diagnostics.
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

- Fixed SQLite returning `database is locked` immediately during concurrent scan writes and history reads. SQLite now uses WAL, a five-second `busy_timeout`, and a single-connection pool to serialize writes; PostgreSQL configuration is unchanged.
- Fixed duplicate scans with the same `scan_id` overwriting cancellation callbacks and stale scan cleanup removing a newer registration. Duplicate IDs and requests above the scan concurrency limit now return HTTP 409, with one concurrent scan by default.
- Fixed large exports having no request-size limit and reading the complete temporary download back into memory. Export JSON requests are now limited to 64 MiB, downloads stream from disk, and temporary files are removed after transmission.
- Fixed the Overview attention queue taking the first three open findings without sorting; it now selects the three highest-priority findings using `priorityScore` with severity-rank fallback.
- Fixed the right-edge desktop resize hit area covering the content scrollbar. Scrollable workspace content now keeps a `12px` logical inset matching the resize edge while preserving invisible hit targets and all edge/corner resize directions.
- Fixed AD group-member and principal lookups discarding the scan context and continuing large-group expansion after cancellation. LDAP searches and member loops now propagate cancellation through the existing cancelled scan state.
- Fixed download filenames allowing quotes, semicolons, or control characters to corrupt `Content-Disposition`. Server-side and download exports now share one sanitizer, with RFC 6266 `filename*` support for Unicode names.
- Fixed a second OpenAD launch showing a port-conflict startup error. One desktop instance now runs per Windows session, and subsequent launches restore and activate the existing window.
- Fixed `/api/scan/ws` accepting every browser Origin. It now allows only same-origin, local/loopback origins, non-browser clients without `Origin`, and origins explicitly configured through `WEBSOCKET_ALLOWED_ORIGINS` (with `FSA_WEBSOCKET_ALLOWED_ORIGINS` compatibility fallback), while logging rejected Origins.
- Fixed startup layout, title-bar drag targeting, window controls, and background dots so they scale consistently with DPI, with an explicit `Segoe UI` fallback for the display title font.
- Fixed AD tree queries degrading to users-only or structural-only filters after LDAP size/admin limits, which could hide groups and users; tree loading now relies on LDAP paged controls instead of narrowing the content filter.
- Fixed AD user results only exposing direct `memberOf` DNs; user responses now preserve direct group DNs and return effective nested group display names in `groups`.
- Fixed AD sync snapshots missing `first_name`, `last_name`, `division`, and `domain`, keeping directory browsing and report data consistent.
- Fixed unknown slash commands being treated as paths and restored stable module-command access to Report Center.
- Fixed persisted Chinese being overwritten during full-page navigation while preserving hydration-safe rendering.
- Restored native borderless resize styles and aligned Web resize hit areas to the true window edges.
- Hid visual indicators for all edge and corner resize hit areas while preserving full resize targeting and directional cursors.
- Localized raw `high`, `medium`, and `low` risk values in Chinese report lists.
- Fixed packaged desktop upgrades potentially reusing stale `_next` scripts in WebView2. Local static assets now disallow stale caching, and the WebView2 profile is isolated by a fingerprint of the bundled Web assets.
- Completed initial loading, empty-result, retryable-error, and AD/Windows-auditing prerequisite states across Overview, Directory Sync, Resources, File Activity, Operation Audit, and report sessions; related AD guidance now opens System Settings.
- Applied the Compact Tables setting across all workspace tables and fixed narrow-window scrolling, sticky headers, full-value hints for long paths/DNs/SIDs, and localization of backend risk and scan-status enums.
- Fixed packaged desktop upgrades potentially reusing stale `_next` scripts in WebView2. Local static assets now disallow stale caching, and the WebView2 profile is isolated by a fingerprint of the bundled Web assets.
