package service

import (
	"log"

	wamp "github.com/wamp3hub/wamp3go"

	router "github.com/wamp3hub/wamp3router"
)

func SpawnEventRepeater(self *wamp.Session, cluster *EventDistributor) {
	URIList := []string{}

	callEvent := wamp.NewCallEvent(&wamp.CallFeatures{"wamp.subscription.uri.list"}, router.Emptiness{})
	replyEvent := cluster.Call(callEvent)
	e := replyEvent.Error()
	if e == nil {
		e = replyEvent.Payload(&URIList)
		if e == nil {
			for _, uri := range URIList {
				_, e := self.Subscribe(
					uri,
					&wamp.SubscribeOptions{},
					func(publishEvent wamp.PublishEvent) {
						log.Printf("publish forward success")
						cluster.Publish(publishEvent)
					},
				)
				if e == nil {
					cluster.Subscribe(
						uri,
						&wamp.SubscribeOptions{},
						func(publishEvent wamp.PublishEvent) {
							e := self.Publish(publishEvent)
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
	}

	callEvent = wamp.NewCallEvent(&wamp.CallFeatures{"wamp.registration.uri.list"}, router.Emptiness{})
	replyEvent = cluster.Call(callEvent)
	e = replyEvent.Error()
	if e == nil {
		e = replyEvent.Payload(&URIList)
		if e == nil {
			for _, uri := range URIList {
				_, e := self.Register(
					uri,
					&wamp.RegisterOptions{},
					func(callEvent wamp.CallEvent) wamp.ReplyEvent {
						replyEvent := cluster.Call(callEvent)
						log.Printf("call forward success")
						return replyEvent
					},
				)
				if e == nil {
					cluster.Register(
						uri,
						&wamp.RegisterOptions{},
						func(callEvent wamp.CallEvent) wamp.ReplyEvent {
							replyEvent := self.Call(callEvent)
							log.Printf("call forward success")
							return replyEvent
						},
					)
				}
			}
		}
	}
}
