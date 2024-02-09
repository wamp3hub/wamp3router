package router

import (
	"log/slog"

	cmap "github.com/orcaman/concurrent-map/v2"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

type SubscriptionList = routerShared.ResourceList[*wamp.SubscribeOptions]

type Broker struct {
	routerID      string
	peers         cmap.ConcurrentMap[string, *wamp.Peer]
	subscriptions *routerShared.URIM[*wamp.SubscribeOptions]
	logger        *slog.Logger
}

func NewBroker(
	routerID string,
	storage routerShared.Storage,
	logger *slog.Logger,
) *Broker {
	return &Broker{
		routerID,
		cmap.New[*wamp.Peer](),
		routerShared.NewURIM[*wamp.SubscribeOptions](storage, logger),
		logger.With("name", "Broker"),
	}
}

func (broker *Broker) matchSubscriptions(
	uri string,
) SubscriptionList {
	subscriptionList := broker.subscriptions.Match(uri)
	return subscriptionList
}

func (broker *Broker) onPublish(publisher *wamp.Peer, request wamp.PublishEvent) {
	route := request.Route()
	route.PublisherID = publisher.Details.ID
	route.VisitedRouters = append(route.VisitedRouters, broker.routerID)

	publishFeatures := request.Features()

	requestLogData := slog.Group(
		"event",
		"ID", request.ID,
		"URI", publishFeatures.URI,
		"PublisherID", route.PublisherID,
		"VisitedRouters", route.VisitedRouters,
		"IncludeSubscribers", publishFeatures.IncludeSubscribers,
		"ExcludeSubscribers", publishFeatures.ExcludeSubscribers,
		"IncludeRoles", publishFeatures.IncludeRoles,
		"ExcludeRoles", publishFeatures.ExcludeRoles,
	)
	broker.logger.Debug("publish", requestLogData)

	subscriptionList := broker.matchSubscriptions(publishFeatures.URI)

	for _, subscription := range subscriptionList {
		subscriptionLogData := slog.Group(
			"subscription",
			"ID", subscription.ID,
			"URI", subscription.URI,
			"SubscriberID", subscription.AuthorID,
		)

		subscriber, found := broker.peers.Get(subscription.AuthorID)
		if !found {
			broker.logger.Error("subscriber not found (invalid subscription)", subscriptionLogData, requestLogData)
			continue
		}

		if !publishFeatures.Authorized(subscriber.Details.ID, subscriber.Details.Role) ||
			!subscription.Options.Authorized(publisher.Details.Role) {
			broker.logger.Debug("exclude subscriber", subscriptionLogData, requestLogData)
			continue
		}

		route.EndpointID = subscription.ID
		route.SubscriberID = subscriber.Details.ID

		ok := subscriber.Send(request, wamp.DEFAULT_RESEND_COUNT)
		if ok {
			broker.logger.Debug("publication sent", subscriptionLogData, requestLogData)
		} else {
			broker.logger.Error("publication dispatch error", subscriptionLogData, requestLogData)
		}
	}
}

func (broker *Broker) onLeave(peer *wamp.Peer) {
	broker.peers.Remove(peer.Details.ID)
	broker.logger.Debug("dettach peer", "ID", peer.Details.ID)
}

func (broker *Broker) onJoin(peer *wamp.Peer) {
	broker.logger.Debug("attach peer", "ID", peer.Details.ID)
	broker.peers.Set(peer.Details.ID, peer)
	peer.IncomingPublishEvents.Observe(
		func(event wamp.PublishEvent) { broker.onPublish(peer, event) },
		func() { broker.onLeave(peer) },
	)
}

func (broker *Broker) Serve(newcomers *wampShared.Observable[*wamp.Peer]) {
	broker.logger.Debug("up...")
	newcomers.Observe(
		broker.onJoin,
		func() { broker.logger.Debug("down...") },
	)
}
