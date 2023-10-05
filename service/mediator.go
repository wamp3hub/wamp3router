package service

import (
	"log"

	wamp "github.com/wamp3hub/wamp3go"
	router "github.com/wamp3hub/wamp3router"
)

type Mediator struct {
	i      *wamp.Session
	friends *EventDistributor
}

func (mediator *Mediator) Initialize() {
	URIList := []string{}
	callEvent := wamp.NewCallEvent(&wamp.CallFeatures{"wamp.subscription.uri.list"}, router.Emptiness{})
	replyEvent := mediator.friends.Call(callEvent)
	e := replyEvent.Error()
	if e == nil {
		e = replyEvent.Payload(&URIList)
		if e == nil {
			for _, uri := range URIList {
				mediator.i.Subscribe(
					uri,
					&wamp.SubscribeOptions{},
					func(publishEvent wamp.PublishEvent) {
						e := mediator.friends.Publish(publishEvent)
						if e == nil {
							log.Printf("publish forward success")
						} else {
							log.Printf("publish forward error %s", e)
						}
					},
				)

				mediator.friends.Subscribe(
					uri,
					&wamp.SubscribeOptions{},
					func(publishEvent wamp.PublishEvent) {
						e := mediator.i.Publish(publishEvent)
						if e == nil {
							log.Printf("publish forward success")
						} else {
							log.Printf("publish forward error %s", e)
						}
					},
				)
			}
		}
	}

	callEvent = wamp.NewCallEvent(&wamp.CallFeatures{"wamp.registration.uri.list"}, router.Emptiness{})
	replyEvent = mediator.friends.Call(callEvent)
	e = replyEvent.Error()
	if e == nil {
		e = replyEvent.Payload(&URIList)
		if e == nil {
			for _, uri := range URIList {
				mediator.i.Register(
					uri,
					&wamp.RegisterOptions{},
					func(callEvent wamp.CallEvent) wamp.ReplyEvent {
						replyEvent := mediator.friends.Call(callEvent)
						log.Printf("call forward success")
						return replyEvent
					},
				)

				mediator.friends.Register(
					uri,
					&wamp.RegisterOptions{},
					func(callEvent wamp.CallEvent) wamp.ReplyEvent {
						replyEvent := mediator.i.Call(callEvent)
						log.Printf("call forward success")
						return replyEvent
					},
				)
			}
		}
	}
}
