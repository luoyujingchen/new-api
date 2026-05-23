# Implementation Prompt

你正在当前仓库中实现“用量日志按时段与实时 RPM/TPM 统计”功能。

请直接在当前仓库中完成实现，不要只输出方案。实现范围以同目录下的 `PRD.md` 为准，只实现本文档描述的需求范围，不要引入未明确列出的其他扩展能力。

实现前先阅读 [../common-implementation-notes.md](../common-implementation-notes.md)；本文只补充当前需求特有的实现范围、附带修复和验证点。

## 目标

在现有用量日志统计体系中：

- 让 `rpm` / `tpm` 表示当前筛选时段内的平均每分钟值。
- 新增 `realtime_rpm` / `realtime_tpm` 表示最近 1 分钟绝对值。
- 在日志统计卡片中展示这两个新增指标。
- 顺带修复组织 RPM 表单的 Zod input/output 与状态值归一化问题。

## 必须实现的范围

### 1. 后端统计结构扩展

在现有日志统计返回结构中新增：

- `realtime_rpm`
- `realtime_tpm`

同时保留：

- `quota`
- `rpm`
- `tpm`

## 2. 后端统计逻辑

在现有日志统计函数中，按以下口径实现：

### 配额

- 统计消费日志的 `quota` 总和。

### 平均 RPM / TPM

- 在当前筛选条件下统计消费日志请求总数。
- 在当前筛选条件下统计 `prompt_tokens + completion_tokens` 总和。
- 若同时给出合法的 `start_timestamp` 和 `end_timestamp`，则用该时间窗口分钟数做除数。
- 否则默认以 1 分钟为窗口。
- 返回整数值。

### 实时 RPM / TPM

- 固定统计最近 60 秒内的消费日志。
- 仍然保留当前筛选中的非时间维度过滤：
  - `username`
  - `token_name`
  - `model_name`
  - `channel`
  - `group`
- 不使用页面传入的 `start_timestamp` / `end_timestamp` 作为实时值过滤条件。

## 3. 过滤与范围要求

- 统计仅针对消费日志。
- 沿用现有日志查询里的 contains/filter 辅助逻辑。
- 不新增新的统计接口路径。
- 继续复用现有：

```text
GET /api/log/stat
GET /api/log/self/stat
```

## 4. 前端展示

更新现有 `CommonLogsStats` 组件，展示：

- `Usage`
- `Avg RPM`
- `Avg TPM`
- `Realtime RPM`
- `Realtime TPM`

要求：

- 保持现有 badge 风格。
- `Usage` 继续受敏感信息开关控制。
- 其他数值直接展示整数。

## 5. 前端类型与默认值

同步更新 usage logs 相关类型与默认值：

- `LogStatistics`
- `DEFAULT_LOG_STATS`

默认值必须完整包含：

- `quota: 0`
- `rpm: 0`
- `tpm: 0`
- `realtime_rpm: 0`
- `realtime_tpm: 0`

## 6. 附带修复：组织 RPM 表单类型稳定性

同一个提交里还包含一个前端表单稳定性修复，也要一并实现：

- 对 Zod schema 使用 `z.input<typeof schema>` 和 `z.output<typeof schema>`。
- `defaultValues` 使用 input 类型。
- `useForm` 使用 `useForm<Input, unknown, Output>(...)`。
- 在组织 RPM 表单编辑回填时，将 `status` 归一化为 `0 | 1`。
- 处理模型选择空值时要更稳健。

这是附带修复，不要把它扩展成新的组织功能改造。

## 明确排除项

本次不要实现：

- 新的统计 API 路由
- 图表视图
- 额外的日志分类统计
- drawing/task logs 的新指标体系
- usage logs 页面重构

## 建议修改面

后端大概率会涉及：

- `model/log.go`

前端大概率会涉及：

- `web/default/src/features/usage-logs/components/common-logs-stats.tsx`
- `web/default/src/features/usage-logs/constants.ts`
- `web/default/src/features/usage-logs/types.ts`
- `web/default/src/features/organizations/components/organization-rate-limit-form-dialog.tsx`

## 需求级验证点

在公共注意项中的通用验证要求基础上，至少额外验证以下行为：

1. 当前筛选时间窗口变化后，`Avg RPM` 和 `Avg TPM` 会变化。
2. 最近 1 分钟无请求时，`Realtime RPM` 和 `Realtime TPM` 为 0。
3. 最近 1 分钟有请求时，`Realtime RPM` 和 `Realtime TPM` 反映最近 60 秒的绝对值。
4. 默认统计对象在空数据场景下不出现 `undefined`。
5. 组织 RPM 表单编辑时状态字段仍稳定为 `0 | 1`。

## 完成标准

只有同时满足以下条件，才算完成：

1. 日志统计接口返回平均 RPM/TPM 与实时 RPM/TPM。
2. 前端卡片正确展示五个统计指标。
3. 平均值与实时值的时间口径清晰且行为正确。
4. 默认值、类型定义和表单附带修复都已同步完成。
