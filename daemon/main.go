package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	router "github.com/wamp3hub/wamp3router"
	routerShared "github.com/wamp3hub/wamp3router/shared"
	routerStorage "github.com/wamp3hub/wamp3router/storage"
)

func main() {
	pid := os.Getpid()

	address := flag.String("address", ":8888", "")
	__storagePath := "/tmp/wamp3router" + fmt.Sprint(pid) + ".db"
	storagePath := flag.String("storagePath", __storagePath, "")
	flag.Parse()

	log.SetFlags(0)

	storage, e := routerStorage.NewBoltDBStorage(*storagePath)
	if e != nil {
		panic("failed to initialize storage")
	}
	defer storage.Destroy()

	consumeNewcomers, produceNewcomer, closeNewcomers := wampShared.NewStream[*wamp.Peer]()
	defer closeNewcomers()

	peerID := xid.New().String()
	session := router.Initialize(peerID, consumeNewcomers, produceNewcomer, storage)

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

	e = router.HTTPServe(session, keyRing, produceNewcomer, *address, true)
}
