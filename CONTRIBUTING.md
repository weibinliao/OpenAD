# 参与 OpenAD 开发

[中文](#参与-openad-开发) | [English](#contributing-to-openad)

修改项目前先阅读 `AGENTS.md` 和 `DEVELOPMENT.md`，其中定义了产品边界、仓库安全规则和
当前验证命令；维护者本地工作区存在 `.codex/memory.md` 时再一并阅读。

## 修改范围

- `apps/backend`：扫描器、API、AD、导出、权限分析、历史和审计
- `apps/web`：当前 OpenAD 产品界面
- `apps/desktop-win`：当前交付的 Windows 桌面壳
- `apps/desktop-win.tests`：桌面运行时和窗口行为测试
- `apps/desktop`：只有明确批准时才处理的历史 Tauri 代码
- `docs` 和根目录治理文件：产品、开发和发行文档

禁止恢复已移除的前端试验或旧网页后台壳。当前有效界面是 `apps/web`，当前有效桌面
宿主是 `apps/desktop-win`。

## 开始修改前

1. 读取最新 Git 状态，并检查最接近需求的实现和测试。
2. 确认功能归属模块，保持模块职责清晰。
3. 检查 `docs/FEATURE_MATRIX.md`；维护者本地工作区存在 `.codex/memory.md` 时一并核对。
4. 所有能力描述必须有代码和验证依据，不能宣传未实现能力或合规结论。
5. 用户可见行为变化要更新 `CHANGELOG.md`。

当前仓库边界和索引并不完整。禁止大范围 Git 清理或回退，也不能把未跟踪源码当作
可删除文件。

## 交付证据

- 后端：局部 Go 测试；涉及共享行为时再运行全量测试
- Web：局部 Jest 测试、typecheck；路由或壳变化还要运行静态构建
- 桌面端：xUnit、Release 构建；涉及启动、图标、窗口或缩放时还要实机验证
- 打包：完整桌面包构建和运行时健康检查
- 文档：根据当前代码核对命令、路径和能力描述

在 PR 模板中记录准确命令和结果。

## 产品边界

- 扫描中心负责扫描执行和进度，不负责报告工作台。
- 报告中心负责模板、配置、预览和导出。
- 系统设置负责应用和运行时配置，不承载业务流程数据。
- 用户可见产品名称为 OpenAD。
- 除非有单独评审的明确需求，Active Directory 始终保持只读。

## 安全要求

遵循 `SECURITY.md`。禁止提交密码、令牌、私钥、真实目录导出、本地数据库、审计日志
或包含敏感路径的扫描结果。

## 许可证

OpenAD 仓库主体（明确排除 `ee/`）采用 AGPL-3.0；`ee/` 适用独立商业许可。完整边界见
`LICENSING.md`。只有项目所有者 Weibin Liao 明确批准后，才能修改许可条款或目录边界。

### 贡献者许可协议（CLA）

向 OpenAD 提交 Pull Request，即表示贡献者同意以下条款：

1. 贡献者保留其贡献的版权。本协议是许可授权，不是版权转让。
2. 贡献者授予项目所有者 Weibin Liao 一项永久、全球、非独占、不可撤销、免版税且已付清
   的许可，允许其使用、复制、修改、创作衍生作品、公开展示、公开执行、分发、提供、进口及
   以其他方式利用该贡献，并**明确包含在任何许可条款（包括 OpenAD 商业许可）下再授权
   （sublicense）该贡献的权利**。
3. 贡献者声明其有权作出上述授权，且据其所知，该贡献不侵犯任何第三方的版权、专利、商业
   秘密或其他权利。贡献者提交雇主或其他主体拥有权利的材料前，必须先取得必要许可。
4. 每次提交 Pull Request 均视为贡献者已阅读并接受本 CLA；不同意这些条款时，不应提交贡献。

## Contributing to OpenAD

Read `AGENTS.md` and `DEVELOPMENT.md` before changing the project. They define product boundaries, repository safety rules, and current verification commands. Also read `.codex/memory.md` when it exists in a maintainer workspace.

### Change Scope

- `apps/backend`: scanner, API, AD, export, access analysis, history, and audit
- `apps/web`: the current OpenAD product interface
- `apps/desktop-win`: the shipping Windows desktop host
- `apps/desktop-win.tests`: desktop runtime and window behavior tests
- `apps/desktop`: historical Tauri code; change only with explicit approval
- `docs` and root governance files: product, development, and release documentation

Do not restore removed frontend experiments or the legacy web-admin shell. The active UI is `apps/web`, and the active desktop host is `apps/desktop-win`.

### Before Making Changes

1. Read the latest Git status and the implementation and tests closest to the request.
2. Confirm module ownership and keep responsibilities explicit.
3. Check current behavior in `docs/FEATURE_MATRIX.md` and, when it exists locally, `.codex/memory.md`.
4. Ground every capability claim in code and verification evidence. Do not claim unimplemented features or compliance coverage.
5. Update `CHANGELOG.md` for user-visible behavior changes.

The current repository boundary and index are incomplete. Do not perform broad Git cleanup or rollback, and do not treat untracked source as disposable.

### Delivery Evidence

- Backend: focused Go tests; run the full suite for shared behavior.
- Web: focused Jest tests and typecheck; routes or shell changes also require a static build.
- Desktop: xUnit and Release build; startup, icon, window, or resize changes also require testing the actual application.
- Packaging: complete desktop package build and runtime health checks.
- Documentation: verify commands, paths, and capability descriptions against current code.

Record exact commands and results in the pull request template.

### Product Boundaries

- Scan Center owns scan execution and progress, not the report workspace.
- Report Center owns templates, configuration, preview, and export.
- System Settings owns application and runtime configuration, not business workflow data.
- OpenAD is the user-visible product name.
- Active Directory remains read-only unless an explicit requirement receives separate review.

### Security and License

Follow `SECURITY.md`. Never commit passwords, tokens, private keys, real directory exports, local databases, audit logs, or scan results containing sensitive paths.

The main OpenAD repository, expressly excluding `ee/`, is licensed under AGPL-3.0. The `ee/` directory
is governed by a separate commercial license. See `LICENSING.md` for the complete boundary. License
terms or directory boundaries may be changed only with explicit approval from the project owner,
Weibin Liao.

#### Contributor License Agreement (CLA)

By submitting a Pull Request to OpenAD, the contributor agrees that:

1. The contributor retains copyright in the contribution. This is a license grant, not a copyright
   assignment.
2. The contributor grants project owner Weibin Liao a perpetual, worldwide, non-exclusive,
   irrevocable, royalty-free, fully paid-up license to use, reproduce, modify, prepare derivative works
   of, publicly display, publicly perform, distribute, make available, import, and otherwise exploit
   the contribution, **including the express right to sublicense the contribution under any license
   terms, including the OpenAD commercial license**.
3. The contributor represents that they have the right to make this grant and that, to their knowledge,
   the contribution does not infringe any third party's copyright, patent, trade secret, or other
   rights. The contributor must obtain any required permission before submitting material owned by an
   employer or another party.
4. Each submitted Pull Request constitutes the contributor's acceptance of this CLA. A contributor who
   does not agree to these terms must not submit a contribution.
