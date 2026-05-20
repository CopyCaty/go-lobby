# go-lobby

基于 Go 的多人在线地图扫雷 / 匹配大厅 Demo。

项目当前已经具备“登录鉴权 -> 匹配 -> 房间 -> 结算 -> 排行榜”的大厅闭环，下一阶段将在此基础上迭代为多人在线地图扫雷：玩家进入中国地图，在分层 Chunk 地图中自由探索；放大后进入 `128x128` 的最小玩法区块，每个格子代表 `1km x 1km`；多人可同时进入同一区块，通过 WebSocket 实时同步开格、标记、触雷和封闭状态。

## 多人地图扫雷目标

- MVP 只做中国地图，不接全球地图。
- 地图抽象为多级 Chunk 网格结构：
  - 缩小时展示聚合 Chunk 状态，例如探索度、封闭状态、活跃人数、危险程度。
  - 放大后展示小 Chunk 或具体扫雷格子。
  - 最小玩法区块固定为 `128x128`，每格代表 `1km x 1km`。
- 后端作为权威状态源：
  - 前端只提交玩家动作和渲染服务端结果。
  - 雷区、格子状态、封闭状态、得分和结算都由后端判定。
- 多人实时协作 / 竞争：
  - 玩家进入同一 Chunk 后加入该 Chunk 的 WebSocket 广播组。
  - 任意玩家开格、标记或触雷后，区块内其他玩家立即收到同步消息。
- 触雷封闭：
  - MVP 第一版触雷后封闭整个 `128x128` 区块。
  - 封闭期间该区块禁止继续开格。
  - 中国地图聚合层用红色高亮展示封闭区块。

## 已有能力与迭代方向

| 已实现（当前代码已有） | 多人扫雷迭代方向 |
| --- | --- |
| 用户注册、登录（JWT 鉴权） | 玩家进入地图前复用登录态 |
| 1v1 / 2v2 匹配队列（Redis） | 后续扩展 `mine_pvp_1v1`、`mine_pvp_2v2` |
| 匹配成功创建比赛与房间（MySQL + 内存房间状态） | PVP 扫雷复用 match / room 作为对局容器 |
| WebSocket 房间内状态推送（当前用于 ready 广播） | 新增 Chunk 级 WS 同步棋盘动作 |
| 比赛结果提交（API）-> RabbitMQ -> Worker 异步结算 | PVP 扫雷按区域占领结果写入结算链路 |
| 积分结算 + Redis 排行榜 TopN | 增加扫雷探索榜、PVP 胜场榜、占领面积榜 |
| `docker-compose` 一键启动 MySQL/Redis/RabbitMQ | 支撑本地多人联调 |
| `static/` 本地手工测试页（`/`） | 后续扩展中国地图、Chunk 缩放和扫雷网格 UI |

## MVP 迭代路线

### 阶段 1：开放地图探索闭环

- 登录后进入中国地图。
- 查询当前视野范围内的聚合 Chunk。
- 点击或缩放进入某个 `128x128` Chunk。
- 获取 Chunk 快照并渲染扫雷网格。
- 玩家通过 WebSocket 提交开格、标记等动作。
- 后端更新权威状态并广播给同区块玩家。

### 阶段 2：触雷封闭与聚合高亮

- 触雷后封闭整个 `128x128` 区块一段时间。
- 封闭状态写入 Redis 热状态，并在 MySQL 记录封闭摘要。
- 封闭区块在中国地图聚合层以红色高亮。
- 封闭过期后允许重新进入和继续探索。

### 阶段 3：PVP 扫雷模式

- 复用现有匹配系统新增模式：
  - `mine_pvp_1v1`
  - `mine_pvp_2v2`
- 玩家匹配后进入同一地图区块。
- 对局默认 5 分钟。
- 采用区域占领规则结算：
  - 玩家或队伍开出的安全格形成占领区域。
  - 触雷产生惩罚。
  - 时间结束后按占领面积、触雷次数等规则计算胜方。
- 胜负结果写入现有 `match/result` 与排行榜结算链路。

### 阶段 4：工程化增强

- Chunk 状态持久化与恢复。
- 玩家断线重连和快照补发。
- 操作序列号、幂等处理和乱序保护。
- 多实例部署下的 Chunk 所有权、消息路由和一致性。
- 观战、回放、赛季排行榜和更复杂匹配策略。

## 状态存储设计

- Redis 保存实时热状态：
  - 活跃 Chunk 棋盘状态。
  - 格子打开 / 标记 / 归属。
  - 雷区信息。
  - 在线玩家列表。
  - 操作序列。
  - 临时封闭状态和过期时间。
- MySQL 保存摘要和可恢复数据：
  - Chunk 元信息。
  - Chunk 探索统计。
  - Chunk 封闭记录。
  - PVP 对局结果。
  - 排行榜结算依据。
- RabbitMQ / Worker 继续用于异步结算：
  - PVP 对局结束后发布结算事件。
  - Worker 更新玩家积分、胜负统计和排行榜。

## 主要接口（摘要）

### 已有大厅接口

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

### 计划新增地图接口

- `GET /api/v1/map/chunks?level=&bbox=`
  - 获取指定视野范围内的聚合 Chunk 状态。
- `GET /api/v1/map/chunks/:chunk_id`
  - 获取单个 Chunk 摘要。
- `GET /api/v1/map/chunks/:chunk_id/snapshot`
  - 获取 `128x128` 区块快照。
- `GET /api/v1/ws/chunks/:chunk_id`
  - 进入 Chunk 实时同步通道。

## WebSocket 消息设计

### 客户端消息

- `chunk_join`
  - 进入 Chunk，请求服务端下发快照。
- `cell_open`
  - 打开某个格子。
- `cell_flag`
  - 标记或取消标记某个格子。
- `chunk_leave`
  - 离开 Chunk。
- `ping`
  - 心跳。

### 服务端消息

- `chunk_snapshot`
  - 当前 Chunk 的完整快照。
- `cell_updated`
  - 单个或多个格子状态变化。
- `mine_triggered`
  - 玩家触雷事件。
- `chunk_closed`
  - Chunk 进入封闭状态。
- `chunk_reopened`
  - Chunk 封闭过期并重新开放。
- `player_joined`
  - 玩家进入 Chunk。
- `player_left`
  - 玩家离开 Chunk。
- `error`
  - 动作失败或协议错误。

## 测试计划

- 单元测试：
  - Chunk 坐标换算。
  - 聚合状态计算。
  - 触雷封闭和过期恢复。
  - PVP 区域占领得分结算。
- 接口测试：
  - 登录后查询地图 Chunk。
  - 获取 Chunk 快照。
  - 封闭 Chunk 时禁止开格。
  - 封闭过期后允许重新进入。
- WebSocket 测试：
  - 多玩家进入同一 Chunk。
  - 玩家 A 开格后玩家 B 收到 `cell_updated`。
  - 触雷后所有玩家收到 `chunk_closed`。
- 前端手工测试：
  - 中国地图缩放查看聚合红色封闭状态。
  - 放大进入 `128x128` 网格。
  - 多浏览器登录不同玩家验证实时同步。

## 技术栈（当前用到）

- Go / Gin
- MySQL / Redis / RabbitMQ
- WebSocket
- Docker Compose

## 协作边界

- 当前仓库已有 Go 后端大厅能力，后续多人扫雷后端实现建议按 `handler -> service -> repository -> model -> dto` 分层继续扩展。
- Chunk 实时状态优先放 Redis，长期摘要和结算依据放 MySQL。
- 现有匹配 / 房间 / 结算 / 排行榜能力不要废弃，作为 PVP 扫雷模式的基础能力继续复用。
