package routerServers

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"

	wamp "github.com/wamp3hub/wamp3go"
	wampSerializers "github.com/wamp3hub/wamp3go/serializers"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

func http2websocketMount(
	keyRing *routerShared.KeyRing,
	newcomers *wampShared.Observable[*wamp.Peer],
	__logger *slog.Logger,
) http.Handler {
	logger := __logger.With("name", "http2websocket")

	websocketUpgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// creates websocket connection
	onWebsocketUpgrade := func(w http.ResponseWriter, r *http.Request) {
		logger.Info("new upgrade request", "clientAddress", r.RemoteAddr)
		query := r.URL.Query()
		ticket := query.Get("ticket")
		claims, e := keyRing.JWTParse(ticket)
		if e == nil {
			header := w.Header()
			header.Set("X-WAMP-RouterID", claims.Issuer)
			connection, e := websocketUpgrader.Upgrade(w, r, nil)
			if e == nil {
				// serializerCode := query.Get("serializer")
				transport := wampTransports.WSTransport{
					Address: r.RemoteAddr,
					Serializer: wampSerializers.DefaultSerializer,
					Connection: connection,
				}
				peer := wamp.SpawnPeer(claims.Subject, &transport, logger)
				newcomers.Next(peer)
				logger.Info("new peer", "ID", peer.ID)
			} else {
				logger.Error("failed to upgrade", "error", e)
			}
		} else {
			writeJSONBody(w, 400, e)
		}
	}

	logger.Info("up...")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", onWebsocketUpgrade)
	return serveMux
}
