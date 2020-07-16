package utils

import (
	"encoding/binary"
	"unsafe"
)

// 系统是否为小端编码
func IsLittleEndian() bool {
	var i int32 = 0x01020304
	u := unsafe.Pointer(&i)
	pb := (*byte)(u)
	b := *pb
	return b == 0x04
}

// 系统的字节序接口
func SystemByteOrder() binary.ByteOrder {
	if IsLittleEndian() {
		return binary.LittleEndian
	} else {
		return binary.BigEndian
	}
}
