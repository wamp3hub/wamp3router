package command

import (
	"github.com/spf13/cobra"

	"github.com/wamp3hub/wamp3router/daemon/command/generateTicket"
	"github.com/wamp3hub/wamp3router/daemon/command/run"
)

var Command = &cobra.Command{
	Use:   "wamp3rd",
	Short: "WAMP3Router",
}

func init() {
	Command.AddCommand(run.Command)
	Command.AddCommand(generateTicket.Command)
}

func Execute() {
	Command.Execute()
}
