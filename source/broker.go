package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

type SubscriptionList = routerShared.ResourceList[*wamp.SubscribeOptions]

type Broker struct {
	subscriptions *routerShared.URIM[*wamp.SubscribeOptions]
	session       *wamp.Session
	logger        *slog.Logger
	peers         map[string]*wamp.Peer
}

func NewBroker(
	session *wamp.Session,
	storage routerShared.Storage,
	logger *slog.Logger,
) *Broker {
	return &Broker{
		routerShared.NewURIM[*wamp.SubscribeOptions](storage, logger),
		session,
		logger.With("name", "Broker"),
		make(map[string]*wamp.Peer),
	}
}

func (broker *Broker) subscribe(
	uri string,
	authorID string,
	options *wamp.SubscribeOptions,
) (*wamp.Subscription, error) {
	logData := slog.Group(
		"subscription",
		"URI", uri,
		"AuthorID", authorID,
	)

	options.Route = append(options.Route, broker.session.ID())
	subscription := wamp.Subscription{
		ID:       wampShared.NewID(),
		URI:      uri,
		AuthorID: authorID,
		Options:  options,
	}
	e := broker.subscriptions.Add(&subscription)
	if e != nil {
		broker.logger.Error("during add subscription into URIM", "error", e, logData)
		return nil, e
	}

	e = wamp.Publish(
		broker.session,
		&wamp.PublishFeatures{
			URI:     "wamp.subscription.new",
			Exclude: []string{authorID},
		},
		subscription,
	)
	if e == nil {
		broker.logger.Info("new subscription", logData)
	}
	return &subscription, nil
}

func (broker *Broker) unsubscribe(
	authorID string,
	subscriptionID string,
) {
	removedSubscriptionList := broker.subscriptions.DeleteByAuthor(authorID, subscriptionID)
	for _, subscription := range removedSubscriptionList {
		logData := slog.Group(
			"subscription",
			"URI", subscription.URI,
			"AuthorID", subscription.AuthorID,
		)

		e := wamp.Publish(
			broker.session,
			&wamp.PublishFeatures{
				URI:     "wamp.subscription.gone",
				Exclude: []string{authorID},
			},
			subscription.URI,
		)
		if e == nil {
			broker.logger.Info("subscription gone", logData)
		} else {
			broker.logger.Info("during publish to 'wamp.subscription.gone'", logData)
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

	requestLogData := slog.Group(
		"event",
		"ID", request.ID,
		"URI", features.URI,
		"Include", features.Include,
		"Exclude", features.Exclude,
		"PublisherID", publisher.ID,
		"VisitedRouters", route.VisitedRouters,
	)
	broker.logger.Info("publish", requestLogData)

	// includeSet := NewSet(features.Include)
	excludeSet := routerShared.NewSet(features.Exclude)

	subscriptionList := broker.matchSubscriptions(features.URI)

	for _, subscription := range subscriptionList {
		subscriptionLogData := slog.Group(
			"subscription",
			"ID", subscription.ID,
			"URI", subscription.URI,
			"SubscriberID", subscription.AuthorID,
		)

		if excludeSet.Contains(subscription.AuthorID) {
			broker.logger.Debug("exclude subscriber", subscriptionLogData, requestLogData)
			continue
		}

		subscriber, exist := broker.peers[subscription.AuthorID]
		if !exist {
			broker.logger.Error("invalid subscription (peer not found)", subscriptionLogData, requestLogData)
			continue
		}

		route.EndpointID = subscription.ID
		route.SubscriberID = subscriber.ID

		e := subscriber.Send(request)
		if e == nil {
			broker.logger.Debug("publication sent", subscriptionLogData, requestLogData)
		} else {
			broker.logger.Error("during publication send", "error", e, subscriptionLogData, requestLogData)
		}
	}

	return nil
}

func (broker *Broker) onLeave(peer *wamp.Peer) {
	broker.unsubscribe(peer.ID, "")
	delete(broker.peers, peer.ID)
	broker.logger.Info("dettach peer", "ID", peer.ID)
}

func (broker *Broker) onJoin(peer *wamp.Peer) {
	broker.logger.Info("attach peer", "ID", peer.ID)
	broker.peers[peer.ID] = peer
	peer.IncomingPublishEvents.Observe(
		func(event wamp.PublishEvent) { broker.onPublish(peer, event) },
		func() { broker.onLeave(peer) },
	)
}

func (broker *Broker) Serve(newcomers *wampShared.ObservableObject[*wamp.Peer]) {
	broker.logger.Info("up...")
	newcomers.Observe(
		broker.onJoin,
		func() { broker.logger.Info("down...") },
	)
}
