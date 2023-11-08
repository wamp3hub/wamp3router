package run

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"

	router "github.com/wamp3hub/wamp3router"
	routerServers "github.com/wamp3hub/wamp3router/servers"
	routerShared "github.com/wamp3hub/wamp3router/shared"
	routerStorages "github.com/wamp3hub/wamp3router/storages"
)

func Run(
	routerID string,
	http2address string,
	enableWebsocket bool,
	unixPath string,
	storageClass string,
	storagePath string,
	debug bool,
) {
	log.SetFlags(0)

	storage, e := routerStorages.NewBoltDBStorage(storagePath)
	if e != nil {
		panic("failed to initialize storage")
	}

	consumeNewcomers, produceNewcomer, closeNewcomers := wampShared.NewStream[*wamp.Peer]()

	alphaTransport, betaTransport := wampTransports.NewDuplexLocalTransport(128)
	alphaPeer := wamp.SpawnPeer(routerID, alphaTransport)
	betaPeer := wamp.SpawnPeer(routerID, betaTransport)
	session := wamp.NewSession(betaPeer)

	router.Serve(session, storage, consumeNewcomers)

	produceNewcomer(alphaPeer)

	keyRing := routerShared.NewKeyRing()

	http2server := routerServers.HTTP2Server{
		EnableWebsocket: enableWebsocket,
		Address:         http2address,
		Session:         session,
		KeyRing:         keyRing,
		ProduceNewcomer: produceNewcomer,
	}
	unixServer := routerServers.UnixServer{
		Path: unixPath,
		Session: session,
		KeyRing: keyRing,
		ProduceNewcomer: produceNewcomer,
	}

	go http2server.Serve()
	go unixServer.Serve()

	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

	<-exitSignal
	log.Printf("Gracefully shutting down...")
	http2server.Shutdown()
	unixServer.Shutdown()
	closeNewcomers()
	storage.Destroy()
	log.Printf("Shutdown complete")
}

var (
	routerIDFlag        *string
	http2addressFlag    *string
	enableWebsocketFlag *bool
	unixPathFlag        *string
	storageClassFlag    *string
	storagePathFlag     *string
	debugFlag           *bool
	Command             = &cobra.Command{
		Use:   "run",
		Short: "Run new instance of Router",
		Run: func(cmd *cobra.Command, args []string) {
			Run(
				*routerIDFlag,
				*http2addressFlag,
				*enableWebsocketFlag,
				*unixPathFlag,
				*storageClassFlag,
				*storagePathFlag,
				*debugFlag,
			)
		},
	}
)

func init() {
	defaultRouterID := wampShared.NewID()
	defaultUnixPath := "/tmp/wamp3rd-" + defaultRouterID + ".socket"
	defaultStoragePath := "/tmp/wamp3rd-" + defaultRouterID + ".db"
	routerIDFlag = Command.Flags().String("id", defaultRouterID, "router id")
	http2addressFlag = Command.Flags().String("http2address", ":8888", "http2 address")
	enableWebsocketFlag = Command.Flags().Bool("websocket", true, "enable websocket")
	unixPathFlag = Command.Flags().String("unix-path", defaultUnixPath, "unix socket path")
	storageClassFlag = Command.Flags().String("storage-class", "BoltDB", "storage class")
	storagePathFlag = Command.Flags().String("storage-path", defaultStoragePath, "storage path")
	debugFlag = Command.Flags().Bool("debug", false, "enable debug")
}
