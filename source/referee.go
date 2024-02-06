package router

import (
	"log/slog"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
)

type Referee struct {
	ID               string
	routerID         string
	caller           *wamp.Peer
	executor         *wamp.Peer
	logger           *slog.Logger
	stopEventPromise wampShared.Promise[wamp.StopEvent]
}

func (referee *Referee) stop() {
	event := wamp.NewStopEvent(referee.ID)
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, referee.routerID)

	ok := referee.executor.Send(event, wamp.DEFAULT_RESEND_COUNT)
	if ok {
		referee.logger.Debug("generator stop success")
	} else {
		referee.logger.Error("stop event dispatch error")
	}
}

func (referee *Referee) yield(nextEvent wamp.NextEvent) {
	nextFeatures := nextEvent.Features()
	timeout := time.Duration(nextFeatures.Timeout) * time.Second
	yieldEventPromise, cancelYieldEventPromise := referee.executor.PendingReplyEvents.New(nextEvent.ID(), timeout)
	ok := referee.executor.Send(nextEvent, wamp.DEFAULT_RESEND_COUNT)
	if ok {
		referee.logger.Debug("next event sent")

		select {
		case response, done := <-yieldEventPromise:
			if !done {
				referee.logger.Debug("yield event timed out")
				response = wamp.NewErrorEvent(nextEvent, wamp.ErrorTimedOut)
			}

			referee.round(response)
		case <-referee.stopEventPromise:
			referee.logger.Debug("generator stop event received")
			cancelYieldEventPromise()
			referee.stop()
		}
	} else {
		referee.logger.Error("next event dispatch error")
		cancelYieldEventPromise()
		errorEvent := wamp.NewErrorEvent(nextEvent, wamp.ErrorApplication)
		referee.round(errorEvent)
	}
}

func (referee *Referee) next(yieldEvent wamp.YieldEvent) {
	nextEventPromise, cancelNextEventPromise := referee.caller.PendingNextEvents.New(yieldEvent.ID(), 0)
	ok := referee.caller.Send(yieldEvent, wamp.DEFAULT_RESEND_COUNT)
	if ok {
		referee.logger.Debug("yield event sent")

		select {
		case nextEvent := <-nextEventPromise:
			referee.yield(nextEvent)
		case <-referee.stopEventPromise:
			referee.logger.Debug("generator stop event received")
			cancelNextEventPromise()
			referee.stop()
		}
	} else {
		referee.logger.Error("yield event dispatch error")
		cancelNextEventPromise()
		referee.stop()
	}
}

func (referee *Referee) round(response wamp.ReplyEvent) {
	if response.Kind() == wamp.MK_YIELD {
		referee.next(response)
	} else {
		ok := referee.caller.Send(response, wamp.DEFAULT_RESEND_COUNT)
		if ok {
			referee.logger.Debug("last event sent")
		} else {
			referee.logger.Error("last event dispatch error")
		}
	}
}

func loopGenerator(
	routerID string,
	caller *wamp.Peer,
	executor *wamp.Peer,
	callEvent wamp.CallEvent,
	yieldEvent wamp.YieldEvent,
	__logger *slog.Logger,
) {
	callFeatures := callEvent.Features()
	generator, _ := wamp.ReadPayload[wamp.NewGeneratorPayload](yieldEvent)
	lifetime := time.Duration(wamp.DEFAULT_GENERATOR_LIFETIME) * time.Second
	stopEventPromise, cancelStopEventPromise := executor.PendingCancelEvents.New(generator.ID, lifetime)
	logger := __logger.With(
		"name", "Referee",
		"GeneratorID", generator.ID,
		"URI", callFeatures.URI,
		"YieldID", yieldEvent.ID(),
		"Lifetime", lifetime,
		"CallerID", caller.ID,
		"ExecutorID", executor.ID,
	)
	referee := Referee{generator.ID, routerID, caller, executor, logger, stopEventPromise}
	referee.next(yieldEvent)
	cancelStopEventPromise()
	logger.Debug("destroy generator")
}
