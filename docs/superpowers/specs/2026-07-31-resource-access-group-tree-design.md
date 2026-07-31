# 按资源访问分析的 AD 组树设计

## 中文

### 背景与根因

“权限分析 > 按资源”当前把扫描会话中的权限行再次按 `trustee_sid` 分类。扫描阶段已经把
AD 组展开为成员用户，并在每条成员权限上保存 `originating_group` 与
`group_inheritance_hierarchy`，但按资源分析忽略了这两个字段。因此，已经解析成功的组成员
被误标为“直接用户”，原始组本身也没有作为结果节点保留；当组在快照中没有成员边时，组还会
被静默丢弃。

在用户报告的真实会话中，扫描数据包含 6 个来源组、239 个“组-成员”组合和 19 个真正的直接
用户，而现有接口显示为 234 个直接用户、11 个组成员和 6 个未解析主体。这证明问题位于按资源
分析的归类与展示层，而不是 SID 或 AD 快照没有解析成功。

### 目标

- 按 AD 树的层级显示结果：先显示 AD 组父节点，再显示该组的成员子行。
- 组节点默认展开，并允许逐组收起或展开。
- 所有组节点排在直接用户之前，未解析主体排在最后。
- 组即使没有可展开成员也必须显示，不能静默丢弃。
- 直接 ACL 用户与通过 AD 组获得权限的用户必须使用不同来源语义。
- 同时修复“权限分析”页面和复用同一接口的“资源管理器”结果。

### 非目标

- 不修改 AD、组成员关系或文件系统 ACL；OpenAD 继续保持只读。
- 不重写扫描历史，也不复制或迁移已有权限行。
- 不改变“按用户”分析、报告模板或组成员导出格式。
- 不引入新的视觉系统或独立页面。

### 后端数据语义

`ResourcePrincipal.source` 增加 `group` 类型，现有类型继续为 `user`、`group-member` 和
`unresolved`。组节点增加成员数量，并聚合该组带来的权限、允许/拒绝类型、路径和最高风险。

归类顺序如下：

1. 权限行存在 `originating_group` 时，把来源组保留为 `group` 父节点，把当前用户归为该组的
   `group-member`，并保留 `group_inheritance_hierarchy`。
2. 权限行的 `trustee_sid` 本身匹配快照组时，先保留组节点，再从快照成员关系生成成员子行；
   即使成员列表为空也保留组节点。
3. 仅当权限行既没有来源组、主体 SID 又匹配快照用户时，归为 `user`。
4. 其余主体归为 `unresolved`，不得丢弃。

来源组名称通过当前扫描绑定的 AD 快照匹配组记录并补充真实 SID。名称无法唯一匹配时仍以组名
作为稳定显示键保留节点，不把成员降级成直接用户。

接口中的 `principals` 保持单一数组以兼容现有消费者，但按树的遍历顺序返回：组节点、该组
成员、下一个组节点及其成员、直接用户、未解析主体。组成员以组 SID（无法匹配时使用规范化组
名）关联父节点。`counts` 增加组数量，现有用户、组成员和未解析数量按修正后的语义计算。

### 前端交互

“权限分析 > 按资源”的结果表继续使用现有列，不建立第二套布局：

- 组父行使用 `Users` 图标、`AD 组` 来源徽标、组 SID 和成员数量。
- 父行前使用展开/收起图标按钮，默认展开；按钮具有可访问名称。
- 成员子行缩进，使用用户图标和 `组成员` 来源徽标，缘由列显示来源组或嵌套链。
- 直接用户显示在所有组树之后；未解析 SID 最后显示。
- 搜索过滤命中组名时保留该组及其成员；命中成员时保留其父组，避免出现孤立子行。

“资源管理器”的紧凑结果使用相同顺序和父子缩进，并支持同样的逐组展开/收起。现有最多显示
100 行的限制按可见行计算，组父节点不能因成员过多而全部被挤出结果。

### 错误与边界情况

- 缺失组 SID：显示组名，隐藏空 SID，成员仍挂在该组下。
- 同名组：优先使用快照 SID 区分；无法区分的历史数据按规范化名称合并，并保留全部成员和权限。
- 空组或成员未进入快照：显示组父节点并标记成员数为 0。
- 用户同时拥有直接权限和组权限：直接用户行与各组成员行分别保留，解释不同访问路径。
- 用户属于多个授权组：在每个相关组下分别显示，不能错误去重为一个来源。
- Windows 内置主体和本地 SID：继续放在未解析区，不冒充 AD 组。

### 测试与验收

- 后端回归测试先证明当前实现会把带 `originating_group` 的成员误标为直接用户，再验证修复。
- 覆盖组父节点、成员顺序、直接用户顺序、空组保留、多组成员和计数语义。
- Web 组件测试覆盖组父行、缩进成员、默认展开、逐组收起和过滤父子关系。
- 运行完整 Go、Web 类型检查、Web 测试、静态构建、桌面测试与 Release 构建。
- 重建 Windows 桌面包，用用户报告的路径验证组名称先出现、成员位于对应组下、直接用户不再
  包含组展开产生的成员，并检查窗口、端口、`/health` 与 AD 连接状态。

## English

### Context and root cause

The resource access analysis reclassifies stored permission rows by `trustee_sid`. Scan-time group
expansion already records `originating_group` and `group_inheritance_hierarchy` on each expanded
member row, but the analysis ignores those fields. Resolved group members are therefore mislabeled
as direct users, the source group is not retained as a result node, and a group with no snapshot
membership edge can disappear entirely.

The reported production session contains six source groups, 239 group-member combinations, and 19
actual direct users. The current endpoint reports 234 direct users, 11 group members, and six
unresolved principals. The defect is in resource-analysis classification and presentation, not SID
or snapshot resolution.

### Goals

- Render an AD-tree hierarchy: each AD group first, followed by its member rows.
- Expand groups by default and allow each group to be collapsed or expanded.
- Place all groups before direct users and unresolved principals last.
- Preserve a group node even when no members can be expanded.
- Keep direct ACL users distinct from users receiving access through a group.
- Fix both Access Analysis and the Explorer consumer of the same endpoint.

### Backend semantics

Add `group` to `ResourcePrincipal.source`, alongside `user`, `group-member`, and `unresolved`. A group
row carries its member count and aggregated rights, ACE types, paths, and highest risk.

Classify rows in this order: honor `originating_group` as a parent group and the current trustee as
its member; preserve raw group trustees before expanding snapshot members; classify a user as direct
only when no source group exists; otherwise keep the trustee unresolved. Resolve source-group names
against the scan-bound snapshot to attach the real SID when possible. A missing or ambiguous SID must
not downgrade members to direct users.

Keep the existing flat `principals` array for compatibility, but return it in tree traversal order:
group, its members, next group, its members, direct users, then unresolved principals. Add a group
count and calculate all existing counts with the corrected semantics.

### Frontend behavior

Use the existing result table and visual system. Group rows use the `Users` icon, an `AD Group`
badge, SID, member count, and an accessible expand/collapse icon button. Member rows are indented and
show their source group or nested chain. Direct users follow all group trees; unresolved SIDs come
last. Filtering must retain the parent group when a child matches and retain children when the group
matches. Explorer uses the same hierarchy and ordering while applying its visible-row limit without
hiding all group parents behind one large member list.

### Verification

Add red-first backend and Web regression tests for expanded member classification, group parent
preservation, ordering, empty groups, counts, default expansion, collapse behavior, and hierarchical
filtering. Run the complete affected-module verification matrix, rebuild the Windows desktop package,
and validate the reported real path against the running desktop application and bound AD snapshot.
