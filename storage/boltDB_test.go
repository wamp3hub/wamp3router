package storage

import (
	"testing"

	"github.com/rs/xid"
)

func TestHappyPathBoltDB(t *testing.T) {
	path := "/tmp/" + xid.New().String() + ".db"
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
