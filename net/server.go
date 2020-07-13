package net

import (
	"encoding/binary"
	"github.com/titus12/ma-commons-go/cryptos"
	"io"
	"net"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/utils"
	"github.com/xtaci/kcp-go"
)

type Config struct {
	Listen                        string
	ReadDeadline                  time.Duration
	Sockbuf                       int
	Udp_sockbuf                   int
	Txqueuelen                    int
	Dscp                          int
	Sndwnd                        int
	Rcvwnd                        int
	MTU                           int
	Nodelay, Interval, Resend, NC int
}

type connWrapper struct {
	conn        net.Conn
	closeStatus int32
}

func makeConn(conn net.Conn) (*connWrapper, func()) {
	cw := &connWrapper{conn: conn, closeStatus: 0}
	closeFn := func() {
		if atomic.CompareAndSwapInt32(&cw.closeStatus, 0, 1) {
			cw.conn.Close()
		}
	}
	return cw, closeFn
}

type ConnectionHandler func(session *NetSession)

//type ConnectionHandle interface {
//	ConnectionHandle(session *SessionBase, in chan []byte, out *Buffer)
//}

type serverHandler struct {
	config      *Config
	connHandler ConnectionHandler
}

func NewServerHandler(cfg *Config, connectionHandler ConnectionHandler) *serverHandler {
	return &serverHandler{config: cfg, connHandler: connectionHandler}
}

type NetSession struct {
	*Session
	conn        net.Conn
	closeStatus int32
	in          chan []byte
	out         *Buffer
}

func makeSession(conn net.Conn, conf *Config) (*NetSession, func()) {
	ns := &NetSession{conn: conn, closeStatus: 0, Session: &Session{}}
	ns.Die = make(chan struct{})
	bi := make(chan []byte)
	ns.in = bi
	buf := NewBuffer(conn, ns.Die, conf.Txqueuelen)
	ns.out = buf
	closeFn := func() {
		log.WithFields(ns.LogFields()).Warnf("user_id %v, close conn", ns.UserId)
		close(ns.in)
		ns.closeConn()
	}
	return ns, closeFn
}

func (ns *NetSession) closeConn() {
	if atomic.CompareAndSwapInt32(&ns.closeStatus, 0, 1) {
		ns.conn.Close()
	}
}

func (ns *NetSession) Receive() chan []byte {
	return ns.in
}

// packet sending procedure
func (ns *NetSession) Send(data []byte) {
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
	if ns.Flag&SESS_ENCRYPT != 0 { // encryption is enabled
		//sess.Encoder.XORKeyStream(data, data)
		var err error
		data, err = cryptos.Encrypt(data, ns.CryptoKey)
		if err != nil {
			log.WithFields(ns.LogFields()).Errorf("des.Encrypt errors %v", err)
			return
		}
	} else if ns.Flag&SESS_KEYEXCG != 0 { // key is exchanged, encryption is not yet enabled
		ns.Flag &^= SESS_KEYEXCG
		ns.Flag |= SESS_ENCRYPT
		// todo: 这里进行密钥交换，交换给谁？
	}

	// queue the data for sending
	select {
	case ns.out.pending <- data:
	default: // packet will be dropped if txqueuelen exceeds
		log.WithFields(ns.LogFields()).Warning("pending full")
	}
	return
}

// 每个连接会调用这个方法，启动一个goroutine
func (serverHandler *serverHandler) ConnectionActive(connection net.Conn) {
	defer utils.PrintPanicStack()
	sess, close := makeSession(connection, serverHandler.config)
	defer close()
	// for reading the 2-Byte header
	header := make([]byte, 2)
	// the input channel for agent()
	defer func() {
		header = nil
	}()

	// create a new session object for the connection
	// and record it's IP address
	host, _, err := net.SplitHostPort(sess.conn.RemoteAddr().String())
	if err != nil {
		log.Error("cannot get remote address err: %v", err)
		return
	}
	sess.IP = net.ParseIP(host)
	sess.Addr = sess.conn.RemoteAddr().String() //sess.IP.String()

	log.WithFields(sess.LogFields()).Info("new connection")

	// create a write buffer
	go func() {
		defer sess.closeConn()
		sess.out.start()
	}()
	// start agent for PACKET processing
	SigAdd()
	go func() {
		defer utils.PrintPanicStack()
		defer SigDone() // will decrease waitGroup by one, useful for manual server shutdown
		serverHandler.connHandler(sess)
	}()

	// read loop
	for {
		// solve dead link problem:
		// physical disconnection without any communication between client and server
		// will cause the read to block FOREVER, so a timeout is a rescue.
		sess.conn.SetReadDeadline(time.Now().Add(serverHandler.config.ReadDeadline))
		// read 2B header
		n, err := io.ReadFull(sess.conn, header)
		if err != nil {
			log.WithFields(sess.LogFields()).Warningf("read header failed, err:%v size:%v", err, n)
			return
		}

		size := binary.BigEndian.Uint16(header)
		if size < 0 {
			log.WithFields(sess.LogFields()).Warningf("read header size failed, err:%v size:%v", err, n)
			return
		}
		// alloc a byte slice of the size defined in the header for reading data
		// TODO::优化
		payload := make([]byte, size)
		n, err = io.ReadFull(sess.conn, payload)
		if err != nil {
			log.WithFields(sess.LogFields()).Warningf("read payload failed, err:%v size:%v", err, n)
			return
		}

		// deliver the data to the input queue of agent()
		select {
		case sess.in <- payload: // payload queued
		case <-sess.Die:
			log.WithFields(sess.LogFields()).Warningf("connection closed by logic, flag:%v", sess.Flag)
			return
		}
	}
}

func (serverHandler *serverHandler) StartTcpServer() error {
	config := serverHandler.config
	// resolve address & start listening
	tcpAddr, err := net.ResolveTCPAddr("tcp4", config.Listen)
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	log.Info("listening on:", listener.Addr())

	// loop accepting
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Warning("accept failed:", err)
			continue
		}
		// set socket read buffer
		conn.SetReadBuffer(config.Sockbuf)
		// set socket write buffer
		conn.SetWriteBuffer(config.Sockbuf)
		// start a goroutine for every incoming connection for reading
		go serverHandler.ConnectionActive(conn)
	}
}

func (serverHandler *serverHandler) StartUdpServer() error {
	config := serverHandler.config
	l, err := kcp.Listen(config.Listen)
	if err != nil {
		return err
	}
	log.Info("udp listening on:", l.Addr())
	lis := l.(*kcp.Listener)

	if err := lis.SetReadBuffer(config.Sockbuf); err != nil {
		log.Println("SetReadBuffer", err)
	}
	if err := lis.SetWriteBuffer(config.Sockbuf); err != nil {
		log.Println("SetWriteBuffer", err)
	}
	if err := lis.SetDSCP(config.Dscp); err != nil {
		log.Println("SetDSCP", err)
	}

	// loop accepting
	for {
		conn, err := lis.AcceptKCP()
		if err != nil {
			log.Warning("accept failed err: %v", err)
			continue
		}
		// set kcp parameters
		conn.SetWindowSize(config.Sndwnd, config.Rcvwnd)
		conn.SetNoDelay(config.Nodelay, config.Interval, config.Resend, config.NC)
		conn.SetStreamMode(true)
		conn.SetMtu(config.MTU)

		// start a goroutine for every incoming connection for reading
		go serverHandler.ConnectionActive(conn)
	}
}
