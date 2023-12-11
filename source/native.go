package router

import (
	"errors"

	wamp "github.com/wamp3hub/wamp3go"
)

func mount[I, O any](
	router *Router,
	uri string,
	options *wamp.RegisterOptions,
	procedure wamp.CallProcedure[I, O],
) {
	registration, _ := router.Dealer.register(uri, router.Session.ID(), options)
	endpoint := wamp.NewCallEventEndpoint[I, O](procedure, router.logger)
	router.Session.Registrations[registration.ID] = endpoint
}

func (router *Router) register(
	payload wamp.NewResourcePayload[wamp.RegisterOptions],
	callEvent wamp.CallEvent,
) (*wamp.Registration, error) {
	route := callEvent.Route()
	if len(payload.URI) > 0 {
		registration, e := router.Dealer.register(payload.URI, route.CallerID, payload.Options)
		if e == nil {
			return registration, nil
		}
	}
	return nil, errors.New("InvalidURI")
}

func (router *Router) unregister(
	registrationID string,
	callEvent wamp.CallEvent,
) (struct{}, error) {
	route := callEvent.Route()
	if len(registrationID) > 0 {
		router.Dealer.unregister(route.CallerID, registrationID)
		return struct{}{}, nil
	}
	return struct{}{}, wamp.InvalidPayload
}

func (router *Router) getRegistrationList(
	payload any,
	callEvent wamp.CallEvent,
) (*RegistrationList, error) {
	source := wamp.Event(callEvent)
	URIList := router.Dealer.registrations.DumpURIList()
	for _, uri := range URIList {
		registrationList := router.Dealer.registrations.Match(uri)
		nextEvent := wamp.Yield(source, registrationList)
		source = nextEvent
	}
	return nil, wamp.GeneratorExit(source)
}

func (router *Router) subscribe(
	payload wamp.NewResourcePayload[wamp.SubscribeOptions],
	callEvent wamp.CallEvent,
) (*wamp.Subscription, error) {
	route := callEvent.Route()
	if len(payload.URI) > 0 {
		subscription, e := router.Broker.subscribe(payload.URI, route.CallerID, payload.Options)
		if e == nil {
			return subscription, nil
		}
	}
	return nil, errors.New("InvalidURI")
}

func (router *Router) unsubscribe(
	subscriptionID string,
	callEvent wamp.CallEvent,
) (struct{}, error) {
	route := callEvent.Route()
	if len(subscriptionID) > 0 {
		router.Broker.unsubscribe(route.CallerID, subscriptionID)
		return struct{}{}, nil
	}
	return struct{}{}, wamp.InvalidPayload
}

func (router *Router) getSubscriptionList(
	payload any,
	callEvent wamp.CallEvent,
) (*SubscriptionList, error) {
	source := wamp.Event(callEvent)
	URIList := router.Broker.subscriptions.DumpURIList()
	for _, uri := range URIList {
		subscriptionList := router.Broker.subscriptions.Match(uri)
		nextEvent := wamp.Yield(source, subscriptionList)
		source = nextEvent
	}
	return nil, wamp.GeneratorExit(source)
}
