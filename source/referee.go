package router

import (
	"log/slog"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
)

type Referee struct {
	ID               string
	dealer           *Dealer
	caller           *wamp.Peer
	executor         *wamp.Peer
	stopEventPromise wampShared.Promise[wamp.StopEvent]
	logger           *slog.Logger
}

func (referee *Referee) stop() {
	event := wamp.NewStopEvent(referee.ID)
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, referee.dealer.session.ID())

	logData := slog.Group(
		"generator",
		"ID", referee.ID,
		"CallerID", referee.caller.ID,
		"ExecutorID", referee.executor.ID,
	)

	e := referee.executor.Send(event)
	if e == nil {
		referee.logger.Info("generator stop success", logData)
	} else {
		referee.logger.Error("during send cancel event", "error", e, logData)
	}
}

func (referee *Referee) onNext(nextEvent wamp.NextEvent) error {
	nextFeatures := nextEvent.Features()

	logData := slog.Group(
		"generator",
		"ID", referee.ID,
		"CallerID", referee.caller.ID,
		"ExecutorID", referee.executor.ID,
		"nextEventID", nextEvent.ID(),
	)

	yieldEventPromise, cancelYieldEventPromise := referee.executor.PendingReplyEvents.New(
		nextEvent.ID(), time.Duration(nextFeatures.Timeout)*time.Second,
	)

	e := referee.executor.Send(nextEvent)
	if e != nil {
		cancelYieldEventPromise()
		referee.logger.Error("during send next event", "error", e, logData)
		response := wamp.NewErrorEvent(nextEvent, wamp.ErrorApplication)
		referee.dealer.sendReply(referee.caller, response)
		return e
	}

	referee.logger.Debug("next event sent", logData)

	select {
	case response, done := <-yieldEventPromise:
		if !done {
			referee.logger.Debug("yield event timeout", logData)
			response = wamp.NewErrorEvent(nextEvent, wamp.ErrorTimedOut)
		} else if response.Kind() == wamp.MK_YIELD {
			return referee.onYield(response)
		}

		referee.dealer.sendReply(referee.caller, response)
	case <-referee.stopEventPromise:
		cancelYieldEventPromise()
		referee.stop()
	}

	return nil
}

func (referee *Referee) onYield(yieldEvent wamp.YieldEvent) error {
	nextEventPromise, cancelNextEventPromise := referee.caller.PendingNextEvents.New(yieldEvent.ID(), 0)

	logData := slog.Group(
		"generator",
		"ID", referee.ID,
		"CallerID", referee.caller.ID,
		"ExecutorID", referee.executor.ID,
		"yieldEventID", yieldEvent.ID(),
	)

	e := referee.caller.Send(yieldEvent)
	if e != nil {
		cancelNextEventPromise()
		referee.logger.Error("during send yield event", "error", e, logData)
		referee.stop()
		return e
	}

	referee.logger.Debug("yield event sent", logData)

	select {
	case nextEvent := <-nextEventPromise:
		return referee.onNext(nextEvent)
	case <-referee.stopEventPromise:
		cancelNextEventPromise()
		referee.stop()
	}
	return nil
}

func loopGenerator(
	dealer *Dealer,
	caller *wamp.Peer,
	executor *wamp.Peer,
	callEvent wamp.CallEvent,
	yieldEvent wamp.YieldEvent,
	__logger *slog.Logger,
) error {
	logger := __logger.With("name", "Referee")

	generator := new(wamp.NewGeneratorPayload)
	yieldEvent.Payload(generator)

	stopEventPromise, cancelStopEventPromise := executor.PendingCancelEvents.New(
		generator.ID, time.Duration(wamp.DEFAULT_GENERATOR_LIFETIME) * time.Second,
	)

	referee := Referee{generator.ID, dealer, caller, executor, stopEventPromise, logger}

	e := referee.onYield(yieldEvent)

	cancelStopEventPromise()

	logData := slog.Group(
		"generator",
		"ID", generator.ID,
		"callerID", caller.ID,
		"executorID", executor.ID,
	)
	logger.Debug("destroy generator", logData)

	return e
}
