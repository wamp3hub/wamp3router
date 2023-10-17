package server

import (
	"log"
	"net/http"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampInterview "github.com/wamp3hub/wamp3go/transport/interview"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/xid"

	service "github.com/wamp3hub/wamp3router/service"
)

func rpcAuthenticate(session *wamp.Session, credentials any) error {
	callEvent := wamp.NewCallEvent(&wamp.CallFeatures{"wamp.authenticate"}, credentials)
	replyEvent := session.Call(callEvent)
	e := replyEvent.Error()
	if e == nil {
		// TODO
		return nil
	} else if e.Error() == "ProcedureNotFound" {
		log.Printf("[interviewer] please, register `wamp.authenticate`")
		return nil
	}
	return e
}

func InterviewMount(session *wamp.Session, keyRing *service.KeyRing) http.Handler {
	onInterview := func(request *http.Request) (int, any) {
		requestPayload := wampInterview.Payload{}
		e := readJSONBody(request.Body, &requestPayload)
		if e == nil {
			e = rpcAuthenticate(session, requestPayload.Credentials)
			if e == nil {
				now := time.Now()
				claims := service.Claims{
					Issuer:    session.ID(),
					Subject:   session.ID() + "-" + xid.New().String(),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
					IssuedAt:  jwt.NewNumericDate(now),
				}
				ticket, e := keyRing.JWTEncode(&claims)
				if e == nil {
					responsePayload := wampInterview.SuccessPayload{claims.Issuer, claims.Subject, ticket}
					log.Printf("[interview] success (peer.ID=%s)", responsePayload.YourID)
					return 200, responsePayload
				}
			}
		}
		log.Printf("[interview] error=%s", e)
		return 400, e
	}

	log.Print("[interview] up...")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", jsonEndpoint(onInterview))
	return serveMux
}
