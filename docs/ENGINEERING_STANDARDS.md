# OpenAD 工程规范

[中文](#openad-工程规范) | [English](#openad-engineering-standards)

最后更新：2026-07-15

本规范描述当前 OpenAD 的真实实现。当历史文档与当前代码冲突时，以源码和测试为准。

## 架构边界

- Go 负责目录集成、扫描、持久化、有效权限计算、导出和 API。
- Next.js 负责产品界面和静态 Web 包。
- WinForms/WebView2 负责原生启动、生命周期、窗口边框、运行进程和桌面集成。
- 禁止在桌面壳或展示组件中复制领域逻辑。

## 前端结构

主要壳和样式来源：

- `components/shell/AppShellV2.tsx`
- `components/shell/DesktopWindowFrame.tsx`
- `styles/tokens.css`
- `styles/desktop-theme.css`
- `styles/desktop-shell.css`
- `styles/openad-operations.css`

增加新 CSS 或抽象前，先复用共享令牌和现有组件。禁止再引入一套主题系统，也不能恢复
旧网页后台布局。

## 产品界面规范

- 运维界面应紧凑、克制、清晰，面向高频重复操作，而不是营销展示。
- 常见操作使用 Lucide 图标，并提供无障碍名称或工具提示。
- 避免卡片嵌套、装饰性 Hero、功能说明文案和无法完成完整操作的控件。
- 路由标题、导航名称和页面标题保持一致。
- 新路由必须具备加载、空、错误和受限窗口状态。
- 可见界面变化必须在正常桌面尺寸和最小支持窗口尺寸下检查，不能出现重叠或控件裁切。

## 模块职责

- 目录浏览：AD 树、自动补全、搜索、对象详情、所属组、组成员和快照同步。
- 扫描中心：扫描目标、参数、AD 状态、运行控制、进度和完成摘要。
- 报告中心：报告模式、模板、字段、范围、预览、导出和历史会话。
- 系统设置：连接、产品身份、外观、语言和运行时。

功能跨模块时，共享数据逻辑只能保留一份，通过明确的路由、会话和路径参数连接工作流。

## 后端规范

- AD 操作保持只读。
- 使用结构化请求/响应类型和集中路由注册。
- 限制目录查询、分页、嵌套成员展开和递归范围。
- 长时间扫描必须保留取消和进度语义。
- 禁止返回保存的密码、加密密钥或原始机密材料。
- 权限控制、路径处理、递归和持久化变化必须补测试。

## 桌面端规范

- 原生职责保留在 `apps/desktop-win`。
- WebView 中的产品行为保留在 `apps/web`。
- 未设计并验证迁移行为前，不得修改兼容可执行文件名和数据路径。
- 修改窗口边框时，要验证启动状态、运行时校验、品牌元数据和八个缩放方向。

## 源码与文档

- 优先使用职责单一的文件和项目已有模式。
- 注释只解释非显而易见的约束，不逐行叙述代码。
- 技术注释使用 ASCII 标点，避免编码和解析问题。
- `AGENTS.md` 记录长期开发规则，`.codex/memory.md` 记录长期项目状态，记忆文件
  不能当作更新日志。
- 用户可见变化写入 `CHANGELOG.md`，发布阻塞项写入
  `docs/OPEN_SOURCE_RELEASE_CHECKLIST.md`。

## 验证要求

使用 `DEVELOPMENT.md` 中的命令。交付时必须提供受影响模块的最新验证证据。编译通过
不能替代行为测试，涉及桌面窗口和启动的修改也不能只依赖单元测试。

## OpenAD Engineering Standards

Last updated: 2026-07-15

These standards describe the current OpenAD implementation. Source code and tests take precedence when historical documentation conflicts with current behavior.

### Architecture Boundaries

- Go owns directory integration, scanning, persistence, effective access computation, export, and API behavior.
- Next.js owns the product UI and static Web package.
- WinForms/WebView2 owns native startup, lifecycle, window chrome, child processes, and desktop integration.
- Do not duplicate domain logic in the desktop host or presentation components.

### Frontend Structure

Primary shell and style sources are `components/shell/AppShellV2.tsx`, `components/shell/DesktopWindowFrame.tsx`, and the shared files under `styles/`. Reuse existing tokens and components before adding CSS or abstractions. Do not introduce a second theme system or restore the legacy web-admin layout.

### Product Interface Standards

- Operational screens should be compact, restrained, and optimized for repeated work rather than marketing presentation.
- Use the existing Lucide icon set for common actions and provide accessible names or tooltips.
- Avoid nested cards, decorative hero sections, explanatory feature copy, and controls that cannot complete their workflow.
- Keep route, navigation, and page titles consistent.
- New routes must include loading, empty, error, and constrained-window states.
- Check visible changes at normal desktop size and the minimum supported window size; text and controls must not overlap or clip.

### Module Responsibilities

- Directory Explorer: AD tree, autocomplete, search, object details, memberships, group members, and snapshot synchronization.
- Scan Center: scan target, parameters, AD status, runtime controls, progress, and completion summary.
- Report Center: report mode, templates, fields, scope, preview, export, and historical sessions.
- System Settings: connections, product identity, appearance, language, and runtime configuration.

When a workflow crosses modules, keep shared data logic in one place and connect the workflow through explicit route, session, and path parameters.

### Backend Standards

- Keep AD operations read-only.
- Use structured request and response types with centralized route registration.
- Bound directory queries, pagination, nested membership expansion, and recursion.
- Preserve cancellation and progress semantics for long-running scans.
- Never return saved passwords, encryption keys, or raw secret material.
- Add tests for permission control, path handling, recursion, and persistence changes.

### Desktop Standards

- Keep native responsibilities in `apps/desktop-win` and WebView product behavior in `apps/web`.
- Do not change compatibility executable names or data paths without a designed and verified migration.
- Window chrome changes must verify startup state, runtime validation, branding metadata, and all eight resize directions.

### Source, Documentation, and Verification

- Prefer single-purpose files and established project patterns.
- Comments should explain non-obvious constraints, not narrate individual lines. Technical comments use ASCII punctuation to reduce encoding and parser risk.
- `AGENTS.md` stores durable development rules; `.codex/memory.md` stores durable project state and is not a changelog.
- Record user-visible changes in `CHANGELOG.md` and release blockers in `docs/OPEN_SOURCE_RELEASE_CHECKLIST.md`.
- Use the commands in `DEVELOPMENT.md` and provide fresh evidence for affected modules. Compilation does not replace behavioral tests, and desktop startup or window changes cannot rely on unit tests alone.
