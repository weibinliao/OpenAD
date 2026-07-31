# AD 快照优先 SID 解析与组成员导出设计

状态：已由项目所有者确认需求与总体方向，等待书面设计复核。

日期：2026-07-31

## 1. 背景与问题

OpenAD 当前从 NTFS ACL 读取 `trustee_sid`，首先依赖 Windows `LookupAccount` 把 SID 转换为账户名称；UNC 扫描在 AD 连接可用时，还会调用实时 LDAP 展开组成员并补全用户属性。

现有流程存在三个相互关联的问题：

1. `NT AUTHORITY\*` 排除模式在短名称匹配时退化成 `*`，导致 SID、组和用户主体被全部过滤。
2. AD 展开返回零行后，Web 前端会重新发起一次不带 AD 解析的扫描。第一次扫描被保存为零权限会话，第二次扫描保存原始 SID，形成重复且误导的历史记录。
3. 历史会话没有绑定用于解析的 AD 快照。读取历史权限时只返回当时保存的字段，无法利用已经存在的 `ad_users`、`ad_groups` 和 `ad_memberships` 解释 SID 与相关组。

历史数据验证表明，部分早期扫描只通过实时 LDAP 补全了直接用户，组 SID 仍未识别；另一些扫描完全进入原始 ACL 回退。同一批 SID 在已完成的 AD 快照中可以稳定匹配。这说明 SID 解析不能继续依赖 Windows 运行账号和实时 LDAP 的偶然可用性。

## 2. 目标

- 修复排除模式，使 `DOMAIN\*` 只匹配该命名空间，不得退化为全局 `*`。
- 新扫描优先使用活动 AD 连接对应的最新已完成快照解析 SID。
- 快照中存在的用户和组必须被稳定识别，并补全可用的账户属性与组来源。
- 快照缺失项可以由实时 LDAP 补充，但实时查询失败不得丢弃原始 ACL。
- 一次用户操作只能产生一个扫描会话；解析失败时在同一会话中保留原始权限并记录状态。
- 历史旧会话在读取时使用最合适的已完成快照进行只读补全，不修改原始 ACL 记录。
- 在 AD 组详情中提供成员报表导出，默认 Excel，同时可选 CSV。
- 导出必须跟随“仅直属成员/包含嵌套成员”的当前选择。

## 3. 非目标

- 不在扫描前自动执行完整 AD 同步。
- 不修改 AD；所有 AD 操作继续保持只读。
- 不为快照中不存在、已删除、超出 Base DN 或属于特殊 Windows 命名空间的 SID 猜测身份。
- 不恢复旧网页后台壳，不把报表矩阵或大型 ACL 表移回扫描中心。
- 不改变侧边栏、窗口缩放或桌面运行时端口行为。

## 4. 方案比较与决策

### 方案 A：只修实时 LDAP

优点是改动最小。缺点是仍依赖网络、域控、凭据和 Windows 运行环境；历史会话也无法稳定恢复。该方案不能满足“之后的扫描借助 AD 快照都能解析”的要求。

### 方案 B：快照优先，实时 LDAP 补充

扫描绑定最新已完成快照，先在本地 SQLite 中解析 SID；只对快照缺失项调用实时 LDAP。历史会话使用绑定快照或推断快照补全。该方案解析稳定、速度可控、离线可用，并能保留真实未解析项。

### 方案 C：每次扫描前强制同步 AD

数据最接近实时状态，但会显著增加扫描启动时间和域控负载；同步失败还会阻断文件权限扫描。

### 决策

采用方案 B。AD 快照是主体解析的主数据源，实时 LDAP 是补充来源，不再是扫描结果完整性的唯一依赖。

## 5. 总体架构

处理链路调整为：

1. NTFS 扫描器读取原始 ACL、SID、权限和继承信息。
2. 扫描 API 根据活动连接选择最新已完成的 `DirectorySyncRun`。
3. 快照解析器批量加载 SID 对应的用户、组和扁平化成员关系。
4. 快照命中的直接用户被补全账户属性；快照命中的组按当前有效权限语义展开为用户行，并记录来源组与继承链。
5. 快照未命中的普通域 SID进入实时 LDAP 补充解析；Windows 特殊 SID直接进入明确的特殊主体分类。
6. 任一补充解析失败时保留原始 ACL 行，并记录未解析原因。
7. 同一会话保存最终权限数据、所用快照和解析摘要，不再由前端重新发起第二次扫描。

解析结果必须满足以下不变量：

- 输入 ACL 非空时，解析流程不得无解释地返回空结果。
- 未解析主体不得被删除。
- 原始 `trustee_sid` 必须保留。
- 每个扩展用户行必须包含可追溯的 `originating_group`；嵌套组必须包含 `group_inheritance_hierarchy`。
- 同一 ACL、用户、来源组和继承链组合不得重复输出。

## 6. 快照选择

### 新扫描

- 请求包含已保存的 `connection_id` 时，选择该连接最新的已完成 `DirectorySyncRun`。
- 只有内联凭据而没有 `connection_id` 时，先按服务器和 Base DN 匹配已保存连接；匹配唯一时使用其快照。
- 无法唯一匹配时不猜测连接，进入实时 LDAP 或原始 SID 模式，并在会话中记录原因。

### 历史会话

- 新会话直接使用已保存的 `directory_sync_run_id`。
- 旧会话没有绑定快照时，从扫描开始时间之前已完成的快照中计算 SID 覆盖率，选择命中不同 SID 数量最多的快照；相同覆盖率时选择时间最近者。
- 扫描前没有任何快照时，可以考虑扫描后的第一份快照，但响应必须标记为 `legacy_inferred_after_scan`。
- 历史推断只影响 API 响应，不回写原始权限记录或会话关联。

## 7. 数据模型

`ScanSession` 增加以下兼容字段：

- `DirectorySyncRunID *uuid.UUID`：新扫描使用的快照。
- `IdentityResolutionMode string`：`snapshot`、`snapshot+ldap`、`ldap`、`raw`、`raw-fallback`。
- `ResolvedPrincipalCount int`：已解析的不同 SID 数。
- `UnresolvedPrincipalCount int`：仍未解析的不同 SID 数。
- `IdentityResolutionWarning string`：非致命解析说明，不包含凭据或敏感信息。

权限记录继续保留现有 SID、账户属性、来源组和继承链字段。为便于审计，每行增加：

- `ResolutionSource string`：`windows`、`snapshot`、`ldap`、`well-known` 或 `unresolved`。
- `ResolutionReason string`：仅在未解析或降级时使用，例如 `not_in_snapshot`、`deleted_or_out_of_scope`、`ldap_unavailable`。

数据库迁移只增加可空或带默认值的列，不删除或重写现有数据。

## 8. 排除规则修复

命名空间模式必须保留完整限定语义：

- `NT AUTHORITY\*` 可以匹配 `NT AUTHORITY\SYSTEM`。
- 它不得匹配 SID、`DOMAIN\user`、普通组名或 `sAMAccountName`。
- 只有短名称模式本身不是单独的 `*` 时，才允许用域限定模式的短名称部分匹配 `sAMAccountName`。
- 默认内置主体排除规则由后端集中维护，Web 前端不再维护一份可能漂移的规则副本。

## 9. 扫描降级与错误处理

- 移除 Web 前端“零行后重新扫描”的逻辑。
- 后端解析器若收到非空输入却产生空输出，必须视为解析异常，并在同一会话中保存原始权限。
- 快照不可用不阻断 ACL 扫描；会话模式标记为 `ldap` 或 `raw`。
- LDAP 不可用不阻断已完成的 NTFS 扫描；已通过快照解析的行继续保留，未命中项保留 SID。
- UI 显示解析来源、已解析数量、未解析数量和非致命警告，不把原始 SID 模式描述成完全解析成功。

## 10. 历史会话补全

`GET /api/sessions/:id/bundle` 返回：

- 原有 `session` 和 `permissions`。
- 新增 `identity_resolution` 元数据，包括快照 ID、选择模式、解析与未解析数量、是否为历史推断。

历史服务在内存中补全返回行：

- 用户 SID：补全显示名、账户名、邮件、部门、域等已有快照字段。
- 组 SID：补全组名和账户类型；需要有效用户视图时使用快照成员关系展开并设置来源组。
- 特殊 Windows SID：使用受控的友好名称映射。
- 未命中 SID：原样返回并附带原因。

原始数据库权限行不在读取时被修改，确保历史证据仍可追溯。

## 11. AD 组成员报表导出

新增 `POST /api/ad/groups/members/export`。请求包含：

- AD 连接信息或 `connection_id`。
- `group_dn`。
- `include_nested` 与可选 `max_depth`。
- `format`：`excel` 或 `csv`。
- `locale` 与可选文件名。

后端重新查询该组，避免信任或回传前端截断列表。导出逻辑复用现有 `excelize`、CSV、文件名清理、流式下载和请求大小限制能力。

导出字段：

- 组名称与组 DN。
- 成员类型。
- 显示名称。
- `sAMAccountName`。
- 邮箱。
- 部门。
- Division。
- 域。
- SID。
- 成员 DN。
- 直属/嵌套。
- 深度。
- 成员关系路径。

Excel 为默认格式，工作表名称使用本地化的“组成员/Members”；CSV 使用 UTF-8 BOM，确保 Windows Excel 正确显示中文。

## 12. 前端交互

沿用 `DirectoryExplorerWorkbench` 当前组详情样式，不建立新的视觉系统。

- 在“组成员”标题行增加下载按钮和格式菜单。
- 主操作默认导出 Excel；菜单提供 CSV。
- 导出跟随当前“仅直属成员/包含嵌套成员”状态。
- 加载成员时禁用导出。
- 空组允许导出带元数据和表头的空报表。
- 导出中显示加载状态；失败在组详情内显示可重试错误。
- 使用项目已启用的 Lucide `Download` 图标和现有 Button/Menu 控件。

扫描中心和报告中心显示会话解析摘要，但不增加新的大型说明区或 ACL 表。

## 13. 安全与性能

- AD 操作继续只读。
- API 和日志不得返回或记录密码、加密密码或绑定凭据。
- SID 查询按批次执行，避免每行独立访问 SQLite。
- 实时 LDAP 只处理快照未命中的不同 SID，并使用请求级缓存。
- 组成员 Excel/CSV 直接流式返回，不在应用数据目录留下临时报表。
- 历史推断快照按会话缓存，避免重复计算覆盖率。

## 14. 测试策略

后端测试：

- 排除规则不得把 `NT AUTHORITY\*` 退化为全局通配。
- 快照解析直接用户、直接组、嵌套组、重复成员和未解析 SID。
- 新扫描选择活动连接的最新已完成快照。
- 非空 ACL 在解析异常时保留原始行，并只创建一个会话。
- 旧历史会话按 SID 覆盖率和时间选择快照。
- 历史补全不修改数据库原始行。
- 组成员 CSV/Excel 的字段、中文、直属/嵌套范围与空报表。
- 导出文件名、请求限制和凭据错误。

Web 测试：

- 组详情默认 Excel 导出并可选择 CSV。
- 导出请求跟随直属/嵌套状态。
- 加载、空、错误和下载状态。
- 扫描不再因零行重新发送第二个请求。
- 历史与报告中心显示快照解析元数据和相关组。

交付验证：

- 完整 Go 测试。
- Web typecheck、完整测试与静态导出。
- 构建完整 Windows 桌面包。
- 启动桌面包，验证端口、`/health`、现有 AD 连接、历史会话解析以及组成员 Excel/CSV 下载。
- 使用至少一个包含直接用户、AD 组、嵌套组、Windows 特殊 SID 和不存在 SID 的测试共享验证结果。

## 15. 文档与兼容性

- 用户可见行为更新写入 `CHANGELOG.md`。
- 如果开发命令不变，不修改 `DEVELOPMENT.md`。
- 数据库只做向前兼容的增量迁移。
- 本设计不修改许可边界，不需要许可文件变更。

---

# AD Snapshot-First SID Resolution and Group Member Export Design

Status: requirements and overall direction approved by the project owner; awaiting written-spec review.

Date: 2026-07-31

## 1. Background and Problem

OpenAD reads `trustee_sid` from NTFS ACLs and first relies on Windows `LookupAccount` to translate a SID into an account name. For UNC scans with an available AD connection, it also uses live LDAP to expand groups and enrich user attributes.

The current flow has three connected defects:

1. The `NT AUTHORITY\*` exclusion pattern degrades to `*` during short-name matching, filtering every SID, group, and user principal.
2. When AD expansion returns zero rows, the Web client starts a second scan without AD resolution. The first scan is saved as a zero-permission session and the second saves raw SIDs, producing duplicate and misleading history.
3. Historical sessions are not bound to the AD snapshot used for resolution. History APIs return only stored fields and do not use existing `ad_users`, `ad_groups`, and `ad_memberships` records to explain SIDs and group sources.

Historical evidence shows that some earlier scans enriched only direct users through live LDAP while group SIDs remained unresolved, and other scans fully fell back to raw ACLs. The same SIDs are consistently available in completed AD snapshots. SID resolution therefore cannot depend on the accidental availability of the Windows runtime identity or live LDAP.

## 2. Goals

- Fix exclusion matching so `DOMAIN\*` remains scoped to that namespace and never becomes a global `*`.
- Resolve new scans from the latest completed snapshot associated with the active AD connection.
- Reliably identify every user or group present in the selected snapshot and enrich available identity and group-source fields.
- Use live LDAP only to supplement snapshot misses, without dropping raw ACLs when live queries fail.
- Produce exactly one scan session per user action; preserve raw permissions and resolution status in that session when enrichment fails.
- Read-time enrich legacy sessions with the best completed snapshot without mutating original ACL rows.
- Export AD group members from group details, defaulting to Excel with CSV available.
- Make export scope follow the current direct-only or nested-members selection.

## 3. Non-Goals

- Do not run a full AD synchronization before every scan.
- Do not write to AD; all directory operations remain read-only.
- Do not guess identities for deleted, out-of-scope, special Windows, or otherwise absent SIDs.
- Do not restore the legacy admin shell or move report matrices and large ACL tables into Scan Center.
- Do not change sidebar, desktop resize, or runtime port behavior.

## 4. Alternatives and Decision

### Option A: Fix Live LDAP Only

This is the smallest change, but it remains dependent on network state, domain controllers, credentials, and the runtime identity. It also cannot reliably repair history.

### Option B: Snapshot First, Live LDAP as Supplement

Bind scans to a completed snapshot, resolve SIDs locally first, query LDAP only for snapshot misses, and enrich history from bound or inferred snapshots. This is stable, fast, offline-capable, and preserves genuinely unresolved identities.

### Option C: Force a Full Sync Before Every Scan

This maximizes freshness but adds significant startup latency and domain-controller load. A failed sync would also block otherwise valid file-permission scans.

### Decision

Adopt Option B. AD snapshots become the primary identity source. Live LDAP is supplementary and is no longer the sole dependency for scan-result completeness.

## 5. High-Level Architecture

The revised pipeline is:

1. Read raw ACLs, SIDs, rights, and inheritance from NTFS.
2. Select the latest completed `DirectorySyncRun` for the active connection.
3. Batch-load matching users, groups, and flattened memberships from SQLite.
4. Enrich direct users and expand snapshot groups into user rows with group source and inheritance path.
5. Query live LDAP only for ordinary domain SIDs missing from the snapshot; classify special Windows SIDs locally.
6. Preserve raw ACL rows with an explicit reason whenever supplemental resolution fails.
7. Save final permissions, the selected snapshot, and a resolution summary in one session. The Web client never starts a second fallback scan.

Resolution invariants:

- Non-empty ACL input must never become an unexplained empty output.
- Unresolved principals must never be dropped.
- Original `trustee_sid` values must remain available.
- Every expanded user row must identify its `originating_group`; nested memberships must include `group_inheritance_hierarchy`.
- Duplicate ACL/user/source-group/inheritance-path combinations must be removed.

## 6. Snapshot Selection

### New Scans

- With a stored `connection_id`, select that connection's latest completed `DirectorySyncRun`.
- With inline credentials only, match saved connections by server and Base DN and use a snapshot only when the match is unique.
- When connection matching is ambiguous, do not guess. Use live LDAP or raw-SID mode and record the reason.

### Historical Sessions

- New sessions use their stored `directory_sync_run_id`.
- For legacy sessions, calculate SID coverage across completed snapshots preceding the scan. Select the snapshot matching the most distinct SIDs, breaking ties by closest time.
- When no snapshot predates the scan, the first later snapshot may be used, but the response must be marked `legacy_inferred_after_scan`.
- Legacy inference affects only API responses and never rewrites stored ACLs or session links.

## 7. Data Model

Add compatible fields to `ScanSession`:

- `DirectorySyncRunID *uuid.UUID`.
- `IdentityResolutionMode string`: `snapshot`, `snapshot+ldap`, `ldap`, `raw`, or `raw-fallback`.
- `ResolvedPrincipalCount int`.
- `UnresolvedPrincipalCount int`.
- `IdentityResolutionWarning string`, excluding credentials and sensitive values.

Keep existing SID, account, group-source, and inheritance fields on permissions, and add:

- `ResolutionSource string`: `windows`, `snapshot`, `ldap`, `well-known`, or `unresolved`.
- `ResolutionReason string`: used only for unresolved or degraded results, such as `not_in_snapshot`, `deleted_or_out_of_scope`, or `ldap_unavailable`.

Migrations add nullable or defaulted columns only. Existing data is never deleted or rewritten.

## 8. Exclusion Rule Fix

Namespace patterns retain their qualified meaning:

- `NT AUTHORITY\*` matches `NT AUTHORITY\SYSTEM`.
- It does not match a SID, `DOMAIN\user`, a normal group name, or a `sAMAccountName`.
- Domain-qualified short-name matching is allowed only when the short pattern is not a bare `*`.
- Default built-in exclusions are centralized in the backend; the Web client no longer maintains a drifting copy.

## 9. Fallback and Error Handling

- Remove the Web client's zero-row rescan.
- If a resolver receives non-empty input and produces empty output, treat it as a resolution failure and persist the original permissions in the same session.
- Missing snapshots do not block ACL scans; the session uses `ldap` or `raw` mode.
- LDAP failure does not discard snapshot-resolved rows. Snapshot misses remain as SIDs.
- The UI reports source, resolved count, unresolved count, and non-fatal warnings. Raw-SID mode is never presented as full resolution success.

## 10. Historical Enrichment

`GET /api/sessions/:id/bundle` keeps `session` and `permissions` and adds `identity_resolution` metadata containing snapshot ID, selection mode, resolved and unresolved counts, and whether the snapshot was inferred.

The history service enriches response rows in memory:

- User SIDs receive snapshot display name, account, email, department, and domain data.
- Group SIDs receive group name and type; effective-user views expand memberships and set group source.
- Special Windows SIDs use controlled friendly-name mappings.
- Unmatched SIDs remain unchanged and include a reason.

Stored permission rows are not modified during reads.

## 11. AD Group Member Export

Add `POST /api/ad/groups/members/export` with connection data or `connection_id`, `group_dn`, `include_nested`, optional `max_depth`, `format`, `locale`, and an optional filename.

The backend re-queries the group so it does not trust a truncated client list. Export reuses existing `excelize`, CSV, filename sanitization, streaming-download, and request-size-limit infrastructure.

Export columns include group name and DN, member type, display name, `sAMAccountName`, email, department, division, domain, SID, member DN, direct/nested status, depth, and membership path.

Excel is the default and uses a localized Members sheet name. CSV includes a UTF-8 BOM for reliable Chinese display in Windows Excel.

## 12. Frontend Interaction

Extend the current `DirectoryExplorerWorkbench` group inspector without introducing a new visual system.

- Add a download action and format menu to the Group Members heading row.
- Primary action exports Excel; the menu offers CSV.
- Export follows the current direct-only or nested-members mode.
- Disable export while members are loading.
- Empty groups export a valid report containing metadata and headers.
- Show in-place progress, retryable errors, and download state.
- Use the existing Lucide `Download` icon and current Button/Menu controls.

Scan Center and Report Center may show compact resolution summaries but do not gain new large explanatory panels or ACL tables.

## 13. Security and Performance

- AD remains read-only.
- APIs and logs never expose passwords, encrypted passwords, or bind credentials.
- Snapshot SID lookups are batched rather than queried per permission row.
- Live LDAP processes only distinct snapshot misses and uses request-scoped caching.
- Group exports stream directly to the response and leave no report files in application data.
- Legacy snapshot inference is cached per session.

## 14. Testing Strategy

Backend coverage includes exclusion scoping, direct users, direct and nested groups, duplicate membership, unresolved SIDs, connection-specific snapshot selection, one-session fallback, legacy snapshot inference, non-mutating history enrichment, CSV/Excel scope and encoding, empty exports, filenames, request limits, and credential errors.

Web coverage includes default Excel and optional CSV, direct/nested request scope, loading/empty/error/download states, removal of the second scan request, and snapshot/group metadata in History and Report Center.

Delivery verification runs the full Go suite, Web typecheck/tests/static export, full Windows desktop packaging, and live desktop checks for ports, `/health`, the existing AD connection, historical resolution, and Excel/CSV downloads. A representative test share must contain direct users, AD groups, nested groups, special Windows SIDs, and at least one nonexistent SID.

## 15. Documentation and Compatibility

- Record user-visible behavior in `CHANGELOG.md`.
- Do not change `DEVELOPMENT.md` unless commands or prerequisites change.
- Use forward-compatible additive database migration only.
- This design does not change licensing boundaries or license files.
