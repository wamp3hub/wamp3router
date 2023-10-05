package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	wamp "github.com/wamp3hub/wamp3go"
	"github.com/wamp3hub/wamp3go/serializer"
	"github.com/wamp3hub/wamp3go/shared"
	"github.com/wamp3hub/wamp3go/transport"

	service "github.com/wamp3hub/wamp3router/service"
)

func WebsocketMount(
	interviewer *service.Interviewer,
	newcomers *shared.Producer[*wamp.Peer],
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
		token := query.Get("token")
		claims, e := interviewer.Decode(token)
		if e == nil {
			// serializerCode := query.Get("serializer")
			__serializer := new(serializer.JSONSerializer)
			connection, e := websocketUpgrader.Upgrade(w, r, nil)
			if e == nil {
				__transport := transport.WSTransport(__serializer, connection)
				peer := wamp.NewPeer(claims.Subject, __transport)
				newcomers.Produce(peer)
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
