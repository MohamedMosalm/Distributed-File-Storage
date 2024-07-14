package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTCPTransport(t *testing.T) {

	opts := TCPTransportOpts{
		ListenAddress: ":8080",
		Decoder:       &GOBDecoder{},
		HandShakeFunc: NOPHandShakeFunc,
	}
	tr := NewTCPTransport(opts)

	assert.Equal(t, tr.ListenAddress, ":8080")

	assert.Nil(t, tr.ListenAndAccept())

}
