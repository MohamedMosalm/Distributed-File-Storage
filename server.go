package main

import (
	// "bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/MohamedMosalm/Distributed-File-Storage/p2p"
)

func init() {
	gob.Register(&Message{})
	gob.Register(&MessageStoreFile{})
	gob.Register(&MessageGetFile{})
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

			if err := s.handleMessage(rpc.From, &msg); err != nil {
				fmt.Printf("handle Message error: %v\n", err)
				continue
			}

		case <-s.quitch:
			return
		}
	}
}

func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case *MessageStoreFile:
		return s.handleMessageStoreFile(from, *v)
	case *MessageGetFile:
		return s.handleMessageGetFile(from, *v)
	default:
		fmt.Printf("Unknown message type: %T\n", v)
	}
	return nil
}

func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %v not found", from)
	}

	n, err := s.store.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	fmt.Printf("Wrote %d bytes to disk\n", n)

	peer.CloseStream()

	return nil
}

func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.Has(msg.Key) {
		return fmt.Errorf("[%s]: file with key %s not found on disk", s.Transport.ListenAddr(), msg.Key)
	}

	fmt.Printf("[%s]: serving file with key %s over the network\n", s.Transport.ListenAddr(), msg.Key)
	fileSize, r, err := s.store.Read(msg.Key)
	if err != nil {
		return nil
	}

	if rc, ok := r.(io.ReadCloser); ok {
		defer rc.Close()
	}

	peer, ok := s.peers[from]

	if !ok {
		return fmt.Errorf("peer %s not found on disk", from)
	}

	peer.Send([]byte{p2p.IncomingStream})

	binary.Write(peer, binary.LittleEndian, fileSize)

	n, err := io.Copy(peer, r)
	if err != nil {
		return nil
	}
	fmt.Printf("[%s]: written %d bytes over the network to %s\n", s.Transport.ListenAddr(), n, from)

	return nil
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
	Key  string
	Size int64
}

type MessageGetFile struct {
	Key string
}

func (s *FileServer) stream(msg *Message) error {
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

func (s *FileServer) broadcast(msg *Message) error {
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileServer) Get(key string) (io.Reader, error) {
	if s.store.Has(key) {
		fmt.Printf("[%s]: serving file with key %s from local storage\n", s.Transport.ListenAddr(), key)
		_, r, err := s.store.Read(key)
		return r, err
	}
	fmt.Printf("[%s]: file with key %s not found in local storage, fetching from network...\n", s.Transport.ListenAddr(), key)

	msg := Message{
		Payload: MessageGetFile{
			Key: key,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 500)

	for _, peer := range s.peers {
		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize)

		n, err := s.store.Write(key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}

		fmt.Printf("[%s] received %d bytes over the network from (%s) \n", s.Transport.ListenAddr(), n, peer.RemoteAddr())

		peer.CloseStream()
	}

	_, r, err := s.store.Read(key)
	return r, err
}

func (s *FileServer) Store(key string, r io.Reader) error {
	fileBuf := new(bytes.Buffer)
	tee := io.TeeReader(r, fileBuf)

	size, err := s.store.Write(key, tee)
	if err != nil {
		return err
	}
	msg := Message{
		Payload: MessageStoreFile{
			Key:  key,
			Size: size,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 5)

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingStream})
		n, err := io.Copy(peer, fileBuf)
		if err != nil {
			return err
		}
		fmt.Printf("received and stored %d bytes\n", n)
	}

	return nil
}
