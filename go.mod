module wamp3router

go 1.20

require (
	github.com/boltdb/bolt v1.3.1
	github.com/google/uuid v1.3.1
	github.com/gorilla/websocket v1.5.0
	wamp3go v0.0.0-00010101000000-000000000000
)

require golang.org/x/sys v0.10.0 // indirect

replace wamp3go => ../wamp3go
