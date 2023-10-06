package service

import (
	"errors"

	wamp "github.com/wamp3hub/wamp3go"
	wampSerializer "github.com/wamp3hub/wamp3go/serializer"
	wampTransport "github.com/wamp3hub/wamp3go/transport"

	router "github.com/wamp3hub/wamp3router"
)

type Friends []*wamp.Session

type EventDistributor struct {
	friends Friends
}

func NewEventDistributor() *EventDistributor {
	return &EventDistributor{Friends{}}
}

func (distributor *EventDistributor) addFreind(instance *wamp.Session) {
	distributor.friends = append(distributor.friends, instance)
}

func (distributor *EventDistributor) Connect(address string) error {
	session, e := wampTransport.WebsocketJoin(address, wampSerializer.DefaultJSONSerializer, router.Emptiness{})
	if e == nil {
		distributor.addFreind(session)
	}
	return e
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

func (distributor *EventDistributor) Publish(event wamp.PublishEvent) {
	for _, friend := range distributor.friends {
		go friend.Publish(event)
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

func (distributor *EventDistributor) Call(event wamp.CallEvent) wamp.ReplyEvent {
	for _, friend := range distributor.friends {
		return friend.Call(event)
	}
	return wamp.NewErrorEvent(event, errors.New(""))
}
