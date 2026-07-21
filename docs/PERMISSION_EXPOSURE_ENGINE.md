# 权限暴露引擎 / Permission Exposure Engine

## 中文

v0.2 权限暴露引擎把原始 ACL 行转换为可执行的风险发现。设计参考了同类产品中有效的分析模式，同时严格限制在当前 OpenAD 产品范围内。

### OpenAD 对行业模式的转化

| 行业模式 | OpenAD 中的实现方式 |
| --- | --- |
| 类 Varonis 的爆炸半径收敛 | 识别具有写入能力的宽泛主体，并将其优先级置于普通 ACL 噪声之上。 |
| 类 Netwrix 的过度暴露数据审查 | 识别 `Everyone`、`Authenticated Users`、`Domain Users` 等宽泛访问路径。 |
| 类 ManageEngine 的审计准备 | 为每条发现保留证据、状态、首次/最后出现时间和重复出现次数。 |
| 类 PingCastle 的管理语言 | 为非工程角色提供暴露评分、业务问题、类别和整改成本。 |
| 数据访问治理与分类 | 当宽泛或高影响权限触达人事、财务、薪资、法务、备份、源码、凭据、客户或机密区域时，使用路径名称启发式提高优先级。 |

### 当前规则族

| 规则族 | 示例规则 ID |
| --- | --- |
| 过度暴露 | `blast-radius-broad-write`、`broad-read-surface`、`inherited-high-risk-spread` |
| 敏感数据 | `sensitive-path-broad-access`、`sensitive-path-high-risk-access` |
| 特权 | `ownership-grade-permission`、`full-control-entitlement`、`privileged-group-on-data` |
| 权限卫生 | `orphaned-identity-on-acl` |
| 治理 | `explicit-dangerous-ace`、`nested-group-high-risk-access` |
| 运维阻力 | `explicit-deny-friction` |

### 风险发现字段

每条发现可以包含严重级别、1-100 优先级评分、类别、置信度、整改成本、ACL 证据、敏感业务标签、业务核查问题和控制映射。证据在可用时包括路径、主体、权限、继承来源和组关系链。

### 当前实现边界

第一版规则在前端纯函数中运行，并在导入或完成扫描结果时执行，以保持现有扫描 API 稳定并便于快速迭代。敏感路径检测只使用启发式路径名称，不检查文件内容，因此删除权限前必须由数据所有者确认业务语义。后续阶段应把同一规则集迁移到后端，以便定时扫描、导出和告警复用分析结果。

### 后续规则候选

- 使用文件元数据或 DLP 标签进行内容分类，而不是只依赖路径名称。
- 同一高风险主体重复出现在多个路径。
- 显式权限偏离父级策略。
- 已禁用 AD 账号仍存在于 ACL。
- 组嵌套深度、循环关系和不可解释关系链。
- 基于扫描会话差异生成风险发现。

## English

The v0.2 exposure engine turns raw ACL rows into action-oriented findings. It is intentionally modeled after useful market patterns while staying practical for the current product scope.

## Product References Translated into OpenAD

| Market pattern | OpenAD interpretation |
| --- | --- |
| Varonis-style blast radius reduction | Identify broad principals with write-capable access and prioritize them above ordinary ACL noise. |
| Netwrix-style overexposed data review | Surface `Everyone`, `Authenticated Users`, `Domain Users`, and similar broad access paths. |
| ManageEngine-style audit readiness | Preserve evidence, status, first/last seen, and repeated occurrence counts for each finding. |
| PingCastle-style management language | Provide an exposure score, business question, category, and remediation effort for non-engineering stakeholders. |
| Data access governance and classification | Use path-name heuristics to raise priority when broad or high-impact permissions touch HR, finance, payroll, legal, backup, source-code, credential, customer, or confidential areas. |

## Current rule families

| Rule family | Example rule IDs |
| --- | --- |
| Overexposure | `blast-radius-broad-write`, `broad-read-surface`, `inherited-high-risk-spread` |
| Sensitive data | `sensitive-path-broad-access`, `sensitive-path-high-risk-access` |
| Privilege | `ownership-grade-permission`, `full-control-entitlement`, `privileged-group-on-data` |
| Hygiene | `orphaned-identity-on-acl` |
| Governance | `explicit-dangerous-ace`, `nested-group-high-risk-access` |
| Operational friction | `explicit-deny-friction` |

## Finding fields

Each finding can include:

- Severity: `critical`, `high`, `medium`, or `low`.
- Priority score: `1-100`, used for sorting and the exposure score.
- Category: overexposure, privilege, hygiene, governance, or operational friction.
- Confidence: high, medium, or low.
- Remediation effort: quick win, owner review, or planned change.
- Evidence: path, trustee, rights, inheritance, source, and group-chain context when available.
- Sensitive labels: path-derived business context such as HR, Finance, Payroll, Legal, Backups, Source Code, Secrets, Customer/PII, IT/Admin, or Confidential.
- Business question: the question an IT owner or data owner should answer.
- Control mapping: least privilege, data access governance, audit readiness, etc.

## Current implementation boundary

The first implementation is frontend-pure and runs when scan results are imported or completed. This keeps the current scan API stable and allows fast iteration. Sensitive path detection is a heuristic, not content inspection, so the UI asks the data owner to validate the business context before removal. A later phase should move the same rule set into the backend so scheduled scans, exports, and alerts can reuse the same analysis.

## Next rule candidates

- Content classification from file metadata or DLP labels instead of path-name heuristics only.
- Repeated risky trustee across many paths.
- Explicit permissions that break from parent policy.
- Disabled AD account still present on ACL.
- Group nesting depth and circular/opaque group chains.
- Delta-based findings from scan-to-scan comparisons.
