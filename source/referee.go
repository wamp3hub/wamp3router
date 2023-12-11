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
	dealer           *Dealer
	caller           *wamp.Peer
	executor         *wamp.Peer
	logger           *slog.Logger
}

func (referee *Referee) stop() {
	event := wamp.NewStopEvent(referee.ID)
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, referee.dealer.session.ID())

	e := referee.executor.Send(event)
	if e == nil {
		referee.logger.Debug("generator stop success")
	} else {
		referee.logger.Error("during send cancel event", "error", e)
	}
}

func (referee *Referee) onNext(nextEvent wamp.NextEvent) {
	nextFeatures := nextEvent.Features()

	yieldEventPromise, cancelYieldEventPromise := referee.executor.PendingReplyEvents.New(
		nextEvent.ID(), time.Duration(nextFeatures.Timeout)*time.Second,
	)

	e := referee.executor.Send(nextEvent)
	if e == nil {
		referee.logger.Debug("next event sent")

		select {
		case response, done := <-yieldEventPromise:
			if done {
				referee.logger.Debug("yield event received")
			} else {
				referee.logger.Debug("yield event timeout")
				response = wamp.NewErrorEvent(nextEvent, wamp.ErrorTimedOut)
			}

			if response.Kind() == wamp.MK_YIELD {
				referee.onYield(response)
			} else {
				referee.dealer.sendReply(referee.caller, response)
			}
		case <-referee.stopEventPromise:
			cancelYieldEventPromise()
			referee.stop()
		}
	} else {
		cancelYieldEventPromise()
		referee.logger.Error("during send next event", "error", e)
		errorEvent := wamp.NewErrorEvent(nextEvent, wamp.ErrorApplication)
		referee.dealer.sendReply(referee.caller, errorEvent)
	}
}

func (referee *Referee) onYield(yieldEvent wamp.YieldEvent) {
	nextEventPromise, cancelNextEventPromise := referee.caller.PendingNextEvents.New(yieldEvent.ID(), 0)

	e := referee.caller.Send(yieldEvent)
	if e == nil {
		referee.logger.Debug("yield event sent")

		select {
		case nextEvent := <-nextEventPromise:
			referee.onNext(nextEvent)
		case <-referee.stopEventPromise:
			cancelNextEventPromise()
			referee.stop()
		}
	} else {
		cancelNextEventPromise()
		referee.logger.Error("during send yield event", "error", e)
		referee.stop()
	}
}

func loopGenerator(
	dealer *Dealer,
	caller *wamp.Peer,
	executor *wamp.Peer,
	callEvent wamp.CallEvent,
	yieldEvent wamp.YieldEvent,
	__logger *slog.Logger,
) {
	generator, _ := wamp.SerializePayload[wamp.NewGeneratorPayload](yieldEvent)

	stopEventPromise, cancelStopEventPromise := executor.PendingCancelEvents.New(
		generator.ID, time.Duration(wamp.DEFAULT_GENERATOR_LIFETIME)*time.Second,
	)

	logger := __logger.With(
		"name", "Referee",
		"GeneratorID", generator.ID,
		"CallerID", caller.ID,
		"ExecutorID", executor.ID,
		"YieldID", yieldEvent.ID(),
	)

	referee := Referee{generator.ID, stopEventPromise, dealer, caller, executor, logger}

	referee.onYield(yieldEvent)

	cancelStopEventPromise()

	logger.Debug("destroy generator")
}
