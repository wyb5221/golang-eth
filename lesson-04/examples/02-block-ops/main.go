package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 使用示例：
//
//	# 查询最新区块
//	go run main.go
//
//	# 查询指定区块
//	go run main.go -number 123456
//
//	# 批量查询区块范围 [100, 105]
//	go run main.go -range-start 100 -range-end 105
//
//	# 批量查询，自定义请求间隔（毫秒）
//	go run main.go -range-start 100 -range-end 105 -rate-limit 500

// 查询最新区块、指定区块以及批量查询区块范围的信息。
func main() {

	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		log.Fatal("ETH_RPC_URL is not set")
	}
	//context.WithTimeout 创建一个新的上下文，会在指定时间后自动取消,30 秒超时：防止网络连接问题导致程序无限等待
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // defer 确保函数退出时取消上下文，释放资源
	//使用 DialContext 连接以太坊节点
	client, err := ethclient.DialContext(ctx, rpcURL) // 连接以太坊节点，使用上下文管理连接的生命周期
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer client.Close()

	//获取当前连接的以太坊网络的链 ID,
	//链 ID 用于识别不同的以太坊网络（如主网=1，Sepolia测试网=11155111）
	chainID, err := client.ChainID(ctx)
	fmt.Printf("连接成功，链ID：%s\n", chainID.String())

	// 最新区块
	latestBlock, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("failed to get latest block: %v", err)
	}
	printBlockInfo("最新区块信息", latestBlock)
	//fmt.Println("最新区块latestBlock:", latestBlock)

	// 1. 定义命令行参数：blockNumberFlag 用于接收用户输入的区块号
	//    - 参数名称为 "number"
	//    - 默认值为 0（0 表示跳过此参数，不查询特定区块）
	//    - 参数说明：要查询的区块号（0 表示跳过）
	blockNumberFlag := flag.Uint64("number", 0, "block number to query (0 means skip)")
	rangeStartFlag := flag.Uint64("range-start", 0, "start block number for range query")
	rangeEndFlag := flag.Uint64("range-end", 0, "end block number for range query")
	rateLimitFlag := flag.Int("rate-limit", 200, "rate limit in milliseconds between requests")
	flag.Parse()
	fmt.Printf("--blockNumberFlag:%d\n", *blockNumberFlag)

	// 指定区块
	if *blockNumberFlag > 0 {
		num := big.NewInt(0).SetUint64(*blockNumberFlag)
		block, err := fetchBlockWithRetry(ctx, client, num, 3)
		if err != nil {
			log.Fatalf("failed to get block %d: %v", *blockNumberFlag, err)
		}
		printBlockInfo(fmt.Sprintf("Block %d", *blockNumberFlag), block)
	}

	// 批量查询区块范围
	if *rangeStartFlag > 0 && *rangeEndFlag > 0 {
		if *rangeStartFlag > *rangeEndFlag {
			log.Fatal("range-start must be <= range-end")
		}
		rateLimit := time.Duration(*rateLimitFlag) * time.Millisecond
		fetchBlockRange(ctx, client, *rangeStartFlag, *rangeEndFlag, rateLimit)
	}

}

// fetchBlockWithRetry 带重试机制的区块查询
func fetchBlockWithRetry(ctx context.Context, client *ethclient.Client, blockNumber *big.Int, maxRetries int) (*types.Block, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// 每次重试使用新的超时上下文，避免上下文被取消
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		block, err := client.BlockByNumber(reqCtx, blockNumber)
		cancel()

		if err == nil {
			return block, nil
		}

		lastErr = err
		if i < maxRetries-1 {
			backoff := time.Duration(i+1) * 500 * time.Millisecond
			log.Printf("[WARN] failed to fetch block %s, retry %d/%d after %v: %v",
				blockNumber.String(), i+1, maxRetries, backoff, err)
			time.Sleep(backoff)
		}
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// fetchBlockRange 批量查询区块范围，带频率控制
func fetchBlockRange(ctx context.Context, client *ethclient.Client, start, end uint64, rateLimit time.Duration) {
	fmt.Printf("\n=== Fetching Block Range [%d, %d] ===\n", start, end)
	fmt.Printf("Rate Limit: %v per request\n\n", rateLimit)

	successCount := 0
	skipCount := 0
	ticker := time.NewTicker(rateLimit)
	defer ticker.Stop()

	for num := start; num <= end; num++ {
		// 等待速率限制
		<-ticker.C

		blockNumber := big.NewInt(0).SetUint64(num)
		block, err := fetchBlockWithRetry(ctx, client, blockNumber, 2)

		if err != nil {
			log.Printf("[ERROR] Block %d: %v", num, err)
			skipCount++
			continue
		}

		successCount++
		printBlockInfo(fmt.Sprintf("Block %d", num), block)

		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			log.Printf("[INFO] Context cancelled, stopping at block %d", num)
			return
		default:
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Success: %d blocks\n", successCount)
	fmt.Printf("Skipped: %d blocks\n", skipCount)
	fmt.Printf("Total: %d blocks\n", end-start+1)
}

// 打印详细的区块信息
func printBlockInfo(title string, block *types.Block) {
	fmt.Println("======================================")
	fmt.Println(title)
	fmt.Println("======================================")
	fmt.Printf("Block: %+v\n", block)

	// 基本信息
	fmt.Printf("区块号Number       : %d\n", block.Number().Uint64())
	fmt.Printf("区块Hash         : %s\n", block.Hash().Hex())
	fmt.Printf("上一级区块号Parent Hash  : %s\n", block.ParentHash().Hex())

	// 时间信息
	fmt.Printf(" block.Time : %d\n", block.Time())
	//将区块时间戳转换为Go的time.Time对象 纳秒部分为0
	blockTime := time.Unix(int64(block.Time()), 0)
	fmt.Printf("Time         : %s\n", blockTime.Format(time.RFC3339))
	//2006 代表年份 01 代表月份 02 代表日期 15 代表小时（24小时制）  04 代表分钟 05 代表秒 MST 代表时区名称
	fmt.Printf("Time (Local) : %s\n", blockTime.Local().Format("2006-01-02 15:04:05 MST"))

	// Gas 信息
	//获取区块中实际使用的gas总量
	gasUsed := block.GasUsed()
	//获取区块的gas上限
	gasLimit := block.GasLimit()
	fmt.Printf("Gas Used       : %d\n", gasUsed)
	fmt.Printf("Gas Limit      : %d\n", gasLimit)
	//计算使用率, gas使用百分比
	gasUsagePercent := float64(gasUsed) / float64(gasLimit) * 100
	fmt.Println("gasUsagePercent    : ", gasUsagePercent)
	fmt.Printf("Gas Used     : %d (%.4f%%)\n", gasUsed, gasUsagePercent)
	fmt.Printf("Gas Limit    : %d\n", gasLimit)
	// 判断区块使用情况
	fmt.Printf("  区块状态: ")
	if gasUsagePercent > 95 {
		fmt.Println("🔥 高度饱和 (网络繁忙)")
	} else if gasUsagePercent > 75 {
		fmt.Println("⚠️  中度饱和")
	} else if gasUsagePercent > 50 {
		fmt.Println("⚡ 正常使用")
	} else {
		fmt.Println("✅ 空闲状态")
	}

	// 交易信息, 区块中包含的交易数量
	//block.Transactions() 返回区块中所有交易的切片（slice）
	txCount := len(block.Transactions())
	fmt.Printf("区块中包含的交易数量Tx Count     : %d\n", txCount)

	// 区块根信息（Merkle 树根）
	fmt.Printf("全局状态树的根哈希State Root   : %s\n", block.Root().Hex())
	//代表该区块中所有交易的Merkle树根哈希
	//用于快速验证某笔交易是否包含在区块中
	fmt.Printf("交易树的根哈希Tx Root      : %s\n", block.TxHash().Hex())
	//代表该区块中所有交易收据的Merkle树根哈希
	//交易收据包含交易执行结果：gas使用量、日志、状态等
	fmt.Printf("收据树的根哈希Receipt Root : %s\n", block.ReceiptHash().Hex())

	// 区块大小估算（简化版，实际大小还包括其他字段）
	if txCount > 0 {
		fmt.Printf("\nFirst Tx Hash: %s\n", block.Transactions()[0].Hash().Hex())
		if txCount > 1 {
			fmt.Printf("Last Tx Hash : %s\n", block.Transactions()[txCount-1].Hash().Hex())
		}
	}

	// 难度信息（PoW 相关，PoS 后基本固定）
	// 这是一个非常大的整数，表示找到有效区块哈希的难度
	fmt.Printf("当前区块难度Difficulty   : %s\n", block.Difficulty().String())

	// 获取Nonce（uint64类型）
	nonce := block.Nonce()
	fmt.Println("--nonce:", nonce)
	nonceBigInt := big.NewInt(0).SetUint64(nonce)
	fmt.Println("--nonceBigInt:", nonceBigInt)

	// 判断区块类型（PoW还是PoS）
	if block.Difficulty().Sign() == 0 {
		fmt.Println("区块类型: 🏦 PoS (权益证明)")
		fmt.Printf("Nonce值: %d (PoS区块的Nonce固定为0)\n", nonce)
	} else {
		fmt.Println("区块类型: 🔨 PoW (工作量证明)")

		// 显示Nonce的多种格式
		fmt.Printf("Nonce值 (十进制): %d\n", nonce)
		fmt.Printf("Nonce值 (十六进制): 0x%x\n", nonce)
		fmt.Printf("Nonce值 (八进制): 0%o\n", nonce)
		fmt.Printf("Nonce值 (二进制): %b\n", nonce)

		// 显示64位表示
		fmt.Printf("Nonce (64位): 0x%016x\n", nonce)
	}

	// 区块奖励相关信息
	//获取的是矿工（区块生产者）的地址,在PoW中叫"矿工"，在PoS中叫"验证者"
	coinbase := block.Coinbase()
	//检查是否是零地址（空地址）
	if coinbase != (common.Address{}) {
		fmt.Printf("区块生产者地址Coinbase     : %s\n", coinbase.Hex())
	}

	fmt.Println("======================================")
	fmt.Println()

}
