package server

import (
	"log"
	"net"

	"github.com/rs/xid"
	wamp "github.com/wamp3hub/wamp3go"
	wampSerializer "github.com/wamp3hub/wamp3go/serializer"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransport "github.com/wamp3hub/wamp3go/transport"
)

func UnixMount(
	address string,
	produceNewcomers wampShared.Producible[*wamp.Peer],
) {
	l, e := net.Listen("unix", address)
	if e == nil {
		log.Printf("[unix-server] listening %s", address)
	} else {
		log.Fatalf("[unix-server] listen error %s", e)
	}

	for {
		fd, e := l.Accept()
		if e == nil {
			transport := wampTransport.UnixTransport(wampSerializer.DefaultSerializer, fd)
			peer := wamp.SpawnPeer(xid.New().String(), transport)
			produceNewcomers(peer)
		} else {
			log.Fatalf("[unix-server] accept error %s", e)
		}
	}

	l.Close()
}
