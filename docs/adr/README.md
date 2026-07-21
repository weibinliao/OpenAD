# 架构决策记录（ADR）

[中文](#架构决策记录adr) | [English](#architecture-decision-records-adr)

涉及多个模块、兼容性、安全、数据格式、发布结构或长期维护的决策，应使用 ADR 记录。

## 使用流程

1. 复制 `0000-template.md`，使用下一个四位序号命名。
2. 文件名使用简短英文标识，例如 `0002-report-session-contract.md`。
3. 记录背景、决策、影响、备选方案和验证要求。
4. 讨论中状态使用“提议”，批准后改为“已接受”。
5. 已接受 ADR 的核心内容不再修改；后续变化通过新 ADR 替代。

ADR 只记录长期架构决策。日常状态写入 `.codex/memory.md`，用户可见变化写入
`CHANGELOG.md`。

## Architecture Decision Records (ADR)

Use an ADR for decisions that affect multiple modules, compatibility, security, data formats, release structure, or long-term maintenance.

1. Copy `0000-template.md` and assign the next four-digit number.
2. Use a short English filename such as `0002-report-session-contract.md`.
3. Record context, decision, consequences, alternatives, and verification requirements.
4. Use `Proposed` during discussion and change to `Accepted` after approval.
5. Do not rewrite the core content of an accepted ADR; supersede it with a new ADR.

ADRs store durable architecture decisions. Daily state belongs in `.codex/memory.md`, and user-visible changes belong in `CHANGELOG.md`.
