package p2p

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type TCPPeer struct {
	net.Conn
	outbound bool
	wg       *sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
		wg:       &sync.WaitGroup{},
	}
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Write(b)
	return err
}

func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

type TCPTransportOpts struct {
	ListenAddress string
	HandShakeFunc HandShakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcChan  chan RPC
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcChan:          make(chan RPC, 1024),
	}
}

func (t *TCPTransport) ListenAddr() string {
	return t.ListenAddress
}

func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcChan
}

func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleconn(conn, true)

	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddress)
	if err != nil {
		return err
	}

	go t.startAcceptLoop()

	fmt.Printf("TCP transport listening on port: %s\n", t.ListenAddress[1:])

	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept()

		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			fmt.Printf("error accepting connection: %v\n", err)
			continue
		}

		go t.handleconn(conn, false)
	}
}

func (t *TCPTransport) handleconn(conn net.Conn, outbound bool) {
	var err error
	defer func() {
		fmt.Printf("dropping peer connection: %s", err)
		conn.Close()
	}()
	peer := NewTCPPeer(conn, true)

	if err := t.HandShakeFunc(peer); err != nil {
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	for {
		rpc := RPC{}
		if err := t.Decoder.Decode(conn, &rpc); err != nil {
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}
		// fmt.Printf("Received message from %v: %v\n", rpc.From, rpc.Payload)

		rpc.From = conn.RemoteAddr().String()

		if rpc.Stream {
			peer.wg.Add(1)
			fmt.Printf("incoming stream from %v:\n", rpc.From)
			peer.wg.Wait()
			fmt.Printf("stream from: %v closed, resuming read loop\n", rpc.From)
			continue
		}

		t.rpcChan <- rpc
	}
}
