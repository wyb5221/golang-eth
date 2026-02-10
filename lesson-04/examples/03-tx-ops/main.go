package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 支持两种操作模式：
// 1. 查询交易：--tx <hash> - 按哈希查询交易与回执，解析关键字段
// 2. 发送交易：--send --to <address> --amount <eth> - 发起 ETH 转账交易
// 使用本地私链的话，默认当前账户是第一个
func main() {
	//获取启动命令行中参数名称: `-tx` 的值
	txHashHex := flag.String("tx", "", "transaction hash (for query mode)")
	//判断启动命令行中参数名称 --send，有返回true，否则返回false
	sendMode := flag.Bool("send", false, "enable send transaction mode")
	toAddrHex := flag.String("to", "", "recipient address (required for send mode)")
	amountEth := flag.Float64("amount", 0, "amount in ETH (required for send mode)")
	flag.Parse()

	// 判断操作模式
	if *sendMode {
		// 发送交易模式
		if *toAddrHex == "" || *amountEth <= 0 {
			log.Fatal("send mode requires --to and --amount flags")
		}
		sendTransaction(*toAddrHex, *amountEth)
	} else {
		// 查询交易模式
		if *txHashHex == "" {
			log.Fatal("query mode requires --tx flag, or use --send for send mode")
		}
		queryTransaction(*txHashHex)
	}
}

// 发送交易
func sendTransaction(toAddrHex string, amountEth float64) {
	//获取地址
	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		log.Fatal("ETH_RPC_URL is not set")
	}
	//获取私钥
	privKeyHex := os.Getenv("SENDER_PRIVATE_KEY")
	if privKeyHex == "" {
		log.Fatal("SENDER_PRIVATE_KEY is not set (required for send mode)")
	}
	//context.WithTimeout 创建一个新的上下文，会在指定时间后自动取消,30 秒超时：防止网络连接问题导致程序无限等待
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // defer 确保函数退出时取消上下文，释放资源
	//使用 DialContext 连接以太坊节点
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer client.Close() // defer 确保函数退出时关闭连接

	// 解析私钥， trim0x 移除十六进制字符串前缀 "0x"
	privKey, err := crypto.HexToECDSA(trim0x(privKeyHex))
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}

	// 获取发送方地址
	publicKey := privKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}
	//获取转账账户
	fromAddr := crypto.PubkeyToAddress(*publicKeyECDSA)
	//获取转账接收账户
	toAddr := common.HexToAddress(toAddrHex)

	// 获取链 ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("failed to get chain id: %v", err)
	}

	// 获取 nonce
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		log.Fatalf("failed to get nonce: %v", err)
	}

	// 获取建议的 Gas 价格（使用 EIP-1559 动态费用）
	gasTipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatalf("failed to get gas tip cap: %v", err)
	}

	// 获取 base fee，计算 fee cap
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("failed to get header: %v", err)
	}

	baseFee := header.BaseFee
	if baseFee == nil {
		// 如果不支持 EIP-1559，使用传统 gas price
		gasPrice, err := client.SuggestGasPrice(ctx)
		if err != nil {
			log.Fatalf("failed to get gas price: %v", err)
		}
		baseFee = gasPrice
	}

	// fee cap = base fee * 2 + tip cap（简单策略）
	gasFeeCap := new(big.Int).Add(
		new(big.Int).Mul(baseFee, big.NewInt(2)),
		gasTipCap,
	)

	// 估算 Gas Limit（普通转账固定为 21000）
	gasLimit := uint64(21000)

	// 转换 ETH 金额为 Wei
	// amountEth * 1e18
	amountWei := new(big.Float).Mul(
		big.NewFloat(amountEth),
		big.NewFloat(1e18),
	)
	valueWei, _ := amountWei.Int(nil)

	// 检查余额是否足够
	balance, err := client.BalanceAt(ctx, fromAddr, nil)
	if err != nil {
		log.Fatalf("failed to get balance: %v", err)
	}

	// 计算总费用：value + gasFeeCap * gasLimit
	totalCost := new(big.Int).Add(
		valueWei,
		new(big.Int).Mul(gasFeeCap, big.NewInt(int64(gasLimit))),
	)

	if balance.Cmp(totalCost) < 0 {
		log.Fatalf("insufficient balance: have %s wei, need %s wei", balance.String(), totalCost.String())
	}

	// 构造交易（EIP-1559 动态费用交易）
	txData := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &toAddr,
		Value:     valueWei,
		Data:      nil,
	}
	tx := types.NewTx(txData)

	// 签名交易
	signer := types.NewLondonSigner(chainID)
	signedTx, err := types.SignTx(tx, signer, privKey)
	if err != nil {
		log.Fatalf("failed to sign transaction: %v", err)
	}

	// 发送交易
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		log.Fatalf("failed to send transaction: %v", err)
	}

	// 输出交易信息
	fmt.Println("=== Transaction Sent ===")
	fmt.Printf("From       : %s\n", fromAddr.Hex())
	fmt.Printf("To         : %s\n", toAddr.Hex())
	fmt.Printf("Value      : %s ETH (%s Wei)\n", fmt.Sprintf("%.6f", amountEth), valueWei.String())
	fmt.Printf("Gas Limit  : %d\n", gasLimit)
	fmt.Printf("Gas Tip Cap: %s Wei\n", gasTipCap.String())
	fmt.Printf("Gas Fee Cap: %s Wei\n", gasFeeCap.String())
	fmt.Printf("Nonce      : %d\n", nonce)
	fmt.Printf("Tx Hash    : %s\n", signedTx.Hash().Hex())
	fmt.Println("\nTransaction is pending. Use --tx flag to query status:")
	fmt.Printf("  go run main.go --tx %s\n", signedTx.Hash().Hex())
}

// 查询交易
func queryTransaction(txHashHex string) {
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
	defer client.Close() // defer 确保函数退出时取消上下文，释放资源

	//获取当前连接的以太坊网络的链 ID,
	//链 ID 用于识别不同的以太坊网络（如主网=1，Sepolia测试网=11155111）
	chainID, err := client.ChainID(ctx)
	fmt.Printf("连接成功，链ID：%s\n", chainID.String())

	txHash := common.HexToHash(txHashHex)

	//获取交易的基础详情（谁发给谁，发了什么）
	// tx: 交易对象本身，包含交易的所有原始信息
	// isPending: 一个布尔值，表示交易是否还在等待被打包（处于待处理状态）
	tx, isPending, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		log.Fatalf("failed to get transaction: %v", err)
	}

	fmt.Println("=== Transaction ===")
	// 输出交易基本信息
	printTxBasicInfo(tx, isPending)

	// 获取交易的执行结果（成功了吗，花了多少gas）
	//收据只有在交易已被打包进区块后才会生成
	// receipt: 交易收据对象，包含交易执行后的结果
	// err: 错误信息，如果交易未被打包或不存在会报错（常用于判断是否确认）
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		log.Printf("failed to get receipt (maybe pending): %v", err)
		return
	}

	fmt.Println("=== Receipt ===")
	//交易的执行结果
	printReceiptInfo(receipt)
}

// 输出交易基本信息
func printTxBasicInfo(tx *types.Transaction, isPending bool) {
	// 交易哈希：这笔交易的唯一ID
	fmt.Printf("交易哈希Hash        : %s\n", tx.Hash().Hex())
	// 账户Nonce：发送方地址的交易序号。用于防止重放攻击和确保交易顺序。从0开始，每发一笔交易递增1。
	fmt.Printf("Nonce       : %d\n", tx.Nonce())
	//限制：发送方愿意为执行此交易支付的最大gas。如果交易执行所需超过此值，交易会失败并消耗所有已用燃气。
	fmt.Printf("Gas         : %d\n", tx.Gas())
	// gas价格：发送方愿意为每单位燃气支付的价格（以Wei为单位）
	fmt.Printf("Gas Price   : %s\n", tx.GasPrice().String())
	// 接收方地址：资金或合约调用发送到的地址。如果为 nil，表示这是一个“合约创建交易”。
	fmt.Printf("To          : %v\n", tx.To())
	// 转账金额：从发送方转移到接收方的原生代币数量（以太坊上为ETH），单位是Wei（1 ETH = 10^18 Wei）
	fmt.Printf("Value (Wei) : %s\n", tx.Value().String())
	// 输入数据长度：调用智能合约函数时附带的参数数据，或合约创建时的初始化代码的长度（字节数）。普通转账此项为空。
	fmt.Printf("Data Len    : %d bytes\n", len(tx.Data()))
	// 交易状态：布尔值。true表示交易已广播但尚未被打包进区块（在内存池中）；false表示交易已被确认并记录在链上。
	fmt.Printf("Pending     : %v\n", isPending)
}

// 交易的执行结果
func printReceiptInfo(r *types.Receipt) {
	// 交易状态码：收据中最重要的字段。1 表示执行成功；0 表示执行失败（例如，燃气不足、合约逻辑报错）。失败交易也会被记录并消耗燃气。
	fmt.Printf("Status      : %d\n", r.Status)
	// 区块号：此交易被包含在哪个区块的高度。这标志着交易被正式确认的时刻。
	fmt.Printf("BlockNumber : %d\n", r.BlockNumber.Uint64())
	// 区块哈希：此交易所在区块的哈希。与BlockNumber一起唯一确定一个区块。
	fmt.Printf("BlockHash   : %s\n", r.BlockHash.Hex())
	// 交易索引：该交易在所属区块中的顺序位置（从0开始）。同一区块内的交易按此顺序执行。
	fmt.Printf("TxIndex     : %d\n", r.TransactionIndex)
	// 实际燃气消耗量：交易执行实际消耗的燃气数量。通常小于或等于交易中的Gas Limit(区块的gas上限)
	fmt.Printf("Gas Used    : %d\n", r.GasUsed)
	// 日志数量：交易执行过程中触发的智能合约事件的数量。每个事件（如代币转账）会生成一条日志。
	fmt.Printf("Logs        : %d\n", len(r.Logs))
	if len(r.Logs) > 0 {
		// 第一条日志的地址：如果交易产生了日志，这里打印第一条日志的发出者地址，通常是触发事件的智能合约地址。
		fmt.Printf("First Log Address : %s\n", r.Logs[0].Address.Hex())
	}
}

// trim0x 移除十六进制字符串前缀 "0x"
func trim0x(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}
