# OpenAD 已知问题与发布限制

[中文](#openad-已知问题与发布限制) | [English](#openad-known-issues-and-release-limits)

## 当前限制

1. 只有 Windows 已验证支持 NTFS 权限扫描。
   - 非 Windows 扫描器会返回“不支持”错误。

2. 当前正式桌面端是 .NET WinForms/WebView2。
   - `apps/desktop` 是历史 Tauri 探索，不能作为发布依据。

3. 历史工作区曾以 `<workspace-parent>` 作为实际 Git 根目录。
   - 大部分 OpenAD 源码尚未进入 Git 索引。
   - 对外发布前必须修复仓库边界并提交完整源码基线。

4. 只要父目录仍是实际 Git 根目录，本项目内 GitHub 工作流就不会生效。
   - 确认标准仓库后，再在正确根目录启用工作流。

5. 项目采用 open core 双许可结构。
   - 除 `ee/` 外的仓库主体采用 AGPL-3.0；`ee/` 被明确排除并适用商业许可，具体边界见
     `../LICENSING.md`。

6. 仓库尚未正式公布对外支持和安全联系渠道。
   - `SECURITY.md` 中的 `<SECURITY_CONTACT_EMAIL>` 仍是不可用占位符；GitHub Private
     Vulnerability Reporting 的启用状态也尚未确认。
   - 当前没有 SLA、支持版本窗口或已验证的外部漏洞私密报告渠道。

7. API 尚未实现产品级登录或 RBAC。
   - API、静态 Web、开发 Web 和 Windows 浏览器模式启动器默认只监听 `127.0.0.1`；空 `ALLOW_ORIGINS` 只允许 `localhost`、回环 IP 上的 `3010` 和 `43110` Web 来源，显式 `ALLOW_ORIGINS=*` 仍受支持。
   - 浏览器连接 `/api/scan/ws` 时只接受同源、`localhost`、回环 IP，或运维显式配置的额外来源；无 `Origin` 头的连接按非浏览器客户端处理并继续放行。
   - LAN Web 部署可通过 `WEBSOCKET_ALLOWED_ORIGINS` 添加逗号、分号或换行分隔的精确 `http(s)://host[:port]` 来源；未设置时回退读取兼容变量 `FSA_WEBSOCKET_ALLOWED_ORIGINS`。
   - Origin 校验不能替代身份认证，也不限制无 `Origin` 的非浏览器客户端。
   - 开发端口 `3010`、桌面 Web 端口 `43110` 和 API 端口 `18080` 只能放在可信管理网络。
   - LAN 暴露必须是运维人员明确执行的决定：以管理员身份运行 `scripts\enable-lan-access.bat` 添加防火墙规则，再替换并执行脚本打印的一次性 LAN 启动命令；脚本不会持久化全网卡绑定。

8. OpenAD 当前不是持续性 AD 威胁检测平台。
   - 实时事件监控、SIEM 集成、攻击路径图和自动响应不是当前支持能力。

9. 文件访问活动依赖 Windows 审计配置。
   - 需要启用对象访问审计、目标 SACL，并具有读取安全事件日志的权限。
   - 就绪检查只负责诊断，不会修改审计策略或 SACL。

10. 服务端导出只能写入配置的导出目录。
    - 可用 `PERMISSION_PROTECTOR_EXPORT_DIR` 修改默认目录。
    - `/api/export`、`/api/export/download` 和 `/api/export/summary` 的 JSON 请求体上限为 64 MiB；超限时需缩小扫描范围或分批导出。
    - 报告、身份和路径都可能包含敏感信息。

11. 内部兼容名称仍使用 PermissionProtector。
    - 没有升级和数据迁移方案前，不得修改可执行文件、命名空间、安装包和数据目录名称。

## 发布建议

当前工作区可作为 OpenAD 内部产品基线。商业与安全联系占位符、仓库/索引、有效 CI 和
全新检出构建验证完成前，不满足对外发布条件。完整清单见
`OPEN_SOURCE_RELEASE_CHECKLIST.md`。

## OpenAD Known Issues and Release Limits

### Current Limits

1. NTFS permission scanning is verified only on Windows. The non-Windows scanner returns an unsupported error.
2. The shipping desktop application is .NET WinForms/WebView2. `apps/desktop` is historical Tauri exploration and is not a release source.
3. The historical worktree used `<workspace-parent>` as its effective Git root. The repository boundary and complete source baseline must be fixed before public release.
4. GitHub workflows inside this project will not run while the parent directory remains the effective Git root. Enable them only after establishing the standard repository root.
5. The project uses an open-core dual-license structure. The repository body excluding `ee/` is under
   AGPL-3.0; `ee/` is expressly excluded and governed by a commercial license. See
   `../LICENSING.md` for the boundary.
6. No public support or security contact channel is formally published. `<SECURITY_CONTACT_EMAIL>` in
   `SECURITY.md` is still an inactive placeholder, and the status of GitHub Private Vulnerability
   Reporting is not confirmed. There is no SLA, supported-version window, or verified private channel
   for external vulnerability reports.
7. The API does not yet provide product-level login or RBAC. The API, static Web server, development Web server, and Windows browser-mode launchers bind only to `127.0.0.1` by default. With an empty `ALLOW_ORIGINS`, only local `localhost` or loopback-IP Web origins on ports `3010` and `43110` are allowed; explicit `ALLOW_ORIGINS=*` remains supported. Browser connections to `/api/scan/ws` accept only same-origin, `localhost`, loopback-IP, or explicitly configured origins; connections without an `Origin` header are treated as non-browser clients and remain allowed. LAN Web deployments can add exact, comma-, semicolon-, or newline-separated `http(s)://host[:port]` origins through `WEBSOCKET_ALLOWED_ORIGINS`, with `FSA_WEBSOCKET_ALLOWED_ORIGINS` as the compatibility fallback. Origin validation is not authentication and does not restrict non-browser clients without `Origin`. Development port `3010`, packaged Web port `43110`, and API port `18080` must remain on a trusted administration network. LAN exposure requires an explicit operator decision: run `scripts\enable-lan-access.bat` as Administrator to add firewall rules, then replace and execute its printed one-time LAN launch command. The script does not persist an all-interface binding.
8. OpenAD is not currently a continuous AD threat-detection platform. Real-time event monitoring, SIEM integration, attack-path graphs, and automated response are not supported.
9. File-access activity depends on Windows auditing. Object-access auditing, a target SACL, and security-event-log read access are required. The readiness check diagnoses configuration but does not modify audit policy or SACLs.
10. Server-side export can write only under the configured export directory. `PERMISSION_PROTECTOR_EXPORT_DIR` changes the default. JSON request bodies for `/api/export`, `/api/export/download`, and `/api/export/summary` are limited to 64 MiB; narrow the scan scope or export in smaller batches when the limit is exceeded. Reports, identities, and paths may contain sensitive information.
11. Internal compatibility names still use PermissionProtector. Executable, namespace, package, and data-directory names must not change without an upgrade and data-migration plan.

### Release Recommendation

The current workspace can serve as an internal OpenAD product baseline. It is not ready for public release until the commercial and security contact placeholders, repository and index repair, active CI, a security contact channel, and clean-checkout build verification are complete. See `OPEN_SOURCE_RELEASE_CHECKLIST.md` for the full list.
