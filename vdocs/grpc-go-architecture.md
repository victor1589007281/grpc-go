# gRPC-Go 架构深度解析

## 目录
1. [概述](#概述)
2. [核心架构](#核心架构)
3. [核心组件](#核心组件)
4. [客户端架构](#客户端架构)
5. [服务端架构](#服务端架构)
6. [负载均衡与解析器](#负载均衡与解析器)
7. [传输层](#传输层)
8. [RPC调用流程](#rpc调用流程)
9. [函数调用链](#函数调用链)
10. [性能特性](#性能特性)

---

## 概述

gRPC-Go 是 gRPC 的 Go 语言实现，提供高性能、跨语言的 RPC 框架。它基于 HTTP/2 协议，使用 Protocol Buffers 作为接口定义语言（IDL）和序列化格式。

### 核心特性
- **HTTP/2 传输**: 多路复用、流控制、头部压缩
- **服务发现与负载均衡**: 可插拔的 Resolver 和 Balancer
- **拦截器支持**: 客户端和服务端的 Unary/Stream 拦截器链
- **安全传输**: TLS/mTLS 支持
- **流式RPC**: 支持 Unary、Server Streaming、Client Streaming、Bidirectional Streaming

---

## 核心架构

```mermaid
graph TB
    subgraph "**gRPC-Go 整体架构**"
        subgraph "**应用层**"
            A[**Generated Code<br/>生成的存根代码**]
            B[**Service Implementation<br/>服务实现**]
        end
        
        subgraph "**gRPC Core Layer<br/>gRPC 核心层**"
            C[**ClientConn<br/>客户端连接**]
            D[**Server<br/>服务端**]
            E[**Interceptors<br/>拦截器**]
        end
        
        subgraph "**Name Resolution & LB<br/>名称解析与负载均衡**"
            F[**Resolver<br/>解析器**]
            G[**Balancer<br/>负载均衡器**]
            H[**Picker<br/>选择器**]
        end
        
        subgraph "**Transport Layer<br/>传输层**"
            I[**http2Client<br/>HTTP/2客户端**]
            J[**http2Server<br/>HTTP/2服务端**]
            K[**Stream<br/>流**]
        end
        
        subgraph "**Network Layer<br/>网络层**"
            L[**TCP/TLS<br/>网络连接**]
        end
    end
    
    A --> C
    B --> D
    C --> E
    D --> E
    C --> F
    F --> G
    G --> H
    H --> I
    C --> I
    D --> J
    I --> K
    J --> K
    K --> L
    
    style A fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style B fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style C fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style D fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style E fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style F fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style G fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style H fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style I fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style J fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style K fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style L fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
```

---

## 核心组件

### 组件关系图

```mermaid
graph LR
    subgraph "**客户端组件**"
        CC[**ClientConn<br/>客户端连接管理**]
        RW[**ccResolverWrapper<br/>解析器包装**]
        BW[**ccBalancerWrapper<br/>负载均衡包装**]
        PW[**pickerWrapper<br/>选择器包装**]
        AC[**addrConn<br/>地址连接**]
    end
    
    subgraph "**服务端组件**"
        S[**Server<br/>gRPC服务器**]
        ST[**ServerTransport<br/>服务端传输**]
        SS[**ServerStream<br/>服务端流**]
    end
    
    CC --> RW
    CC --> BW
    BW --> PW
    CC --> AC
    AC --> CT[**ClientTransport<br/>客户端传输**]
    
    S --> ST
    ST --> SS
    
    style CC fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style RW fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style BW fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style PW fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style AC fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style S fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style ST fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SS fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style CT fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```

### 核心数据结构

| **组件** | **文件位置** | **职责** |
|---------|-------------|---------|
| **ClientConn** | `clientconn.go` | 管理客户端连接、解析器、负载均衡器 |
| **Server** | `server.go` | 管理服务端、监听器、连接处理 |
| **addrConn** | `clientconn.go` | 管理单个地址的连接 |
| **ccResolverWrapper** | `resolver_wrapper.go` | 包装 Resolver，处理地址解析 |
| **ccBalancerWrapper** | `balancer_wrapper.go` | 包装 Balancer，处理负载均衡 |
| **http2Client** | `internal/transport/http2_client.go` | HTTP/2 客户端传输实现 |
| **http2Server** | `internal/transport/http2_server.go` | HTTP/2 服务端传输实现 |

---

## 客户端架构

### ClientConn 结构

```mermaid
graph TB
    subgraph "**ClientConn 核心结构**"
        CC[**ClientConn**]
        
        subgraph "**初始化时设置 - 只读**"
            T[**target<br/>目标地址**]
            PT[**parsedTarget<br/>解析后的目标**]
            AU[**authority<br/>权限**]
            DO[**dopts<br/>拨号选项**]
            CZ[**channelz<br/>监控**]
            RB[**resolverBuilder<br/>解析器构建器**]
        end
        
        subgraph "**状态管理**"
            CSM[**csMgr<br/>连接状态管理**]
            PW[**pickerWrapper<br/>选择器包装**]
            RT[**retryThrottler<br/>重试节流**]
        end
        
        subgraph "**运行时组件 - mu保护**"
            RW[**resolverWrapper<br/>解析器包装**]
            BW[**balancerWrapper<br/>负载均衡包装**]
            SC[**sc<br/>服务配置**]
            CONNS[**conns<br/>连接映射**]
        end
    end
    
    CC --> T
    CC --> PT
    CC --> AU
    CC --> DO
    CC --> CZ
    CC --> RB
    CC --> CSM
    CC --> PW
    CC --> RT
    CC --> RW
    CC --> BW
    CC --> SC
    CC --> CONNS
    
    style CC fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style T fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style PT fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style AU fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style DO fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style CZ fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style RB fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style CSM fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style PW fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style RT fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style RW fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style BW fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SC fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style CONNS fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```

### 客户端连接建立时序图

```mermaid
sequenceDiagram
    participant App as **应用程序**
    participant CC as **ClientConn**
    participant RW as **ResolverWrapper**
    participant R as **Resolver**
    participant BW as **BalancerWrapper**
    participant B as **Balancer**
    participant AC as **addrConn**
    participant T as **Transport**
    
    App->>CC: **1. NewClient(target, opts)**
    CC->>CC: **2. 解析目标地址<br/>initParsedTargetAndResolverBuilder()**
    CC->>CC: **3. 验证传输凭证<br/>validateTransportCredentials()**
    CC->>CC: **4. 初始化authority<br/>initAuthority()**
    CC->>CC: **5. 链接拦截器<br/>chainInterceptors()**
    CC->>RW: **6. 创建解析器包装<br/>newCCResolverWrapper()**
    CC->>BW: **7. 创建负载均衡包装<br/>newCCBalancerWrapper()**
    CC-->>App: **8. 返回 ClientConn**
    
    Note over App,T: **延迟连接 - 首次RPC或Connect()时**
    
    App->>CC: **9. Connect() 或 首次RPC**
    CC->>CC: **10. exitIdleMode()**
    CC->>RW: **11. start() 启动解析器**
    RW->>R: **12. Build(target, cc, opts)**
    R->>RW: **13. UpdateState(addresses)**
    RW->>CC: **14. updateResolverStateAndUnlock()**
    CC->>BW: **15. updateClientConnState()**
    BW->>B: **16. UpdateClientConnState()**
    B->>BW: **17. NewSubConn(addrs)**
    BW->>CC: **18. newAddrConnLocked()**
    CC->>AC: **19. 创建 addrConn**
    B->>AC: **20. Connect()**
    AC->>AC: **21. resetTransportAndUnlock()**
    AC->>T: **22. NewHTTP2Client()**
    T-->>AC: **23. 连接建立成功**
    AC->>BW: **24. updateState(Ready)**
    BW->>B: **25. SubConn状态更新**
    B->>BW: **26. UpdateState(Ready, picker)**
    BW->>CC: **27. 更新picker和状态**
```

---

## 服务端架构

### Server 结构

```mermaid
graph TB
    subgraph "**Server 核心结构**"
        S[**Server**]
        
        subgraph "**配置选项**"
            OPTS[**opts<br/>服务器选项**]
            SH[**statsHandler<br/>统计处理**]
        end
        
        subgraph "**连接管理 - mu保护**"
            LIS[**lis<br/>监听器集合**]
            CONNS[**conns<br/>传输连接映射**]
            SVC[**services<br/>服务注册**]
        end
        
        subgraph "**生命周期管理**"
            QUIT[**quit<br/>退出事件**]
            DONE[**done<br/>完成事件**]
            WG[**serveWG<br/>服务协程组**]
            HWG[**handlersWG<br/>处理器协程组**]
        end
        
        subgraph "**工作池**"
            WC[**serverWorkerChannel<br/>工作通道**]
        end
    end
    
    S --> OPTS
    S --> SH
    S --> LIS
    S --> CONNS
    S --> SVC
    S --> QUIT
    S --> DONE
    S --> WG
    S --> HWG
    S --> WC
    
    style S fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style OPTS fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style SH fill:#ffe1e1,stroke:#333,stroke-width:2px,color:#000
    style LIS fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CONNS fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SVC fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style QUIT fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style DONE fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style WG fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style HWG fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style WC fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
```

### 服务端请求处理时序图

```mermaid
sequenceDiagram
    participant C as **客户端**
    participant L as **net.Listener**
    participant S as **Server**
    participant ST as **ServerTransport**
    participant SS as **ServerStream**
    participant H as **Handler**
    
    C->>L: **1. TCP连接请求**
    L->>S: **2. Accept() 返回 rawConn**
    S->>S: **3. handleRawConn(lisAddr, rawConn)**
    S->>ST: **4. newHTTP2Transport(rawConn)**
    ST->>ST: **5. TLS握手 (如果启用)**
    ST->>ST: **6. HTTP/2 Settings交换**
    S->>S: **7. addConn(lisAddr, st)**
    
    par **并行处理流**
        S->>S: **8. serveStreams(ctx, st, rawConn)**
        ST->>ST: **9. HandleStreams()**
        
        loop **每个请求流**
            C->>ST: **10. HTTP/2 HEADERS帧**
            ST->>SS: **11. 创建 ServerStream**
            ST->>S: **12. handleStream(st, stream)**
            S->>S: **13. 解析 service/method**
            
            alt **Unary RPC**
                S->>S: **14a. processUnaryRPC()**
                S->>H: **15a. md.Handler()**
                H-->>S: **16a. 返回响应**
                S->>SS: **17a. sendResponse()**
            else **Streaming RPC**
                S->>S: **14b. processStreamingRPC()**
                S->>H: **15b. sd.Handler()**
                H->>SS: **16b. 多次 SendMsg/RecvMsg**
            end
            
            SS->>C: **18. 发送响应**
            S->>SS: **19. WriteStatus()**
        end
    end
```

---

## 负载均衡与解析器

### Resolver 工作机制

```mermaid
graph TB
    subgraph "**Resolver 架构**"
        subgraph "**Resolver 接口**"
            RB[**resolver.Builder<br/>构建器接口**]
            R[**resolver.Resolver<br/>解析器接口**]
        end
        
        subgraph "**内置解析器**"
            DNS[**dns Resolver<br/>DNS解析**]
            PASS[**passthrough Resolver<br/>直通解析**]
            UNIX[**unix Resolver<br/>Unix套接字**]
        end
        
        subgraph "**解析结果**"
            STATE[**resolver.State**]
            ADDR[**Addresses<br/>地址列表**]
            EP[**Endpoints<br/>端点列表**]
            SC[**ServiceConfig<br/>服务配置**]
        end
    end
    
    RB --> R
    RB --> DNS
    RB --> PASS
    RB --> UNIX
    R --> STATE
    STATE --> ADDR
    STATE --> EP
    STATE --> SC
    
    style RB fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style R fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style DNS fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style PASS fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style UNIX fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style STATE fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style ADDR fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style EP fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SC fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```

### Balancer 工作机制

```mermaid
graph TB
    subgraph "**Balancer 架构**"
        subgraph "**Balancer 接口**"
            BB[**balancer.Builder<br/>构建器**]
            B[**balancer.Balancer<br/>负载均衡器**]
            P[**balancer.Picker<br/>选择器**]
        end
        
        subgraph "**内置策略**"
            PF[**pick_first<br/>选择第一个**]
            RR[**round_robin<br/>轮询**]
            WRR[**weighted_round_robin<br/>加权轮询**]
            RH[**ring_hash<br/>环形哈希**]
        end
        
        subgraph "**SubConn 管理**"
            SC[**SubConn<br/>子连接**]
            SCS[**SubConnState<br/>连接状态**]
        end
    end
    
    BB --> B
    B --> P
    BB --> PF
    BB --> RR
    BB --> WRR
    BB --> RH
    B --> SC
    SC --> SCS
    
    style BB fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style B fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style P fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style PF fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style RR fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style WRR fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style RH fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SC fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SCS fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
```

### 负载均衡选择流程

```mermaid
sequenceDiagram
    participant CS as **ClientStream**
    participant PW as **pickerWrapper**
    participant P as **Picker**
    participant SC as **SubConn**
    participant T as **Transport**
    
    CS->>PW: **1. pick(ctx, failFast, pickInfo)**
    
    loop **等待可用的Picker**
        PW->>PW: **2. 检查当前picker**
        alt **picker为空或阻塞**
            PW->>PW: **3. 等待新的picker**
        end
    end
    
    PW->>P: **4. Pick(PickInfo)**
    
    alt **成功选择**
        P-->>PW: **5a. PickResult{SubConn, Done}**
        PW->>SC: **6. 获取Transport**
        SC->>SC: **7. getReadyTransport()**
        
        alt **Transport就绪**
            SC-->>PW: **8a. 返回Transport**
            PW-->>CS: **9a. pickResult{transport}**
        else **Transport未就绪**
            PW->>PW: **8b. 重新pick**
        end
    else **无可用SubConn**
        P-->>PW: **5b. ErrNoSubConnAvailable**
        PW->>PW: **6b. 等待新picker**
    else **TransientFailure**
        P-->>PW: **5c. 错误状态**
        PW-->>CS: **6c. 返回错误 (非waitForReady)**
    end
```

---

## 传输层

### HTTP/2 传输架构

```mermaid
graph TB
    subgraph "**Transport Layer 架构**"
        subgraph "**客户端传输**"
            HC[**http2Client**]
            CL[**loopyWriter<br/>写入循环**]
            CF[**framer<br/>帧读写**]
            CCB[**controlBuf<br/>控制缓冲**]
        end
        
        subgraph "**服务端传输**"
            HS[**http2Server**]
            SL[**loopyWriter<br/>写入循环**]
            SF[**framer<br/>帧读写**]
            SCB[**controlBuf<br/>控制缓冲**]
        end
        
        subgraph "**流管理**"
            CS[**ClientStream**]
            SS[**ServerStream**]
        end
        
        subgraph "**流控制**"
            FC[**Flow Control<br/>流量控制**]
            BDP[**BDP Estimator<br/>带宽估算**]
        end
    end
    
    HC --> CL
    HC --> CF
    HC --> CCB
    HC --> CS
    
    HS --> SL
    HS --> SF
    HS --> SCB
    HS --> SS
    
    HC --> FC
    HS --> FC
    FC --> BDP
    
    style HC fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style CL fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CF fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CCB fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style HS fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style SL fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SF fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style SCB fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CS fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style SS fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style FC fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style BDP fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
```

### HTTP/2 帧处理

| **帧类型** | **用途** | **处理函数** |
|-----------|---------|-------------|
| **HEADERS** | 请求/响应头 | `handleHeaders()` |
| **DATA** | 消息数据 | `handleData()` |
| **RST_STREAM** | 流重置 | `handleRSTStream()` |
| **SETTINGS** | 设置协商 | `handleSettings()` |
| **PING** | 心跳检测 | `handlePing()` |
| **GOAWAY** | 优雅关闭 | `handleGoAway()` |
| **WINDOW_UPDATE** | 流控更新 | `handleWindowUpdate()` |

---

## RPC调用流程

### Unary RPC 完整时序

```mermaid
sequenceDiagram
    participant App as **应用程序**
    participant CC as **ClientConn**
    participant INT as **Interceptor<br/>拦截器**
    participant CS as **clientStream**
    participant PW as **pickerWrapper**
    participant T as **Transport**
    participant Net as **Network**
    participant ST as **ServerTransport**
    participant S as **Server**
    participant H as **Handler**
    
    App->>CC: **1. cc.Invoke(ctx, method, req, reply)**
    CC->>INT: **2. unaryInt(ctx, method, req, reply, cc, invoke)**
    INT->>CC: **3. invoke(ctx, method, req, reply, cc)**
    CC->>CS: **4. newClientStream()**
    CS->>PW: **5. pick(ctx, failFast, pickInfo)**
    PW-->>CS: **6. 返回 transport**
    CS->>T: **7. NewStream(ctx, callHdr)**
    T-->>CS: **8. 返回 stream**
    
    CS->>CS: **9. SendMsg(req)**
    CS->>T: **10. Write(hdr, data)**
    T->>Net: **11. 发送 HEADERS + DATA 帧**
    
    Net->>ST: **12. 接收帧**
    ST->>S: **13. handleStream()**
    S->>S: **14. processUnaryRPC()**
    S->>H: **15. Handler(srv, ctx, dec, interceptor)**
    H-->>S: **16. 返回响应**
    S->>ST: **17. sendResponse()**
    ST->>Net: **18. 发送响应帧**
    
    Net->>T: **19. 接收响应帧**
    T->>CS: **20. 数据到达**
    CS->>CS: **21. RecvMsg(reply)**
    CS-->>INT: **22. 返回 nil**
    INT-->>App: **23. 返回结果**
```

---

## 函数调用链

### 客户端 Unary RPC 调用链

```text
ClientConn.Invoke() - clientconn.go:29
├── 合并 CallOptions
│   └── combine(cc.dopts.callOptions, opts) - call.go:40
├── 拦截器处理
│   └── cc.dopts.unaryInt(ctx, method, args, reply, cc, invoke, opts...) - call.go:35
└── invoke() - call.go:65
    └── newClientStream() - stream.go:202
        ├── channelz.IsOn() - 统计
        ├── cc.idlenessMgr.OnCallBegin() - 空闲管理
        ├── cc.waitForResolvedAddrs(ctx) - stream.go:238
        │   └── 等待解析器返回地址
        ├── cc.safeConfigSelector.SelectConfig(rpcInfo) - stream.go:250
        │   └── 选择服务配置
        └── newClientStreamWithParams() - stream.go:284
            ├── defaultCallInfo() - 默认调用信息
            ├── 应用 CallOptions
            │   └── o.before(callInfo) - stream.go:308
            ├── 设置消息大小限制
            │   └── getMaxSize() - stream.go:312-313
            ├── 创建 clientStream 结构
            │   └── ┌────────────────┬──────────────────────────────────────┐
            │       │  字段           │  说明                                 │
            │       ├────────────────┼──────────────────────────────────────┤
            │       │  callHdr        │  调用头信息(Host, Method等)           │
            │       ├────────────────┼──────────────────────────────────────┤
            │       │  ctx            │  请求上下文                           │
            │       ├────────────────┼──────────────────────────────────────┤
            │       │  codec          │  序列化编解码器                        │
            │       ├────────────────┼──────────────────────────────────────┤
            │       │  compressorV1   │  压缩器                               │
            │       ├────────────────┼──────────────────────────────────────┤
            │       │  cancel         │  取消函数                             │
            │       ├────────────────┼──────────────────────────────────────┤
            │       │  retryThrottler │  重试节流器                           │
            │       └────────────────┴──────────────────────────────────────┘
            ├── cs.withRetry(op, onSuccess) - stream.go:395
            │   ├── cs.newAttemptLocked(isTransparent) - stream.go:437
            │   │   ├── 创建 csAttempt
            │   │   └── sh.HandleRPC(ctx, &stats.Begin{}) - 统计开始
            │   └── op(a) - 执行操作
            │       ├── a.getTransport() - stream.go:498
            │       │   └── cs.cc.pickerWrapper.pick(ctx, failFast, pickInfo) - picker_wrapper.go
            │       └── a.newStream() - stream.go:520
            │           └── a.transport.NewStream(ctx, cs.callHdr) - transport
            └── 返回 cs
```

### 服务端请求处理调用链

```text
Server.Serve(lis) - server.go:871
├── s.serveWG.Add(1) - 服务协程计数
├── 创建 listenSocket
│   └── channelz.RegisterSocket() - 注册监控
└── for 循环
    ├── lis.Accept() - server.go:917
    │   └── 接受TCP连接
    ├── s.serveWG.Add(1) - 连接协程计数
    └── go s.handleRawConn(lisAddr, rawConn) - server.go:957
        └── handleRawConn() - server.go:965
            ├── rawConn.SetDeadline() - 设置超时
            ├── s.newHTTP2Transport(rawConn) - server.go:973
            │   └── NewServerTransport() - http2_server.go:149
            │       ├── TLS握手 (如果启用)
            │       │   └── config.Credentials.ServerHandshake(rawConn) - http2_server.go:153
            │       ├── newFramer() - 创建帧读写器
            │       ├── framer.fr.WriteSettings() - 发送SETTINGS帧
            │       └── 创建 http2Server 结构
            │           └── ┌────────────────┬──────────────────────────────────────┐
            │               │  字段           │  说明                                 │
            │               ├────────────────┼──────────────────────────────────────┤
            │               │  conn           │  底层网络连接                         │
            │               ├────────────────┼──────────────────────────────────────┤
            │               │  framer         │  HTTP/2帧读写器                       │
            │               ├────────────────┼──────────────────────────────────────┤
            │               │  maxStreams     │  最大并发流数                         │
            │               ├────────────────┼──────────────────────────────────────┤
            │               │  controlBuf     │  控制缓冲区                           │
            │               ├────────────────┼──────────────────────────────────────┤
            │               │  activeStreams  │  活跃流映射                           │
            │               ├────────────────┼──────────────────────────────────────┤
            │               │  kp             │  Keepalive参数                       │
            │               └────────────────┴──────────────────────────────────────┘
            ├── s.addConn(lisAddr, st) - server.go:985
            └── go s.serveStreams(ctx, st, rawConn) - server.go:989
                └── serveStreams() - server.go:1036
                    ├── transport.SetConnection(ctx, rawConn) - 设置连接上下文
                    ├── peer.NewContext(ctx, st.Peer()) - 设置peer信息
                    ├── s.statsHandler.HandleConn() - 统计连接
                    └── st.HandleStreams(ctx, func(stream)) - server.go:1055
                        └── handleStream() - server.go:1765
                            ├── contextWithServer(ctx, s) - 设置server上下文
                            ├── 解析 service/method
                            │   └── strings.LastIndex(sm, "/") - server.go:1788
                            ├── s.statsHandler.HandleRPC() - 统计RPC
                            └── 分发请求
                                ├── [Unary] processUnaryRPC() - server.go:1243
                                │   ├── recvAndDecompress() - 接收请求
                                │   ├── md.Handler() - 调用处理器
                                │   │   └── ┌────────────────┬──────────────────────────────────────┐
                                │   │       │  参数           │  说明                                 │
                                │   │       ├────────────────┼──────────────────────────────────────┤
                                │   │       │  srv            │  服务实现                             │
                                │   │       ├────────────────┼──────────────────────────────────────┤
                                │   │       │  ctx            │  请求上下文                           │
                                │   │       ├────────────────┼──────────────────────────────────────┤
                                │   │       │  dec            │  解码函数                             │
                                │   │       ├────────────────┼──────────────────────────────────────┤
                                │   │       │  interceptor    │  拦截器                               │
                                │   │       └────────────────┴──────────────────────────────────────┘
                                │   ├── s.sendResponse() - 发送响应
                                │   └── stream.WriteStatus() - 写入状态
                                └── [Stream] processStreamingRPC() - server.go:1573
                                    ├── 创建 serverStream
                                    ├── sd.Handler(server, ss) - 调用处理器
                                    └── ss.s.WriteStatus() - 写入状态
```

---

## 性能特性

### 性能优化机制

```mermaid
graph TB
    subgraph "**gRPC-Go 性能优化**"
        subgraph "**连接复用**"
            H2[**HTTP/2 多路复用<br/>单连接多流**]
            CP[**连接池<br/>SubConn管理**]
        end
        
        subgraph "**内存优化**"
            BP[**Buffer Pool<br/>缓冲池**]
            ZC[**零拷贝<br/>mem.BufferSlice**]
        end
        
        subgraph "**并发处理**"
            WP[**Worker Pool<br/>服务端工作池**]
            AS[**Async Send<br/>异步发送**]
        end
        
        subgraph "**流控制**"
            FC[**Flow Control<br/>HTTP/2流控**]
            BDP[**BDP Estimation<br/>带宽延迟积估算**]
        end
        
        subgraph "**压缩**"
            GZ[**gzip Compression<br/>gzip压缩**]
            PB[**Protobuf<br/>高效序列化**]
        end
    end
    
    H2 --> CP
    BP --> ZC
    WP --> AS
    FC --> BDP
    GZ --> PB
    
    style H2 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style CP fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style BP fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style ZC fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style WP fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style AS fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style FC fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style BDP fill:#f5e1ff,stroke:#333,stroke-width:2px,color:#000
    style GZ fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style PB fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
```

### 关键性能参数

| **参数** | **默认值** | **说明** |
|---------|-----------|---------|
| **MaxRecvMsgSize** | 4MB | 最大接收消息大小 |
| **MaxSendMsgSize** | MaxInt32 | 最大发送消息大小 |
| **MaxConcurrentStreams** | MaxUint32 | 最大并发流数 |
| **InitialWindowSize** | 64KB | 初始流控窗口 |
| **InitialConnWindowSize** | 64KB | 初始连接窗口 |
| **WriteBufferSize** | 32KB | 写缓冲区大小 |
| **ReadBufferSize** | 32KB | 读缓冲区大小 |
| **NumServerWorkers** | 0 (禁用) | 服务端工作协程数 |

### 连接状态机

```mermaid
stateDiagram-v2
    [*] --> Idle: 创建
    Idle --> Connecting: Connect()
    Connecting --> Ready: 连接成功
    Connecting --> TransientFailure: 连接失败
    TransientFailure --> Idle: 退避后
    Ready --> Idle: 空闲超时
    Ready --> TransientFailure: 连接断开
    Idle --> Shutdown: Close()
    Connecting --> Shutdown: Close()
    Ready --> Shutdown: Close()
    TransientFailure --> Shutdown: Close()
    Shutdown --> [*]
```

---

## 传输协议支持

### gRPC 支持哪些传输协议？

```mermaid
graph TB
    subgraph "**gRPC-Go 传输协议支持**"
        subgraph "**原生支持**"
            TCP[**TCP<br/>默认协议**]
            UNIX[**Unix Socket<br/>本地进程通信**]
        end
        
        subgraph "**HTTP/2 层**"
            H2[**HTTP/2<br/>gRPC协议层**]
        end
        
        subgraph "**可扩展（需自定义）**"
            CUSTOM[**自定义传输<br/>via net.Conn**]
            QUIC[**QUIC/HTTP3<br/>实验性**]
        end
        
        subgraph "**不支持**"
            UDP[**UDP<br/>不支持**]
            RDMA[**RDMA<br/>不支持**]
        end
    end
    
    TCP --> H2
    UNIX --> H2
    CUSTOM --> H2
    
    style TCP fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style UNIX fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
    style H2 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style CUSTOM fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style QUIC fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style UDP fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style RDMA fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
```

### 为什么 gRPC 基于 TCP 而非 UDP？

| **因素** | **TCP** | **UDP** |
|---------|--------|--------|
| **可靠性** | 保证有序、无丢失 | 不保证，需应用层处理 |
| **流控制** | TCP + HTTP/2 双重流控 | 无，需自行实现 |
| **HTTP/2** | 原生支持 | HTTP/3 使用 QUIC（基于 UDP） |
| **连接复用** | HTTP/2 多路复用 | 无连接概念 |
| **兼容性** | 防火墙友好 | 可能被过滤 |

**gRPC 选择 HTTP/2 over TCP 的原因**：
1. HTTP/2 提供了多路复用、流控制、头部压缩等特性
2. HTTP/2 本身设计为运行在 TCP 上
3. 更好的防火墙兼容性和代理支持

### gRPC-Go 支持的传输方式

| **传输方式** | **支持状态** | **使用方法** |
|-------------|-------------|-------------|
| **TCP** | ✅ 原生支持 | 默认，无需配置 |
| **Unix Socket** | ✅ 原生支持 | `unix:///path/to/socket` 或 `unix-abstract://name` |
| **TLS/mTLS** | ✅ 原生支持 | `WithTransportCredentials()` |
| **自定义 Dialer** | ✅ 支持 | `WithContextDialer()` |
| **QUIC/HTTP3** | ⚠️ 实验性 | 需第三方库 |
| **UDP** | ❌ 不支持 | HTTP/2 不支持 |
| **RDMA** | ❌ 不支持 | 需深度定制 |

### 使用 Unix Socket

```go
// 服务端
lis, _ := net.Listen("unix", "/tmp/grpc.sock")
server.Serve(lis)

// 客户端
conn, _ := grpc.NewClient("unix:///tmp/grpc.sock",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
```

### 自定义传输层

gRPC-Go 通过 `net.Conn` 接口抽象传输层，可以通过自定义 Dialer 实现任何传输：

```go
// 自定义 Dialer 示例
customDialer := func(ctx context.Context, addr string) (net.Conn, error) {
    // 可以返回任何实现了 net.Conn 的连接
    // 例如：自定义的 RDMA 连接包装器
    return myCustomConnection(addr)
}

conn, _ := grpc.NewClient("target",
    grpc.WithContextDialer(customDialer),
    grpc.WithTransportCredentials(insecure.NewCredentials()))
```

### 为什么 RDMA 需要深度定制？

**RDMA (Remote Direct Memory Access)** 是一种高性能网络技术，要在 gRPC 中使用 RDMA 面临以下挑战：

```mermaid
graph TB
    subgraph "**RDMA 集成挑战**"
        subgraph "**协议层面**"
            C1[**HTTP/2 设计为字节流<br/>RDMA 是内存语义**]
            C2[**HTTP/2 帧格式<br/>vs RDMA 消息格式**]
        end
        
        subgraph "**实现层面**"
            C3[**需要零拷贝路径**]
            C4[**需要绕过内核**]
            C5[**需要注册内存**]
        end
        
        subgraph "**可能的方案**"
            S1[**自定义 Transport**]
            S2[**替换整个 HTTP/2 层**]
            S3[**使用专门的 RPC 框架<br/>如 eRPC, UCX**]
        end
    end
    
    C1 --> S1
    C2 --> S2
    C3 --> S3
    C4 --> S3
    C5 --> S3
    
    style C1 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style C2 fill:#ffd7d7,stroke:#333,stroke-width:2px,color:#000
    style C3 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style C4 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style C5 fill:#fff3e1,stroke:#333,stroke-width:2px,color:#000
    style S1 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style S2 fill:#e1f5ff,stroke:#333,stroke-width:2px,color:#000
    style S3 fill:#e1ffe1,stroke:#333,stroke-width:2px,color:#000
```

### 高性能场景的替代方案

如果需要 UDP 或 RDMA 支持，考虑以下替代方案：

| **方案** | **适用场景** | **特点** |
|---------|------------|---------|
| **gRPC + QUIC** | 高丢包网络 | HTTP/3 基于 QUIC（UDP） |
| **eRPC** | 低延迟 RDMA | 微软开源，专为 RDMA 设计 |
| **UCX** | HPC/ML 场景 | 统一通信接口，支持 RDMA |
| **自定义 RPC** | 极致性能 | 需要大量开发工作 |

---

## 总结

gRPC-Go 的架构设计具有以下特点：

1. **分层清晰**: 应用层、核心层、传输层职责明确
2. **可扩展性强**: Resolver、Balancer、Interceptor 均可自定义
3. **高性能**: HTTP/2 多路复用、连接池、Worker Pool
4. **可观测性**: Channelz、Stats Handler、Tracing 支持
5. **安全性**: TLS/mTLS、Per-RPC Credentials 支持
6. **传输灵活性**: 支持 TCP、Unix Socket，可通过 `net.Conn` 扩展

核心组件之间通过 Wrapper 模式解耦，使用 Serializer 保证并发安全，通过事件驱动实现状态变化通知。

### 传输协议总结

- **gRPC 默认基于 TCP**（HTTP/2 要求）
- **原生支持 Unix Socket**（本地高性能通信）
- **不支持 UDP/RDMA**（HTTP/2 协议限制）
- **可通过自定义 Dialer 扩展传输层**

