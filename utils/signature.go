package utils

import (
	"crypto"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

//-----BEGIN RSA PUBLIC KEY-----
//const RSA_PUBLIC_KEY  =
const (
	rsaPublicKeyStart = "-----BEGIN RSA PUBLIC KEY-----"
	resPublicKeyEnd   = "-----END RSA PUBLIC KEY-----"
)

const (
	pemStart = "-----BEGIN "
	pemEnd   = "KEY-----"
)

// HMAC-SHA1 签名
func SignWithHmacSha1(content, secureKey string) string {
	h := hmac.New(sha1.New, StringToBytes(secureKey))
	h.Write(StringToBytes(content))
	return hex.EncodeToString(h.Sum(nil))
}

func SignWithHmacSha1WithHexString(content, secureKey string) string {
	h := hmac.New(sha1.New, StringToBytes(secureKey))
	h.Write(StringToBytes(content))
	return hex.EncodeToString(h.Sum(nil))
}

func SignWithHmacSha1WithByteArray(content, secureKey string) []byte {
	h := hmac.New(sha1.New, StringToBytes(secureKey))
	h.Write(StringToBytes(content))
	return h.Sum(nil)
}

func SignWithHmacSha1WithBase64String(content, secureKey string) string {
	h := hmac.New(sha1.New, StringToBytes(secureKey))
	h.Write(StringToBytes(content))
	b := h.Sum(nil)
	return base64.StdEncoding.EncodeToString(b)
}

// md5码签名
func SignWithMd5(content, secureKey string) string {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s%s", content, secureKey)))
	return hex.EncodeToString(h.Sum(nil))
}

func Md5(content string) string {
	b := Md5WithByteArray(StringToBytes(content))
	return hex.EncodeToString(b)
}

func Md5WithByteArray(content []byte) []byte {
	h := md5.New()
	h.Write(content)
	return h.Sum(nil)
}

// 通过 SHA1WithRSA 对内容进行验签
// content: 验签内容
// base64Sign: 一个base64的签名字符串
// base64PublicKey: 一个base64的公钥
func VerifyBySha1WithRsa(content, base64Sign, base64PublicKey string) bool {
	if !strings.HasPrefix(base64PublicKey, pemStart) { //没有前缀加入前缀
		base64PublicKey = fmt.Sprintf("%s\n%s", rsaPublicKeyStart, base64PublicKey)
	}
	if !strings.HasSuffix(base64PublicKey, pemEnd) { //没有后缀加入后缀
		base64PublicKey = fmt.Sprintf("%s\n%s", base64PublicKey, resPublicKeyEnd)
	}
	block, _ := pem.Decode(StringToBytes(base64PublicKey))
	if block == nil {
		logrus.Errorf("public key error  %s", base64PublicKey)
		return false
	}

	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logrus.WithError(err).Errorf("public key error ix_publickey  %s", base64PublicKey)
		return false
	}

	pub, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		logrus.Errorf("public key error convert *rsa.PublicKey fail  %s", base64PublicKey)
		return false
	}

	signBytes, err := base64.StdEncoding.DecodeString(base64Sign)
	if err != nil {
		logrus.WithError(err).Errorf("base64Sign不是有一个正常的base64编码  %s", base64Sign)
		return false
	}

	myhash := crypto.SHA1
	hashInstance := myhash.New()
	hashInstance.Write(StringToBytes(content))
	hashed := hashInstance.Sum(nil)

	err = rsa.VerifyPKCS1v15(pub, myhash, hashed, signBytes)
	if err != nil {
		logrus.WithError(err).Errorf("Sha256WithRsa verify fail... content: %s, sign: %s, publickey: %s",
			content, base64Sign, base64PublicKey)
		return false
	}
	return true
}

// 通过 SHA256WithRSA 对内容进行验签
// content: 验签内容
// base64Sign: 一个base64的签名字符串
// base64PublicKey: 一个base64的公钥
func VerifyBySha256WithRsa(content, base64Sign, base64PublicKey string) bool {
	if !strings.HasPrefix(base64PublicKey, pemStart) { //没有前缀加入前缀
		base64PublicKey = fmt.Sprintf("%s\n%s", rsaPublicKeyStart, base64PublicKey)
	}
	if !strings.HasSuffix(base64PublicKey, pemEnd) { //没有后缀加入后缀
		base64PublicKey = fmt.Sprintf("%s\n%s", base64PublicKey, resPublicKeyEnd)
	}

	block, _ := pem.Decode(StringToBytes(base64PublicKey))
	if block == nil {
		logrus.Errorf("public key error  %s", base64PublicKey)
		return false
	}

	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logrus.WithError(err).Errorf("public key error ix_publickey  %s", base64PublicKey)
		return false
	}

	pub, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		logrus.Errorf("public key error convert *rsa.PublicKey fail  %s", base64PublicKey)
		return false
	}

	signBytes, err := base64.StdEncoding.DecodeString(base64Sign)
	if err != nil {
		logrus.WithError(err).Errorf("base64Sign不是有一个正常的base64编码  %s", base64Sign)
		return false
	}

	myhash := crypto.SHA256
	hashInstance := myhash.New()
	hashInstance.Write(StringToBytes(content))
	hashed := hashInstance.Sum(nil)

	err = rsa.VerifyPKCS1v15(pub, myhash, hashed, signBytes)
	if err != nil {
		logrus.WithError(err).Errorf("Sha256WithRsa verify fail... content: %s, sign: %s, publickey: %s",
			content, base64Sign, base64PublicKey)
		return false
	}
	return true
}
