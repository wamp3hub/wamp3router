package router

import (
	"errors"
	"log/slog"
	"sort"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type RegistrationList = routerShared.ResourceList[*wamp.RegisterOptions]

type Dealer struct {
	session       *wamp.Session
	registrations *routerShared.URIM[*wamp.RegisterOptions]
	logger        *slog.Logger
	peers         map[string]*wamp.Peer
	counter       map[string]int
}

func NewDealer(
	session *wamp.Session,
	storage routerShared.Storage,
	logger *slog.Logger,
) *Dealer {
	return &Dealer{
		session,
		routerShared.NewURIM[*wamp.RegisterOptions](storage, logger),
		logger,
		make(map[string]*wamp.Peer),
		make(map[string]int),
	}
}

func (dealer *Dealer) register(
	uri string,
	authorID string,
	options *wamp.RegisterOptions,
) (*wamp.Registration, error) {
	logData := slog.Group(
		"registration",
		"URI", uri,
		"AuthorID", authorID,
	)

	options.Route = append(options.Route, dealer.session.ID())
	registration := wamp.Registration{
		ID:       wampShared.NewID(),
		URI:      uri,
		AuthorID: authorID,
		Options:  options,
	}
	e := dealer.registrations.Add(&registration)
	if e != nil {
		dealer.logger.Error("during add registration into URIM", logData)
		return nil, e
	}

	e = wamp.Publish(
		dealer.session,
		&wamp.PublishFeatures{
			URI:     "wamp.registration.new",
			Exclude: []string{authorID},
		},
		registration,
	)
	if e == nil {
		dealer.logger.Info("new registeration", logData)
	}
	return &registration, nil
}

func (dealer *Dealer) unregister(
	authorID string,
	registrationID string,
) {
	removedRegistrationList := dealer.registrations.DeleteByAuthor(authorID, registrationID)
	for _, registration := range removedRegistrationList {
		logData := slog.Group(
			"registration",
			"URI", registration.URI,
			"AuthorID", registration.AuthorID,
		)

		e := wamp.Publish(
			dealer.session,
			&wamp.PublishFeatures{
				URI:     "wamp.registration.gone",
				Exclude: []string{authorID},
			},
			registration.URI,
		)
		if e == nil {
			dealer.logger.Info("registration gone", logData)
		} else {
			dealer.logger.Error("during publish to topic 'wamp.registration.gone'", logData)
		}
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
	features.VisitedRouters = append(features.VisitedRouters, dealer.session.ID())

	logData := slog.Group(
		"response",
		"CallerID", caller.ID,
		"InvocationID", features.InvocationID,
		"VisitedRouters", features.VisitedRouters,
	)

	e := caller.Send(event)
	if e == nil {
		dealer.logger.Debug("invocation processed successfully", logData)
	} else {
		dealer.logger.Error("during send reply event", "error", e, logData)
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
	route.VisitedRouters = append(route.VisitedRouters, dealer.session.ID())

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
	dealer.logger.Info("call", requestLogData)

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
		e := executor.Send(callEvent)
		if e != nil {
			dealer.logger.Error("during send call event", "error", e, registrationLogData, requestLogData)
			continue
		}

		dealer.logger.Debug("reply event sent", registrationLogData, requestLogData)

		select {
		case cancelEvent, done := <-cancelCallEventPromise:
			cancelReplyEventPromise()

			if done {
				cancelFeatures := cancelEvent.Features()
				cancelFeatures.VisitedRouters = append(cancelFeatures.VisitedRouters, dealer.session.ID())
				e := executor.Send(cancelEvent)
				if e == nil {
					dealer.logger.Info("call event cancelled", registrationLogData, requestLogData)
				} else {
					dealer.logger.Error("during send cancel event", registrationLogData, requestLogData)
				}
			} else {
				dealer.logger.Debug("call event timeout", registrationLogData, requestLogData)

				response := wamp.NewErrorEvent(callEvent, wamp.ErrorTimedOut)
				dealer.sendReply(caller, response)
			}
		case response := <-replyEventPromise:
			cancelCancelEventPromise()

			if response.Kind() == wamp.MK_YIELD {
				loopGenerator(dealer, caller, executor, callEvent, response, dealer.logger)
			} else {
				dealer.sendReply(caller, response)
			}
		}

		return nil
	}

	cancelCancelEventPromise()

	dealer.logger.Debug("procedure not found", requestLogData)
	response := wamp.NewErrorEvent(callEvent, errors.New("ProcedureNotFound"))
	dealer.sendReply(caller, response)

	return nil
}

func (dealer *Dealer) onLeave(peer *wamp.Peer) {
	dealer.unregister(peer.ID, "")
	delete(dealer.peers, peer.ID)
	dealer.logger.Info("dettach peer", "ID", peer.ID)
}

func (dealer *Dealer) onJoin(peer *wamp.Peer) {
	dealer.logger.Info("attach peer", "ID", peer.ID)
	dealer.peers[peer.ID] = peer
	peer.IncomingCallEvents.Observe(
		func(event wamp.CallEvent) { dealer.onCall(peer, event) },
		func() { dealer.onLeave(peer) },
	)
}

func (dealer *Dealer) Serve(newcomers *wampShared.ObservableObject[*wamp.Peer]) {
	dealer.logger.Info("up...")
	newcomers.Observe(
		dealer.onJoin,
		func() { dealer.logger.Info("down...") },
	)
}

func (dealer *Dealer) Setup(broker *Broker) {
	mount := func(
		uri string,
		options *wamp.RegisterOptions,
		procedure wamp.CallProcedure,
	) {
		registration, _ := dealer.register(uri, dealer.session.ID(), options)
		endpoint := wamp.NewCallEventEndpoint(procedure, broker.logger)
		dealer.session.Registrations[registration.ID] = endpoint
	}

	mount(
		"wamp.router.register",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) any {
			route := callEvent.Route()
			payload := new(wamp.NewResourcePayload[wamp.RegisterOptions])
			e := callEvent.Payload(payload)
			if e == nil && len(payload.URI) > 0 {
				registration, e := dealer.register(payload.URI, route.CallerID, payload.Options)
				if e == nil {
					return registration
				}
			} else {
				e = errors.New("InvalidPayload")
			}
			return e
		},
	)

	mount(
		"wamp.router.subscribe",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) any {
			route := callEvent.Route()
			payload := new(wamp.NewResourcePayload[wamp.SubscribeOptions])
			e := callEvent.Payload(payload)
			if e == nil && len(payload.URI) > 0 {
				subscription, e := broker.subscribe(payload.URI, route.CallerID, payload.Options)
				if e == nil {
					return subscription
				}
			} else {
				e = errors.New("InvalidPayload")
			}
			return e
		},
	)

	mount(
		"wamp.router.unregister",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) any {
			route := callEvent.Route()
			registrationID := ""
			e := callEvent.Payload(&registrationID)
			if e == nil && len(registrationID) > 0 {
				dealer.unregister(route.CallerID, registrationID)
				return nil
			}
			return e
		},
	)

	mount(
		"wamp.router.unsubscribe",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) any {
			route := callEvent.Route()
			subscriptionID := ""
			e := callEvent.Payload(&subscriptionID)
			if e == nil && len(subscriptionID) > 0 {
				broker.unsubscribe(route.CallerID, subscriptionID)
				return nil
			}
			return e
		},
	)

	mount(
		"wamp.router.registration.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) any {
			source := wamp.Event(callEvent)
			URIList := dealer.registrations.DumpURIList()
			for _, uri := range URIList {
				registrationList := dealer.registrations.Match(uri)
				nextEvent := wamp.Yield(source, registrationList)
				source = nextEvent
			}
			return wamp.ExitGenerator
		},
	)

	mount(
		"wamp.router.subscription.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) any {
			source := wamp.Event(callEvent)
			URIList := broker.subscriptions.DumpURIList()
			for _, uri := range URIList {
				subscriptionList := broker.subscriptions.Match(uri)
				nextEvent := wamp.Yield(source, subscriptionList)
				source = nextEvent
			}
			return wamp.ExitGenerator
		},
	)
}
