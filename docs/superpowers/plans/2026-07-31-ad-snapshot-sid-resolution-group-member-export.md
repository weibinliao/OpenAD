# AD 快照 SID 解析与组成员导出实施计划

> **供代理执行者使用：** 必须使用 `superpowers:executing-plans` 逐项执行；所有生产代码之前先写失败测试，并使用复选框跟踪进度。

**目标：** 让新扫描和历史扫描优先使用绑定的 AD 快照解析 SID，保证解析失败时同一会话保留原始 ACL，并在目录浏览中提供默认 Excel、可选 CSV 的组成员导出。

**架构：** 新增独立的 `identityresolution` 包，批量读取 `directory_sync_runs`、`ad_users`、`ad_groups` 和 `ad_memberships`，再以实时 LDAP 只补充快照未命中的 SID。扫描服务负责单会话降级和解析元数据持久化；历史服务只在响应内补全旧记录。组成员导出使用独立的纯 Go 导出器，由 AD 处理器重新查询组后流式返回。

**技术栈：** Go 1.23、Gin、GORM、SQLite、excelize、Next.js 14、React 18、TypeScript、Jest、Testing Library。

---

## 文件结构

- 新建 `apps/backend/internal/identityresolution/service.go`：快照选择、SID 批量解析、组展开、LDAP 补充和历史推断。
- 新建 `apps/backend/internal/identityresolution/service_test.go`：快照命中、组成员、嵌套链、未解析原因和历史快照选择测试。
- 新建 `apps/backend/internal/groupexport/exporter.go`：组成员行模型、CSV/XLSX 输出和本地化列名。
- 新建 `apps/backend/internal/groupexport/exporter_test.go`：BOM、字段、空组、直属/嵌套和 Excel 内容测试。
- 修改 `apps/backend/internal/models/models.go`、`apps/backend/internal/scanner/ntfs.go`：解析字段和会话元数据。
- 修改 `apps/backend/internal/scanservice/service.go`：解析失败时同一会话原始 ACL 降级。
- 修改 `apps/backend/cmd/api/handlers_scan.go`、`app.go`、`handlers_ad.go`、`routes.go`：快照优先扫描工厂和组成员下载 API。
- 修改 `apps/backend/internal/historyservice/service.go`：历史 bundle 只读补全和解析元数据。
- 修改 `apps/web/pages/scan-workspace.tsx`：删除二次扫描和前端排除规则副本，展示解析摘要。
- 修改 `apps/web/components/DirectoryExplorerWorkbench.tsx`：Excel 主操作、CSV 菜单、下载状态。
- 修改 `apps/web/components/ScanCompletionSummary.tsx`、`ReportCenterWorkspace.tsx`：紧凑显示解析来源和未解析数量。
- 修改对应 Go/Jest 测试和 `CHANGELOG.md`。

### 任务 1：修复命名空间排除规则

**文件：**
- 修改：`apps/backend/internal/ad/exclusion_filter.go`
- 测试：`apps/backend/internal/ad/exclusion_filter_test.go`

- [ ] **步骤 1：写失败测试**

```go
func TestExclusionFilterDoesNotTurnQualifiedWildcardIntoGlobalWildcard(t *testing.T) {
	filter := NewExclusionFilter()
	filter.AddGroupPattern(`NT AUTHORITY\*`)

	assert.True(t, filter.ShouldExclude(`NT AUTHORITY\SYSTEM`))
	assert.False(t, filter.ShouldExclude(`S-1-5-21-1-2-3-1001`))
	assert.False(t, filter.ShouldExclude(`CORP\Finance`))
	assert.False(t, filter.ShouldExclude(`alice`))
}
```

- [ ] **步骤 2：运行测试并确认因裸 `*` 误匹配失败**

运行：`& .\tools\go\bin\go.exe -C .\apps\backend test ./internal/ad -run QualifiedWildcard -count=1`

- [ ] **步骤 3：仅在短模式不是裸 `*` 时执行短名称匹配**

```go
shortPattern := strings.TrimSpace(normalizedPattern[index+1:])
if shortPattern == "" || shortPattern == "*" {
	continue
}
```

- [ ] **步骤 4：运行 `internal/ad` 测试并提交**

### 任务 2：建立快照优先解析器和数据字段

**文件：**
- 新建：`apps/backend/internal/identityresolution/service.go`
- 新建：`apps/backend/internal/identityresolution/service_test.go`
- 修改：`apps/backend/internal/models/models.go`
- 修改：`apps/backend/internal/scanner/ntfs.go`
- 修改：`apps/backend/internal/database/db_test.go`

- [ ] **步骤 1：写失败测试，覆盖直接用户、直接组、嵌套组、Windows SID 和缺失 SID**

```go
result, err := resolver.Expand(context.Background(), run.ID, []scanner.Permission{
	{TrusteeSID: userSID, Trustee: userSID},
	{TrusteeSID: groupSID, Trustee: groupSID},
	{TrusteeSID: "S-1-1-0", Trustee: "S-1-1-0"},
	{TrusteeSID: missingSID, Trustee: missingSID},
})
require.NoError(t, err)
assert.Equal(t, "snapshot", result.Permissions[0].ResolutionSource)
assert.Contains(t, collectOriginatingGroups(result.Permissions), "Finance")
assert.Contains(t, collectTrustees(result.Permissions), "Everyone")
assert.Contains(t, collectReasons(result.Permissions), "not_in_snapshot")
```

- [ ] **步骤 2：运行测试并确认包或 API 尚不存在**

- [ ] **步骤 3：增加兼容字段**

```go
DirectorySyncRunID       *uuid.UUID `gorm:"type:uuid;index" json:"directory_sync_run_id,omitempty"`
IdentityResolutionMode   string     `json:"identity_resolution_mode,omitempty"`
ResolvedPrincipalCount   int        `json:"resolved_principal_count"`
UnresolvedPrincipalCount int        `json:"unresolved_principal_count"`
IdentityResolutionWarning string    `json:"identity_resolution_warning,omitempty"`
```

权限结构同时增加 `ResolutionSource` 和 `ResolutionReason`，并保持 `trustee_sid` 原值可追踪。

- [ ] **步骤 4：实现批量快照解析**

解析器必须：按连接选择最新 completed run；一次性加载输入 SID 对应的用户、组和组成员；组 ACE 展开成快照用户；设置 `originating_group` 和 `group_inheritance_hierarchy`；保留 well-known 和未命中 SID；按路径、SID、来源组和继承链去重。

- [ ] **步骤 5：实现实时 LDAP 仅补充快照 miss**

实时扩展器仅收到未命中的不同 SID 对应 ACL。LDAP 错误或空输出改为带 `ldap_unavailable`/`ldap_empty_result` 原因的原始行，不删除快照已解析结果。

- [ ] **步骤 6：运行新包和模型测试并提交**

### 任务 3：单会话扫描降级与快照绑定

**文件：**
- 修改：`apps/backend/internal/scanservice/service.go`
- 修改：`apps/backend/internal/scanservice/service_test.go`
- 修改：`apps/backend/cmd/api/handlers_scan.go`
- 修改：`apps/backend/cmd/api/main_test.go`

- [ ] **步骤 1：写失败测试，解析器错误和空结果都保留原始 ACL**

```go
response, err := service.Run(Request{EffectivePermissionExpander: failingExpander})
require.NoError(t, err)
require.Len(t, response.Permissions, 1)
assert.Equal(t, originalSID, response.Permissions[0].TrusteeSID)
assert.Equal(t, "raw-fallback", response.IdentityResolution.Mode)
assert.Equal(t, 1, repository.completeCalls)
assert.Equal(t, 0, repository.failedCalls)
```

- [ ] **步骤 2：确认测试按旧行为失败**

- [ ] **步骤 3：增加 `IdentityResolutionSummary` 并让扫描服务拥有降级规则**

工厂失败、Expand 非取消错误、非空输入得到空输出都继续完成原会话。仅 `context.Canceled` 取消会话。响应和数据库同时保存 run ID、模式、已解析/未解析计数和非敏感警告。

- [ ] **步骤 4：处理器构造快照优先扩展器**

stored `connection_id` 直接绑定对应最新快照；inline credentials 仅在 server/Base DN 唯一匹配连接时绑定快照。凭据或 LDAP 不可用只影响补充，不阻止快照/原始 ACL 扫描。

- [ ] **步骤 5：运行 scanservice 和 API 定向测试并提交**

### 任务 4：历史会话只读补全

**文件：**
- 修改：`apps/backend/internal/historyservice/service.go`
- 修改：`apps/backend/internal/historyservice/service_test.go`
- 修改：`apps/backend/cmd/api/main_test.go`

- [ ] **步骤 1：写失败测试**

测试新会话使用绑定 run；旧会话按扫描前 SID 覆盖率最大、时间最近选择快照；无前置快照时选择第一份后置快照并标记 `legacy_inferred_after_scan`；数据库原始 Permission 不变化。

- [ ] **步骤 2：实现 `identity_resolution` 响应元数据**

```go
type SessionBundleResponse struct {
	Session            models.ScanSession              `json:"session"`
	Permissions        []models.Permission             `json:"permissions"`
	IdentityResolution IdentityResolutionMetadata      `json:"identity_resolution"`
}
```

- [ ] **步骤 3：在内存中补全用户、组展开、来源组、继承链和未解析原因**

- [ ] **步骤 4：运行 historyservice 和 API 测试并提交**

### 任务 5：组成员 Excel/CSV 导出 API

**文件：**
- 新建：`apps/backend/internal/groupexport/exporter.go`
- 新建：`apps/backend/internal/groupexport/exporter_test.go`
- 修改：`apps/backend/cmd/api/handlers_ad.go`
- 修改：`apps/backend/cmd/api/routes.go`
- 修改：`apps/backend/cmd/api/middleware.go`
- 修改：`apps/backend/cmd/api/main_test.go`

- [ ] **步骤 1：写失败测试，验证 CSV BOM、空组表头、直属/嵌套范围和 XLSX 工作表**

```go
assert.Equal(t, []byte{0xEF, 0xBB, 0xBF}, csvBytes[:3])
assert.Contains(t, string(csvBytes), "Membership Path")
assert.Equal(t, "Members", workbook.GetSheetName(0))
```

- [ ] **步骤 2：实现纯导出器**

列固定为组名、组 DN、成员类型、显示名、`sAMAccountName`、邮箱、部门、Division、域、SID、成员 DN、直属/嵌套、深度、成员关系路径。Excel 默认，CSV 加 UTF-8 BOM。

- [ ] **步骤 3：新增 `POST /api/ad/groups/members/export`**

处理器使用 1 MiB 请求体上限、重新查询组、按 `include_nested` 调用 resolver、清理文件名、设置 RFC 5987 `Content-Disposition` 并直接写响应，不落地临时报告。

- [ ] **步骤 4：运行 groupexport 和 API 测试并提交**

### 任务 6：Web 删除二次扫描并增加导出交互

**文件：**
- 修改：`apps/web/pages/scan-workspace.tsx`
- 修改：`apps/web/components/ScanCompletionSummary.tsx`
- 修改：`apps/web/components/DirectoryExplorerWorkbench.tsx`
- 修改：`apps/web/components/ReportCenterWorkspace.tsx`
- 修改：`apps/web/components/__tests__/DirectoryExplorerWorkbench.test.tsx`
- 修改：`apps/web/components/__tests__/ScanCompletionSummary.test.tsx`
- 修改：`apps/web/components/__tests__/ReportCenterWorkspace.test.tsx`
- 新建：`apps/web/components/__tests__/ScanWorkspaceSIDResolution.test.ts`

- [ ] **步骤 1：先写失败测试**

测试扫描源码不再包含 `legacyExclusionGroupPatterns` 和第二次 `executeScan(false)`；组详情默认点击下载 Excel，菜单可下载 CSV，请求携带当前 `include_nested`；加载中禁用导出；解析摘要在扫描完成和报告中心可见。

- [ ] **步骤 2：删除 Web 排除规则副本和所有二次扫描路径**

扫描只发送一次请求。后端响应中的 `identity_resolution` 决定完成提示和解析摘要。

- [ ] **步骤 3：实现组成员导出按钮和菜单**

使用 Lucide `Download`、现有 `Button` 和 `DropdownMenu`。下载函数读取 blob、使用响应文件名或本地兜底名，并在 finally 中释放 object URL。

- [ ] **步骤 4：在 Report Center 显示紧凑解析徽章**

只增加一组小型 Badge，不创建说明面板或新页面。

- [ ] **步骤 5：运行相关 Jest 测试、typecheck 并提交**

### 任务 7：文档和完整验证

**文件：**
- 修改：`CHANGELOG.md`

- [ ] **步骤 1：更新中英文变更记录**

记录快照优先 SID 解析、单会话降级、历史补全和组成员 Excel/CSV 下载。开发命令未变化，不修改 `DEVELOPMENT.md`。

- [ ] **步骤 2：运行完整后端与 Web 验证**

```powershell
& .\tools\go\bin\go.exe -C .\apps\backend test ./...
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

- [ ] **步骤 3：运行桌面验证**

```powershell
dotnet test .\apps\desktop-win.tests\PermissionProtector.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\PermissionProtector.Desktop.csproj -c Release
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

- [ ] **步骤 4：检查 Git 差异、无敏感数据、提交最终整合**

---

# AD Snapshot SID Resolution and Group Member Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: use `superpowers:executing-plans` task by task. Track each checkbox and write a failing test before every production change.

**Goal:** Resolve new and historical scan SIDs from a bound AD snapshot first, preserve raw ACLs in the same session on resolution failure, and add default-Excel/optional-CSV group member export.

**Architecture:** A focused `identityresolution` package selects snapshots, batch-loads users/groups/memberships, expands group ACEs, and sends only snapshot misses to live LDAP. Scan Service owns single-session fallback and persistence; History Service performs read-only legacy enrichment. A separate `groupexport` package writes streamed CSV/XLSX output after the handler re-queries the group.

**Tech Stack:** Go 1.23, Gin, GORM, SQLite, excelize, Next.js 14, React 18, TypeScript, Jest, Testing Library.

## English Execution Summary

1. Add the qualified-wildcard regression test and prevent `DOMAIN\*` from becoming a short-name global wildcard.
2. Add session/permission resolution fields and a tested snapshot resolver covering users, groups, nested membership, well-known SIDs, LDAP supplementation, deduplication, and explicit unresolved reasons.
3. Change Scan Service so factory errors, expansion errors, and empty expansion results preserve raw ACLs and complete one session with `raw-fallback` metadata; bind stored connections to their latest completed snapshot.
4. Enrich history bundles in memory using a bound run or the legacy run with best SID coverage before scan time, falling back to the earliest later run with an explicit inference marker.
5. Add a pure group-export package plus `POST /api/ad/groups/members/export`, with Excel as default, CSV BOM, direct/nested scope, sanitized filenames, a 1 MiB request limit, and no temporary report files.
6. Remove all Web retry scans and the duplicate exclusion list; add the existing-design-system download action/menu and compact scan/report resolution badges.
7. Update the bilingual changelog and run the complete Go, Web, desktop, package, Git-diff, and sensitive-data checks listed above.
