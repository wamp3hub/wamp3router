package wamp3router

import (
	"log"

	wamp "github.com/wamp3hub/wamp3go"

	"github.com/rs/xid"
)

type Broker struct {
	subscriptions *URIM[*wamp.SubscribeOptions]
	peers         map[string]*wamp.Peer
}

func NewBroker(storage Storage) *Broker {
	return &Broker{
		NewURIM[*wamp.SubscribeOptions](storage),
		make(map[string]*wamp.Peer),
	}
}

func (broker *Broker) onPublish(publisher *wamp.Peer, request wamp.PublishEvent) (e error) {
	route := request.Route()
	route.PublisherID = publisher.ID
	features := request.Features()
	log.Printf("[broker] publish (peer.ID=%s URI=%s)", publisher.ID, features.URI)

	// includeSet := NewSet(features.Include)
	excludeSet := NewSet(features.Exclude)
	subscriptionList := broker.subscriptions.Match(features.URI)
	for _, subscription := range subscriptionList {
		if excludeSet.Contains(subscription.AuthorID) {
			continue
		}

		subscriber, exist := broker.peers[subscription.AuthorID]
		if !exist {
			log.Printf(
				"[broker] subscriber not found (URI=%s publisher.ID=%s subscriber.ID=%s)",
				features.URI, publisher.ID, subscription.AuthorID,
			)
			continue
		}

		// TODO clone request
		route.EndpointID = subscription.ID
		route.SubscriberID = subscriber.ID

		e := subscriber.Send(request)
		if e == nil {
			log.Printf(
				"[broker] publication sent (URI=%s publisher.ID=%s subscriber.ID=%s subscription.ID=%s)",
				features.URI, publisher.ID, subscription.AuthorID, subscription.ID,
			)
		}
	}

	return nil
}

func (broker *Broker) onLeave(peer *wamp.Peer) {
	broker.subscriptions.ClearByAuthor(peer.ID)
	delete(broker.peers, peer.ID)
	log.Printf("[broker] dettach peer (ID=%s)", peer.ID)
}

func (broker *Broker) onJoin(peer *wamp.Peer) {
	log.Printf("[broker] attach peer (ID=%s)", peer.ID)
	broker.peers[peer.ID] = peer
	peer.IncomingPublishEvents.Consume(
		func(event wamp.PublishEvent) { broker.onPublish(peer, event) },
		func() { broker.onLeave(peer) },
	)
}

func (broker *Broker) Serve(newcomers *Newcomers) {
	log.Printf("[broker] up...")
	newcomers.Consume(
		broker.onJoin,
		func() { log.Printf("[broker] down...") },
	)
}

func (broker *Broker) Setup(
	session *wamp.Session,
	dealer *Dealer,
) {
	mount := func(
		uri string,
		options *wamp.SubscribeOptions,
		procedure wamp.PublishEndpoint,
	) {
		subscription := wamp.Subscription{xid.New().String(), uri, session.ID(), options}
		broker.subscriptions.Add(&subscription)
		session.Subscriptions[subscription.ID] = procedure
	}

	mount(
		"wamp.wamp.new",
		&wamp.SubscribeOptions{},
		func(request wamp.PublishEvent) {},
	)
}
