package routerServers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampInterview "github.com/wamp3hub/wamp3go/transports/interview"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

var readJSONBody = wampInterview.ReadJSONBody

func writeJSONBody(
	w http.ResponseWriter,
	statusCode int,
	payload any,
) error {
	userError, ok := payload.(error)
	if ok {
		payload = wampInterview.ErrorPayload{Code: userError.Error()}
	}

	responseBodyBytes, e := json.Marshal(payload)
	if e == nil {
		responseHeaders := w.Header()
		responseHeaders.Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(responseBodyBytes)
	}
	return e
}

func jsonEndpoint(
	procedure func(*http.Request) (int, any),
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode, payload := procedure(r)
		writeJSONBody(w, statusCode, payload)
	}
}

type HTTP2Server struct {
	EnableWebsocket bool
	Address         string
	ProduceNewcomer wampShared.Producible[*wamp.Peer]
	Session         *wamp.Session
	KeyRing         *routerShared.KeyRing
	super           *http.Server
}

func (server *HTTP2Server) Serve() error {
	serveMux := http.NewServeMux()

	serveMux.Handle("/wamp/v1/interview", http2interviewMount(server.Session, server.KeyRing))
	if server.EnableWebsocket {
		serveMux.Handle("/wamp/v1/websocket", http2websocketMount(server.KeyRing, server.ProduceNewcomer))
	}

	server.super = &http.Server{Addr: server.Address, Handler: serveMux}

	log.Printf("[http2-server] listening %s", server.Address)
	e := server.super.ListenAndServe()
	return e
}

func (server *HTTP2Server) Shutdown() error {
	log.Printf("[http2-server] shutting down...")
	e := server.super.Shutdown(context.TODO())
	return e
}
