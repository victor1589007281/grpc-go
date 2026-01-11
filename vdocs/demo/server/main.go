// Copyright 2024 gRPC authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main 实现了一个简单的 gRPC 服务端
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"runtime"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	pb "demo/proto"
)

var (
	port = flag.Int("port", 50051, "服务端监听端口")
)

// helloServer 实现了 HelloServiceServer 接口
type helloServer struct {
	pb.UnimplementedHelloServiceServer
}

// SayHello 实现了 Unary RPC
func (s *helloServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("[Unary] 收到请求: name=%s", req.GetName())

	reply := &pb.HelloReply{
		Message:   fmt.Sprintf("你好, %s! 来自 gRPC 服务端的问候。", req.GetName()),
		Timestamp: time.Now().UnixMilli(),
	}

	log.Printf("[Unary] 发送响应: message=%s", reply.GetMessage())
	return reply, nil
}

// SayHelloStream 实现了 Server Streaming RPC
func (s *helloServer) SayHelloStream(req *pb.HelloRequest, stream grpc.ServerStreamingServer[pb.HelloReply]) error {
	log.Printf("[Stream] 收到请求: name=%s", req.GetName())

	// 发送多条消息
	messages := []string{
		fmt.Sprintf("你好, %s! 这是第1条消息。", req.GetName()),
		fmt.Sprintf("你好, %s! 这是第2条消息。", req.GetName()),
		fmt.Sprintf("你好, %s! 这是第3条消息。", req.GetName()),
		fmt.Sprintf("再见, %s! 流式传输结束。", req.GetName()),
	}

	for i, msg := range messages {
		reply := &pb.HelloReply{
			Message:   msg,
			Timestamp: time.Now().UnixMilli(),
		}

		if err := stream.Send(reply); err != nil {
			log.Printf("[Stream] 发送失败: %v", err)
			return err
		}

		log.Printf("[Stream] 发送消息 %d: %s", i+1, msg)

		// 模拟处理时间
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("[Stream] 流式传输完成")
	return nil
}

// loggingInterceptor 是一个 Unary 拦截器，用于记录请求日志
func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	log.Printf("[拦截器] 开始处理: method=%s", info.FullMethod)

	// 调用实际的处理器
	resp, err := handler(ctx, req)

	duration := time.Since(start)
	if err != nil {
		log.Printf("[拦截器] 处理失败: method=%s, duration=%v, error=%v", info.FullMethod, duration, err)
	} else {
		log.Printf("[拦截器] 处理成功: method=%s, duration=%v", info.FullMethod, duration)
	}

	return resp, err
}

func main() {
	flag.Parse()

	// 创建 TCP 监听器
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("监听失败: %v", err)
	}

	// 服务端配置选项
	opts := []grpc.ServerOption{
		// 设置工作池（推荐在高并发场景使用）
		grpc.NumStreamWorkers(uint32(runtime.NumCPU())),

		// 设置最大并发流
		grpc.MaxConcurrentStreams(1000),

		// 设置 Keepalive 参数
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     30 * time.Second, // 连接空闲超时
			MaxConnectionAge:      5 * time.Minute,  // 连接最大存活时间
			MaxConnectionAgeGrace: 10 * time.Second, // 优雅关闭等待时间
			Time:                  10 * time.Second, // Keepalive ping 间隔
			Timeout:               3 * time.Second,  // Keepalive ping 超时
		}),

		// 设置 Keepalive 执行策略
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // 客户端 ping 最小间隔
			PermitWithoutStream: true,            // 允许没有活跃流时 ping
		}),

		// 设置消息大小限制
		grpc.MaxRecvMsgSize(4 * 1024 * 1024), // 4MB
		grpc.MaxSendMsgSize(4 * 1024 * 1024), // 4MB

		// 设置拦截器
		grpc.UnaryInterceptor(loggingInterceptor),
	}

	// 创建 gRPC 服务器
	s := grpc.NewServer(opts...)

	// 注册服务
	pb.RegisterHelloServiceServer(s, &helloServer{})

	// 注册反射服务（用于 grpcurl 等工具）
	reflection.Register(s)

	// 打印服务器信息
	log.Printf("========================================")
	log.Printf("gRPC 服务端启动成功")
	log.Printf("监听地址: %s", lis.Addr().String())
	log.Printf("工作池大小: %d", runtime.NumCPU())
	log.Printf("========================================")

	// 启动服务
	if err := s.Serve(lis); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

