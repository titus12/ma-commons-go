package net

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/utils"
	"net"
)

type Buffer struct {
	ctrl    chan struct{} // receive exit signal
	pending chan []byte   // pending packets
	conn    net.Conn      // connection
	// TODO::优化
	cache []byte // for combined syscall write
}

// packet sending goroutine
func (buf *Buffer) start() {
	defer utils.PrintPanicStack()
	defer close(buf.pending)
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
