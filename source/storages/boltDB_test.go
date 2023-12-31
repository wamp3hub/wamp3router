package routerStorages

import (
	"testing"

	"github.com/wamp3hub/wamp3go/shared"
)

func TestBoltDB(t *testing.T) {
	path := "/tmp/" + wampShared.NewID() + ".db"
	storage, e := NewBoltDBStorage(path)
	if e != nil {
		t.Fatal(e)
	}

	if e = storage.Set("test", "alpha", true); e != nil {
		t.Fatal(e)
	}
	if e = storage.Set("test", "beta", true); e != nil {
		t.Fatal(e)
	}
	v := new(bool)
	if e = storage.Get("test", "alpha", v); e != nil && *v != true {
		t.Fatal(e)
	}
	if e = storage.Get("test", "beta", v); e != nil && *v != true {
		t.Fatal(e)
	}
	storage.Delete("test", "alpha")
	if e = storage.Get("test", "alpha", v); e == nil {
		t.Fatal(e)
	}

	if e = storage.Destroy(); e != nil {
		t.Fatal(e)
	}
}
