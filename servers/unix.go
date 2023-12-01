package routerServers

import (
	"log"
	"net"
	"os"

	wamp "github.com/wamp3hub/wamp3go"
	wampSerializers "github.com/wamp3hub/wamp3go/serializers"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type UnixServer struct {
	Path      string
	Newcomers *wampShared.ObservableObject[*wamp.Peer]
	Session   *wamp.Session
	KeyRing   *routerShared.KeyRing
	super     net.Listener
}

func (server *UnixServer) onConnect(
	connection net.Conn,
) error {
	log.Printf("[unix-server] new connection")
	transport := wampTransports.UnixTransport(wampSerializers.DefaultSerializer, connection)
	routerID := server.Session.ID()
	serverMessage := wampTransports.UnixServerMessage{
		RouterID: routerID,
		YourID:   routerID + "-" + wampShared.NewID(),
	}
	e := transport.WriteJSON(serverMessage)
	if e == nil {
		clientMessage := new(wampTransports.UnixClientMessage)
		e = transport.ReadJSON(clientMessage)
		if e == nil {
			peer := wamp.SpawnPeer(serverMessage.YourID, transport)
			server.Newcomers.Next(peer)
		}
	}
	return e
}

func (server *UnixServer) Serve() (e error) {
	server.super, e = net.Listen("unix", server.Path)
	if e == nil {
		log.Printf("[unix-server] listening %s", server.Path)
	} else {
		log.Printf("[unix-server] %s", e)
		return e
	}

	for {
		fd, e := server.super.Accept()
		if e == nil {
			go server.onConnect(fd)
			continue
		}

		log.Printf("[unix-server] %s", e)
		return e
	}
}

func (server *UnixServer) Shutdown() error {
	log.Printf("[unix-server] shutting down...")
	e := server.super.Close()
	os.Remove(server.Path)
	return e
}
