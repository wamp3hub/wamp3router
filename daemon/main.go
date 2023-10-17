package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransport "github.com/wamp3hub/wamp3go/transport"

	router "github.com/wamp3hub/wamp3router"
	"github.com/wamp3hub/wamp3router/server"
	"github.com/wamp3hub/wamp3router/service"
	"github.com/wamp3hub/wamp3router/storage"
)

func clusterJoin(
	addressList []string,
	ticket string,
	ring *service.KeyRing,
) (cluster *service.EventDistributor, e error) {
	cluster = service.NewEventDistributor(ticket)
	cluster.WebsocketJoinCluster(addressList)

	if cluster.FriendsCount() > 0 {
		e = service.LoadClusterKeys(ring, cluster)
	} else {
		e = errors.New("failed to join cluster")
	}

	return cluster, e
}

func routerServe(
	peerID string,
	consumeNewcomers wampShared.Consumable[*wamp.Peer],
	produceNewcomer wampShared.Producible[*wamp.Peer],
	storage router.Storage,
) *wamp.Session {
	log.Printf("[router] up...")

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
	session *wamp.Session,
	keyRing *service.KeyRing,
	produceNewcomer wampShared.Producible[*wamp.Peer],
	address string,
	enableWebsocket bool,
) error {
	http.Handle("/wamp3/interview", server.InterviewMount(session, keyRing))
	if enableWebsocket {
		http.Handle("/wamp3/websocket", server.WebsocketMount(keyRing, produceNewcomer))
	}

	log.Printf("[router] Starting at %s", address)
	e := http.ListenAndServe(address, nil)
	return e
}

func main() {
	pid := os.Getpid()

	address := flag.String("address", ":8888", "")
	rawCluster := flag.String("cluster", "", "")
	ticket := flag.String("ticket", "", "")
	__storagePath := "/tmp/wamp3router" + fmt.Sprint(pid) + ".db"
	storagePath := flag.String("storagePath", __storagePath, "")
	flag.Parse()

	log.SetFlags(0)

	storage, e := storage.NewBoltDBStorage(*storagePath)
	if e != nil {
		panic("failed to initialize storage")
	}
	defer storage.Destroy()

	consumeNewcomers, produceNewcomer, closeNewcomers := wampShared.NewStream[*wamp.Peer]()
	defer closeNewcomers()

	keyRing := service.NewKeyRing()

	var session *wamp.Session
	if len(*rawCluster) > 0 && len(*ticket) > 0 {
		addressList := strings.Split(*rawCluster, ",")

		cluster, e := clusterJoin(addressList, *ticket, keyRing)
		if e != nil {
			panic(e)
		}

		myClaims, e := keyRing.JWTDecode(*ticket)
		if e != nil {
			panic(e)
		}

		session = routerServe(myClaims.Subject, consumeNewcomers, produceNewcomer, storage)

		service.SpawnEventRepeater(session, cluster)
	} else {
		peerID := xid.New().String()
		session = routerServe(peerID, consumeNewcomers, produceNewcomer, storage)
	}

	now := time.Now()
	claims := service.Claims{
		Issuer:    session.ID(),
		Subject:   xid.New().String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	__ticket, _ := keyRing.JWTEncode(&claims)
	log.Printf("new cluster ticket %s", __ticket)

	time.Sleep(time.Second)

	service.ShareClusterKeys(keyRing, session)

	httpServe(session, keyRing, produceNewcomer, *address, true)
}
