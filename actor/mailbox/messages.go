package mailbox

//定义邮箱内部的系统消息
// 恢复邮箱处理的消息
type ResumeMailbox struct{}

//定义邮箱内部的系统消息
// 挂起邮箱处理的消息
type SuspendMailbox struct{}
