package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 06-subscribe-logs.go
// 订阅指定合约的日志事件（如 ERC-20 Transfer），并解析事件参数。
// 本示例展示了如何从 logs 中解析出事件，包括 indexed 参数和普通参数。

// ERC-20 标准 ABI（包含 Transfer 事件定义）
//https://cryptomus.com/zh/blog/everything-you-need-to-know-about-usdt-networks
//通过地址获取主网contract
//https://etherscan.io/   
//可以通过主网地址查询eth账户的转账信息
// 这里定义了 ERC-20 代币的两个标准事件：
// - Transfer: 代币转账事件
// - Approval: 代币授权事件
// 事件参数说明:
// - indexed: 可索引参数，存储在 Topics 中，可被高效过滤查询
// - non-indexed: 非索引参数，存储在 Data 字段中
const erc20ABIJSON = `[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "from", "type": "address"},
      {"indexed": true, "name": "to", "type": "address"},
      {"indexed": false, "name": "value", "type": "uint256"}
    ],
    "name": "Transfer",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "owner", "type": "address"},
      {"indexed": true, "name": "spender", "type": "address"},
      {"indexed": false, "name": "value", "type": "uint256"}
    ],
    "name": "Approval",
    "type": "event"
  }
]`

func main() {
	// 定义合约地址参数：要监听哪个智能合约的日志
	// - 参数名称: `-contract`
	// - 类型: 字符串指针
	// - 默认值: 空字符串
	// - 说明: 必须提供，否则程序退出
	contractAddr := flag.String("contract", "", "contract address to subscribe logs from (required)")
	flag.Parse()// 解析命令行参数，将用户输入绑定到变量
	// 验证合约地址参数是否提供
	if *contractAddr == "" {
		log.Fatal("missing --contract flag")
	}
	//从环境变量获取节点连接地址
	//优先获取 WebSocket URL（必须，因为日志订阅需要持久连接）
	rpcURL := os.Getenv("ETH_WS_URL")
	if rpcURL == "" {
		rpcURL = os.Getenv("ETH_RPC_URL")
	}
	if rpcURL == "" {
		log.Fatal("ETH_WS_URL or ETH_RPC_URL must be set")
	}
	//创建上下文和连接节点
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 连接以太坊节点
	// 注意: 如果使用 HTTP URL，SubscribeFilterLogs 会返回错误
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer client.Close() // 程序退出时关闭连接

	// 解析合约  ABI
	// 将 JSON 格式的 ABI 字符串解析为 abi.ABI 结构体
	// 这个结构体包含了事件的签名、参数类型等信息，用于后续的事件识别和参数解析
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABIJSON))
	if err != nil {
		log.Fatalf("failed to parse ABI: %v", err)
	}
	// 将入参字符串格式的合约地址转换为以太坊地址类型
	contract := common.HexToAddress(*contractAddr)
	//创建日志过滤查询
	// FilterQuery 定义了我们要订阅的日志过滤条件
	// 这里我们只过滤指定合约地址产生的所有日志
	query := ethereum.FilterQuery{
		Addresses: []common.Address{contract},// 只监听这一个合约地址
		// 还可以添加其他过滤条件:
		// - Topics: 过滤特定事件类型或特定参数
		// - FromBlock/ToBlock: 指定区块范围
	}
	// 创建日志通道：用于接收节点推送的原始日志
	logsCh := make(chan types.Log)
	// SubscribeFilterLogs: 订阅符合过滤条件的日志
	// 参数1: 上下文，用于取消订阅
	// 参数2: 过滤查询条件
	// 参数3: 接收日志的通道
	// 返回: 订阅对象，包含错误通道 Err()
	// 注意: 此功能必须使用 WebSocket 连接
	sub, err := client.SubscribeFilterLogs(ctx, query, logsCh)
	if err != nil {
		log.Fatalf("failed to subscribe logs: %v", err)
	}
	// 注意: 这里没有 defer sub.Unsubscribe()，因为程序退出时直接返回
	// 连接关闭时会自动取消订阅

	fmt.Printf("Subscribed to logs of contract %s via %s\n", contract.Hex(), rpcURL)
	fmt.Printf("Listening for events...\n\n")

	// 创建信号通道，用于接收 Ctrl+C 等退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	//主事件循环
	for {
		select {
		//收到新的日志事件
		case vLog := <-logsCh:
			// 解析日志事件，  将原始的 types.Log 解析为结构化的合约事件
			parseLogEvent(&vLog, parsedABI)
		//订阅发生错误（连接断开、节点问题等）
		case err := <-sub.Err():
			log.Printf("subscription error: %v", err)
			return
		//收到系统退出信号（用户按 Ctrl+C）
		case sig := <-sigCh:
			fmt.Printf("received signal %s, shutting down...\n", sig.String())
			return
		//上下文被取消
		case <-ctx.Done():
			fmt.Println("context cancelled, exiting...")
			return
		}
	}
}

//将原始的以太坊日志解析为结构化的合约事件
// parseLogEvent 解析日志事件，展示如何从 logs 中提取事件信息
func parseLogEvent(vLog *types.Log, parsedABI abi.ABI) {
	// 检查是否有 Topics（没有 Topics 的日志可能是无效的）
	if len(vLog.Topics) == 0 {
		return
	}

	// 步骤 1: 识别事件类型
	// Topics[0] 是事件签名的 keccak256 哈希值（固定长度32字节）
	// - Topics[1..]: indexed 参数（最多3个，每个32字节）
	// - Data: 非 indexed 参数（ABI编码，可变长度）
	// 例如: Transfer(address,address,uint256) 的哈希
	eventTopic := vLog.Topics[0]

	// 尝试识别是哪个事件（通过比较 Topics[0] 和事件签名的哈希）
	var eventName string
	var eventSig abi.Event

	// 遍历 ABI 中定义的所有事件，查找匹配的事件签名
	for name, event := range parsedABI.Events {
		// 计算事件的签名哈希
		// 例如: "Transfer(address,address,uint256)" 的 Keccak256 哈希
		eventSigHash := crypto.Keccak256Hash([]byte(event.Sig))
		// 比较哈希值是否匹配
		if eventSigHash == eventTopic {
			eventName = name
			eventSig = event
			break
		}
	}

	// 如果无法识别事件类型，打印原始信息并返回
	if eventName == "" {
		// 如果无法识别事件类型，打印原始信息
		fmt.Printf("[%s] Unknown Event - Block: %d, Tx: %s, Topic[0]: %s\n",
			time.Now().Format(time.RFC3339),
			vLog.BlockNumber,
			vLog.TxHash.Hex(),
			eventTopic.Hex(),
		)
		return
	}

	// 步骤 2: 解析事件参数
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("[%s] Event: %s\n", time.Now().Format(time.RFC3339), eventName)
	fmt.Printf("  Block Number: %d\n", vLog.BlockNumber) // 区块高度
	fmt.Printf("  Tx Hash     : %s\n", vLog.TxHash.Hex())// 交易哈希
	fmt.Printf("  Log Index   : %d\n", vLog.Index)		 // 日志索引（区块内）
	fmt.Printf("  Contract    : %s\n", vLog.Address.Hex())// 合约地址
	fmt.Printf("  Topics Count: %d\n", len(vLog.Topics))  // Topics 数量

	// 步骤 3: 解析 indexed 参数（从 Topics 中解析）
	// Topics[0] 是事件签名哈希，Topics[1..N] 是 indexed 参数
	// 注意：只有前 3 个 indexed 参数会放在 Topics 中（Ethereum 限制）
	fmt.Printf("\n  Indexed Parameters (from Topics):\n")

	// 关键概念：Topics 索引
	// - Topics[0]: 事件签名哈希（始终存在）
	// - Topics[1]: 第一个 indexed 参数
	// - Topics[2]: 第二个 indexed 参数
	// - Topics[3]: 第三个 indexed 参数
	// 注意：最多只能有 3 个 indexed 参数，这是以太坊协议的限制

	// Topics[0] 是事件签名，所以 indexed 参数从 Topics[1] 开始
	// 注意：topicIndex 只针对 indexed 参数计数，不考虑非 indexed 参数
	indexedParamIndex := 0	// 记录当前是第几个 indexed 参数
	for i, input := range eventSig.Inputs {
		// 只处理 indexed 参数
		if !input.Indexed {
			continue
		}
		// indexed 参数在 Topics 中的位置 = 1 + 已处理的 indexed 参数数量
		topicIndex := 1 + indexedParamIndex
		indexedParamIndex++
		// 检查 Topics 长度是否足够，判断是否已经获取了所有indexed参数了
		if topicIndex >= len(vLog.Topics) {
			continue
		}

		topic := vLog.Topics[topicIndex]
		fmt.Printf("    [%d] %s (%s): ", i+1, input.Name, input.Type)

		// 根据类型解析 indexed 参数
		// indexed 参数在 Topics 中总是以 32 字节的哈希值存储
		switch input.Type.T {
		case abi.AddressTy:
			// address 类型: 在 Topics 中是 32 字节，前12字节为0，后20字节是地址
			// address 类型：去除前 12 字节的 0 填充，后 20 字节是地址
			// 例如: 0x000000000000000000000000742d35Cc6634C0532925a3b844Bc9eC8d731e3e1
			addr := common.BytesToAddress(topic.Bytes())
			fmt.Printf("%s\n", addr.Hex())
		case abi.IntTy, abi.UintTy:
			// 整数类型：直接转换为 big.Int
			value := new(big.Int).SetBytes(topic.Bytes())
			fmt.Printf("%s\n", value.String())
		case abi.BoolTy:
			// bool 类型：检查最后一个字节
			fmt.Printf("%t\n", topic[31] != 0)
		case abi.BytesTy, abi.FixedBytesTy:
			// bytes 类型：直接显示十六进制
			fmt.Printf("%s\n", topic.Hex())
		default:
			// 其他类型：显示原始十六进制
			fmt.Printf("%s (raw)\n", topic.Hex())
		}
	}

	// 步骤 4: 解析非 indexed 参数（从 Data 字段中解析）
	// Data 字段包含所有非 indexed 参数的编码数据
	if len(vLog.Data) > 0 {
		fmt.Printf("\n  Non-Indexed Parameters (from Data):\n")

		// 创建一个结构体来接收解码后的参数
		// 注意：这里使用通用方法，实际应用中可能需要根据具体事件定义结构体
		nonIndexedInputs := make([]abi.Argument, 0)
		for _, input := range eventSig.Inputs {
			if !input.Indexed {
				nonIndexedInputs = append(nonIndexedInputs, input)
			}
		}

		if len(nonIndexedInputs) > 0 {
			// 使用 ABI 解码 Data 字段
			// 方法 1: 使用 UnpackIntoInterface（需要预定义结构体）
			// 方法 2: 使用 Unpack（返回 []interface{}）
			// Unpack 方法会根据事件名称和 Data 内容，解码出所有非 indexed 参数
			values, err := parsedABI.Unpack(eventName, vLog.Data)
			if err != nil {
				fmt.Printf("    Error decoding data: %v\n", err)
			} else {
				// 只输出非 indexed 参数
				nonIndexedIdx := 0
				for i, input := range eventSig.Inputs {
					if !input.Indexed {
						if nonIndexedIdx < len(values) {
							value := values[nonIndexedIdx]
							fmt.Printf("    [%d] %s (%s): ", i+1, input.Name, input.Type)

							// 根据类型格式化输出
							switch v := value.(type) {
							case *big.Int:
								fmt.Printf("%s\n", v.String())
							case common.Address:
								fmt.Printf("%s\n", v.Hex())
							case []byte:
								fmt.Printf("0x%x\n", v)
							default:
								fmt.Printf("%v\n", v)
							}
							nonIndexedIdx++
						}
					}
				}
			}
		}
	} else {
		fmt.Printf("\n  Non-Indexed Parameters: None\n")
	}

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}
