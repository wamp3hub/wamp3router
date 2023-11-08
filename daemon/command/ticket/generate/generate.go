package generate

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/wamp3hub/wamp3go"
	"github.com/wamp3hub/wamp3go/serializers"
	"github.com/wamp3hub/wamp3go/shared"
	"github.com/wamp3hub/wamp3go/transports"
)

var Command = &cobra.Command{
	Use:   "generate",
	Short: "Generates new authentication ticket",
	Run: func(cmd *cobra.Command, args []string) {
		session, e := wampTransports.UnixJoin("/tmp/wamp-cli.socket", wampSerializers.DefaultSerializer)
		if e == nil {
			type GenerateTicketPayload struct {
				PeerID   string
				Duration time.Duration
			}
			peerID := wampShared.NewID()
			pendingResponse := wamp.Call[string](
				session,
				&wamp.CallFeatures{URI: "wamp.ticket.generate"},
				GenerateTicketPayload{peerID, time.Hour * 24},
			)
			_, ticket, e := pendingResponse.Await()
			if e == nil {
				log.Print(ticket)
			} else {

			}
		}
	},
}

func Execute() {
	Command.Execute()
}
