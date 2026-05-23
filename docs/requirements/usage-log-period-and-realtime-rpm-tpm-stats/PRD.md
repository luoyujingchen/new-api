# 用量日志按时段与实时 RPM/TPM 统计 PRD

## 元信息

- 需求标识：`usage-log-period-and-realtime-rpm-tpm-stats`
- 文档目的：为 AI 或开发者完整实现该需求中的统计能力提供明确边界
- 范围说明：主范围是日志统计卡片中新增按时间段平均 RPM/TPM 与最近 1 分钟实时 RPM/TPM；同时包含一个组织 RPM 表单类型修正，作为附带稳定性修复处理

## 1. 背景

当前用量日志页面已经展示总体用量和基础统计，但 RPM、TPM 的展示口径不够清晰。

业务需要同时看到两类指标：

- 当前筛选时间段内的平均每分钟请求量与平均每分钟 token 量。
- 最近 1 分钟的实时请求量与实时 token 量。

这样管理员和用户可以同时观察长期平均趋势和短时瞬时压力。

## 2. 目标

- 在现有日志统计接口中新增 `realtime_rpm` 与 `realtime_tpm` 字段。
- 明确 `rpm` 与 `tpm` 的语义为“当前筛选时间段内的平均每分钟值”。
- 在现有用量日志统计卡片中新增“Realtime RPM”和“Realtime TPM”两个指标展示。
- 保持现有筛选体系不变，复用已有日志统计接口和页面结构。
- 附带修正组织 RPM 表单的 Zod input/output 类型与状态值归一化，避免表单状态类型不稳定。

## 3. 非目标

- 不新增独立的统计页面。
- 不新增新的统计接口路径。
- 不新增折线图、趋势图、对比图等可视化图表。
- 不改变日志列表筛选交互。
- 不扩展到 drawing logs 或 task logs 的额外指标。
- 不重构整套 usage logs 架构。

## 4. 用户角色与适用范围

- 管理员在后台公共日志页查看管理员视角统计。
- 普通用户在个人日志页查看个人视角统计。
- 两类用户都复用现有日志统计卡片组件与既有接口封装。

## 5. 核心指标定义

### 5.1 Quota

- 语义：当前筛选范围内的总配额消耗。
- 数据来源：消费日志的 `quota` 求和。

### 5.2 RPM

- 字段名：`rpm`
- 语义：当前筛选时间段内的平均每分钟请求数。
- 计算方式：
  - 在当前筛选条件下统计消费日志请求总数。
  - 用请求总数除以当前时间窗口分钟数。
  - 向下取整为整数。

### 5.3 TPM

- 字段名：`tpm`
- 语义：当前筛选时间段内的平均每分钟 token 数。
- 计算方式：
  - 在当前筛选条件下统计消费日志 `prompt_tokens + completion_tokens` 总和。
  - 用 token 总数除以当前时间窗口分钟数。
  - 向下取整为整数。

### 5.4 Realtime RPM

- 字段名：`realtime_rpm`
- 语义：最近 1 分钟内的请求总数。
- 这是一个“近 1 分钟绝对值”，不是平均值。

### 5.5 Realtime TPM

- 字段名：`realtime_tpm`
- 语义：最近 1 分钟内的 token 总数。
- 这是一个“近 1 分钟绝对值”，不是平均值。

## 6. 统计口径与过滤规则

### 6.1 日志类型范围

- 统计仅针对消费日志进行计算。
- 其他类型日志不参与本次指标计算。

### 6.2 平均值统计范围

平均 RPM/TPM 必须使用当前筛选条件中的以下维度：

- `username`
- `token_name`
- `model_name`
- `channel`
- `group`
- `start_timestamp`
- `end_timestamp`

### 6.3 实时值统计范围

实时 RPM/TPM 必须满足：

- 固定统计“当前时间往前最近 60 秒”的消费日志。
- 仍然沿用当前筛选中的非时间维度条件：
  - `username`
  - `token_name`
  - `model_name`
  - `channel`
  - `group`
- 不受当前页面 `start_timestamp` / `end_timestamp` 的限制。

也就是说，实时值与当前筛选对象保持一致，但时间窗口永远是最近 1 分钟。

### 6.4 时间窗口分钟数计算

- 当同时提供 `start_timestamp` 和 `end_timestamp`，且 `end_timestamp > start_timestamp` 时：
  - 时间窗口分钟数 = `(end_timestamp - start_timestamp) / 60`
- 当时间范围不完整、无效或未提供时：
  - 默认按 1 分钟处理

## 7. 功能需求

### 7.1 后端统计返回扩展

在现有日志统计数据结构中新增：

- `realtime_rpm`
- `realtime_tpm`

并明确：

- `rpm` = 当前筛选时段平均每分钟请求数
- `tpm` = 当前筛选时段平均每分钟 token 数

### 7.2 前端统计卡片扩展

在现有公共日志统计卡片中展示以下五个指标：

- `Usage`
- `Avg RPM`
- `Avg TPM`
- `Realtime RPM`
- `Realtime TPM`

要求：

- 保持原有卡片样式风格一致。
- 仍然支持敏感数据隐藏逻辑，其中 `Usage` 继续受隐藏开关控制。
- RPM/TPM 数值直接展示整数。

### 7.3 前端默认值扩展

当接口没有返回数据或接口失败时，默认统计对象必须包含：

- `quota = 0`
- `rpm = 0`
- `tpm = 0`
- `realtime_rpm = 0`
- `realtime_tpm = 0`

### 7.4 附带表单稳定性修复

在组织 RPM 配置表单中，补齐以下稳定性处理：

- 区分 Zod 的 `input` 与 `output` 类型。
- `defaultValues` 使用 input 类型。
- `useForm` 使用 `useForm<Input, unknown, Output>` 形式。
- `status` 在编辑回填时归一化为 `0 | 1`。
- 选择模型时空值处理更稳健。

这是提交中的附带修复，不属于日志统计主线功能，但应一并实现。

## 8. 接口范围

本次不新增接口路径，继续使用现有日志统计接口：

```text
GET /api/log/stat
GET /api/log/self/stat
```

请求参数继续沿用现有结构，例如：

- `type`
- `username`
- `token_name`
- `model_name`
- `start_timestamp`
- `end_timestamp`
- `channel`
- `group`
- `request_id`
- `upstream_request_id`

响应数据结构扩展为：

```json
{
  "quota": 0,
  "rpm": 0,
  "tpm": 0,
  "realtime_rpm": 0,
  "realtime_tpm": 0
}
```

## 9. 技术实现要求

### 9.1 后端

- 在现有日志统计模型或统计返回结构上扩展字段，不要新增平行结构。
- 配额、平均请求数、平均 token 数、实时请求数、实时 token 数应通过明确分开的查询逻辑计算。
- 所有查询继续使用现有日志表与过滤辅助逻辑。
- 出错时保持现有错误处理风格。

### 9.2 前端

- 复用现有 `CommonLogsStats` 组件。
- 复用现有 `getLogStats` / `getUserLogStats` API。
- 复用现有 `DEFAULT_LOG_STATS` 默认值对象。
- 新增字段要同步更新类型定义与默认值。

## 10. 验收标准

1. 管理员和普通用户的用量日志页都可以看到 `Avg RPM`、`Avg TPM`、`Realtime RPM`、`Realtime TPM`。
2. `rpm` 与 `tpm` 的计算口径是“当前筛选时段平均每分钟值”，不再是最近 60 秒绝对值。
3. `realtime_rpm` 与 `realtime_tpm` 始终表示最近 1 分钟绝对值。
4. 实时指标不受页面选择的开始/结束时间限制，但仍受模型、用户、渠道、令牌、分组等筛选影响。
5. 没有数据时，五个指标都能稳定显示为 0 或占位值，不出现 `undefined`。
6. 当只修改时间筛选时，平均 RPM/TPM 会随所选时间窗口变化。
7. 当最近 1 分钟内有请求但当前筛选时间范围很长时，平均 RPM/TPM 与实时 RPM/TPM 可以明显不同。
8. 组织 RPM 表单仍能正常打开、回填和提交，不因类型问题导致状态字段异常。
