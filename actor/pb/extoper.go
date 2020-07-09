package pb

import (
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

func NewWrapMsg(message proto.Message) (wrap *WrapMsg, err error) {
	msgdata, err := proto.Marshal(message)
	if err != nil {
		return
	}
	msgtype := proto.MessageName(message)
	wrap = &WrapMsg{
		MsgType: msgtype,
		MsgData: msgdata,
	}
	return
}

func (wrap *WrapMsg) UnPack() (proto.Message, error) {
	t := proto.MessageType(wrap.MsgType)
	if t == nil {
		return nil, errors.Errorf("wrap.MsgType is Null[%s]", wrap.MsgType)
	}

	msg := reflect.New(t.Elem()).Interface()
	err := proto.Unmarshal(wrap.MsgData, msg.(proto.Message))

	if err != nil {
		return nil, err
	}
	return msg.(proto.Message), nil
}
