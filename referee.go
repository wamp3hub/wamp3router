package router

import (
	"log"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
)

type Referee struct {
	ID               string
	dealer           *Dealer
	caller           *wamp.Peer
	executor         *wamp.Peer
	stopEventPromise wampShared.Promise[wamp.StopEvent]
}

func (referee *Referee) stop() {
	event := wamp.NewStopEvent(referee.ID)
	features := event.Features()
	features.VisitedRouters = append(features.VisitedRouters, referee.dealer.session.ID())
	e := referee.executor.Send(event)
	if e == nil {
		log.Printf(
			"[dealer] generator stop success (executor.ID=%s generator.ID=%s)",
			referee.executor.ID, features.InvocationID,
		)
	} else {
		log.Printf(
			"[dealer] generator stop error %s (executor.ID=%s generator.ID=%s)",
			e, referee.executor.ID, features.InvocationID,
		)
	}
}

func (referee *Referee) onNext(nextEvent wamp.NextEvent) error {
	nextFeatures := nextEvent.Features()

	yieldEventPromise, cancelYieldEventPromise := referee.executor.PendingReplyEvents.New(
		nextEvent.ID(), nextFeatures.Timeout,
	)

	e := referee.executor.Send(nextEvent)
	if e != nil {
		cancelYieldEventPromise()

		log.Printf(
			"[dealer] send next error %s (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
			e, referee.caller.ID, referee.executor.ID, nextEvent.ID(),
		)

		response := wamp.NewErrorEvent(nextEvent, wamp.SomethingWentWrong)
		referee.dealer.sendReply(referee.caller, response)

		return e
	}

	log.Printf(
		"[dealer] send next success (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
		referee.caller.ID, referee.executor.ID, nextEvent.ID(),
	)

	select {
	case response, done := <-yieldEventPromise:
		if !done {
			log.Printf(
				"[dealer] yield error (caller.ID=%s executor.ID=%s nextEvent.ID=%s)",
				referee.caller.ID, referee.executor.ID, nextEvent.ID(),
			)

			response = wamp.NewErrorEvent(nextEvent, wamp.TimedOutError)
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

	e := referee.caller.Send(yieldEvent)
	if e != nil {
		cancelNextEventPromise()

		log.Printf(
			"[dealer] send yield error %s (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
			e, referee.caller.ID, referee.executor.ID, yieldEvent.ID(),
		)

		referee.stop()

		return e
	}

	log.Printf(
		"[dealer] send yield success (caller.ID=%s executor.ID=%s yieldEvent.ID=%s)",
		referee.caller.ID, referee.executor.ID, yieldEvent.ID(),
	)

	select {
	case nextEvent := <-nextEventPromise:
		// TODO log

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
) error {
	generator := new(wamp.NewGeneratorPayload)
	yieldEvent.Payload(generator)

	stopEventPromise, cancelStopEventPromise := executor.PendingCancelEvents.New(
		generator.ID, wamp.DEFAULT_GENERATOR_LIFETIME,
	)

	referee := Referee{
		generator.ID, dealer, caller, executor, stopEventPromise,
	}

	e := referee.onYield(yieldEvent)

	cancelStopEventPromise()

	log.Printf(
		"[dealer] destroy generator (caller.ID=%s executor.ID=%s)",
		caller.ID, executor.ID,
	)

	return e
}
