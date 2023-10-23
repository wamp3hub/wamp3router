package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	wamp "github.com/wamp3hub/wamp3go"
	wampSerializer "github.com/wamp3hub/wamp3go/serializer"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransport "github.com/wamp3hub/wamp3go/transport"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

func WebsocketMount(
	keyRing *routerShared.KeyRing,
	produceNewcomers wampShared.Producible[*wamp.Peer],
) http.Handler {
	websocketUpgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// creates websocket connection
	onWebsocketUpgrade := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[websocket] new upgrade request (ip=%s)", r.RemoteAddr)
		query := r.URL.Query()
		ticket := query.Get("ticket")
		claims, e := keyRing.JWTParse(ticket)
		if e == nil {
			header := w.Header()
			header.Set("X-WAMP-RouterID", claims.Issuer)
			connection, e := websocketUpgrader.Upgrade(w, r, nil)
			if e == nil {
				// serializerCode := query.Get("serializer")
				__transport := wampTransport.WSTransport(wampSerializer.DefaultSerializer, connection)
				peer := wamp.SpawnPeer(claims.Subject, __transport)
				produceNewcomers(peer)
				log.Printf("[websocket] new peer (ID=%s)", peer.ID)
			} else {
				log.Printf("[websocket] %s", e)
			}
		} else {
			writeJSONBody(w, 400, e)
		}
	}

	log.Print("[websocket] up...")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", onWebsocketUpgrade)
	return serveMux
}
