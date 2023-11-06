package server

import (
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"
	wamp "github.com/wamp3hub/wamp3go"
	wampInterview "github.com/wamp3hub/wamp3go/transport/interview"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

func rpcAuthenticate(session *wamp.Session, credentials any) error {
	pendingResponse := wamp.Call[string](session, &wamp.CallFeatures{URI: "wamp.authenticate"}, credentials)
	_, _, e := pendingResponse.Await()
	if e == nil {
		// TODO roles
		return nil
	} else if e.Error() == "ProcedureNotFound" {
		log.Printf("[interview] please, register `wamp.authenticate`")
		return nil
	}
	return e
}

func InterviewMount(session *wamp.Session, keyRing *routerShared.KeyRing) http.Handler {
	onInterview := func(request *http.Request) (int, any) {
		requestPayload := wampInterview.Payload{}
		e := readJSONBody(request.Body, &requestPayload)
		if e == nil {
			e = rpcAuthenticate(session, requestPayload.Credentials)
			if e == nil {
				now := time.Now()
				claims := routerShared.JWTClaims{
					Issuer:    session.ID(),
					Subject:   session.ID() + "-" + xid.New().String(),
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
