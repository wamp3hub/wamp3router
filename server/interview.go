package server

import (
	"log"
	"net/http"

	interview "github.com/wamp3hub/wamp3go/transport/interview"

	service "github.com/wamp3hub/wamp3router/service"
)

func InterviewMount(interviewer *service.Interviewer) http.Handler {
	onInterview := func(request *http.Request) (int, any) {
		requestPayload := interview.Payload{}
		e := readJSONBody(request.Body, &requestPayload)
		if e == nil {
			claims, e := interviewer.GenerateClaims(requestPayload.Credentials)
			if e == nil {
				token, e := interviewer.Encode(claims)
				if e == nil {
					responsePayload := interview.SuccessPayload{claims.Subject, token}
					log.Printf("[interview] success (peer.ID=%s)", responsePayload.PeerID)
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
