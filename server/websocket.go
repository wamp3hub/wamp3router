package server

import (
	"log"
	"net/http"

	client "github.com/wamp3hub/wamp3go"
	"github.com/wamp3hub/wamp3go/serializer"
	"github.com/wamp3hub/wamp3go/shared"
	"github.com/wamp3hub/wamp3go/transport"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func WebsocketMount(
	newcomers *shared.Producer[*client.Peer],
) http.Handler {
	// Upgrades http request to websocket
	onWebsocketUpgrade := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[websocket] new upgrade request (ip=%s)", r.RemoteAddr)
		// e := verifyToken()
		e := error(nil)
		if e == nil {
			log.Printf("[websocket] token found")
			websocketUpgrader := websocket.Upgrader{
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			}
			connection, e := websocketUpgrader.Upgrade(w, r, nil)
			if e == nil {
				peerID := uuid.NewString()
				log.Printf("[websocket] new peer (ID=%s)", peerID)
				serializer := new(serializer.JSONSerializer)
				transport := transport.WSTransport(serializer, connection)
				peer := client.NewPeer(peerID, transport)
				newcomers.Produce(peer)
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
