# ADR 0002：OpenAD 公开品牌与仓库迁移

[中文](#adr-0002openad-公开品牌与仓库迁移) | [English](#adr-0002-openad-public-brand-and-repository-migration)

- 状态：已接受
- 日期：2026-07-21
- 负责人：OpenAD 项目维护者

## 背景

OpenAD 的实际源码目录历史上使用 `folder-security-analyzer-release-no-go`，而
`<repository-root>` 曾只是指向该目录的 Junction。父目录 `<workspace-parent>` 还包含无关项目和
个人资料，因此不能作为 OpenAD 的公开 Git 根。产品界面已经以 OpenAD 为主品牌，但少量
用户可见文字、报告默认标题和开发路径仍引用旧名称或 PermissionProtector。

当前工作区被运行中的开发环境占用，Windows 不允许立即移动实际目录。公开源代码仍需要
使用正确名称、完整边界和可审计的首次提交发布到 `weibinliao/OpenAD`。

## 决策

- `<repository-root>` 是 OpenAD 的规范发布目录和独立 Git 根。
- 在当前工作区句柄释放前，从活动源目录生成一个排除缓存、备份、本地代理状态和本地设计
  工作文件的干净 OpenAD 发布副本；该副本是 GitHub 唯一发布源。
- 所有用户可见的窗口标题、导航、空状态、错误提示、报告默认标题、README 和公开开发文档
  使用 OpenAD。
- PermissionProtector 只可作为不可见兼容标识保留在已有本地数据路径、WebView 协议键、
  历史可执行文件和命名空间中。公开文档仅能在明确的兼容说明中提及它。
- 首次提交不包含 `.codex`、`.agents`、`.design`、`网页设计`、`backups`、`dist`、`tools`、
  本地数据库、缓存、依赖目录、抓包或凭据文件。

## 影响

- GitHub 仓库、正式文档和用户产品体验会统一显示 OpenAD。
- 已有本地安装仍可通过兼容名称读取历史数据；兼容层的正式删除需要另一个经过升级与回退
  验证的 ADR。
- 当前旧目录在本工作区关闭前保留为回退副本，不能视为 GitHub 发布源。

## 备选方案

- 直接从父目录 `<workspace-parent>` 推送：未采用，会混入无关项目且无法启用项目内 CI。
- 立即全局替换 PermissionProtector：未采用，会破坏现有数据路径、桌面宿主和升级兼容性。
- 等待物理目录可移动后再开始发布：未采用，会无谓阻塞已验证的独立公开源码基线。

## 验证

- 前端品牌契约测试证明公开 UI 文本不再显示 PermissionProtector。
- 前端 typecheck、测试和静态导出通过；桌面测试与构建通过。
- 独立仓库的 `git status` 只包含 OpenAD 源码与文档，且远端为
  `https://github.com/weibinliao/OpenAD.git`。
- GitHub 的 `main` 分支可从干净 clone 构建，不包含本地配置、备份或凭据。

## ADR 0002: OpenAD Public Brand and Repository Migration

- Status: Accepted
- Date: 2026-07-21
- Owner: OpenAD maintainers

### Context

The active source directory historically used `folder-security-analyzer-release-no-go`, while
`<repository-root>` was only a Junction to it. The parent `<workspace-parent>` also contains unrelated
projects and personal material, so it cannot be the public Git root for OpenAD. The application
already presents OpenAD as its primary product name, but a small number of visible strings, report
defaults, and development paths still use legacy names.

The active workspace is held by the development environment, so Windows cannot immediately move the
physical directory. Public source still needs a correctly named, complete, auditable initial commit
published to `weibinliao/OpenAD`.

### Decision

- `<repository-root>` is the canonical publication directory and independent Git root.
- Until the active workspace handle is released, create a clean OpenAD publication copy that excludes
  caches, backups, local agent state, and local design working files. That copy is the only GitHub
  publication source.
- Use OpenAD for every user-visible title, navigation label, empty state, error message, report
  default, README, and public development document.
- Retain PermissionProtector only as an invisible compatibility identifier for existing data paths,
  WebView protocol keys, historical executables, and namespaces. Public documentation may mention it
  only in an explicit compatibility note.
- Exclude `.codex`, `.agents`, `.design`, `网页设计`, `backups`, `dist`, `tools`, local databases,
  caches, dependency directories, packet captures, and credential files from the first commit.

### Consequences

- The GitHub repository, formal documentation, and visible product experience consistently use
  OpenAD.
- Existing local installations keep their data compatibility; removal of compatibility aliases needs
  a separate ADR with upgrade and rollback verification.
- The old active directory remains a local recovery copy until this workspace is closed and is never
  used as the GitHub publication source.

### Alternatives

- Push from `<workspace-parent>`: rejected because it mixes unrelated projects and cannot activate project CI.
- Globally rename PermissionProtector immediately: rejected because it would break local data paths,
  desktop hosting, and upgrade compatibility.
- Wait for the physical directory to become movable before publishing: rejected because it would
  unnecessarily block a verified independent source baseline.

### Verification

- Frontend branding-contract tests prove no public UI surface displays PermissionProtector.
- Frontend typecheck, tests, static export, desktop tests, and desktop build pass.
- The independent repository contains only OpenAD source and documentation and points to
  `https://github.com/weibinliao/OpenAD.git`.
- GitHub `main` can build from a clean clone without local configuration, backups, or credentials.
