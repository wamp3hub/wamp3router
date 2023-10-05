module github.com/wamp3hub/wamp3router

go 1.20

require (
	github.com/boltdb/bolt v1.3.1
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/gorilla/websocket v1.5.0
	github.com/rs/xid v1.5.0
	github.com/wamp3hub/wamp3go v0.2.3
)

require golang.org/x/sys v0.12.0 // indirect

replace github.com/wamp3hub/wamp3go => ../wamp3go
