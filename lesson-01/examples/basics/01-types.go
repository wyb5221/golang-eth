package main

import (
	"fmt"
	"unsafe"
)

func main() {
	basicTypesDemo()
}

func basicTypesDemo() {
	fmt.Println("=== 基础类型示例 ===")

	// 布尔类型
	var isTrue bool = true
	var isFalse bool = false
	fmt.Printf("bool: %t\n", isTrue)
	fmt.Printf("bool: %t\n", isFalse)

	// 整数类型 - 有符号
	var age int = 25
	fmt.Printf("int: %d\n", age)

	// 不同长度的整数
	var count int8 = 127                      // 8位整数，范围: -128 到 127
	var number int16 = 30000                  // 16位整数，范围: -32768 到 32767
	var data int32 = 2000000000               // 32位整数，范围: -2^31 到 2^31-1
	var bigNumber int64 = 9223372036854775807 // 64位整数，范围: -2^63 到 2^63-1
	fmt.Printf("int8: %d, int16: %d, int32: %d, int64: %d\n", count, number, data, bigNumber)

	// 无符号整数
	var u8 uint8 = 255                    // 0 到 255
	var u16 uint16 = 65535                // 0 到 65535
	var u32 uint32 = 4294967295           // 0 到 2^32-1
	var u64 uint64 = 18446744073709551615 // 0  to 2^64-1
	fmt.Printf("uint8: %d, uint16: %d, uint32: %d, uint64: %d\n", u8, u16, u32, u64)

	// 类型别名
	var b byte = 65  // byte 是 uint8 的别名
	var r rune = '中' // rune 是 int32 的别名
	fmt.Printf("byte: %d (%c), rune: %d (%c)\n", b, b, r, r)

	// 显示类型占用的内存大小
	fmt.Printf("\n类型大小:\n")
	fmt.Printf("int8 size: %d bytes\n", unsafe.Sizeof(count))
	fmt.Printf("int16 size: %d bytes\n", unsafe.Sizeof(number))
	fmt.Printf("int32 size: %d bytes\n", unsafe.Sizeof(data))
	fmt.Printf("int64 size: %d bytes\n", unsafe.Sizeof(bigNumber))
	fmt.Printf("bool size: %d bytes\n", unsafe.Sizeof(isTrue))

}
