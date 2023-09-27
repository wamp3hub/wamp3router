package wamp3router

import (
	"errors"
	"log"

	client "github.com/wamp3hub/wamp3go"

	"github.com/google/uuid"
)

type Broker struct {
	subscriptions *URIM[*client.SubscribeOptions]
	peers         map[string]*client.Peer
}

func newBroker(storage Storage) *Broker {
	return &Broker{
		NewURIM[*client.SubscribeOptions](storage),
		make(map[string]*client.Peer),
	}
}

func (broker *Broker) onPublish(publisher *client.Peer, request client.PublishEvent) (e error) {
	route := request.Route()
	route.PublisherID = publisher.ID
	features := request.Features()
	log.Printf("publish (peer.ID=%s URI=%s)", publisher.ID, features.URI)

	// Acknowledgment
	response := client.NewAcceptEvent(request.ID())
	e = publisher.Transport.Send(response)
	if e == nil {
		log.Printf("publish acknowledgment sent (peer.ID=%s URI=%s)", publisher.ID, features.URI)
	} else {
		log.Printf("publish acknowledgment not sent (peer.ID=%s URI=%s) %s", publisher.ID, features.URI, e)
	}

	// includeSet := NewSet(features.Include)
	excludeSet := NewSet(features.Exclude)
	subscriptionList := broker.subscriptions.Match(features.URI)
	for _, subscription := range subscriptionList {
		if excludeSet.Contains(subscription.AuthorID) {
			continue
		}

		// TODO clone request
		route.EndpointID = subscription.ID
		subscriber, exist := broker.peers[subscription.AuthorID]
		if exist {
			route.SubscriberID = subscriber.ID
			e = subscriber.Transport.Send(request)
			if e == nil {
				log.Printf(
					"publication sent (URI=%s publisher.ID=%s subscriber.ID=%s subscription.ID=%s) %s",
					features.URI, publisher.ID, subscription.AuthorID, subscription.ID, e,
				)
				// TODO catch accept event
			}
		} else {
			e = errors.New("SubscriberNotFound")
		}
		log.Printf(
			"publication not sent (URI=%s publisher.ID=%s subscriber.ID=%s) %s",
			features.URI, publisher.ID, subscription.AuthorID, e,
		)
	}

	return nil
}

func (broker *Broker) onLeave(peer *client.Peer) {
	broker.subscriptions.ClearByAuthor(peer.ID)
	delete(broker.peers, peer.ID)
	log.Printf("[broker] dettach peer (ID=%s)", peer.ID)
}

func (broker *Broker) onJoin(peer *client.Peer) {
	log.Printf("[broker] attach peer (ID=%s)", peer.ID)
	broker.peers[peer.ID] = peer
	peer.IncomingPublishEvents.Consume(
		func(event client.PublishEvent) { broker.onPublish(peer, event) },
		func() { broker.onLeave(peer) },
	)

}

func (broker *Broker) serve(newcomers *Newcomers) {
	log.Printf("[broker] up...")
	newcomers.Consume(
		broker.onJoin,
		func() { log.Printf("[broker] down...") },
	)
}

func (broker *Broker) setup(
	session *client.Session,
	dealer *Dealer,
) {
	mount := func(
		uri string,
		options *client.SubscribeOptions,
		procedure func(request client.PublishEvent),
	) {
		subscription := client.Subscription{uuid.NewString(), uri, session.ID(), options}
		broker.subscriptions.Add(&subscription)
		session.Subscriptions[subscription.ID] = procedure
	}

	mount(
		"wamp.client.new",
		&client.SubscribeOptions{},
		func(request client.PublishEvent) {},
	)
}
