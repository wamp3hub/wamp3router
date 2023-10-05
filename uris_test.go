package wamp3router

import (
	"testing"

	"github.com/rs/xid"

	"strings"
)

func insertResource[T any](root *URISegment[T], path Path, resourceID string, data T) {
	segment := root.GetSert(path)
	segment.Data[resourceID] = data
}

func deleteResource[T any](root *URISegment[T], path Path, resourceID string) {
	segment := root.GetSert(path)
	delete(segment.Data, resourceID)
}

func TestDump(t *testing.T) {
	root := newURISegment[emptiness](nil)

	temporaryResourcePath := Path{"wamp", "test"}
	temporaryResourceID := xid.New().String()
	insertResource(root, temporaryResourcePath, temporaryResourceID, emptiness{})
	deleteResource(root, temporaryResourcePath, temporaryResourceID)

	expectedPathList := []Path{
		Path{"wamp", "subscription", "new"},
		Path{"wamp", "registration", "new"},
	}
	for _, path := range expectedPathList {
		insertResource(root, path, xid.New().String(), emptiness{})
	}

	pathDump := root.PathDump()
	if len(expectedPathList) != len(pathDump) {
		t.Fatalf("dump returns unexpected values")
	}

	URISet := NewEmptySet[string]()
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
}
