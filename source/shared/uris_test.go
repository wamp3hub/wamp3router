package routerShared_test

import (
	"strings"
	"testing"

	wampShared "github.com/wamp3hub/wamp3go/shared"
	routerShared "github.com/wamp3hub/wamp3router/source/shared"
)

func insertResource[T any](root *routerShared.URISegment[T], path routerShared.Path, resourceID string, data T) {
	segment := root.GetSert(path)
	segment.Data[resourceID] = data
}

func deleteResource[T any](root *routerShared.URISegment[T], path routerShared.Path, resourceID string) {
	segment := root.GetSert(path)
	delete(segment.Data, resourceID)
}

func TestDump(t *testing.T) {
	root := routerShared.NewURISegment[struct{}](nil)

	temporaryResourcePath := routerShared.Path{"wamp", "test"}
	temporaryResourceID := wampShared.NewID()
	insertResource(root, temporaryResourcePath, temporaryResourceID, struct{}{})
	deleteResource(root, temporaryResourcePath, temporaryResourceID)

	expectedPathList := []routerShared.Path{
		routerShared.Path{"wamp", "subscription", "new"},
		routerShared.Path{"wamp", "registration", "new"},
		routerShared.Path{"wamp", "*"},
	}
	for _, path := range expectedPathList {
		insertResource(root, path, wampShared.NewID(), struct{}{})
	}

	t.Run("Case: Get particular", func(t *testing.T) {
		result := root.Get(routerShared.Path{"wamp", "registration", "new"})
		if result == nil {
			t.Fatalf("get returns unexpected values")
		}
	})

	t.Run("Case: Match particular", func(t *testing.T) {
		result := root.Match(routerShared.Path{"wamp", "registration", "new"})
		if len(result) != 1 {
			t.Fatalf("match returns unexpected values size=%d", len(result))
		}
	})

	t.Run("Case: Match wildcard", func(t *testing.T) {
		result := root.Match(routerShared.Path{"wamp", "example"})
		if len(result) != 1 {
			t.Fatalf("match returns unexpected values size=%d", len(result))
		}
	})

	t.Run("Case: Dump", func(t *testing.T) {
		pathDump := root.PathDump()
		if len(expectedPathList) != len(pathDump) {
			t.Fatalf("dump returns unexpected values")
		}

		URISet := routerShared.NewEmptySet[string]()
		for _, path := range pathDump {
			uri := strings.Join(path, ".")
			URISet.Add(uri)
		}

		for _, path := range expectedPathList {
			uri := strings.Join(path, ".")
			if URISet.Contains(uri) {
				continue
			}

			t.Fatalf("dump did not return required value")
		}
	})
}
