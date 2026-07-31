# ADR 0003：风险发现服务端持久化与扫描完成边界

[中文](#adr-0003风险发现服务端持久化与扫描完成边界) | [English](#adr-0003-server-side-risk-finding-persistence-and-scan-completion-boundary)

- 状态：已接受
- 日期：2026-07-31
- 负责人：OpenAD 项目维护者

## 背景

风险中心最初把完整风险发现集合保存在 WebView `localStorage` 中。每条发现包含路径、主体、
证据、控制映射和复核状态；大规模扫描生成数千条发现后，序列化结果会超过浏览器通常约
5–10 MiB 的单源存储配额。`setItem` 抛出的 `QuotaExceededError` 与扫描请求共用同一异常
边界，导致已经写入 SQLite 且状态为 `completed` 的扫描在界面上被改成 `failed`。

扫描历史是权限采集的权威记录，风险分析属于扫描完成后的派生处理。浏览器配额或风险服务
短暂不可用都不应改变已经完成的扫描事实。

## 决策

- 在 OpenAD 数据库中新增 `risk_findings` 表，由 Go `riskservice` 负责按 `fingerprint`
  合并发现、保存复核状态和备注、累计不同扫描会话中的出现次数。
- Web 暴露分析引擎继续从本次扫描响应生成风险发现，但通过 `/api/risk-findings` API 批量
  写入和读取，不再把完整集合写入 `localStorage`。
- `permissionProtector.riskFindings` 仅作为升级迁移源：首次读取风险中心或首次完成扫描时先
  幂等导入服务端，导入成功后删除本地键；导入失败则保留数据并允许下次重试。
- 扫描会话的成功或失败只由扫描请求和服务端会话结果决定。风险写入、监控目录元数据或本地
  操作日志属于非关键后处理，失败时记录独立警告，不得把 `completed` 改为 `failed`。
- 风险批量写入和迁移请求限制为 32 MiB，并继续经过现有网络准入、速率限制和请求审计。

## 影响

- 大规模风险集合不再受 WebView `localStorage` 小配额限制，桌面重启后仍由 SQLite 或配置的
  PostgreSQL 保存。
- 风险中心和总览改为异步读取；加载、错误和状态更新必须呈现明确界面状态。
- 升级不会丢弃旧的已接受、已解决状态、备注或 `seenCount`；重复迁移不会重复累计。
- 当前风险列表接口返回完整集合，以保持现有筛选和摘要语义。若数据规模继续增长，应另行增加
  服务端筛选、分页和摘要端点，而不是重新引入浏览器整包缓存。

## 备选方案

- 捕获 `QuotaExceededError` 后继续使用 `localStorage`：未采用，只能消除误报，仍会丢失新风险
  和复核状态，下一次大扫描还会再次达到上限。
- 改用 IndexedDB：未采用，虽然配额更高，但风险治理数据仍被绑定到单个 WebView 配置，无法
  与 OpenAD 已有数据库备份、历史会话和未来多客户端访问保持一致。
- 只保留最近一次扫描风险：未采用，会破坏跨会话的首次/最近出现时间、复核状态和累计次数。

## 验证

- Go 服务测试验证按指纹合并、按会话幂等、已接受状态保留、已解决状态在新扫描重新打开，以及
  旧数据重复迁移不重复计数。
- API 测试验证读取、批量写入、导入、状态更新、输入校验和数据库不可用响应。
- Web 测试模拟 `QuotaExceededError`，证明扫描仍保持 `completed`、完成摘要存在、错误栏为空且
  显示独立风险同步警告。
- Web 测试验证旧 `localStorage` 数据导入成功后才删除，并确认后续风险写入不调用 `setItem`。
- 完整 Go、Web typecheck、Web 测试、静态导出、桌面测试和 Windows 打包必须通过。

## ADR 0003: Server-side Risk Finding Persistence and Scan Completion Boundary

- Status: Accepted
- Date: 2026-07-31
- Owner: OpenAD maintainers

### Context

Exposure Center originally stored the complete risk-finding collection in WebView `localStorage`.
Each finding carries paths, trustees, evidence, control mappings, and review state. Large scans can
produce thousands of findings whose serialized form exceeds the browser origin's typical 5–10 MiB
quota. The resulting `QuotaExceededError` shared the scan request's exception boundary, so a scan
already persisted in SQLite as `completed` was changed to `failed` in the UI.

Scan history is the source of truth for permission collection, while risk analysis is derived
post-processing. Browser quota or temporary risk-service availability must not rewrite the fact that
a scan completed.

### Decision

- Add a `risk_findings` table to the OpenAD database. The Go `riskservice` merges by `fingerprint`,
  persists review state and notes, and counts observations across distinct scan sessions.
- Keep exposure generation in the Web client for the current scan response, but read and batch-write
  findings through `/api/risk-findings`; never store the complete collection in `localStorage` again.
- Treat `permissionProtector.riskFindings` only as an upgrade source. The first finding load or
  completed scan imports it idempotently, removes the key only after success, and retains it for retry
  after a failed import.
- Determine scan success only from the scan request and server session result. Risk persistence,
  watched-share metadata, and local operation logs are non-critical post-processing; failures produce
  independent warnings and never change `completed` to `failed`.
- Limit risk upsert and migration payloads to 32 MiB and retain existing network admission, rate
  limiting, and request auditing.

### Consequences

- Large finding sets are no longer constrained by the WebView `localStorage` quota and survive desktop
  restarts in SQLite or the configured PostgreSQL database.
- Exposure Center and Overview load asynchronously and must expose loading, error, and update states.
- Upgrade migration preserves accepted/resolved states, notes, and `seenCount`; retries are idempotent.
- The list endpoint currently returns the complete collection to preserve existing filtering and
  summary behavior. Continued growth should be addressed with server-side filters, pagination, and a
  summary endpoint rather than restoring a browser-wide cache.

### Alternatives

- Catch `QuotaExceededError` while retaining `localStorage`: rejected because it only removes the false
  failure while silently losing new findings and review state, and the next large scan hits the limit
  again.
- Use IndexedDB: rejected because the governance record would remain tied to one WebView profile and
  outside OpenAD's database backup, history, and future multi-client boundaries.
- Retain findings only for the latest scan: rejected because it breaks first/last-seen history, review
  state, and cumulative observations across sessions.

### Verification

- Go service tests cover fingerprint merging, per-session idempotency, accepted-state preservation,
  reopening resolved findings on a new scan, and idempotent legacy import.
- API tests cover listing, batch upsert, import, status updates, validation, and database-unavailable
  responses.
- A Web regression test injects `QuotaExceededError` and proves the scan remains `completed`, its
  completion summary remains visible, the error field stays empty, and a separate risk-sync warning is
  shown.
- Web tests prove legacy data is removed only after successful import and subsequent finding writes do
  not call `setItem`.
- The complete Go, Web typecheck/test/static-export, desktop-test, and Windows packaging matrix must pass.
