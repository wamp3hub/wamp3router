package router

import (
	"log/slog"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
)

type Referee struct {
	ID               string
	stopEventPromise wampShared.Promise[wamp.StopEvent]
	session          *wamp.Session
	caller           *wamp.Peer
	executor         *wamp.Peer
	logger           *slog.Logger
}

func (referee *Referee) stop() {
	event := wamp.NewStopEvent(referee.ID)
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, referee.session.ID())

	e := referee.executor.Send(event)
	if e == nil {
		referee.logger.Debug("generator stop success")
	} else {
		referee.logger.Error("during send cancel event", "error", e)
	}
}

func (referee *Referee) onNext(nextEvent wamp.NextEvent) {
	nextFeatures := nextEvent.Features()
	timeout := time.Duration(nextFeatures.Timeout) * time.Second
	yieldEventPromise, cancelYieldEventPromise := referee.executor.PendingReplyEvents.New(nextEvent.ID(), timeout)
	e := referee.executor.Send(nextEvent)
	if e == nil {
		referee.logger.Debug("next event sent")

		select {
		case response, done := <-yieldEventPromise:
			if !done {
				referee.logger.Debug("yield event timedout")
				response = wamp.NewErrorEvent(nextEvent, wamp.ErrorTimedOut)
			}

			referee.onYield(response)
		case <-referee.stopEventPromise:
			referee.logger.Debug("generator stop event received")
			cancelYieldEventPromise()
			referee.stop()
		}
	} else {
		referee.logger.Error("during send next event", "error", e)
		cancelYieldEventPromise()
		errorEvent := wamp.NewErrorEvent(nextEvent, wamp.ErrorApplication)
		referee.onYield(errorEvent)
	}
}

func (referee *Referee) do(yieldEvent wamp.YieldEvent) {
	nextEventPromise, cancelNextEventPromise := referee.caller.PendingNextEvents.New(yieldEvent.ID(), 0)
	e := referee.caller.Send(yieldEvent)
	if e == nil {
		referee.logger.Debug("yield event sent")

		select {
		case nextEvent := <-nextEventPromise:
			referee.onNext(nextEvent)
		case <-referee.stopEventPromise:
			referee.logger.Debug("generator stop event received")
			cancelNextEventPromise()
			referee.stop()
		}
	} else {
		referee.logger.Error("during send yield event", "error", e)
		cancelNextEventPromise()
		referee.stop()
	}
}

func (referee *Referee) onYield(response wamp.ReplyEvent) {
	if response.Kind() == wamp.MK_YIELD {
		referee.do(response)
	} else {
		e := referee.caller.Send(response)
		if e == nil {
			referee.logger.Debug("yield event sent")
		} else {
			referee.logger.Error("during send yield event", "error", e)
		}
	}
}

func loopGenerator(
	session *wamp.Session,
	caller *wamp.Peer,
	executor *wamp.Peer,
	callEvent wamp.CallEvent,
	yieldEvent wamp.YieldEvent,
	__logger *slog.Logger,
) {
	callFeatures := callEvent.Features()
	generator, _ := wamp.SerializePayload[wamp.NewGeneratorPayload](yieldEvent)
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
	referee := Referee{generator.ID, stopEventPromise, session, caller, executor, logger}
	referee.do(yieldEvent)
	cancelStopEventPromise()
	logger.Debug("destroy generator")
}
