package actor

// 定义处理接口
type Process interface {
	SendUserMessage(pid *PID, message interface{})
	SendSystemMessage(pid *PID, message interface{})
	Stop(pid *PID)
}
