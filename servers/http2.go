package routerServers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	wampInterview "github.com/wamp3hub/wamp3go/transports/interview"
	router "github.com/wamp3hub/wamp3router"
)

var readJSONBody = wampInterview.ReadJSONBody

func writeJSONBody(
	w http.ResponseWriter,
	statusCode int,
	payload any,
) error {
	userError, ok := payload.(error)
	if ok {
		payload = wampInterview.ErrorPayload{Message: userError.Error()}
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
	router          *router.Router
	logger          *slog.Logger
	super           *http.Server
}

func NewHTTP2Server(
	address string,
	enableWebsocket bool,
	router *router.Router,
	logger *slog.Logger,
) *HTTP2Server {
	return &HTTP2Server{
		enableWebsocket,
		address,
		router,
		logger.With("name", "HTTP2Server"),
		&http.Server{},
	}
}

func (server *HTTP2Server) Serve() error {
	serveMux := http.NewServeMux()

	serveMux.Handle(
		"/wamp/v1/interview",
		http2interviewMount(server.router.Session, server.router.KeyRing, server.logger),
	)
	if server.EnableWebsocket {
		serveMux.Handle(
			"/wamp/v1/websocket",
			http2websocketMount(server.router.KeyRing, server.router.Newcomers, server.logger),
		)
	}

	server.super = &http.Server{Addr: server.Address, Handler: serveMux}

	server.logger.Info("listening...", "HTTP2Server.Address", server.Address)
	e := server.super.ListenAndServe()
	return e
}

func (server *HTTP2Server) Shutdown() error {
	server.logger.Info("shutting down...")
	e := server.super.Shutdown(context.TODO())
	return e
}
