package wamp3router

import (
	"log"

	"github.com/rs/xid"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type Broker struct {
	session       *wamp.Session
	subscriptions *routerShared.URIM[*wamp.SubscribeOptions]
	peers         map[string]*wamp.Peer
}

func NewBroker(
	session *wamp.Session,
	storage routerShared.Storage,
) *Broker {
	return &Broker{
		session,
		routerShared.NewURIM[*wamp.SubscribeOptions](storage),
		make(map[string]*wamp.Peer),
	}
}

func (broker *Broker) subscribe(
	uri string,
	authorID string,
	options *wamp.SubscribeOptions,
) (*wamp.Subscription, error) {
	isNew := broker.subscriptions.Count(uri) == 0
	subscription := wamp.Subscription{
		ID:       xid.New().String(),
		URI:      uri,
		AuthorID: authorID,
		Options:  options,
	}
	e := broker.subscriptions.Add(&subscription)
	if e == nil {
		if isNew {
			e = wamp.Publish(broker.session, &wamp.PublishFeatures{URI: "wamp.subscription.new"}, subscription)
			if e == nil {
				log.Printf("[broker] new subscription URI=%s", uri)
			}
		}
		return &subscription, nil
	}
	return nil, e
}

func (broker *Broker) unsubscribe(
	authorID string,
	subscriptionID string,
) {
	emptyBranches := broker.subscriptions.DeleteByAuthor(authorID, subscriptionID)
	for _, uri := range emptyBranches {
		e := wamp.Publish(broker.session, &wamp.PublishFeatures{URI: "wamp.subscription.idle"}, uri)
		if e == nil {
			log.Printf("[broker] idle subscription URI=%s", uri)
		}
	}
}

func (broker *Broker) onPublish(publisher *wamp.Peer, request wamp.PublishEvent) (e error) {
	route := request.Route()
	route.PublisherID = publisher.ID
	features := request.Features()
	log.Printf("[broker] publish (peer.ID=%s URI=%s)", publisher.ID, features.URI)

	// includeSet := NewSet(features.Include)
	excludeSet := routerShared.NewSet(features.Exclude)
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

		route.EndpointID = subscription.ID
		route.SubscriberID = subscriber.ID

		e := subscriber.Send(request)
		if e == nil {
			log.Printf(
				"[broker] publication sent (URI=%s publisher.ID=%s subscriber.ID=%s subscription.ID=%s)",
				features.URI, publisher.ID, subscription.AuthorID, subscription.ID,
			)
		} else {
			log.Printf(
				"[broker] subscriber did not accept (URI=%s publisher.ID=%s subscriber.ID=%s subscription.ID=%s)",
				features.URI, publisher.ID, subscription.AuthorID, subscription.ID,
			)
		}
	}

	return nil
}

func (broker *Broker) onLeave(peer *wamp.Peer) {
	broker.unsubscribe(peer.ID, "")
	delete(broker.peers, peer.ID)
	log.Printf("[broker] dettach peer (ID=%s)", peer.ID)
}

func (broker *Broker) onJoin(peer *wamp.Peer) {
	log.Printf("[broker] attach peer (ID=%s)", peer.ID)
	broker.peers[peer.ID] = peer
	peer.ConsumeIncomingPublishEvents(
		func(event wamp.PublishEvent) { broker.onPublish(peer, event) },
		func() { broker.onLeave(peer) },
	)
}

func (broker *Broker) Serve(consumeNewcomers wampShared.Consumable[*wamp.Peer]) {
	log.Printf("[broker] up...")
	consumeNewcomers(
		broker.onJoin,
		func() { log.Printf("[broker] down...") },
	)
}

func (broker *Broker) Setup(dealer *Dealer) {
	// mount := func(
	// 	uri string,
	// 	options *wamp.SubscribeOptions,
	// 	procedure wamp.PublishEndpoint,
	// ) {
	// 	subscription, _ := broker.subscribe(uri, broker.session.ID(), options)
	// 	broker.session.Subscriptions[subscription.ID] = procedure
	// }
}
