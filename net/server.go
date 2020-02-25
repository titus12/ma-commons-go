package net

import (
	"encoding/binary"
	"io"
	"net"
	"os"
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

type ConnectionHandler func(session *Session, in chan []byte, out *Buffer)

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

// 每个连接会调用这个方法，启动一个goroutine
func (serverHandler *serverHandler) ConnectionActive(conn net.Conn) {
	defer utils.PrintPanicStack()
	defer conn.Close()

	// for reading the 2-Byte header
	header := make([]byte, 2)
	// the input channel for agent()
	in := make(chan []byte)
	defer func() {
		header = nil
		close(in) // session will close
	}()

	// create a new session object for the connection
	// and record it's IP address
	var sess Session
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Error("cannot get remote address:", err)
		return
	}
	sess.IP = net.ParseIP(host)
	sess.Addr = conn.RemoteAddr().String() //sess.IP.String()

	log.WithFields(log.Fields{
		"client_ip": sess.IP.String(),
	}).Info("new connection")

	// session die signal, will be triggered by agent()
	sess.Die = make(chan struct{})

	// create a write buffer
	// todo: 这里实际是启动一个 goroutine 进行写操作，命名成 Buffer ,名称有岐义
	out := NewBuffer(conn, sess.Die, serverHandler.config.Txqueuelen)
	go out.start()

	// start agent for PACKET processing
	SigAdd()
	go func() {
		defer SigDone() // will decrease waitgroup by one, useful for manual server shutdown
		defer utils.PrintPanicStack()
		serverHandler.connHandler(&sess, in, out)
	}()

	// read loop
	for {
		// solve dead link problem:
		// physical disconnection without any communcation between client and server
		// will cause the read to block FOREVER, so a timeout is a rescue.
		conn.SetReadDeadline(time.Now().Add(serverHandler.config.ReadDeadline))

		// read 2B header
		n, err := io.ReadFull(conn, header)
		if err != nil {
			log.Warningf("read header failed, ip:%v reason:%v size:%v", sess.IP, err, n)
			return
		}
		size := binary.BigEndian.Uint16(header)

		// alloc a byte slice of the size defined in the header for reading data
		payload := make([]byte, size)
		n, err = io.ReadFull(conn, payload)
		if err != nil {
			// todo: 错误后没有后续的处理了? 前面的 go程已经开启了？ 直接退出估计会有问题，go程会有泄露
			log.Warningf("read payload failed, ip:%v reason:%v size:%v", sess.IP, err, n)
			return
		}

		// deliver the data to the input queue of agent()
		select {
		case in <- payload: // payload queued
		case <-sess.Die:
			log.Warningf("connection closed by logic, flag:%v ip:%v", sess.Flag, sess.IP)
			return
		}
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
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
			log.Warning("accept failed:", err)
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
