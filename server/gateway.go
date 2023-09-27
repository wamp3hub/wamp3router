package server

import (
	"log"
	"net/http"

	client "github.com/wamp3hub/wamp3go"
	clientJoin "github.com/wamp3hub/wamp3go/transport/join"
)

func GatewayMount(
	session *client.Session,
) http.Handler {
	onJoin := func(request *http.Request) (int, any) {
		log.Print("[gateway] new join request")
		requestPayload := clientJoin.JoinPayload{}
		e := readJSONBody(request.Body, &requestPayload)
		if e == nil {
			callEvent := client.NewCallEvent(&client.CallFeatures{"wamp.join"}, requestPayload)
			replyEvent := session.Call(callEvent)
			replyFeatures := replyEvent.Features()
			if replyFeatures.OK {
				responsePayload := clientJoin.SuccessJoinPayload{}
				e = replyEvent.Payload(&responsePayload)
				if e == nil {
					log.Printf("[gateway] successfull call(wamp.join) (peer.ID=%s)", responsePayload.PeerID)
					return 200, responsePayload
				}
			} else {
				e = client.ExtractError(replyEvent)
			}
		}
		log.Printf("[gateway] %s", e)
		return 400, e
	}

	log.Print("[gateway] up...")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", jsonEndpoint(onJoin))
	return serveMux
}
