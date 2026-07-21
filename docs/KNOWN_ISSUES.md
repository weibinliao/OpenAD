# OpenAD 已知问题与发布限制

[中文](#openad-已知问题与发布限制) | [English](#openad-known-issues-and-release-limits)

## 当前限制

1. 只有 Windows 已验证支持 NTFS 权限扫描。
   - 非 Windows 扫描器会返回“不支持”错误。

2. 当前正式桌面端是 .NET WinForms/WebView2。
   - `apps/desktop` 是历史 Tauri 探索，不能作为发布依据。

3. 当前实际 Git 根目录是父目录 `<workspace-parent>`。
   - 大部分 OpenAD 源码尚未进入 Git 索引。
   - 对外发布前必须修复仓库边界并提交完整源码基线。

4. 只要父目录仍是实际 Git 根目录，本项目内 GitHub 工作流就不会生效。
   - 确认标准仓库后，再在正确根目录启用工作流。

5. 项目没有顶层许可证。
   - 项目所有者明确选择许可证前，不能推定任何法律复用条款。

6. 仓库尚未正式公布对外支持和安全联系渠道。
   - 当前没有 SLA、支持版本窗口或公开漏洞报告邮箱。

7. API 尚未实现产品级登录或 RBAC。
   - 开发端口 `3010`、桌面 Web 端口 `43110` 和 API 端口 `18080` 只能放在可信管理网络。
   - LAN 暴露必须是运维人员明确执行的决定。

8. OpenAD 当前不是持续性 AD 威胁检测平台。
   - 实时事件监控、SIEM 集成、攻击路径图和自动响应不是当前支持能力。

9. 文件访问活动依赖 Windows 审计配置。
   - 需要启用对象访问审计、目标 SACL，并具有读取安全事件日志的权限。
   - 就绪检查只负责诊断，不会修改审计策略或 SACL。

10. 服务端导出只能写入配置的导出目录。
    - 可用 `PERMISSION_PROTECTOR_EXPORT_DIR` 修改默认目录。
    - 报告、身份和路径都可能包含敏感信息。

11. 内部兼容名称仍使用 PermissionProtector。
    - 没有升级和数据迁移方案前，不得修改可执行文件、命名空间、安装包和数据目录名称。

## 发布建议

当前工作区可作为 OpenAD 内部产品基线。许可证、仓库/索引、有效 CI、安全联系渠道和
全新检出构建验证完成前，不满足对外发布条件。完整清单见
`OPEN_SOURCE_RELEASE_CHECKLIST.md`。

## OpenAD Known Issues and Release Limits

### Current Limits

1. NTFS permission scanning is verified only on Windows. The non-Windows scanner returns an unsupported error.
2. The shipping desktop application is .NET WinForms/WebView2. `apps/desktop` is historical Tauri exploration and is not a release source.
3. The effective Git root is currently the parent directory `<workspace-parent>`. Most OpenAD source is not indexed, so the repository boundary and complete source baseline must be fixed before public release.
4. GitHub workflows inside this project will not run while the parent directory remains the effective Git root. Enable them only after establishing the standard repository root.
5. The project has no top-level license. No legal reuse terms may be inferred until the project owner explicitly selects one.
6. No public support or security contact channel is formally published. There is no SLA, supported-version window, or public vulnerability mailbox.
7. The API does not yet provide product-level login or RBAC. Development port `3010`, packaged Web port `43110`, and API port `18080` must remain on a trusted administration network. LAN exposure must be an explicit operator decision.
8. OpenAD is not currently a continuous AD threat-detection platform. Real-time event monitoring, SIEM integration, attack-path graphs, and automated response are not supported.
9. File-access activity depends on Windows auditing. Object-access auditing, a target SACL, and security-event-log read access are required. The readiness check diagnoses configuration but does not modify audit policy or SACLs.
10. Server-side export can write only under the configured export directory. `PERMISSION_PROTECTOR_EXPORT_DIR` changes the default. Reports, identities, and paths may contain sensitive information.
11. Internal compatibility names still use PermissionProtector. Executable, namespace, package, and data-directory names must not change without an upgrade and data-migration plan.

### Release Recommendation

The current workspace can serve as an internal OpenAD product baseline. It is not ready for public release until license selection, repository and index repair, active CI, a security contact channel, and clean-checkout build verification are complete. See `OPEN_SOURCE_RELEASE_CHECKLIST.md` for the full list.
