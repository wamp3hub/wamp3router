package router

import (
	"errors"
	"log"
	"sort"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type RegistrationList = routerShared.ResourceList[*wamp.RegisterOptions]

type Dealer struct {
	session       *wamp.Session
	registrations *routerShared.URIM[*wamp.RegisterOptions]
	peers         map[string]*wamp.Peer
	counter       map[string]int
}

func NewDealer(
	session *wamp.Session,
	storage routerShared.Storage,
) *Dealer {
	return &Dealer{
		session,
		routerShared.NewURIM[*wamp.RegisterOptions](storage),
		make(map[string]*wamp.Peer),
		make(map[string]int),
	}
}

func (dealer *Dealer) register(
	uri string,
	authorID string,
	options *wamp.RegisterOptions,
) (*wamp.Registration, error) {
	options.Route = append(options.Route, dealer.session.ID())
	registration := wamp.Registration{
		ID:       wampShared.NewID(),
		URI:      uri,
		AuthorID: authorID,
		Options:  options,
	}
	e := dealer.registrations.Add(&registration)
	if e == nil {
		e = wamp.Publish(
			dealer.session,
			&wamp.PublishFeatures{
				URI:     "wamp.registration.new",
				Exclude: []string{authorID},
			},
			registration,
		)
		if e == nil {
			log.Printf("[dealer] new registeration URI=%s", uri)
		}
		return &registration, nil
	}
	return nil, e
}

func (dealer *Dealer) unregister(
	authorID string,
	registrationID string,
) {
	removedRegistrationList := dealer.registrations.DeleteByAuthor(authorID, registrationID)
	for _, registration := range removedRegistrationList {
		e := wamp.Publish(
			dealer.session,
			&wamp.PublishFeatures{
				URI:     "wamp.registration.gone",
				Exclude: []string{authorID},
			},
			registration.URI,
		)
		if e == nil {
			log.Printf("[dealer] registration gone URI=%s", registration.URI)
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
	e := caller.Send(event)
	if e == nil {
		log.Printf("[dealer] invocation processed successfully (caller.ID=%s, invocation.ID=%s)", caller.ID, features.InvocationID)
	} else {
		log.Printf("[dealer] reply not delivered %s (caller.ID=%s, invocation.ID=%s)", e, caller.ID, features.InvocationID)
	}
}

func (dealer *Dealer) sendStop(
	executor *wamp.Peer,
	generatorID string,
) {
	event := wamp.NewStopEvent(generatorID)
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, dealer.session.ID())
	e := executor.Send(event)
	if e == nil {
		log.Printf(
			"[dealer] generator stop success (executor.ID=%s generator.ID=%s)",
			executor.ID, features.InvocationID,
		)
	} else {
		log.Printf(
			"[dealer] generator stop error %s (executor.ID=%s generator.ID=%s)",
			e, executor.ID, features.InvocationID,
		)
	}
}

func (dealer *Dealer) onNext(
	generatorID string,
	caller *wamp.Peer,
	executor *wamp.Peer,
	nextEvent wamp.NextEvent,
	stopEventPromise wampShared.Promise[wamp.StopEvent],
) error {
	nextFeatures := nextEvent.Features()

	yieldEventPromise, cancelYieldEventPromise := executor.PendingReplyEvents.New(
		nextEvent.ID(), nextFeatures.Timeout,
	)

	e := executor.Send(nextEvent)
	if e != nil {
		cancelYieldEventPromise()

		log.Printf(
			"[dealer] send next error %s (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
			e, caller.ID, executor.ID, nextEvent.ID(),
		)

		response := wamp.NewErrorEvent(nextEvent, wamp.InternalError)
		dealer.sendReply(caller, response)

		return e
	}

	log.Printf(
		"[dealer] send next success (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
		caller.ID, executor.ID, nextEvent.ID(),
	)

	select {
	case response, done := <-yieldEventPromise:
		if !done {
			log.Printf(
				"[dealer] yield error (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
				caller.ID, executor.ID, nextEvent.ID(),
			)

			response = wamp.NewErrorEvent(nextEvent, wamp.TimedOut)
		} else if response.Kind() == wamp.MK_YIELD {
			return dealer.onYield(
				generatorID, caller, executor, response, stopEventPromise,
			)
		}

		dealer.sendReply(caller, response)
	case <-stopEventPromise:
		cancelYieldEventPromise()

		dealer.sendStop(executor, generatorID)
	}

	return nil
}

func (dealer *Dealer) onYield(
	generatorID string,
	caller *wamp.Peer,
	executor *wamp.Peer,
	yieldEvent wamp.YieldEvent,
	stopEventPromise wampShared.Promise[wamp.StopEvent],
) error {
	nextEventPromise, cancelNextEventPromise := caller.PendingNextEvents.New(
		yieldEvent.ID(), wamp.DEFAULT_GENERATOR_LIFETIME,
	)

	e := caller.Send(yieldEvent)
	if e != nil {
		cancelNextEventPromise()

		log.Printf(
			"[dealer] send yield error %s (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
			e, caller.ID, executor.ID, yieldEvent.ID(),
		)

		dealer.sendStop(executor, generatorID)

		return e
	}

	log.Printf(
		"[dealer] send yield success (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
		caller.ID, executor.ID, yieldEvent.ID(),
	)

	select {
	case nextEvent := <-nextEventPromise:
		return dealer.onNext(
			generatorID, caller, executor, nextEvent, stopEventPromise,
		)
	case <-stopEventPromise:
		cancelNextEventPromise()

		dealer.sendStop(executor, generatorID)
	}

	return nil
}

func (dealer *Dealer) generator(
	caller *wamp.Peer,
	executor *wamp.Peer,
	callEvent wamp.CallEvent,
	yieldEvent wamp.YieldEvent,
) error {
	generator := new(wamp.NewGeneratorPayload)
	yieldEvent.Payload(generator)

	stopEventPromise, cancelStopEventPromise := executor.PendingCancelEvents.New(
		generator.ID, wamp.DEFAULT_GENERATOR_LIFETIME,
	)

	e := dealer.onYield(
		generator.ID, caller, executor, yieldEvent, stopEventPromise,
	)

	cancelStopEventPromise()

	log.Printf(
		"[dealer] destroy generator (caller.ID=%s executor.ID=%s)",
		caller.ID, executor.ID,
	)

	return e
}

func (dealer *Dealer) onCall(
	caller *wamp.Peer,
	callEvent wamp.CallEvent,
) error {
	features := callEvent.Features()
	log.Printf("[dealer] call (URI=%s caller.ID=%s)", features.URI, caller.ID)

	cancelCallEventPromise, cancelCancelEventPromise := caller.PendingCancelEvents.New(
		callEvent.ID(),
		features.Timeout,
	)

	route := callEvent.Route()
	route.CallerID = caller.ID
	route.VisitedRouters = append(route.VisitedRouters, dealer.session.ID())

	registrationList := dealer.matchRegistrations(features.URI)

	for _, registration := range registrationList {
		executor, exists := dealer.peers[registration.AuthorID]
		if !exists {
			log.Printf(
				"[dealer] invalid registartion (peer not found) (registration.ID=%s peer.ID=%s URI=%s caller.ID=%s)",
				features.URI, caller.ID, registration.ID, registration.AuthorID,
			)
			continue
		}

		route.EndpointID = registration.ID
		route.ExecutorID = executor.ID

		replyEventPromise, cancelReplyEventPromise := executor.PendingReplyEvents.New(
			callEvent.ID(), features.Timeout,
		)
		e := executor.Send(callEvent)
		if e != nil {
			log.Printf(
				"[dealer] executor did not accept invocation (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
				features.URI, caller.ID, registration.AuthorID, registration.ID, e,
			)
			continue
		}

		log.Printf(
			"[dealer] executor (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
			features.URI, caller.ID, registration.AuthorID, registration.ID,
		)

		select {
		case cancelEvent, done := <-cancelCallEventPromise:
			cancelReplyEventPromise()

			if done {
				cancelFeatures := cancelEvent.Features()
				cancelFeatures.VisitedRouters = append(cancelFeatures.VisitedRouters, dealer.session.ID())
				e := executor.Send(cancelEvent)
				if e == nil {
					log.Printf(
						"[dealer] call event successfully cancelled (executor.ID=%s invocation.ID=%s)",
						executor.ID, cancelFeatures.InvocationID,
					)
				} else {
					log.Printf(
						"[dealer] cancel error %s (executor.ID=%s invocation.ID=%s)",
						e, executor.ID, cancelFeatures.InvocationID,
					)
				}
			} else {
				log.Printf(
					"[dealer] executor did not respond (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
					features.URI, caller.ID, executor.ID, registration.ID,
				)

				response := wamp.NewErrorEvent(callEvent, wamp.TimedOut)
				dealer.sendReply(caller, response)
			}
		case response := <-replyEventPromise:
			cancelCancelEventPromise()

			if response.Kind() == wamp.MK_YIELD {
				dealer.generator(caller, executor, callEvent, response)
			} else {
				dealer.sendReply(caller, response)
			}
		}

		return nil
	}

	cancelCancelEventPromise()

	log.Printf("[dealer] procedure not found (URI=%s caller.ID=%s)", features.URI, caller.ID)
	response := wamp.NewErrorEvent(callEvent, errors.New("ProcedureNotFound"))
	dealer.sendReply(caller, response)

	return nil
}

func (dealer *Dealer) onLeave(peer *wamp.Peer) {
	dealer.unregister(peer.ID, "")
	delete(dealer.peers, peer.ID)
	log.Printf("[dealer] dettach peer (ID=%s)", peer.ID)
}

func (dealer *Dealer) onJoin(peer *wamp.Peer) {
	log.Printf("[dealer] attach peer (ID=%s)", peer.ID)
	dealer.peers[peer.ID] = peer
	peer.ConsumeIncomingCallEvents(
		func(event wamp.CallEvent) { dealer.onCall(peer, event) },
		func() { dealer.onLeave(peer) },
	)
}

func (dealer *Dealer) Serve(consumeNewcomers wampShared.Consumable[*wamp.Peer]) {
	log.Printf("[dealer] up...")
	consumeNewcomers(
		dealer.onJoin,
		func() { log.Printf("[dealer] down...") },
	)
}

func (dealer *Dealer) Setup(broker *Broker) {
	mount := func(
		uri string,
		options *wamp.RegisterOptions,
		procedure func(callEvent wamp.CallEvent) wamp.ReplyEvent,
	) {
		registration, _ := dealer.register(uri, dealer.session.ID(), options)
		dealer.session.Registrations[registration.ID] = procedure
	}

	mount(
		"wamp.router.register",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			payload := new(wamp.NewResourcePayload[wamp.RegisterOptions])
			e := callEvent.Payload(payload)
			if e == nil && len(payload.URI) > 0 {
				registration, e := dealer.register(payload.URI, route.CallerID, payload.Options)
				if e == nil {
					return wamp.NewReplyEvent(callEvent, *registration)
				}
			} else {
				e = errors.New("InvalidPayload")
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.router.subscribe",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			payload := new(wamp.NewResourcePayload[wamp.SubscribeOptions])
			e := callEvent.Payload(payload)
			if e == nil && len(payload.URI) > 0 {
				subscription, e := broker.subscribe(payload.URI, route.CallerID, payload.Options)
				if e == nil {
					return wamp.NewReplyEvent(callEvent, *subscription)
				}
			} else {
				e = errors.New("InvalidPayload")
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.router.unregister",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			registrationID := ""
			e := callEvent.Payload(&registrationID)
			if e == nil && len(registrationID) > 0 {
				dealer.unregister(route.CallerID, registrationID)
				return wamp.NewReplyEvent(callEvent, struct{}{})
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.router.unsubscribe",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			subscriptionID := ""
			e := callEvent.Payload(&subscriptionID)
			if e == nil && len(subscriptionID) > 0 {
				broker.unsubscribe(route.CallerID, subscriptionID)
				return wamp.NewReplyEvent(callEvent, struct{}{})
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.router.registration.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			source := wamp.Event(callEvent)
			URIList := dealer.registrations.DumpURIList()
			for _, uri := range URIList {
				registrationList := dealer.registrations.Match(uri)
				nextEvent, e := wamp.Yield(source, registrationList)
				if e == nil {
					source = nextEvent
				} else {
					break
				}
			}
			return wamp.NewReplyEvent(source, RegistrationList{})
		},
	)

	mount(
		"wamp.router.subscription.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			source := wamp.Event(callEvent)
			URIList := broker.subscriptions.DumpURIList()
			for _, uri := range URIList {
				subscriptionList := broker.subscriptions.Match(uri)
				nextEvent, e := wamp.Yield(source, subscriptionList)
				if e == nil {
					source = nextEvent
				} else {
					break
				}
			}
			return wamp.NewReplyEvent(source, SubscriptionList{})
		},
	)
}
