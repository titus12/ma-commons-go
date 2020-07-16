package utils

import (
	"encoding/base64"
	"encoding/hex"

	uuid "github.com/satori/go.uuid"
)

func genUuid() []byte {
	u := uuid.NewV4()
	return u.Bytes()
}

// 生成uuid
func GenUuid() string {
	byteuuid := genUuid()
	buf := make([]byte, len(byteuuid)*2)
	hex.Encode(buf, genUuid())
	return string(buf)
}

//
// 生成uuid，给出16位=>8个字节的字节数组
func GenUuidWithByteArray16() []byte {
	uuid := genUuid()
	uuid = Md5WithByteArray(uuid)
	uuid = uuid[4:12]
	return uuid
}

func GenUuidWithUint64() uint64 {
	b := GenUuidWithByteArray16()
	return SystemByteOrder().Uint64(b)
}

// 生成uuid, 给出16位(8个字)的base64代表的字符串
func GenUuidWithBase64String16() string {
	b := GenUuidWithByteArray16()
	return base64.StdEncoding.EncodeToString(b)
}

// 生成uuid, 给出16位(8个字节的)的hex代表的字符串
func GenUuidWithHexString16() string {
	b := GenUuidWithByteArray16()
	return hex.EncodeToString(b)
}

func GenUuidWithByteArray32() []byte {
	uuid := genUuid()
	return uuid
}

func GenUuidWithBase64String32() string {
	b := GenUuidWithByteArray32()
	return base64.StdEncoding.EncodeToString(b)
}

func GenUuidWithHexString32() string {
	b := GenUuidWithByteArray32()
	return hex.EncodeToString(b)
}
