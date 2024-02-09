package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
)

func receive(
	ID string,
	left, right *wamp.Peer,
	logger *slog.Logger,
) {
	onEvent := func(subEvent wamp.SubEvent) {
		ok := right.Send(subEvent, wamp.DEFAULT_RESEND_COUNT)
		if ok {
			logger.Debug("subevent successfully delivered")
		} else {
			logger.Error("subevent dispatch error")
		}
	}

	left.IncomingSubEvents.Observe(onEvent, nil)

	stopEventPromise, _ := left.PendingCancelEvents.New(ID, 0)
	stopEvent := <-stopEventPromise
	ok := right.Send(stopEvent, wamp.DEFAULT_RESEND_COUNT)
	if ok {
		logger.Debug("stop event successfully delivered")
	} else {
		logger.Error("stop event dispatch error")
	}
}

func Tranceive(
	caller *wamp.Peer,
	executor *wamp.Peer,
	initial wamp.SubEvent,
	__logger *slog.Logger,
) {
	logger := __logger.With()
	go receive(initial.Features(), caller, executor, logger)
	go receive(initial.Features(), executor, caller, logger)
}
