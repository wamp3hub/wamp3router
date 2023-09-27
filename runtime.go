package wamp3router

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	client "github.com/wamp3hub/wamp3go"
	clientShared "github.com/wamp3hub/wamp3go/shared"
	"github.com/wamp3hub/wamp3go/transport"

	"github.com/wamp3hub/wamp3router/server"
	"github.com/wamp3hub/wamp3router/storage"
)

type Newcomers = clientShared.Consumer[*client.Peer]

type Storage interface {
	Get(bucketName string, key string, data any) error
	Set(bucketName string, key string, data any) error
	Delete(bucketName string, key string)
	Destroy() error
}

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

		broker := newBroker(storage)
		dealer := newDealer(storage)

		broker.setup(session, dealer)
		dealer.setup(session, broker)

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
		broker.serve(newcomers)
		dealer.serve(newcomers)

		go alphaPeer.Consume()
		newcomersProducer.Produce(betaPeer)

		http.Handle("/wamp3/gateway", server.GatewayMount(session))
		if !disableWebsocket {
			http.Handle("/wamp3/websocket", server.WebsocketMount(newcomersProducer))
		}
		log.Printf("[router] Starting at %s", address)
		e = http.ListenAndServe(address, nil)
	}

	return e
}
