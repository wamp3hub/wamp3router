package main

import "github.com/wamp3hub/wamp3router"

func main() {
	wamp3router.Serve("localhost:9999", false)
}
