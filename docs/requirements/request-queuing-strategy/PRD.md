# 请求排队策略 PRD

## 元信息

- 需求标识：`request-queuing-strategy`
- 文档目的：为 AI 或开发者完整实现按模型分队列、加权调度、优先级排队的请求调度系统提供可执行的产品与技术边界
- 范围说明：仅描述排队调度子系统本身；不包含现有限流逻辑的重写，不包含渠道负载均衡的改造

## 1. 背景

当前系统已有全局、分组、组织维度的模型请求限流。当请求触及 RPM 阈值时，系统立即返回 HTTP 429 拒绝请求。

业务上存在以下未满足的需求：

- 用户短时突发请求时，希望请求能排队等待而非直接报错。
- 系统负载过高时，需要有序调度而非无序争抢。
- 不同用户或组织有不同优先级，高优先级用户应更快获得服务。

本需求引入请求排队机制，作为现有限流的缓冲层：未超限的请求直接放行，超限的请求进入按模型分区的优先级队列等待调度，而非立即拒绝。

## 2. 目标

- 在现有模型请求限流中间件之后新增排队中间件，作为限流超限时的缓冲层。
- 按模型分队列，每个模型维护独立的优先级队列。
- 支持基于 Token 配置和公司（Company）配置组合计算请求优先级。
- 使用加权调度算法，高优先级请求获得更多调度机会，低优先级请求不会被完全饿死。
- 对流式请求（SSE）支持实时队列位置通知。
- 对非流式请求通过响应头返回初始队列位置。
- 支持可配置的队列超时，超时后返回明确错误。
- 提供队列状态监控 API 和管理界面。
- 在默认前端提供 Token 优先级配置、公司优先级配置、队列状态监控页面。

## 3. 非目标

- 不重写现有的全局/分组/组织级限流逻辑。
- 不改造渠道选择（Distribute）和负载均衡机制。
- 不实现排队请求执行失败后的重入队——现有 relay 内部多渠道重试已覆盖此场景。
- 不实现模型确定之前的排队——排队发生在模型解析和限流检查之后。
- 不实现跨模型的全局队列。
- 不实现基于部门（Department）维度的优先级——仅使用公司级优先级。
- 不做队列历史数据持久化或审计日志。
- 不做客户端主动取消排队请求的能力。
- 不覆盖 `/v1/realtime` WebSocket 路由。
- 不覆盖当前 `/pg` playground 路由；本期不为仅 `UserAuth` 的请求链路引入 Token 优先级语义。

## 4. 用户角色与适用范围

- **管理员**：配置队列全局参数、查看队列监控面板、为 Token 和公司配置排队优先级。
- **普通用户**：在使用 API 时自动受排队策略影响；可在 Token 管理中查看已配置的优先级。
- **API 客户端**：在流式响应中接收队列位置通知；可通过请求头指定排队超时。

### 4.1 请求范围

- 本期仅适用于当前使用 API Token 鉴权的 HTTP relay 请求链路，包括 `/v1` 下的 HTTP relay 路由和 `/v1beta` Gemini HTTP relay 路由。
- 不适用于 `/v1/realtime` WebSocket 路由。
- 不适用于 `/pg` playground 路由。

## 5. 核心概念

### 5.1 模型队列（ModelQueue）

系统为每个模型维护一个独立的优先级队列。队列内部按优先级（1-10）分为 10 个桶（bucket），同一桶内按入队时间 FIFO 排序。

### 5.2 优先级计算

请求的最终优先级由 Token 配置和公司配置组合得出：

```
最终优先级 = clamp(Token.queue_priority + Company.queue_priority - 5, 1, 10)
```

- Token 和 Company 的 `queue_priority` 默认值均为 5。
- Company 优先级来源于鉴权后用户所属的 Company，而不是客户端直接传入的 `company_id`。
- 当用户无公司归属时，公司优先级按默认值 5 计算。
- 最终优先级范围为 1-10，数值越高越优先。

### 5.3 加权调度

调度器使用加权随机选取策略：

- 每个优先级桶的权重为 `bucket_priority^2 * bucket_length`。
- 高优先级桶被选中的概率远高于低优先级桶。
- 低优先级桶仍有非零概率被选中，不会被完全饿死。
- 从被选中的桶内取出队头请求（同优先级 FIFO）。

### 5.4 队列位置通知

对 SSE 流式请求，在正式响应开始前插入自定义 SSE 事件：

```
event: queue
data: {"position":5,"estimated_wait_sec":12}
```

- `position` 表示当前排队位置，从 1 开始，0 表示即将执行。
- `estimated_wait_sec` 为预估等待秒数，基于近期吞吐量估算。
- 近期吞吐量以滚动 60 秒窗口内成功出队并继续执行的请求数估算；若窗口内无有效样本，则 `estimated_wait_sec` 返回 0。
- 位置变化时推送更新。
- 非流式请求仅通过响应头 `X-Queue-Position` 返回初始位置。

### 5.5 超时

请求在队列中等待超过指定时间后，返回 HTTP 408 并从队列中移除。

超时值按以下优先级取第一个有效值：

1. 请求头 `X-Queue-Timeout-Seconds`
2. Token 配置 `queue_timeout`
3. 模型队列配置 `queue_timeout`
4. 系统全局默认 `QueueDefaultTimeout`

- 请求头值必须是正整数；解析失败或值小于等于 0 时按“未提供”处理，继续取下一优先级。
- 若模型不存在单独的 `QueueConfig` 记录，则仍视为启用排队；该情况下模型级超时视为“未提供”，继续回退到全局默认。
- 最终生效超时值若大于 `QueueMaxTimeout`，则截断为 `QueueMaxTimeout`。

### 5.6 直通语义

排队中间件仅对触及限流阈值的请求生效。未超限的请求直接通过，不经过队列。这意味着队列是限流的"缓冲区"而非所有请求的必经之路。

## 6. 业务规则

### 6.1 入队条件

- 请求经过 `ModelRequestRateLimit` 中间件时，若被判定为超限，该中间件在上下文中标记 `queue_required = true`。
- `ModelRequestRateLimit` 在首次解析出模型名后，将规范化后的模型名写入上下文，供 `QueueMiddleware` 复用。
- 鉴权层在鉴权完成后将用户所属 `company_id` 写入上下文；若为空则按无公司处理。
- 上述上下文字段必须纳入统一的 typed context key 体系，通过公共上下文 helper 读写，不使用裸字符串临时键。
- `QueueMiddleware` 检测到此标记后执行入队逻辑。
- 若未标记，直接调用 `c.Next()` 放行。

### 6.2 队列容量

- 若模型不存在单独的 `QueueConfig` 记录，则默认仍参与排队；该模型的容量上限取系统设置 `QueueGlobalMaxSize`，当其为 0 时表示无限制。
- 管理员可通过模型队列配置覆盖 `max_queue_size`。
- 当队列已满时，新到达的请求不再入队，直接返回 HTTP 429，响应体中包含 `queue_full` 错误码。

### 6.3 调度触发

调度器通过以下两种机制触发：

- **事件驱动**：当同模型已有请求完成、排队请求新入队、请求超时移除，或相关配置发生变化时，立即触发一次调度重试。
- **定时兜底**：调度器以固定间隔（500ms）扫描所有模型队列，防止事件遗漏。
- 每次尝试调度前，必须按现有限流语义再次判断当前请求是否可继续执行；若仍超限，请求保持在队列中等待下次重试。

### 6.4 请求生命周期

```
入队 → 等待调度重试 → 限流复检通过 → 被选中 → 继续执行 Distribute → Relay → 完成/失败 → 触发下一轮调度重试
                     ↘ 限流复检未通过 → 继续等待
                     ↘ 超时 → 返回 408
                     ↘ 队列满 → 返回 429
```

### 6.5 超时响应格式

```json
{
  "error": {
    "message": "request timed out in queue after 60s (model: gpt-4, position was 2)",
    "type": "queue_timeout",
    "code": "queue_timeout"
  }
}
```

### 6.6 队列满响应格式

```json
{
  "error": {
    "message": "queue is full for model gpt-4 (max size: 100)",
    "type": "queue_full",
    "code": "queue_full"
  }
}
```

### 6.7 系统开关

- 全局配置 `QueueEnabled` 控制排队功能总开关。
- 关闭时，排队中间件跳过所有逻辑，请求触及限流仍按原有行为返回 429。
- 每个模型也可单独配置是否启用排队。
- 若模型不存在单独的 `QueueConfig` 记录，则默认视为启用，并回退到全局默认容量与超时配置。

## 7. 功能需求

### 7.1 排队中间件

在受支持的 HTTP relay 中间件链中，`ModelRequestRateLimit` 和 `Distribute` 之间新增 `QueueMiddleware`。

中间件行为：

1. 检查全局开关是否启用。
2. 检查当前模型是否启用排队。
3. 检查上下文中是否存在 `queue_required` 标记。
4. 若无需排队，直接放行。
5. 若需排队：
  - 从上下文读取 `ModelRequestRateLimit` 预先写入的规范化模型名，不重新定义另一套模型解析规则。
  - 从鉴权上下文读取 `company_id`；不接受客户端直接指定公司优先级来源。
  - 计算请求优先级。
  - 校验并确定超时时长，并应用 `QueueMaxTimeout` 上限。
  - 调用队列服务的入队接口；由 service 内部负责构建排队请求、解析生效配置并定位模型队列。
  - 启动超时计时器。
  - 对 SSE 请求发送初始队列位置通知。
  - 阻塞等待调度信号或超时。
  - 被调度后继续执行后续中间件和 relay。
  - 超时后返回 408。

`QueueMiddleware` 仅负责上下文提取、调用队列服务、处理 SSE/超时和继续放行；不直接维护全局队列 map、调度循环或配置缓存，也不直接操作 `ModelQueue` 等 service 内部结构。

### 7.2 调度器

全局运行一个调度器，负责：

- 监听请求完成、入队、超时移除等“调度重试”事件。
- 定时扫描队列。
- 对每个有待处理请求的模型队列先执行一次限流复检；仅当当前可继续执行时才执行加权选取。
- 通知被选中的请求继续执行。
- 更新队列中剩余请求的位置，触发 SSE 推送。
- 请求完成事件仅作为“触发重试”信号，不表示存在可直接转移的 RPM 令牌。

调度器、模型队列状态、位置更新和统计快照由 service 层统一维护；controller 与 middleware 通过 service 暴露的接口访问，不各自维护一份状态。

### 7.3 监控 API

新增管理员接口：

```text
GET /api/queue/status
GET /api/queue/status/:model
```

由于队列为进程内纯内存状态，该接口返回当前节点快照，不聚合其他节点；其 JSON 字段沿用常规 HTTP API 的 snake_case 风格，与系统设置持久化键命名无关。

当前节点状态接口返回：

```json
{
  "queue_enabled": true,
  "total_queued": 42,
  "models": {
    "gpt-4": {
      "queued": 15,
      "avg_wait_sec": 3.2,
      "max_wait_sec": 12,
      "throughput_rpm": 58,
      "max_queue_size": 100,
      "enabled": true,
      "buckets": {
        "10": 2, "9": 1, "8": 3, "7": 0, "6": 2,
        "5": 4, "4": 1, "3": 1, "2": 0, "1": 1
      }
    }
  }
}
```

单模型详情接口返回该模型在当前节点上的完整状态。

### 7.4 队列配置 API

新增管理员接口：

```text
GET    /api/queue/config
GET    /api/queue/config/:model
PUT    /api/queue/config/:model
DELETE /api/queue/config/:model
```

配置对象：

```json
{
  "model_name": "gpt-4",
  "enabled": true,
  "max_queue_size": 100,
  "queue_timeout": 300
}
```

`PUT` 为 upsert 语义：若该模型配置不存在则创建，存在则更新。

`GET /api/queue/config` 返回所有已配置模型的列表。

### 7.5 Token 优先级配置

在 Token 创建和编辑接口中新增以下字段：

- `queue_priority`：整数，1-10，默认 5。
- `queue_timeout`：整数，秒，默认 0 表示跟随系统默认。

在 Token 管理前端表单中新增对应输入控件。

### 7.6 公司优先级配置

在 Company 创建和编辑接口中新增以下字段：

- `queue_priority`：整数，1-10，默认 5。

在公司管理前端表单中新增对应输入控件。

## 8. 前端要求

### 8.1 队列监控页面

在管理员侧边栏新增 `Queue Monitor` 菜单项，位于 `Admin` 分组下。

要求：

- 路由：`/queue/monitor`。
- 展示全局队列开关状态。
- 展示每个有排队请求的模型的实时状态：排队数、平均等待时间、各优先级桶分布。
- 支持查看单个模型的详细队列信息。
- 页面应自动刷新（建议 5-10 秒间隔）。

### 8.2 队列配置页面

在管理员侧边栏新增 `Queue Config` 菜单项，位于 `Admin` 分组下。

要求：

- 路由：`/queue/config`。
- 展示所有已配置模型的队列参数列表。
- 支持新增模型队列配置。
- 支持编辑已有配置的 `enabled`、`max_queue_size`、`queue_timeout`。
- 支持删除配置。

### 8.3 Token 表单扩展

在现有 Token 创建/编辑抽屉中新增：

- 排队优先级下拉选择器（1-10，默认 5）。
- 排队超时输入框（秒，0 表示跟随系统默认，可选填）。

### 8.4 公司表单扩展

在现有公司创建/编辑表单中新增：

- 排队优先级下拉选择器（1-10，默认 5）。

### 8.5 多语言

新增排队相关文案到所有已支持语言包，至少覆盖：

- 队列监控菜单与页面标题
- 队列配置菜单与页面标题
- 队列状态指标名称（排队数、等待时间、吞吐量等）
- Token 和公司表单中的优先级和超时字段
- 队列错误提示（超时、队列满）

### 8.6 全局队列设置

管理员可配置的全局队列参数应复用现有后台的系统设置入口，不新增独立的全局队列设置页面。

要求：

- 在现有系统设置的请求限制区域补充队列相关字段。
- 至少可配置 `QueueEnabled`、`QueueDefaultTimeout`、`QueueMaxTimeout`、`QueueGlobalMaxSize`。
- 保存逻辑复用现有的系统设置存储与刷新机制。
- 系统设置持久化键沿用当前 `OptionMap` 的 CamelCase 风格；监控/配置 HTTP API 的 JSON 字段可继续使用 snake_case。

## 9. 数据模型

### 9.1 Token 表变更

新增字段：

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `queue_priority` | int | 5 | 排队优先级 1-10 |
| `queue_timeout` | int | 0 | 排队超时秒数，0=跟随系统默认 |

### 9.2 Company 表变更

新增字段：

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `queue_priority` | int | 5 | 排队优先级 1-10 |

### 9.3 新增 QueueConfig 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int64 | 主键 |
| `model_name` | varchar(128) | 模型名，唯一索引 |
| `enabled` | bool | 是否启用排队，默认 true |
| `max_queue_size` | int | 队列容量上限，0=无限制 |
| `queue_timeout` | int | 默认超时秒数，0=跟随全局 |
| `created_at` | int64 | 创建时间 |
| `updated_at` | int64 | 更新时间 |

默认不要求为每个模型预先创建记录；当某模型不存在 `QueueConfig` 记录时，仍视为排队启用，`max_queue_size` 回退到 `QueueGlobalMaxSize`，`queue_timeout` 视为未提供并继续回退到全局默认。

### 9.4 系统设置新增

| 设置键 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `QueueEnabled` | bool | true | 排队功能总开关 |
| `QueueDefaultTimeout` | int | 300 | 全局默认超时秒数 |
| `QueueMaxTimeout` | int | 3600 | 允许的最大超时秒数 |
| `QueueGlobalMaxSize` | int | 0 | 全局单队列默认容量上限，0=无限制 |

以上设置复用现有系统设置与 Option 存储机制，不新增独立的配置注册中心；持久化键命名沿用当前 `OptionMap` 的 CamelCase 约定，而非额外发明一套 snake_case 系统键。

## 10. 中间件位置

本期需纳入本需求的 HTTP relay 中间件链：

```
/v1 HTTP: RouteTag → SystemPerformanceCheck → TokenAuth → ModelRequestRateLimit → Distribute → Relay
/v1beta HTTP: RouteTag → SystemPerformanceCheck → TokenAuth → ModelRequestRateLimit → Distribute → Relay
```

变更后：

```
/v1 HTTP: RouteTag → SystemPerformanceCheck → TokenAuth → ModelRequestRateLimit → QueueMiddleware → Distribute → Relay
/v1beta HTTP: RouteTag → SystemPerformanceCheck → TokenAuth → ModelRequestRateLimit → QueueMiddleware → Distribute → Relay
```

`QueueMiddleware` 插入在 `ModelRequestRateLimit` 之后、`Distribute` 之前；其中模型名与公司信息需要由前置中间件显式写入上下文后再复用。

不在本期范围内：

- `/v1/realtime` WebSocket 子路由
- `/pg` playground 路由

## 11. 技术约束

- 队列调度器必须在内存中运行，不依赖外部消息队列。
- 队列数据为纯内存状态，进程重启后丢失（不持久化排队中的请求）。
- 多实例部署下，每个节点维护独立内存队列；监控 API、排队位置与调度公平性均以当前节点视角解释，不要求跨节点聚合。
- 新增数据模型必须纳入统一数据库自动迁移流程。
- 新增文案至少覆盖项目支持的前端语言文件（en, zh, fr, ja, ru, vi）。
- SSE 队列事件不得干扰现有流式响应协议。
- 排队中间件不得修改请求体或响应体中的业务数据。
- 请求完成事件仅作为调度重试信号，不得被实现为“释放并转移 RPM 令牌”。
- `QueueMiddleware` 不得假设 `company_id` 或 `model_name` 天然已在上下文中；两者都必须由前置流程显式写入并复用。

### 11.1 目录与职责边界

为贴合当前仓库风格，本需求建议按“薄 middleware、厚 service、轻 model”的方式落地：

- `constant/context_key.go`：定义新增的 typed context keys，例如用户公司、队列标记、队列模型名。
- `model/queue_config.go`：负责 `QueueConfig` 的持久化模型与 CRUD，不承载调度逻辑。
- `service/request_queue.go`：负责内存队列状态、入队/出队/移除/位置更新/状态快照等核心业务逻辑。
- `service/request_queue_scheduler.go`：负责调度重试循环、事件投递和启动入口。
- `setting/queue.go`：负责全局队列设置变量、校验和字符串编解码辅助。
- `middleware/model-rate-limit.go`：负责模型解析、现有限流判断、以及 queue 相关上下文标记写入。
- `middleware/queue.go`：作为薄适配层，读取上下文并调用队列服务，不直接维护全局队列状态。
- `controller/queue.go` 与 `dto/queue.go`：负责管理端 API 入参/出参与状态查询接口。
- `model/option.go`、系统设置 API 与默认前端系统设置页：沿用现有全局设置接入方式，不额外发明新的设置注册中心。
- `main.go`：显式启动队列调度器，与现有后台循环任务启动方式保持一致。

### 11.2 上下文约束

- 新增上下文字段必须通过统一的 typed context key 和公共 helper 读写。
- 不应落成裸字符串 `c.Set("company_id", ...)`、`c.Set("queue_model", ...)` 之类的临时键方案。

## 12. 验收标准

1. 全局开关关闭时，排队中间件不拦截任何请求，限流超限仍返回 429。
2. 全局开关开启且模型排队启用时，超限请求进入队列等待而非立即 429。
3. 未超限的请求直接通过，不经过队列。
4. 本期功能仅在受支持的 HTTP relay 路由生效，不改变 `/v1/realtime` 和 `/pg` 的现有行为。
5. 优先级计算正确：`clamp(Token.queue_priority + Company.queue_priority - 5, 1, 10)`。
6. 高优先级请求的平均等待时间显著低于低优先级请求。
7. 低优先级请求不会被完全饿死，在合理时间内仍能被调度。
8. SSE 流式请求在排队期间能收到 `event: queue` 位置通知。
9. 非流式请求响应中包含 `X-Queue-Position` 头。
10. 请求头超时值非法时按未提供处理；超过 `QueueMaxTimeout` 时按上限截断。
11. 请求在队列中超时后返回 HTTP 408，响应体包含 `queue_timeout` 错误码。
12. 队列满时新请求返回 HTTP 429，响应体包含 `queue_full` 错误码。
13. 管理员可在队列监控页面看到当前节点上各模型的实时排队状态。
14. 管理员可在队列配置页面为模型配置排队参数。
15. Token 和公司的排队优先级可在管理界面中配置。
16. 排队请求执行失败后直接返回错误，不会重入队。
17. 新增文案在所有已支持 locale 中都有对应条目。
18. 本次实现不重写现有限流逻辑，不改造渠道选择机制。
