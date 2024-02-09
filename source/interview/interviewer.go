package routerInterview

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampInterview "github.com/wamp3hub/wamp3go/interview"
	router "github.com/wamp3hub/wamp3router/source"
)

var defaultOffer = wampInterview.Offer{
	RegistrationsLimit: 10,
	SubscriptionsLimit: 10,
	TicketLifeTime:     10080, // minutes (1 weak)
}

// RENAME
type Interviewer struct {
	session *wamp.Session
	logger  *slog.Logger
}

func NewInterviewer(
	session *wamp.Session,
	logger *slog.Logger,
) *Interviewer {
	return &Interviewer{
		session,
		logger.With("name", "Interviewer"),
	}
}

func (interviewer *Interviewer) Authenticate(
	resume *wampInterview.Resume[any],
) (*wampInterview.Offer, error) {
	pendingResponse := wamp.Call[wampInterview.Offer](
		interviewer.session,
		&wamp.CallFeatures{URI: "wamp.authenticate", IncludeRoles: []string{"interviewer"}},
		resume,
	)
	_, offer, e := pendingResponse.Await()
	if e == nil {
		interviewer.logger.Info("authentication success", "offer", offer)
		if offer.TicketLifeTime == 0 {
			offer.TicketLifeTime = 1
		}
		return &offer, nil
	} else if e.Error() == router.ErrorProcedureNotFound.Error() {
		interviewer.logger.Warn("please, register `wamp.authenticate`")
		return &defaultOffer, nil
	}
	return nil, e
}
