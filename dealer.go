package wamp3router

import (
	"errors"
	"log"

	wamp "github.com/wamp3hub/wamp3go"

	"github.com/rs/xid"
)

type Dealer struct {
	registrations *URIM[*wamp.RegisterOptions]
	peers         map[string]*wamp.Peer
	counter       map[string]uint64
}

func NewDealer(storage Storage) *Dealer {
	return &Dealer{
		NewURIM[*wamp.RegisterOptions](storage),
		make(map[string]*wamp.Peer),
		make(map[string]uint64),
	}
}

func (dealer *Dealer) onYield(
	caller *wamp.Peer,
	executor *wamp.Peer,
	yieldEvent wamp.ReplyEvent,
) (e error) {
	for yieldEvent.Kind() == wamp.MK_YIELD {
		nextEventPromise := caller.PendingNextEvents.New(yieldEvent.ID(), wamp.DEFAULT_GENERATOR_LIFETIME)
		e = caller.Send(yieldEvent)
		if e == nil {
			log.Printf(
				"[dealer] generator yield (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
				caller.ID, executor.ID, yieldEvent.ID(),
			)
		}

		nextEvent, done := <-nextEventPromise
		if done {
			log.Printf(
				"[dealer] generator next step (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
				caller.ID, executor.ID, nextEvent.ID(),
			)

			yieldEventPromise := executor.PendingReplyEvents.New(nextEvent.ID(), wamp.DEFAULT_TIMEOUT)
			e := executor.Send(nextEvent)
			if e == nil {
				yieldEvent, done = <-yieldEventPromise
				if done {
					log.Printf(
						"[dealer] generator respond (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
						caller.ID, executor.ID, yieldEvent.ID(),
					)
				}
			}
		}
	}

	e = caller.Send(yieldEvent)
	if e == nil {
		log.Printf(
			"[dealer] generator done (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
			caller.ID, executor.ID, yieldEvent.ID(),
		)
	}

	log.Printf(
		"[dealer] destroy generator (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
		caller.ID, executor.ID, yieldEvent.ID(),
	)

	return e
}

func shift[T any](items []T, x int) []T {
	return append(items[x:], items[:x]...)
}

func (dealer *Dealer) onCall(caller *wamp.Peer, request wamp.CallEvent) (e error) {
	route := request.Route()
	route.CallerID = caller.ID

	features := request.Features()
	log.Printf("[dealer] call (URI=%s caller.ID=%s)", features.URI, caller.ID)

	registrationList := dealer.registrations.Match(features.URI)

	n := len(registrationList)
	if n > 0 {
		offset := int(dealer.counter[features.URI]) % n
		registrationList = shift(registrationList, offset)
		dealer.counter[features.URI] += 1
	}

	for _, registration := range registrationList {
		executor, exists := dealer.peers[registration.AuthorID]
		if !exists {
			log.Printf(
				"[dealer] peer not found (URI=%s caller.ID=%s registration.ID=%s peer.ID=%s)",
				features.URI, caller.ID, registration.ID, registration.AuthorID,
			)
			continue
		}

		route.EndpointID = registration.ID
		route.ExecutorID = executor.ID

		replyEventPromise := executor.PendingReplyEvents.New(request.ID(), wamp.DEFAULT_TIMEOUT)
		e = executor.Send(request)
		if e == nil {
			log.Printf(
				"[dealer] executor (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
				features.URI, caller.ID, registration.AuthorID, registration.ID,
			)
		} else {
			log.Printf(
				"[dealer] executor not accepted request (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
				features.URI, caller.ID, registration.AuthorID, registration.ID, e,
			)
			continue
		}

		response, done := <-replyEventPromise
		if !done {
			log.Printf(
				"[dealer] executor not respond (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
				features.URI, caller.ID, executor.ID, registration.ID, e,
			)
			response = wamp.NewErrorEvent(request, e)
		} else if response.Kind() == wamp.MK_YIELD {
			return dealer.onYield(caller, executor, response)
		}

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
		return nil
	}

	log.Printf("[dealer] procedure not found (URI=%s caller.ID=%s)", features.URI, caller.ID)
	response := wamp.NewErrorEvent(request, errors.New("ProcedureNotFound"))
	e = caller.Send(response)
	return e
}

func (dealer *Dealer) onLeave(peer *wamp.Peer) {
	dealer.registrations.ClearByAuthor(peer.ID)
	delete(dealer.peers, peer.ID)
	log.Printf("[dealer] dettach peer (ID=%s)", peer.ID)
}

func (dealer *Dealer) onJoin(peer *wamp.Peer) {
	log.Printf("[dealer] attach peer (ID=%s)", peer.ID)
	dealer.peers[peer.ID] = peer
	peer.IncomingCallEvents.Consume(
		func(event wamp.CallEvent) { dealer.onCall(peer, event) },
		func() { dealer.onLeave(peer) },
	)
}

func (dealer *Dealer) Serve(newcomers *Newcomers) {
	log.Printf("[dealer] up...")
	newcomers.Consume(
		dealer.onJoin,
		func() { log.Printf("[dealer] down...") },
	)
}

func (dealer *Dealer) Setup(
	session *wamp.Session,
	broker *Broker,
) {
	mount := func(
		uri string,
		options *wamp.RegisterOptions,
		procedure func(callEvent wamp.CallEvent) wamp.ReplyEvent,
	) {
		registration := wamp.Registration{xid.New().String(), uri, session.ID(), options}
		dealer.registrations.Add(&registration)
		session.Registrations[registration.ID] = procedure
	}

	mount(
		"wamp.register",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			payload := new(wamp.NewResourcePayload[wamp.RegisterOptions])
			e := callEvent.Payload(payload)
			if e == nil {
				registration := wamp.Registration{xid.New().String(), payload.URI, route.CallerID, payload.Options}
				e = dealer.registrations.Add(&registration)
				if e == nil {
					return wamp.NewReplyEvent(callEvent, registration)
				}
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.subscribe",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			payload := new(wamp.NewResourcePayload[wamp.SubscribeOptions])
			e := callEvent.Payload(payload)
			if e == nil {
				subscription := wamp.Subscription{xid.New().String(), payload.URI, route.CallerID, payload.Options}
				e = broker.subscriptions.Add(&subscription)
				if e == nil {
					return wamp.NewReplyEvent(callEvent, subscription)
				}
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.unregister",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			payload := new(wamp.DeleteResourcePayload)
			e := callEvent.Payload(payload)
			if e == nil {
				e = dealer.registrations.DeleteByAuthor(route.CallerID, payload.ID)
				if e == nil {
					return wamp.NewReplyEvent(callEvent, true)
				}
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.unsubscribe",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			route := callEvent.Route()
			payload := new(wamp.DeleteResourcePayload)
			e := callEvent.Payload(payload)
			if e == nil {
				e = broker.subscriptions.DeleteByAuthor(route.CallerID, payload.ID)
				if e == nil {
					return wamp.NewReplyEvent(callEvent, true)
				}
			}
			return wamp.NewErrorEvent(callEvent, e)
		},
	)

	mount(
		"wamp.registration.uri.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			URIList := dealer.registrations.DumpURIList()
			return wamp.NewReplyEvent(callEvent, URIList)
		},
	)

	mount(
		"wamp.subscription.uri.list",
		&wamp.RegisterOptions{},
		func(callEvent wamp.CallEvent) wamp.ReplyEvent {
			URIList := broker.subscriptions.DumpURIList()
			return wamp.NewReplyEvent(callEvent, URIList)
		},
	)
}
