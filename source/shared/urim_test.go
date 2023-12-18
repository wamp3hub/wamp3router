package routerShared_test

import (
	"log/slog"
	"testing"

	wamp "github.com/wamp3hub/wamp3go"
	wampShared "github.com/wamp3hub/wamp3go/shared"
	routerShared "github.com/wamp3hub/wamp3router/source/shared"
	routerStorages "github.com/wamp3hub/wamp3router/source/storages"
)

func TestURIM(t *testing.T) {
	expectedRegistration := wamp.Registration{
		ID:       wampShared.NewID(),
		URI:      "net.example.echo",
		AuthorID: wampShared.NewID(),
		Options:  nil,
	}

	logger := slog.Default()

	storagePath := "/tmp/" + wampShared.NewID() + ".db"
	storage, _ := routerStorages.NewBoltDBStorage(storagePath)

	urim := routerShared.NewURIM[*wamp.RegisterOptions](storage, logger)
	e := urim.Add(&expectedRegistration)
	if e != nil {
		t.Fatalf("invalid behaviour %s", e)
	}

	registrationList := urim.Match("net.example.echo")
	if len(registrationList) != 1 {
		t.Fatalf("invalid behaviour")
	}

	registrationsCount := urim.Count("net.example.echo")
	if registrationsCount != 1 {
		t.Fatalf("invalid behaviour")
	}

	removedResourceList := urim.DeleteByAuthor(expectedRegistration.AuthorID, "")
	if len(removedResourceList) != 1 {
		t.Fatalf("invalid behaviour")
	}

	registrationList = urim.GetByAuthor(expectedRegistration.AuthorID)
	if len(registrationList) != 0 {
		t.Fatalf("invalid behaviour")
	}
}
