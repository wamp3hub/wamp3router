package router_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"
	router "github.com/wamp3hub/wamp3router"
	routerStorages "github.com/wamp3hub/wamp3router/storages"
)

func runRouter() wampShared.Producible[*wamp.Peer] {
	routerID := wampShared.NewID()

	storagePath := "/tmp/wamp3rd-" + routerID + ".db"
	storage, _ := routerStorages.NewBoltDBStorage(storagePath)

	consumeNewcomers, produceNewcomer, _ := wampShared.NewStream[*wamp.Peer]()

	lTransport, rTransport := wampTransports.NewDuplexLocalTransport(128)
	lPeer := wamp.SpawnPeer(routerID, lTransport)
	rPeer := wamp.SpawnPeer(routerID, rTransport)
	session := wamp.NewSession(rPeer)

	router.Serve(session, storage, consumeNewcomers)

	produceNewcomer(lPeer)

	return produceNewcomer
}

func joinSession(
	produceNewcomer wampShared.Producible[*wamp.Peer],
) *wamp.Session {
	alphaID := wampShared.NewID()
	lTransport, rTransport := wampTransports.NewDuplexLocalTransport(128)
	lPeer := wamp.SpawnPeer(alphaID, lTransport)
	rPeer := wamp.SpawnPeer(alphaID, rTransport)
	session := wamp.NewSession(rPeer)
	produceNewcomer(lPeer)
	time.Sleep(time.Second)
	return session
}

func TestSubscribePublish(t *testing.T) {
	produceNewcomer := runRouter()

	t.Run("Case: Happy Path", func(t *testing.T) {
		alphaSession := joinSession(produceNewcomer)
		betaSession := joinSession(produceNewcomer)

		wg := new(sync.WaitGroup)

		subscription, e := wamp.Subscribe(
			alphaSession,
			"net.example",
			&wamp.SubscribeOptions{},
			func(publishEvent wamp.PublishEvent) {
				var message string
				e := publishEvent.Payload(&message)
				if e == nil {
					t.Logf("new message %s", message)
					wg.Done()
				}
			},
		)
		if e == nil {
			t.Log("subscribe success")
		} else {
			t.Fatalf("subscribe error %s", e)
		}

		wg.Add(1)

		e = wamp.Publish(
			betaSession,
			&wamp.PublishFeatures{URI: "net.example"}, "Hello, I'm session beta!")
		if e == nil {
			t.Logf("publish success")
		} else {
			t.Fatalf("publish error %s", e)
		}

		wg.Wait()

		e = wamp.Unsubscribe(alphaSession, subscription.ID)
		if e == nil {
			t.Logf("unsubscribe success")
		} else {
			t.Fatalf("unsubscribe error %s", e)
		}
	})
}

func TestRPC(t *testing.T) {
	produceNewcomer := runRouter()

	t.Run("Case: Happy Path", func(t *testing.T) {
		alphaSession := joinSession(produceNewcomer)
		betaSession := joinSession(produceNewcomer)

		registration, e := wamp.Register(
			alphaSession,
			"net.example.greeting",
			&wamp.RegisterOptions{},
			func(callEvent wamp.CallEvent) wamp.ReplyEvent {
				var name string
				e := callEvent.Payload(&name)
				if e == nil {
					result := "Hello, " + name + "!"
					return wamp.NewReplyEvent(callEvent, result)
				}
				return wamp.NewErrorEvent(callEvent, errors.New("InvalidPayload"))
			},
		)
		if e == nil {
			t.Log("register success")
		} else {
			t.Fatalf("register error %s", e)
		}

		expectedResult := "Hello, beta!"
	
		pendingResponse := wamp.Call[string](
			betaSession,
			&wamp.CallFeatures{URI: "net.example.greeting"},
			"beta",
		)
		_, result, _ := pendingResponse.Await()
		if result == expectedResult {
			t.Logf("RPC success")
		} else {
			t.Fatalf("RPC expected %v, but got %v", expectedResult, result)
		}

		e = wamp.Unregister(alphaSession, registration.ID)
		if e == nil {
			t.Log("unregister success")
		} else {
			t.Fatalf("unregister error %s", e)
		}
	})

	t.Run("Case: Cancellation", func(t *testing.T) {
		alphaSession := joinSession(produceNewcomer)
		betaSession := joinSession(produceNewcomer)

		_, e := wamp.Register(
			alphaSession,
			"net.example.long",
			&wamp.RegisterOptions{},
			func(callEvent wamp.CallEvent) wamp.ReplyEvent {
				time.Sleep(time.Minute)
				return wamp.NewReplyEvent(callEvent, true)
			},
		)
		if e == nil {
			t.Log("register success")
		} else {
			t.Fatalf("register error %s", e)
		}

		pendingResponse := wamp.Call[string](
			betaSession,
			&wamp.CallFeatures{URI: "net.example.long"},
			struct{}{},
		)
		pendingResponse.Cancel()
	})

	t.Run("Case: Registration Not Found", func(t *testing.T) {
		session := joinSession(produceNewcomer)
	
		pendingResponse := wamp.Call[struct {}](
			session,
			&wamp.CallFeatures{URI: "net.example.not_existing"},
			struct{}{},
		)
		_, _, e := pendingResponse.Await()
		if e.Error() == "ProcedureNotFound" {
			t.Log("Success")
		} else {
			t.Fatalf("Invalid behaviour %v", e)
		}
	})
}

func TestGenerator(t *testing.T) {
	produceNewcomer := runRouter()

	t.Run("Case: Happy Path", func(t *testing.T) {
		alphaSession := joinSession(produceNewcomer)
		betaSession := joinSession(produceNewcomer)

		registration, e := wamp.Register(
			alphaSession,
			"net.example.reverse",
			&wamp.RegisterOptions{},
			func(callEvent wamp.CallEvent) wamp.ReplyEvent {
				source := wamp.Event(callEvent)
				var n int
				e := callEvent.Payload(&n)
				if e == nil {
					for i := n; i > 0; i-- {
						source, _ = wamp.Yield(source, i)
					}
					return wamp.NewReplyEvent(source, 0)
				}
				return wamp.NewErrorEvent(callEvent, errors.New("InvalidPayload"))
			},
		)
		if e == nil {
			t.Log("register success")
		} else {
			t.Fatalf("register error %s", e)
		}

		generator := wamp.NewRemoteGenerator[int](
			betaSession,
			&wamp.CallFeatures{URI: "net.example.reverse"},
			100,
		)
		for generator.Active() {
			_, result, e := generator.Next(wamp.DEFAULT_TIMEOUT)
			if e == nil {
				t.Logf("result %d", result)
			} else {
				t.Fatalf("generator error %s", e)
			}
		}

		e = wamp.Unregister(alphaSession, registration.ID)
		if e == nil {
			t.Log("unregister success")
		} else {
			t.Fatalf("unregister error %s", e)
		}
	})
}
