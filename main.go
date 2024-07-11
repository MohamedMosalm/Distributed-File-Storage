package main

import (
	"fmt"
	"log"

	"github.com/MohamedMosalm/Distributed-File-Storage/p2p"
)

func OnPeer(peer p2p.Peer) error {
	peer.Close()
	return nil
}

func main() {

	opts := p2p.TCPTransportOpts{
		ListenAddress: ":8080",
		Decoder:       &p2p.DefaultDecoder{},
		HandShakeFunc: p2p.NOPHandShakeFunc,
		OnPeer:        OnPeer,
	}
	tr := p2p.NewTCPTransport(opts)

	go func() {
		for {
			msg := <-tr.Consume()
			fmt.Printf("msg %v\n", msg)
		}
	}()

	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}

	select {}
}
