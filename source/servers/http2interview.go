package routerServers

import (
	"log/slog"
	"net/http"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampInterview "github.com/wamp3hub/wamp3go/interview"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerInterview "github.com/wamp3hub/wamp3router/source/interview"
	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

func http2interviewMount(
	session *wamp.Session,
	keyRing *routerShared.KeyRing,
	__logger *slog.Logger,
) http.Handler {
	logger := __logger.With("server", "http2interview")
	interviewer := routerInterview.NewInterviewer(session, __logger)

	onInterview := func(request *http.Request) (int, any) {
		if request.Method == "OPTIONS" {
			return 200, nil
		}

		resume := new(wampInterview.Resume[any])
		e := readJSONBody(request.Body, resume)
		if e != nil {
			logger.Error("invalid payload", "error", e)
			return 400, e
		}

		offer, e := interviewer.Authenticate(resume)
		if e != nil {
			logger.Error("during authenticate", "error", e)
			return 400, e
		}

		claims := routerShared.NewJWTClaims(
			session.ID(),
			session.ID()+"-"+wampShared.NewID(),
			resume.Role,
			offer,
			time.Hour*24*7, // 1 weak
		)
		ticket, _ := keyRing.JWTSign(claims)

		result := wampInterview.Result{
			RouterID: claims.Issuer,
			YourID:   claims.Subject,
			Ticket:   ticket,
			Offer:    offer,
		}
		return 200, result
	}

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", jsonEndpoint(onInterview))
	return serveMux
}
