package routerServers

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
)

type Authenticator interface {
	authenticate() error
}

type DynamicAuthenticator struct {
	session *wamp.Session
	logger  *slog.Logger
}

func NewDynamicAuthenticator(
	session *wamp.Session,
	logger *slog.Logger,
) *DynamicAuthenticator {
	return &DynamicAuthenticator{
		session,
		logger.With("name", "DynamicAuthenticator"),
	}
}

func (authenticator *DynamicAuthenticator) authenticate(
	session *wamp.Session,
	credentials any,
) error {
	pendingResponse := wamp.Call[string](
		session,
		&wamp.CallFeatures{URI: "wamp.authenticate"},
		credentials,
	)
	_, role, e := pendingResponse.Await()
	if e == nil {
		authenticator.logger.Info("authentication success", "Role", role)
		return nil
	} else if e.Error() == wamp.ErrorProcedureNotFound.Error() {
		authenticator.logger.Warn("please, register `wamp.authenticate`")
		return nil
	}
	return e
}
