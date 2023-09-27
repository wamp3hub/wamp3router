module main

go 1.20

replace wamp3router => ../

replace wamp3go => ../../wamp3go

require (
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	wamp3go v0.0.0-00010101000000-000000000000 // indirect
	wamp3router v0.0.0-00010101000000-000000000000 // indirect
)
