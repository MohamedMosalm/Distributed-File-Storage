package p2p

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type TCPPeer struct {
	conn     net.Conn
	outbound bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		outbound: outbound,
	}
}

type TCPTransportOpts struct {
	ListenAddress string
	HandShakeFunc HandShakeFunc
	Decoder       Decoder
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener

	mu    sync.RWMutex
	peers map[net.Addr]Peer
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
	}
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddress)
	if err != nil {
		return err
	}

	go t.startAcceptLoop()

	return nil
}

func (t *TCPTransport) startAcceptLoop() error {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			fmt.Printf("error accepting connection: %v\n", err)
			continue
		}

		fmt.Printf("new connection %v\n", conn)
		go t.handleconn(conn)
	}
}

func (t *TCPTransport) handleconn(conn net.Conn) {
	peer := NewTCPPeer(conn, true)

	if err := t.HandShakeFunc(peer); err != nil {
		conn.Close()
		fmt.Printf("TCP handshake failed: %v\n", err)
		return
	}

	for {
		rpc := &RPC{}
		if err := t.Decoder.Decode(conn, rpc); err != nil {
			if err == io.EOF {
				fmt.Printf("Connection closed by remote: %v\n", conn.RemoteAddr())
				break
			}
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}
		rpc.From = conn.RemoteAddr()
		fmt.Printf("Received message from %v: %v\n", rpc.From, string(rpc.Payload))
	}
	conn.Close()
}
