module github.com/wamp3hub/wamp3router

go 1.21

require (
	github.com/boltdb/bolt v1.3.1
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/gorilla/websocket v1.5.1
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/rs/cors v1.10.1
	github.com/spf13/cobra v1.8.0
	github.com/wamp3hub/wamp3go v0.5.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
)

replace github.com/wamp3hub/wamp3go => ../wamp3go
