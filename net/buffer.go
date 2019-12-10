package net

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/crypto"
	"github.com/titus12/ma-commons-go/utils"
	"net"
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

	// encryption
	// (NOT_ENCRYPTED) -> KEYEXCG -> ENCRYPT
	if sess.Flag&SESS_ENCRYPT != 0 { // encryption is enabled
		//sess.Encoder.XORKeyStream(data, data)
		var err error
		data, err = crypto.Encrypt(data, sess.CryptoKey)
		if err != nil {
			log.WithFields(log.Fields{"user_id": sess.UserId, "ip": sess.IP}).Errorf("des.Encrypt error %v", err)
			return
		}
	} else if sess.Flag&SESS_KEYEXCG != 0 { // key is exchanged, encryption is not yet enabled
		sess.Flag &^= SESS_KEYEXCG
		sess.Flag |= SESS_ENCRYPT
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
