# Requirement Packages

本目录用于沉淀“可交给 AI 或开发者直接执行”的需求包。

约定如下：

- 一个需求一个子目录，目录名使用小写 `kebab-case`。
- 同一需求目录内至少包含两份文件：`PRD.md` 与 `implementation-prompt.md`。
- `PRD.md` 负责描述业务背景、目标、范围、规则、接口和验收标准。
- `implementation-prompt.md` 负责把 PRD 转成可直接发给 AI 编码代理的执行说明。
- 文档应只描述当前需求本身，不把具体提交号、已有实现细节或后续迭代实现当作阅读前提。
- 如确有依赖，可以简要说明所需的前序能力或前置条件，但保持说明最小且自洽。
- 若某类实现注意事项可原样复用于多个需求，应上收为公共注意项，而不是在每个 PRD/prompt 中重复展开。
- 如有额外材料，可在同目录补充 `notes.md`、`api-examples.md`、`screenshots/` 等文件或子目录。

公共注意项建议统一维护在 [common-implementation-notes.md](common-implementation-notes.md) 中；单个需求包只保留与当前需求强相关、不可替代的业务约束和交互要求。
Implementation prompt 可以在开头简要引用公共注意项，而不是重复整段仓库约束或通用验证模板。

推荐结构：

```text
docs/requirements/
  <requirement-slug>/
    PRD.md
    implementation-prompt.md
```

当前已收录：

- `sub-business-company-department-management/`
- `organization-shared-time-slot-rpm-limits/`
- `usage-log-period-and-realtime-rpm-tpm-stats/`
- `application-management-and-x-app-id-validation/`

公共材料：

- `common-implementation-notes.md`




