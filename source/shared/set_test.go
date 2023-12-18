package routerShared_test

import (
	"testing"

	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

func TestSet(t *testing.T) {
	instance := routerShared.NewSet[string]([]string{"alpha"})

	if !instance.Contains("alpha") {
		t.Fatal("ItemNotFound")
	}

	instance.Add("alpha")
	if !instance.Contains("alpha") {
		t.Fatal("ItemNotFound")
	}

	instance.Add("beta")
	if !instance.Contains("beta") {
		t.Fatal("ItemNotFound")
	}

	if instance.Size() != 2 {
		t.Fatal("InvalidSize")
	}

	instance.Delete("alpha")

	items := instance.Values()
	if len(items) != 1 {
		t.Fatal("Invalid behaviour")
	}
}
