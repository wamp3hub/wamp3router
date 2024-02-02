package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

type SubscriptionList = routerShared.ResourceList[*wamp.SubscribeOptions]

type Broker struct {
	routerID      string
	peers         map[string]*wamp.Peer
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
		make(map[string]*wamp.Peer),
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

func (broker *Broker) onPublish(publisher *wamp.Peer, request wamp.PublishEvent) (e error) {
	route := request.Route()
	route.PublisherID = publisher.ID
	route.VisitedRouters = append(route.VisitedRouters, broker.routerID)

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
	broker.logger.Debug("publish", requestLogData)

	includeSet := routerShared.NewSet(features.Include)
	excludeSet := routerShared.NewSet(features.Exclude)

	subscriptionList := broker.matchSubscriptions(features.URI)

	for _, subscription := range subscriptionList {
		subscriptionLogData := slog.Group(
			"subscription",
			"ID", subscription.ID,
			"URI", subscription.URI,
			"SubscriberID", subscription.AuthorID,
		)

		if excludeSet.Contains(subscription.AuthorID) || (includeSet.Size() > 0 && !includeSet.Contains(subscription.AuthorID)) {
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

		ok := subscriber.Send(request, wamp.DEFAULT_RESEND_COUNT)
		if ok {
			broker.logger.Debug("publication sent", subscriptionLogData, requestLogData)
		} else {
			broker.logger.Error("publication dispatch error", subscriptionLogData, requestLogData)
		}
	}

	return nil
}

func (broker *Broker) onLeave(peer *wamp.Peer) {
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

func (broker *Broker) Serve(newcomers *wampShared.Observable[*wamp.Peer]) {
	broker.logger.Info("up...")
	newcomers.Observe(
		broker.onJoin,
		func() { broker.logger.Info("down...") },
	)
}
