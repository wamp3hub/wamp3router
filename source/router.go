package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"
	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

type Server interface {
	Serve() error
	Shutdown() error
}

type Router struct {
	ID        string
	metaPeer  *wamp.Peer
	Session   *wamp.Session
	KeyRing   *routerShared.KeyRing
	Storage   routerShared.Storage
	Broker    *Broker
	Dealer    *Dealer
	Newcomers *wampShared.Observable[*wamp.Peer]
	logger    *slog.Logger
}

func NewRouter(
	ID string,
	storage routerShared.Storage,
	keyRing *routerShared.KeyRing,
	logger *slog.Logger,
) *Router {
	lTransport, rTransport := wampTransports.NewDuplexLocalTransport(128)
	lPeer := wamp.SpawnPeer(ID, lTransport, logger)
	rPeer := wamp.SpawnPeer(ID, rTransport, logger)
	session := wamp.NewSession(rPeer, logger)
	router := Router{
		ID,
		lPeer,
		session,
		keyRing,
		storage,
		NewBroker(ID, storage, logger),
		NewDealer(ID, storage, logger),
		wampShared.NewObservable[*wamp.Peer](),
		logger.With("name", "Router"),
	}

	router.Newcomers.Observe(
		func(peer *wamp.Peer) {
			router.logger.Info("attach peer", "ID", peer.ID)
			peer.RejoinEvents.Observe(
				func(__ struct{}) {},
				func() {
					router.unregister(peer.ID, "")
					router.unsubscribe(peer.ID, "")
					router.logger.Info("dettach peer", "ID", peer.ID)
				},
			)
		},
		func() {
			router.logger.Info("down...")
		},
	)

	router.intialize()
	return &router
}

func (router *Router) Serve() {
	router.logger.Info("up...")
	router.Broker.Serve(router.Newcomers)
	router.Dealer.Serve(router.Newcomers)
	router.Newcomers.Next(router.metaPeer)
}

func (router *Router) Shutdown() {
	router.logger.Info("shutting down...")
	router.Newcomers.Complete()
}
