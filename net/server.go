package net

import (
	"encoding/binary"
	log "github.com/sirupsen/logrus"
	"github.com/xtaci/kcp-go"
	"io"
	"ma-commons-go/utils"
	"net"
	"os"
	"time"
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

type ConnectionHandle func(session *SessionBase, in chan []byte, out *Buffer)

//type ConnectionHandle interface {
//	ConnectionHandle(session *SessionBase, in chan []byte, out *Buffer)
//}

type serverHandle struct {
	config     *Config
	connHandle ConnectionHandle
}

func NewServerHandle(cfg *Config, connectionHandle ConnectionHandle) *serverHandle {
	return &serverHandle{config: cfg, connHandle: connectionHandle}
}

func (serverHandle *serverHandle) ConnectionActive(conn net.Conn) {
	defer utils.PrintPanicStack()
	defer conn.Close()

	// for reading the 2-Byte header
	header := make([]byte, 2)
	// the input channel for agent()
	in := make(chan []byte)
	defer func() {
		close(in) // session will close
	}()

	// create a new session object for the connection
	// and record it's IP address
	var sess SessionBase
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
	out := newBuffer(conn, sess.Die, serverHandle.config.Txqueuelen)
	go out.start()

	// start agent for PACKET processing
	wg.Add(1)
	handle := serverHandle.connHandle
	go handle(&sess, in, out)

	// read loop
	for {
		// solve dead link problem:
		// physical disconnection without any communcation between client and server
		// will cause the read to block FOREVER, so a timeout is a rescue.
		conn.SetReadDeadline(time.Now().Add(serverHandle.config.ReadDeadline))

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

func (serverHandle *serverHandle) StartTcpServer() error {
	config := serverHandle.config
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
		go serverHandle.ConnectionActive(conn)
	}
}

func (serverHandle *serverHandle) StartUdpServer() error {
	config := serverHandle.config
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
		go serverHandle.ConnectionActive(conn)
	}
}
