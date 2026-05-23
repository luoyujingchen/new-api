# Implementation Prompt

你正在当前仓库中实现“子业务公司与部门管理”功能。

请直接在当前仓库中完成实现，不要只输出方案。实现范围以同目录下的 `PRD.md` 为准，只实现本文档描述的需求范围，不要引入未明确列出的其他组织扩展能力。

实现前先阅读 [../common-implementation-notes.md](../common-implementation-notes.md)；本文只补充当前需求特有的实现范围、交互要求和验证点。

## 目标

交付一套完整的管理员组织管理能力，包括：

- 公司管理
- 部门管理
- 用户组织归属维护
- 后台导航与页面入口
- 数据模型与自动迁移
- 中英文文案

## 必须实现的范围

### 1. 后端数据模型

新增或补齐以下模型与字段：

- `Company`
- `Department`
- `User.company_id`
- `User.department_id`

要求：

- 公司名唯一、公司编码唯一。
- 部门属于公司。
- 部门支持父子层级，最大 4 级。
- 部门维护 `level` 和 `path`，由后端自动计算。
- 新模型和字段必须接入统一迁移流程。

### 2. 后端接口

实现以下管理员接口：

```text
GET    /api/company
GET    /api/company/all
GET    /api/company/:id
POST   /api/company
PUT    /api/company/:id
DELETE /api/company/:id
PATCH  /api/company/:id/status
GET    /api/company/:id/users

GET    /api/department
GET    /api/department/all
GET    /api/department/tree
GET    /api/department/:id
POST   /api/department
PUT    /api/department/:id
DELETE /api/department/:id
POST   /api/department/:id/move
PATCH  /api/department/:id/status
GET    /api/department/:id/users

PUT    /api/user/:id/department
DELETE /api/user/:id/department
```

接口要求：

- 列表接口返回标准分页结构。
- 公司列表支持 `status` 过滤。
- 部门列表支持 `company_id` 和 `status` 过滤。
- `GET /api/department/tree` 必须要求 `company_id`。
- 公司列表与详情返回部门数、用户数。
- 部门列表与详情返回子部门数、用户数，并带公司/父部门摘要信息。

### 3. 后端业务规则

必须严格实现以下校验：

- 公司名称或编码重复时拒绝保存。
- 同一公司同一父部门下，部门名称不能重复。
- 父部门必须存在且属于同一公司。
- 部门最大深度为 4。
- 不能把部门移动到自己下面。
- 不能把部门移动到自己的子孙下面。
- 删除公司前，如果还有部门或用户，拒绝删除。
- 删除部门前，如果还有子部门或用户，拒绝删除。
- 设置用户部门时，如未传公司但传了部门，自动补齐部门所属公司。
- 传入的部门若不属于指定公司，拒绝保存。
- 清空用户归属时，应同时清空公司和部门字段。

### 4. 前端组织管理页面

实现管理员页面与路由：

- `/organizations`
- `/organizations/companies`
- `/organizations/departments`

要求：

- 非管理员访问返回或跳转 `403`。
- `/organizations` 默认跳转到公司页。
- 在后台侧边栏增加组织管理入口。
- 组织管理至少包含 `Companies` 与 `Departments` 两个页面。

#### 公司页

- 列表列：名称、编码、描述、部门数、用户数、状态、操作。
- 支持分页。
- 支持新建公司。
- 支持编辑公司。
- 支持启用/禁用公司。
- 支持删除公司。
- 支持从行操作进入该公司的部门页。

#### 部门页

- 列表列：名称、层级、所属公司、描述、用户数、状态、操作。
- 名称需按层级缩进。
- 列表顺序必须保持树形可读性，父部门始终位于其子部门之前。
- 支持分页。
- 支持通过 `company_id` 查询参数进入某个公司的部门视图。
- 在公司上下文下展示当前公司名，并允许清除筛选。
- 支持新建部门。
- 支持编辑部门。
- 支持启用/禁用部门。
- 支持删除部门。
- 需要提供从已有部门直接发起“添加子部门”的入口；当当前部门已位于第 4 级时，不提供该入口。

### 5. 前端表单与交互

#### 公司表单

字段：

- `name`
- `code`
- `description`
- `status`
- `sort_order`

#### 部门表单

字段：

- `company_id`
- `name`
- `parent_id`
- `description`
- `status`
- `sort_order`

规则：

- 创建时可选所属公司和父部门。
- 编辑时不允许切换所属公司。
- 父部门选项只展示当前公司内的部门。
- 编辑时父部门候选必须排除自己。
- 如果 UI 支持在编辑时改变父部门，则必须正确调用移动接口，不能让用户改了但后端无效。

### 6. 用户管理集成

在现有用户管理里补齐组织信息：

- 用户列表增加 `Company`、`Department` 两列。
- 用户编辑抽屉增加 `Company (Optional)`、`Department (Optional)` 两个字段。
- 切换公司时清空已选部门。
- 未选公司时禁用部门选择。
- 部门下拉只展示当前公司部门。
- 保存用户后，再调用用户组织归属接口完成设置或清空。

### 7. 文案

- 新增组织管理相关文案至少补齐 `web/default/src/i18n/locales/en.json` 与 `web/default/src/i18n/locales/zh.json`。

## 明确排除项

本次不要实现以下内容：

- 组织限流
- 组织计费
- 组织配额
- 普通用户组织自助页面
- 审批流或复杂组织权限系统

## 建议修改面

后端大概率会涉及：

- `model/`
- `dto/`
- `service/`
- `controller/`
- `router/api-router.go`

前端大概率会涉及：

- `web/default/src/hooks/use-sidebar-data.ts`
- `web/default/src/routes/_authenticated/organizations/`
- `web/default/src/features/organizations/`
- `web/default/src/features/users/`
- `web/default/src/i18n/locales/`

## 完成标准

只有同时满足以下条件，才算完成：

1. 管理员可以完成公司与部门的增删改查及启停。
2. 用户可以被挂到公司和部门，并可清空归属。
3. 删除保护与层级校验都生效。
4. 用户列表和用户编辑抽屉都能正确展示组织信息。
5. 非管理员无法访问组织管理功能。
6. 所有新增能力经过至少一轮有意义的验证。

## 需求级验证点

在公共注意项中的通用验证要求基础上，至少额外验证以下行为：

- 组织管理页面主内容区能正常渲染，不出现空白页。
- 公司页点击“查看部门”后，以站内导航进入部门页，不触发整页刷新。
- 部门列表父子顺序正确，父部门显示在子部门之前。
- 从部门行可以直接添加子部门，第 4 级部门不出现该入口。
