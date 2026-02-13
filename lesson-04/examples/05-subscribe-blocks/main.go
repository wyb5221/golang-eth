package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 01-subscribe-blocks.go
// 通过 SubscribeNewHead 订阅新区块头。
// 注意：大多数节点要求使用 WebSocket RPC，例如：ws://127.0.0.1:8546 或 wss://...
//通过 WebSocket 连接到以太坊节点，实时接收新区块头信息并打印
func main() {
	//从环境变量获取节点连接地址
	rpcURL := os.Getenv("ETH_WS_URL")
	if rpcURL == "" {
		// 回退到 ETH_RPC_URL，便于在只配置了 HTTP 的环境中看到错误提示
		rpcURL = os.Getenv("ETH_RPC_URL")
	}
	if rpcURL == "" {
		log.Fatal("ETH_WS_URL or ETH_RPC_URL must be set")
	}
	// WithCancel 创建一个可取消的上下文
	// 当调用 cancel() 时，所有监听 ctx.Done() 的地方都会收到退出信号
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 确保函数退出时释放上下文资源
	//DialContext 建立与以太坊节点的连接
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer client.Close() // 程序退出时关闭连接

	//创建订阅通道并开始订阅
	// headers: 用于接收新区块头的通道，缓冲区大小为0（无缓冲）
	// 当新区块产生时，节点会将区块头发送到这个通道
	headers := make(chan *types.Header)
	
	// SubscribeNewHead: 订阅新区块头事件
	// 参数1: 上下文，用于取消订阅
	// 参数2: 接收区块头的通道
	// 返回: 订阅对象，包含错误通道 Err()
	// 注意: 此功能必须使用 WebSocket 连接，HTTP 连接会返回错误
	sub, err := client.SubscribeNewHead(ctx, headers)
	if err != nil {
		log.Fatalf("failed to subscribe new heads: %v", err)
	}

	fmt.Printf("Subscribed to new blocks via %s\n", rpcURL)

	// 捕获 Ctrl+C 退出
	//设置系统信号捕获
	// 创建一个缓冲通道，用于接收操作系统信号
	sigCh := make(chan os.Signal, 1)
	// 订阅 SIGINT (Ctrl+C) 和 SIGTERM (kill) 信号
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	//主事件循环
	//无限循环，等待多个通道的事件
	for {
		select {
		// 场景1: 收到新区块头
		case h := <-headers:
			//检查区块头是否为 nil
			if h == nil {
				continue
			}
			// 打印新区块信息
			// - 时间戳: 当前系统时间（RFC3339格式）
			// - 区块号: 区块高度（从0开始递增）
			// - 区块哈希: 区块头的唯一标识符
			fmt.Printf("[%s] New Block - Number: %d, Hash: %s\n",
				time.Now().Format(time.RFC3339),
				h.Number.Uint64(),
				h.Hash().Hex(),
			)
		//场景2: 订阅发生错误（连接断开、节点问题等）
		case err := <-sub.Err():
			// 记录错误并退出程序
			// 常见错误: WebSocket 连接断开、节点未同步完成、网络超时等
			log.Printf("subscription error: %v", err)
			return
		//收到系统退出信号（用户按 Ctrl+C）
		case sig := <-sigCh:
			//打印收到的信号并优雅退出
			fmt.Printf("received signal %s, shutting down...\n", sig.String())
			return
		//上下文被取消（通常由程序其他部分触发）
		case <-ctx.Done():
			fmt.Println("context cancelled, exiting...")
			return
		}
	}
}
