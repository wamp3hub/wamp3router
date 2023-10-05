package service

import (
	wamp "github.com/wamp3hub/wamp3go"
)

type EventDistributor struct {
	friends []*wamp.Session
}

func (distributor *EventDistributor) Add(friend *wamp.Session) {
	distributor.friends = append(distributor.friends, friend)
}

func (distributor *EventDistributor) Subscribe(
	uri string,
	features *wamp.SubscribeOptions,
	endpoint wamp.PublishEndpoint,
) {
	for _, friend := range distributor.friends {
		friend.Subscribe(uri, features, endpoint)
	}
}

func (distributor *EventDistributor) Unsubscribe(subscriptionID string) {
	for _, friend := range distributor.friends {
		friend.Unsubscribe(subscriptionID)
	}
}

func (distributor *EventDistributor) Register(
	uri string,
	features *wamp.RegisterOptions,
	endpoint wamp.CallEndpoint,
) {
	for _, friend := range distributor.friends {
		friend.Register(uri, features, endpoint)
	}
}

func (distributor *EventDistributor) Unregister(registrationID string) {
	for _, friend := range distributor.friends {
		friend.Unregister(registrationID)
	}
}

func (distributor *EventDistributor) Publish(event wamp.PublishEvent) error {
	for _, friend := range distributor.friends {
		go friend.Publish(event)
	}
	return nil
}

func (distributor *EventDistributor) Call(event wamp.CallEvent) wamp.ReplyEvent {
	for _, friend := range distributor.friends {
		return friend.Call(event)
	}
	return nil
}

func (distributor *EventDistributor) Leave() {

}
