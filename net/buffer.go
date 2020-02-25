package net

import (
	"encoding/binary"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/cryptos"
	"github.com/titus12/ma-commons-go/utils"
)

type Buffer struct {
	ctrl    chan struct{} // receive exit signal
	pending chan []byte   // pending packets
	conn    net.Conn      // connection
	cache   []byte        // for combined syscall write
}

// packet sending procedure
func (buf *Buffer) Send(sess *Session, data []byte) {
	// in case of empty packet
	if data == nil {
		return
	}

	// todo: 下面有提到先是 NOT_ENCRYPTED -> KEYEXCG -> ENCRYPT
	// todo: 我没有看到哪里有对sess.Flag 进特 sess.Flag |= SESS_KEYEXCG
	// todo: 所以下面 if和else if包含的代码都是没办法执行的
	// todo: 即便是能执行，这个密钥是交换给谁，客户端，那是什么协议交换？
	// encryption
	// (NOT_ENCRYPTED) -> KEYEXCG -> ENCRYPT
	if sess.Flag&SESS_ENCRYPT != 0 { // encryption is enabled
		//sess.Encoder.XORKeyStream(data, data)
		var err error
		data, err = cryptos.Encrypt(data, sess.CryptoKey)
		if err != nil {
			log.WithFields(log.Fields{"user_id": sess.UserId, "ip": sess.IP}).Errorf("des.Encrypt errors %v", err)
			return
		}
	} else if sess.Flag&SESS_KEYEXCG != 0 { // key is exchanged, encryption is not yet enabled
		sess.Flag &^= SESS_KEYEXCG
		sess.Flag |= SESS_ENCRYPT
		// todo: 这里进行密钥交换，交换给谁？
	}

	// queue the data for sending
	select {
	case buf.pending <- data:
	default: // packet will be dropped if txqueuelen exceeds
		log.WithFields(log.Fields{"user_id": sess.UserId, "ip": sess.IP}).Warning("pending full")
	}
	return
}

// packet sending goroutine
func (buf *Buffer) start() {
	defer utils.PrintPanicStack()
	for {
		select {
		case data := <-buf.pending:
			buf.RawSend(data)
		case <-buf.ctrl: // receive session end signal
			//close(buf.pending)
			return
		}
	}
}

// raw packet encapsulation and put it online
func (buf *Buffer) RawSend(data []byte) bool {
	// combine output to reduce syscall.write
	sz := len(data)
	binary.BigEndian.PutUint16(buf.cache, uint16(sz))
	copy(buf.cache[2:], data)

	// todo: 首先我不知道下面 buf.conn.Write的内部是怎么实现的，但通过先把数据copy到buf.cache,然后又从
	// todo: cache 读出来写 buf.conn.Write, 这里怎么说也经过了二次字节copy吧？
	// write data
	n, err := buf.conn.Write(buf.cache[:sz+2])
	if err != nil {
		log.Warningf("Error send reply data, bytes: %v reason: %v", n, err)
		return false
	}

	return true
}

// create a associated write buffer for a session
func NewBuffer(conn net.Conn, ctrl chan struct{}, txqueuelen int) *Buffer {
	buf := Buffer{conn: conn}
	buf.pending = make(chan []byte, txqueuelen)
	buf.ctrl = ctrl
	buf.cache = make([]byte, PACKET_LIMIT+2)
	return &buf
}
