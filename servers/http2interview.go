package routerServers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampInterview "github.com/wamp3hub/wamp3go/transports/interview"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

func http2interviewMount(
	session *wamp.Session,
	keyRing *routerShared.KeyRing,
	__logger *slog.Logger,
) http.Handler {
	logger := __logger.With("server", "http2interview")

	authenticate := func(session *wamp.Session, credentials any) error {
		pendingResponse := wamp.Call[string](
			session,
			&wamp.CallFeatures{URI: "wamp.authenticate"},
			credentials,
		)
		_, role, e := pendingResponse.Await()
		if e == nil {
			logger.Info("authentication success", "Role", role)
			return nil
		} else if e.Error() == "ProcedureNotFound" {
			logger.Warn("please, register `wamp.authenticate`")
			return nil
		}
		return e
	}

	onInterview := func(request *http.Request) (int, any) {
		requestPayload := new(wampInterview.Payload)
		e := readJSONBody(request.Body, requestPayload)
		if e != nil {
			logger.Error("invalid payload", "error", e)
			return 400, e
		}

		e = authenticate(session, requestPayload.Credentials)
		if e != nil {
			logger.Error("during authentication", "error", e)
			return 400, e
		}

		now := time.Now()
		claims := routerShared.JWTClaims{
			Issuer:    session.ID(),
			Subject:   session.ID() + "-" + wampShared.NewID(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		}
		ticket, _ := keyRing.JWTSign(&claims)
		responsePayload := wampInterview.SuccessPayload{
			RouterID: claims.Issuer,
			YourID:   claims.Subject,
			Ticket:   ticket,
		}
		logger.Debug("success", "peerID", responsePayload.YourID)
		return 200, responsePayload
	}

	logger.Info("up...")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", jsonEndpoint(onInterview))
	return serveMux
}
