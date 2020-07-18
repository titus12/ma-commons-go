package net

import (
	log "github.com/sirupsen/logrus"
	"net"
)

const (
	SESS_KEYEXCG    = 0x1 // 是否已经交换完毕KEY
	SESS_ENCRYPT    = 0x2 // 是否可以开始加密
	SESS_KICKED_OUT = 0x4 // 踢掉
	SESS_AUTHORIZED = 0x8 // 已授权访问
)

type Session struct {
	IP   net.IP
	Addr string // game需要的数据
	//Encoder *rc4.Cipher                 // 加密器
	//Decoder *rc4.Cipher                 // 解密器
	CryptoKey []byte        // des 密钥
	UserId    uint64        // 玩家ID
	Die       chan struct{} // 会话关闭信号

	// 会话标记
	Flag int32
}

// append fields to log
func (p *Session) LogFields() log.Fields {
	return log.Fields{
		"user_id": p.UserId,
		"user_ip": p.Addr,
	}
}
