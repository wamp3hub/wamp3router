package router

import (
	"log"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/shared"
)

type SubscriptionList = routerShared.ResourceList[*wamp.SubscribeOptions]

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
	options.Route = append(options.Route, broker.session.ID())
	subscription := wamp.Subscription{
		ID:       wampShared.NewID(),
		URI:      uri,
		AuthorID: authorID,
		Options:  options,
	}
	e := broker.subscriptions.Add(&subscription)
	if e == nil {
		e = wamp.Publish(
			broker.session,
			&wamp.PublishFeatures{
				URI:     "wamp.subscription.new",
				Exclude: []string{authorID},
			},
			subscription,
		)
		if e == nil {
			log.Printf("[broker] new subscription URI=%s", uri)
		}
		return &subscription, nil
	}
	return nil, e
}

func (broker *Broker) unsubscribe(
	authorID string,
	subscriptionID string,
) {
	removedSubscriptionList := broker.subscriptions.DeleteByAuthor(authorID, subscriptionID)
	for _, subscription := range removedSubscriptionList {
		e := wamp.Publish(
			broker.session,
			&wamp.PublishFeatures{
				URI:     "wamp.subscription.gone",
				Exclude: []string{authorID},
			},
			subscription.URI,
		)
		if e == nil {
			log.Printf("[broker] subscription gone URI=%s", subscription.URI)
		}
	}
}

func (broker *Broker) matchSubscriptions(
	uri string,
) SubscriptionList {
	subscriptionList := broker.subscriptions.Match(uri)
	return subscriptionList
}

func (broker *Broker) onPublish(publisher *wamp.Peer, request wamp.PublishEvent) (e error) {
	route := request.Route()
	route.PublisherID = publisher.ID
	route.VisitedRouters = append(route.VisitedRouters, broker.session.ID())

	features := request.Features()
	log.Printf("[broker] publish (peer.ID=%s URI=%s)", publisher.ID, features.URI)

	// includeSet := NewSet(features.Include)
	excludeSet := routerShared.NewSet(features.Exclude)

	subscriptionList := broker.matchSubscriptions(features.URI)

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
