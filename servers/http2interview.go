package routerServers

import (
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampInterview "github.com/wamp3hub/wamp3go/transports/interview"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

func rpcAuthenticate(session *wamp.Session, credentials any) error {
	pendingResponse, e := wamp.Call[string](session, &wamp.CallFeatures{URI: "wamp.authenticate"}, credentials)
	_, role, e := pendingResponse.Await()
	if e == nil {
		log.Printf("authentication success role=%s", role)
		return nil
	} else if e.Error() == "ProcedureNotFound" {
		log.Printf("[interview] please, register `wamp.authenticate`")
		return nil
	}
	return e
}

func http2interviewMount(session *wamp.Session, keyRing *routerShared.KeyRing) http.Handler {
	onInterview := func(request *http.Request) (int, any) {
		requestPayload := new(wampInterview.Payload)
		e := readJSONBody(request.Body, requestPayload)
		if e == nil {
			e = rpcAuthenticate(session, requestPayload.Credentials)
			if e == nil {
				now := time.Now()
				claims := routerShared.JWTClaims{
					Issuer:    session.ID(),
					Subject:   session.ID() + "-" + wampShared.NewID(),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
					IssuedAt:  jwt.NewNumericDate(now),
				}
				ticket, e := keyRing.JWTSign(&claims)
				if e == nil {
					responsePayload := wampInterview.SuccessPayload{
						RouterID: claims.Issuer,
						YourID:   claims.Subject,
						Ticket:   ticket,
					}
					log.Printf("[http2-interview] success (peer.ID=%s)", responsePayload.YourID)
					return 200, responsePayload
				}
			}
		}
		log.Printf("[http2-interview] error=%s", e)
		return 400, e
	}

	log.Print("[http2-interview] up...")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", jsonEndpoint(onInterview))
	return serveMux
}
