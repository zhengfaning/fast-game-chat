# 时序图 (异步流程)

该图展示了 v2.0 架构中聊天消息的逻辑流转过程。

```plantuml  {kroki=true}
@startuml
skinparam ParticipantPadding 20
skinparam BoxPadding 10

actor "客户端" as Client
participant "网关 (Gateway)" as GW
queue "Redis 消息队列" as MQ
participant "聊天服务 (Chat Service)" as SVC
database "PostgreSQL 数据库" as DB

== 消息发送流程 ==

Client -> GW: 发送消息 (WebSocket)
activate GW

note right of GW: 1. 封装数据包\n2. 非阻塞处理

GW -> MQ: 发布 (PUBLISH) 到 "game:request:mmo"
activate MQ
deactivate GW

MQ -> SVC: 接收消息 (订阅者接收)
deactivate MQ
activate SVC

note right of SVC: **启动 Goroutine**\n并发处理业务逻辑

SVC -> SVC: 逻辑检查 (权限/过滤)

par 异步持久化 (Async Persistence)
    SVC -> SVC: 推送到 saveChan
    SVC -> DB: 插入数据 (INSERT) (后台工作线程)
end

SVC -> MQ: 发布 (PUBLISH) 到 "broadcast" (回复 + 路由信息)
deactivate SVC
activate MQ

MQ -> GW: 接收广播 (订阅者接收)
deactivate MQ
activate GW

GW -> GW: 查找会话 (本地 Session Map)

GW -> Client: 发送确认 (ACK) (WebSocket)
deactivate GW

== 消息广播流程 (私聊/群聊) ==

note over SVC: 逻辑判断目标对象\n(例如：接收者 ID)

SVC -> MQ: 发布 (PUBLISH) 到 "broadcast" (目标消息)
activate MQ

MQ -> GW: 接收广播
deactivate MQ
activate GW

GW -> GW: 查找接收者会话
GW -> Client: 推送消息 (WebSocket)
deactivate GW

@enduml
```

## 设计说明

1. **解耦与非阻塞**: 架构的核心在于消除了 Gateway 到 Chat Service 的直接同步调用。
   - 之前版本需要从连接池中 `Get()` 连接，在高并发下容易产生阻塞。
   - 当前版本通过消息队列发布请求，Gateway 可以在发布后立即处理下一个连接。
2. **消息队列抽象**: 虽然当前实现使用的是 Redis Pub/Sub，但系统代码已经对消息队列接口进行了通用抽象（`mq.Producer` 和 `mq.Consumer`）。
   - 这种设计允许我们在不修改核心业务逻辑的情况下，根据实际性能需求，轻松地将 Redis 替换为 **Kafka**、**RabbitMQ** 或其他专业级消息中间件。
3. **性能优势**: 这种异步模式极大地提高了系统的吞吐量，将单机平均延迟从秒级降低到了毫秒级（~2.2ms）。