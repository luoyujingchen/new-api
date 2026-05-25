# Implementation Prompt

你正在当前仓库中实现"请求排队策略"功能。

请直接在当前仓库中完成实现，不要只输出方案。实现范围以同目录下的 `PRD.md` 为准，只实现本文档描述的需求范围，不要引入未明确列出的其他扩展能力。

实现前先阅读 [../common-implementation-notes.md](../common-implementation-notes.md)；本文只补充当前需求特有的实现范围、约束和验证点。

## 目标

在当前系统中补齐请求排队调度能力：

- 在 relay 中间件链中，`ModelRequestRateLimit` 和 `Distribute` 之间新增 `QueueMiddleware`。
- 实现按模型分区的优先级队列，支持加权调度。
- 优先级由 Token 配置和公司配置组合计算。
- 流式请求支持 SSE 实时队列位置通知。
- 提供队列状态监控 API 和配置 API。
- 在默认前端补齐队列监控页、配置页和表单扩展。

本期仅覆盖当前使用 API Token 鉴权的 HTTP relay 路由；不包含 `/v1/realtime` WebSocket 和 `/pg` playground 路由。

注意：本次只建立排队调度基础设施，不重写现有限流逻辑，不改造渠道选择机制，不做排队请求失败后的重入队。

## 推荐目录与职责拆分

为贴合当前仓库风格，本需求建议按“薄 middleware、厚 service、轻 model”的方式落地，而不是把排队引擎主体塞进 middleware。

- `constant/context_key.go`：只定义新增的 typed context keys，例如 `ContextKeyUserCompanyId`、`ContextKeyQueueRequired`、`ContextKeyQueueModelName`。
- `model/queue_config.go`：只负责 `QueueConfig` 的数据模型与 CRUD，不承载调度逻辑。
- `service/request_queue.go`：实现 `RequestQueueService` 单例、队列内存状态、入队/出队/移除/位置更新/状态快照等核心业务逻辑。
- `service/request_queue_scheduler.go`：实现调度重试循环、事件投递、启动入口 `StartRequestQueueScheduler()`。
- `setting/queue.go`：只负责全局队列设置变量、校验和字符串编解码辅助。
- `middleware/model-rate-limit.go`：负责解析模型、执行现有限流判断、按条件写入 queue 相关上下文标记。
- `middleware/queue.go`：作为薄适配层，读取上下文并调用 `RequestQueueService`；不要在这里维护全局队列状态。
- `controller/queue.go` + `dto/queue.go`：负责管理端 HTTP 入参/出参绑定；状态聚合来自 service，不在 controller 内部拼状态机。
- `router/relay-router.go`、`router/api-router.go`：只负责挂载 middleware 和 API 路由。
- `model/option.go`、`controller/option.go`、`web/default/src/features/system-settings/request-limits/rate-limit-section.tsx`：沿用现有全局设置接入方式，不额外发明新的设置注册中心。
- `main.go`：显式启动 `service.StartRequestQueueScheduler()`，与现有 `SyncOptions`、`SyncChannelCache`、周期任务启动方式保持一致。

## 必须实现的范围

### 1. 后端数据模型变更

#### 1.1 Token 表新增字段

在 `model/token.go` 的 `Token` 结构体中新增：

- `QueuePriority int` — 排队优先级 1-10，默认 5，`gorm:"default:5"`
- `QueueTimeout int` — 排队超时秒数，默认 0（跟随系统），`gorm:"default:0"`

确保新增字段纳入统一数据库自动迁移流程，且兼容 SQLite、MySQL、PostgreSQL。

除模型定义外，必须同步更新 Token 的创建、更新、返回契约，使这两个字段可以完整地“前端表单 → API 请求 → 数据库存储 → API 回填”。当前仓库中的 Token 更新存在手工字段拷贝，不要只改结构体。

#### 1.2 Company 表新增字段

在 `model/company.go` 的 `Company` 结构体中新增：

- `QueuePriority int` — 排队优先级 1-10，默认 5，`gorm:"default:5"`

确保新增字段纳入统一数据库自动迁移流程。

除模型定义外，必须同步更新 Company 的 DTO、service、controller 和前端类型，使 `queue_priority` 可绑定、可持久化、可回填、可展示。

#### 1.3 新增 QueueConfig 模型

创建 `model/queue_config.go`，包含以下字段：

- `Id int64` — 主键
- `ModelName string` — 模型名，唯一索引，最大长度 128
- `Enabled bool` — 是否启用排队，默认 true
- `MaxQueueSize int` — 队列容量上限，0=无限制
- `QueueTimeout int` — 默认超时秒数，0=跟随全局
- `CreatedAt int64` — 自动创建时间
- `UpdatedAt int64` — 自动更新时间

提供以下数据访问能力：

- 按 `model_name` 查询配置
- 查询全部配置
- Upsert（按 `model_name` 插入或更新）
- 按 `model_name` 删除

若某模型不存在 `QueueConfig` 记录，默认仍视为启用排队：容量上限回退到 `QueueGlobalMaxSize`，模型级 `QueueTimeout` 视为未提供并继续回退到全局默认超时。

纳入统一数据库自动迁移流程。

建议涉及文件：

- `model/token.go`
- `controller/token.go`
- `model/company.go`
- `dto/company.go`
- `service/company.go`
- `controller/company.go`
- `model/queue_config.go`

#### 1.4 上下文与契约同步

本需求依赖的关键上下文和契约面，需要一并打通：

- `constant/context_key.go`：新增 typed context keys，至少包括 `ContextKeyUserCompanyId`、`ContextKeyQueueRequired`、`ContextKeyQueueModelName`。
- `middleware/auth.go` 或等效鉴权上下文设置点：将鉴权用户所属的 `company_id` 写入上下文；若无公司则保持空值。
- `middleware/model-rate-limit.go` 或共享 helper：在首次解析请求模型后，将规范化模型名写入上下文，供 `QueueMiddleware` 复用。
- 所有新增上下文字段一律通过 `common.SetContextKey` / `common.GetContextKey*` 访问，不要落成裸字符串 `c.Set("company_id", ...)`、`c.Get("queue_model", ...)` 这种与当前 typed context key 约定相冲突的写法。
- Token 和 Company 的 API 返回结构、默认前端 TS 类型、表单默认值与提交转换逻辑，都要同步纳入新字段。

### 2. 系统设置

创建 `setting/queue.go`，包含以下全局变量：

- `QueueEnabled bool` — 默认 `true`
- `QueueDefaultTimeout int` — 默认 `300`
- `QueueMaxTimeout int` — 默认 `3600`
- `QueueGlobalMaxSize int` — 默认 `0`（无限制）

从系统设置表加载这些值，并在设置变更时刷新。参考现有 `setting/rate_limit.go` 的模式。

注意：当前仓库的全局设置入口是 `model.Option` + `common.OptionMap` + `controller/option.go`，不是一个单独的 `setting/setting.go` 注册中心。队列全局设置必须沿用这套方式接入。

系统设置持久化键与 Go 侧变量名保持同一套 CamelCase 命名，例如 `QueueEnabled`、`QueueDefaultTimeout`，与现有 `ModelRequestRateLimitEnabled` 风格一致；不要额外引入一套 snake_case 的持久化系统键。

超时解析规则必须明确实现为：

- 请求头 `X-Queue-Timeout-Seconds` 仅接受正整数。
- 请求头值无效或小于等于 0 时，按“未提供”处理，继续回退到 Token / 模型 / 全局默认值。
- 最终超时值若大于 `QueueMaxTimeout`，则截断为 `QueueMaxTimeout`。

建议涉及文件：

- `setting/queue.go`
- `model/option.go`
- `controller/option.go`

#### 2.1 默认前端 — 全局队列设置

管理员可配置的全局队列参数，应并入现有默认前端的 request limits / system settings 页面，而不是新增一套独立的全局设置页面。

至少补齐以下字段：

- `QueueEnabled`
- `QueueDefaultTimeout`
- `QueueMaxTimeout`
- `QueueGlobalMaxSize`

建议涉及文件：

- `web/default/src/features/system-settings/request-limits/rate-limit-section.tsx`
- 该设置页关联的默认值加载与保存 hooks / page 入口文件

### 3. 排队核心实现

#### 3.1 队列数据结构

创建 `service/request_queue.go`，实现以下核心类型：

**QueuedRequest**：表示一个排队中的请求，包含：

- 唯一 ID
- Token ID、公司 ID（可为空，表示用户未归属公司）
- 规范化后的模型名
- 优先级（1-10）
- 入队时间
- 超时时长
- `ready chan struct{}` — 调度器通知通道
- `ctx context.Context` / `cancel context.CancelFunc` — 超时控制
- 流式通知写入器（SSE 请求时非 nil）
- 当前排队位置（原子变量）

该结构由 service 内部创建和持有；middleware 只传递入队所需元数据，不直接 new 或管理 `QueuedRequest` 实例。

**ModelQueue**：按模型分区的优先级队列，包含：

- 模型名
- 互斥锁
- 10 个优先级桶（`map[int]*list.List`），每个桶为 FIFO 链表
- 队列当前大小和容量上限
- 总权重值（用于加权选取）

ModelQueue 需提供以下方法：

- `Enqueue(req *QueuedRequest) error` — 入队，检查容量
- `DequeueByWeight() *QueuedRequest` — 加权选取并出队
- `Remove(req *QueuedRequest) bool` — 超时时移除
- `Size() int` — 当前队列深度
- `BucketSizes() map[int]int` — 各桶深度
- `UpdatePositions()` — 更新所有请求的排队位置

**RequestQueueService**：全局队列服务，建议使用单例模式，包含：

- `map[string]*ModelQueue` — 按模型名索引
- `sync.RWMutex` — 保护 map
- 获取或创建模型队列的方法
- 全局队列启用状态检查
- 队列状态快照、统计和事件投递入口

不要把这些核心结构定义在 middleware 中；middleware 只应调用 service。

#### 3.2 加权调度算法

在 `ModelQueue.DequeueByWeight()` 中实现：

1. 计算每个非空桶的权重：`w = priority^2 * bucketLength`。
2. 加权随机选取一个桶（按权重比例）。
3. 从选中桶的队头取出最早的请求。
4. 更新总权重。
5. 返回该请求。

确保 `math/rand` 使用全局安全随机源或 `crypto/rand`。

#### 3.3 SSE 队列通知

对于流式请求，在排队等待期间通过 SSE 协议发送位置更新：

```
event: queue
data: {"position":N,"estimated_wait_sec":M}
```

- 入队时发送初始位置。
- 每次有请求出队时，更新剩余请求位置并推送通知。
- 被调度时发送 `position:0`。

SSE 写入器需要直接写入 `gin.Context.Writer`，使用标准 SSE 格式。注意在写入前检查连接是否已断开。

预估等待时间基于模型队列的近期吞吐量计算：`estimated_wait = position / recent_throughput`。

近期吞吐量定义为滚动 60 秒窗口内成功出队并继续执行的请求数；若窗口内没有有效样本，则 `estimated_wait_sec` 返回 `0`。

#### 3.4 调度器

创建 `service/request_queue_scheduler.go`，实现全局调度器：

- 监听“调度重试”事件（请求完成、请求入队、请求超时移除、队列配置变化等）。
- 固定间隔（500ms）定时扫描所有模型队列作为兜底。
- 对每个有待处理请求的模型队列，先按现有限流语义执行一次“当前是否可继续执行”的复检；仅在复检通过时才执行加权调度。
- 通知被选中的请求（关闭 `ready` channel）。
- 调度后触发所有剩余请求的位置更新。

注意：请求完成事件只是“触发重试”的信号，不表示存在可直接转移的 RPM 令牌；不要把本需求实现成令牌交接系统。

调度器应通过 `service.StartRequestQueueScheduler()` 在 `main.go` 中显式启动，保持与现有后台循环任务一致；不要在 `init()`、middleware 构造函数或首个请求到来时偷偷启动。

#### 3.5 ModelRequestRateLimit 集成

修改 `middleware/model-rate-limit.go`：

- 抽取共享的模型解析与“当前是否允许继续执行”判断逻辑，供 `ModelRequestRateLimit`、`QueueMiddleware`、`RequestQueueService` / scheduler 复用；不要在多个位置维护彼此漂移的解析规则。
- 当限流检查发现超限时，在 `gin.Context` 中设置 `queue_required = true`，写入规范化模型名，然后继续 `c.Next()` 进入 `QueueMiddleware`，而不是立即返回 429。
- 仅当全局排队开关启用且该模型排队已启用时，才设置此标记。
- 当限流检查通过时，保持原有放行和成功计数语义。
- 当排队未启用时，保持原有的直接 429 行为。

建议涉及文件：

- `constant/context_key.go`
- `middleware/auth.go`
- `service/request_queue.go`
- `service/request_queue_scheduler.go`
- `middleware/model-rate-limit.go`

### 4. 排队中间件

创建排队中间件函数 `QueueMiddleware()`，插入到受支持的 HTTP relay 中间件链中：

```
ModelRequestRateLimit → QueueMiddleware → Distribute
```

不要把它挂到 `/v1/realtime` WebSocket 或 `/pg` playground 路由上。

该中间件应保持“薄适配层”职责：

- 读取 typed context keys
- 调用 `RequestQueueService`
- 处理超时 / 队列满 / SSE 写出
- 在 `ready` 后继续 `c.Next()`

不要在这里直接维护全局 map、调度循环、配置缓存或统计窗口，也不要直接调用 `ModelQueue.Enqueue()` 这类 service 内部结构。

中间件逻辑：

1. 检查全局开关 `setting.QueueEnabled`。
2. 通过 `RequestQueueService` 或等效 helper 获取当前模型的生效排队配置；无 `QueueConfig` 记录时默认视为启用，并回退到全局默认容量/超时配置。
3. 检查上下文中的 `queue_required` 标记。
4. 若无需排队，直接 `c.Next()`。
5. 若需排队：
   - 从上下文获取 Token ID、公司 ID、规范化模型名；其中公司信息来自鉴权用户，不接受客户端直接传入。
   - 计算 Token 和公司的优先级，得出最终优先级。
   - 确定超时时长（请求头 > Token > 模型配置 > 全局默认），并应用 `QueueMaxTimeout` 上限。
   - 调用 `RequestQueueService` 的入队接口，由 service 内部构建 `QueuedRequest`、解析生效配置并完成入队；若队列满，返回 429 + `queue_full`。
   - 对 SSE 请求设置流式通知。
   - 阻塞等待 `ready` 或 `ctx.Done()`。
   - `ready` 到达：继续 `c.Next()`。
   - `ctx.Done()`（超时）：从队列移除，返回 408 + `queue_timeout`。
6. 请求执行完成后（无论成功失败），发送一次调度重试事件通知调度器。

建议涉及文件：

- `middleware/queue.go`
- `service/request_queue.go`
- `router/relay-router.go`（插入中间件）

### 5. 监控与配置 API

#### 5.1 监控接口

```text
GET /api/queue/status       — 全局队列概览
GET /api/queue/status/:model — 单模型队列详情
```

仅管理员可访问。返回当前节点上的实时队列状态数据；由于队列为进程内纯内存状态，不要求跨节点聚合。

#### 5.2 配置接口

```text
GET  /api/queue/config        — 所有模型队列配置
GET  /api/queue/config/:model — 单模型队列配置
PUT  /api/queue/config/:model — Upsert 模型队列配置
DELETE /api/queue/config/:model — 删除模型队列配置
```

仅管理员可访问。`PUT` 为 upsert 语义。

建议涉及文件：

- `controller/queue.go`
- `router/api-router.go`
- `dto/queue.go`
- `service/request_queue.go`

### 6. 默认前端 — 队列监控页

新增管理员可访问的队列监控页面。

要求：

- 路由：`/queue/monitor`。
- 在管理员侧边栏增加 `Queue Monitor` 菜单项。
- 展示全局排队开关状态。
- 展示各模型实时队列状态表格：排队数、平均等待时间、吞吐量、桶分布。
- 页面自动定时刷新（建议 5 秒间隔）。

建议涉及文件：

- `web/default/src/features/queue/monitor/api.ts`
- `web/default/src/features/queue/monitor/types.ts`
- `web/default/src/features/queue/monitor/components/queue-monitor-page.tsx`
- `web/default/src/routes/_authenticated/queue/monitor.tsx`
- `web/default/src/hooks/use-sidebar-data.ts`
- `service/request_queue.go`（状态快照来源）

### 7. 默认前端 — 队列配置页

新增管理员可访问的队列配置页面。

要求：

- 路由：`/queue/config`。
- 在管理员侧边栏增加 `Queue Config` 菜单项。
- 展示所有已配置模型的列表。
- 支持新增、编辑、删除模型队列配置。
- 表单字段：模型名、启用开关、队列上限、默认超时。

建议涉及文件：

- `web/default/src/features/queue/config/api.ts`
- `web/default/src/features/queue/config/types.ts`
- `web/default/src/features/queue/config/components/queue-config-page.tsx`
- `web/default/src/features/queue/config/components/queue-config-mutate-drawer.tsx`
- `web/default/src/routes/_authenticated/queue/config.tsx`

### 8. 默认前端 — Token 表单扩展

在现有 Token 创建/编辑抽屉表单中新增：

- 排队优先级下拉选择器（1-10，默认 5）。
- 排队超时输入框（整数秒，0=跟随系统默认，提供占位提示）。

要求：

- 与现有表单布局风格一致。
- 编辑时正确回填已有值。
- 提交时包含新字段。

建议涉及文件：

- `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`
- `web/default/src/features/keys/lib/api-key-form.ts`
- `web/default/src/features/keys/types.ts`
- `controller/token.go`

### 9. 默认前端 — 公司表单扩展

在现有公司创建/编辑表单中新增：

- 排队优先级下拉选择器（1-10，默认 5）。

建议涉及文件：

- `web/default/src/features/organizations/components/companies-mutate-drawer.tsx`
- `web/default/src/features/organizations/types.ts`
- `web/default/src/features/organizations/api.ts`
- `dto/company.go`
- `service/company.go`
- `controller/company.go`

### 10. 国际化

补齐排队相关文案到以下 locale：

- `web/default/src/i18n/locales/en.json`
- `web/default/src/i18n/locales/zh.json`
- `web/default/src/i18n/locales/fr.json`
- `web/default/src/i18n/locales/ja.json`
- `web/default/src/i18n/locales/ru.json`
- `web/default/src/i18n/locales/vi.json`

至少包括：

- 队列监控菜单与页面标题
- 队列配置菜单与页面标题
- 队列状态指标名称
- Token 表单优先级和超时字段
- 公司表单优先级字段
- 队列错误提示

## 明确排除项

本次不要实现：

- 现有限流逻辑的重写
- 渠道选择和负载均衡的改造
- 排队请求失败后的重入队
- 跨模型的全局队列
- 基于部门维度的优先级
- 队列历史数据持久化
- 客户端主动取消排队请求
- `/v1/realtime` WebSocket 路由排队
- `/pg` playground 路由排队
- Prometheus 指标暴露
- 独立的排队审计日志

## 需求级验证点

在公共注意项中的通用验证要求基础上，至少额外验证以下行为：

1. 全局排队关闭时，超限请求仍返回 429，不进入队列。
2. 全局排队开启且模型排队启用时，超限请求入队等待，不被立即拒绝。
3. 未超限请求直接通过，不经过队列。
4. 本期实现只在受支持的 HTTP relay 路由生效，不影响 `/v1/realtime` 和 `/pg` 的现有行为。
5. 优先级计算：`Token=8, Company=7 → 10`；`Token=3, Company=3 → 1`；`Token=5, Company=5 → 5`。
6. 高优先级请求的平均出队速度快于低优先级。
7. 低优先级请求不会被完全饿死。
8. SSE 流式请求在排队期间能收到 `event: queue` 事件，包含 `position` 和 `estimated_wait_sec`。
9. 非流式请求响应中包含 `X-Queue-Position` 头。
10. 请求头超时值非法时按未提供处理；超过 `QueueMaxTimeout` 时按上限截断。
11. 请求超时后返回 408 + `queue_timeout` 错误码。
12. 队列满时新请求返回 429 + `queue_full` 错误码。
13. `GET /api/queue/status` 返回当前节点上各模型的实时排队数据。
14. `PUT /api/queue/config/:model` 能正确创建和更新配置，`DELETE /api/queue/config/:model` 能正确删除配置。
15. 默认前端 `/queue/monitor` 页面可正常加载并显示队列状态。
16. 默认前端 `/queue/config` 页面可正常管理模型队列配置。
17. Token 和公司表单中的优先级字段可正确编辑和保存。
18. 新增文案在所有 6 个 locale 中都有对应条目。

## 完成标准

只有同时满足以下条件，才算完成：

1. 排队中间件正确插入受支持的 HTTP relay 中间件链，未超限请求不受影响。
2. 按模型分区的优先级队列和加权调度器工作正常。
3. SSE 队列通知对流式请求生效。
4. 超时和队列满的错误响应格式正确。
5. 监控 API 和配置 API 可用。
6. 默认前端具备队列监控页、配置页和表单扩展。
7. 多语言文案已同步。
8. 没有重写现有限流逻辑或渠道选择机制，且未把请求完成实现为 RPM 令牌转移。
