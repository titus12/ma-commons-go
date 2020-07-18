package net

import (
	log "github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/utils"
	"net"
)

type Buffer struct {
	ctrl    chan struct{} // receive exit signal
	pending chan []byte   // pending packets
	conn    net.Conn      // connection
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
	data[0] = byte(sz >> 8)
	data[1] = byte(sz)
	// write data
	n, err := buf.conn.Write(data)
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
	return &buf
}
