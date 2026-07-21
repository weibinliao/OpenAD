# OpenAD 项目目录整理规范

[中文](#openad-项目目录整理规范) | [English](#openad-workspace-hygiene)

本规范用于区分源码、本地开发状态和可重建输出。之所以需要严格区分，是因为当前 Git
索引尚未完整覆盖 OpenAD 源码树。

## 必须保留

- `apps/**` 下的源码、测试、包清单和资源
- `scripts/**` 下的正式脚本
- `docs/**` 下的产品和工程文档
- `tools/**` 下供离线开发使用的便携工具链
- `apps/web/node_modules/**`，除非明确要求清理依赖
- `dist/**`，除非明确要求清理桌面包
- `.design/**` 下的有效视觉参考
- `.local/**` 下的数据库和本地运行配置
- `backups/**` 下的恢复备份，直到仓库源码基线完整
- 任何尚未证明是生成物的未跟踪文件

## 普通清理范围

默认清理脚本只删除可重建或临时内容：

- Go 构建缓存和模块缓存
- Next.js 开发、构建和静态导出结果
- .NET `bin`、`obj`
- 后端开发 exe
- 测试和开发日志
- 浏览器自动化日志和临时截图
- 代理会话状态和明确标记的回收目录

预览和执行：

```powershell
.\scripts\clean-workspace.ps1 -WhatIf
.\scripts\clean-workspace.ps1
```

## 显式扩展清理

以下内容默认不会删除：

```powershell
# 同时删除 npm 依赖；下次构建前需要重新 npm ci
.\scripts\clean-workspace.ps1 -IncludeDependencies

# 同时删除打包结果
.\scripts\clean-workspace.ps1 -IncludeReleases

# 同时删除项目内本地数据库和运行状态
.\scripts\clean-workspace.ps1 -IncludeLocalData
```

未确认数据可丢弃前，禁止使用 `-IncludeLocalData`。该脚本永远不会处理
`%APPDATA%\PermissionProtector`。

## 恢复备份

`backups/source-archives` 保存仓库边界尚未明确前生成的安全副本。这些文件已被 Git
忽略，不是源码事实来源。只有完整 OpenAD 源码已提交并可从可信远程仓库恢复后，才能
删除这些备份。

## 清理记录

2026-07-15 使用明确路径白名单整理工作区。源码、测试、当前桌面包、便携工具、npm
依赖、设计参考、本地数据库和恢复备份均被保留。工作区约从 2.75 GiB 降至 1.11 GiB，
释放约 1.64 GiB。

## OpenAD Workspace Hygiene

These rules separate source, local development state, and rebuildable output. Strict separation is required because the current Git index does not yet cover the complete OpenAD source tree.

### Always Preserve

- Source, tests, package manifests, and assets under `apps/**`
- Maintained scripts under `scripts/**`
- Product and engineering documentation under `docs/**`
- Portable offline-development toolchains under `tools/**`
- `apps/web/node_modules/**` unless dependency cleanup is explicitly requested
- `dist/**` unless release-package cleanup is explicitly requested
- Valid visual references under `.design/**`
- Databases and local runtime configuration under `.local/**`
- Recovery backups under `backups/**` until the repository source baseline is complete
- Any untracked file that has not been proven to be generated output

### Normal Cleanup

The default cleanup script removes only rebuildable or temporary content: Go caches, Next.js output, .NET `bin` and `obj`, backend development executables, test and development logs, browser-automation logs and temporary screenshots, agent session state, and explicitly marked recycle directories.

```powershell
.\scripts\clean-workspace.ps1 -WhatIf
.\scripts\clean-workspace.ps1
```

### Explicit Extended Cleanup

The following content is preserved unless its switch is supplied:

```powershell
# Also remove npm dependencies; npm ci is required before the next build.
.\scripts\clean-workspace.ps1 -IncludeDependencies

# Also remove packaged releases.
.\scripts\clean-workspace.ps1 -IncludeReleases

# Also remove project-local databases and runtime state.
.\scripts\clean-workspace.ps1 -IncludeLocalData
```

Do not use `-IncludeLocalData` until data loss is explicitly accepted. The script never touches `%APPDATA%\PermissionProtector`.

### Recovery Backups and Cleanup Record

`backups/source-archives` contains safety copies created while the repository boundary remains unresolved. These ignored files are not a source of truth. Remove them only after the complete OpenAD source is committed and recoverable from a trusted remote.

On 2026-07-15, the workspace was cleaned with an explicit path allowlist. Source, tests, the current desktop package, portable tools, npm dependencies, design references, local databases, and recovery backups were preserved. Workspace size decreased from approximately 2.75 GiB to 1.11 GiB, reclaiming approximately 1.64 GiB.
