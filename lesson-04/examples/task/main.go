package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

func main() {

	//打印区块基本信息
	//printInfo()

	//发送交易
	sendTransaction()
}

// 打印区块基本信息
// func printInfo(client *ethclient.Client, ctx context.Context)
func printInfo() {
	err := godotenv.Load() // 加载 .env 文件中的环境变量
	if err != nil {
		fmt.Printf("failed to load .env file: %v\n", err)
	}
	rpcURL, exists := os.LookupEnv("ETH_RPC_URL")
	if !exists {
		fmt.Printf("ETH_RPC_URL environment variable is not set\n")
	}
	fmt.Printf("ETH_RPC_URL: %s\n", rpcURL)

	// rpcURL := os.Getenv("ETH_RPC_URL")
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
		fmt.Printf("---1---")
		block, err1 = client.BlockByNumber(ctx, nil)
	} else {
		fmt.Printf("---2---")
		//将 uint64 类型的数值转换为 *big.Int 类型
		num := big.NewInt(0).SetUint64(*blockNumberFlag)
		fmt.Printf("big.Int: %v\n", num)

		block, err1 = client.BlockByNumber(ctx, num)
	}

	if err1 != nil {
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

// 发送交易
func sendTransaction() {
	fmt.Println("------- Sending Transaction -------")
	err := godotenv.Load() // 加载 .env 文件中的环境变量
	if err != nil {
		fmt.Printf("failed to load .env file: %v\n", err)
	}
	infuraURL, exists := os.LookupEnv("INFURA_API_KEY")
	if !exists {
		fmt.Printf("infuraURL environment variable is not set\n")
	}
	fmt.Printf("infuraURL: %s\n", infuraURL)

	//获取私钥
	privKeyHex := os.Getenv("SENDER_PRIVATE_KEY")
	//	fmt.Printf("私钥原始值: %s\n", privKeyHex)
	if privKeyHex == "" {
		log.Fatal("SENDER_PRIVATE_KEY is not set (required for send mode)")
	}
	//context.WithTimeout 创建一个新的上下文，会在指定时间后自动取消,30 秒超时：防止网络连接问题导致程序无限等待
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // defer 确保函数退出时取消上下文，释放资源
	//使用 DialContext 连接以太坊节点
	client, err := ethclient.DialContext(ctx, infuraURL)
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
	fmt.Printf("获取转账账户fromAddr: %s\n", fromAddr)

	//获取转账接收账户
	toAddrHex := os.Getenv("TO_ADDRESS")
	toAddr := common.HexToAddress(toAddrHex)
	fmt.Printf("转账接收账户toAddr: %s\n", toAddr)

	// 获取链 ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("failed to get chain id: %v", err)
	}
	fmt.Printf("链 ID chainID: %s\n", chainID)

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
	amountStr := os.Getenv("AMOUNT")
	if amountStr == "" {
		log.Fatal("AMOUNT environment variable is not set")
	}
	// 2. 转换为 float64
	amountEth, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		log.Fatalf("金额格式错误 '%s': %v", amountStr, err)
	}
	amountWei := new(big.Float).Mul(
		big.NewFloat(amountEth),
		big.NewFloat(1e18),
	)
	valueWei, _ := amountWei.Int(nil)

	// 检查余额是否足够
	balance, err := client.BalanceAt(ctx, fromAddr, nil)
	fmt.Printf("转账账户余额balance: %s\n", balance)
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
		ChainID:   chainID,   //链 ID
		Nonce:     nonce,     //账户Nonce：发送方地址的交易序号。用于防止重放攻击和确保交易顺序。从0开始，每发一笔交易递增1。
		GasTipCap: gasTipCap, // Gas Tip Cap：发送方愿意为每单位燃气支付的额外小费（以Wei为单位），激励矿工优先处理此交易
		GasFeeCap: gasFeeCap, // Gas Fee Cap：发送方愿意为每单位燃气支付的最高费用（以Wei为单位）。如果交易执行所需的费用超过此值，交易会失败并消耗所有已用燃气。
		Gas:       gasLimit,  // Gas Limit：发送方愿意为执行此交易支付的最大gas。如果交易执行所需超过此值，交易会失败并消耗所有已用燃气。
		To:        &toAddr,   // 接收方地址：资金或合约调用发送到的地址。如果为 nil，表示这是一个“合约创建交易”。
		Value:     valueWei,  // 转账金额：从发送方转移到接收方的原生代币数量（以太坊上为ETH），单位是Wei（1 ETH = 10^18 Wei）
		Data:      nil,       // 输入数据：调用智能合约函数时附带的参数数据，或合约创建时的初始化代码。普通转账此项为空。
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

// trim0x 移除十六进制字符串前缀 "0x"
func trim0x(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}
