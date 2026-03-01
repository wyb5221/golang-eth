pragma solidity ^0.8.20;


//1、npm install -g solc  安装solidity编译器
//2、solcjs --abi --bin Counter.sol -o build/  编译合约，生成二进制文件到指定目录build
//3、安装abign：go install github.com/ethereum/go-ethereum/cmd/abigen@latest
// 查看abign 版本：abigen --version
//4、abigen --bin=build/Counter_sol_Counter.bin --abi=build/Counter_sol_Counter.abi --pkg=contract --out=contract/Counter.go
//5、安装依赖：
// go get github.com/ethereum/go-ethereum
// go get github.com/joho/godotenv
// go mod tidy
// go run main.go
contract Counter {
    uint256 public count;

    constructor(uint256 _initialCount) {
        count = _initialCount;
    }

    function increment() public {
        count += 1;
    }

    function getCount() public view returns (uint256) {
        return count;
    }
}