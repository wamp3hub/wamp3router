package service

import (
	"errors"

	wamp "github.com/wamp3hub/wamp3go"
	wampSerializer "github.com/wamp3hub/wamp3go/serializer"
	wampTransport "github.com/wamp3hub/wamp3go/transport"
)

type FriendMap map[string]*wamp.Session

type EventDistributor struct {
	ticket  string
	friends FriendMap
}

func NewEventDistributor(ticket string) *EventDistributor {
	return &EventDistributor{ticket, FriendMap{}}
}

func (distributor *EventDistributor) FriendsCount() int {
	return len(distributor.friends)
}

func (distributor *EventDistributor) Subscribe(
	uri string,
	features *wamp.SubscribeOptions,
	endpoint wamp.PublishEndpoint,
) {
	for _, friend := range distributor.friends {
		go friend.Subscribe(uri, features, endpoint)
	}
}

func (distributor *EventDistributor) Unsubscribe(subscriptionID string) {
	for _, friend := range distributor.friends {
		go friend.Unsubscribe(subscriptionID)
	}
}

func (distributor *EventDistributor) Register(
	uri string,
	features *wamp.RegisterOptions,
	endpoint wamp.CallEndpoint,
) {
	for _, friend := range distributor.friends {
		go friend.Register(uri, features, endpoint)
	}
}

func (distributor *EventDistributor) Unregister(registrationID string) {
	for _, friend := range distributor.friends {
		go friend.Unregister(registrationID)
	}
}

func (distributor *EventDistributor) Publish(event wamp.PublishEvent) {
	for _, friend := range distributor.friends {
		go friend.Publish(event)
	}
}

func (distributor *EventDistributor) Call(event wamp.CallEvent) wamp.ReplyEvent {
	for _, friend := range distributor.friends {
		return friend.Call(event)
	}
	return wamp.NewErrorEvent(event, errors.New(""))
}

func (distributor *EventDistributor) WebsocketJoinCluster(addressList []string) {
	for _, address := range addressList {
		wsAddress := "ws://" + address + "/wamp3/websocket?ticket=" + distributor.ticket
		routerID, transport, e := wampTransport.WebsocketConnect(wsAddress, &wampSerializer.DefaultSerializer)
		if e == nil {
			peer := wamp.NewPeer(routerID, transport)
			go peer.Consume()
			session := wamp.NewSession(peer)
			distributor.friends[routerID] = session
		}
	}
}
