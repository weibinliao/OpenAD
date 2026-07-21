# OpenAD 开源发布检查表

[中文](#openad-开源发布检查表) | [English](#openad-open-source-release-checklist)

本检查表记录 OpenAD 对外开源前必须完成的工作，但不会代替项目所有者选择法律条款。

## 阻塞性决策

- [ ] 明确选择并批准开源许可证。
- [ ] 增加批准后的 `LICENSE`，并在 `README.md` 中引用。
- [ ] 在 `SECURITY.md` 中公布安全问题私密报告渠道。
- [x] 确认标准 Git 仓库根目录，并提交完整源码基线。
- [x] 在标准仓库根目录启用 CI。

## 源码与构建

- [ ] 确认生成二进制、缓存、本地数据库、凭据和扫描导出已排除在版本控制外。
- [ ] 确认全新检出可以按文档安装依赖并构建 Windows 桌面包。
- [ ] 在 CI 中运行后端测试、前端测试/typecheck/静态导出和桌面端测试。
- [ ] 为发布包提供校验和与软件物料清单（SBOM）。
- [ ] 说明支持的 Windows 和 WebView2 版本。

## 产品与命名

- [ ] 明确 OpenAD 是产品名称，PermissionProtector 是可执行文件、命名空间和本地数据
      路径的临时兼容名称。
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
- [ ] 项目所有者批准后的 `LICENSE`
- [ ] 对外支持和联系渠道

## OpenAD Open-Source Release Checklist

This checklist records work required before a public OpenAD release. It does not replace the project owner's legal license decision.

### Blocking Decisions

- [ ] Explicitly select and approve an open-source license.
- [ ] Add the approved `LICENSE` and reference it from `README.md`.
- [ ] Publish a private vulnerability-reporting channel in `SECURITY.md`.
- [x] Establish the standard Git repository root and commit the complete source baseline.
- [x] Enable CI from the standard repository root.

### Source and Build

- [ ] Confirm generated binaries, caches, local databases, credentials, and scan exports are excluded from version control.
- [ ] Verify a clean checkout can install dependencies and build the Windows desktop package using documented commands.
- [ ] Run backend tests, Web tests/typecheck/static export, and desktop tests in CI.
- [ ] Publish checksums and a software bill of materials (SBOM) for release packages.
- [ ] Document supported Windows and WebView2 versions.

### Product and Naming

- [ ] State that OpenAD is the product name and PermissionProtector is a temporary compatibility name for executables, namespaces, and local data paths.
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
- [ ] Project-owner-approved `LICENSE`
- [ ] Public support and contact channel
