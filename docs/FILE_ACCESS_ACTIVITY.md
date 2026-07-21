# 文件访问活动审计 / File Access Activity Audit

## 中文

OpenAD 将权限暴露和访问活动明确区分：

- 权限暴露回答谁根据 ACL 可能访问或修改文件夹。
- 文件活动回答谁根据 Windows 审计事件实际访问了文件或共享。
- 活动模块不会打开、解析、哈希、分类或扫描文件内容。

### 产品定位

| 行业模式 | OpenAD 中的实现方式 |
| --- | --- |
| Netwrix 和 ManageEngine 文件服务器审计 | 查询文件/共享访问事件，并显示用户、路径、动作、时间、进程/客户端和事件 ID。 |
| Varonis 访问行为上下文 | 在不检查内容的前提下，把权限暴露发现与真实访问行为关联。 |
| 内部部署信任边界 | 在文件服务器本机或转发后的 Windows 安全事件中完成采集。 |

### Windows 事件来源

第一版通过 `wevtutil` 读取本机 Windows 安全事件日志，并解析以下事件元数据：

| 事件 ID | 含义 |
| --- | --- |
| `4656` | 请求对象句柄。 |
| `4663` | 尝试访问对象。 |
| `4660` | 删除对象。 |
| `4670` | 修改对象权限。 |
| `5140` | 访问网络共享对象。 |
| `5145` | 对网络共享对象执行详细访问检查。 |

界面位于 `/file-activity`。事件查询和只读就绪检查分别使用：

```text
GET /api/file-activity/events?hours=24&limit=100&path=Finance&user=alice&action=write
GET /api/file-activity/readiness?path=C:\Shares\Finance
```

就绪检查会诊断 Windows 主机、安全日志读取权限、对象访问审计策略和可选目标目录 SACL，并返回可复制到管理员终端的命令。它不会修改系统策略或目录 ACL。需要 AD 身份解析且不能把凭据放入 URL 时，Web 界面使用 `POST /api/file-activity/events/query`。支持的动作筛选为 `read`、`write`、`delete`、`permission-change`、`share-access` 和 `other`。

### AD 身份解析

Windows 事件可能同时包含账号名和 SID。文件活动页面复用当前 AD 连接模型，优先解析 `SubjectUserSid`，再回退到 `DOMAIN\user` 等事件账号名。解析成功时显示 `DOMAIN\samAccountName (Display Name)`，同时保留原始 SID 作为证据。浏览器刷新后不会保留 AD 密码；如需 SID 到名称解析，应先重新测试 AD 连接。

### Windows 前置配置

- 在文件服务器上启用对象访问高级审计策略。
- 为需要产生事件的目录或共享配置 SACL 审计项。
- 在文件服务器上运行 OpenAD，或把 Windows 安全事件转发到 API 所在主机。
- 让 API 进程具有读取安全日志的权限，例如管理员或 Event Log Readers 成员。
- 如需把 SID 显示为域用户和组，在当前 Web 会话中连接 AD。

建议先在 `/file-activity` 中执行无路径就绪检查，再针对一个试点目录检查 SACL；必要时把生成的审计策略和 SACL 命令复制到文件服务器的管理员终端，产生一次读写事件后刷新活动台账。

### 边界

该模块不是内容扫描或 DLP，不能判断文件是否包含 PII、源码、合同或机密，只报告 Windows 已经记录的事件元数据。后续可以增加远程事件转发、持久化和报告导出，但当前版本明确不检查文件内容。

## English

OpenAD separates permission exposure from access activity:

- Permission exposure answers: who could access or change a folder based on ACLs.
- File activity answers: who actually accessed a file or share based on Windows audit events.
- The activity module does not open, parse, hash, classify, or scan file contents.

## Product positioning

| Market pattern | OpenAD interpretation |
| --- | --- |
| Netwrix and ManageEngine file-server auditing | Query file/share access events and show user, path, action, time, process/client, and event ID. |
| Varonis access behavior context | Pair permission exposure findings with actual access behavior without content inspection. |
| Internal deployment trust boundary | Keep collection local to the file server or to forwarded Windows Security events. |

## Windows event source

The first implementation reads the local Windows Security event log through `wevtutil` and parses event metadata for these event IDs:

| Event ID | Meaning |
| --- | --- |
| `4656` | A handle to an object was requested. |
| `4663` | An attempt was made to access an object. |
| `4660` | An object was deleted. |
| `4670` | Permissions on an object were changed. |
| `5140` | A network share object was accessed. |
| `5145` | A network share object was checked for detailed access. |

The UI is available at `/file-activity` and calls:

```text
GET /api/file-activity/events?hours=24&limit=100&path=Finance&user=alice&action=write
```

The readiness wizard calls a read-only diagnostic endpoint:

```text
GET /api/file-activity/readiness?path=C:\Shares\Finance
```

It checks the Windows host, Security log read permission, Object Access audit policy, and optional target-folder SACL presence. It also returns administrator commands that can be copied into an elevated prompt. The endpoint does not change system policy or folder ACLs.

The web UI uses the POST query endpoint when it needs AD identity resolution without placing credentials in the URL:

```text
POST /api/file-activity/events/query
```

Supported action filters are `read`, `write`, `delete`, `permission-change`, `share-access`, and `other`.

## AD identity resolution

Windows events may contain both account names and SIDs. OpenAD should not leave users staring at raw SIDs when AD is available.

The File Activity page therefore reuses the AD Workspace connection model:

- First connect and test AD in `/ad-workspace`.
- Keep the current browser session active so the AD password remains in memory.
- Open `/file-activity` and refresh the ledger.
- The backend resolves `SubjectUserSid` first, then falls back to event-log account names such as `DOMAIN\user`.
- Resolved rows display as `DOMAIN\samAccountName (Display Name)` while retaining the original SID as evidence.

If the browser was refreshed, the stored AD connection intentionally does not retain the password. Re-test the AD connection before refreshing File Activity if SID-to-name resolution is needed.

## Required Windows configuration

File activity will only appear when Windows is already generating the right audit events:

- Enable Advanced Audit Policy for Object Access on the file server.
- Add SACL auditing entries on the folders or shares that should emit file access events.
- Run OpenAD on the file server, or forward Windows Security events to the host running the API.
- Run the API process with permission to read the Security log, such as Administrator or Event Log Readers membership.
- Connect AD in the current web session if event SIDs should be displayed as domain users and groups.

The recommended small-business trial flow is:

- Open `/file-activity`.
- Run the readiness check without a path to verify host and Security log prerequisites.
- Enter one pilot folder path and run readiness again to check SACL presence.
- Copy the generated audit-policy and pilot SACL commands into an elevated prompt on the file server if needed.
- Generate one read/write event, then refresh the access ledger.

## Boundary

This is not content scanning and not DLP. It does not tell whether a file contains PII, source code, contracts, or secrets. It only reports event metadata that Windows has already recorded.

Future extensions can add remote event forwarding, persistence, and report export, but the current version intentionally avoids file-content inspection.
