package main

import (
	"log"

	"github.com/MohamedMosalm/Distributed-File-Storage/p2p"
)

func main() {

	opts := p2p.TCPTransportOpts{
		ListenAddress: ":8080",
		Decoder:       &p2p.DefaultDecoder{},
		HandShakeFunc: p2p.NOPHandShakeFunc,
	}
	tr := p2p.NewTCPTransport(opts)

	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}

	select {}
}
