package main

import (
	"flag"
	"fmt"
	"log"
	"os"

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
	e = router.HTTPServe(session, keyRing, produceNewcomer, *address, true)
}
