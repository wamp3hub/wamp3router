package router

import (
	"log/slog"

	wamp "github.com/wamp3hub/wamp3go"
)

type Stream struct {
	ID          string
	left, right *wamp.Peer
	logger      *slog.Logger
}

func NewStream(
	ID string,
	left, right *wamp.Peer,
	logger *slog.Logger,
) *Stream {
	return &Stream{
		ID,
		left,
		right,
		logger.With(
			slog.Group(
				"stream",
				"ID", "ID",
			),
		),
	}
}

func Tranceive(
	stream *Stream,
) {
	stopEventPromise, _ := stream.left.PendingReplyEvents.New(stream.ID, 0)
	subEventsIterator := stream.left.IncomingSubEvents.Iterator(0)
	for {
		select {
		case subEvent := <-subEventsIterator:
			ok := stream.right.Send(subEvent, wamp.DEFAULT_RESEND_COUNT)
			if ok {
				stream.logger.Debug("subevent successfully delivered")
			} else {
				stream.logger.Error("subevent dispatch error")
			}
		case stopEvent := <-stopEventPromise:
			ok := stream.right.Send(stopEvent, wamp.DEFAULT_RESEND_COUNT)
			if ok {
				stream.logger.Debug("stop event successfully delivered")
			} else {
				stream.logger.Error("stop event dispatch error")
			}
		}
	}
}
