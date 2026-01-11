# gRPC-Go 快速入门指南

## 目录
1. [项目结构](#项目结构)
2. [环境准备](#环境准备)
3. [构建步骤](#构建步骤)
4. [运行示例](#运行示例)
5. [代码解析](#代码解析)
6. [常用配置](#常用配置)

---

## 项目结构

```
demo/
├── go.mod              # Go 模块定义
├── README.md           # 本文档
├── proto/
│   ├── hello.proto     # Protocol Buffers 定义
│   ├── hello.pb.go     # 生成的消息代码
│   └── hello_grpc.pb.go # 生成的 gRPC 代码
├── server/
│   └── main.go         # 服务端实现
└── client/
    └── main.go         # 客户端实现
```

---

## 环境准备

### 1. 安装 Go

确保已安装 Go 1.21 或更高版本：

```bash
go version
# 输出示例: go version go1.21.0 darwin/amd64
```

### 2. 安装 Protocol Buffers 编译器

**macOS (使用 Homebrew)**:
```bash
brew install protobuf
```

**Linux (Ubuntu/Debian)**:
```bash
sudo apt update
sudo apt install -y protobuf-compiler
```

### 3. 安装 Go protobuf 插件

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

确保 `$GOPATH/bin` 在 PATH 中：
```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

---

## 构建步骤

### 1. 进入 demo 目录

```bash
cd vdocs/demo
```

### 2. 下载依赖

```bash
go mod tidy
```

### 3. 生成 gRPC 代码（可选）

如果需要重新生成 protobuf 代码：

```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/hello.proto
```

---

## 运行示例

### 1. 启动服务端

在一个终端中运行：

```bash
cd vdocs/demo
go run server/main.go
```

输出示例：
```
2024/01/06 10:00:00 ========================================
2024/01/06 10:00:00 gRPC 服务端启动成功
2024/01/06 10:00:00 监听地址: [::]:50051
2024/01/06 10:00:00 工作池大小: 8
2024/01/06 10:00:00 ========================================
```

### 2. 运行客户端

在另一个终端中运行：

```bash
cd vdocs/demo
go run client/main.go
```

或指定参数：

```bash
go run client/main.go -addr localhost:50051 -name "张三"
```

输出示例：
```
2024/01/06 10:00:01 正在连接服务端: localhost:50051
2024/01/06 10:00:01 ========================================
2024/01/06 10:00:01 gRPC 客户端启动成功
2024/01/06 10:00:01 服务端地址: localhost:50051
2024/01/06 10:00:01 ========================================
2024/01/06 10:00:01 ========== 测试 Unary RPC ==========
2024/01/06 10:00:01 [Unary] 发送请求: name=张三
2024/01/06 10:00:01 [Unary] 收到响应:
2024/01/06 10:00:01   - message: 你好, 张三! 来自 gRPC 服务端的问候。
2024/01/06 10:00:01   - timestamp: 1704520801000
2024/01/06 10:00:01   - 耗时: 1.234567ms

2024/01/06 10:00:01 ========== 测试 Server Streaming RPC ==========
2024/01/06 10:00:01 [Stream] 发送请求: name=张三
2024/01/06 10:00:01 [Stream] 收到消息 1:
2024/01/06 10:00:01   - message: 你好, 张三! 这是第1条消息。
...
```

---

## 代码解析

### Protocol Buffers 定义

```protobuf
// proto/hello.proto
syntax = "proto3";

package hello;

option go_package = "demo/proto;hello";

// 定义服务
service HelloService {
  // Unary RPC
  rpc SayHello (HelloRequest) returns (HelloReply) {}
  
  // Server Streaming RPC
  rpc SayHelloStream (HelloRequest) returns (stream HelloReply) {}
}

// 请求消息
message HelloRequest {
  string name = 1;
}

// 响应消息
message HelloReply {
  string message = 1;
  int64 timestamp = 2;
}
```

### 服务端实现关键点

```go
// 1. 实现服务接口
type helloServer struct {
    pb.UnimplementedHelloServiceServer
}

// 2. 实现 Unary RPC
func (s *helloServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
    return &pb.HelloReply{
        Message:   fmt.Sprintf("你好, %s!", req.GetName()),
        Timestamp: time.Now().UnixMilli(),
    }, nil
}

// 3. 创建并配置服务器
s := grpc.NewServer(
    grpc.NumStreamWorkers(uint32(runtime.NumCPU())),  // 工作池
    grpc.MaxConcurrentStreams(1000),                   // 最大并发流
)

// 4. 注册服务
pb.RegisterHelloServiceServer(s, &helloServer{})

// 5. 启动服务
s.Serve(lis)
```

### 客户端实现关键点

```go
// 1. 创建连接
conn, err := grpc.NewClient(addr,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)

// 2. 创建客户端存根
client := pb.NewHelloServiceClient(conn)

// 3. 发起 Unary RPC
reply, err := client.SayHello(ctx, &pb.HelloRequest{Name: "张三"})

// 4. 发起 Streaming RPC
stream, err := client.SayHelloStream(ctx, &pb.HelloRequest{Name: "张三"})
for {
    reply, err := stream.Recv()
    if err == io.EOF {
        break
    }
    // 处理响应...
}
```

---

## 常用配置

### 服务端配置选项

| **选项** | **说明** | **示例** |
|---------|---------|---------|
| `NumStreamWorkers` | 工作池大小 | `grpc.NumStreamWorkers(8)` |
| `MaxConcurrentStreams` | 最大并发流 | `grpc.MaxConcurrentStreams(1000)` |
| `MaxRecvMsgSize` | 最大接收消息大小 | `grpc.MaxRecvMsgSize(4<<20)` |
| `MaxSendMsgSize` | 最大发送消息大小 | `grpc.MaxSendMsgSize(4<<20)` |
| `KeepaliveParams` | Keepalive 参数 | 见下方示例 |
| `UnaryInterceptor` | Unary 拦截器 | `grpc.UnaryInterceptor(fn)` |
| `StreamInterceptor` | Stream 拦截器 | `grpc.StreamInterceptor(fn)` |

### Keepalive 配置示例

```go
// 服务端
grpc.KeepaliveParams(keepalive.ServerParameters{
    MaxConnectionIdle:     30 * time.Second,
    MaxConnectionAge:      5 * time.Minute,
    MaxConnectionAgeGrace: 10 * time.Second,
    Time:                  10 * time.Second,
    Timeout:               3 * time.Second,
})

// 客户端
grpc.WithKeepaliveParams(keepalive.ClientParameters{
    Time:                10 * time.Second,
    Timeout:             3 * time.Second,
    PermitWithoutStream: true,
})
```

### 服务配置 (Service Config)

```go
grpc.WithDefaultServiceConfig(`{
    "loadBalancingPolicy": "round_robin",
    "methodConfig": [{
        "name": [{"service": "hello.HelloService"}],
        "waitForReady": true,
        "retryPolicy": {
            "maxAttempts": 3,
            "initialBackoff": "0.1s",
            "maxBackoff": "1s",
            "backoffMultiplier": 2,
            "retryableStatusCodes": ["UNAVAILABLE"]
        }
    }]
}`)
```

---

## 故障排查

### 常见问题

1. **连接失败**
   - 检查服务端是否启动
   - 检查端口是否正确
   - 检查防火墙设置

2. **超时错误**
   - 增加 Context 超时时间
   - 检查网络延迟

3. **消息过大**
   - 增加 `MaxRecvMsgSize` / `MaxSendMsgSize`

### 调试工具

使用 `grpcurl` 测试服务：

```bash
# 安装 grpcurl
brew install grpcurl

# 列出服务
grpcurl -plaintext localhost:50051 list

# 调用方法
grpcurl -plaintext -d '{"name": "测试"}' \
    localhost:50051 hello.HelloService/SayHello
```

---

## 参考资料

- [gRPC-Go 官方文档](https://grpc.io/docs/languages/go/)
- [Protocol Buffers 文档](https://protobuf.dev/)
- [gRPC-Go GitHub](https://github.com/grpc/grpc-go)

