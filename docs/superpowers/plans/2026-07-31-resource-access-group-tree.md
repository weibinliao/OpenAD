# Resource Access AD Group Tree Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct resource-access classification so AD groups render as expandable parent rows with their resolved members beneath them, followed by direct users and unresolved SIDs.

**Architecture:** Keep `/api/access/by-resource` as the single source of truth. Extend its flat principal contract with a `group` source and `member_count`, derive parent groups from persisted `originating_group` evidence or raw group SIDs, and return rows in tree traversal order. Both existing Web consumers render that order as expandable group trees without introducing a new page or visual system.

**Tech Stack:** Go 1.23, GORM/SQLite, Next.js 14, React 18, TypeScript, Jest, Testing Library, .NET 10 WinForms/WebView2 packaging.

---

### Task 1: Reproduce and fix resource principal classification

**Files:**
- Modify: `apps/backend/internal/access/service_test.go`
- Modify: `apps/backend/internal/access/service.go`

- [ ] **Step 1: Write failing backend tests for persisted group evidence**

Add `TestByResourceUsesOriginatingGroupAsParent` with a completed snapshot containing group
`Sales-Team` and user `alice`, then persist an already-expanded permission row:

```go
models.Permission{
    ScanSessionID: session.ID,
    Path: `D:\Share\Sales`,
    Trustee: `EXAMPLE\alice`,
    TrusteeSID: aliceSID,
    Rights: "Modify",
    Type: "allow",
    OriginatingGroup: "Sales-Team",
    GroupInheritanceHierarchy: "Sales-Team",
}
```

Assert the result order and semantics:

```go
if len(result.Principals) != 2 {
    t.Fatalf("principals = %+v, want group parent and one member", result.Principals)
}
group, member := result.Principals[0], result.Principals[1]
if group.Source != "group" || group.SID != salesSID || group.Name != "Sales-Team" || group.MemberCount != 1 {
    t.Fatalf("group parent = %+v", group)
}
if member.Source != "group-member" || member.SID != aliceSID || member.GroupSID != salesSID {
    t.Fatalf("group member = %+v", member)
}
if result.Counts.Groups != 1 || result.Counts.Users != 0 || result.Counts.ViaGroups != 1 {
    t.Fatalf("counts = %+v", result.Counts)
}
```

Add `TestByResourceKeepsGroupWithoutSnapshotMembers` with a raw `Domain Users` group ACE and no
membership rows. Assert one `group` principal with `MemberCount == 0` remains in the response.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```powershell
& .\tools\go\bin\go.exe -C .\apps\backend test ./internal/access -run 'TestByResource(UsesOriginatingGroupAsParent|KeepsGroupWithoutSnapshotMembers)' -count=1
```

Expected: FAIL because the expanded member is currently classified as `user`, no group parent is
created from `originating_group`, and an empty raw group is dropped.

- [ ] **Step 3: Extend the response contract and snapshot lookup helpers**

In `ResourcePrincipal`, add:

```go
MemberCount int `json:"member_count,omitempty"`
```

In `ByResourceCounts`, add:

```go
Groups int `json:"groups"`
```

Load all snapshot groups once for the selected run and build case-insensitive lookup maps by SID and
name. Normalize group names with trimmed lowercase text; when a source-group name does not resolve to
a unique record, preserve the name with an empty SID instead of reclassifying its members.

- [ ] **Step 4: Classify permission rows using persisted provenance**

Refactor `ByResource` around four explicit branches:

```go
switch {
case strings.TrimSpace(permission.OriginatingGroup) != "":
    addGroupAndExpandedMember(permission)
case rawGroup, ok := groupsBySID[permission.TrusteeSID]; ok:
    addRawGroupAndSnapshotMembers(permission, rawGroup)
case rawUser, ok := usersBySID[permission.TrusteeSID]; ok:
    addDirectUser(permission, rawUser)
default:
    addUnresolved(permission)
}
```

Always add the `group` candidate before attempting member expansion. Use a group aggregation key based
on SID, falling back to normalized name. Aggregate rights, types, paths, and highest risk onto group
parents. Deduplicate members per group while preserving a separate direct-user row when the same SID
also has a direct ACE.

- [ ] **Step 5: Return deterministic tree traversal order and corrected counts**

Sort principals with the following comparator:

```go
// group key ascending; parent before its members; then direct users; unresolved last.
```

For each group, set `MemberCount` to the number of distinct `group-member` rows associated with that
group. Count `group`, `user`, `group-member`, and `unresolved` independently, and set `Principals` to
the returned row count.

- [ ] **Step 6: Run focused and complete access tests and verify GREEN**

Run:

```powershell
& .\tools\go\bin\go.exe -C .\apps\backend test ./internal/access -count=1
```

Expected: PASS, including existing by-user behavior and corrected by-resource expectations.

- [ ] **Step 7: Commit the backend behavior**

```powershell
git add apps/backend/internal/access/service.go apps/backend/internal/access/service_test.go
git commit -m "fix(access): preserve AD groups in resource analysis"
```

### Task 2: Render the tree in Access Analysis

**Files:**
- Create: `apps/web/components/__tests__/AccessAnalysisPage.test.tsx`
- Modify: `apps/web/pages/access.tsx`
- Modify: `apps/web/lib/i18n/zh.ts`
- Modify: `apps/web/lib/i18n/en.ts`

- [ ] **Step 1: Write a failing page-level interaction test**

Mock `next/router` with `query.path`, mock `/api/access/by-resource` to return rows in this order:
Finance group, Alice member, Bob direct user, Everyone unresolved. Render the page inside
`I18nProvider` and assert:

```tsx
expect(await screen.findByText('Finance')).toBeInTheDocument();
expect(screen.getByText('Alice')).toBeInTheDocument();
expect(screen.getByText('AD Group')).toBeInTheDocument();
expect(screen.getByRole('button', { name: 'Collapse Finance' })).toBeInTheDocument();
```

Read table rows and assert Finance precedes Alice, Alice precedes Bob, and Bob precedes Everyone.
Click `Collapse Finance`, then assert Alice is absent while Finance and Bob remain.

- [ ] **Step 2: Run the new test and verify RED**

Run:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web test -- --runTestsByPath components/__tests__/AccessAnalysisPage.test.tsx
```

Expected: FAIL because `group` has no badge/icon, no parent control exists, and members cannot be
collapsed.

- [ ] **Step 3: Add group-aware types, counts, and localized labels**

Extend `ResourcePrincipal` with `group_sid`, `member_count`, and the `group` source. Extend counts with
`groups`. Add bilingual labels for `AD Group`, group count, member count, `Expand {group}`, and
`Collapse {group}`.

- [ ] **Step 4: Render group parents and collapsible children**

Track collapsed groups with `Set<string>`. Derive a stable group key from `group_sid || sid ||
group_name || name`. Render:

```tsx
<Button variant="ghost" size="icon" aria-label={collapseLabel}>
  <ChevronDown className="h-3.5 w-3.5" />
</Button>
```

Use `Users` for group parents and `UserRound` for indented members. Hide a member row only when its
parent group is collapsed. Show `member_count` on the group row and preserve the existing rights,
allow/deny, and risk columns.

- [ ] **Step 5: Run the page test and Web typecheck**

Run:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web test -- --runTestsByPath components/__tests__/AccessAnalysisPage.test.tsx
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
```

Expected: PASS with no TypeScript errors.

- [ ] **Step 6: Commit the Access Analysis UI**

```powershell
git add apps/web/pages/access.tsx apps/web/lib/i18n/zh.ts apps/web/lib/i18n/en.ts apps/web/components/__tests__/AccessAnalysisPage.test.tsx
git commit -m "feat(access): render AD groups as expandable parents"
```

### Task 3: Apply the same hierarchy to Explorer

**Files:**
- Modify: `apps/web/components/explorer/Explorer.tsx`
- Create: `apps/web/components/__tests__/ExplorerResourceAnswer.test.tsx`

- [ ] **Step 1: Add a failing Explorer regression assertion**

Render the default `Explorer`, mock its completed-session response with one share, select the generated
`Who can access ...?` suggestion, and return a group parent plus one member from the resource-access
endpoint. Assert the compact answer shows the group before the member, exposes an accessible collapse
control, and keeps the parent visible after collapse.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web test -- --runTestsByPath components/__tests__/ExplorerResourceAnswer.test.tsx
```

Expected: FAIL because Explorer treats `group` as a user and has no hierarchy control.

- [ ] **Step 3: Implement compact group rows and child indentation**

Extend `ResourcePrincipalLite` with `group_sid` and `member_count`. Track collapsed group keys inside
`ResourceAnswer`, render group parents with `Users` and chevron controls, indent `group-member` rows,
and omit collapsed children. Apply the 100-row cap after collapse filtering so parent rows are never
lost behind hidden children. Filtering by a group retains its members; filtering by a member retains
its parent group.

- [ ] **Step 4: Run focused tests, the full Web suite, and static build**

Run:

```powershell
& .\tools\node\npm.cmd --prefix .\apps\web test -- --runTestsByPath components/__tests__/ExplorerResourceAnswer.test.tsx
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
```

Expected: all suites pass and 16 static pages generate.

- [ ] **Step 5: Commit Explorer behavior**

```powershell
git add apps/web/components/explorer/Explorer.tsx apps/web/components/__tests__/ExplorerResourceAnswer.test.tsx
git commit -m "feat(explorer): show resource access as an AD group tree"
```

### Task 4: Document, verify, package, and validate real data

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add bilingual changelog entries**

Under both Unreleased Fixed sections, record that resource access now honors persisted group
provenance, renders AD groups before members, preserves empty groups, and distinguishes true direct
users from group-derived members.

- [ ] **Step 2: Run the complete affected-module verification matrix**

Run:

```powershell
& .\tools\go\bin\go.exe -C .\apps\backend test ./...
& .\tools\node\npm.cmd --prefix .\apps\web run typecheck
& .\tools\node\npm.cmd --prefix .\apps\web test
& .\tools\node\npm.cmd --prefix .\apps\web run build:static
dotnet test .\apps\desktop-win.tests\PermissionProtector.Desktop.Tests.csproj -c Release
dotnet build .\apps\desktop-win\PermissionProtector.Desktop.csproj -c Release
```

Expected: all Go packages pass, all Web suites pass, 16 pages build, desktop tests pass, and Release
build reports zero errors.

- [ ] **Step 3: Commit the changelog**

```powershell
git add CHANGELOG.md
git commit -m "docs: record resource AD group tree fix"
```

- [ ] **Step 4: Rebuild the complete Windows package**

Stop only processes whose executable paths are under
`<repository-root>\dist\OpenAD-Windows-Desktop-v0.1.0`, then run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-desktop-windows.ps1
```

Expected: `[OK] Desktop package created` at
`<repository-root>\dist\OpenAD-Windows-Desktop-v0.1.0`.

- [ ] **Step 5: Start the package and validate the reported path**

Start `OpenAD.exe`, wait for `http://127.0.0.1:18080/health`, and POST the exact path
`\\files.example.com\software\example-team` to `/api/access/by-resource`. Assert the response begins with group
parents, each `group-member` follows its group, `counts.groups` is nonzero, and users carried by
`originating_group` are not counted as direct users. Confirm ports `18080` and `43110`, window title
`OpenAD`, process responsiveness, Web HTTP 200, and the saved AD connection test.

- [ ] **Step 6: Final repository audit**

Run `git status --short --branch`, `git diff --check`, inspect the commits created by this plan, and
confirm license files and unrelated user changes were not modified. Do not push unless explicitly
requested.
