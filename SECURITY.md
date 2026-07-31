# OpenAD 安全策略 / Security Policy

[中文](#中文) | [English](#english)

## 中文

### 当前支持范围

本仓库包含权限分析产品的 API、用户界面、报告导出和 Active Directory 集成。项目目前尚未定义按版本划分的安全支持周期，因此安全审查应以当前默认分支以及维护者正在使用的活动发布分支为准。

### 报告安全漏洞

> **发布阻塞项：** `<SECURITY_CONTACT_EMAIL>` 是占位符，当前不能接收报告。项目所有者必须
> 在对外发布前将其替换为真实、可用且可私密收件的安全邮箱。不要向 GitHub noreply 地址
> 发送安全报告。

发现安全问题时：

1. 不要在公开议题中披露利用细节。
2. 占位符替换完成后，通过 `<SECURITY_CONTACT_EMAIL>` 私密提交报告。
3. 如果仓库已启用 GitHub Private Vulnerability Reporting，研究者也可以在仓库的
   `Security` 页面选择 `Report a vulnerability` 私密提交；该功能必须由仓库所有者另行启用，
   本文不声称它当前已经启用。
4. 报告中应包含受影响区域、复现步骤、潜在影响，以及验证问题所需的最小概念验证材料。

在占位符被替换或 GitHub Private Vulnerability Reporting 被确认启用之前，本仓库尚无可供
外部研究者使用的已验证私密报告渠道，因此不能据此文档对外发布。

### 重点审查区域

- 发送到 API 端点的 AD 凭据。Web 客户端不再将 AD 密码持久化到本地存储；浏览器刷新后必须重新输入密码。
- 可能包含路径或身份数据的报告导出内容。
- 审计请求日志和本地浏览器操作日志。
- Windows 扫描器和桌面壳中的文件系统访问。

### 当前限制

- 私密安全邮箱仍是 `<SECURITY_CONTACT_EMAIL>` 占位符，尚未启用。
- 建议启用 GitHub Private Vulnerability Reporting；启用状态尚未在本文件中确认。
- 本仓库尚未公布 CVE 处理流程。
- 本文不声明项目已经通过渗透测试、加固认证或合规认证。

## English

### Supported Security Posture

This repository contains a permission analysis product with API, UI, report export, and Active Directory integration surfaces. The project does not yet define versioned support windows, so security review should target the current default branch and any active release branch used by maintainers.

### Reporting a Vulnerability

> **Release blocker:** `<SECURITY_CONTACT_EMAIL>` is a placeholder and cannot currently receive
> reports. The project owner must replace it with a real, working private security mailbox before
> public release. Do not send vulnerability reports to a GitHub noreply address.

If you find a security issue:

1. Do not open a public issue containing exploit details.
2. After the placeholder has been replaced, submit the report privately to
   `<SECURITY_CONTACT_EMAIL>`.
3. If GitHub Private Vulnerability Reporting has been enabled for the repository, you may instead use
   `Report a vulnerability` on the repository's `Security` page. The repository owner must enable
   this feature separately; this document does not claim that it is currently enabled.
4. Include the affected area, reproduction steps, potential impact, and the minimum proof of concept required to validate the issue.

Until the placeholder is replaced or GitHub Private Vulnerability Reporting is confirmed enabled, the
repository has no verified private reporting channel usable by external researchers and is not ready
for public release on the strength of this document.

### Areas Worth Reviewing

- AD credentials sent to API endpoints. The Web client no longer persists AD passwords to local storage; passwords must be re-entered after a browser refresh.
- Exported reports that may contain path or identity data.
- Audit request logs and local browser-operation logs.
- File-system access in the Windows scanner and desktop host.

### Current Limits

- The private security mailbox remains the inactive `<SECURITY_CONTACT_EMAIL>` placeholder.
- Enabling GitHub Private Vulnerability Reporting is recommended, but its status is not confirmed here.
- No published CVE process is declared in this repository.
- This document makes no claim about penetration testing, hardening certification, or compliance coverage.
