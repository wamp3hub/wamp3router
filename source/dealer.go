package router

import (
	"log/slog"
	"sort"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

type RegistrationList = routerShared.ResourceList[*wamp.RegisterOptions]

type Dealer struct {
	routerID      string
	peers         map[string]*wamp.Peer
	counter       map[string]int
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
		make(map[string]*wamp.Peer),
		make(map[string]int),
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

		offset := dealer.counter[uri] % n
		registrationList = shift(registrationList, offset)
		dealer.counter[uri] += 1
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
		"CallerID", caller.ID,
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
	features := callEvent.Features()
	timeout := time.Duration(features.Timeout) * time.Second

	route := callEvent.Route()
	route.CallerID = caller.ID
	route.VisitedRouters = append(route.VisitedRouters, dealer.routerID)

	cancelCallEventPromise, cancelCancelEventPromise := caller.PendingCancelEvents.New(
		callEvent.ID(), timeout,
	)

	requestLogData := slog.Group(
		"event",
		"ID", callEvent.ID,
		"URI", features.URI,
		"CallerID", caller.ID,
		"Timeout", timeout,
		"VisitedRouters", route.VisitedRouters,
	)
	dealer.logger.Debug("call", requestLogData)

	registrationList := dealer.matchRegistrations(features.URI)

	for _, registration := range registrationList {
		registrationLogData := slog.Group(
			"subscription",
			"ID", registration.ID,
			"URI", registration.URI,
			"SubscriberID", registration.AuthorID,
		)

		executor, exists := dealer.peers[registration.AuthorID]
		if !exists {
			dealer.logger.Error("invalid registartion (peer not found)", registrationLogData, requestLogData)
			continue
		}

		route.EndpointID = registration.ID
		route.ExecutorID = executor.ID

		replyEventPromise, cancelReplyEventPromise := executor.PendingReplyEvents.New(callEvent.ID(), 0)
		ok := executor.Send(callEvent, wamp.DEFAULT_RESEND_COUNT)
		if !ok {
			dealer.logger.Error("call event dispatch error", registrationLogData, requestLogData)
			continue
		}
		dealer.logger.Debug("reply event sent", registrationLogData, requestLogData)

		select {
		case cancelEvent, done := <-cancelCallEventPromise:
			cancelReplyEventPromise()

			if done {
				cancelFeatures := cancelEvent.Features()
				cancelFeatures.VisitedRouters = append(cancelFeatures.VisitedRouters, dealer.routerID)
				ok := executor.Send(cancelEvent, wamp.DEFAULT_RESEND_COUNT)
				if ok {
					dealer.logger.Info("call event cancelled", registrationLogData, requestLogData)
				} else {
					dealer.logger.Error("call event dispatch error", registrationLogData, requestLogData)
				}
			} else {
				dealer.logger.Debug("call event timeout", registrationLogData, requestLogData)

				response := wamp.NewErrorEvent(callEvent, wamp.ErrorTimedOut)
				dealer.sendReply(caller, response)
			}
		case response := <-replyEventPromise:
			cancelCancelEventPromise()

			if response.Kind() == wamp.MK_YIELD {
				loopGenerator(dealer.routerID, caller, executor, callEvent, response, dealer.logger)
			} else {
				dealer.sendReply(caller, response)
			}
		}

		return nil
	}

	cancelCancelEventPromise()

	dealer.logger.Debug("procedure not found", requestLogData)
	response := wamp.NewErrorEvent(callEvent, wamp.ErrorProcedureNotFound)
	dealer.sendReply(caller, response)

	return nil
}

func (dealer *Dealer) onLeave(peer *wamp.Peer) {
	delete(dealer.peers, peer.ID)
	dealer.logger.Debug("dettach peer", "ID", peer.ID)
}

func (dealer *Dealer) onJoin(peer *wamp.Peer) {
	dealer.logger.Debug("attach peer", "ID", peer.ID)
	dealer.peers[peer.ID] = peer
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
