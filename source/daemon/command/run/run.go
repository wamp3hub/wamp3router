package run

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	wampShared "github.com/wamp3hub/wamp3go/shared"

	router "github.com/wamp3hub/wamp3router/source"
	routerServers "github.com/wamp3hub/wamp3router/source/servers"
	routerShared "github.com/wamp3hub/wamp3router/source/shared"
	routerStorages "github.com/wamp3hub/wamp3router/source/storages"
)

func ReadKeyPair(
	// publicKeyPath string,
	privateKeyPath string,
	__logger *slog.Logger,
) *routerShared.KeyRing {
	logger := __logger.With("name", "ReadKeyPair")
	logger.Debug("Reading rsa private key")
	privateKey, e := routerShared.ReadRSAPrivateKey(privateKeyPath)
	if e == nil {
		return routerShared.NewKeyRing(privateKey)
	}

	logger.Warn("Unable to parse RSA private key, generating a temporary one", "error", e)
	keyRing := routerShared.GenerateKeyRing()
	routerShared.WriteRSAPrivateKey(privateKeyPath, keyRing.PrivateKey)
	return keyRing
}

func Run(
	routerID string,
	http2address string,
	enableWebsocket bool,
	unixPath string,
	storageClass string,
	storagePath string,
	privateKeyPath string,
	debug bool,
) {
	routerShared.PrintLogotype()

	loggingLevel := slog.LevelInfo
	if debug {
		loggingLevel = slog.LevelDebug
	}
	handler := slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{AddSource: false, Level: loggingLevel},
	)
	logger := slog.New(handler)

	storage, e := routerStorages.NewBoltDBStorage(storagePath)
	if e != nil {
		logger.Error("during initialization of storage", "error", e)
		panic("failed to initialize storage")
	}

	keyRing := ReadKeyPair(privateKeyPath, logger)

	__router := router.NewRouter(
		wampShared.NewID(),
		storage,
		keyRing,
		logger,
	)
	http2server := routerServers.NewHTTP2Server(
		http2address,
		enableWebsocket,
		__router,
		logger,
	)
	unixServer := routerServers.NewUnixServer(
		unixPath,
		__router,
		logger,
	)
	go http2server.Serve()
	go unixServer.Serve()
	go __router.Serve()

	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)
	<-exitSignal

	logger.Info("gracefully shutting down...")
	http2server.Shutdown()
	unixServer.Shutdown()
	__router.Shutdown()
	storage.Destroy()
	logger.Info("shutdown complete")
}

var (
	routerIDFlag        *string
	http2addressFlag    *string
	enableWebsocketFlag *bool
	unixPathFlag        *string
	storageClassFlag    *string
	storagePathFlag     *string
	privateKeyPathFlag  *string
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
				*privateKeyPathFlag,
				*debugFlag,
			)
		},
	}
)

func init() {
	defaultRouterID := wampShared.NewID()
	defaultUnixPath := "/tmp/wamp3rd-" + defaultRouterID + ".socket"
	defaultStoragePath := "/tmp/wamp3rd-" + defaultRouterID + ".db"
	defaultPrivateKeyPath := "/tmp/wamp3rd.pem"
	routerIDFlag = Command.Flags().String("id", defaultRouterID, "router id")
	http2addressFlag = Command.Flags().String("http2address", ":8800", "http2 address")
	enableWebsocketFlag = Command.Flags().Bool("websocket", true, "enable websocket")
	unixPathFlag = Command.Flags().String("unix-path", defaultUnixPath, "unix socket path")
	storageClassFlag = Command.Flags().String("storage-class", "BoltDB", "storage class")
	storagePathFlag = Command.Flags().String("storage-path", defaultStoragePath, "storage path")
	privateKeyPathFlag = Command.Flags().String("private-key-path", defaultPrivateKeyPath, "rsa private key path in pem format")
	debugFlag = Command.Flags().Bool("debug", false, "enable debug")
}
