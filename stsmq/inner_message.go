package stsmq

import (
	"errors"
	"github.com/golang/protobuf/proto"
)

var (
	ErrNotSupportBodyData = errors.New("NotSupport BodyData") //不支持的包体数据
	ErrNullRecvName       = errors.New("Null RecvName")       //接收队列名称为空
)

// 包体数据，包体数据最终都会是[]byte，但在这之前有可能是正常的[]byte,也有可能是proto.Message
type BodyData interface {
	Bytes() ([]byte, error)
}

// 内部服务器消息结构
type InnerSerMsg struct {
	revName string   //接收消息队列的名称
	Body    BodyData //消息
}

// proto包体定义
type ProtoBufBody struct {
	proto.Message
}

// 字结数组包体定义
type ByteArrayBody []byte

func (body ByteArrayBody) Bytes() ([]byte, error) {
	return []byte(body), nil
}

func (b *ProtoBufBody) Bytes() ([]byte, error) {
	return proto.Marshal(b.Message)
}

// 构建新的body数据
func newBodyData(body interface{}) (BodyData, error) {
	var bodydata BodyData
	switch bd := body.(type) {
	case []byte:
		bodydata = ByteArrayBody(bd)
	case proto.Message:
		bodydata = &ProtoBufBody{
			Message: bd,
		}
	default:
		return nil, ErrNotSupportBodyData
	}
	return bodydata, nil
}

// 对一个字节数组进行解码
func decodeToInnerSerMsg(data []byte) (*InnerSerMsg, error) {
	innerMsg := &InnerSerMsg{}

	var err error
	innerMsg.Body, err = newBodyData(data)
	if err != nil {
		return nil, err
	}
	return innerMsg, nil
}

// 构建的消息body参数，只能接收[]byte或proto.Message，所以在使用时，如果是这二个类型，可以不用
// 判断error是否正确
func NewInnerSerMsg(revQname string, body interface{}) (*InnerSerMsg, error) {
	if revQname == "" {
		return nil, ErrNullRecvName
	}
	b, err := newBodyData(body)
	if err != nil {
		return nil, err
	}
	return &InnerSerMsg{
		revName: revQname,
		Body:    b,
	}, nil
}
