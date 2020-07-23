package main

import (
	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/setting"
	"github.com/titus12/ma-commons-go/testconsole"
	_ "github.com/titus12/ma-commons-go/testconsole/testmsg"
	"github.com/titus12/ma-commons-go/wlog"
	"runtime"
)

func main() {

	//arr1 := utils.GenUuidWithByteArray16()
	//key := utils.GenUuidWithByteArray16()
	//
	//
	//arr := XorEncode(arr1, key)
	//
	//arr2 := XorDecode(arr, key)
	//
	//fmt.Println(arr2)


	setting.Initialize()

	numCpu := runtime.NumCPU()
	runtime.GOMAXPROCS(numCpu)


	if setting.TestConsole {
		console := testconsole.NewConsole()
		console.Command("LocalRun", testconsole.LocalRunRequest)
		console.Command("LocalRunPending", testconsole.LocalRunPendingRequest)
		console.Command("RunMsg", testconsole.RunMsgRequest)
		console.Command("mreq", testconsole.MultiMsgRequest)
		console.Command("query", testconsole.QueryRequest)
		console.Run()
	} else {
		wlog.Initialize(logrus.DebugLevel, wlog.WithELK([]string{"127.0.0.1:9092"}, setting.Key, "game-log"))
		if setting.Test {
			testconsole.Example()
		} else {
			logrus.Panic("斩时不支持非测试起动...")
			//example()
		}
	}
}


//func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
//	padding := blockSize - len(ciphertext)%blockSize
//	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
//	return append(ciphertext, padtext...)
//}
//
//func PKCS5UnPadding(origData []byte) []byte {
//	length := len(origData)
//	unpadding := int(origData[length-1])
//	return origData[:(length - unpadding)]
//}
//
//
//
//func DesEncrypt(origData, key []byte) ([]byte, error) {
//
//	block, err := des.NewCipher(key)
//	if err != nil {
//		return nil, err
//	}
//	origData = PKCS5Padding(origData, block.BlockSize())
//	blockMode := cipher.NewCBCEncrypter(block, key)
//
//	crypted := make([]byte, len(origData))
//	blockMode.CryptBlocks(crypted, origData)
//	return crypted, nil
//
//}
//
//
//func DesDecrypt(crypted, key []byte) ([]byte, error) {
//	block, err := des.NewCipher(key)
//	if err != nil {
//		return nil, err
//	}
//
//	blockMode := cipher.NewCBCDecrypter(block, key)
//	origData := make([]byte, len(crypted))
//
//	blockMode.CryptBlocks(origData, crypted)
//
//	origData = PKCS5UnPadding(origData)
//
//	return origData, nil
//}
//
//func XorEncode(msg []byte, key []byte) []byte {
//	ml := len(msg)
//	kl := len(key)
//
//	var pwd []byte
//
//	for i:=0; i<ml; i++ {
//		b := key[i%kl] ^ msg[i]
//		pwd = append(pwd, b)
//	}
//	return pwd
//}
//
//func XorDecode(msg, key []byte) []byte {
//	ml := len(msg)
//	kl := len(key)
//
//	var pwd []byte
//	for i:=0; i<ml; i++ {
//		b := msg[i] ^ key[i%kl]
//		pwd = append(pwd, b)
//	}
//	return pwd
//}