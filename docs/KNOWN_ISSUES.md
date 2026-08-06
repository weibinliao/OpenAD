# OpenAD 已知问题与发布限制

[中文](#openad-已知问题与发布限制) | [English](#openad-known-issues-and-release-limits)

## 当前限制

1. 只有 Windows 已验证支持 NTFS 权限扫描。
   - 非 Windows 扫描器会返回“不支持”错误。

2. 当前正式桌面端是 .NET WinForms/WebView2。
   - `apps/desktop` 是历史 Tauri 探索，不能作为发布依据。

3. `<repository-root>` 是唯一规范 Git 根目录和发布入口。旧工作区仅作为本地回退副本保留，
   不能从中提交、推送或生成发布物。

4. 项目采用 open core 双许可结构。
   - 除 `ee/` 外的仓库主体采用 AGPL-3.0；`ee/` 被明确排除并适用商业许可，具体边界见
     `../LICENSING.md`。

5. 仓库尚未正式公布对外支持和安全联系渠道。
   - `SECURITY.md` 中的 `<SECURITY_CONTACT_EMAIL>` 仍是不可用占位符；GitHub Private
     Vulnerability Reporting 的启用状态也尚未确认。
   - 当前没有 SLA、支持版本窗口或已验证的外部漏洞私密报告渠道。

6. API 尚未实现产品级登录或 RBAC。
   - API、静态 Web、开发 Web 和 Windows 浏览器模式启动器默认只监听 `127.0.0.1`；空 `ALLOW_ORIGINS` 只允许 `localhost`、回环 IP 上的 `3010` 和 `43110` Web 来源，显式 `ALLOW_ORIGINS=*` 仍受支持。
   - 浏览器连接 `/api/scan/ws` 时只接受同源、`localhost`、回环 IP，或运维显式配置的额外来源；无 `Origin` 头的连接按非浏览器客户端处理并继续放行。
   - LAN Web 部署可通过 `WEBSOCKET_ALLOWED_ORIGINS` 添加逗号、分号或换行分隔的精确 `http(s)://host[:port]` 来源；未设置时回退读取兼容变量 `FSA_WEBSOCKET_ALLOWED_ORIGINS`。
   - Origin 校验不能替代身份认证，也不限制无 `Origin` 的非浏览器客户端。
   - 开发端口 `3010`、桌面 Web 端口 `43110` 和 API 端口 `18080` 只能放在可信管理网络。
   - LAN 暴露必须是运维人员明确执行的决定：以管理员身份运行 `scripts\enable-lan-access.bat` 添加防火墙规则，再替换并执行脚本打印的一次性 LAN 启动命令；脚本不会持久化全网卡绑定。

7. OpenAD 当前不是持续性 AD 威胁检测平台。
   - 实时事件监控、SIEM 集成、攻击路径图和自动响应不是当前支持能力。

8. 文件访问活动依赖 Windows 审计配置。
   - 需要启用对象访问审计、目标 SACL，并具有读取安全事件日志的权限。
   - 就绪检查只负责诊断，不会修改审计策略或 SACL。

9. 服务端导出只能写入配置的导出目录。
    - 可用 `PERMISSION_PROTECTOR_EXPORT_DIR` 修改默认目录。
    - `/api/export`、`/api/export/download` 和 `/api/export/summary` 的 JSON 请求体上限为 64 MiB；超限时需缩小扫描范围或分批导出。
    - 报告、身份和路径都可能包含敏感信息。

10. 少量内部兼容标识仍使用 PermissionProtector。
    - .NET 命名空间、环境变量和旧数据迁移路径仍需兼容；发布文件和安装目录已统一为 OpenAD。

## 发布建议

`v1.0.0` 已作为公司同事内部试用版发布，适合在可信 Windows 环境中受控使用和反馈。分发时
应同时提供 `INSTALL_WINDOWS.md`，并保留本文记录的平台、安全和认证限制。面向外部开源社区
的支持渠道与漏洞响应工作继续按 `OPEN_SOURCE_RELEASE_CHECKLIST.md` 和 `../SECURITY.md` 跟踪。

## OpenAD Known Issues and Release Limits

### Current Limits

1. NTFS permission scanning is verified only on Windows. The non-Windows scanner returns an unsupported error.
2. The shipping desktop application is .NET WinForms/WebView2. `apps/desktop` is historical Tauri exploration and is not a release source.
3. `<repository-root>` is the only canonical Git root and release entry point. The legacy worktree is a
   local recovery copy and must not be used for commits, pushes, or release artifacts.
4. The project uses an open-core dual-license structure. The repository body excluding `ee/` is under
   AGPL-3.0; `ee/` is expressly excluded and governed by a commercial license. See
   `../LICENSING.md` for the boundary.
5. No public support or security contact channel is formally published. `<SECURITY_CONTACT_EMAIL>` in
   `SECURITY.md` is still an inactive placeholder, and the status of GitHub Private Vulnerability
   Reporting is not confirmed. There is no SLA, supported-version window, or verified private channel
   for external vulnerability reports.
6. The API does not yet provide product-level login or RBAC. The API, static Web server, development Web server, and Windows browser-mode launchers bind only to `127.0.0.1` by default. With an empty `ALLOW_ORIGINS`, only local `localhost` or loopback-IP Web origins on ports `3010` and `43110` are allowed; explicit `ALLOW_ORIGINS=*` remains supported. Browser connections to `/api/scan/ws` accept only same-origin, `localhost`, loopback-IP, or explicitly configured origins; connections without an `Origin` header are treated as non-browser clients and remain allowed. LAN Web deployments can add exact, comma-, semicolon-, or newline-separated `http(s)://host[:port]` origins through `WEBSOCKET_ALLOWED_ORIGINS`, with `FSA_WEBSOCKET_ALLOWED_ORIGINS` as the compatibility fallback. Origin validation is not authentication and does not restrict non-browser clients without `Origin`. Development port `3010`, packaged Web port `43110`, and API port `18080` must remain on a trusted administration network. LAN exposure requires an explicit operator decision: run `scripts\enable-lan-access.bat` as Administrator to add firewall rules, then replace and execute its printed one-time LAN launch command. The script does not persist an all-interface binding.
7. OpenAD is not currently a continuous AD threat-detection platform. Real-time event monitoring, SIEM integration, attack-path graphs, and automated response are not supported.
8. File-access activity depends on Windows auditing. Object-access auditing, a target SACL, and security-event-log read access are required. The readiness check diagnoses configuration but does not modify audit policy or SACLs.
9. Server-side export can write only under the configured export directory. `PERMISSION_PROTECTOR_EXPORT_DIR` changes the default. JSON request bodies for `/api/export`, `/api/export/download`, and `/api/export/summary` are limited to 64 MiB; narrow the scan scope or export in smaller batches when the limit is exceeded. Reports, identities, and paths may contain sensitive information.
10. A small set of internal compatibility identifiers still uses PermissionProtector. .NET namespaces, environment variables, and legacy data migration paths remain compatible, while release files and install directories use OpenAD.

### Release Recommendation

`v1.0.0` is an internal pilot release built from the canonical OpenAD repository. It is suitable for
controlled colleague feedback within a trusted Windows environment, while the limitations above and
the remaining public support/security-channel checklist items continue to apply.
