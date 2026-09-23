# 渠道监控 V2 Admin API

本文对应当前项目的渠道监控管理接口。新增的卡片式监控页面（组件名 V3）复用 V2 数据接口，没有独立的 `/channel-monitor-v3` API。V2 从真实请求记录聚合数据，不主动向模型发起探活请求。

## 1. 地址与认证

接口前缀：`/api/v1/admin/channel-monitor-v2`。

本机部署地址：`http://127.0.0.1:18081`；外部调用请替换为自己的站点地址。

任选一种认证方式：

| 请求头 | 值 |
|---|---|
| `x-api-key` | 后台配置的 **管理员 API Key**，不是模型调用用的普通 API Key |
| `Authorization` | `Bearer <管理员登录获得的 JWT>` |

同时提供两种请求头时，优先校验 `x-api-key`。管理员 API Key 以系统首个管理员身份调用；JWT 以登录管理员身份调用，这会影响 `updated_by` 和 `is_self`。

以下示例需要 curl；配置编辑示例另需 jq。先在调用环境设置 `ADMIN_API_KEY`，不要把真实密钥写入文档或代码仓库。

```bash
BASE_URL='http://127.0.0.1:18081'
MONITOR_API="$BASE_URL/api/v1/admin/channel-monitor-v2"

curl --fail-with-body -sS "$MONITOR_API/config" \
  -H "x-api-key: $ADMIN_API_KEY"
```

使用 JWT 时，将认证头替换为 `-H "Authorization: Bearer $ADMIN_TOKEN"`。

## 2. 功能开关与模式

需要区分三个字段：

| 字段 | 所属接口 | 含义 |
|---|---|---|
| `channel_monitor_enabled` | `/api/v1/admin/settings` | 渠道监控全局开关 |
| `channel_monitor_mode` | `/api/v1/admin/settings` | `v1` 主动探活、`v2` 被动统计 |
| `enabled` | `/api/v1/admin/channel-monitor-v2/config` | V2 聚合和查询开关 |

访问规则：

- `GET /config`、`PUT /config`：全局开关开启即可；V1 模式也能预先配置 V2，V2 的 `enabled=false` 也不妨碍读取和修改配置。
- 其余六个数据接口：必须同时满足全局开启、模式为 `v2`、V2 配置 `enabled=true`。
- `PUT /config` 不会切换全局模式。

切换模式使用系统设置接口。该接口对未提交的设置保留原值；与下文 V2 配置的完整对象更新语义不同。

```bash
# 实际执行后会切换到真实请求统计，并停止 V1 主动探活。
curl --fail-with-body -sS -X PUT "$BASE_URL/api/v1/admin/settings" \
  -H "x-api-key: $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"channel_monitor_enabled":true,"channel_monitor_mode":"v2"}'
```

若 V2 的 `enabled` 为 false，再按第 5 节读取、修改并保存完整 V2 配置。首次启用需要等待后台聚合，接口成功并不代表历史数据已经全部补齐。本文只提供调用说明，不会自动更改线上模式。

## 3. 接口清单与响应格式

以下路径均相对于监控接口前缀。

| 方法 | 路径 | 用途 | 返回的 `data` |
|---|---|---|---|
| GET | `/config` | 读取配置 | `Config` |
| PUT | `/config` | 更新配置 | 更新后的 `Config` |
| GET | `/dimensions` | 平台、分组、模型选项 | `Dimensions` |
| GET | `/snapshot` | 概览和趋势 | `Snapshot` |
| GET | `/matrix` | 分组卡片和时间轴 | `Matrix` |
| GET | `/models` | 模型统计 | `{coverage, items: ModelRow[]}` |
| GET | `/errors` | 错误分类和明细样本 | `{coverage, items: ErrorRow[]}` |
| GET | `/users` | 用户请求量排行 | `{coverage, items: UserRow[]}` |

成功均返回 HTTP 200，JSON 包装格式如下：

```json
{"code":0,"message":"success","data":{}}
```

下文结构中的对象指 `data` 内部。没有 `page`、`page_size`、`total` 等分页字段；未实现自定义起止时间、按用户 ID 钻取、主动检测或强制重建聚合接口。即使返回 `can_drilldown=true`，也不代表这些接口接受 `user_id` 参数。

时间字段为 RFC 3339 时间字符串。空列表可能是 `[]` 或 `null`；可选字段可能省略，调用端应兼容。

## 4. 数据查询参数

适用于 `/dimensions`、`/snapshot`、`/matrix`、`/models`、`/errors`、`/users`。

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `range` | string | `90m` | 仅支持 `90m`、`24h`、`7d`、`30d` |
| `platform` | string，多选 | 不额外筛选 | 如 `openai`、`anthropic`；实际范围受配置中启用的平台限制 |
| `group_id` | 正整数，多选 | 不额外筛选 | 与配置中的分组范围取交集 |
| `model` | string，多选 | 不额外筛选 | 模型名称；`__other__` 表示合并后的其他模型 |
| `group_by` | string | `platform_group` | 仅 `/matrix` 使用 |

多选支持重复参数和逗号分隔，两种写法可以混用：

```text
?range=24h&platform=openai&platform=anthropic&group_id=1&group_id=2
?range=24h&platform=openai,anthropic&group_id=1,2
```

使用原始参数名，不要写成 `platform[]` 或 `group_id[]`。空值和首尾空格会被清理；查询平台名请使用配置中的小写值。未知平台或配置范围外的筛选通常返回空数据，而非参数错误。非法 `range`、非正整数 `group_id`、非法 `group_by` 返回 HTTP 400。

`/dimensions` 是目录接口：保留时间范围与配置范围，但不会因当前 `platform`、`group_id`、`model` 多选条件而缩小选项列表。

时间范围与显示粒度固定对应：

| range | 时间桶 | 显示桶数 |
|---|---|---|
| `90m` | 5 分钟（300 秒） | 18 |
| `24h` | 1 小时（3600 秒） | 24 |
| `7d` | 12 小时（43200 秒） | 14 |
| `30d` | 1 天（86400 秒） | 30 |

窗口按 UTC 时间桶边界对齐，采用 `[requested_start, requested_end)`；`requested_end` 可能略晚于当前时间，最后一个桶可能只覆盖部分时间。以响应的 `coverage` 判断实际覆盖情况。

## 5. 读取和更新配置

### GET /config

返回 `Config`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `version` | integer | 乐观锁版本号；更新时必须提交最新值 |
| `enabled` | boolean | V2 是否开启 |
| `refresh_interval_seconds` | integer | 刷新周期，60 或 300 秒 |
| `platforms` | array | 平台配置列表 |
| `platforms[].platform` | string | 平台标识，保存时转小写并清理空格；不能为空或重复 |
| `platforms[].enabled` | boolean | 是否纳入展示和查询 |
| `platforms[].models` | string[] | 空数组显示真实模型名；非空时保留选中模型名，其余合并到 `__other__`，不是禁止采集这些模型 |
| `group_ids` | integer[] | 空数组表示不限制分组；非空为正整数分组 ID，保存时去重排序 |
| `health_thresholds` | object | 健康度阈值，见下表 |
| `ignored_error_categories` | string[] | 不参与错误率评分的分类，仍显示在错误列表中 |
| `updated_at` | string | 服务端最后更新时间 |
| `updated_by` | integer，可省略 | 最后修改的管理员 ID |

健康度配置及代码回退默认值（已保存的实际值以 GET 为准）：

| 字段 | 默认值 | 单位／含义 |
|---|---|---|
| `minimum_sample` | 50 | 最小样本门槛；保存后范围 1～10000 |
| `warning_error_rate` | 0.05 | 警告错误率 |
| `critical_error_rate` | 0.20 | 严重错误率，不低于警告值 |
| `target_ttft_ms` | 3000 | 首 Token 延迟目标，毫秒 |
| `warning_ttft_ms` | 3000 | 首 Token 延迟警告值，毫秒 |
| `critical_ttft_ms` | 10000 | 首 Token 延迟严重值，毫秒 |
| `warning_cache_rate` | 0 | 缓存率低于此值时警告 |
| `critical_cache_rate` | 0 | 缓存率低于此值时严重，不高于警告值 |
| `error_weight` | 0.60 | 错误率权重 |
| `ttft_weight` | 0.20 | 首 Token 延迟权重 |
| `cache_weight` | 0.20 | 缓存率权重 |

比例使用小数，例如 5% 写成 `0.05`。建议权重非负且合计为 1。缓存率两个阈值均为 0 时，不因缓存未命中扣分。服务端会对缺失或部分越界阈值进行默认填充、修正，实际保存值以 PUT 响应为准。

### PUT /config

请求体为完整 `Config` 对象，不是局部 PATCH。未提交的布尔值、数组和阈值可能被清空或重置。`updated_at` 和 `updated_by` 由服务端生成。

推荐先读取配置，再修改所需字段并提交：

```bash
# 保存完整配置，编辑时保留 version。
curl --fail-with-body -sS "$MONITOR_API/config" \
  -H "x-api-key: $ADMIN_API_KEY" \
  | jq -e '.data | objects' > monitor-config.json

# 示例：开启 V2 配置并设为每 300 秒刷新，其余配置保持原值。
jq '.enabled = true | .refresh_interval_seconds = 300' \
  monitor-config.json > monitor-config-update.json

curl --fail-with-body -sS -X PUT "$MONITOR_API/config" \
  -H "x-api-key: $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  --data-binary @monitor-config-update.json
```

刷新间隔为 0／未提供时归一化为 300；除 60、300 外的其他非零值会返回 400。版本匹配则保存并将 `version` 加 1；版本过期或缺失通常返回 409。遇到 409，应重新 GET、合并修改后再提交，不要盲目覆盖。

可配置的错误分类：

```text
content_policy, authentication, context_limit, invalid_request,
model_unsupported, group_access, quota_or_balance, account_pool_unavailable,
rate_or_capacity, timeout, transport_or_stream, upstream_forbidden,
not_found, client_cancelled, upstream_5xx, internal, other
```

忽略分类会转小写、去重排序，未知分类会被移除。代码工厂默认忽略 `authentication`、`client_cancelled`、`content_policy`、`context_limit`、`group_access`、`model_unsupported`、`not_found`、`quota_or_balance`；提交 `[]` 表示不忽略任何分类，读取现有配置不会强制恢复这些默认值。

## 6. 各数据接口

### GET /dimensions

```bash
curl --fail-with-body -sS "$MONITOR_API/dimensions?range=24h" \
  -H "x-api-key: $ADMIN_API_KEY"
```

`data` 结构示例（所有示例值均为说明用途，不是线上实测数据）：

```json
{
  "platforms": [{"value":"openai","label":"openai","request_count":100}],
  "groups": [{"id":1,"name":"示例分组","platform":"openai","request_count":100}],
  "models": [{"value":"example-model","label":"example-model","platform":"openai","request_count":100}]
}
```

平台和模型选项均为 `{value, label, request_count, platform?}`，分组选项为 `{id, name, request_count, platform?}`。同名模型可能属于不同平台，应结合 `platform` 区分。

### GET /snapshot

```bash
curl --fail-with-body -sS --get "$MONITOR_API/snapshot" \
  -H "x-api-key: $ADMIN_API_KEY" \
  --data-urlencode 'range=90m' \
  --data-urlencode 'platform=openai'
```

返回 `{config, coverage, metrics, health, trend}`：

| 字段 | 含义 |
|---|---|
| `config` | 完整监控配置 |
| `coverage` | 查询窗口的聚合覆盖情况 |
| `metrics` | 所选范围内的汇总指标 |
| `health` | 汇总健康度 |
| `trend[]` | `{bucket_start, metrics, health}`，每个时间桶的趋势点 |

### GET /matrix

这是新卡片页的主要数据来源。

```bash
curl --fail-with-body -sS --get "$MONITOR_API/matrix" \
  -H "x-api-key: $ADMIN_API_KEY" \
  --data-urlencode 'range=90m' \
  --data-urlencode 'group_by=platform_group' \
  --data-urlencode 'group_id=1,2'
```

| group_by | 组织方式 | 行维度字段 |
|---|---|---|
| `platform` | 每个平台一行 | `platform` |
| `platform_group` | 每个平台／分组一行，默认 | `platform`、`group_id`、`group_name` |
| `platform_model` | 每个平台／模型一行 | `platform`、`model` |
| `platform_group_model` | 每个平台／分组／模型一行 | 以上全部 |

返回 `{group_by, coverage, items}`。`items[]` 为：

```typescript
{
  platform: string
  group_id?: number
  group_name?: string
  model?: string
  metrics: Metric
  health: Health
  buckets: Array<{ bucket_start: string; metrics: Metric; health: Health }>
}
```

行级 `metrics` 是所选范围的汇总；新卡片页取最新时间桶的指标，时间轴使用 `buckets`。不要把行级汇总误当作最新状态。配置中的分组即使暂时没有请求，也可能返回 `unknown` 状态；样本不足不等于上游故障。

### GET /models

```bash
curl --fail-with-body -sS "$MONITOR_API/models?range=24h&platform=openai" \
  -H "x-api-key: $ADMIN_API_KEY"
```

返回 `{coverage, items}`，每项为 `{platform, model, metrics, health}`。`model="__other__"` 是配置导致的模型合并项。

### GET /errors

```bash
curl --fail-with-body -sS "$MONITOR_API/errors?range=24h" \
  -H "x-api-key: $ADMIN_API_KEY"
```

返回 `{coverage, items}`，每项为：

| 字段 | 类型 | 含义 |
|---|---|---|
| `category` | string | 错误分类 |
| `count` | integer | 此分类错误次数 |
| `rate` | number | 此分类占全部错误的比例，分母不是全部请求 |
| `ignored` | boolean | 是否被排除出错误率评分 |
| `details` | array，可省略 | 管理员可见的错误明细样本，并非全部原始日志 |

`details[]` 包含 `platform?`、`model?`、`error_type?`、`status_code?`、`upstream_status_code?`、`message?`、`count`。错误列表先列未忽略分类，再列忽略分类，各组按次数降序。忽略分类仍计入这里的 `count` 和 `rate`。

### GET /users

```bash
curl --fail-with-body -sS "$MONITOR_API/users?range=24h" \
  -H "x-api-key: $ADMIN_API_KEY"
```

返回 `{coverage, items}`。每项包含 `user_id?`、`rank`、`email?`、`username?`、`display_label`、`is_self`、`can_drilldown`、`metrics`。

按请求量降序返回前 10 名，并可能额外附带当前管理员，因此最多 11 项。无请求的当前管理员可能以 `rank=0`、`display_label="Me"` 返回。管理员可见真实身份，`is_self` 表示当前认证身份。

用户错误分类未单独按用户聚合，因此用户指标中的忽略错误影响按窗口整体比例估算，不是逐用户精确分类扣减。

## 7. 公共数据结构

### Metric：指标

| 字段 | 类型 | 含义 |
|---|---|---|
| `success_requests` | integer | 成功请求数 |
| `error_requests` | integer | 错误请求数，包含被忽略分类 |
| `request_count` | integer | 成功加错误请求总数 |
| `input_tokens`、`output_tokens` | integer | 输入、输出 Token |
| `cache_creation_tokens`、`cache_read_tokens` | integer | 缓存写入、读取 Token |
| `token_count` | integer | 以上四类 Token 之和 |
| `rpm` | number | 每分钟请求量，按实际覆盖时长计算 |
| `tpm` | number | 每分钟 Token 量，按实际覆盖时长计算 |
| `error_rate` | number | 参与评分的错误数／请求总数，已扣除忽略分类 |
| `success_rate` | number | 真实成功请求数／请求总数 |
| `cache_rate` | number | 缓存读取 Token／（输入＋缓存写入＋缓存读取 Token） |
| `cache_rate_numerator`、`cache_rate_denominator` | integer | 缓存率分子、分母 |
| `ttft` | Latency | 首 Token 延迟 |
| `duration` | Latency | 总耗时 |
| `upstream_affected_requests` | integer，可省略 | 上游错误影响的请求数，部分管理员指标返回 |
| `upstream_attempt_count` | integer，可省略 | 上游尝试计数，部分管理员指标返回 |

`error_rate`、`success_rate`、`cache_rate` 都使用 0～1 的比例。**新页面“可用率”按 `1 - error_rate` 展示，不等同于 `success_rate`**：忽略部分错误分类后，两者可能不同。无样本时应结合 `health` 判断，不能仅凭 `1 - 0` 判定健康。

`Latency` 为 `{sample_count, p50_ms, p90_ms, p95_ms, avg_ms}`。无数据时延迟值为 `null`；分位数由直方图估计，单位毫秒。

### Health：健康度

| 字段 | 含义 |
|---|---|
| `overall`、`error_rate`、`ttft`、`cache` | `healthy`、`warning`、`critical` 或 `unknown` |
| `score`、`error_rate_score`、`ttft_score`、`cache_score` | 可选的 0～100 分；样本不足时可能省略 |
| `minimum_sample` | 评分的样本门槛 |
| `thresholds` | 本次实际采用的健康度阈值 |

建议直接使用服务端健康度结果，避免前端另算阈值产生差异。

### Coverage：覆盖范围

| 字段 | 含义 |
|---|---|
| `requested_start`、`requested_end` | 对齐后的查询窗口，右端不包含 |
| `coverage_start` | 已聚合数据的有效起点 |
| `data_through` | 已聚合数据推进到的时间 |
| `computed_at` | 聚合计算时间 |
| `aggregation_lag_seconds` | 聚合延迟秒数 |
| `coverage_complete` | 历史覆盖是否已到达查询起点；不保证末尾完全实时 |
| `bucket_seconds` | 时间桶长度 |
| `bootstrap` | 可选的历史回填进度 |

`bootstrap` 包含 `active`、`progress_percent`（0～100）、`covered_from`、`target_start`。进度面向 30 天产品窗口，不能把部分覆盖时的空数据直接解释为渠道不可用。尚未初始化的时间字段可能为 Go 零时间 `0001-01-01T00:00:00Z`。

## 8. 错误处理

客户端先检查 HTTP 状态，再解析 `code`、`reason`、`message`。当前认证中间件的 `code` 是字符串，业务响应中的 `code` 是整数。

| HTTP 状态 | 标识／消息 | 原因与处理 |
|---|---|---|
| 400 | `invalid group_id` | 分组 ID 不是正整数 |
| 400 | `invalid channel monitor v2 range` / `group_by` | 时间范围或分组方式非法 |
| 400 | `invalid channel monitor v2 config` | 配置格式或内容非法 |
| 401 | `UNAUTHORIZED`、`INVALID_ADMIN_KEY`、`INVALID_TOKEN`、`TOKEN_EXPIRED` 等 | 检查管理员凭据 |
| 403 | `FORBIDDEN` | JWT 用户不是管理员 |
| 403 | `reason=CHANNEL_MONITOR_DISABLED` | 全局监控或 V2 配置关闭 |
| 403 | `reason=CHANNEL_MONITOR_MODE_MISMATCH` | 当前不是 V2 模式 |
| 409 | `channel monitor v2 config was modified` | 配置版本冲突，重新读取合并 |
| 423 | `ADMIN_COMPLIANCE_ACK_REQUIRED` | 认证管理员尚未完成后台已有的合规确认流程 |
| 429 | 由面板限流中间件返回 | 降低频率后重试 |
| 500 | 服务端错误 | 检查服务日志和数据库／聚合状态 |

模式不匹配示例：

```json
{
  "code":403,
  "message":"channel monitor mode does not allow this operation",
  "reason":"CHANNEL_MONITOR_MODE_MISMATCH"
}
```

认证失败示例：

```json
{"code":"INVALID_ADMIN_KEY","message":"Invalid admin API key"}
```

## 9. 新卡片页接入建议

1. 管理集成先调用 `/config` 检查配置，确保全局和 V2 开关均满足查询条件。
2. 用相同筛选条件请求 `/snapshot` 和 `/matrix?group_by=platform_group`。
3. `matrix.items` 按 `platform` 分区，再按 `group_id` 展示卡片；最新 `buckets` 用于指标，完整 `buckets` 用于时间轴。
4. 按 `snapshot.config.refresh_interval_seconds` 刷新；用 `coverage` 展示更新时间、延迟和历史覆盖进度。
5. 新页面中的“用户倍率”来自另行调用的用户分组接口，不在 V2 监控 API 返回值中。

Admin 接口用于可信管理端。普通用户页面实际调用 `/api/v1/channel-monitor-v2/*`，由登录身份限制可见分组并脱敏；不要把管理员密钥放入普通用户页面。

## 10. 实现依据

- [路由与模式门禁](../backend/internal/server/routes/admin.go)
- [请求参数解析与 Handler](../backend/internal/handler/channel_monitor_v2_handler.go)
- [数据结构、配置校验与健康度](../backend/internal/service/channel_monitor_v2.go)
- [查询、版本控制与统计口径](../backend/internal/repository/channel_monitor_v2_repo.go)
- [管理员认证](../backend/internal/server/middleware/admin_auth.go)
- [系统设置的局部更新](../backend/internal/handler/admin/setting_handler_update.go)
- [前端接口封装](../frontend/src/api/channelMonitorV2.ts)
- [新卡片页面](../frontend/src/views/user/ChannelStatusV3View.vue)
