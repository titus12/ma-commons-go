package mailbox

import (
	"github.com/titus12/ma-commons-go/internal/queue/goring"
	"github.com/titus12/ma-commons-go/internal/queue/mpsc"
)

// 使用传统环境消息队列作为默认邮箱的队列，此类队列是用锁来保证队列的并发性
func Unbounded(mailboxStats ...Statistics) Producer {
	return func() Mailbox {
		return &defaultMailbox{
			systemMailbox: mpsc.New(),
			userMailbox:   goring.New(10),
			mailboxStats:  mailboxStats,
		}
	}
}

// 使用mpsc队列作为默认邮箱的队列，此类队为多生产者，单消费者的无锁队列，程序使用上需要保证消费者必须只有一个
func UnboundedLockfree(mailboxStats ...Statistics) Producer {
	return func() Mailbox {
		return &defaultMailbox{
			userMailbox:   mpsc.New(),
			systemMailbox: mpsc.New(),
			mailboxStats:  mailboxStats,
		}
	}
}
