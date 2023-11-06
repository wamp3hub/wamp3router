package generate

import (
	"log"
	"time"

	"github.com/rs/xid"
	"github.com/spf13/cobra"
	wamp "github.com/wamp3hub/wamp3go"
	wampSerializer "github.com/wamp3hub/wamp3go/serializer"
	wampTransport "github.com/wamp3hub/wamp3go/transport"
)

var Command = &cobra.Command{
	Use:   "generate",
	Short: "Generates new authentication ticket",
	Run: func(cmd *cobra.Command, args []string) {
		session, e := wampTransport.UnixJoin("/tmp/wamp-cli.socket", wampSerializer.DefaultSerializer)
		if e == nil {
			type GenerateTicketPayload struct {
				PeerID   string
				Duration time.Duration
			}
			peerID := xid.New().String()
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
