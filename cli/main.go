package main

import "wamp3router"

func main() {
	wamp3router.Serve("localhost:9999", false)
}
