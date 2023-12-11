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
	mainPeer  *wamp.Peer
	Session   *wamp.Session
	KeyRing   *routerShared.KeyRing
	Storage   routerShared.Storage
	Broker    *Broker
	Dealer    *Dealer
	Newcomers *wampShared.ObservableObject[*wamp.Peer]
	logger    *slog.Logger
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
		routerShared.NewKeyRing(),
		storage,
		NewBroker(session, storage, logger),
		NewDealer(session, storage, logger),
		wampShared.NewObservable[*wamp.Peer](),
		logger.With("name", "Router"),
	}
}

func (router *Router) Serve() {
	router.logger.Info("up...")

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

	router.Newcomers.Next(router.mainPeer)

	mount(router, "wamp.router.register", &wamp.RegisterOptions{}, router.register)
	mount(router, "wamp.router.unregister", &wamp.RegisterOptions{}, router.unregister)
	mount(router, "wamp.router.registration.list", &wamp.RegisterOptions{}, router.getRegistrationList)
	mount(router, "wamp.router.subscribe", &wamp.RegisterOptions{}, router.subscribe)
	mount(router, "wamp.router.unsubscribe", &wamp.RegisterOptions{}, router.unsubscribe)
	mount(router, "wamp.router.subscription.list", &wamp.RegisterOptions{}, router.getSubscriptionList)
}

func (router *Router) Shutdown() {
	router.logger.Info("shutting down...")
	router.Newcomers.Complete()
}
