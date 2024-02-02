package router_test

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	wampTransports "github.com/wamp3hub/wamp3go/transports"
	router "github.com/wamp3hub/wamp3router/source"
	routerStorages "github.com/wamp3hub/wamp3router/source/storages"
)

func runRouter() *wampShared.Observable[*wamp.Peer] {
	routerID := wampShared.NewID()
	storagePath := "/tmp/wamp3rd-" + routerID + ".db"
	storage, _ := routerStorages.NewBoltDBStorage(storagePath)
	__router := router.NewRouter(
		routerID,
		storage,
		slog.Default(),
	)
	__router.Serve()
	return __router.Newcomers
}

func joinSession(
	newcomers *wampShared.Observable[*wamp.Peer],
) *wamp.Session {
	logger := slog.Default()
	alphaID := wampShared.NewID()
	lTransport, rTransport := wampTransports.NewDuplexLocalTransport(128)
	lPeer := wamp.SpawnPeer(alphaID, lTransport, logger)
	rPeer := wamp.SpawnPeer(alphaID, rTransport, logger)
	session := wamp.NewSession(rPeer, logger)
	newcomers.Next(lPeer)
	time.Sleep(time.Second)
	return session
}

func TestSubscribePublish(t *testing.T) {
	nextNewcomer := runRouter()

	t.Run("Case: Happy Path", func(t *testing.T) {
		alphaSession := joinSession(nextNewcomer)
		betaSession := joinSession(nextNewcomer)

		wg := new(sync.WaitGroup)

		subscription, e := wamp.Subscribe(
			alphaSession,
			"net.example",
			&wamp.SubscribeOptions{},
			func(message string, publishEvent wamp.PublishEvent) {
				t.Logf("new message %s", message)
				wg.Done()
			},
		)
		if e == nil {
			t.Logf("subscribe success ID=%s", subscription.ID)
		} else {
			t.Fatalf("subscribe error %s", e)
		}

		wg.Add(1)

		e = wamp.Publish(
			betaSession,
			&wamp.PublishFeatures{URI: "net.example"},
			"Hello, I'm session beta!",
		)
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
	nextNewcomer := runRouter()

	t.Run("Case: Happy Path", func(t *testing.T) {
		alphaSession := joinSession(nextNewcomer)
		betaSession := joinSession(nextNewcomer)

		registration, e := wamp.Register(
			alphaSession,
			"net.example.greeting",
			&wamp.RegisterOptions{},
			func(name string, callEvent wamp.CallEvent) (string, error) {
				result := "Hello, " + name + "!"
				return result, nil
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
		alphaSession := joinSession(nextNewcomer)
		betaSession := joinSession(nextNewcomer)

		_, e := wamp.Register(
			alphaSession,
			"net.example.long",
			&wamp.RegisterOptions{},
			func(payload any, callEvent wamp.CallEvent) (struct{}, error) {
				time.Sleep(time.Minute)
				return struct{}{}, nil
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
		session := joinSession(nextNewcomer)

		pendingResponse := wamp.Call[struct{}](
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
	nextNewcomer := runRouter()

	alphaSession := joinSession(nextNewcomer)

	_, e := wamp.Register(
		alphaSession,
		"net.example.reverse",
		&wamp.RegisterOptions{},
		func(n int, callEvent wamp.CallEvent) (int, error) {
			source := wamp.Event(callEvent)
			for i := n; i > -1; i-- {
				source = wamp.Yield(source, i)
			}
			return -1, wamp.GeneratorExit(source)
		},
	)
	if e == nil {
		t.Log("register success")
	} else {
		t.Fatalf("register error %s", e)
	}

	t.Run("Case: Happy Path", func(t *testing.T) {
		betaSession := joinSession(nextNewcomer)

		generator, e := wamp.CallGenerator[int](
			betaSession,
			&wamp.CallFeatures{URI: "net.example.reverse"},
			10,
		)
		if e == nil {
			t.Log("generator success")
		} else {
			t.Fatalf("generator error %s", e)
		}
		for generator.Active() {
			_, result, e := generator.Next(wamp.DEFAULT_TIMEOUT)
			if e == nil {
				t.Logf("result %d", result)
			} else if e.Error() == "GeneratorExit" {
				t.Logf("generator done")
			} else {
				t.Fatalf("generator error %s", e)
			}
		}
	})

	t.Run("Case: Stop", func(t *testing.T) {
		betaSession := joinSession(nextNewcomer)

		generator, e := wamp.CallGenerator[int](
			betaSession,
			&wamp.CallFeatures{URI: "net.example.reverse"},
			100,
		)
		if e == nil {
			t.Log("generator successfully created")
		} else {
			t.Fatalf("create generator error %s", e)
		}
		for i := 0; i < 10; i++ {
			_, result, e := generator.Next(wamp.DEFAULT_TIMEOUT)
			if e == nil {
				t.Logf("result %d", result)
			} else if e.Error() == "GeneratorExit" {
				t.Logf("generator done")
			} else {
				t.Fatalf("generator error %s", e)
			}
		}

		e = generator.Stop()
		if e == nil {
			t.Log("stop generator success")
		} else {
			t.Fatalf("stop generator error %s", e)
		}
	})
}
