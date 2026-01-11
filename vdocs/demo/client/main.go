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

// Package main 实现了一个简单的 gRPC 客户端
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	pb "demo/proto"
)

var (
	addr = flag.String("addr", "localhost:50051", "服务端地址")
	name = flag.String("name", "gRPC用户", "要问候的名字")
)

func main() {
	flag.Parse()

	// 客户端配置选项
	opts := []grpc.DialOption{
		// 使用不安全的连接（仅用于演示）
		grpc.WithTransportCredentials(insecure.NewCredentials()),

		// 设置 Keepalive 参数
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // Keepalive ping 间隔
			Timeout:             3 * time.Second,  // Keepalive ping 超时
			PermitWithoutStream: true,             // 允许没有活跃流时 ping
		}),

		// 设置默认服务配置
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
		}`),
	}

	// 创建客户端连接
	log.Printf("正在连接服务端: %s", *addr)
	conn, err := grpc.NewClient(*addr, opts...)
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	// 创建客户端存根
	client := pb.NewHelloServiceClient(conn)

	log.Printf("========================================")
	log.Printf("gRPC 客户端启动成功")
	log.Printf("服务端地址: %s", *addr)
	log.Printf("========================================")

	// 测试 Unary RPC
	testUnaryRPC(client, *name)

	log.Println()

	// 测试 Server Streaming RPC
	testStreamingRPC(client, *name)
}

// testUnaryRPC 测试 Unary RPC
func testUnaryRPC(client pb.HelloServiceClient, name string) {
	log.Println("========== 测试 Unary RPC ==========")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 发起 RPC 调用
	req := &pb.HelloRequest{Name: name}
	log.Printf("[Unary] 发送请求: name=%s", name)

	start := time.Now()
	reply, err := client.SayHello(ctx, req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("[Unary] 调用失败: %v", err)
		return
	}

	log.Printf("[Unary] 收到响应:")
	log.Printf("  - message: %s", reply.GetMessage())
	log.Printf("  - timestamp: %d", reply.GetTimestamp())
	log.Printf("  - 耗时: %v", duration)
}

// testStreamingRPC 测试 Server Streaming RPC
func testStreamingRPC(client pb.HelloServiceClient, name string) {
	log.Println("========== 测试 Server Streaming RPC ==========")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 发起流式 RPC 调用
	req := &pb.HelloRequest{Name: name}
	log.Printf("[Stream] 发送请求: name=%s", name)

	start := time.Now()
	stream, err := client.SayHelloStream(ctx, req)
	if err != nil {
		log.Printf("[Stream] 调用失败: %v", err)
		return
	}

	// 接收流式响应
	count := 0
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[Stream] 接收失败: %v", err)
			return
		}

		count++
		log.Printf("[Stream] 收到消息 %d:", count)
		log.Printf("  - message: %s", reply.GetMessage())
		log.Printf("  - timestamp: %d", reply.GetTimestamp())
	}

	duration := time.Since(start)
	log.Printf("[Stream] 流式传输完成，共收到 %d 条消息，总耗时: %v", count, duration)
}

