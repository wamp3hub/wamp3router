package router

import wamp "github.com/wamp3hub/wamp3go"

func mount[O any](
	router *Router,
	uri string,
	options *wamp.RegisterOptions,
	procedure wamp.CallProcedure,
) {
	registration, _ := router.Dealer.register(uri, router.Session.ID(), options)
	endpoint := wamp.NewCallEventEndpoint[O](procedure, router.logger)
	router.Session.Registrations[registration.ID] = endpoint
}

func (router *Router) register(callEvent wamp.CallEvent) any {
	route := callEvent.Route()
	payload := new(wamp.NewResourcePayload[wamp.RegisterOptions])
	e := callEvent.Payload(payload)
	if e == nil && len(payload.URI) > 0 {
		registration, e := router.Dealer.register(payload.URI, route.CallerID, payload.Options)
		if e == nil {
			return *registration
		}
	} else {
		e = wamp.InvalidPayload
	}
	return e
}

func (router *Router) unregister(callEvent wamp.CallEvent) any {
	route := callEvent.Route()
	registrationID := ""
	e := callEvent.Payload(&registrationID)
	if e == nil && len(registrationID) > 0 {
		router.Dealer.unregister(route.CallerID, registrationID)
		return true
	}
	return e
}

func (router *Router) getRegistrationList(callEvent wamp.CallEvent) any {
	source := wamp.Event(callEvent)
	URIList := router.Dealer.registrations.DumpURIList()
	for _, uri := range URIList {
		registrationList := router.Dealer.registrations.Match(uri)
		nextEvent := wamp.Yield(source, registrationList)
		source = nextEvent
	}
	return wamp.ExitGenerator
}

func (router *Router) subscribe(callEvent wamp.CallEvent) any {
	route := callEvent.Route()
	payload := new(wamp.NewResourcePayload[wamp.SubscribeOptions])
	e := callEvent.Payload(payload)
	if e == nil && len(payload.URI) > 0 {
		subscription, e := router.Broker.subscribe(payload.URI, route.CallerID, payload.Options)
		if e == nil {
			return *subscription
		}
	} else {
		e = wamp.InvalidPayload
	}
	return e
}

func (router *Router) unsubscribe(callEvent wamp.CallEvent) any {
	route := callEvent.Route()
	subscriptionID := ""
	e := callEvent.Payload(&subscriptionID)
	if e == nil && len(subscriptionID) > 0 {
		router.Broker.unsubscribe(route.CallerID, subscriptionID)
		return true
	}
	return e
}

func (router *Router) getSubscriptionList(callEvent wamp.CallEvent) any {
	source := wamp.Event(callEvent)
	URIList := router.Broker.subscriptions.DumpURIList()
	for _, uri := range URIList {
		subscriptionList := router.Broker.subscriptions.Match(uri)
		nextEvent := wamp.Yield(source, subscriptionList)
		source = nextEvent
	}
	return wamp.ExitGenerator
}
