# 功能矩阵

[中文](#功能矩阵) | [English](#feature-matrix)

本矩阵只依据当前仓库中的实际实现，不依据路线图推测。

| 能力 | 状态 | 说明 |
| --- | --- | --- |
| 扫描本地 NTFS 目录权限 | 已支持 | Windows 构建中的 Go 扫描器已实现。 |
| 扫描 UNC 或共享路径 | 有条件支持 | UNC 目录访问使用 OpenAD 后端的 Windows 运行身份；已验证的 AD 连接只增强 SID、用户、组和嵌套成员解析，不会授予共享访问权限。 |
| 包含继承权限 | 已支持 | 扫描请求和界面默认值中均已提供。 |
| 权限风险提示 | 已支持 | 扫描器记录 `risk_level` 和相关权限元数据。 |
| 权限暴露规则引擎 | 已支持 | 前端把 ACL 记录转换为包含类别、影响、证据和整改建议的评分风险。 |
| 敏感路径暴露判断 | 已支持 | 对人事、财务、薪资、法务、备份、源码、凭据、客户/PII、IT/管理和机密路径提高风险优先级。 |
| 风险处置工作流 | 已支持 | 风险可设为打开、接受、解决，并可在再次出现时重新打开。 |
| 文件访问活动审计 | 满足 Windows 条件时支持 | `/file-activity` 查询 Windows 安全事件元数据，不扫描文件内容；需要对象访问审计、SACL 和安全日志读取权限。 |
| 文件活动就绪检查 | 已支持 | `/api/file-activity/readiness` 检查安全日志、审计策略可见性和可选目录 SACL，并生成管理员命令，不自动修改系统。 |
| 会话持久化与历史 | 已支持 | 提供历史 API 和页面；未设置 `DATABASE_URL` 时本地默认使用 SQLite。 |
| 已完成会话对比 | 已支持 | 提供对比 API、CLI 命令和历史页面入口。 |
| 原始权限文件导出 | 已支持 | 提供 CSV、Excel 和 HTML 导出。 |
| 管理摘要 Markdown 导出 | 已支持 | Web 中提供摘要模板和 Markdown 导出流程。 |
| AD 连接测试 | 已支持 | 提供 `/api/ad/test`。 |
| AD 树浏览 | 已支持 | 目录浏览可加载 AD 树并选择用户、组、OU、容器和域。 |
| AD 用户和组查询 | 已支持 | 目录浏览支持防抖自动补全、用户所属组和组成员。 |
| 审计请求历史 | 已支持 | 提供审计工作台和审计导出接口。 |
| 本地浏览器操作日志 | 已支持 | 保存在浏览器本地存储，不是中心化服务器日志。 |
| 实时扫描进度 | 已支持 | 已提供 WebSocket 扫描进度流程。 |
| 报告中心 | 已支持 | `/reports` 提供用户、文件夹、所有者模式，以及模板、范围/字段配置、预览、历史会话和导出。 |
| Windows 静态发布包 | 已支持 | 打包脚本会构建后端服务、静态 Web、测试和桌面宿主。 |
| 桌面应用打包 | 已支持 | .NET WinForms/WebView2 壳会发布到 Windows 桌面包；Tauri 仅为历史探索。 |
| 桌面四边和四角缩放 | 已支持 | 原生和 WebView 缩放处理覆盖四条边和四个角。 |
| Linux/macOS NTFS 扫描 | 不支持 | 非 Windows 扫描器会返回“不支持”错误。 |
| 定时扫描 | 未证实 | 当前未发现完整调度实现。 |
| 通知或邮件告警 | 未证实 | 当前未发现完整消息投递流程。 |
| 产品登录或 API 令牌强制 | 未证实 | 已有设计方向，但尚未实现产品级认证。 |
| 自动更新 | 未证实 | 当前未发现完整更新机制。 |
| 对外 SLA 或托管服务 | 未定义 | 仓库中没有正式支持政策。 |

## 状态说明

- “已支持”表示仓库中存在明确且可验证的完整实现路径。
- “有条件支持”表示功能可用，但依赖系统配置或外部条件。
- “未证实”表示当前仓库不足以支持对外声明该能力。

## Feature Matrix

This matrix is based only on implementation present in the current repository, not roadmap assumptions.

| Capability | Status | Notes |
| --- | --- | --- |
| Scan local NTFS directory permissions | Supported | Implemented by the Go scanner in Windows builds. |
| Scan UNC or shared paths | Conditional | UNC directory access uses the Windows identity running the OpenAD backend. A verified AD connection enriches SID, user, group, and nested-membership resolution but does not grant share access. |
| Include inherited permissions | Supported | Available in both scan requests and UI defaults. |
| Permission risk hints | Supported | The scanner records `risk_level` and related permission metadata. |
| Permission exposure rule engine | Supported | The frontend converts ACL evidence into scored findings with category, impact, evidence, and remediation guidance. |
| Sensitive-path exposure detection | Supported | HR, finance, payroll, legal, backup, source, credential, customer/PII, IT/admin, and confidential paths receive higher priority. |
| Finding disposition workflow | Supported | Findings can be open, accepted, or resolved and can reopen when detected again. |
| File-access activity audit | Conditional on Windows | `/file-activity` queries Windows security-event metadata without scanning file content. Object-access auditing, SACLs, and security-log read access are required. |
| File-activity readiness check | Supported | `/api/file-activity/readiness` diagnoses security-log visibility, audit policy, and optional directory SACLs and generates administrator commands without changing the system. |
| Session persistence and history | Supported | History APIs and UI are available; local SQLite is the default when `DATABASE_URL` is unset. |
| Completed-session comparison | Supported | Comparison API, CLI command, and history-page entry points are implemented. |
| Raw permission export | Supported | CSV, Excel, and HTML export are available. |
| Management-summary Markdown export | Supported | The Web UI provides summary templates and Markdown export. |
| AD connection test | Supported | Available at `/api/ad/test`. |
| AD tree browsing | Supported | Directory Explorer loads the AD tree and selects users, groups, OUs, containers, and domains. |
| AD user and group search | Supported | Directory Explorer provides debounced autocomplete, user memberships, and group members. |
| Audit request history | Supported | Audit workspace and export endpoints are available. |
| Local browser-operation log | Supported | Stored in browser local storage; this is not a centralized server log. |
| Live scan progress | Supported | WebSocket scan progress is implemented. |
| Report Center | Supported | `/reports` provides user, folder, and owner modes, templates, scope and field configuration, preview, historical sessions, and export. |
| Windows static release package | Supported | The packaging script builds backend services, static Web assets, tests, and the desktop host. |
| Desktop application packaging | Supported | The .NET WinForms/WebView2 host ships in the Windows package; Tauri is historical exploration only. |
| Desktop resize from all edges and corners | Supported | Native and WebView resize handling cover all four edges and four corners. |
| Linux/macOS NTFS scanning | Unsupported | The non-Windows scanner returns an unsupported error. |
| Scheduled scans | Unverified | No complete scheduler implementation has been found. |
| Notifications or email alerts | Unverified | No complete delivery flow has been found. |
| Product login or mandatory API tokens | Unverified | A design direction exists, but product-level authentication is not implemented. |
| Automatic updates | Unverified | No complete update mechanism has been found. |
| Public SLA or hosted service | Undefined | The repository does not define a formal support policy. |

### Status Definitions

- **Supported** means the repository contains a clear, verifiable end-to-end implementation.
- **Conditional** means the capability is available but depends on system configuration or external prerequisites.
- **Unverified** means the repository does not provide enough evidence to make a public support claim.
