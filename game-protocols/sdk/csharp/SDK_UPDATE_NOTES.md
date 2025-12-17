# C# SDK 更新说明

## 概览
C# SDK 已更新以支持新的二进制通信协议。旧的 `Envelope` 封装均已移除。

## 变更内容

### 1. 新增 `GameClient.Network` 命名空间
- **`Packet` 类**: 处理 16 字节二进制头部（Magic, Route, Flags, Sequence 等）。
- **`GameNetworkClient` 类**: 提供基于 WebSocket 的异步连接和消息收发。

### 2. 移除废弃文件
- Removed `Protos/Envelope.cs` - 不再使用。

### 3. 项目结构
- 新增 `GameClient.csproj` - 标准 .NET Standard 2.0 项目文件。

## 使用示例

```csharp
using GameClient.Network;
using Chat; // Your Protobuf namespace

var client = new GameNetworkClient();
client.OnConnected += () => Console.WriteLine("Connected!");

client.OnPacketReceived += (packet) => 
{
    Console.WriteLine($"Received packet: Route={packet.Route}");
    
    // 解析具体的 Protobuf 消息
    if (packet.Route == RouteType.Chat)
    {
         // 根据上下文判断是 Response 还是 Broadcast
         // 这里可以使用 protobuf 解析尝试
    }
};

await client.ConnectAsync("ws://localhost:8080/ws");

// 发送消息
var req = new ChatRequest 
{ 
    Base = new Common.MessageBase { GameId = "mmo", UserId = 1001 },
    Content = "Hello",
    Type = ChatRequest.Types.MessageType.Text
};

await client.SendAsync(RouteType.Chat, req);
```

## 注意事项

1. **Protobuf 文件**: `Protos/` 目录下的文件应保持与服务器端 `.proto` 定义一致。虽然客户端不需要读取路由字段（`TargetUserId`），但建议保持定义同步。
2. **字节序**: SDK 内部已处理大端/小端转换，确保与 Go 服务器（大端）兼容。
