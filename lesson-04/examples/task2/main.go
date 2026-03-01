package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"

	"task2/contract" // 导入生成的绑定包

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv" // 用于加载 .env 文件
)

func main() {
	// 1. 加载 .env 文件
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	rpcURL := os.Getenv("SEPOLIA_RPC_URL")
	privateKeyHex := os.Getenv("PRIVATE_KEY")
	contractAddressHex := os.Getenv("CONTRACT_ADDRESS")

	fmt.Printf("rpcURL: %s\n", rpcURL)
	fmt.Printf("privateKeyHex: %s\n", privateKeyHex)
	fmt.Printf("contractAddressHex: %s\n", contractAddressHex)

	// 2. 连接 Sepolia 网络
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	fmt.Println("Connected to Sepolia network")

	// 3. 从私钥加载账户
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Failed to cast public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	fmt.Printf("Using account: %s\n", fromAddress.Hex())

	// 4. 获取链 ID（Sepolia 链 ID 是 11155111）
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		log.Fatalf("Failed to get network ID: %v", err)
	}
	fmt.Printf("Chain ID: %s\n", chainID.String())

	// 5. 创建交易授权对象（Auth）
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("Failed to create authorized transactor: %v", err)
	}
	// 设置 Gas 价格（可选，建议动态获取）
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	auth.GasPrice = gasPrice

	var counterInstance *contract.Contract

	// 6. 判断是部署新合约还是使用已有合约
	if contractAddressHex == "" {
		// 6a. 部署新合约
		fmt.Println("Deploying new Counter contract...")

		// 设置初始值，例如 100
		initialCount := big.NewInt(100)

		address, tx, instance, err := contract.DeployContract(
			auth,
			client,
			initialCount,
		)
		if err != nil {
			log.Fatalf("Failed to deploy contract: %v", err)
		}

		fmt.Printf("Contract deployed! Address: %s\n", address.Hex())
		fmt.Printf("Transaction hash: %s\n", tx.Hash().Hex())

		counterInstance = instance

		// 等待交易被确认（可选）
		// bind.WaitMined(context.Background(), client, tx)
	} else {
		// 6b. 加载已有合约
		contractAddress := common.HexToAddress(contractAddressHex)
		instance, err := contract.NewContract(contractAddress, client)
		if err != nil {
			log.Fatalf("Failed to load contract: %v", err)
		}
		fmt.Printf("Loaded contract at: %s\n", contractAddress.Hex())
		counterInstance = instance
	}

	// 7. 读取当前计数值
	count, err := counterInstance.GetCount(&bind.CallOpts{})
	if err != nil {
		log.Fatalf("Failed to get count: %v", err)
	}
	fmt.Printf("Current count: %s\n", count.String())

	// 8. 调用 increment 方法（需要发送交易）
	fmt.Println("Sending increment transaction...")
	tx, err := counterInstance.Increment(auth)
	if err != nil {
		log.Fatalf("Failed to increment: %v", err)
	}
	fmt.Printf("Increment transaction sent! Hash: %s\n", tx.Hash().Hex())

	// 9. 等待交易确认并读取新值（可选）
	// 生产环境建议等待确认
	// receipt, err := bind.WaitMined(context.Background(), client, tx)
	// if err != nil {
	//     log.Fatalf("Failed to wait for mining: %v", err)
	// }
	// fmt.Printf("Transaction mined! Block: %d\n", receipt.BlockNumber)

	// 10. 再次读取计数值（注意：因为状态未更新，需要等待区块确认）
	// 为了演示，这里添加短暂延迟并重新读取
	fmt.Println("Waiting for transaction to be mined...")
	// 在实际代码中应该使用 WaitMined，这里简化处理
	newCount, err := counterInstance.GetCount(&bind.CallOpts{})
	if err != nil {
		log.Fatalf("Failed to get new count: %v", err)
	}
	fmt.Printf("New count after increment: %s\n", newCount.String())
}
