# go-lobby
go 轻量 匹配大厅后端

一个面向游戏后端场景的轻量级竞技大厅服务 Demo，支持玩家登录鉴权、1v1 匹配、房间状态同步、比赛结算与赛季排行榜，项目使用 Go 实现，并结合 Redis、MySQL、RabbitMQ、gRPC、Docker 与 Kubernetes

功能特性
玩家登录（JWT 鉴权）
1v1 匹配队列
自动创建对战房间
WebSocket 房间状态同步
比赛结果提交与幂等结算
赛季排行榜（TopN / 周边排名）
异步任务处理（结算 → 排行榜更新）

技术栈：
Go
Gin（HTTP API）
gRPC（内部服务通信）
MySQL（持久化）
Redis（缓存 / 队列 / 排行榜）
RabbitMQ（消息队列）
WebSocket
Docker / Docker Compose
系统架构

整体为简化的服务拆分结构：
API Service：对外提供 HTTP / WebSocket 接口
Core Service：处理匹配、房间、排行榜等核心逻辑
Worker Service：消费消息队列，执行异步任务

核心设计
Redis 使用
匹配队列：queue:season:{sid}:1v1
房间状态：room:{rid}:state
排行榜：leaderboard:season:{sid}（Sorted Set）

排行榜基于分数排序，支持：
TopN 查询
玩家当前排名
周边排名
匹配机制
玩家加入 Redis 队列
后台轮询或触发匹配
成功后创建 match 和 room
返回房间信息

房间同步

通过 WebSocket 实现：
初始状态下发（snapshot）
玩家事件上报（ready / action / end）
服务端广播房间状态
结算流程
客户端提交比赛结果
服务校验幂等 key
写入数据库
发送 MQ 事件
Worker 更新排行榜
一致性策略
数据库为最终数据来源
Redis 为读优化层
使用幂等 key 防止重复结算
排行榜采用异步更新（最终一致）
