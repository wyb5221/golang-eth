package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {

	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		fmt.Printf("rpcURL is null \n")
	}
	//context.WithTimeout 创建一个新的上下文，会在指定时间后自动取消,10 秒超时：防止网络连接问题导致程序无限等待
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	//使用 DialContext 连接以太坊节点
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		fmt.Printf("failed to connect to Ethereum node: %v\n", err)
	}
	defer client.Close() // defer 确保函数退出时关闭客户端连接，释放网络资源
	//打印区块基本信息
	printInfo(client, ctx)

}

// 打印区块基本信息
func printInfo(client *ethclient.Client, ctx context.Context) {
	//获取当前连接的以太坊网络的链 ID,
	chainId, err := client.ChainID(ctx)
	fmt.Printf("连接成功，链ID：%s\n", chainId)
	if err != nil {
		fmt.Printf("failed to get chain id: %v\n", err)
	}

	blockNumberFlag := flag.Uint64("number", 0, "block number to query (0 means skip)")
	flag.Parse()

	fmt.Printf("入参blockNumberFlag:%p\n", blockNumberFlag)
	fmt.Printf("原始值: %d\n", *blockNumberFlag)

	var (
		block *types.Block
		err1  error
	)

	if *blockNumberFlag == 0 {
		block, err1 = client.BlockByNumber(ctx, nil)
	} else {
		//将 uint64 类型的数值转换为 *big.Int 类型
		num := big.NewInt(0).SetUint64(*blockNumberFlag)
		fmt.Printf("big.Int: %v\n", num)

		block, err1 = client.BlockByNumber(ctx, num)
	}

	if err != nil {
		fmt.Printf("failed to get block header: %v", err1)
	}
	fmt.Printf("区块号 Block number  : %d\n", block.NumberU64())
	fmt.Printf("区块Block Hash    : %s\n", block.Hash().Hex())
	fmt.Printf("时间戳block Time: %d\n", block.Time())
	fmt.Printf("Block Time    : %s\n", time.Unix(int64(block.Time()), 0).Format(time.RFC3339))
	//Transactions返回的是交易的hash切片数组
	txCount := len(block.Transactions())
	fmt.Printf("区块中包含的交易数量Tx Count     : %d\n", txCount)
	fmt.Printf("区块中包含的交易Transactions      : %v\n", block.Transactions())

}
