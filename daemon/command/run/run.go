package run

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
	"github.com/spf13/cobra"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransport "github.com/wamp3hub/wamp3go/transport"

	router "github.com/wamp3hub/wamp3router"
	routerServer "github.com/wamp3hub/wamp3router/server"
	routerShared "github.com/wamp3hub/wamp3router/shared"
	routerStorage "github.com/wamp3hub/wamp3router/storage"
)

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

func Serve(
	address string,
	storagePath string,
) {
	log.SetFlags(0)

	storage, e := routerStorage.NewBoltDBStorage(storagePath)
	if e != nil {
		panic("failed to initialize storage")
	}
	defer storage.Destroy()

	consumeNewcomers, produceNewcomer, closeNewcomers := wampShared.NewStream[*wamp.Peer]()
	defer closeNewcomers()

	peerID := xid.New().String()

	alphaTransport, betaTransport := wampTransport.NewDuplexLocalTransport(128)
	alphaPeer := wamp.SpawnPeer(peerID, alphaTransport)
	betaPeer := wamp.SpawnPeer(peerID, betaTransport)
	session := wamp.NewSession(alphaPeer)

	router.Serve(session, storage, consumeNewcomers)

	produceNewcomer(betaPeer)

	// go routerServer.UnixMount("/tmp/wamp-cli.socket", produceNewcomer)

	keyRing := routerShared.NewKeyRing()

	now := time.Now()
	claims := routerShared.JWTClaims{
		Issuer:    session.ID(),
		Subject:   xid.New().String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	__ticket, _ := keyRing.JWTSign(&claims)
	log.Printf("new cluster ticket %s", __ticket)

	e = HTTPServe(session, keyRing, produceNewcomer, address, true)
}

var Command = &cobra.Command{
	Use:   "run",
	Short: "Run",
	Run: func(cmd *cobra.Command, args []string) {
		pid := os.Getpid()
		storagePath := "/tmp/wamp3router" + fmt.Sprint(pid) + ".db"
		address := ":8889"
		Serve(address, storagePath)
	},
}

func init() {
}
