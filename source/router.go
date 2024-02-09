package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampInterview "github.com/wamp3hub/wamp3go/interview"
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
	peerDetails := wamp.PeerDetails{
		ID:   ID,
		Role: "root",
		Offer: &wampInterview.Offer{
			RegistrationsLimit: 10,
			SubscriptionsLimit: 0,
			TicketLifeTime:     0,
		},
	}
	lTransport, rTransport := wampTransports.NewDuplexLocalTransport(128)
	lPeer := wamp.SpawnPeer(&peerDetails, lTransport, logger)
	rPeer := wamp.SpawnPeer(&peerDetails, rTransport, logger)
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
			router.logger.Info("attach peer", "ID", peer.Details.ID)
			peer.RejoinEvents.Observe(
				func(__ struct{}) {},
				func() {
					router.unregister(peer.Details.ID, "")
					router.unsubscribe(peer.Details.ID, "")
					router.logger.Info("dettach peer", "ID", peer.Details.ID)
				},
			)
		},
		func() {
			router.logger.Warn("down...")
		},
	)

	router.initialize()
	return &router
}

func (router *Router) Serve() {
	router.logger.Info("up...")
	router.Broker.Serve(router.Newcomers)
	router.Dealer.Serve(router.Newcomers)
	router.Newcomers.Next(router.metaPeer)
}

func (router *Router) Shutdown() {
	router.logger.Warn("shutting down...")
	router.Newcomers.Complete()
}
