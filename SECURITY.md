# OpenAD 安全策略 / Security Policy

[中文](#中文) | [English](#english)

## 中文

### 当前支持范围

本仓库包含权限分析产品的 API、用户界面、报告导出和 Active Directory 集成。项目目前尚未定义按版本划分的安全支持周期，因此安全审查应以当前默认分支以及维护者正在使用的活动发布分支为准。

### 报告安全漏洞

发现安全问题时：

1. 不要在公开议题中披露利用细节。
2. 通过仓库所有者或内部维护者的私密渠道提交报告。
3. 报告中应包含受影响区域、复现步骤、潜在影响，以及验证问题所需的最小概念验证材料。

如果本仓库尚未公布私密安全联系人，请使用组织或项目所有者现有的内部软件安全报告渠道。

### 重点审查区域

- 发送到 API 端点的 AD 凭据。Web 客户端不再将 AD 密码持久化到本地存储；浏览器刷新后必须重新输入密码。
- 可能包含路径或身份数据的报告导出内容。
- 审计请求日志和本地浏览器操作日志。
- Windows 扫描器和桌面壳中的文件系统访问。

### 当前限制

- 本仓库尚未公布正式的公共安全邮箱。
- 本仓库尚未公布 CVE 处理流程。
- 本文不声明项目已经通过渗透测试、加固认证或合规认证。

## English

### Supported Security Posture

This repository contains a permission analysis product with API, UI, report export, and Active Directory integration surfaces. The project does not yet define versioned support windows, so security review should target the current default branch and any active release branch used by maintainers.

### Reporting a Vulnerability

If you find a security issue:

1. Do not open a public issue containing exploit details.
2. Submit a private report to the repository owners or internal maintainers.
3. Include the affected area, reproduction steps, potential impact, and the minimum proof of concept required to validate the issue.

If no private security contact is published, use the organization or project owner's established internal software-reporting channel.

### Areas Worth Reviewing

- AD credentials sent to API endpoints. The Web client no longer persists AD passwords to local storage; passwords must be re-entered after a browser refresh.
- Exported reports that may contain path or identity data.
- Audit request logs and local browser-operation logs.
- File-system access in the Windows scanner and desktop host.

### Current Limits

- No formal public security mailbox is declared in this repository.
- No published CVE process is declared in this repository.
- This document makes no claim about penetration testing, hardening certification, or compliance coverage.
