package wamp3router

import (
	"log"
	"net/http"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransport "github.com/wamp3hub/wamp3go/transport"

	routerServer "github.com/wamp3hub/wamp3router/server"
	routerShared "github.com/wamp3hub/wamp3router/shared"
)

// Creates new instance of `wamp.Session`
// Where session represents router
func Initialize(
	peerID string,
	consumeNewcomers wampShared.Consumable[*wamp.Peer],
	produceNewcomer wampShared.Producible[*wamp.Peer],
	storage Storage,
) *wamp.Session {
	log.Printf("[router] up...")

	alphaTransport, betaTransport := wampTransport.NewDuplexLocalTransport()
	alphaPeer := wamp.SpawnPeer(peerID, alphaTransport)
	betaPeer := wamp.SpawnPeer(peerID, betaTransport)
	session := wamp.NewSession(alphaPeer)

	broker := NewBroker(storage)
	dealer := NewDealer(storage)

	broker.Setup(session, dealer)
	dealer.Setup(session, broker)

	consumeNewcomers(
		func(peer *wamp.Peer) {
			log.Printf("[router] attach peer (ID=%s)", peer.ID)
			<-peer.Alive
			log.Printf("[router] dettach peer (ID=%s)", peer.ID)
		},
		func() { log.Printf("[router] down...") },
	)
	broker.Serve(consumeNewcomers)
	dealer.Serve(consumeNewcomers)

	produceNewcomer(betaPeer)

	return session
}

func HTTPServe(
	session *wamp.Session,
	keyRing *routerShared.KeyRing,
	produceNewcomer wampShared.Producible[*wamp.Peer],
	address string,
	enableWebsocket bool,
) error {
	http.Handle("/wamp3/interview", routerServer.InterviewMount(session, keyRing))
	if enableWebsocket {
		http.Handle("/wamp3/websocket", routerServer.WebsocketMount(keyRing, produceNewcomer))
	}

	log.Printf("[router] Starting at %s", address)
	e := http.ListenAndServe(address, nil)
	return e
}
