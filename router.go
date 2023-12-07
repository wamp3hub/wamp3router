package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"
	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type Server interface {
	Serve() error
	Shutdown() error
}

type Router struct {
	ID          string
	peer *wamp.Peer
	Session     *wamp.Session
	Broker      *Broker
	Dealer      *Dealer
	Newcomers   *wampShared.ObservableObject[*wamp.Peer]
	KeyRing     *routerShared.KeyRing
	Storage     routerShared.Storage
	logger      *slog.Logger
}

func NewRouter(
	ID string,
	storage routerShared.Storage,
	logger *slog.Logger,
) *Router {
	lTransport, rTransport := wampTransports.NewDuplexLocalTransport(128)
	lPeer := wamp.SpawnPeer(ID, lTransport, logger)
	rPeer := wamp.SpawnPeer(ID, rTransport, logger)
	session := wamp.NewSession(rPeer, logger)
	return &Router{
		ID,
		lPeer,
		session,
		NewBroker(session, storage, logger),
		NewDealer(session, storage, logger),
		wampShared.NewObservable[*wamp.Peer](),
		routerShared.NewKeyRing(),
		storage,
		logger.With("name", "router"),
	}
}

func (router *Router) Serve() {
	router.logger.Info("up...")

	router.Broker.Setup(router.Dealer)
	router.Dealer.Setup(router.Broker)

	router.Newcomers.Observe(
		func(peer *wamp.Peer) {
			router.logger.Info("attach peer", "ID", peer.ID)
			<-peer.Alive
			router.logger.Info("dettach peer", "ID", peer.ID)
		},
		func() { router.logger.Info("down...") },
	)

	router.Broker.Serve(router.Newcomers)
	router.Dealer.Serve(router.Newcomers)

	router.Newcomers.Next(router.peer)
}

func (router *Router) Shutdown() {
	router.logger.Info("shutting down...")
	router.Newcomers.Complete()
}
