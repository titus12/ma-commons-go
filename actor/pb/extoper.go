package pb

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func NewWrapMsg(message proto.Message) (wrap *WrapMsg, err error) {
	msgdata, err := proto.Marshal(message)
	if err != nil {
		return
	}

	msgtype := string(message.ProtoReflect().Descriptor().FullName())

	wrap = &WrapMsg{
		MsgType: msgtype,
		MsgData: msgdata,
	}
	return
}

func (wrap *WrapMsg) UnPack() (proto.Message, error) {
	fullName := protoreflect.FullName(wrap.MsgType)
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
	if err != nil {
		return nil, err
	}
	msg := msgType.New().Interface()

	err = proto.Unmarshal(wrap.MsgData, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

//// 请求解包
//func (reqmsg *RequestMsg) UnPack() (proto.Message, error) {
//	if len(reqmsg.MsgType) <= 0 {
//		return nil, nil
//	}
//
//	fullName := protoreflect.FullName(reqmsg.MsgType)
//	msgType, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
//	if err != nil {
//		return nil, err
//	}
//	msg := msgType.New().Interface()
//
//	err = proto.Unmarshal(reqmsg.MsgData, msg)
//	if err != nil {
//		return nil, err
//	}
//	return msg, nil
//}
//
//func (respmsg *ResponseMsg) UnPack() (proto.Message, error) {
//	if len(respmsg.MsgType) <= 0 {
//		return nil, nil
//	}
//	fullName := protoreflect.FullName(respmsg.MsgType)
//	msgType, err := protoregistry.GlobalTypes.FindMessageByName(fullName)
//	if err != nil {
//		return nil, err
//	}
//	msg := msgType.New().Interface()
//
//	err = proto.Unmarshal(respmsg.MsgData, msg)
//	if err != nil {
//		return nil, err
//	}
//	return msg, nil
//
//}
