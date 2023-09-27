package wamp3router

import (
	"errors"
	"log"

	client "wamp3go"

	"github.com/google/uuid"
)

type Dealer struct {
	registrations *URIM[*client.RegisterOptions]
	peers         map[string]*client.Peer
}

func newDealer(storage Storage) *Dealer {
	return &Dealer{
		NewURIM[*client.RegisterOptions](storage),
		make(map[string]*client.Peer),
	}
}

func (dealer *Dealer) onCall(caller *client.Peer, request client.CallEvent) (e error) {
	route := request.Route()
	route.CallerID = caller.ID
	features := request.Features()
	log.Printf("call (URI=%s caller.ID=%s)", features.URI, caller.ID)

	// TODO select best registration
	registrationList := dealer.registrations.Match(features.URI)
	for _, registration := range registrationList {
		executor, exist := dealer.peers[registration.AuthorID]
		if exist {
			route.EndpointID = registration.ID
			route.ExecutorID = executor.ID
			e = executor.Transport.Send(request)
			if e == nil {
				response, e := executor.PendingReplyEvents.Catch(request.ID(), client.DEFAULT_TIMEOUT)
				if e != nil {
					log.Printf(
						"not respond (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
						features.URI, caller.ID, executor.ID, registration.ID, e,
					)
					response = client.NewErrorEvent(request.ID(), e)
				}
				e = caller.Transport.Send(response)
				if e == nil {
					log.Printf(
						"invocation processed successfully (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s)",
						features.URI, caller.ID, executor.ID, registration.ID,
					)
				} else {
					log.Printf(
						"response not delivered (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
						features.URI, caller.ID, executor.ID, registration.ID, e,
					)
				}
				return e
			}
		} else {
			e = errors.New("PeerNotFound")
		}
		log.Printf(
			"during invocation (URI=%s caller.ID=%s executor.ID=%s registration.ID=%s) %s",
			features.URI, caller.ID, registration.AuthorID, registration.ID, e,
		)
	}

	log.Printf("procedure not found (URI=%s caller.ID=%s)", features.URI, caller.ID)
	response := client.NewErrorEvent(request.ID(), errors.New("ProcedureNotFound"))
	e = caller.Transport.Send(response)
	return e
}

func (dealer *Dealer) onLeave(peer *client.Peer) {
	dealer.registrations.ClearByAuthor(peer.ID)
	delete(dealer.peers, peer.ID)
	log.Printf("[dealer] dettach peer (ID=%s)", peer.ID)
}

func (dealer *Dealer) onJoin(peer *client.Peer) {
	log.Printf("[dealer] attach peer (ID=%s)", peer.ID)
	dealer.peers[peer.ID] = peer
	peer.IncomingCallEvents.Consume(
		func(event client.CallEvent) { dealer.onCall(peer, event) },
		func() { dealer.onLeave(peer) },
	)
}

func (dealer *Dealer) serve(newcomers *Newcomers) {
	log.Printf("[dealer] up...")
	newcomers.Consume(
		dealer.onJoin,
		func() { log.Printf("[dealer] down...") },
	)
}

func (dealer *Dealer) setup(
	session *client.Session,
	broker *Broker,
) {
	mount := func(
		uri string,
		options *client.RegisterOptions,
		procedure func(request client.CallEvent) client.ReplyEvent,
	) {
		registration := client.Registration{uuid.NewString(), uri, session.ID(), options}
		dealer.registrations.Add(&registration)
		session.Registrations[registration.ID] = procedure
	}

	mount(
		"wamp.register",
		&client.RegisterOptions{},
		func(request client.CallEvent) client.ReplyEvent {
			route := request.Route()
			payload := new(client.NewResourcePayload[client.RegisterOptions])
			e := request.Payload(payload)
			if e == nil {
				registration := client.Registration{uuid.NewString(), payload.URI, route.CallerID, payload.Options}
				e = dealer.registrations.Add(&registration)
				if e == nil {
					return client.NewReplyEvent(request.ID(), registration)
				}
			}
			return client.NewErrorEvent(request.ID(), e)
		},
	)

	mount(
		"wamp.unregister",
		&client.RegisterOptions{},
		func(request client.CallEvent) client.ReplyEvent {
			route := request.Route()
			payload := new(client.DeleteResourcePayload)
			e := request.Payload(payload)
			if e == nil {
				e = dealer.registrations.DeleteByAuthor(route.CallerID, payload.ID)
				if e == nil {
					return client.NewReplyEvent(request.ID(), true)
				}
			}
			return client.NewErrorEvent(request.ID(), e)
		},
	)

	mount(
		"wamp.subscribe",
		&client.RegisterOptions{},
		func(request client.CallEvent) client.ReplyEvent {
			route := request.Route()
			payload := new(client.NewResourcePayload[client.SubscribeOptions])
			e := request.Payload(payload)
			if e == nil {
				subscription := client.Subscription{uuid.NewString(), payload.URI, route.CallerID, payload.Options}
				e = broker.subscriptions.Add(&subscription)
				if e == nil {
					return client.NewReplyEvent(request.ID(), subscription)
				}
			}
			return client.NewErrorEvent(request.ID(), e)
		},
	)

	mount(
		"wamp.unsubscribe",
		&client.RegisterOptions{},
		func(request client.CallEvent) client.ReplyEvent {
			route := request.Route()
			payload := new(client.DeleteResourcePayload)
			e := request.Payload(payload)
			if e == nil {
				e = broker.subscriptions.DeleteByAuthor(route.CallerID, payload.ID)
				if e == nil {
					return client.NewReplyEvent(request.ID(), true)
				}
			}
			return client.NewErrorEvent(request.ID(), e)
		},
	)

	mount(
		"wamp.join",
		&client.RegisterOptions{},
		func(request client.CallEvent) client.ReplyEvent {
			payload := new(client.JoinPayload)
			e := request.Payload(payload)
			if e == nil {
				replyPayload := client.SuccessJoinPayload{uuid.NewString(), "test"}
				return client.NewReplyEvent(request.ID(), replyPayload)
			}
			return client.NewErrorEvent(request.ID(), e)
		},
	)
}
