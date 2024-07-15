package main

import (
	"bytes"
	"log"
	"time"

	"github.com/MohamedMosalm/Distributed-File-Storage/p2p"
)

func makeFileServer(listenAddr string, nodes ...string) *FileServer {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddress: listenAddr,
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       &p2p.DefaultDecoder{},
	}

	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)
	fileServerOpts := FileServerOpts{
		StorageRoot:       "root/" + listenAddr[1:] + "_network",
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	s := NewFileServer(fileServerOpts)

	tcpTransport.OnPeer = s.OnPeer
	return s
}

func main() {
	s1 := makeFileServer(":3000")
	s2 := makeFileServer(":4000", ":3000")

	go func() {
		log.Fatal(s1.Start())
	}()

	time.Sleep(time.Second)

	go func() {
		log.Fatal(s2.Start())
	}()

	time.Sleep(time.Second)

	data := bytes.NewReader([]byte("hello world"))
	s2.StoreData("key", data)

	select {}
}
