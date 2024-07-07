package p2p

type HandShakeFunc func(Peer) error

func NOPHandShakeFunc(Peer) error { return nil }
