package p2p

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
)

type TCPPeer struct {
	net.Conn
	outbound bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
	}
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Write(b)
	return err
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
		rpcChan:          make(chan RPC),
	}
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
		p := Payload{}
		err = gob.NewDecoder(conn).Decode(&p)

		if err != nil {
			if err == io.EOF {
				fmt.Printf("Connection closed by remote: %v\n", conn.RemoteAddr())
			}
			fmt.Printf("Error decoding message: %v\n", err)
			continue
		}
		fmt.Println("recieved message: ", p)

		buf := new(bytes.Buffer)

		encoder := gob.NewEncoder(buf)
		if err := encoder.Encode(p); err != nil {
			fmt.Printf("error encoding payload: %v\n", err)
		}
		payload := buf.Bytes()

		rpc := RPC{
			From:    conn.RemoteAddr(),
			Payload: payload,
		}

		t.rpcChan <- rpc
	}
}
