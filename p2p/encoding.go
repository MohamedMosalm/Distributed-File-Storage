package p2p

import (
	"bufio"
	"encoding/gob"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type GOBDecoder struct{}

func (dec *GOBDecoder) Decode(r io.Reader, rpc *RPC) error {
	return gob.NewDecoder(r).Decode(rpc)
}

type DefaultDecoder struct{}

func (dec *DefaultDecoder) Decode(r io.Reader, rpc *RPC) error {
	reader := bufio.NewReader(r)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return err
	}
	rpc.Payload = line
	return nil
}
