# 兼容性维护时间表

本页面记录 Komari 对历史数据库结构、主题字段、Agent 协议和 HTTP 接口的兼容性维护计划。除非发布说明另有说明，列出的计划移除项将在对应主版本中不再提供。

## 时间轴

### 1.3.0

计划移除以下数据库迁移：

- `0.x` 的 `client_infos` 和旧 `load_notifications` 表结构迁移。
- `1.0.x` 的 OIDC、消息发送器配置迁移。
- `1.1.x` 的 `configs` 到 `config_items` 配置迁移，以及旧 Ping Task `all_clients` 字段迁移。
- 旧监控记录表和旧时间格式的迁移支持。

计划移除 `/api/public` 中为旧主题保留的字段：

- `record_enabled`
- `record_preserve_time`
- `ping_record_preserve_time`

计划移除 /api/mjpeg_live 支持

### Agent v2-only

Agent v1 协议已正式退役，以下接口已移除：

- `GET /api/clients/report`
- `POST /api/clients/report`
- `POST /api/clients/uploadBasicInfo`
- `POST /api/clients/task/result`

Agent 现在必须使用 protocol/v2 和 `/api/clients/v2/rpc`，包括 `agent.report`、`agent.basicInfo`、`agent.pingResult` 和 `agent.taskResult`。

以下旧接口仍属于独立的兼容性清理项：

- `/api/records/*`
- `/api/clients`
- `/api/recent/:uuid`
