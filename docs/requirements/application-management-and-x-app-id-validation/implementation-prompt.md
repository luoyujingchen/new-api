# Implementation Prompt

你正在当前仓库中实现“应用管理、x-app-id 校验与应用 Header 来源识别/严格匹配”功能。

请直接在当前仓库中完成实现，不要只输出方案。实现范围以同目录下的 `PRD.md` 为准，只实现本文档描述的需求范围，不要引入未明确列出的其他扩展能力。

实现前先阅读 [../common-implementation-notes.md](../common-implementation-notes.md)；本文只补充当前需求特有的实现范围、约束和验证点。

## 目标

在当前系统中补齐以下能力：

- 新增 Application 资源及后台管理。
- 为 Application 自动生成唯一 `app_key`。
- 允许 API Key 可选绑定一个启用中的应用。
- 在请求鉴权阶段校验 `x-app-id` 与绑定应用的一致性。
- 在请求鉴权阶段支持应用 Header 来源识别，并按全局模式与应用级开关决定是否严格拦截。
- 支持 `off | observe | enforce` 三种全局识别模式，默认 `off`。
- 支持应用级 `header_match_required`，用于要求 Header 推断应用必须与 Token 绑定应用一致。
- 在默认前端补齐应用管理页面、路由、侧边栏入口和多语言文案。

注意：本次只是在系统中建立“按应用归因”的基础设施，不要扩展成新的应用统计报表或应用计费后台。

## 当前需要完成的增量变更

当前仓库中可能已经存在 Application、Token 绑定、`x-app-id` 校验以及基础 `header_validation_rules` 实现。本轮需要把应用匹配相关能力调整为 PRD 中的最终形态，重点完成：

1. 新增或补齐系统选项 `ApplicationHeaderDetectionMode`，可选值为 `off`、`observe`、`enforce`，默认 `off`。
2. 新增或补齐应用字段 `header_match_required`，默认 `false`，并贯通后端 DTO、接口响应、前端表单和列表展示。
3. 调整请求期 Header 识别逻辑：
   - `off`：跳过 Header 识别。
   - `observe`：执行识别并写入现有请求日志，但不拦截。
   - `enforce`：只有 Token 绑定应用开启 `header_match_required` 时才按 Header 推断结果拦截。
4. 严格匹配判定必须以 Token 绑定应用为准：Token 绑定应用表示授权范围，Header 推断应用表示请求侧身份验证证据；开启严格匹配时，Header 推断应用必须与 Token 绑定应用一致；`enforce` 模式下 Token 未绑定应用且 Header 推断到任意应用或多个应用时必须拒绝。
5. 禁用应用完全不参与 Header 匹配；Token 绑定禁用应用仍按既有绑定校验拒绝。
6. Header 识别热路径必须使用缓存，不得每个请求查询全量应用并逐条解析 JSON；应用创建、更新、启停、删除后必须失效。
7. Header 识别结果写入现有请求日志或请求上下文快照，不新增独立 Header 明细日志表。
8. 现有日志页或日志详情需要能查看应用 Header 识别结果。
9. 前端需要提供全局模式配置、应用级严格匹配开关、Header 规则编辑说明，并同步所有默认前端 locale。

## 必须实现的范围

### 1. 后端数据模型与服务

实现或补齐：

- `Application` 模型
- Application 的分页、全量、详情、创建、更新、删除、状态更新能力
- Application 与 Token 的关联统计能力
- 统一的应用选择校验逻辑

关键规则：

- `app_key` 创建时自动生成，且必须唯一。
- `name` 必填，最大 128，且未删除记录中唯一。
- `description` 最大 500。
- `status` 仅允许 `0 | 1`。
- `sort_order` 为整数。
- `header_validation_rules` 为可选 Header 来源识别规则数组，支持 `equals` 和 `one_of`。
- `header_match_required` 为布尔值，默认 `false`，表示该应用是否要求 Header 推断应用与 Token 绑定应用一致。
- 删除应用前必须检查是否存在关联的 Token。

建议涉及文件：

- `model/application.go`
- `service/application.go`
- `dto/application.go`
- `model/token.go`

### 2. 后端接口与路由

实现以下接口：

```text
GET    /api/application
GET    /api/application/all
GET    /api/application/:id
POST   /api/application
PUT    /api/application/:id
DELETE /api/application/:id
PATCH  /api/application/:id/status
GET    /api/application/self/all
```

要求：

- 管理接口走管理员权限。
- `self/all` 走普通用户权限。
- 分页列表支持 `page`、`page_size`、`status`。
- 列表/详情返回 `token_count`。
- 删除被绑定应用时返回清晰错误。

建议涉及文件：

- `controller/application.go`
- `router/api-router.go`

### 3. API Key 绑定应用

在现有 API Key 创建/更新逻辑中新增 `application_id` 支持。

要求：

- 创建 Token 时允许提交 `application_id`。
- 更新 Token 时也允许修改或清空 `application_id`。
- 创建和更新都必须校验：
  - 应用存在
  - 应用已启用
- 当未选择应用时，应允许正常保存。

建议涉及文件：

- `controller/token.go`
- `model/token.go`
- 与 Token 请求/响应绑定相关的现有类型文件

### 4. 请求期 x-app-id 校验

在 Token 鉴权链路中新增应用校验逻辑。

要求：

- 当 Token 已绑定应用时，先加载并验证该应用可用性。
- 将应用信息写入上下文：
  - `application_id`
  - `application_key`
  - `application_name`
- 上述应用上下文字段只表示 Token 绑定且已校验的授权应用，不得写入仅由 Header 推断出的应用。
- Header 推断应用必须写入独立字段，例如 `application_header_detection.matched_application_*` 或等价的 `detected_application_*`。
- 读取请求头 `x-app-id`。
- 若请求未携带 `x-app-id`，允许继续执行。
- 若请求携带 `x-app-id` 但 Token 未绑定应用，拒绝请求。
- 若请求携带 `x-app-id` 且与绑定应用 `app_key` 不一致，拒绝请求。
- 若 Token 绑定的应用已禁用，拒绝请求。
- 若 Token 绑定的应用记录不存在，拒绝请求。
- 校验后不要把原始 `x-app-id` 继续透传到后续上游链路。

建议涉及文件：

- `middleware/auth.go`
- `constant/context_key.go`

### 5. 应用 Header 来源识别与严格匹配

在 Token 鉴权链路中实现或调整 Header 来源识别。

语义边界：

- Token 绑定应用表示该 Token 的授权使用范围。
- Header 推断应用表示本次请求的来源身份验证证据。
- Header 推断结果不能单独替代 Token 绑定授权关系。
- 某些第三方应用可能无法携带本系统指定的自定义 Header；这类应用不应开启 `header_match_required`，或应使用该第三方请求中天然稳定、可被管理员确认的 Header 作为规则来源。

全局模式：

- `off`：不执行 Header 应用识别。
- `observe`：执行 Header 应用识别，记录结果，不拒绝请求。
- `enforce`：执行 Header 应用识别；只有 Token 绑定应用开启 `header_match_required` 时才执行严格拦截。

Header 规则：

- 支持 `equals` 和 `one_of`。
- 同一应用多条规则为 AND。
- Header 名大小写不敏感。
- Header 值大小写敏感，精确匹配。
- 无规则应用不参与识别。
- 禁用应用不参与识别。
- 推荐使用稳定自定义 Header，例如 `X-Client-App`。

`enforce` 模式下：

- 如果 Token 绑定应用 `header_match_required=false`，只记录识别结果，不因 Header 识别结果拒绝请求。
- 如果 Token 绑定应用 `header_match_required=true`：
  - Header 推断到绑定应用：允许。
  - Header 缺失或未推断出应用：拒绝。
  - Header 推断到其他应用：拒绝。
  - Header 推断到多个应用：拒绝。
- 如果 Token 未绑定应用：
  - Header 未推断出应用：允许继续，并记录识别结果。
  - Header 推断到任意应用或多个应用：拒绝。

规则冲突策略：

- 开启严格匹配的应用规则必须能在所有启用应用规则中唯一识别应用。
- 开启 `header_match_required` 的应用，其规则不得与任何启用应用规则重叠。
- 创建或更新应用 Header 规则、开启 `header_match_required`、启用应用时，都必须重新执行重叠校验。
- 禁用应用不参与请求期 Header 匹配；禁用状态下允许保存与启用应用重叠的规则。
- 禁用应用时，不应因它的 Header 规则重叠而阻止禁用；但重新启用时必须校验，如果会导致严格应用无法唯一识别，则拒绝启用并提示冲突应用。
- 两个应用规则集如果可能被同一个请求同时满足，则视为重叠。
- 在当前仅支持 `equals`、`one_of` 且同应用多规则为 AND 的前提下，只有当两个规则集存在至少一个共同 Header，且该 Header 的允许值集合无交集时，才能判定二者不重叠。
- 如果无法找到上述互斥证据，则视为可能重叠。例如 `Origin = https://a.example.com` 与 `X-Client-App = mobile` 可能被同一个请求同时满足，因此视为可能重叠。
- `observe` 模式可以记录 ambiguous，但不应因此拦截请求。
- `enforce` 模式下，Token 绑定应用未开启 `header_match_required` 时，可以记录 ambiguous，但不应因此拦截请求。

建议涉及文件：

- `service/application.go`
- `types/application_header_rule.go`
- `middleware/auth.go`
- `model/application.go`
- `dto/application.go`

### 6. 缓存、日志与查看入口

Header 识别在鉴权热路径上，必须做缓存。

缓存要求：

- 缓存内容至少包括启用应用、Header 规则、`header_match_required`。
- 应用创建、更新 Header 规则、更新严格匹配开关、启用、禁用、删除后必须失效。
- 可保留短 TTL 作为兜底。
- 不得在每个请求上查询全量应用表并逐条解析 JSON。

日志要求：

- 不新增独立 Header 识别明细日志表。
- 将识别结果写入现有请求日志结构化字段，例如 `logs.other.application_header_detection` 或等价请求上下文快照。
- 不得把 Header 推断应用写入主应用上下文字段 `application_id`、`application_key`、`application_name`；这些字段只表示 Token 绑定且已校验的授权应用。
- 至少记录：
  - `mode`
  - `checked`
  - `enforced`
  - `result`
  - `blocked`
  - `reason`
  - `matched_application_id/key/name`
  - `bound_application_id/key/name`
  - `ambiguous_application_ids`

查看入口：

- 复用现有日志页面。
- 日志详情中增加“应用 Header 识别”区块。
- 日志列表可增加识别结果、是否拦截、模式等筛选或标识。
- 系统设置的应用 Header 识别模式旁可提供“查看观察日志”入口，跳转到日志页并带筛选条件。

### 7. 系统设置：应用 Header 识别模式

在系统设置中增加 `ApplicationHeaderDetectionMode` 配置。

要求：

- 可选值为 `off`、`observe`、`enforce`。
- 默认值为 `off`。
- 配置变更应影响后续请求。
- 页面说明必须明确：
  - `off` 不执行 Header 应用识别。
  - `observe` 只记录识别结果，不拦截请求。
  - `enforce` 只对开启 `header_match_required` 的绑定应用执行严格拦截。

建议涉及文件：

- `model/option.go`
- `common` 或 `setting` 中现有系统选项相关文件
- `web/default/src/features/system-settings/**`

### 8. 默认前端应用管理页

在默认前端补齐应用管理页面。

要求：

- 新增管理员可访问的应用管理路由。
- `/applications` 重定向到 `/applications/list`。
- 在管理员侧边栏增加 `Applications` 菜单。
- 新增应用列表页、表格、创建/编辑抽屉。
- 支持以下操作：
  - 查看列表
  - 新建应用
  - 编辑应用
  - 启用/禁用应用
  - 删除应用

列表至少展示：

- 应用名称
- `app_key`
- 描述
- API Key 数量
- Header 严格匹配状态
- 状态
- 操作列

表单至少支持：

- `name`
- `description`
- `status`
- `sort_order`
- `header_validation_rules`
- `header_match_required`

建议涉及文件：

- `web/default/src/features/applications/api.ts`
- `web/default/src/features/applications/types.ts`
- `web/default/src/features/applications/components/applications-page.tsx`
- `web/default/src/features/applications/components/applications-table.tsx`
- `web/default/src/features/applications/components/applications-mutate-drawer.tsx`
- `web/default/src/routes/_authenticated/applications/route.tsx`
- `web/default/src/routes/_authenticated/applications/index.tsx`
- `web/default/src/routes/_authenticated/applications/list.tsx`
- `web/default/src/hooks/use-sidebar-data.ts`

### 9. API Key 前端表单集成

在现有 API Key 抽屉表单中新增应用选择器。

要求：

- 通过 `GET /api/application/self/all` 拉取启用中的应用。
- 下拉包含“无应用”选项。
- 编辑已有 API Key 时正确回填 `application_id`。
- 提交时把 `application_id` 带给后端。
- 类型、默认值、表单转换逻辑保持一致。

建议涉及文件：

- `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`
- `web/default/src/features/keys/lib/api-key-form.ts`
- `web/default/src/features/keys/types.ts`

### 10. 国际化

补齐应用管理相关文案到以下 locale：

- `web/default/src/i18n/locales/en.json`
- `web/default/src/i18n/locales/zh.json`
- `web/default/src/i18n/locales/fr.json`
- `web/default/src/i18n/locales/ja.json`
- `web/default/src/i18n/locales/ru.json`
- `web/default/src/i18n/locales/vi.json`

至少包括：

- 应用管理菜单
- 页面标题与描述
- 应用表格列名
- 创建/编辑/删除/启用/禁用提示
- API Key 绑定应用字段说明
- Header 规则、严格匹配开关、应用 Header 识别模式、日志详情区块说明

## 明确排除项

本次不要实现：

- 应用维度统计报表
- 应用维度计费规则后台
- 强制每个请求必须传 `x-app-id`
- 一个 Token 绑定多个应用
- 普通用户的应用管理后台
- 独立的 Header 识别明细日志表
- 应用维度 Header 命中率统计报表

## 需求级验证点

在公共注意项中的通用验证要求基础上，至少额外验证以下行为：

1. 管理员可以创建应用，且返回结果包含自动生成的 `app_key`。
2. 应用名称重复时，创建或更新被拒绝。
3. 已被 API Key 绑定的应用无法删除。
4. 普通用户创建或编辑 API Key 时，可以绑定启用中的应用，也可以选择不绑定。
5. 绑定不存在或禁用应用时，API Key 保存失败。
6. 请求未带 `x-app-id` 时，已绑定应用的 Token 仍可正常请求。
7. 请求带 `x-app-id` 且与绑定应用不一致时，请求被拒绝。
8. 请求带 `x-app-id` 但 Token 未绑定应用时，请求被拒绝。
9. 默认 `ApplicationHeaderDetectionMode=off`，升级后已配置 Header 规则也不会自动导致请求被拒绝。
10. `off` 模式下不执行 Header 应用识别，`x-app-id` 校验仍按既有规则生效。
11. `observe` 模式下执行 Header 应用识别并写入现有请求日志，但不因 Header 识别结果拒绝请求。
12. `enforce` 模式下，Token 绑定应用未开启 `header_match_required` 时，只记录识别结果，不因 Header 识别结果拒绝请求。
13. `enforce` 模式下，Token 绑定应用开启 `header_match_required` 且 Header 推断到同一应用时，请求通过。
14. `enforce` 模式下，Token 绑定应用开启 `header_match_required` 且 Header 缺失、未命中、推断到其他应用或推断到多个应用时，请求被拒绝。
15. `enforce` 模式下，Token 未绑定应用且 Header 未推断出应用时允许继续；Header 推断到任意应用或多个应用时请求被拒绝。
16. 禁用应用不参与 Header 匹配；Token 绑定禁用应用仍被拒绝。
17. Header 规则匹配满足 `equals`、`one_of`、多规则 AND、Header 名大小写不敏感、Header 值大小写敏感的定义；重叠判定遵循“能证明互斥才不重叠，否则视为可能重叠”。
18. 开启严格匹配的应用规则不得与任何启用应用规则重叠；创建或更新规则、开启严格匹配、启用应用时都会重新校验。
19. Header 识别热路径不再每次查询全量应用表；应用创建、更新 Header 规则、更新严格匹配开关、启用、禁用、删除后缓存及时失效。
20. 现有日志页或日志详情能查看应用 Header 识别结果，包括模式、绑定应用、推断应用、结果、是否拦截和原因。
21. 默认前端可以进入 `/applications/list`，并在管理员菜单中看到 `Applications`。
22. 前端可以配置全局 `off | observe | enforce` 模式、应用 Header 规则和应用级严格匹配开关。
23. 新增应用相关文案在所有已支持 locale 中都有对应条目。

## 完成标准

只有同时满足以下条件，才算完成：

1. 后端应用管理接口可用。
2. API Key 可以稳定绑定应用。
3. `x-app-id` 校验行为与绑定关系一致。
4. 应用 Header 来源识别支持 `off | observe | enforce`，默认 `off`。
5. 应用级 `header_match_required` 已贯通后端、前端和请求期校验。
6. Header 严格匹配只在全局 `enforce` 且绑定应用开启 `header_match_required` 时拦截。
7. Header 识别结果写入现有请求日志，并能在日志查看位置看到。
8. Header 识别热路径已缓存，并在应用配置变化后失效。
9. 默认前端具备应用管理入口与页面。
10. API Key 表单已集成应用选择器。
11. 系统设置已集成应用 Header 识别模式。
12. 多语言文案已同步。
13. 没有把范围扩展到新的应用统计、计费后台或独立 Header 明细日志表。
