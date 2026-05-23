# Implementation Prompt

你正在当前仓库中实现“组织级共享时段 RPM 限流”功能。

请直接在当前仓库中完成实现，不要只输出方案。实现范围以同目录下的 `PRD.md` 为准，只实现本文档描述的需求范围，不要引入未明确列出的其他组织扩展能力。

实现前先阅读 [../common-implementation-notes.md](../common-implementation-notes.md)；本文只补充当前需求特有的实现范围、规则和验证点。

## 目标

交付一套面向公司和部门的共享 RPM 限流能力，包括：

- 组织限流规则数据模型
- 后台管理 API
- relay / playground 运行时生效逻辑
- 公司页与部门页的配置抽屉
- 时段、星期、模型范围、优先级匹配
- 缓存与缓存失效

## 必须实现的范围

### 1. 数据模型与迁移

新增并接入迁移：

- `OrganizationRateLimit`

字段至少包含：

- `org_type`
- `org_id`
- `model_id`
- `model_name`
- `time_slots`
- `rpms`
- `priority`
- `status`

要求：

- `org_type` 只允许 `company` 或 `department`。
- `time_slots` 与 `rpms` 数量必须一致。
- `time_slots` 使用 JSON 序列化。
- `rpms` 使用 JSON 序列化。
- 模型字段支持 `model_id`、`model_name` 双向补全与一致性校验。

### 2. 后端 API

实现以下管理员接口：

```text
GET    /api/rate-limit
POST   /api/rate-limit
GET    /api/rate-limit/:id
PUT    /api/rate-limit/:id
DELETE /api/rate-limit/:id
GET    /api/rate-limit/user/:id
```

要求：

- 列表查询按组织维度返回规则。
- 创建时验证组织存在。
- 更新只允许改 `time_slots`、`rpms`、`priority`、`status`。
- 单条详情返回组织名、模型名、时间段与 RPM。
- 支持查询某个用户当前命中的组织 RPM 规则。

### 3. 规则校验与匹配逻辑

必须实现以下规则：

- `weekdays` 仅允许 `0-6`。
- 时间必须是合法 `HH:MM`。
- 空 `weekdays` 表示每天生效。
- 支持跨天时段，例如 `23:00-02:00`。
- 跨天时段凌晨部分使用起始日 weekday 语义。
- RPM 必须大于等于 0。
- 某个命中时段的 RPM 若为 0，则不应作为有效组织规则命中。
- 精确模型规则优先于通配规则。
- 组织内规则按 `priority DESC, id ASC` 匹配。
- 当前部门优先于父部门，父部门优先于更高祖先。
- 部门链都不命中时，才回退到公司规则。
- 若仍未命中，继续保留原有 group/global 限流逻辑。

### 4. 运行时接入

将组织 RPM 规则接入现有 `ModelRequestRateLimit` 中间件。

要求：

- 在 playground 请求中生效。
- 在主要 relay v1 请求中生效。
- 在 Gemini relay 请求中生效。
- 命中组织规则时，覆盖原有 group/global 的 `totalMaxCount` 与 `successMaxCount`。
- 命中组织规则时，限流键必须从用户维度切换为组织共享维度。

组织共享键要求：

- 至少包含 `org_type`
- 至少包含 `org_id`
- 若规则绑定具体模型，还应包含模型名

### 5. 缓存

实现组织限流规则缓存。

要求：

- 缓存粒度按 `org_type + org_id`。
- 缓存只加载启用中的规则。
- 创建、更新、删除规则后必须使对应缓存失效。
- 提供全量清空缓存的能力用于测试。

### 6. 前端组织管理集成

在现有组织管理页面中补齐配置入口：

- 公司列表的操作菜单新增 `Configure RPM`
- 部门列表的操作菜单新增 `Configure RPM`
- 点击后打开组织 RPM 管理抽屉

抽屉要求：

- 展示当前组织名称与组织类型
- 列出已有规则
- 支持新建、编辑、删除规则

表单要求：

- 模型可选，支持 `All Models`
- 创建时可搜索或输入模型名
- 编辑时模型不可修改
- 支持 `priority`
- 支持 `status`
- 支持多个时间段
- 每个时间段有 `start_time`、`end_time`、`weekdays`、`rpm`
- 至少保留一个时间段

### 7. 前端 API 客户端与类型

在组织管理前端模块中补齐：

- 规则列表类型
- 规则详情类型
- 创建/更新请求类型
- 获取规则列表 API
- 获取规则详情 API
- 创建规则 API
- 更新规则 API
- 删除规则 API
- 查询用户当前生效规则 API

## 明确排除项

本次不要实现以下内容：

- 组织配额
- 组织计费
- 组织级审批
- 普通用户的组织 RPM 自助页面
- 复杂的生效链路可视化页面
- 完整的 group/global 生效详情查询页面

## 重要行为要求

实现时必须特别注意以下行为：

1. 当前部门规则必须优先于父部门规则，不允许仅凭 `priority` 覆盖层级优先级。
2. 精确模型规则必须优先于通配规则。
3. 未命中组织规则时，不是“无限制”，而是回退到原有 group/global 中间件逻辑。
4. 命中组织规则时，必须使用共享计数键，否则功能会退化成按用户独立限流。
5. 编辑规则时模型范围不可变更。
6. 跨天时段必须正确匹配到次日凌晨。

## 建议修改面

后端大概率会涉及：

- `dto/organization_rate_limit.go`
- `model/organization_rate_limit.go`
- `model/model_meta.go`
- `service/organization_rate_limit.go`
- `service/organization_rate_limit_cache.go`
- `controller/organization_rate_limit.go`
- `middleware/model-rate-limit.go`
- `router/api-router.go`
- `router/relay-router.go`

前端大概率会涉及：

- `web/default/src/features/organizations/api.ts`
- `web/default/src/features/organizations/types.ts`
- `web/default/src/features/organizations/components/companies-page.tsx`
- `web/default/src/features/organizations/components/companies-table.tsx`
- `web/default/src/features/organizations/components/departments-page.tsx`
- `web/default/src/features/organizations/components/departments-table.tsx`
- `web/default/src/features/organizations/components/organization-rate-limit-drawer.tsx`
- `web/default/src/features/organizations/components/organization-rate-limit-form-dialog.tsx`
- 相关前端 i18n 文件

## 需求级验证点

在公共注意项中的通用验证要求基础上，至少额外覆盖以下验证点：

- 跨天时段匹配
- 非法 weekday 校验
- 子部门优先于父部门
- 精确模型优先于通配规则
- 命中组织规则时中间件使用组织共享键
- 创建/更新/删除后的缓存失效

如可行，优先运行：

- `common/rate_limit_test.go`
- `model/organization_rate_limit_test.go`
- `service/organization_rate_limit_test.go`

## 完成标准

只有同时满足以下条件，才算完成：

1. 管理员可以为公司和部门配置共享 RPM 规则。
2. 规则能按时间段、星期、模型、优先级正确匹配。
3. 子部门优先、公司回退和 group/global 回退全部正确。
4. relay / playground 请求中实际生效。
5. 前端抽屉可以增删改查规则。
6. 缓存和测试都被正确处理。
