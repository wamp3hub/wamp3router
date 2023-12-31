package generateTicket

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"
)

func GenerateTicket(
	unixPath string,
	peerID string,
	duration time.Duration,
) {
	session, e := wampTransports.UnixJoin(&wampTransports.UnixJoinOptions{Path: "/tmp/wamp3rd.socket"})
	if e == nil {
		type GenerateTicketPayload struct {
			PeerID   string
			Duration time.Duration
		}
		pendingResponse := wamp.Call[string](
			session,
			&wamp.CallFeatures{URI: "wamp.ticket.generate"},
			GenerateTicketPayload{peerID, time.Minute * duration},
		)
		_, ticket, e := pendingResponse.Await()
		if e == nil {
			log.Print(ticket)
		} else {
			log.Printf("generate ticket error=%s", e)
		}
	}
}

var (
	unixPathFlag *string
	peerIDFlag   *string
	durationFlag *time.Duration
	Command      = &cobra.Command{
		Use:   "generate-ticket",
		Short: "Generates new authentication ticket",
		Run: func(cmd *cobra.Command, args []string) {
			GenerateTicket(*unixPathFlag, *peerIDFlag, *durationFlag)
		},
	}
)

func init() {
	unixPathFlag = Command.Flags().String("unix-path", "", "unix-path")
	peerIDFlag = Command.Flags().String("peer", wampShared.NewID(), "peer id")
	durationFlag = Command.Flags().Duration("duration", 1440, "duration minutes")
}
