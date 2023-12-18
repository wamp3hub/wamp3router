package router

import (
	"errors"
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
)

var (
	SomethingWentWrong = errors.New("SomethingWentWrong")
)

func mount[I, O any](
	router *Router,
	uri string,
	options *wamp.RegisterOptions,
	procedure wamp.CallProcedure[I, O],
) {
	registration := wamp.Registration{
		ID:       wampShared.NewID(),
		URI:      uri,
		AuthorID: router.ID,
		Options:  options,
	}
	router.Dealer.registrations.Add(&registration)
	endpoint := wamp.NewCallEventEndpoint[I, O](procedure, router.logger)
	router.Session.Registrations[registration.ID] = endpoint
}

func (router *Router) intialize() {
	mount(router, "wamp.router.register", &wamp.RegisterOptions{}, router.__register)
	mount(router, "wamp.router.unregister", &wamp.RegisterOptions{}, router.__unregister)
	mount(router, "wamp.router.registration.list", &wamp.RegisterOptions{}, router.__getRegistrationList)
	mount(router, "wamp.router.subscribe", &wamp.RegisterOptions{}, router.__subscribe)
	mount(router, "wamp.router.unsubscribe", &wamp.RegisterOptions{}, router.__unsubscribe)
	mount(router, "wamp.router.subscription.list", &wamp.RegisterOptions{}, router.__getSubscriptionList)
}

func (router *Router) __register(
	payload wamp.NewResourcePayload[wamp.RegisterOptions],
	callEvent wamp.CallEvent,
) (*wamp.Registration, error) {
	if len(payload.URI) == 0 {
		return nil, wamp.InvalidPayload
	}

	route := callEvent.Route()

	logData := slog.Group(
		"registration",
		"URI", payload.URI,
		"AuthorID", route.CallerID,
	)

	registration := wamp.Registration{
		ID:       wampShared.NewID(),
		URI:      payload.URI,
		AuthorID: route.CallerID,
		Options:  payload.Options,
	}
	payload.Options.Route = append(payload.Options.Route, router.ID)
	e := router.Dealer.registrations.Add(&registration)
	if e != nil {
		router.logger.Error("during add registration into URIM", "error", e, logData)
		return nil, SomethingWentWrong
	}

	e = wamp.Publish(
		router.Session,
		&wamp.PublishFeatures{
			URI:     "wamp.registration.new",
			Exclude: []string{registration.AuthorID},
		},
		registration,
	)
	if e == nil {
		router.logger.Info("new registeration", logData)
	} else {
		router.logger.Error(
			"during publish to topic 'wamp.registration.new'", "error", e, logData,
		)
	}

	return &registration, nil
}

func (router *Router) unregister(
	authorID string,
	registrationID string,
) {
	removedRegistrationList := router.Dealer.registrations.DeleteByAuthor(authorID, registrationID)
	for _, registration := range removedRegistrationList {
		logData := slog.Group(
			"registration",
			"URI", registration.URI,
			"AuthorID", registration.AuthorID,
		)

		e := wamp.Publish(
			router.Session,
			&wamp.PublishFeatures{
				URI:     "wamp.registration.gone",
				Exclude: []string{registration.AuthorID},
			},
			registration.URI,
		)
		if e == nil {
			router.logger.Info("registration gone", logData)
		} else {
			router.logger.Error("during publish to topic 'wamp.registration.gone'", logData)
		}
	}
}

func (router *Router) __unregister(
	registrationID string,
	callEvent wamp.CallEvent,
) (struct{}, error) {
	if len(registrationID) == 0 {
		return struct{}{}, wamp.InvalidPayload
	}

	route := callEvent.Route()
	router.unregister(route.CallerID, registrationID)

	return struct{}{}, nil
}

func (router *Router) __getRegistrationList(
	payload any,
	callEvent wamp.CallEvent,
) (*RegistrationList, error) {
	source := wamp.Event(callEvent)
	URIList := router.Dealer.registrations.DumpURIList()
	for _, uri := range URIList {
		registrationList := router.Dealer.registrations.Match(uri)
		source = wamp.Yield(source, registrationList)
	}
	return nil, wamp.GeneratorExit(source)
}

func (router *Router) __subscribe(
	payload wamp.NewResourcePayload[wamp.SubscribeOptions],
	callEvent wamp.CallEvent,
) (*wamp.Subscription, error) {
	if len(payload.URI) == 0 {
		return nil, errors.New("InvalidURI")
	}

	route := callEvent.Route()

	logData := slog.Group(
		"registration",
		"URI", payload.URI,
		"AuthorID", route.CallerID,
	)

	subscription := wamp.Subscription{
		ID:       wampShared.NewID(),
		URI:      payload.URI,
		AuthorID: route.CallerID,
		Options:  payload.Options,
	}
	subscription.Options.Route = append(subscription.Options.Route, router.ID)
	e := router.Broker.subscriptions.Add(&subscription)
	if e != nil {
		router.logger.Error("during add subscription into URIM", "error", e, logData)
		return nil, SomethingWentWrong
	}

	e = wamp.Publish(
		router.Session,
		&wamp.PublishFeatures{
			URI:     "wamp.subscription.new",
			Exclude: []string{subscription.AuthorID},
		},
		subscription,
	)
	if e == nil {
		router.logger.Info("new subscription", logData)
	} else {
		router.logger.Error("during publish to 'wamp.subscription.new'", "error", e, logData)
	}

	return &subscription, nil
}

func (router *Router) unsubscribe(
	authorID string,
	subscriptionID string,
) {
	removedSubscriptionList := router.Broker.subscriptions.DeleteByAuthor(authorID, subscriptionID)
	for _, subscription := range removedSubscriptionList {
		logData := slog.Group(
			"subscription",
			"URI", subscription.URI,
			"AuthorID", subscription.AuthorID,
		)

		e := wamp.Publish(
			router.Session,
			&wamp.PublishFeatures{
				URI:     "wamp.subscription.gone",
				Exclude: []string{subscription.AuthorID},
			},
			subscription.URI,
		)
		if e == nil {
			router.logger.Info("subscription gone", logData)
		} else {
			router.logger.Error("during publish to 'wamp.subscription.gone'", logData)
		}
	}
}

func (router *Router) __unsubscribe(
	subscriptionID string,
	callEvent wamp.CallEvent,
) (struct{}, error) {
	if len(subscriptionID) == 0 {
		return struct{}{}, wamp.InvalidPayload
	}

	route := callEvent.Route()
	router.unsubscribe(route.CallerID, subscriptionID)

	return struct{}{}, nil
}

func (router *Router) __getSubscriptionList(
	payload any,
	callEvent wamp.CallEvent,
) (*SubscriptionList, error) {
	source := wamp.Event(callEvent)
	URIList := router.Broker.subscriptions.DumpURIList()
	for _, uri := range URIList {
		subscriptionList := router.Broker.subscriptions.Match(uri)
		source = wamp.Yield(source, subscriptionList)
	}
	return nil, wamp.GeneratorExit(source)
}
