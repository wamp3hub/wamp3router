package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	client "github.com/wamp3hub/wamp3go"
	clientShared "github.com/wamp3hub/wamp3go/shared"
	"github.com/wamp3hub/wamp3go/transport"

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
		peerID := uuid.NewString()
		alphaTransport, betaTransport := transport.NewDuplexLocalTransport()
		alphaPeer := client.NewPeer(peerID, alphaTransport)
		betaPeer := client.NewPeer(peerID, betaTransport)
		session := client.NewSession(alphaPeer)

		broker := router.NewBroker(storage)
		dealer := router.NewDealer(storage)

		broker.Setup(session, dealer)
		dealer.Setup(session, broker)

		newcomersProducer, newcomers := clientShared.NewStream[*client.Peer]()
		defer newcomersProducer.Close()
		newcomers.Consume(
			func(peer *client.Peer) {
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
