package router

import (
	"errors"
	"log/slog"
	"sort"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

var (
	ErrorProcedureNotFound = errors.New("procedure not found")
)

type RegistrationList = routerShared.ResourceList[*wamp.RegisterOptions]

type Dealer struct {
	routerID      string
	peers         cmap.ConcurrentMap[string, *wamp.Peer]
	counter       cmap.ConcurrentMap[string, int]
	registrations *routerShared.URIM[*wamp.RegisterOptions]
	logger        *slog.Logger
}

func NewDealer(
	routerID string,
	storage routerShared.Storage,
	logger *slog.Logger,
) *Dealer {
	return &Dealer{
		routerID,
		cmap.New[*wamp.Peer](),
		cmap.New[int](),
		routerShared.NewURIM[*wamp.RegisterOptions](storage, logger),
		logger.With("name", "Dealer"),
	}
}

func shift[T any](items []T, x int) []T {
	return append(items[x:], items[:x]...)
}

func (dealer *Dealer) matchRegistrations(
	uri string,
) RegistrationList {
	registrationList := dealer.registrations.Match(uri)

	n := len(registrationList)
	if n > 0 {
		sort.Slice(
			registrationList,
			func(i, j int) bool {
				return registrationList[i].Options.Distance() > registrationList[j].Options.Distance()
			},
		)

		count, _ := dealer.counter.Get(uri)
		offset := count % n
		registrationList = shift(registrationList, offset)
		dealer.counter.Set(uri, count+1)
	}

	return registrationList
}

func (dealer *Dealer) sendReply(
	caller *wamp.Peer,
	event wamp.ReplyEvent,
) {
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, dealer.routerID)

	logData := slog.Group(
		"response",
		"CallerID", caller.Details.ID,
		"InvocationID", features.InvocationID,
		"VisitedRouters", features.VisitedRouters,
	)

	ok := caller.Send(event, wamp.DEFAULT_RESEND_COUNT)
	if ok {
		dealer.logger.Debug("invocation processed successfully", logData)
	} else {
		dealer.logger.Error("reply event dispatch error", logData)
	}
}

func (dealer *Dealer) onCall(
	caller *wamp.Peer,
	callEvent wamp.CallEvent,
) error {
	callFeatures := callEvent.Features()
	timeout := time.Duration(callFeatures.Timeout) * time.Second

	route := callEvent.Route()
	route.CallerID = caller.Details.ID
	route.VisitedRouters = append(route.VisitedRouters, dealer.routerID)

	cancelEventPromise, cancelCancelEventPromise := caller.PendingReplyEvents.New(
		callEvent.ID(), timeout,
	)

	requestLogData := slog.Group(
		"event",
		"ID", callEvent.ID,
		"URI", callFeatures.URI,
		"CallerID", route.CallerID,
		"VisitedRouters", route.VisitedRouters,
		"Timeout", timeout,
	)
	dealer.logger.Debug("call", requestLogData)

	registrationList := dealer.matchRegistrations(callFeatures.URI)

	for _, registration := range registrationList {
		registrationLogData := slog.Group(
			"subscription",
			"ID", registration.ID,
			"URI", registration.URI,
			"SubscriberID", registration.AuthorID,
		)

		if caller.Details.ID == registration.AuthorID {
			// It is forbidden to call yourself
			continue
		}

		executor, found := dealer.peers.Get(registration.AuthorID)
		if !found {
			dealer.logger.Error("invalid registration (peer not found)", registrationLogData, requestLogData)
			continue
		}

		if !callFeatures.Authorized(executor.Details.Role) ||
			!registration.Options.Authorized(caller.Details.Role) {
			dealer.logger.Debug("exclude registration (denied)", registrationLogData, requestLogData)
			continue
		}

		route.EndpointID = registration.ID
		route.ExecutorID = executor.Details.ID

		replyEventPromise, cancelReplyEventPromise := executor.PendingReplyEvents.New(callEvent.ID(), 0)

		ok := executor.Send(callEvent, wamp.DEFAULT_RESEND_COUNT)
		if !ok {
			dealer.logger.Error("call event dispatch error", registrationLogData, requestLogData)
			continue
		}

		dealer.logger.Debug("reply event sent", registrationLogData, requestLogData)

		select {
		case cancelEvent, done := <-cancelEventPromise:
			cancelReplyEventPromise()

			if done {
				dealer.logger.Debug("call event cancelled", registrationLogData, requestLogData)

				dealer.sendReply(executor, cancelEvent)
			} else {
				dealer.logger.Debug("call event timeout", registrationLogData, requestLogData)

				response := wamp.NewErrorEvent(callEvent, wamp.ErrorTimedOut)
				dealer.sendReply(caller, response)
				dealer.sendReply(executor, response)
			}
		case response := <-replyEventPromise:
			cancelCancelEventPromise()

			dealer.sendReply(caller, response)
		}

		return nil
	}

	cancelCancelEventPromise()

	dealer.logger.Debug("procedure not found", requestLogData)
	response := wamp.NewErrorEvent(callEvent, ErrorProcedureNotFound)
	dealer.sendReply(caller, response)

	return nil
}

func (dealer *Dealer) onLeave(peer *wamp.Peer) {
	dealer.peers.Remove(peer.Details.ID)
	dealer.logger.Debug("dettach peer", "ID", peer.Details.ID)
}

func (dealer *Dealer) onJoin(peer *wamp.Peer) {
	dealer.logger.Debug("attach peer", "ID", peer.Details.ID)
	dealer.peers.Set(peer.Details.ID, peer)
	peer.IncomingCallEvents.Observe(
		func(event wamp.CallEvent) { dealer.onCall(peer, event) },
		func() { dealer.onLeave(peer) },
	)
}

func (dealer *Dealer) Serve(newcomers *wampShared.Observable[*wamp.Peer]) {
	dealer.logger.Debug("up...")
	newcomers.Observe(
		dealer.onJoin,
		func() { dealer.logger.Debug("down...") },
	)
}
