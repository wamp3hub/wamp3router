package routerServers

import (
	"encoding/json"
	"log/slog"
	"net"
	"os"

	wamp "github.com/wamp3hub/wamp3go"
	wampSerializers "github.com/wamp3hub/wamp3go/serializers"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"
	router "github.com/wamp3hub/wamp3router"
)

type UnixServer struct {
	Path   string
	router *router.Router
	logger *slog.Logger
	super  net.Listener
}

func NewUnixServer(
	path string,
	router *router.Router,
	logger *slog.Logger,
) *UnixServer {
	return &UnixServer{
		path,
		router,
		logger.With("name", "UnixServer"),
		nil,
	}
}

func (server *UnixServer) onConnect(
	connection net.Conn,
) error {
	server.logger.Info("new unix connection", "clientAddress", connection.RemoteAddr())
	transport := wampTransports.UnixTransport(wampSerializers.DefaultSerializer, connection)
	routerID := server.router.Session.ID()
	serverMessage := wampTransports.UnixServerMessage{
		RouterID: routerID,
		YourID:   routerID + "-" + wampShared.NewID(),
	}
	rawServerMessage, _ := json.Marshal(serverMessage)
	e := transport.WriteRaw(rawServerMessage)
	if e == nil {
		rawClientMessage, e := transport.ReadRaw()
		if e == nil {
			clientMessage := new(wampTransports.UnixClientMessage)
			e = json.Unmarshal(rawClientMessage, clientMessage)
			if e == nil {
				peer := wamp.SpawnPeer(serverMessage.YourID, transport, server.logger)
				server.logger.Info("new peer", "ID", peer.ID)
				server.router.Newcomers.Next(peer)
			}
		}
	}
	return e
}

func (server *UnixServer) Serve() (e error) {
	logData := slog.Group(
		"UnixServer",
		"Path", server.Path,
	)

	server.super, e = net.Listen("unix", server.Path)
	if e == nil {
		server.logger.Info("listening...", logData)
	} else {
		server.logger.Error("during listen", "error", e, logData)
		return e
	}

	for {
		fd, e := server.super.Accept()
		if e == nil {
			go server.onConnect(fd)
			continue
		}

		server.logger.Debug("during listening new connections", "error", e, logData)
		return e
	}
}

func (server *UnixServer) Shutdown() error {
	server.logger.Info("shutting down...")
	e := server.super.Close()
	os.Remove(server.Path)
	return e
}
