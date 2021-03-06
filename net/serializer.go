package net

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io"
	"reflect"
)

const (
	MAX_BUFSIZE     = 32767     // 单个消息最大长度
	COMPRESS_SIZE   = 16 * 1024 // 需要压缩大小
	MAX_PACKET_SIZE = 65535
)

const (
	COMPRESS_ZLIB = iota
)

func compress(in []byte) (out []byte) {
	if len(in) == 0 {
		return
	}

	var buff bytes.Buffer
	writer := zlib.NewWriter(&buff)
	writer.Write(in)
	writer.Close()

	return buff.Bytes()
}

func decompress(in []byte) (out []byte, err error) {
	reader := bytes.NewReader(in)
	var r io.ReadCloser
	var buff bytes.Buffer
	r, err = zlib.NewReader(reader)
	if err != nil {
		return
	}
	defer r.Close()
	io.Copy(&buff, r)
	out = buff.Bytes()
	return
}

func NewMessageByName(name string) interface{} {
	mType := proto.MessageType(name)
	return reflect.New(mType.Elem()).Interface()
}

func Deserialize(protocolCode int16, bits uint16, data []byte) (interface{}, error) {
	name := GetProtocolName(protocolCode)
	if name == "" {
		return nil, fmt.Errorf("message %v not bind", protocolCode)
	}

	if (bits & (1 >> COMPRESS_ZLIB)) != 0 {
		var err error
		data, err = decompress(data)
		if err != nil {
			return nil, fmt.Errorf("bits %v decompress err %v", bits, err)
		}
	}

	mType := proto.MessageType(name)
	message := reflect.New(mType.Elem()).Interface()
	//proto.UnmarshalMerge(data, message.(proto.Message))
	//proto.Unmarshal(data, message.(proto.Message))
	return message, proto.UnmarshalMerge(data, message.(proto.Message))
}

func Serialize(errorCode int32, message interface{}) ([]byte, error) {
	name := proto.MessageName(message.(proto.Message))
	protocolCode := GetProtocolId(name)
	if protocolCode == 0 {
		return nil, fmt.Errorf("message %v not bind", name)
	}
	// data
	data, err := proto.Marshal(message.(proto.Message))
	if err != nil {
		return nil, fmt.Errorf("message %v marshal err %v", protocolCode, err)
	}

	var bits int16
	size := len(data)
	if COMPRESS_SIZE < size {
		data = compress(data)
		size = len(data)
		bits |= int16(1 >> COMPRESS_ZLIB)
	}

	if size > MAX_BUFSIZE {
		return nil, fmt.Errorf("message %v size %v limited", protocolCode, size)
	}

	writer := Writer()
	writer.WriteU16(0)
	writer.WriteS16(protocolCode)
	writer.WriteS16(bits)
	writer.WriteS16(int16(errorCode))
	writer.WriteBinary(data)
	return writer.Data(), nil
}
