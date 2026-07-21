## 修改摘要 / Summary

说明用户或运维问题，以及本次实现的变化。
Describe the user or operational problem and the implemented change.

## 修改范围 / Scope

- [ ] 后端或扫描器 / Backend or scanner
- [ ] Web 界面 / Web interface
- [ ] Windows 桌面端 / Windows desktop
- [ ] 打包或运维 / Packaging or operations
- [ ] 仅文档 / Documentation only

## 验证结果 / Verification

列出准确命令和结果。涉及可见界面时，附实际桌面端或页面验证。
List exact commands and results. Include actual desktop or page verification for visible changes.

```text
命令 / Command:
结果 / Result:
```

## 产品边界检查 / Product Boundary Check

- [ ] 扫描中心仍只负责扫描执行和进度。 / Scan Center still owns only scan execution and progress.
- [ ] 报告功能仍位于 `/reports`。 / Report functionality remains under `/reports`.
- [ ] 系统设置只包含应用和运行时配置。 / System Settings contains only application and runtime configuration.
- [ ] 用户可见产品名称为 OpenAD。 / The user-visible product name is OpenAD.
- [ ] 未加入凭据、目录数据、扫描导出或本地数据库。 / No credentials, directory data, scan exports, or local databases were added.

## 截图 / Screenshots

可见界面修改必须提供截图。涉及布局或窗口行为时，同时提供正常桌面尺寸和受限窗口状态。
Visible UI changes require screenshots. For layout or window behavior, include both normal desktop size and a constrained-window state.

## 风险与后续工作 / Risks and Follow-up

说明迁移、兼容行为、已知缺口或延后事项。
Describe migration impact, compatibility behavior, known gaps, or deferred work.
