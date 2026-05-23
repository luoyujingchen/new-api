# Implementation Prompt

你正在当前仓库中实现“应用管理与 x-app-id 校验”功能。

请直接在当前仓库中完成实现，不要只输出方案。实现范围以同目录下的 `PRD.md` 为准，只实现本文档描述的需求范围，不要引入未明确列出的其他扩展能力。

实现前先阅读 [../common-implementation-notes.md](../common-implementation-notes.md)；本文只补充当前需求特有的实现范围、约束和验证点。

## 目标

在当前系统中补齐以下能力：

- 新增 Application 资源及后台管理。
- 为 Application 自动生成唯一 `app_key`。
- 允许 API Key 可选绑定一个启用中的应用。
- 在请求鉴权阶段校验 `x-app-id` 与绑定应用的一致性。
- 在默认前端补齐应用管理页面、路由、侧边栏入口和多语言文案。

注意：本次只是在系统中建立“按应用归因”的基础设施，不要扩展成新的应用统计报表或应用计费后台。

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

### 5. 默认前端应用管理页

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
- 状态
- 操作列

表单至少支持：

- `name`
- `description`
- `status`
- `sort_order`

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

### 6. API Key 前端表单集成

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

### 7. 国际化

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

## 明确排除项

本次不要实现：

- 应用维度统计报表
- 应用维度计费规则后台
- 强制每个请求必须传 `x-app-id`
- 一个 Token 绑定多个应用
- 普通用户的应用管理后台

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
9. 默认前端可以进入 `/applications/list`，并在管理员菜单中看到 `Applications`。
10. 新增应用相关文案在所有已支持 locale 中都有对应条目。

## 完成标准

只有同时满足以下条件，才算完成：

1. 后端应用管理接口可用。
2. API Key 可以稳定绑定应用。
3. `x-app-id` 校验行为与绑定关系一致。
4. 默认前端具备应用管理入口与页面。
5. API Key 表单已集成应用选择器。
6. 多语言文案已同步。
7. 没有把范围扩展到新的应用统计或计费后台。