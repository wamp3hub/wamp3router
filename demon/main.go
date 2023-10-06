package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransport "github.com/wamp3hub/wamp3go/transport"

	"github.com/rs/xid"

	router "github.com/wamp3hub/wamp3router"
	"github.com/wamp3hub/wamp3router/server"
	"github.com/wamp3hub/wamp3router/service"
	"github.com/wamp3hub/wamp3router/storage"
)

func routerServe(
	consumeNewcomers wampShared.Consumable[*wamp.Peer],
	produceNewcomer wampShared.Producible[*wamp.Peer],
	storage router.Storage,
) *wamp.Session {
	log.Printf("[router] up...")

	peerID := xid.New().String()
	alphaTransport, betaTransport := wampTransport.NewDuplexLocalTransport()
	alphaPeer := wamp.NewPeer(peerID, alphaTransport)
	betaPeer := wamp.NewPeer(peerID, betaTransport)
	session := wamp.NewSession(alphaPeer)

	broker := router.NewBroker(storage)
	dealer := router.NewDealer(storage)

	broker.Setup(session, dealer)
	dealer.Setup(session, broker)

	consumeNewcomers(
		func(peer *wamp.Peer) {
			log.Printf("[router] attach peer (ID=%s)", peer.ID)
			peer.Consume()
			log.Printf("[router] dettach peer (ID=%s)", peer.ID)
		},
		func() { log.Printf("[router] down...") },
	)
	broker.Serve(consumeNewcomers)
	dealer.Serve(consumeNewcomers)

	go alphaPeer.Consume()
	produceNewcomer(betaPeer)

	return session
}

func httpServe(
	interviewer *service.Interviewer,
	produceNewcomer wampShared.Producible[*wamp.Peer],
	address string,
	enableWebsocket bool,
) error {
	http.Handle("/wamp3/interview", server.InterviewMount(interviewer))
	if enableWebsocket {
		http.Handle("/wamp3/websocket", server.WebsocketMount(interviewer, produceNewcomer))
	}

	log.Printf("[router] Starting at %s", address)
	e := http.ListenAndServe(address, nil)
	return e
}

func main() {
	address := flag.String("address", ":8888", "")
	flag.Parse()

	log.SetFlags(0)

	pid := fmt.Sprint(os.Getpid())

	storagePath := "/tmp/" + pid + ".db"
	storage, e := storage.NewBoltDBStorage(storagePath)
	defer storage.Destroy()
	if e == nil {
		consumeNewcomers, produceNewcomer, closeNewcomers := wampShared.NewStream[*wamp.Peer]()
		defer closeNewcomers()

		session := routerServe(consumeNewcomers, produceNewcomer, storage)

		cluster := service.NewEventDistributor()
		if cluster.FriendsCount() > 0 {
			service.SpawnEventRepeater(session, cluster)
		}

		interviewer, _ := service.NewInterviewer(session)
		httpServe(interviewer, produceNewcomer, *address, true)
	}
}
