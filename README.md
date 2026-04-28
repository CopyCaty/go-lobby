# go-lobby
Go 轻量匹配大厅（简历技术 Demo）

面向“登录鉴权 → 匹配 → 房间 → 结算 → 排行榜”的最小闭环 Demo：
- API（Gin）提供 HTTP / WebSocket 接口
- Redis 用于匹配队列与排行榜
- MySQL 持久化用户/比赛/积分
- RabbitMQ + Worker 做异步结算

## 已实现 / 待完善

| 已实现（当前代码已有） | 待完善（扩展方向） |
| --- | --- |
| 用户注册、登录（JWT 鉴权） | Refresh Token / 黑名单 / 踢下线等完善 |
| 1v1 / 2v2 匹配队列（Redis） | 更复杂匹配策略（分段/超时/并发一致性） |
| 匹配成功创建比赛与房间（MySQL + 内存房间状态） | 房间状态持久化、多实例一致性（扩展方向） |
| WebSocket 房间内状态推送（当前用于 ready 广播） | 更完整 WS 协议（snapshot/增量/重连/心跳） |
| 比赛结果提交（API）→ RabbitMQ → Worker 异步结算 | 重试/死信队列/可观测性等工程化完善 |
| 积分结算 + Redis 排行榜 TopN | 周边排名/分页/赛季等能力扩展 |
| `docker-compose` 一键启动 MySQL/Redis/RabbitMQ | 部署（K8s/服务拆分/gRPC）作为后续扩展点 |
| `static/` 本地手工测试页（`/`） | 更完整前端交互与自动化测试 |

## 主要接口（摘要）

- `POST /api/v1/users/register`
- `POST /api/v1/users/login`
- `GET /api/v1/me`
- `POST /api/v1/match/queue/join`
- `GET /api/v1/match/queue/status`
- `POST /api/v1/match/queue/cancel`
- `GET /api/v1/room/:id`
- `POST /api/v1/room/:id/ready`
- `GET /api/v1/ws/room/:id`（WebSocket）
- `POST /api/v1/match/result`
- `GET /api/v1/leaderboard`

## 技术栈（当前用到）

- Go / Gin
- MySQL / Redis / RabbitMQ
- WebSocket
- Docker Compose
