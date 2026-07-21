# ADR 0001：OpenAD 产品身份与兼容名称

[中文](#adr-0001openad-产品身份与兼容名称) | [English](#adr-0001-openad-product-identity-and-compatibility-names)

- 状态：已接受
- 日期：2026-07-15
- 负责人：OpenAD 项目维护者

## 背景

产品早期使用 PermissionProtector 名称。当前产品方向已明确使用 OpenAD，但已有可执行
文件名、.NET 命名空间、安装包目录和本地数据路径仍使用旧名称。如果一次性全部改名，
可能破坏升级、启动脚本、本地数据发现和现有自动化。

## 决策

- 导航、窗口标题、品牌、文档和产品文案统一使用 OpenAD。
- PermissionProtector 暂时保留在可执行文件、命名空间、安装包目录和
  `%APPDATA%\PermissionProtector` 中。
- 只有具备明确迁移方案、升级测试、回退行为和发行说明后，才能迁移兼容名称。

## 影响

- 部分内部路径暂时不会与产品名称一致。
- 文档必须明确区分产品身份和兼容名称。
- 打包和安装程序不能静默移动或遗弃已有本地数据。

## 备选方案

- 立即全部改名：未采用，因为当前没有经过验证的数据和升级迁移方案。
- 继续把 PermissionProtector 作为产品名：未采用，因为与已确认的 OpenAD 产品方向冲突。

## 验证

- 界面和原生窗口显示 OpenAD。
- 现有桌面二进制仍能启动并找到本地数据。
- 在兼容迁移完成前，发行说明持续解释旧名称。

## ADR 0001: OpenAD Product Identity and Compatibility Names

- Status: Accepted
- Date: 2026-07-15
- Owner: OpenAD maintainers

### Context

The product originally used the PermissionProtector name. The current product direction is OpenAD, while existing executable names, .NET namespaces, package directories, and local data paths still use the old name. Renaming everything at once could break upgrades, startup scripts, local data discovery, and existing automation.

### Decision

- Use OpenAD consistently in navigation, window titles, branding, documentation, and product copy.
- Temporarily preserve PermissionProtector in executables, namespaces, package directories, and `%APPDATA%\PermissionProtector`.
- Migrate compatibility names only after defining a migration plan, upgrade tests, rollback behavior, and release notes.

### Consequences

- Some internal paths will temporarily differ from the product name.
- Documentation must clearly distinguish product identity from compatibility names.
- Packaging and installation must not silently move or abandon existing local data.

### Alternatives

- Rename everything immediately: rejected because there is no verified data and upgrade migration plan.
- Continue using PermissionProtector as the product name: rejected because it conflicts with the approved OpenAD direction.

### Verification

- The interface and native window display OpenAD.
- Existing desktop binaries still start and locate local data.
- Release notes continue to explain the old name until compatibility migration is complete.
