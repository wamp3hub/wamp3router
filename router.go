package router

import (
	"log"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type Server interface {
	Serve() error
	Shutdown() error
}

func Serve(
	session *wamp.Session,
	storage routerShared.Storage,
	newcomers *wampShared.ObservableObject[*wamp.Peer],
) {
	log.Printf("[router] up...")

	broker := NewBroker(session, storage)
	dealer := NewDealer(session, storage)

	broker.Setup(dealer)
	dealer.Setup(broker)

	newcomers.Observe(
		func(peer *wamp.Peer) {
			log.Printf("[router] attach peer (ID=%s)", peer.ID)
			<-peer.Alive
			log.Printf("[router] dettach peer (ID=%s)", peer.ID)
		},
		func() { log.Printf("[router] down...") },
	)

	broker.Serve(newcomers)
	dealer.Serve(newcomers)
}
