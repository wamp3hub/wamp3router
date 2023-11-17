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

func (dealer *Dealer) onYield(
	caller *wamp.Peer,
	executor *wamp.Peer,
	yieldEvent wamp.YieldEvent,
	stopEventPromise wampShared.Promise[wamp.CancelEvent],
) error {
	for yieldEvent.Kind() == wamp.MK_YIELD {
		nextEventPromise, cancelNextEventPromise := caller.PendingNextEvents.New(yieldEvent.ID(), wamp.DEFAULT_GENERATOR_LIFETIME)
		e := caller.Send(yieldEvent)
		if e != nil {
			log.Printf(
				"[dealer] generator yield error %s (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
				e, caller.ID, executor.ID, yieldEvent.ID(),
			)

			// TODO wamp.NewStopEvent()
			break
		}

		log.Printf(
			"[dealer] generator yield success (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
			caller.ID, executor.ID, yieldEvent.ID(),
		)

		select {
		case stopEvent := <-stopEventPromise:
			cancelNextEventPromise()

			e := executor.Send(stopEvent)
			if e == nil {
				log.Printf(
					"[dealer] generator stop success (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
					caller.ID, executor.ID, yieldEvent.ID(),
				)
			} else {
				log.Printf(
					"[dealer] generator stop error %s (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
					e, caller.ID, executor.ID, yieldEvent.ID(),
				)
			}

			break
		case nextEvent, done := <-nextEventPromise:
			if !done {
				log.Printf(
					"[dealer] generator lifetime expired (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
					caller.ID, executor.ID, yieldEvent.ID(),
				)

				// TODO wamp.NewStopEvent()
				break
			}

			log.Printf(
				"[dealer] generator next step (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
				caller.ID, executor.ID, nextEvent.ID(),
			)

			yieldEventPromise, _ := executor.PendingReplyEvents.New(nextEvent.ID(), wamp.DEFAULT_TIMEOUT)
			e := executor.Send(nextEvent)
			if e == nil {
				yieldEvent, done = <-yieldEventPromise
				if done {
					log.Printf(
						"[dealer] generator step success (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
						caller.ID, executor.ID, yieldEvent.ID(),
					)
					continue
				} else {
					e = errors.New("TimedOut")
				}
			} else {
				e = errors.New("InternalError")
			}

			yieldEvent = wamp.NewErrorEvent(nextEvent, e)
			break
		}
	}

	log.Printf(
		"[dealer] destroy generator (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
		caller.ID, executor.ID, yieldEvent.ID(),
	)

}

func (dealer *Dealer) onCall(
	caller *wamp.Peer,
	request wamp.CallEvent,
) error {
	features := request.Features()
	log.Printf("[dealer] call (URI=%s caller.ID=%s)", features.URI, caller.ID)

	cancelCallEventPromise, cancelCancelEventPromise := caller.PendingCancelEvents.New(
		request.ID(),
		features.Timeout*2,
	)

	route := request.Route()
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

		replyEventPromise, cancelReplyEventPromise := executor.PendingReplyEvents.New(request.ID(), features.Timeout)
		e := executor.Send(request)
		if e == nil {
			log.Printf(
				"[dealer] executor (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
				features.URI, caller.ID, registration.AuthorID, registration.ID,
			)
		} else {
			log.Printf(
				"[dealer] executor did not accept request (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
				features.URI, caller.ID, registration.AuthorID, registration.ID, e,
			)
			continue
		}

		select {
		case cancelEvent := <-cancelCallEventPromise:
			cancelReplyEventPromise()

			cancelFeatures := cancelEvent.Features()
			cancelFeatures.VisitedRouters = append(cancelFeatures.VisitedRouters, dealer.session.ID())

			e := executor.Send(cancelEvent)
			if e == nil {
				log.Printf(
					"[dealer] call event successfully cancelled (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
					features.URI, caller.ID, executor.ID, registration.ID,
				)
			} else {
				log.Printf(
					"[dealer] failed to cancel call event (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s error=%s)",
					features.URI, caller.ID, executor.ID, registration.ID, e,
				)
			}
		case response, done := <-replyEventPromise:
			if !done {
				log.Printf(
					"[dealer] executor did not respond (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
					features.URI, caller.ID, executor.ID, registration.ID,
				)
				response = wamp.NewErrorEvent(request, errors.New("TimedOut"))
			} else if response.Kind() == wamp.MK_YIELD {
				response = dealer.onYield(caller, executor, response, cancelCallEventPromise)
			}

			cancelCancelEventPromise()

			replyFeatures := response.Features()
			replyFeatures.VisitedRouters = append(replyFeatures.VisitedRouters, dealer.session.ID())

			e = caller.Send(response)
			if e == nil {
				log.Printf(
					"[dealer] invocation processed successfully (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
					features.URI, caller.ID, executor.ID, registration.ID,
				)
			} else {
				log.Printf(
					"[dealer] reply not delivered (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
					features.URI, caller.ID, executor.ID, registration.ID, e,
				)
			}
		}

		return nil
	}

	log.Printf("[dealer] procedure not found (URI=%s caller.ID=%s)", features.URI, caller.ID)
	response := wamp.NewErrorEvent(request, errors.New("ProcedureNotFound"))
	e := caller.Send(response)
	cancelCancelEventPromise()
	return e
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
