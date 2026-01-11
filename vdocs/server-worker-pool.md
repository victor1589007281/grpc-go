# gRPC-Go 服务端工作池实现解析

## 目录
1. [工作池概述](#工作池概述)
2. [架构设计](#架构设计)
3. [实现原理](#实现原理)
4. [时序流程](#时序流程)
5. [函数调用链](#函数调用链)
6. [性能优化原理](#性能优化原理)

---

## 工作池概述

### 为什么需要工作池？

在 Go 语言中，每个 goroutine 都有自己的栈空间。当栈空间不足时，Go 运行时会调用 `runtime.morestack` 来扩展栈空间，这个操作涉及：

1. **栈复制**: 分配新栈空间并复制旧栈内容
2. **指针调整**: 更新所有栈上的指针
3. **性能开销**: 在高并发场景下可能成为瓶颈

工作池通过**复用 goroutine** 来避免频繁的栈分配和扩展，显著提升性能。

### 核心配置

| **配置项** | **默认值** | **说明** |
|-----------|-----------|---------|
| `numServerWorkers` | 0 (禁用) | 工作池中 worker 数量 |
| `serverWorkerResetThreshold` | 65536 (2^16) | Worker 重置阈值 |
| `maxConcurrentStreams` | MaxUint32 | 最大并发流数 |

---

## 架构设计

### 工作池整体架构

```mermaid
graph TB
    subgraph "**gRPC Server 工作池架构**"
        subgraph "**Server 核心组件**"
            S[**Server<br/>gRPC服务器**]
            WC[**serverWorkerChannel<br/>任务通道**]
            CLOSE[**serverWorkerChannelClose<br/>关闭函数**]
        end
        
        subgraph "**Worker 池**"
            W1[**Worker-1<br/>处理协程**]
            W2[**Worker-2<br/>处理协程**]
            W3[**Worker-3<br/>处理协程**]
            WN[**Worker-N<br/>处理协程**]
        end
        
        subgraph "**流处理**"
            ST[**ServerTransport<br/>传输层**]
            HS[**HandleStreams<br/>流处理器**]
            QUOTA[**handlerQuota<br/>流配额**]
        end
        
        subgraph "**任务分发**"
            F1[**处理函数-1**]
            F2[**处理函数-2**]
            FN[**处理函数-N**]
        end
    end
    
    ST --> HS
    HS --> QUOTA
    QUOTA --> F1
    QUOTA --> F2
    QUOTA --> FN
    
    F1 --> WC
    F2 --> WC
    FN --> WC
    
    WC --> W1
    WC --> W2
    WC --> W3
    WC --> WN
    
    S --> WC
    S --> CLOSE
    
    style S fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style WC fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CLOSE fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style W1 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style W2 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style W3 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style WN fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style ST fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style HS fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style QUOTA fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style F1 fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style F2 fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style FN fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
```

### Server 结构中的工作池字段

```mermaid
graph TB
    subgraph "**Server 结构体**"
        subgraph "**工作池相关字段**"
            OPTS[**opts.numServerWorkers<br/>Worker数量配置**]
            WC[**serverWorkerChannel<br/>chan func**]
            CLOSE[**serverWorkerChannelClose<br/>func**]
        end
        
        subgraph "**协程管理**"
            SWG[**serveWG<br/>Serve协程等待组**]
            HWG[**handlersWG<br/>Handler协程等待组**]
        end
        
        subgraph "**并发控制**"
            MCS[**maxConcurrentStreams<br/>最大并发流**]
            QUOTA[**handlerQuota<br/>处理器配额**]
        end
    end
    
    OPTS --> WC
    WC --> CLOSE
    SWG --> HWG
    MCS --> QUOTA
    
    style OPTS fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style WC fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CLOSE fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SWG fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style HWG fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style MCS fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style QUOTA fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
```

---

## 实现原理

### Worker 生命周期

```mermaid
graph TB
    subgraph "**Worker 生命周期**"
        START[**启动<br/>initServerWorkers**]
        WAIT[**等待任务<br/>阻塞在channel**]
        EXEC[**执行任务<br/>f**]
        COUNT[**计数检查<br/>completed++**]
        RESET[**重置检查<br/>是否达到阈值**]
        RESPAWN[**重生<br/>go serverWorker**]
        EXIT[**退出<br/>channel关闭**]
    end
    
    START --> WAIT
    WAIT -->|**收到任务**| EXEC
    WAIT -->|**channel关闭**| EXIT
    EXEC --> COUNT
    COUNT --> RESET
    RESET -->|**未达阈值**| WAIT
    RESET -->|**达到阈值**| RESPAWN
    RESPAWN --> EXIT
    
    style START fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style WAIT fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style EXEC fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style COUNT fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style RESET fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style RESPAWN fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style EXIT fill:#d7d7d7,stroke:#333,stroke-width:2px,color:#000
```

### Worker 重置机制

Worker 重置是为了解决 **goroutine 栈无法收缩** 的问题。

#### 问题根源：Go 栈管理机制

```mermaid
graph TB
    subgraph "**Go 栈生命周期问题**"
        subgraph "**栈增长过程**"
            G1[**初始栈<br/>2KB**]
            G2[**栈扩展<br/>runtime.morestack**]
            G3[**扩展后栈<br/>可能达到MB级**]
        end
        
        subgraph "**问题：栈不收缩**"
            P1[**处理大请求<br/>栈扩展到1MB**]
            P2[**后续小请求<br/>栈保持1MB**]
            P3[**内存浪费<br/>无法回收**]
        end
    end
    
    G1 -->|**栈不足**| G2
    G2 --> G3
    P1 --> P2 --> P3
    
    style G1 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style G2 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style G3 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style P1 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style P2 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style P3 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
```

#### Go 运行时源码层面分析

**Go 的栈管理特点**（参考 `runtime/stack.go`）：

1. **栈增长机制**：当函数调用时栈空间不足，触发 `runtime.morestack`
2. **栈复制策略**：分配更大的栈（通常2倍），复制旧栈内容，更新所有指针
3. **栈不收缩**：Go 运行时**不会主动收缩** goroutine 的栈空间

```go
// Go runtime 伪代码 - runtime/stack.go
func newstack() {
    // 当前栈不足时调用
    oldsize := gp.stack.hi - gp.stack.lo
    newsize := oldsize * 2  // 通常翻倍
    
    // 分配新栈
    new := stackalloc(newsize)
    
    // 复制旧栈到新栈
    memmove(new, old, oldsize)
    
    // 调整栈上的指针（开销大！）
    adjustpointers(...)
}
```

#### Linux 内核层面分析

**虚拟内存与物理内存**：

```mermaid
graph TB
    subgraph "**Linux 内存管理视角**"
        subgraph "**Go 栈 → 虚拟内存**"
            VA[**虚拟地址空间<br/>mmap 分配**]
            PA[**物理页框<br/>按需分配**]
        end
        
        subgraph "**为什么不自动收缩**"
            R1[**已分配物理页<br/>不会自动归还**]
            R2[**munmap 代价高<br/>需要页表操作**]
            R3[**碎片化风险<br/>频繁分配释放**]
        end
        
        subgraph "**重置的效果**"
            E1[**goroutine 退出**]
            E2[**整块栈内存释放<br/>munmap**]
            E3[**新 goroutine<br/>从小栈开始**]
        end
    end
    
    VA --> PA
    R1 --> R2 --> R3
    E1 --> E2 --> E3
    
    style VA fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style PA fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style R1 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style R2 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style R3 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style E1 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
    style E2 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
    style E3 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
```

**Linux 层面的内存释放**：

| **阶段** | **Linux 操作** | **说明** |
|---------|---------------|---------|
| goroutine 创建 | `mmap(PROT_READ\|PROT_WRITE)` | 分配虚拟内存区域 |
| 栈使用 | Page Fault → 分配物理页 | 按需分配物理内存 |
| 栈扩展 | `mmap` 更大区域 + `munmap` 旧区域 | 重新映射 |
| goroutine 退出 | `munmap` | 释放虚拟地址空间，物理页归还 |

#### 源码实现详解

```go
// server.go:649-671
// serverWorkerResetThreshold = 1 << 16 = 65536
// 
// 为什么是 65536？
// - 假设 QPS = 5000，每个 Worker 约 13 秒重置一次
// - 既不会太频繁（创建 goroutine 有开销）
// - 也不会让大栈存在太久（内存浪费）

func (s *Server) serverWorker() {
    for completed := 0; completed < serverWorkerResetThreshold; completed++ {
        f, ok := <-s.serverWorkerChannel
        if !ok {
            return  // channel 关闭，Worker 退出
        }
        f()  // 执行任务，可能导致栈扩展
    }
    // 关键：启动新 Worker，当前 Worker 退出
    // - 当前 goroutine 的大栈被 GC 回收
    // - 新 goroutine 从 2KB 小栈开始
    go s.serverWorker()
}
```

#### 重置机制的效果图

```mermaid
sequenceDiagram
    participant OLD as **旧Worker<br/>栈已扩展到1MB**
    participant NEW as **新Worker<br/>初始栈2KB**
    participant GC as **Go GC**
    participant OS as **Linux内核**
    
    Note over OLD: **已处理 65535 个请求<br/>栈从2KB扩展到1MB**
    
    OLD->>OLD: **1. completed == 65536**
    OLD->>NEW: **2. go s.serverWorker()<br/>创建新 goroutine**
    NEW->>NEW: **3. 分配初始栈 2KB**
    OLD->>OLD: **4. 函数返回，goroutine 退出**
    
    Note over OLD,GC: **旧 Worker 栈变为垃圾**
    
    GC->>GC: **5. 标记旧栈为可回收**
    GC->>OS: **6. 归还内存<br/>munmap 或 madvise**
    OS->>OS: **7. 释放物理页框**
    
    NEW->>NEW: **8. 继续处理请求<br/>必要时再扩展栈**
```

#### 为什么 Go 不自动收缩栈？

| **原因** | **说明** |
|---------|---------|
| **指针调整成本高** | 收缩栈需要扫描并调整所有栈指针，开销大 |
| **收缩时机难确定** | 何时收缩？收缩到多大？难以预测 |
| **可能很快又扩展** | 收缩后立即又需要扩展，浪费更多 |
| **设计哲学** | Go 选择简单的"只增不减"策略 |

#### 相关 Go Issue

源码注释引用的 Issue：[golang/go#18138](https://github.com/golang/go/issues/18138)

该 Issue 讨论了 `runtime.morestack` 在高并发场景下的性能问题，gRPC 的 Worker Pool 正是为了缓解这个问题。

### 任务分发策略

```mermaid
graph TB
    subgraph "**任务分发决策**"
        STREAM[**新Stream到达**]
        CHECK[**检查工作池配置**]
        
        subgraph "**有工作池**"
            TRY[**尝试发送到channel**]
            SUCCESS[**Worker接收处理**]
            FALLBACK[**channel满/阻塞**]
        end
        
        subgraph "**无工作池/回退**"
            SPAWN[**创建新goroutine**]
        end
    end
    
    STREAM --> CHECK
    CHECK -->|**numServerWorkers > 0**| TRY
    CHECK -->|**numServerWorkers == 0**| SPAWN
    TRY -->|**select成功**| SUCCESS
    TRY -->|**default分支**| FALLBACK
    FALLBACK --> SPAWN
    
    style STREAM fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style CHECK fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style TRY fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SUCCESS fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style FALLBACK fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style SPAWN fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
```

---

## 时序流程

### 服务器启动与工作池初始化

```mermaid
sequenceDiagram
    participant App as **应用程序**
    participant S as **Server**
    participant WP as **WorkerPool**
    participant W as **Workers**
    
    App->>S: **1. NewServer(NumStreamWorkers(N))**
    S->>S: **2. 保存配置<br/>opts.numServerWorkers = N**
    S-->>App: **3. 返回 Server**
    
    App->>S: **4. Serve(listener)**
    S->>S: **5. 检查 numServerWorkers > 0**
    
    alt **启用工作池**
        S->>WP: **6. initServerWorkers()**
        WP->>WP: **7. 创建任务通道<br/>serverWorkerChannel = make(chan func)**
        WP->>WP: **8. 设置关闭函数<br/>sync.OnceFunc(close)**
        
        loop **创建 N 个 Worker**
            WP->>W: **9. go serverWorker()**
            W->>W: **10. 阻塞等待任务**
        end
    end
    
    S->>S: **11. 开始接受连接**
```

### 请求处理流程

```mermaid
sequenceDiagram
    participant C as **客户端**
    participant ST as **ServerTransport**
    participant S as **Server**
    participant WC as **WorkerChannel**
    participant W as **Worker**
    participant H as **Handler**
    
    C->>ST: **1. 新RPC请求**
    ST->>S: **2. HandleStreams回调<br/>handle(stream)**
    S->>S: **3. handlersWG.Add(1)**
    S->>S: **4. streamQuota.acquire()**
    S->>S: **5. 构造处理函数 f()**
    
    alt **工作池启用且有空闲Worker**
        S->>WC: **6a. select: case channel <- f**
        WC->>W: **7a. Worker接收任务**
        W->>H: **8a. 执行 f() -> handleStream()**
    else **工作池满或未启用**
        S->>S: **6b. select: default分支**
        S->>H: **7b. go f() 新建goroutine**
    end
    
    H->>H: **9. 处理RPC请求**
    H->>H: **10. defer streamQuota.release()**
    H->>H: **11. defer handlersWG.Done()**
    H-->>C: **12. 返回响应**
```

### 服务器关闭流程

```mermaid
sequenceDiagram
    participant App as **应用程序**
    participant S as **Server**
    participant WC as **WorkerChannel**
    participant W as **Workers**
    participant H as **Handlers**
    
    App->>S: **1. Stop() 或 GracefulStop()**
    
    alt **GracefulStop**
        S->>S: **2a. 设置 drain = true**
        S->>S: **3a. 等待连接关闭**
    end
    
    S->>WC: **4. serverWorkerChannelClose()**
    WC->>WC: **5. close(serverWorkerChannel)**
    
    par **所有Worker收到关闭信号**
        WC->>W: **6a. Worker-1 退出**
        WC->>W: **6b. Worker-2 退出**
        WC->>W: **6c. Worker-N 退出**
    end
    
    alt **等待Handler完成**
        S->>H: **7. handlersWG.Wait()**
        H-->>S: **8. 所有Handler完成**
    end
    
    S-->>App: **9. 关闭完成**
```

---

## 函数调用链

### 工作池初始化调用链

```
NewServer(opt ...ServerOption) - server.go:687
├── 应用选项
│   └── for _, o := range opt { o.apply(&opts) }
│       └── NumStreamWorkers(n) - server.go:618
│           └── opts.numServerWorkers = n
├── 创建 Server 结构
│   └── s := &Server{
│       opts: opts,
│       conns: make(map[string]map[transport.ServerTransport]bool),
│       services: make(map[string]*serviceInfo),
│       ...
│   }
└── 返回 Server

Serve(lis net.Listener) - server.go:840
├── 检查工作池配置
│   └── if s.opts.numServerWorkers > 0
├── 初始化工作池
│   └── s.initServerWorkers() - server.go:675
│       ├── 创建任务通道
│       │   └── s.serverWorkerChannel = make(chan func())
│       ├── 设置关闭函数（只执行一次）
│       │   └── s.serverWorkerChannelClose = sync.OnceFunc(func() {
│       │           close(s.serverWorkerChannel)
│       │       })
│       └── 启动 Worker 协程
│           └── for i := uint32(0); i < s.opts.numServerWorkers; i++ {
│                   go s.serverWorker()
│               }
└── 开始接受连接
    └── for { ... lis.Accept() ... }
```

### Worker 执行调用链

```
serverWorker() - server.go:662
├── 循环处理任务
│   └── for completed := 0; completed < serverWorkerResetThreshold; completed++
│       ├── 从通道接收任务
│       │   └── f, ok := <-s.serverWorkerChannel
│       │       └── ┌────────────────┬────────────────────────────────┐
│       │           │  返回值         │  说明                          │
│       │           ├────────────────┼────────────────────────────────┤
│       │           │  f (func)      │  要执行的处理函数               │
│       │           ├────────────────┼────────────────────────────────┤
│       │           │  ok (bool)     │  channel是否仍然打开            │
│       │           └────────────────┴────────────────────────────────┘
│       ├── 检查通道状态
│       │   └── if !ok { return }  // channel关闭，Worker退出
│       └── 执行任务
│           └── f()
│               └── 实际执行 handleStream()
└── 达到阈值后重生
    └── go s.serverWorker()
        └── ┌────────────────┬────────────────────────────────┐
            │  参数           │  值                            │
            ├────────────────┼────────────────────────────────┤
            │  阈值           │  serverWorkerResetThreshold    │
            │                │  = 1 << 16 = 65536             │
            ├────────────────┼────────────────────────────────┤
            │  目的           │  释放旧栈空间，防止栈无限增长    │
            └────────────────┴────────────────────────────────┘
```

### 任务分发调用链

```
serveStreams(ctx, st, rawConn) - server.go:1036
├── 设置连接上下文
│   └── ctx = transport.SetConnection(ctx, rawConn)
├── 创建流配额管理器
│   └── streamQuota := newHandlerQuota(s.opts.maxConcurrentStreams)
└── 处理流
    └── st.HandleStreams(ctx, func(stream *transport.ServerStream) {
            ├── 增加等待组计数
            │   └── s.handlersWG.Add(1)
            ├── 获取流配额
            │   └── streamQuota.acquire()
            ├── 构造处理函数
            │   └── f := func() {
            │           defer streamQuota.release()
            │           defer s.handlersWG.Done()
            │           s.handleStream(st, stream)
            │       }
            └── 分发任务
                └── if s.opts.numServerWorkers > 0 {
                        select {
                        case s.serverWorkerChannel <- f:
                            return  // Worker接收成功
                        default:
                            // channel满，回退到新goroutine
                        }
                    }
                    go f()  // 新建goroutine处理
        })
```

### handleStream 处理调用链

```
handleStream(st, stream) - server.go:1795
├── 解析方法名
│   └── sm := stream.Method()
│       service, method := parseMethod(sm)
├── 查找服务
│   └── srv, ok := s.services[service]
├── 统计调用开始
│   └── s.incrCallsStarted()
└── 根据类型处理
    ├── 找到 Unary 方法
    │   └── if md, ok := srv.methods[method]; ok
    │       └── s.processUnaryRPC(ctx, st, stream, srv, md)
    │           ├── 接收请求
    │           │   └── recvMsg(stream, codec, decomp)
    │           ├── 调用拦截器链
    │           │   └── s.opts.unaryInt(ctx, req, info, handler)
    │           ├── 调用实际处理器
    │           │   └── md.Handler(srv.serviceImpl, ctx, dec, interceptor)
    │           └── 发送响应
    │               └── s.sendResponse(ctx, stream, reply, cp, opts, comp)
    │
    └── 找到 Stream 方法
        └── if sd, ok := srv.streams[method]; ok
            └── s.processStreamingRPC(ctx, st, stream, srv, sd)
                ├── 创建 ServerStream 包装
                │   └── ss := &serverStream{...}
                ├── 调用拦截器链
                │   └── s.opts.streamInt(srv.serviceImpl, ss, info, handler)
                └── 调用实际处理器
                    └── sd.Handler(srv.serviceImpl, ss)
```

---

## 性能优化原理

### 栈复用优势

```mermaid
graph TB
    subgraph "**无工作池模式**"
        subgraph "**每个请求**"
            N1[**分配新栈<br/>初始2KB**]
            N2[**可能扩展栈<br/>runtime.morestack**]
            N3[**请求完成<br/>goroutine退出**]
            N4[**栈空间释放<br/>GC回收**]
        end
    end
    
    subgraph "**工作池模式**"
        subgraph "**Worker生命周期**"
            W1[**启动时分配栈<br/>初始2KB**]
            W2[**逐渐扩展栈<br/>根据需要**]
            W3[**栈稳定后复用<br/>无需重新分配**]
            W4[**周期性重置<br/>防止无限增长**]
        end
    end
    
    N1 --> N2 --> N3 --> N4
    W1 --> W2 --> W3 --> W4
    W4 -.-> W1
    
    style N1 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style N2 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style N3 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style N4 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style W1 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
    style W2 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
    style W3 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
    style W4 fill:#d7ffd7,stroke:#333,stroke-width:2px,color:#000
```

### 性能对比

| **指标** | **无工作池** | **有工作池** |
|---------|------------|-------------|
| **栈分配** | 每请求分配 | 复用现有栈 |
| **GC压力** | 高（频繁分配释放） | 低（长期复用） |
| **延迟稳定性** | 可能有毛刺 | 更稳定 |
| **内存使用** | 峰值高 | 更可控 |
| **适用场景** | 低QPS | 高QPS |

### 最佳实践建议

1. **Worker 数量设置**
   - 建议设置为 CPU 核心数的 1-2 倍
   - 过多会增加调度开销
   - 过少会回退到创建新 goroutine

2. **适用场景**
   - QPS > 1000 的高并发服务
   - 对延迟敏感的服务
   - 需要稳定内存使用的服务

3. **监控指标**
   - 观察 Worker channel 是否经常满
   - 监控 goroutine 数量是否稳定
   - 关注 GC 暂停时间

```go
// 示例配置
server := grpc.NewServer(
    grpc.NumStreamWorkers(uint32(runtime.NumCPU())),
    grpc.MaxConcurrentStreams(1000),
)
```

---

## 总结

gRPC-Go 服务端工作池通过以下机制提升性能：

1. **goroutine 复用**: 避免频繁创建和销毁 goroutine
2. **栈空间复用**: 减少 `runtime.morestack` 调用
3. **周期性重置**: 防止栈空间无限增长
4. **优雅降级**: 工作池满时自动回退到新 goroutine

这种设计在高并发场景下能显著提升性能和稳定性。

