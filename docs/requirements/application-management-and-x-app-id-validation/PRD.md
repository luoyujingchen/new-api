# 应用管理与 x-app-id 校验 PRD

## 元信息

- 需求标识：`application-management-and-x-app-id-validation`
- 文档目的：为 AI 或开发者完整实现应用管理、API Key 绑定应用与请求期应用校验能力提供明确边界
- 范围说明：当前需求聚焦“应用实体 + 后台管理 + 令牌绑定 + x-app-id 校验 + Header 来源识别与严格匹配 + 前端入口与多语言文案”这一组应用归因与来源校验基础设施，不包含应用分析报表或独立计费页面

## 1. 背景

当前系统已经支持用户创建多个 API Key，但缺少“一个 API Key 代表哪个业务应用”的稳定标识。

业务上需要引入 Application 概念，解决以下问题：

- 管理员可以维护平台认可的应用清单。
- 每个应用有一个稳定的 `app_key`，可作为请求侧声明的应用标识。
- 用户在创建或编辑 API Key 时，可以把该 Key 绑定到某个应用。
- 当客户端显式携带 `x-app-id` 时，系统可以校验该请求是否确实来自该 API Key 绑定的应用。
- 当客户端携带管理员配置的来源 Header 时，系统可以推断请求来源应用，并在严格匹配开启时校验该推断结果是否与 Token 绑定应用一致，降低 Token 被挪用后的误用风险。

这一步的重点是建立应用归因基础设施，而不是立即做完整的应用统计面板。

## 2. 目标

- 新增 `Application` 资源模型，由系统自动生成唯一 `app_key`。
- 提供管理员可用的应用增删改查与启停能力。
- 在 API Key 创建/编辑流程中支持可选绑定应用。
- 仅允许绑定存在且已启用的应用。
- 在请求鉴权阶段支持读取并校验 `x-app-id` 请求头。
- 在请求鉴权阶段支持按配置进行应用 Header 来源识别。
- 支持全局 `off | observe | enforce` 模式，默认关闭，允许先观察再拦截。
- 支持应用级“要求 Header 严格匹配”开关。
- 在请求上下文中写入应用相关信息，供后续链路使用。
- 在默认前端中提供应用管理页面、菜单入口、路由与多语言文案。

## 3. 非目标

- 不新增“按应用统计”报表页。
- 不新增“按应用计费配置”管理页。
- 不要求所有请求都必须携带 `x-app-id`。
- 不支持一个 API Key 绑定多个应用。
- 不开放普通用户创建或管理应用。
- 不重构现有 API Key 体系或认证体系。
- 不把 Header 来源识别当作独立强认证机制；它是 Token 绑定应用之上的辅助来源校验。
- 第一阶段不新增独立的 Header 识别明细日志表。

## 4. 用户角色与适用范围

- 管理员：维护应用列表，查看应用基础信息与已绑定 API Key 数量。
- 普通用户：在创建或编辑 API Key 时，从“可选应用”列表里选择一个启用中的应用，或保持不绑定。
- API 客户端：在调用模型接口时可选传入 `x-app-id`，用于声明请求应用身份。
- API 客户端：在调用模型接口时可携带稳定自定义 Header，例如 `X-Client-App`，用于来源识别。Header 值必须由管理员配置规则后才会被系统用于应用推断。

## 5. 核心领域对象与规则

### 5.1 Application

Application 至少包含以下字段：

- `id`
- `app_key`
- `name`
- `description`
- `status`
- `sort_order`
- `header_validation_rules`
- `header_match_required`
- `created_at`
- `updated_at`

规则如下：

- `app_key` 由系统自动生成，必须全局唯一。
- `app_key` 创建后不可由用户编辑。
- `name` 为必填，长度上限 128，且在未删除记录中必须唯一。
- `description` 为可选，长度上限 500。
- `status` 仅允许 `0` 或 `1`，其中：
  - `1` = 启用
  - `0` = 禁用
- `sort_order` 为整数，用于排序。
- `header_validation_rules` 为可选 JSON 规则数组，用于从请求 Header 推断应用来源。
- `header_match_required` 为布尔值，表示该应用是否要求请求 Header 推断出的应用必须与 Token 绑定应用一致，默认 `false`。

### 5.2 API Key 与 Application 绑定

- `Token` 新增可空字段 `application_id`。
- 一个 API Key 最多绑定一个应用。
- API Key 可以不绑定应用。
- Token 绑定应用表示该 Token 的授权使用范围；绑定后，该 Token 只应被用于对应应用。
- 创建或编辑 API Key 时，如果提交了 `application_id`：
  - 对应应用必须存在。
  - 对应应用必须处于启用状态。
- 如果应用已经被 API Key 引用，则不允许删除该应用。

### 5.3 x-app-id 请求头

- 请求头名：`x-app-id`
- 值语义：应用的 `app_key`
- 该请求头为可选字段。

校验规则如下：

- 若请求未携带 `x-app-id`，则不因为缺少该头而拒绝请求。
- 若请求携带 `x-app-id`，但当前 API Key 未绑定应用，则请求必须被拒绝。
- 若请求携带 `x-app-id`，且其值与当前 API Key 绑定应用的 `app_key` 不一致，则请求必须被拒绝。
- 若 API Key 绑定的应用已被禁用，则请求必须被拒绝。
- 若 API Key 绑定的应用记录不存在，则请求必须被拒绝。
- 验证完成后，该请求头应仅用于本系统内部校验，不应继续作为原始业务头透传到后续上游链路。

### 5.4 应用 Header 来源识别与严格匹配

应用 Header 来源识别用于判断请求来源应用，并在严格匹配开启时拒绝没有许可或来源不一致的请求。

Token 绑定应用与 Header 推断应用的语义必须区分：

- Token 绑定应用表示授权范围，即该 Token 被允许用于哪个应用。
- Header 推断应用表示请求侧身份验证证据，即本次请求看起来来自哪个应用。
- Header 推断结果不能单独替代 Token 绑定授权关系。
- 某些第三方应用可能无法携带本系统指定的自定义 Header；这类应用不应开启 `header_match_required`，或应使用该第三方请求中天然稳定、可被管理员确认的 Header 作为规则来源。

#### 5.4.1 全局识别模式

新增系统选项 `ApplicationHeaderDetectionMode`，取值如下：

- `off`：完全不执行 Header 应用识别。
- `observe`：执行 Header 应用识别并写入请求日志，但不因为 Header 识别结果拒绝请求。
- `enforce`：执行 Header 应用识别；当 Token 绑定应用要求严格匹配时，按识别结果决定是否拒绝请求。

默认值必须是 `off`，避免升级后改变现有请求行为。

#### 5.4.2 应用级严格匹配开关

每个应用新增 `header_match_required`：

- `false`：该应用的 Header 规则只用于识别、观察和日志，不强制拦截。
- `true`：该应用要求请求 Header 推断出的应用必须与 Token 绑定应用一致。

#### 5.4.3 Header 规则

`header_validation_rules` 支持：

- `equals`：Header 值必须与配置值精确一致。
- `one_of`：Header 值必须在配置值列表中。

规则语义：

- Header 名大小写不敏感，存储与展示时使用标准 Header 名。
- Header 值大小写敏感，使用精确字符串比较。
- 同一应用配置多条规则时使用 AND 语义，即所有规则都必须满足才视为匹配该应用。
- 无规则应用不参与 Header 识别。
- 禁用应用完全不参与 Header 识别。
- 推荐使用稳定自定义 Header，例如 `X-Client-App`；不建议把 `Origin` 等容易被代理、浏览器或客户端环境影响的 Header 当作强安全凭据。

禁用应用规则处理：

- 禁用应用不参与请求期 Header 匹配。
- 禁用应用时，不因它的 Header 规则与其他应用重叠而阻止禁用。
- 禁用状态下允许保存与启用应用重叠的 Header 规则。
- 当禁用应用重新启用、开启 `header_match_required`，或更新规则会影响严格匹配唯一性时，必须重新校验；若会导致严格应用无法唯一识别，则拒绝操作并提示冲突应用。

规则重叠判定：

- 如果两个应用规则集可能被同一个请求同时满足，则视为重叠。
- 在当前仅支持 `equals`、`one_of` 且同应用多规则为 AND 的前提下，如果两个规则集存在至少一个共同 Header，且该 Header 的允许值集合无交集，则可判定二者不重叠。
- 如果无法找到上述“共同 Header 且值集合无交集”的互斥证据，则视为可能重叠。
- 示例：`Origin = https://a.example.com` 与 `X-Client-App = mobile` 可能被同一个请求同时满足，因此视为可能重叠。

#### 5.4.4 请求期判定

当全局模式为 `off`：

- 不执行 Header 应用识别。
- 不因为 Header 未命中、冲突、不一致拒绝请求。
- `x-app-id` 既有校验仍然生效。

当全局模式为 `observe`：

- 执行 Header 应用识别。
- 将识别结果写入现有请求日志或日志上下文。
- 不因为 Header 识别结果拒绝请求。
- Token 绑定应用禁用或不存在时仍按绑定应用校验拒绝。

当全局模式为 `enforce`：

- 执行 Header 应用识别。
- 如果 Token 绑定应用的 `header_match_required=false`，只记录识别结果，不因为 Header 识别结果拒绝请求。
- 如果 Token 绑定应用的 `header_match_required=true`：
  - Header 未推断出应用：拒绝请求。
  - Header 推断到其他应用：拒绝请求。
  - Header 推断到多个应用：拒绝请求。
  - Header 推断到 Token 绑定应用：允许继续。
- 如果 Token 未绑定应用：
  - Header 未推断出应用：允许继续，并记录识别结果。
  - Header 推断到任意应用：拒绝请求，因为当前 Token 没有绑定该来源应用的授权关系。
  - Header 推断到多个应用：拒绝请求。

#### 5.4.5 识别日志

第一阶段不新增独立日志表。Header 识别结果写入现有请求日志的结构化字段，例如 `logs.other.application_header_detection` 或等价请求上下文快照。

至少记录：

- `mode`：`off | observe | enforce`
- `checked`：是否执行识别
- `enforced`：本次是否执行严格拦截判断
- `result`：`skipped_off | skipped_no_rules | matched | unmatched | mismatch | ambiguous | blocked_unmatched | blocked_mismatch | blocked_ambiguous`
- `blocked`：是否拦截
- `reason`：未命中、冲突、不一致等原因
- `matched_application_id/key/name`
- `bound_application_id/key/name`
- `ambiguous_application_ids`

主应用上下文字段必须保持授权语义：

- `application_id`
- `application_key`
- `application_name`

以上字段只表示 Token 绑定且已校验的应用，不得写入仅由 Header 推断出的应用。

Header 推断结果必须使用独立字段，例如 `application_header_detection.matched_application_*` 或等价的 `detected_application_*`。即使 Token 未绑定应用但 Header 命中了应用，也不能把该推断应用写入主应用上下文字段。

查看入口复用现有日志页面：

- 请求日志列表可增加识别结果、是否拦截、模式等筛选或标识。
- 请求日志详情抽屉增加“应用 Header 识别”区块。
- 系统设置的模式开关旁可提供“查看观察日志”入口，跳转到日志页并带筛选条件。

只有当后续需要高效统计命中率、按天聚合或独立安全审计生命周期时，才考虑新增汇总表；不建议第一阶段新增每请求明细表。

## 6. 功能需求

### 6.1 后端应用管理接口

新增管理员接口组：

```text
GET    /api/application
GET    /api/application/all
GET    /api/application/:id
POST   /api/application
PUT    /api/application/:id
DELETE /api/application/:id
PATCH  /api/application/:id/status
```

要求如下：

- 这些接口仅管理员可访问。
- `GET /api/application` 支持分页查询：
  - 默认 `page=1`
  - 默认 `page_size=20`
  - `page_size` 最大为 100
  - 支持按 `status` 过滤
- 列表与详情响应应返回应用基础信息。
- 列表与详情应补充 `token_count`，表示当前绑定到该应用的 API Key 数量。
- 列表排序按 `sort_order ASC, id ASC`。
- 创建应用时系统自动生成 `app_key`。
- 更新应用时不得修改 `app_key`。
- 删除应用前必须检查是否已有 API Key 绑定：
  - 若存在绑定，返回“无法删除”的错误。
- `PATCH .../status` 只负责更新启用状态，且只接受 `0 | 1`。

### 6.2 面向普通用户的可选应用接口

新增用户接口：

```text
GET /api/application/self/all
```

要求如下：

- 该接口要求用户已登录。
- 仅返回 `status = 1` 的应用。
- 返回结果用于 API Key 表单下拉选择。
- 返回顺序与后台列表保持一致：`sort_order ASC, id ASC`。

### 6.3 API Key 创建与编辑

在现有 API Key 创建/编辑能力中新增 `application_id` 字段支持。

要求如下：

- API Key 表单应允许用户：
  - 不绑定应用
  - 绑定一个启用中的应用
- 创建 API Key 时要校验所选应用是否合法。
- 编辑 API Key 时也要执行同样的应用合法性校验。
- 若提交了不存在应用，返回“所选应用不存在”。
- 若提交了禁用应用，返回“所选应用已禁用”。
- 删除应用不会自动迁移或清空既有绑定，因为被引用的应用本就不允许删除。

### 6.4 请求鉴权与上下文注入

在令牌鉴权流程中新增应用上下文处理。

要求如下：

- 当 API Key 已绑定应用时，应先加载对应应用并校验其状态。
- 若应用可用，则将以下信息写入请求上下文：
  - `application_id`
  - `application_key`
  - `application_name`
- 以上上下文字段表示 Token 绑定且已校验的授权应用，不表示 Header 推断应用。
- Header 推断应用必须写入独立的识别结果字段，不得覆盖或伪造主应用上下文。
- 若客户端传入 `x-app-id`，则必须与绑定应用的 `app_key` 完全一致。
- 当校验失败时，按失败类型返回明确错误：
  - 绑定应用已禁用
  - 绑定应用不存在
  - 当前 API Key 未绑定应用，不能使用 `x-app-id`
  - `x-app-id` 与绑定应用不匹配

### 6.5 前端应用管理页面

在默认前端新增“应用管理”功能入口和页面。

要求如下：

- 仅管理员角色可访问该页面。
- 侧边栏 `Admin` 分组下新增 `Applications` 菜单项。
- 路由结构至少包含：
  - `/applications`
  - `/applications/list`
- `/applications` 应重定向到 `/applications/list`。
- 页面需要支持：
  - 查看应用列表
  - 新建应用
  - 编辑应用
  - 启用/禁用应用
  - 删除应用

列表至少展示以下列：

- 应用名称
- `app_key`
- 描述
- 已绑定 API Key 数量
- Header 严格匹配状态
- 状态
- 操作菜单

应用表单至少包含以下字段：

- `name`
- `description`
- `status`
- `sort_order`
- `header_validation_rules`
- `header_match_required`

其中：

- `name` 为必填。
- `app_key` 仅在列表或详情中展示，不出现在可编辑表单中。
- `header_validation_rules` 为可选 JSON 数组，支持 `equals` 和 `one_of`。
- `header_match_required` 使用开关控件展示，并提示只有全局模式为 `enforce` 时才会产生拦截效果。

### 6.6 API Key 表单前端交互

在现有 API Key 抽屉表单中新增应用选择器。

要求如下：

- 通过 `/api/application/self/all` 拉取可选应用。
- 下拉中必须包含“无应用”选项。
- 下拉仅展示已启用应用。
- 编辑已有 API Key 时，应用绑定值要能正确回填。
- 提交创建/更新时，把 `application_id` 一并发送给后端。

### 6.7 多语言

新增应用管理相关文案时，需要同步到默认前端已支持的语言包：

- `en`
- `zh`
- `fr`
- `ja`
- `ru`
- `vi`

至少覆盖以下类别文案：

- 应用管理菜单与页面标题
- 应用表格列名
- 新建/编辑应用表单提示
- 启用/禁用/删除成功失败提示
- API Key 绑定应用选择器文案
- Header 来源识别模式、Header 规则、严格匹配开关、日志详情区块文案

### 6.8 系统设置：应用 Header 识别模式

在系统设置中新增 `ApplicationHeaderDetectionMode` 配置入口。

要求如下：

- 可选值为 `off`、`observe`、`enforce`。
- 默认值为 `off`。
- 页面需要说明：
  - `off` 不执行 Header 应用识别。
  - `observe` 只记录识别结果，不拦截请求。
  - `enforce` 只会对开启 `header_match_required` 的绑定应用执行严格拦截。
- 配置变更应即时影响后续请求。

## 7. 接口与数据契约

### 7.1 Application 列表响应

分页查询的响应结构应包含：

```json
{
  "items": [
    {
      "id": 1,
      "app_key": "generated_app_key",
      "name": "Example App",
      "description": "optional",
      "status": 1,
      "sort_order": 0,
      "header_validation_rules": [],
      "header_match_required": false,
      "created_at": 0,
      "updated_at": 0,
      "token_count": 3
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

### 7.2 Application 创建/更新请求

创建/更新应用时，`name` 为必填；`description`、`status`、`sort_order`、`header_validation_rules`、`header_match_required` 按规则可选或使用默认值。

普通应用示例，不配置 Header 识别：

```json
{
  "name": "Example App",
  "description": "optional",
  "status": 1,
  "sort_order": 0
}
```

配置 Header 识别但不严格拦截：

```json
{
  "name": "Web App",
  "status": 1,
  "sort_order": 0,
  "header_validation_rules": [
    {
      "header": "X-Client-App",
      "operator": "equals",
      "value": "web"
    }
  ],
  "header_match_required": false
}
```

配置 Header 识别并要求严格匹配：

```json
{
  "name": "Web App",
  "status": 1,
  "sort_order": 0,
  "header_validation_rules": [
    {
      "header": "X-Client-App",
      "operator": "equals",
      "value": "web"
    }
  ],
  "header_match_required": true
}
```

### 7.3 API Key 请求体扩展

在现有 API Key 创建/编辑请求体中增加：

```json
{
  "application_id": 1
}
```

允许传 `null` 或不传，表示不绑定应用。

## 8. 技术实现要求

### 8.1 后端

- 令牌绑定校验应复用统一应用服务，不要在多个控制器或中间件里复制逻辑。
- 删除应用前必须先检查是否存在关联令牌，而不是依赖不明确的前端限制。
- Header 来源识别必须在鉴权热路径使用缓存，不得每个请求查询全量应用并逐条解析 JSON。
- 缓存内容至少包括启用应用、Header 规则、`header_match_required`。应用创建、更新、启用、禁用、删除后必须失效；可保留短 TTL 作为兜底。
- 开启 `header_match_required` 的应用，其 Header 规则不得与任何启用应用规则重叠，确保严格匹配应用可以被唯一识别。
- 创建或更新应用 Header 规则、开启 `header_match_required`、启用应用时，都必须重新执行重叠校验。
- `observe` 模式允许记录 ambiguous，不因 ambiguous 拦截请求。
- `enforce` 模式下，开启严格匹配的绑定应用如果遇到 unmatched、mismatch 或 ambiguous，必须拒绝请求并写入明确日志。

### 8.2 前端

- API Key 表单中的应用字段应同步更新类型、默认值、回填与提交转换逻辑。
- 管理页路由需要做管理员权限保护。
- 文案新增后要同步到所有已支持 locale。
- 应用表单中的 Header 规则继续使用 JSON 编辑即可，但必须有清晰说明、示例与校验错误提示。
- 系统设置页需要提供应用 Header 识别模式配置。
- 日志详情中需要展示应用 Header 识别结果，不要求第一阶段新增独立日志页面。

## 9. 验收标准

1. 管理员可以在后台看到 `Applications` 菜单，并进入应用管理页面。
2. 管理员可以创建应用，系统会自动生成唯一 `app_key`。
3. 应用名称重复时，创建或更新会被拒绝。
4. 管理员可以编辑应用的名称、描述、状态和排序，但不能编辑 `app_key`。
5. 管理员可以启用或禁用应用。
6. 已被 API Key 绑定的应用不能删除，并且会得到明确错误提示。
7. 普通用户创建或编辑 API Key 时，可以从启用中的应用列表中选择一个应用，也可以选择“不绑定应用”。
8. 若用户尝试绑定不存在或已禁用应用，创建或更新 API Key 会失败。
9. API Key 已绑定应用时，请求链路会把应用 id、key、name 写入上下文。
10. 当请求携带 `x-app-id` 且与绑定应用不一致时，请求会被拒绝。
11. 当请求携带 `x-app-id` 但当前 API Key 未绑定应用时，请求会被拒绝。
12. 当请求未携带 `x-app-id` 时，不会仅因缺少该头而拒绝请求。
13. 默认前端中的应用管理页、API Key 绑定应用选择器和相关提示文案在多语言环境下可正常显示。
14. 升级后默认 `ApplicationHeaderDetectionMode=off`，已配置 Header 规则也不会自动导致请求被拒绝。
15. `off` 模式下不执行 Header 应用识别，不因 Header 未命中、冲突或不一致拒绝请求，`x-app-id` 校验仍然生效。
16. `observe` 模式下执行 Header 应用识别，识别结果写入现有请求日志，但不因 Header 识别结果拒绝请求。
17. `enforce` 模式下，如果 Token 绑定应用未开启 `header_match_required`，仅记录 Header 识别结果，不因 Header 识别结果拒绝请求。
18. `enforce` 模式下，如果 Token 绑定应用开启 `header_match_required`，Header 推断到同一应用时请求通过。
19. `enforce` 模式下，如果 Token 绑定应用开启 `header_match_required`，Header 缺失、未命中、推断到其他应用或推断到多个应用时请求被拒绝。
20. `enforce` 模式下，Token 未绑定应用且 Header 未推断出应用时允许继续；Header 推断到任意应用或多个应用时请求被拒绝。
21. 禁用应用完全不参与 Header 匹配；Token 绑定禁用应用时仍拒绝。
22. Header 规则中 `equals`、`one_of`、多规则 AND、Header 名大小写不敏感、Header 值大小写敏感行为符合定义。
23. 开启严格匹配的应用规则不得与任何启用应用规则重叠；创建或更新规则、开启严格匹配、启用应用时都会重新校验。
24. Header 识别热路径不再每次查询全量应用表；应用创建、更新 Header 规则、更新严格匹配开关、启用、禁用、删除后缓存及时失效。
25. 现有日志页或日志详情能查看应用 Header 识别结果，包括模式、绑定应用、推断应用、结果、是否拦截和原因。
26. 前端可配置 Header 规则、应用级严格匹配开关和全局 `off | observe | enforce` 模式，所有新增文案完成 `en`、`zh`、`fr`、`ja`、`ru`、`vi` 翻译。
27. 本次实现不应额外出现新的应用统计报表、应用计费配置页、独立 Header 明细日志表或强制 `x-app-id` 输入流程。
