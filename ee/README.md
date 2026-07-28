# OpenAD 商业组件 / OpenAD Commercial Components

[中文](#中文) | [English](#english)

## 中文

`ee/` 是 OpenAD 为后续商业功能保留的明确许可边界。此目录当前不包含任何功能代码，加入
此目录不会改变开源产品的构建、测试或运行方式。

此目录中的当前和未来内容不受仓库顶层 AGPL-3.0 许可覆盖，也不能仅凭顶层 `LICENSE` 使用。
使用、复制、修改、部署或分发这里的内容前，必须取得 Weibin Liao 发出的有效商业授权，并
遵守 [`LICENSE`](LICENSE) 中的商业条款。仓库整体许可地图见 [`../LICENSING.md`](../LICENSING.md)。

规划中的商业功能包括：

- 多用户与基于角色的访问控制（RBAC）；
- 定时任务与告警；
- 高级合规报告。

这些项目仅表示后续规划，不代表当前版本已经实现或承诺交付。AGPL-3.0 覆盖的开源部分必须
始终能够独立构建，并构成完整可用的 OpenAD 产品；商业组件不得成为开源构建或基本功能的
必需依赖。

## English

The `ee/` directory is the explicit license boundary reserved for future OpenAD commercial features.
It currently contains no functional code, and its presence does not change how the open-source product
is built, tested, or run.

Current and future content in this directory is not covered by the repository's top-level AGPL-3.0
license and cannot be used under the top-level `LICENSE` alone. Before using, copying, modifying,
deploying, or distributing anything here, you must obtain a valid commercial authorization issued by
Weibin Liao and comply with the commercial terms in [`LICENSE`](LICENSE). See
[`../LICENSING.md`](../LICENSING.md) for the repository-wide license map.

Planned commercial areas include:

- multi-user support and role-based access control (RBAC);
- scheduled jobs and alerts; and
- advanced compliance reports.

These items are roadmap directions only; they are not implemented features or delivery commitments.
The AGPL-3.0 open-source portion must remain independently buildable and form a complete, usable OpenAD
product. Commercial Components must not become required dependencies of the open-source build or core
functionality.
