package shared

import "testing"

func TestSimpleSet(t *testing.T) {
	instance := NewSet[string]([]string{})

	instance.Add("alpha")
	if !instance.Contains("alpha") {
		t.Fatal("KeyNotFound")
	}

	instance.Add("alpha")
	if !instance.Contains("alpha") {
		t.Fatal("KeyNotFound")
	}

	instance.Add("beta")
	if !instance.Contains("alpha") {
		t.Fatal("KeyNotFound")
	}
	if !instance.Contains("beta") {
		t.Fatal("KeyNotFound")
	}
}
