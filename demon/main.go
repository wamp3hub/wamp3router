package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	wamp "github.com/wamp3hub/wamp3go"
	clientShared "github.com/wamp3hub/wamp3go/shared"
	"github.com/wamp3hub/wamp3go/transport"

	"github.com/rs/xid"

	router "github.com/wamp3hub/wamp3router"
	"github.com/wamp3hub/wamp3router/server"
	"github.com/wamp3hub/wamp3router/service"
	"github.com/wamp3hub/wamp3router/storage"
)

func Serve(
	address string,
	disableWebsocket bool,
) error {
	log.SetFlags(0)

	pid := fmt.Sprint(os.Getpid())
	log.Printf("[router] up... (PID=%s)", pid)

	storagePath := "/tmp/" + pid + ".db"
	storage, e := storage.NewBoltDBStorage(storagePath)
	defer storage.Destroy()
	if e == nil {
		peerID := xid.New().String()
		alphaTransport, betaTransport := transport.NewDuplexLocalTransport()
		alphaPeer := wamp.NewPeer(peerID, alphaTransport)
		betaPeer := wamp.NewPeer(peerID, betaTransport)
		session := wamp.NewSession(alphaPeer)

		broker := router.NewBroker(storage)
		dealer := router.NewDealer(storage)

		broker.Setup(session, dealer)
		dealer.Setup(session, broker)

		newcomersProducer, newcomers := clientShared.NewStream[*wamp.Peer]()
		defer newcomersProducer.Close()
		newcomers.Consume(
			func(peer *wamp.Peer) {
				log.Printf("[router] attach peer (ID=%s)", peer.ID)
				peer.Consume()
				log.Printf("[router] dettach peer (ID=%s)", peer.ID)
			},
			func() { log.Printf("[router] down...") },
		)
		broker.Serve(newcomers)
		dealer.Serve(newcomers)

		go alphaPeer.Consume()
		newcomersProducer.Produce(betaPeer)

		interviewer, _ := service.NewInterviewer(session)

		http.Handle("/wamp3/interview", server.InterviewMount(interviewer))
		if !disableWebsocket {
			http.Handle("/wamp3/websocket", server.WebsocketMount(interviewer, newcomersProducer))
		}
		log.Printf("[router] Starting at %s", address)
		e = http.ListenAndServe(address, nil)
	}

	return e
}

func main() {
	Serve(":8888", false)
}
