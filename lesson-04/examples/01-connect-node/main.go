package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

/**
 * 1、获取ETH_RPC_URL地址 在https://chainlist.org/?search=sepolia&testnets=true 地址获取
 * 2、设置环境变量：执行脚本：$env:ETH_RPC_URL="https://sepolia.gateway.tenderly.co"
 * 3、运行代码：go run main.go
 */

func main() {
	rpcURL := os.Getenv("ETH_RPC_URL")
	fmt.Printf("eth rpc 地址rpcURL：%s\n", rpcURL)

	if rpcURL == "" {
		log.Fatal("ETH_RPC_URL is not set")
	}
	//context.WithTimeout 创建一个新的上下文，会在指定时间后自动取消,10 秒超时：防止网络连接问题导致程序无限等待
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // defer 确保函数退出时取消上下文，释放资源
	fmt.Println("连接以太坊节点...")

	//使用 DialContext 连接以太坊节点
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatal("failed to connect to Ethereum node: %v", err)
	}
	defer client.Close() // defer 确保函数退出时关闭客户端连接，释放网络资源

	//获取当前连接的以太坊网络的链 ID,
	//链 ID 用于识别不同的以太坊网络（如主网=1，Sepolia测试网=11155111）
	chainID, err := client.ChainID(ctx)
	fmt.Printf("连接成功，链ID：%s\n", chainID.String())
	if err != nil {
		log.Fatalf("failed to get chain id: %v", err)
	}
	//获取最新的区块头信息，HeaderByNumber 传入 nil 表示获取最新的区块
	//区块头包含区块号、时间戳、父哈希等信息
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("failed to get latest block header: %v", err)
	}
	fmt.Printf("最新区块号: %d\n", header.Number)

	fmt.Println("=== Ethereum Node Info ===")
	fmt.Printf("RPC URL       : %s\n", rpcURL)
	fmt.Printf("Chain ID      : %s\n", chainID.String())
	fmt.Println("\n⚠️  注意: 'Latest' 区块是节点当前认为的最新区块，可能尚未被所有节点确认")
	fmt.Println("   不同RPC节点可能返回不同的 'latest' 区块，导致与浏览器不匹配")
	fmt.Println("   建议对比 'Safe' 或 'Finalized' 区块（已确认的区块）")
	fmt.Println()
	fmt.Printf("Latest Block  : %d\n", header.Number.Uint64())
	fmt.Printf("Block Hash    : %s\n", header.Hash().Hex())
	fmt.Printf("Block Time    : %s\n", time.Unix(int64(header.Time), 0).Format(time.RFC3339))
	fmt.Println("==========================")

	fmt.Println("\n=== 方法2: 直接调用 RPC ===")
	rpcHash := getHashFromRPC(client, "latest")
	fmt.Printf("RPC 直接返回的哈希:    %s\n", rpcHash.Hex())

	//也可以获取任意指定高度的区块头
	if header.Number.Uint64() > 0 {
		//计算上一个区块的区块号：当前区块号 - 1
		num := new(big.Int).Sub(header.Number, big.NewInt(1))
		// 获取上一个区块的区块头信息
		prevHeader, err := client.HeaderByNumber(ctx, num)
		if err == nil {
			fmt.Printf("Prev Block    : %d (%s)\n", prevHeader.Number.Uint64(), prevHeader.Hash().Hex())
		}
	}
	fmt.Println("=============================")

	fmt.Println("\n=== 最近5个区块 ===")
	for i := 0; i < 5; i++ {
		blockNum := new(big.Int).Sub(header.Number, big.NewInt(int64(i)))
		if blockNum.Sign() < 0 { // 检查是否小于0
			break
		}

		blockHeader, err := client.HeaderByNumber(ctx, blockNum)
		if err != nil {
			break
		}

		age := time.Since(time.Unix(int64(blockHeader.Time), 0))
		fmt.Printf("区块 %d: 哈希 %s... 年龄 %v\n",
			blockNum.Uint64(),
			blockHeader.Hash().Hex()[:16], // 只显示前16个字符
			age.Round(time.Second))
	}

	fmt.Println("=============================")

	// 获取 Gas 价格
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err == nil {
		// 将 wei 转换为 gwei（1 gwei = 10^9 wei）
		gasPriceGwei := float64(gasPrice.Uint64()) / 1_000_000_000
		fmt.Printf("✓ 建议 Gas 价格: %.2f gwei\n", gasPriceGwei)
	}

	//查询 10000000 区块
	tHeader, tHash, terr := getBlockByTag(ctx, client, "10000000")
	if err != nil {
		log.Printf("failed to get 10000000 block: %v (this may not be supported by all nodes)", terr)
	} else {
		fmt.Println("\n=== Safe Block (推荐对比) ===")
		fmt.Printf("Block Number  : %d\n", tHeader.Number.Uint64())
		fmt.Printf("Block Hash    : %s (RPC提供的hash，与浏览器一致)\n", tHash.Hex())
		fmt.Printf("Calculated    : %s (计算出的hash，可能不匹配)\n", tHeader.Hash().Hex())
		fmt.Printf("Block Time    : %s\n", time.Unix(int64(tHeader.Time), 0).Format(time.RFC3339))
		fmt.Printf("Confirmations : %d\n", header.Number.Uint64()-tHeader.Number.Uint64())
		fmt.Println("=============================")
	}

	//查询 safe 区块（浏览器通常显示这个）
	safeHeader, safeHash, err := getBlockByTag(ctx, client, "safe")
	if err != nil {
		log.Printf("failed to get safe block: %v (this may not be supported by all nodes)", err)
	} else {
		fmt.Println("\n=== Safe Block (推荐对比) ===")
		fmt.Printf("Block Number  : %d\n", safeHeader.Number.Uint64())
		fmt.Printf("Block Hash    : %s (RPC提供的hash，与浏览器一致)\n", safeHash.Hex())
		fmt.Printf("Calculated    : %s (计算出的hash，可能不匹配)\n", safeHeader.Hash().Hex())
		fmt.Printf("Block Time    : %s\n", time.Unix(int64(safeHeader.Time), 0).Format(time.RFC3339))
		fmt.Printf("Confirmations : %d\n", header.Number.Uint64()-safeHeader.Number.Uint64())
		fmt.Println("=============================")
	}

	//查询 finalized 区块
	finalizedHeader, finalizedHash, err := getBlockByTag(ctx, client, "finalized")
	if err != nil {
		log.Printf("failed to get finalized block: %v (this may not be supported by all nodes)", err)
	} else {
		fmt.Println("\n=== Finalized Block ===")
		fmt.Printf("Block Number  : %d\n", finalizedHeader.Number.Uint64())
		fmt.Printf("Block Hash    : %s (RPC提供的hash，与浏览器一致)\n", finalizedHash.Hex())
		fmt.Printf("Calculated    : %s (计算出的hash，可能不匹配)\n", finalizedHeader.Hash().Hex())
		fmt.Printf("Block Time    : %s\n", time.Unix(int64(finalizedHeader.Time), 0).Format(time.RFC3339))
		fmt.Printf("Confirmations : %d\n", header.Number.Uint64()-finalizedHeader.Number.Uint64())
		fmt.Println("========================")
	}
}

// getBlockByTag 查询指定标签的区块头（safe, finalized, latest 等）
// 返回 Header、RPC 提供的 Hash 和错误
// 注意：需要使用底层 RPC 调用，因为 ethclient 的高级 API 不直接支持这些标签
func getBlockByTag(ctx context.Context, client *ethclient.Client, tag string) (*types.Header, common.Hash, error) {
	//步骤1: 获取底层 RPC 客户端
	//ethclient.Client() 返回底层的 RPC 客户端，这样可以直接调用原始的 JSON-RPC 方法
	rpcClient := client.Client()

	//步骤2: 调用原始 JSON-RPC 方法获取区块数据
	//raw 用于接收原始的 JSON 响应数据
	var raw json.RawMessage
	// CallContext 执行 JSON-RPC 调用,
	// ctx: 上下文，控制超时, &raw: 响应数据的存储地址, "eth_getBlockByNumber": RPC 方法名, tag: 区块标签或区块号
	//false: 是否包含完整的交易信息（false 表示只获取区块头，不包含交易）
	err := rpcClient.CallContext(ctx, &raw, "eth_getBlockByNumber", tag, false)
	if err != nil {
		//返回自定义错误，使用 %w 包裹原始错误，方便错误链追踪
		return nil, common.Hash{}, fmt.Errorf("RPC call failed: %w", err)
	}
	//检查返回的数据是否为空
	if len(raw) == 0 || string(raw) == "null" {
		return nil, common.Hash{}, fmt.Errorf("%s block not found", tag)
	}
	fmt.Println("raw block data:", string(raw))

	// 步骤3: 定义区块数据解析结构体
	// 解析完整的区块头字段，这个结构体映射 JSON-RPC 返回的字段
	var blockData struct {
		Number      string         `json:"number"`
		Hash        common.Hash    `json:"hash"`
		ParentHash  common.Hash    `json:"parentHash"`
		UncleHash   common.Hash    `json:"sha3Uncles"`
		Coinbase    common.Address `json:"miner"`
		Root        common.Hash    `json:"stateRoot"`
		TxHash      common.Hash    `json:"transactionsRoot"`
		ReceiptHash common.Hash    `json:"receiptsRoot"`
		Bloom       hexutil.Bytes  `json:"logsBloom"`
		Difficulty  *hexutil.Big   `json:"difficulty"`
		GasLimit    hexutil.Uint64 `json:"gasLimit"`
		GasUsed     hexutil.Uint64 `json:"gasUsed"`
		Time        hexutil.Uint64 `json:"timestamp"`
		Extra       hexutil.Bytes  `json:"extraData"`
		MixDigest   common.Hash    `json:"mixHash"`
		Nonce       hexutil.Bytes  `json:"nonce"`
		BaseFee     *hexutil.Big   `json:"baseFeePerGas"`
	}
	//解析 JSON 数据到结构体
	if err := json.Unmarshal(raw, &blockData); err != nil {
		return nil, common.Hash{}, fmt.Errorf("failed to unmarshal block header: %w", err)
	}

	//步骤4: 解析区块号

	// 区块号在 JSON-RPC 中以十六进制字符串返回，如 "0x1a4b2"
	// 需要去掉 "0x" 前缀，然后按16进制解析

	// 检查字符串是否以 "0x" 开头且长度足够
	if !strings.HasPrefix(blockData.Number, "0x") {
		return nil, common.Hash{}, fmt.Errorf("invalid block number format: %s", blockData.Number)
	}
	// 解析十六进制字符串为 big.Int
	// blockData.Number[2:] 去掉 "0x" 前缀
	// 16 表示十六进制
	num, ok := new(big.Int).SetString(blockData.Number[2:], 16)
	if !ok {
		return nil, common.Hash{}, fmt.Errorf("invalid block number: %s", blockData.Number)
	}

	// 步骤5: 构造 types.Header 结构体

	// 构造完整的 Header
	header := &types.Header{
		ParentHash:  blockData.ParentHash,
		UncleHash:   blockData.UncleHash,
		Coinbase:    blockData.Coinbase,
		Root:        blockData.Root,
		TxHash:      blockData.TxHash,
		ReceiptHash: blockData.ReceiptHash,
		Bloom:       types.BytesToBloom(blockData.Bloom),
		Difficulty:  big.NewInt(0),
		Number:      num,
		GasLimit:    uint64(blockData.GasLimit),
		GasUsed:     uint64(blockData.GasUsed),
		Time:        uint64(blockData.Time),
		Extra:       blockData.Extra,
		MixDigest:   blockData.MixDigest,
		BaseFee:     nil,
	}

	// 步骤6: 设置额外的字段
	// 设置 Difficulty,  设置难度值（工作量证明的重要参数）
	if blockData.Difficulty != nil {
		header.Difficulty = blockData.Difficulty.ToInt()
	}

	// 设置 BaseFee（EIP-1559）,设置基础费用（EIP-1559 引入，用于交易费市场）
	if blockData.BaseFee != nil {
		header.BaseFee = blockData.BaseFee.ToInt()
	}

	// 设置 Nonce 随机数,  Nonce 是8字节（64位）的随机数，用于工作量证明
	if len(blockData.Nonce) >= 8 {
		var nonceBytes [8]byte
		copy(nonceBytes[:], blockData.Nonce[:8]) // 复制前8个字节
		header.Nonce = types.BlockNonce(nonceBytes)
	}

	// 步骤7: 返回结果
	// 返回 Header 和 RPC 提供的 hash
	// 注意：手动构造的 Header 计算出的 hash 可能不准确，因为：
	// 1. RPC 返回的某些字段可能格式不完全匹配 go-ethereum 的内部格式, blockData.Hash是RPC接口返回的区块哈希
	// 2. Header 的内部缓存字段可能未正确初始化
	// 因此，我们应该直接使用 RPC 返回的 hash，它与浏览器显示的 hash 一致
	return header, blockData.Hash, nil

}

// 直接调用 RPC 获取哈希
func getHashFromRPC(client *ethclient.Client, tag string) common.Hash {
	rpcClient := client.Client()

	var result struct {
		Hash common.Hash `json:"hash"`
	}

	//err := rpcClient.Call(context.Background(), &result, "eth_getBlockByNumber", tag, false)
	// ✅ 正确的写法
	err := rpcClient.Call(&result, "eth_getBlockByNumber", tag, false)

	if err != nil {
		log.Fatal(err)
	}

	return result.Hash
}
