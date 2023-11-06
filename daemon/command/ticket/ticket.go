package ticket

import (
	"github.com/spf13/cobra"

	"github.com/wamp3hub/wamp3router/daemon/command/ticket/generate"
)

var Command = &cobra.Command{
	Use:   "ticket",
	Short: "Ticket",
}

func init() {
	Command.AddCommand(generate.Command)
}
