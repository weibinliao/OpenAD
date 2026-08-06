# OpenAD 开源发布检查表

[中文](#openad-开源发布检查表) | [English](#openad-open-source-release-checklist)

本检查表记录 OpenAD 对外开源前必须完成的工作。许可法律效力以 `LICENSE`、
`ee/LICENSE` 及项目所有者另行签发的书面商业授权为准。

## 阻塞性决策

- [x] 明确选择并批准 AGPL-3.0 开源许可证以及 `ee/` 商业许可边界。
- [x] 增加批准后的 `LICENSE`、`LICENSING.md`、`NOTICE` 和 `ee/LICENSE`，并在 `README.md` 中引用。
- [ ] 在 `SECURITY.md` 中公布可用的安全问题私密报告渠道（当前仍为
      `<SECURITY_CONTACT_EMAIL>` 占位符；也可启用并确认 GitHub Private Vulnerability Reporting）。
- [x] 确认标准 Git 仓库根目录，并提交完整源码基线。
- [x] 在标准仓库根目录启用 CI。

## 源码与构建

- [ ] 确认生成二进制、缓存、本地数据库、凭据和扫描导出已排除在版本控制外。
- [ ] 确认全新检出可以按文档安装依赖并构建 Windows 桌面包。
- [ ] 在 CI 中运行后端测试、前端测试/typecheck/静态导出和桌面端测试。
- [ ] 为发布包提供校验和与软件物料清单（SBOM）。
- [ ] 说明支持的 Windows 和 WebView2 版本。

## 产品与命名

- [ ] 明确 OpenAD 是产品名称，PermissionProtector 只存在于内部命名空间、兼容环境变量和
      旧数据迁移路径。
- [ ] 确认应用图标、可执行文件元数据、窗口标题、文档和发布包名称符合统一命名策略。
- [ ] 检查所有界面文案，删除没有实现依据的能力和合规声明。

## 安全与隐私

- [ ] 评审 AD 凭据、本地密钥、网络准入、审计日志、导出和文件系统访问。
- [ ] 说明 SQLite、日志和加密连接材料的保存位置。
- [ ] 确认公开历史中不存在真实 AD 身份、路径、凭据、数据库或扫描导出。
- [ ] 定义支持版本和漏洞响应策略。

## 社区文件

- [x] `README.md`
- [x] `CONTRIBUTING.md`
- [x] `SECURITY.md`
- [x] `CHANGELOG.md`
- [x] PR 模板
- [x] Bug 和功能需求模板
- [x] 项目所有者批准后的 `LICENSE` 与 `ee/LICENSE`
- [x] `CONTRIBUTING.md` 中支持商业再授权的 CLA 条款
- [ ] 对外支持和联系渠道

## OpenAD Open-Source Release Checklist

This checklist records work required before a public OpenAD release. The controlling legal terms are
`LICENSE`, `ee/LICENSE`, and any separate written commercial authorization issued by the project owner.

### Blocking Decisions

- [x] Explicitly select and approve AGPL-3.0 and the commercial license boundary for `ee/`.
- [x] Add the approved `LICENSE`, `LICENSING.md`, `NOTICE`, and `ee/LICENSE`, and reference them from `README.md`.
- [ ] Publish a working private vulnerability-reporting channel in `SECURITY.md` (currently the
      `<SECURITY_CONTACT_EMAIL>` placeholder; alternatively enable and confirm GitHub Private Vulnerability Reporting).
- [x] Establish the standard Git repository root and commit the complete source baseline.
- [x] Enable CI from the standard repository root.

### Source and Build

- [ ] Confirm generated binaries, caches, local databases, credentials, and scan exports are excluded from version control.
- [ ] Verify a clean checkout can install dependencies and build the Windows desktop package using documented commands.
- [ ] Run backend tests, Web tests/typecheck/static export, and desktop tests in CI.
- [ ] Publish checksums and a software bill of materials (SBOM) for release packages.
- [ ] Document supported Windows and WebView2 versions.

### Product and Naming

- [ ] State that OpenAD is the product name and PermissionProtector remains only in internal namespaces, compatibility environment variables, and legacy data migration paths.
- [ ] Align application icons, executable metadata, window titles, documentation, and package names with the approved naming strategy.
- [ ] Review all interface copy and remove capability or compliance claims without implementation evidence.

### Security and Privacy

- [ ] Review AD credentials, local keys, network admission, audit logs, exports, and file-system access.
- [ ] Document storage locations for SQLite databases, logs, and encrypted connection material.
- [ ] Confirm public history contains no real AD identities, paths, credentials, databases, or scan exports.
- [ ] Define supported versions and a vulnerability-response policy.

### Community Files

- [x] `README.md`
- [x] `CONTRIBUTING.md`
- [x] `SECURITY.md`
- [x] `CHANGELOG.md`
- [x] Pull request template
- [x] Bug and feature request templates
- [x] Project-owner-approved `LICENSE` and `ee/LICENSE`
- [x] CLA terms in `CONTRIBUTING.md` supporting commercial sublicensing
- [ ] Public support and contact channel
