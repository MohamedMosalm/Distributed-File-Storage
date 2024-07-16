package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"sync"

	"github.com/MohamedMosalm/Distributed-File-Storage/p2p"
)

func init() {
	gob.Register(&Message{})
	gob.Register(&MessageStoreFile{})
}

type FileServerOpts struct {
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	BootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts

	peerLock sync.RWMutex
	peers    map[string]p2p.Peer
	store    *Store
	quitch   chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

func (s *FileServer) loop() {

	defer func() {
		fmt.Println("File Server stopped...")
		s.Transport.Close()
	}()

	for {
		select {
		case rpc := <-s.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				fmt.Printf("error decoding payload: %v\n", err)
				continue
			}

			fmt.Printf("received rpc: %v\n", msg.Payload)

			peer, ok := s.peers[rpc.From]

			if !ok {
				fmt.Printf("Peer not found for address: %v\n", rpc.From)
				continue
			}

			reader := bufio.NewReader(peer)
			var fullData bytes.Buffer
			buf := make([]byte, 1024)
			for {
				n, err := reader.Read(buf)
				if err != nil {
					if err == io.EOF {
						fullData.Write(buf[:n])
						break
					}
					fmt.Printf("Error reading from peer: %v\n", err)
					break
				}
				fullData.Write(buf[:n])
				if n < 1024 {
					break
				}
			}

			fmt.Printf("Received data: %v\n", fullData.String())
			peer.(*p2p.TCPPeer).Wg.Done()

		case <-s.quitch:
			return
		}
	}
}

func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.BootstrapNodes {
		go func(addr string) {
			if err := s.Transport.Dial(addr); err != nil {
				fmt.Println("Dial error: ", err)
			}
		}(addr)
	}
	return nil
}

func (s *FileServer) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}

	if len(s.BootstrapNodes) != 0 {
		s.bootstrapNetwork()
	}

	s.loop()

	return nil
}

func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	s.peers[p.RemoteAddr().String()] = p

	fmt.Printf("connected to %s\n", p.RemoteAddr().String())
	return nil
}

func (s *FileServer) Stop() {
	close(s.quitch)
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	Key string
}

func (s *FileServer) broadcast(msg *Message) error {
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	fmt.Printf("broadcasting message:payload=%v\n", msg.Payload)
	mw := io.MultiWriter(peers...)
	if err := gob.NewEncoder(mw).Encode(msg); err != nil {
		fmt.Printf("error broadcasting payload: %v\n", err)
		return err
	}

	return nil
}

func (s *FileServer) StoreData(key string, r io.Reader) error {
	buf := new(bytes.Buffer)
	msg := Message{
		Payload: MessageStoreFile{
			Key: key,
		},
	}
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	payload := []byte("this is a large file")
	for _, peer := range s.peers {
		if err := peer.Send(payload); err != nil {
			return err
		}
	}

	return nil
}
